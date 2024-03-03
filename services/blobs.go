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

func (bs *BlobsService) GetSlotNumber(ctx context.Context, block_id string) (uint64, error) {

	slot, err := strconv.ParseUint(block_id, 10, 64)
	if err == nil {
		return slot, nil
	}

	slotInfo, err := bs.GetSlotInfoById(ctx, block_id)
	if err != nil {
		return 0, err
	}

	return slotInfo.Slot, nil
}

func (bs *BlobsService) GetSlotInfoById(ctx context.Context, block_id string) (*pbbl.SlotInfo, error) {

	if block_id == "head" {
		resp, err := bs.sinkClient.Get(ctx, &pbkv.GetRequest{Key: "head"})
		if err != nil {
			return nil, err
		}
		slotInfo := &pbbl.SlotInfo{}
		err = proto.Unmarshal(resp.GetValue(), slotInfo)
		if err != nil {
			return nil, err
		}
		return slotInfo, nil
	}

	if len(block_id) > 2 && block_id[:2] == "0x" {
		resp, err := bs.sinkClient.Get(ctx, &pbkv.GetRequest{Key: "block_root:" + block_id})
		if err != nil {
			return 0, err
		}
		return binary.BigEndian.Uint64(resp.GetValue()), nil
	}

	return 0, internal.ErrInvalidSlot
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
