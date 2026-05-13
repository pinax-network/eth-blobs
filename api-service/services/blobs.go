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

// IsFinalized returns true when the given slot is at least `finalityLag`
// slots behind head. This is a heuristic, not a real consensus-layer
// finality check — under normal conditions blocks finalize within 2 epochs,
// but extended non-finality periods can produce false positives. On head
// lookup error this returns (false, err) so callers can fall back to a
// conservative finalized=false.
func (bs *BlobsService) IsFinalized(ctx context.Context, slot uint64) (bool, error) {
	head, err := bs.GetHeadSlot(ctx)
	if err != nil {
		return false, err
	}
	return head >= slot+bs.finalityLag, nil
}

func (bc *BlobsService) GetSlotNumber(ctx context.Context, block_id string) (uint64, error) {
	if block_id == "head" {
		resp, err := bc.sinkClient.Get(ctx, &pbkv.GetRequest{Key: "head"})
		if err != nil {
			return 0, err
		}
		return binary.BigEndian.Uint64(resp.GetValue()), nil
	}

	if len(block_id) > 2 && block_id[:2] == "0x" {
		resp, err := bc.sinkClient.Get(ctx, &pbkv.GetRequest{Key: "block_root:" + block_id})
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

func (bs *BlobsService) GetSlotByBlockId(ctx context.Context, blockId string) (*pbbl.Slot, error) {

	slotNum, err := bs.GetSlotNumber(ctx, blockId)
	if err != nil {
		return nil, err
	}

	resp, err := bs.sinkClient.Get(ctx, &pbkv.GetRequest{Key: fmt.Sprintf("slot:%d", slotNum)})
	if err != nil {
		return nil, err
	}

	slot := &pbbl.Slot{}
	err = proto.Unmarshal(resp.GetValue(), slot)
	if err != nil {
		return nil, err
	}

	return slot, nil
}
