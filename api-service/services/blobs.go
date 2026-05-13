package services

import (
	"blob-service/internal"
	pbbl "blob-service/pb/pinax/ethereum/blobs/v1"
	"context"
	"encoding/binary"
	"fmt"
	"strconv"

	pbkv "github.com/streamingfast/substreams-sink-kv/pb/substreams/sink/kv/v1"

	"google.golang.org/protobuf/proto"
)

type BlobsService struct {
	sinkClient  pbkv.KvClient
	finalityLag uint64
}

func NewBlobsService(sinkClient pbkv.KvClient, finalityLag uint64) *BlobsService {
	return &BlobsService{sinkClient: sinkClient, finalityLag: finalityLag}
}

// GetHeadSlot returns the current head slot from the sink.
func (bs *BlobsService) GetHeadSlot(ctx context.Context) (uint64, error) {
	resp, err := bs.sinkClient.Get(ctx, &pbkv.GetRequest{Key: "head"})
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(resp.GetValue()), nil
}

// IsFinalized reports whether `slot` is at least `finalityLag` slots behind
// `headSlot`. This is a heuristic, not a real consensus-layer finality check:
// under normal conditions blocks finalize within 2 epochs, but extended
// non-finality periods can produce false positives. When finalityLag is 0
// finality tracking is disabled and every slot is reported as finalized.
//
// The caller is expected to fetch the head slot once per request (see
// GetSlotByBlockId) and pass it in here, so finality determination is free
// of additional RPCs.
func (bs *BlobsService) IsFinalized(slot, headSlot uint64) bool {
	if bs.finalityLag == 0 {
		return true
	}
	return headSlot >= slot+bs.finalityLag
}

func (bs *BlobsService) GetSlotNumber(ctx context.Context, block_id string) (uint64, error) {
	if block_id == "head" {
		return bs.GetHeadSlot(ctx)
	}

	if len(block_id) > 2 && block_id[:2] == "0x" {
		resp, err := bs.sinkClient.Get(ctx, &pbkv.GetRequest{Key: "block_root:" + block_id})
		if err != nil {
			return 0, err
		}
		return binary.BigEndian.Uint64(resp.GetValue()), nil
	}

	slot, err := strconv.ParseUint(block_id, 10, 64)
	if err != nil {
		return 0, internal.ErrInvalidSlot
	}

	return slot, nil
}

// GetSlotByBlockId fetches the head slot once, uses it to resolve
// blockId == "head", and returns it alongside the requested slot so callers
// can compute finality without a second RPC.
func (bs *BlobsService) GetSlotByBlockId(ctx context.Context, blockId string) (*pbbl.Slot, uint64, error) {

	headSlot, err := bs.GetHeadSlot(ctx)
	if err != nil {
		return nil, 0, err
	}

	var slotNum uint64
	if blockId == "head" {
		slotNum = headSlot
	} else {
		slotNum, err = bs.GetSlotNumber(ctx, blockId)
		if err != nil {
			return nil, 0, err
		}
	}

	resp, err := bs.sinkClient.Get(ctx, &pbkv.GetRequest{Key: fmt.Sprintf("slot:%d", slotNum)})
	if err != nil {
		return nil, 0, err
	}

	slot := &pbbl.Slot{}
	if err := proto.Unmarshal(resp.GetValue(), slot); err != nil {
		return nil, 0, err
	}

	return slot, headSlot, nil
}
