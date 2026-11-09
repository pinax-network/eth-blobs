package services

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/pinax-network/blob-service/blobs/internal"
	pbbl "github.com/pinax-network/blob-service/blobs/pb/pinax/ethereum/blobs/v1"
	"github.com/pinax-network/blob-service/blobs/store"
)

type BlobsService struct {
	store       store.Store
	finalityLag uint64

	headTTL   time.Duration
	headMu    sync.Mutex
	head      store.Head
	headAt    time.Time
	headValid bool
}

// NewBlobsService serves slots from the store. headTTL bounds how long a
// fetched head is reused; zero disables caching.
func NewBlobsService(st store.Store, finalityLag uint64, headTTL time.Duration) *BlobsService {
	return &BlobsService{store: st, finalityLag: finalityLag, headTTL: headTTL}
}

// GetHead returns the newest head any writer has published, cached for headTTL.
func (bs *BlobsService) GetHead(ctx context.Context) (store.Head, error) {
	bs.headMu.Lock()
	defer bs.headMu.Unlock()
	if bs.headValid && bs.headTTL > 0 && time.Since(bs.headAt) < bs.headTTL {
		return bs.head, nil
	}
	head, err := bs.store.GetHead(ctx)
	if err != nil {
		return store.Head{}, err
	}
	bs.head, bs.headAt, bs.headValid = head, time.Now(), true
	return head, nil
}

// GetHeadSlot returns the current head slot.
func (bs *BlobsService) GetHeadSlot(ctx context.Context) (uint64, error) {
	head, err := bs.GetHead(ctx)
	if err != nil {
		return 0, err
	}
	return head.Slot, nil
}

// IsFinalized reports whether `slot` is at least `finalityLag` slots behind
// `headSlot`. This is a heuristic, not a real consensus-layer finality check:
// under normal conditions blocks finalize within 2 epochs, but extended
// non-finality periods can produce false positives. When finalityLag is 0
// finality tracking is disabled and every slot is reported as finalized.
func (bs *BlobsService) IsFinalized(slot, headSlot uint64) bool {
	if bs.finalityLag == 0 {
		return true
	}
	return headSlot >= slot+bs.finalityLag
}

// GetSlotNumber resolves a Beacon API block_id ("head", a slot number or a
// 0x-prefixed block root) to a slot number.
func (bs *BlobsService) GetSlotNumber(ctx context.Context, blockId string) (uint64, error) {
	if blockId == "head" {
		return bs.GetHeadSlot(ctx)
	}

	if len(blockId) > 2 && (blockId[:2] == "0x" || blockId[:2] == "0X") {
		root, err := store.NormalizeRoot(blockId)
		if err != nil {
			return 0, internal.ErrInvalidSlot
		}
		return bs.store.GetRoot(ctx, root)
	}

	slot, err := strconv.ParseUint(blockId, 10, 64)
	if err != nil {
		return 0, internal.ErrInvalidSlot
	}
	return slot, nil
}

// GetSlotByBlockId fetches the head once, resolves blockId against it, loads
// the slot and returns both so callers can compute finality without another
// round trip. A slot reached through a block root must carry that root: after
// a reorg the root index can still point at a slot now holding another block.
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

	data, err := bs.store.GetSlot(ctx, slotNum)
	if err != nil {
		return nil, 0, err
	}

	slot := &pbbl.Slot{}
	if err := proto.Unmarshal(data, slot); err != nil {
		return nil, 0, fmt.Errorf("slot %d: unmarshal: %w", slotNum, err)
	}

	if len(blockId) > 2 && (blockId[:2] == "0x" || blockId[:2] == "0X") {
		want, _ := store.NormalizeRoot(blockId)
		if store.RootHex(slot.Root) != want {
			return nil, 0, fmt.Errorf("root %s no longer canonical at slot %d: %w", want, slotNum, store.ErrNotFound)
		}
	}

	return slot, headSlot, nil
}
