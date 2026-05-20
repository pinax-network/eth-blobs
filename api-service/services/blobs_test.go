package services

import (
	"blob-service/internal"
	pbbl "blob-service/pb/pinax/ethereum/blobs/v1"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"

	pbkv "github.com/streamingfast/substreams-sink-kv/pb/substreams/sink/kv/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// fakeKv is a minimal in-memory KvClient implementation. Test cases preload
// `values` (and optionally `notFound`); every Get is recorded in `calls` so
// assertions can verify call counts/order.
type fakeKv struct {
	pbkv.KvClient
	values   map[string][]byte
	notFound map[string]bool
	calls    []string
}

func newFakeKv() *fakeKv {
	return &fakeKv{
		values:   map[string][]byte{},
		notFound: map[string]bool{},
	}
}

func (f *fakeKv) Get(ctx context.Context, in *pbkv.GetRequest, opts ...grpc.CallOption) (*pbkv.GetResponse, error) {
	f.calls = append(f.calls, in.Key)
	if f.notFound[in.Key] {
		return nil, status.Error(codes.NotFound, "not found")
	}
	v, ok := f.values[in.Key]
	if !ok {
		return nil, fmt.Errorf("fakeKv: unexpected key %q", in.Key)
	}
	return &pbkv.GetResponse{Value: v}, nil
}

func (f *fakeKv) callsFor(key string) int {
	n := 0
	for _, k := range f.calls {
		if k == key {
			n++
		}
	}
	return n
}

func u64Bytes(n uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return b
}

func marshalSlot(t *testing.T, s *pbbl.Slot) []byte {
	t.Helper()
	b, err := proto.Marshal(s)
	if err != nil {
		t.Fatalf("marshal slot: %v", err)
	}
	return b
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
			bs := NewBlobsService(newFakeKv(), tc.finalityLag)
			if got := bs.IsFinalized(tc.slot, tc.head); got != tc.want {
				t.Errorf("slot=%d head=%d lag=%d: got=%v want=%v", tc.slot, tc.head, tc.finalityLag, got, tc.want)
			}
		})
	}
}

func TestIsFinalizedZeroLagDoesNoRPC(t *testing.T) {
	kv := newFakeKv()
	bs := NewBlobsService(kv, 0)
	_ = bs.IsFinalized(1234, 0)
	if len(kv.calls) != 0 {
		t.Errorf("expected zero RPCs when finalityLag=0, got %v", kv.calls)
	}
}

func TestGetSlotNumber(t *testing.T) {
	kv := newFakeKv()
	kv.values["head"] = u64Bytes(7777)
	kv.values["block_root:0xdead"] = u64Bytes(42)
	bs := NewBlobsService(kv, 64)
	ctx := context.Background()

	tests := []struct {
		name    string
		in      string
		want    uint64
		wantErr error
	}{
		{"head", "head", 7777, nil},
		{"block root", "0xdead", 42, nil},
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

func TestGetSlotByBlockId_HeadFetchedOnceForHeadRequest(t *testing.T) {
	kv := newFakeKv()
	kv.values["head"] = u64Bytes(1000)
	kv.values["slot:1000"] = marshalSlot(t, &pbbl.Slot{Slot: 1000})
	bs := NewBlobsService(kv, 64)

	slot, head, err := bs.GetSlotByBlockId(context.Background(), "head")
	if err != nil {
		t.Fatal(err)
	}
	if head != 1000 || slot.Slot != 1000 {
		t.Errorf("got head=%d slot=%d, want 1000/1000", head, slot.Slot)
	}
	if got := kv.callsFor("head"); got != 1 {
		t.Errorf("expected head fetched exactly once, got %d (calls=%v)", got, kv.calls)
	}
	if got := kv.callsFor("slot:1000"); got != 1 {
		t.Errorf("expected slot fetched once, got %d", got)
	}
}

func TestGetSlotByBlockId_Numeric(t *testing.T) {
	kv := newFakeKv()
	kv.values["head"] = u64Bytes(1000)
	kv.values["slot:500"] = marshalSlot(t, &pbbl.Slot{Slot: 500})
	bs := NewBlobsService(kv, 64)

	slot, head, err := bs.GetSlotByBlockId(context.Background(), "500")
	if err != nil {
		t.Fatal(err)
	}
	if head != 1000 || slot.Slot != 500 {
		t.Errorf("got head=%d slot=%d, want 1000/500", head, slot.Slot)
	}
	// head + slot, no block_root lookup
	if got := len(kv.calls); got != 2 {
		t.Errorf("expected 2 RPCs (head, slot), got %d: %v", got, kv.calls)
	}
}

func TestGetSlotByBlockId_BlockRoot(t *testing.T) {
	kv := newFakeKv()
	kv.values["head"] = u64Bytes(1000)
	kv.values["block_root:0xabc"] = u64Bytes(700)
	kv.values["slot:700"] = marshalSlot(t, &pbbl.Slot{Slot: 700})
	bs := NewBlobsService(kv, 64)

	slot, head, err := bs.GetSlotByBlockId(context.Background(), "0xabc")
	if err != nil {
		t.Fatal(err)
	}
	if head != 1000 || slot.Slot != 700 {
		t.Errorf("got head=%d slot=%d, want 1000/700", head, slot.Slot)
	}
	if got := len(kv.calls); got != 3 {
		t.Errorf("expected 3 RPCs (head, block_root, slot), got %d: %v", got, kv.calls)
	}
}

func TestGetSlotByBlockId_NotFound(t *testing.T) {
	kv := newFakeKv()
	kv.values["head"] = u64Bytes(1000)
	kv.notFound["slot:1000"] = true
	bs := NewBlobsService(kv, 64)

	_, _, err := bs.GetSlotByBlockId(context.Background(), "head")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if st, ok := status.FromError(err); !ok || st.Code() != codes.NotFound {
		t.Errorf("expected codes.NotFound, got %v", err)
	}
}
