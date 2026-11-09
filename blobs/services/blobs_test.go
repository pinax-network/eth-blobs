package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/pinax-network/blob-service/blobs/internal"
	pbbl "github.com/pinax-network/blob-service/blobs/pb/pinax/ethereum/blobs/v1"
	"github.com/pinax-network/blob-service/blobs/store"
)

const (
	rootA = "0x" + "aa" + "00000000000000000000000000000000000000000000000000000000000000"
	rootB = "0x" + "bb" + "00000000000000000000000000000000000000000000000000000000000000"
)

func rootBytes(t *testing.T, root string) []byte {
	t.Helper()
	b := make([]byte, 32)
	for i := 0; i < 32; i++ {
		var v byte
		for _, c := range root[2+2*i : 4+2*i] {
			v <<= 4
			switch {
			case c >= '0' && c <= '9':
				v |= byte(c - '0')
			case c >= 'a' && c <= 'f':
				v |= byte(c-'a') + 10
			}
		}
		b[i] = v
	}
	return b
}

func putSlot(t *testing.T, st *store.MemStore, s *pbbl.Slot) {
	t.Helper()
	raw, err := proto.Marshal(s)
	if err != nil {
		t.Fatalf("marshal slot: %v", err)
	}
	if err := st.PutSlot(context.Background(), s.Slot, raw); err != nil {
		t.Fatal(err)
	}
	if len(s.Root) == 32 {
		if err := st.PutRoot(context.Background(), store.RootHex(s.Root), s.Slot); err != nil {
			t.Fatal(err)
		}
	}
}

func putHead(t *testing.T, st *store.MemStore, writer string, slot uint64) {
	t.Helper()
	if err := st.PutHead(context.Background(), writer, store.Head{Slot: slot}); err != nil {
		t.Fatal(err)
	}
}

func newStore(t *testing.T) *store.MemStore {
	t.Helper()
	st := store.NewMemStore()
	putHead(t, st, "riv", 1000)
	st.Calls = nil
	return st
}

func TestIsFinalized(t *testing.T) {
	tests := []struct {
		name        string
		finalityLag uint64
		slot, head  uint64
		want        bool
	}{
		{"lag=0 disables tracking (slot==head)", 0, 1000, 1000, true},
		{"lag=0 disables tracking (slot>head)", 0, 9999, 1000, true},
		{"exactly at boundary", 64, 936, 1000, true},
		{"one slot short of boundary", 64, 937, 1000, false},
		{"far behind head", 64, 100, 1000, true},
		{"slot equals head", 64, 1000, 1000, false},
		{"slot ahead of head (no underflow)", 64, 2000, 1000, false},
		{"large lag", 1024, 0, 1024, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bs := NewBlobsService(store.NewMemStore(), tc.finalityLag, 0)
			if got := bs.IsFinalized(tc.slot, tc.head); got != tc.want {
				t.Errorf("slot=%d head=%d lag=%d: got=%v want=%v", tc.slot, tc.head, tc.finalityLag, got, tc.want)
			}
		})
	}
}

func TestGetSlotNumber(t *testing.T) {
	st := newStore(t)
	putSlot(t, st, &pbbl.Slot{Slot: 42, Root: rootBytes(t, rootA)})
	bs := NewBlobsService(st, 64, 0)
	ctx := context.Background()

	tests := []struct {
		name    string
		in      string
		want    uint64
		wantErr error
	}{
		{"head", "head", 1000, nil},
		{"block root", rootA, 42, nil},
		{"block root uppercase hex", "0x" + "AA" + "00000000000000000000000000000000000000000000000000000000000000", 42, nil},
		{"unknown root", rootB, 0, store.ErrNotFound},
		{"malformed root", "0xdead", 0, internal.ErrInvalidSlot},
		{"numeric", "12345", 12345, nil},
		{"invalid", "notaslot", 0, internal.ErrInvalidSlot},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := bs.GetSlotNumber(ctx, tc.in)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("got=%d want=%d", got, tc.want)
			}
		})
	}
}

func TestGetSlotByBlockId_Head(t *testing.T) {
	st := newStore(t)
	putSlot(t, st, &pbbl.Slot{Slot: 1000})
	st.Calls = nil
	bs := NewBlobsService(st, 64, 0)

	slot, head, err := bs.GetSlotByBlockId(context.Background(), "head")
	if err != nil {
		t.Fatal(err)
	}
	if head != 1000 || slot.Slot != 1000 {
		t.Errorf("got head=%d slot=%d, want 1000/1000", head, slot.Slot)
	}
	if got := st.CallsFor("head/"); got != 1 {
		t.Errorf("expected head fetched exactly once, got %d (calls=%v)", got, st.Calls)
	}
	if got := st.CallsFor(store.SlotKey(1000)); got != 1 {
		t.Errorf("expected slot fetched once, got %d", got)
	}
}

func TestGetSlotByBlockId_HeadIsMaxAcrossWriters(t *testing.T) {
	st := newStore(t)
	putHead(t, st, "kan", 1003)
	putSlot(t, st, &pbbl.Slot{Slot: 1003})
	bs := NewBlobsService(st, 64, 0)

	slot, head, err := bs.GetSlotByBlockId(context.Background(), "head")
	if err != nil {
		t.Fatal(err)
	}
	if head != 1003 || slot.Slot != 1003 {
		t.Errorf("got head=%d slot=%d, want 1003/1003", head, slot.Slot)
	}
}

func TestGetSlotByBlockId_Numeric(t *testing.T) {
	st := newStore(t)
	putSlot(t, st, &pbbl.Slot{Slot: 500})
	st.Calls = nil
	bs := NewBlobsService(st, 64, 0)

	slot, head, err := bs.GetSlotByBlockId(context.Background(), "500")
	if err != nil {
		t.Fatal(err)
	}
	if head != 1000 || slot.Slot != 500 {
		t.Errorf("got head=%d slot=%d, want 1000/500", head, slot.Slot)
	}
	// head + slot, no root lookup
	if got := len(st.Calls); got != 2 {
		t.Errorf("expected 2 store calls (head, slot), got %d: %v", got, st.Calls)
	}
}

func TestGetSlotByBlockId_BlockRoot(t *testing.T) {
	st := newStore(t)
	putSlot(t, st, &pbbl.Slot{Slot: 700, Root: rootBytes(t, rootA)})
	st.Calls = nil
	bs := NewBlobsService(st, 64, 0)

	slot, head, err := bs.GetSlotByBlockId(context.Background(), rootA)
	if err != nil {
		t.Fatal(err)
	}
	if head != 1000 || slot.Slot != 700 {
		t.Errorf("got head=%d slot=%d, want 1000/700", head, slot.Slot)
	}
	if got := len(st.Calls); got != 3 {
		t.Errorf("expected 3 store calls (head, root, slot), got %d: %v", got, st.Calls)
	}
}

func TestGetSlotByBlockId_StaleRootAfterReorgIsNotFound(t *testing.T) {
	st := newStore(t)
	// The root index still points rootA at slot 700, but slot 700 now holds
	// the block with rootB (the sink rewrote it after a reorg).
	putSlot(t, st, &pbbl.Slot{Slot: 700, Root: rootBytes(t, rootA)})
	putSlot(t, st, &pbbl.Slot{Slot: 700, Root: rootBytes(t, rootB)})
	bs := NewBlobsService(st, 64, 0)

	_, _, err := bs.GetSlotByBlockId(context.Background(), rootA)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for a stale root, got %v", err)
	}
	if _, _, err := bs.GetSlotByBlockId(context.Background(), rootB); err != nil {
		t.Fatalf("canonical root must resolve, got %v", err)
	}
}

func TestGetSlotByBlockId_NotFound(t *testing.T) {
	st := newStore(t)
	bs := NewBlobsService(st, 64, 0)

	_, _, err := bs.GetSlotByBlockId(context.Background(), "head")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestGetSlotByBlockId_NoHeadYet(t *testing.T) {
	st := store.NewMemStore()
	bs := NewBlobsService(st, 64, 0)

	_, _, err := bs.GetSlotByBlockId(context.Background(), "head")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expected ErrNotFound when no writer has published a head, got %v", err)
	}
}

func TestGetHead_CachedWithinTTL(t *testing.T) {
	st := newStore(t)
	bs := NewBlobsService(st, 64, time.Minute)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if _, err := bs.GetHeadSlot(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if got := st.CallsFor("head/"); got != 1 {
		t.Errorf("expected a single head fetch within the TTL, got %d", got)
	}

	// A newer head published by a writer is not seen until the TTL expires.
	putHead(t, st, "riv", 1001)
	if slot, _ := bs.GetHeadSlot(ctx); slot != 1000 {
		t.Errorf("expected cached head 1000, got %d", slot)
	}
	bs.headAt = time.Now().Add(-2 * time.Minute)
	if slot, _ := bs.GetHeadSlot(ctx); slot != 1001 {
		t.Errorf("expected refreshed head 1001, got %d", slot)
	}
}

func TestGetHead_NoCacheWhenTTLZero(t *testing.T) {
	st := newStore(t)
	bs := NewBlobsService(st, 64, 0)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if _, err := bs.GetHeadSlot(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if got := st.CallsFor("head/"); got != 3 {
		t.Errorf("expected 3 head fetches without a TTL, got %d", got)
	}
}
