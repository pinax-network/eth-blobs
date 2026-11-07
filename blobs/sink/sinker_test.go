package sink

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/streamingfast/bstream"
	"github.com/streamingfast/shutter"
	sink "github.com/streamingfast/substreams-sink"
	pbsubstreamsrpc "github.com/streamingfast/substreams/pb/sf/substreams/rpc/v2"
	pbsubstreams "github.com/streamingfast/substreams/pb/sf/substreams/v1"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pbbl "github.com/pinax-network/blob-service/blobs/pb/pinax/ethereum/blobs/v1"
	"github.com/pinax-network/blob-service/blobs/store"
)

func newTestSinker(t *testing.T, cfg Config) (*Sinker, *store.MemStore) {
	t.Helper()
	cfg.Writer = "test"
	st := store.NewMemStore()
	return &Sinker{
		Shutter: shutter.New(),
		store:   st,
		cfg:     cfg.withDefaults(),
		logger:  zap.NewNop(),
	}, st
}

func rootOf(slot uint64) []byte {
	b := make([]byte, 32)
	b[0] = byte(slot >> 8)
	b[1] = byte(slot)
	return b
}

func blockData(t *testing.T, slot uint64, blobs int, step bstream.StepType) (*pbsubstreamsrpc.BlockScopedData, *sink.Cursor) {
	t.Helper()
	pb := &pbbl.Slot{Slot: slot, Root: rootOf(slot), Timestamp: timestamppb.New(time.Unix(1_700_000_000+int64(slot)*12, 0))}
	for i := 0; i < blobs; i++ {
		pb.Blobs = append(pb.Blobs, &pbbl.Blob{Index: uint32(i), Blob: []byte{byte(i)}})
	}
	raw, err := proto.Marshal(pb)
	if err != nil {
		t.Fatal(err)
	}
	ref := bstream.NewBlockRef(fmt.Sprintf("%x", rootOf(slot)), slot)
	cursor := &sink.Cursor{Cursor: &bstream.Cursor{Step: step, Block: ref, LIB: ref, HeadBlock: ref}}
	data := &pbsubstreamsrpc.BlockScopedData{
		Output: &pbsubstreamsrpc.MapModuleOutput{Name: OutputModuleName, MapOutput: &anypb.Any{Value: raw}},
		Clock:  &pbsubstreams.Clock{Id: ref.ID(), Number: slot, Timestamp: pb.Timestamp},
		Cursor: cursor.String(),
	}
	return data, cursor
}

func feed(t *testing.T, s *Sinker, from, to uint64, step bstream.StepType, live bool) {
	t.Helper()
	for slot := from; slot <= to; slot++ {
		data, cursor := blockData(t, slot, int(slot%3), step)
		isLive := live
		if err := s.HandleBlockScopedData(context.Background(), data, &isLive, cursor); err != nil {
			t.Fatalf("slot %d: %v", slot, err)
		}
	}
}

func TestCatchUpBatchesUntilBatchSize(t *testing.T) {
	s, st := newTestSinker(t, Config{BatchSize: 5})
	feed(t, s, 100, 103, bstream.StepNewIrreversible, false)
	if len(st.Keys()) != 0 {
		t.Fatalf("nothing should be written before the batch fills, got %v", st.Keys())
	}
	feed(t, s, 104, 104, bstream.StepNewIrreversible, false)

	for slot := uint64(100); slot <= 104; slot++ {
		if !st.Has(store.SlotKey(slot)) {
			t.Errorf("slot %d not written", slot)
		}
		if !st.Has(store.RootKey(store.RootHex(rootOf(slot)))) {
			t.Errorf("root for slot %d not written", slot)
		}
	}
	head, err := st.GetHead(context.Background())
	if err != nil || head.Slot != 104 || head.Root != store.RootHex(rootOf(104)) || head.Timestamp != 1_700_000_000+104*12 {
		t.Errorf("head after flush: %+v err=%v", head, err)
	}
	if cur, err := st.GetCursor(context.Background(), "test"); err != nil || cur == "" {
		t.Errorf("cursor not written: %q err=%v", cur, err)
	}
	if len(s.pending) != 0 || s.lastWritten != 104 {
		t.Errorf("pending=%d lastWritten=%d after flush", len(s.pending), s.lastWritten)
	}
}

func TestLiveBlocksFlushOneByOne(t *testing.T) {
	s, st := newTestSinker(t, Config{BatchSize: 100})
	feed(t, s, 200, 200, bstream.StepNew, true)
	if !st.Has(store.SlotKey(200)) {
		t.Fatal("live block must be written immediately")
	}
	// Non-irreversible step alone also counts as live, even without the checker.
	data, cursor := blockData(t, 201, 1, bstream.StepNew)
	if err := s.HandleBlockScopedData(context.Background(), data, nil, cursor); err != nil {
		t.Fatal(err)
	}
	if !st.Has(store.SlotKey(201)) {
		t.Fatal("reversible block must be written immediately")
	}
}

func TestWritesSlotsWithoutBlobs(t *testing.T) {
	s, st := newTestSinker(t, Config{})
	data, cursor := blockData(t, 300, 0, bstream.StepNew)
	if err := s.HandleBlockScopedData(context.Background(), data, nil, cursor); err != nil {
		t.Fatal(err)
	}
	if !st.Has(store.SlotKey(300)) {
		t.Fatal("a block with no blobs still gets a slot object, so the API answers 200 with an empty list")
	}
}

func TestSlotMismatchIsFatal(t *testing.T) {
	s, _ := newTestSinker(t, Config{})
	data, cursor := blockData(t, 400, 0, bstream.StepNew)
	data.Clock.Number = 401
	if err := s.HandleBlockScopedData(context.Background(), data, nil, cursor); err == nil {
		t.Fatal("a slot that disagrees with the block number must fail")
	}
}

func TestUndoDropsPendingDeletesAboveAndRepublishesHead(t *testing.T) {
	s, st := newTestSinker(t, Config{BatchSize: 100, CleanupWindow: 4})
	feed(t, s, 500, 505, bstream.StepNew, true) // written: 500..505
	feed(t, s, 506, 508, bstream.StepNewIrreversible, false)
	// 506..508 pending; the batch has not filled. Make 507 and 508 invalid.
	s.cfg.BatchSize = 100

	_, cursor := blockData(t, 506, 0, bstream.StepNew)
	undo := &pbsubstreamsrpc.BlockUndoSignal{LastValidBlock: &pbsubstreams.BlockRef{Id: "abc", Number: 506}}
	if err := s.HandleBlockUndoSignal(context.Background(), undo, cursor); err != nil {
		t.Fatal(err)
	}

	if !st.Has(store.SlotKey(506)) {
		t.Error("pending block at the last valid slot must be flushed")
	}
	for _, slot := range []uint64{507, 508} {
		if st.Has(store.SlotKey(slot)) {
			t.Errorf("pending block %d above last valid must not be written", slot)
		}
	}
	// 507..510 (window 4 above 506) are deleted; 505 and below stay.
	if !st.Has(store.SlotKey(505)) {
		t.Error("slot 505 below last valid must survive")
	}
	head, err := st.GetHead(context.Background())
	if err != nil || head.Slot != 506 || head.Root != store.RootHex(rootOf(506)) {
		t.Errorf("head after undo: %+v err=%v", head, err)
	}
	if s.lastWritten != 506 || len(s.pending) != 0 {
		t.Errorf("lastWritten=%d pending=%d after undo", s.lastWritten, len(s.pending))
	}
}

func TestUndoDeletesEverythingWrittenAboveLastValid(t *testing.T) {
	s, st := newTestSinker(t, Config{CleanupWindow: 2})
	feed(t, s, 600, 620, bstream.StepNew, true) // 600..620 written live

	_, cursor := blockData(t, 610, 0, bstream.StepNew)
	undo := &pbsubstreamsrpc.BlockUndoSignal{LastValidBlock: &pbsubstreams.BlockRef{Id: "abc", Number: 610}}
	if err := s.HandleBlockUndoSignal(context.Background(), undo, cursor); err != nil {
		t.Fatal(err)
	}
	for slot := uint64(611); slot <= 620; slot++ {
		if st.Has(store.SlotKey(slot)) {
			t.Errorf("slot %d above last valid still present after undo", slot)
		}
	}
	for slot := uint64(600); slot <= 610; slot++ {
		if !st.Has(store.SlotKey(slot)) {
			t.Errorf("slot %d at or below last valid was deleted", slot)
		}
	}
}

func TestUndoBelowWrittenRangeUsesSignalRoot(t *testing.T) {
	s, st := newTestSinker(t, Config{CleanupWindow: 2})
	feed(t, s, 700, 701, bstream.StepNew, true)
	_, cursor := blockData(t, 650, 0, bstream.StepNew)
	undo := &pbsubstreamsrpc.BlockUndoSignal{LastValidBlock: &pbsubstreams.BlockRef{Id: "ab" + "00000000000000000000000000000000000000000000000000000000000000", Number: 650}}
	if err := s.HandleBlockUndoSignal(context.Background(), undo, cursor); err != nil {
		t.Fatal(err)
	}
	head, err := st.GetHead(context.Background())
	if err != nil || head.Slot != 650 || head.Root != "0xab00000000000000000000000000000000000000000000000000000000000000" {
		t.Errorf("head after undo below our data: %+v err=%v", head, err)
	}
}

func TestFlushRetriesThenFails(t *testing.T) {
	s, st := newTestSinker(t, Config{FlushRetries: 2})
	st.FailKey[store.SlotKey(800)] = errors.New("boom")
	data, cursor := blockData(t, 800, 0, bstream.StepNew)
	start := time.Now()
	err := s.HandleBlockScopedData(context.Background(), data, nil, cursor)
	if err == nil {
		t.Fatal("expected the flush to fail after retries")
	}
	if st.CallsFor(store.SlotKey(800)) != 2 {
		t.Errorf("expected 2 attempts, got %d", st.CallsFor(store.SlotKey(800)))
	}
	if st.Has(store.CursorKey("test")) {
		t.Error("cursor must not advance when the batch failed")
	}
	if time.Since(start) < time.Second {
		t.Error("expected a backoff between attempts")
	}
}

func TestCleanupAbove(t *testing.T) {
	s, st := newTestSinker(t, Config{})
	feed(t, s, 900, 910, bstream.StepNew, true)
	if err := s.cleanupAbove(context.Background(), 905, 908); err != nil {
		t.Fatal(err)
	}
	for slot := uint64(900); slot <= 910; slot++ {
		want := slot <= 905 || slot > 908
		if st.Has(store.SlotKey(slot)) != want {
			t.Errorf("slot %d present=%v want=%v", slot, !want, want)
		}
	}
	if err := s.cleanupAbove(context.Background(), 10, 10); err != nil {
		t.Fatal("empty window must be a no-op")
	}
}
