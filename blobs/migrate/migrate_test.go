package migrate

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	pbkv "github.com/streamingfast/substreams-sink-kv/pb/substreams/sink/kv/v1"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	pbbl "github.com/pinax-network/blob-service/blobs/pb/pinax/ethereum/blobs/v1"
	"github.com/pinax-network/blob-service/blobs/store"
)

// fakeKV serves the two kinds of keys migrate reads from substreams-sink-kv:
// "head" (8-byte big-endian slot) and "slot:<n>" (serialized Slot).
type fakeKV struct {
	pbkv.UnimplementedKvServer
	head  uint64
	slots map[uint64][]byte
}

func (f *fakeKV) Get(_ context.Context, req *pbkv.GetRequest) (*pbkv.GetResponse, error) {
	if req.Key == "head" {
		v := make([]byte, 8)
		binary.BigEndian.PutUint64(v, f.head)
		return &pbkv.GetResponse{Value: v}, nil
	}
	var slot uint64
	if _, err := fmt.Sscanf(req.Key, "slot:%d", &slot); err == nil {
		if v, ok := f.slots[slot]; ok {
			return &pbkv.GetResponse{Value: v}, nil
		}
	}
	return nil, status.Error(codes.NotFound, "not found")
}

func rootOf(slot uint64) []byte {
	b := make([]byte, 32)
	binary.BigEndian.PutUint64(b, slot)
	return b
}

func slotBytes(t *testing.T, slot uint64) []byte {
	t.Helper()
	raw, err := proto.Marshal(&pbbl.Slot{Slot: slot, Root: rootOf(slot), Timestamp: timestamppb.New(time.Unix(1_700_000_000+int64(slot)*12, 0))})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func serveKV(t *testing.T, head uint64, slots ...uint64) string {
	t.Helper()
	kv := &fakeKV{head: head, slots: map[uint64][]byte{}}
	for _, s := range slots {
		kv.slots[s] = slotBytes(t, s)
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	pbkv.RegisterKvServer(srv, kv)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

func run(t *testing.T, addr string, st store.Store, from, to uint64) *Report {
	t.Helper()
	report, err := Run(context.Background(), Config{KVAddr: addr, From: from, To: to, Writer: "migration", Workers: 2, Retries: 1}, st, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func requireRoot(t *testing.T, st store.Store, slot uint64) {
	t.Helper()
	got, err := st.GetRoot(context.Background(), store.RootHex(rootOf(slot)))
	if err != nil || got != slot {
		t.Errorf("root of slot %d: got %d err=%v", slot, got, err)
	}
}

func requireHead(t *testing.T, st store.Store, slot uint64) {
	t.Helper()
	head, err := st.GetHead(context.Background())
	if err != nil || head.Slot != slot || head.Writer != "migration" || head.Root != store.RootHex(rootOf(slot)) {
		t.Errorf("head: %+v err=%v, want slot %d", head, err, slot)
	}
}

func TestCopiesRangeAndPublishesHead(t *testing.T) {
	addr := serveKV(t, 103, 100, 101, 103) // 102 is a missed slot
	st := store.NewMemStore()
	r := run(t, addr, st, 100, 103)
	if r.Copied != 3 || r.Missing != 1 || r.Skipped != 0 {
		t.Errorf("report: %+v", r)
	}
	for _, slot := range []uint64{100, 101, 103} {
		requireRoot(t, st, slot)
	}
	requireHead(t, st, 103)
}

// Review finding 4a: a previous run wrote slot 101 but stopped before its
// root. The rerun must not skip it forever; it repairs the root index.
func TestRerunRepairsMissingRoot(t *testing.T) {
	addr := serveKV(t, 102, 100, 101, 102)
	st := store.NewMemStore()
	ctx := context.Background()
	if err := st.PutSlot(ctx, 100, slotBytes(t, 100)); err != nil {
		t.Fatal(err)
	}
	if err := st.PutRoot(ctx, store.RootHex(rootOf(100)), 100); err != nil {
		t.Fatal(err)
	}
	if err := st.PutSlot(ctx, 101, slotBytes(t, 101)); err != nil { // root never written
		t.Fatal(err)
	}

	r := run(t, addr, st, 100, 102)
	if r.Copied != 1 || r.Skipped != 2 || r.Repaired != 1 {
		t.Errorf("report: %+v", r)
	}
	for _, slot := range []uint64{100, 101, 102} {
		requireRoot(t, st, slot)
	}
	requireHead(t, st, 102)
}

// Review finding 4b: everything was copied but publishing the head failed.
// The rerun copies nothing and must still publish the head.
func TestRerunWithEverythingCopiedPublishesHead(t *testing.T) {
	addr := serveKV(t, 102, 100, 101, 102)
	st := store.NewMemStore()
	ctx := context.Background()
	for slot := uint64(100); slot <= 102; slot++ {
		if err := st.PutSlot(ctx, slot, slotBytes(t, slot)); err != nil {
			t.Fatal(err)
		}
		if err := st.PutRoot(ctx, store.RootHex(rootOf(slot)), slot); err != nil {
			t.Fatal(err)
		}
	}

	r := run(t, addr, st, 100, 102)
	if r.Copied != 0 || r.Skipped != 3 || r.Repaired != 0 || r.HighSlot != 102 {
		t.Errorf("report: %+v", r)
	}
	requireHead(t, st, 102)
}

// A root object pointing at the wrong slot is corrected too.
func TestRerunFixesWrongRoot(t *testing.T) {
	addr := serveKV(t, 100, 100)
	st := store.NewMemStore()
	ctx := context.Background()
	if err := st.PutSlot(ctx, 100, slotBytes(t, 100)); err != nil {
		t.Fatal(err)
	}
	if err := st.PutRoot(ctx, store.RootHex(rootOf(100)), 99); err != nil {
		t.Fatal(err)
	}
	r := run(t, addr, st, 100, 100)
	if r.Repaired != 1 {
		t.Errorf("report: %+v", r)
	}
	requireRoot(t, st, 100)
}

// flakyStore fails the first read of every slot and root object, like a reset
// connection or a store busy resharding its index.
type flakyStore struct {
	*store.MemStore
	mu     sync.Mutex
	failed map[string]bool
}

func (f *flakyStore) firstReadFails(key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failed[key] {
		return nil
	}
	f.failed[key] = true
	return errors.New("read tcp: connection reset by peer")
}

func (f *flakyStore) GetSlot(ctx context.Context, slot uint64) ([]byte, error) {
	if err := f.firstReadFails(store.SlotKey(slot)); err != nil {
		return nil, err
	}
	return f.MemStore.GetSlot(ctx, slot)
}

func (f *flakyStore) GetRoot(ctx context.Context, root string) (uint64, error) {
	if err := f.firstReadFails(store.RootKey(root)); err != nil {
		return 0, err
	}
	return f.MemStore.GetRoot(ctx, root)
}

// A transient read error while checking existing slots and roots is retried,
// not fatal: a failed run starts its whole range over.
func TestTransientReadErrorsAreRetried(t *testing.T) {
	addr := serveKV(t, 102, 100, 101, 102)
	mem := store.NewMemStore()
	ctx := context.Background()
	for slot := uint64(100); slot <= 101; slot++ {
		if err := mem.PutSlot(ctx, slot, slotBytes(t, slot)); err != nil {
			t.Fatal(err)
		}
		if err := mem.PutRoot(ctx, store.RootHex(rootOf(slot)), slot); err != nil {
			t.Fatal(err)
		}
	}
	st := &flakyStore{MemStore: mem, failed: map[string]bool{}}

	r, err := Run(ctx, Config{KVAddr: addr, From: 100, To: 102, Writer: "migration", Workers: 2, Retries: 2}, st, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	if r.Copied != 1 || r.Skipped != 2 || r.Repaired != 0 {
		t.Errorf("report: %+v", r)
	}
	for _, slot := range []uint64{100, 101, 102} {
		requireRoot(t, mem, slot)
	}
	requireHead(t, mem, 102)
}
