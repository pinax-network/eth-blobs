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
		unfinal: map[uint64]string{},
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

// otherBlock is what a different block at the same slot looks like: same slot
// number, another root. It stands in for the canonical block a faster writer
// wrote, or an orphan from a previous process.
func otherBlock(t *testing.T, slot uint64) []byte {
	t.Helper()
	root := rootOf(slot)
	root[31] = 0xff
	raw, err := proto.Marshal(&pbbl.Slot{Slot: slot, Root: root})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func undoTo(t *testing.T, s *Sinker, lastValid uint64) {
	t.Helper()
	_, cursor := blockData(t, lastValid, 0, bstream.StepNew)
	undo := &pbsubstreamsrpc.BlockUndoSignal{LastValidBlock: &pbsubstreams.BlockRef{Id: fmt.Sprintf("%x", rootOf(lastValid)), Number: lastValid}}
	if err := s.HandleBlockUndoSignal(context.Background(), undo, cursor); err != nil {
		t.Fatal(err)
	}
}

func deliver(t *testing.T, s *Sinker, slot uint64) {
	t.Helper()
	data, cursor := blockData(t, slot, 1, bstream.StepNew)
	live := true
	if err := s.HandleBlockScopedData(context.Background(), data, &live, cursor); err != nil {
		t.Fatalf("slot %d: %v", slot, err)
	}
}

func storedCursorBlock(t *testing.T, st *store.MemStore) uint64 {
	t.Helper()
	raw, err := st.GetCursor(context.Background(), "test")
	if err != nil {
		t.Fatal(err)
	}
	c, err := sink.NewCursor(raw)
	if err != nil {
		t.Fatal(err)
	}
	return c.Block().Num()
}

func TestUndoDropsPendingAndRepublishesHead(t *testing.T) {
	s, st := newTestSinker(t, Config{BatchSize: 100})
	feed(t, s, 500, 505, bstream.StepNew, true)              // written: 500..505
	feed(t, s, 506, 508, bstream.StepNewIrreversible, false) // pending, batch not full

	undoTo(t, s, 506)

	if !st.Has(store.SlotKey(506)) {
		t.Error("pending block at the last valid slot must be flushed")
	}
	for _, slot := range []uint64{507, 508} {
		if st.Has(store.SlotKey(slot)) {
			t.Errorf("pending block %d above last valid must not be written", slot)
		}
	}
	head, err := st.GetHead(context.Background())
	if err != nil || head.Slot != 506 || head.Root != store.RootHex(rootOf(506)) {
		t.Errorf("head after undo: %+v err=%v", head, err)
	}
	if got := storedCursorBlock(t, st); got != 506 {
		t.Errorf("cursor after undo at %d, want 506", got)
	}
	if !s.havePrev || s.prev != 506 || len(s.pending) != 0 {
		t.Errorf("prev=%d havePrev=%v pending=%d after undo", s.prev, s.havePrev, len(s.pending))
	}
}

func TestUndoDeletesOwnOrphans(t *testing.T) {
	s, st := newTestSinker(t, Config{})
	feed(t, s, 600, 620, bstream.StepNew, true) // 600..620 written live, none final

	undoTo(t, s, 610)
	for slot := uint64(611); slot <= 620; slot++ {
		if st.Has(store.SlotKey(slot)) {
			t.Errorf("orphan %d above last valid still present after undo", slot)
		}
	}
	for slot := uint64(600); slot <= 610; slot++ {
		if !st.Has(store.SlotKey(slot)) {
			t.Errorf("slot %d at or below last valid was deleted", slot)
		}
	}
}

// Another writer sharing the bucket may already have written the new chain's
// block at a slot this writer orphaned. The undo must leave that block alone.
func TestUndoKeepsBlocksAnotherWriterRewrote(t *testing.T) {
	s, st := newTestSinker(t, Config{})
	feed(t, s, 700, 705, bstream.StepNew, true)
	canonical := otherBlock(t, 704)
	if err := st.PutSlot(context.Background(), 704, canonical); err != nil {
		t.Fatal(err)
	}

	undoTo(t, s, 702)
	for _, slot := range []uint64{703, 705} {
		if st.Has(store.SlotKey(slot)) {
			t.Errorf("own orphan %d must be deleted", slot)
		}
	}
	got, err := st.GetSlot(context.Background(), 704)
	if err != nil || string(got) != string(canonical) {
		t.Errorf("slot 704 rewritten by another writer must survive the undo: err=%v", err)
	}
}

// Final blocks cannot be orphaned, so they are not tracked for undo.
func TestFinalBlocksAreNotTrackedForUndo(t *testing.T) {
	s, _ := newTestSinker(t, Config{BatchSize: 1})
	for slot := uint64(800); slot <= 803; slot++ {
		data, cursor := blockData(t, slot, 0, bstream.StepNewIrreversible)
		data.FinalBlockHeight = 802
		if err := s.HandleBlockScopedData(context.Background(), data, nil, cursor); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := s.unfinal[802]; ok || len(s.unfinal) != 1 {
		t.Errorf("only slot 803 (above final height 802) should be tracked, got %v", s.unfinal)
	}
}

func TestMissedSlotsAreDeleted(t *testing.T) {
	s, st := newTestSinker(t, Config{})
	ctx := context.Background()
	// Stale objects at 101 and 102, e.g. orphans from an earlier fork.
	for _, slot := range []uint64{101, 102} {
		if err := st.PutSlot(ctx, slot, otherBlock(t, slot)); err != nil {
			t.Fatal(err)
		}
	}
	deliver(t, s, 100)
	deliver(t, s, 103) // the chain has no block at 101 or 102

	for _, slot := range []uint64{101, 102} {
		if st.Has(store.SlotKey(slot)) {
			t.Errorf("slot %d has no block on the chain; its object must be deleted", slot)
		}
	}
	for _, slot := range []uint64{100, 103} {
		if !st.Has(store.SlotKey(slot)) {
			t.Errorf("slot %d must be written", slot)
		}
	}
}

func TestOutOfOrderBlockIsFatal(t *testing.T) {
	s, _ := newTestSinker(t, Config{})
	deliver(t, s, 100)
	data, cursor := blockData(t, 100, 0, bstream.StepNew)
	if err := s.HandleBlockScopedData(context.Background(), data, nil, cursor); err == nil {
		t.Fatal("a block at or below the previous one without an undo must fail")
	}
}

// Review finding 2: restarting one writer must not delete slots another
// writer wrote above its cursor.
func TestRestartDoesNotDeleteOtherWritersSlots(t *testing.T) {
	s, st := newTestSinker(t, Config{})
	ctx := context.Background()
	_, cursor := blockData(t, 100, 0, bstream.StepNew)
	if err := st.PutCursor(ctx, "test", cursor.String()); err != nil {
		t.Fatal(err)
	}
	other := map[uint64][]byte{}
	for slot := uint64(101); slot <= 110; slot++ {
		data, _ := blockData(t, slot, 1, bstream.StepNew)
		other[slot] = data.Output.MapOutput.Value
		if err := st.PutSlot(ctx, slot, other[slot]); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := s.start(ctx); err != nil {
		t.Fatal(err)
	}
	for slot := uint64(101); slot <= 110; slot++ {
		if !st.Has(store.SlotKey(slot)) {
			t.Fatalf("startup deleted slot %d written by another writer", slot)
		}
	}
	if !s.havePrev || s.prev != 100 {
		t.Fatalf("missed-slot tracking must start at the cursor block, got prev=%d havePrev=%v", s.prev, s.havePrev)
	}
	// Catching up rewrites the same blocks and deletes nothing.
	feed(t, s, 101, 110, bstream.StepNew, true)
	for slot := uint64(101); slot <= 110; slot++ {
		if !st.Has(store.SlotKey(slot)) {
			t.Errorf("slot %d missing after catch-up", slot)
		}
	}
}

// Review finding 1: after a deep reorg the process restarts with its cursor on
// the orphaned fork. The undo the server sends first reaches the handler (no
// library buffer), and the new chain's blocks remove every orphan: one is
// overwritten, the other sits in a slot the new chain skips and is deleted.
func TestRestartOnOrphanedForkCleansOrphans(t *testing.T) {
	s, st := newTestSinker(t, Config{})
	ctx := context.Background()
	for _, slot := range []uint64{101, 102} { // orphans from the previous process
		if err := st.PutSlot(ctx, slot, otherBlock(t, slot)); err != nil {
			t.Fatal(err)
		}
	}
	_, cursor := blockData(t, 102, 0, bstream.StepNew)
	if err := st.PutCursor(ctx, "test", cursor.String()); err != nil {
		t.Fatal(err)
	}

	if _, err := s.start(ctx); err != nil {
		t.Fatal(err)
	}
	undoTo(t, s, 100) // nothing tracked in this process yet: no eager deletes
	deliver(t, s, 101)
	deliver(t, s, 103) // the new chain has no block at 102

	got, err := st.GetSlot(ctx, 101)
	if err != nil || string(got) == string(otherBlock(t, 101)) {
		t.Errorf("orphan 101 must be overwritten by the new chain's block (err=%v)", err)
	}
	if st.Has(store.SlotKey(102)) {
		t.Error("orphan 102 must be deleted: the new chain skips slot 102")
	}
}

// Review finding 3: the cursor saved at range completion is the last pending
// block's, never the library's completion cursor.
func TestRangeCompletionSavesLastPendingCursor(t *testing.T) {
	s, st := newTestSinker(t, Config{BatchSize: 100})
	feed(t, s, 100, 101, bstream.StepNewIrreversible, false)
	_, beyond := blockData(t, 105, 0, bstream.StepNewIrreversible)
	if err := s.HandleBlockRangeCompletion(context.Background(), beyond); err != nil {
		t.Fatal(err)
	}
	if !st.Has(store.SlotKey(101)) {
		t.Fatal("pending blocks must be flushed at range completion")
	}
	if got := storedCursorBlock(t, st); got != 101 {
		t.Fatalf("cursor saved at block %d, want 101 (the last block written)", got)
	}
}

// A transient failure on the head or cursor write is retried like the batch,
// in order: batch, then head, then cursor.
func TestHeadAndCursorWritesAreRetried(t *testing.T) {
	s, st := newTestSinker(t, Config{FlushRetries: 3})
	st.FailTimes[store.HeadKey("test")] = 1
	st.FailTimes[store.CursorKey("test")] = 1
	feed(t, s, 900, 900, bstream.StepNew, true)
	if st.CallsFor(store.HeadKey("test")) != 2 || st.CallsFor(store.CursorKey("test")) != 2 {
		t.Errorf("head/cursor attempts: %d/%d, want 2/2", st.CallsFor(store.HeadKey("test")), st.CallsFor(store.CursorKey("test")))
	}
	head, err := st.GetHead(context.Background())
	if err != nil || head.Slot != 900 {
		t.Fatalf("head after retried write: %+v err=%v", head, err)
	}
	if !st.Has(store.CursorKey("test")) {
		t.Fatal("cursor must be written after its retry")
	}
	if s.lastWritten != 900 {
		t.Errorf("lastWritten = %d", s.lastWritten)
	}
}

// Flushed blocks must not stay reachable through the reused pending slice.
func TestFlushedPayloadsAreReleased(t *testing.T) {
	s, _ := newTestSinker(t, Config{BatchSize: 5})
	feed(t, s, 100, 104, bstream.StepNewIrreversible, false)
	if len(s.pending) != 0 || cap(s.pending) < 5 {
		t.Fatalf("pending len=%d cap=%d after the flush", len(s.pending), cap(s.pending))
	}
	for i, p := range s.pending[:cap(s.pending)] {
		if p.data != nil || p.cursor != nil {
			t.Fatalf("flushed block %d still referenced from the backing array", i)
		}
	}
}
