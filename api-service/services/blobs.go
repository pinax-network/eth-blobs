package services

import (
	"blob-service/internal"
	pbbl "blob-service/pb/pinax/ethereum/blobs/v1"
	"context"
	"encoding/binary"
	"fmt"
	"strconv"

	pbkv "github.com/streamingfast/substreams-sink-kv/pb/substreams/sink/kv/v1"

	"github.com/golang/protobuf/proto"
)

type BlobsService struct {
	sinkClient pbkv.KvClient
}

func NewBlobsService(sinkClient pbkv.KvClient) *BlobsService {
	return &BlobsService{sinkClient: sinkClient}
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
