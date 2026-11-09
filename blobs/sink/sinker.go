// Package sink streams map_blobs from a Substreams endpoint and writes one
// object per beacon block into a store.Store, plus the root index and the
// writer's head and cursor objects.
//
// Reorgs. The library's undo buffer is never used (cmd/sink does not register
// --undo-buffer-size): with a buffer the library swallows undo signals, and a
// fork deeper than the buffer kills the stream and is then swallowed on
// restart, so nothing would ever clean up what was already written. Instead
// every block is written as it arrives and the sink keeps the store
// consistent with the stream with two targeted deletes:
//
//   - Missed slots. When the stream delivers block S right after block P, the
//     slots in (P, S) hold no block on the chain the stream follows, so any
//     object there is stale (an orphan from a reorg, or a crash leftover) and is
//     deleted in the same flush as S, before the head and cursor move.
//   - Orphans on undo. Slots this process wrote above the last valid block are
//     deleted only if they still hold the block this process wrote; a slot a
//     faster writer already refilled with the canonical block is left alone.
//     Orphans the new chain also has a block for are overwritten by the stream.
//
// Both deletes follow this writer's own view of the chain, which during a
// fork may differ from another writer's. A block another writer wrote on the
// chain that wins can therefore be deleted for a moment: the deleting writer
// then receives an undo and streams the winning chain from the fork point, so
// it rewrites every block it removed. The cursor never moves past a delete
// that has not happened, so a crash in between replays it the same way. The
// store converges within the fork's resolution; the object store offers no
// conditional delete that could close the window itself.
package sink

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/streamingfast/bstream"
	"github.com/streamingfast/shutter"
	sink "github.com/streamingfast/substreams-sink"
	pbsubstreamsrpc "github.com/streamingfast/substreams/pb/sf/substreams/rpc/v2"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"

	pbbl "github.com/pinax-network/blob-service/blobs/pb/pinax/ethereum/blobs/v1"
	"github.com/pinax-network/blob-service/blobs/store"
)

// OutputModuleType is the Substreams output the sink accepts.
const OutputModuleType = "pinax.ethereum.blobs.v1.Slot"

// OutputModuleName is the module streamed when the caller does not override it.
const OutputModuleName = "map_blobs"

type Config struct {
	// Writer names this sink's head and cursor objects. One per DC.
	Writer string
	// BatchSize is how many blocks are collected before a flush while
	// catching up. Live blocks flush one by one.
	BatchSize int
	// Parallelism bounds concurrent object writes within a flush.
	Parallelism int
	// FlushRetries is how many times each failed store write (the batch, the
	// head, the cursor, an undo's deletes) is attempted before the sink gives
	// up. Writes and deletes are idempotent, so a retry is always safe.
	FlushRetries int
}

func (c Config) withDefaults() Config {
	if c.BatchSize <= 0 {
		c.BatchSize = 200
	}
	if c.Parallelism <= 0 {
		c.Parallelism = 16
	}
	if c.FlushRetries <= 0 {
		c.FlushRetries = 5
	}
	return c
}

type pendingBlock struct {
	slot   uint64
	root   string
	time   time.Time
	data   []byte
	blobs  int
	cursor *sink.Cursor
	// missed are the slots between the previously delivered block and this
	// one: no block exists there on this chain, so their objects are deleted.
	missed []uint64
}

// Sinker adapts the substreams-sink library to a store.Store.
type Sinker struct {
	*shutter.Shutter
	*sink.Sinker

	store  store.Store
	cfg    Config
	logger *zap.Logger

	pending []pendingBlock

	// prev is the last block the stream delivered (or the last valid block
	// of an undo, or the cursor's block on startup); havePrev is false only
	// before the first block of a fresh start.
	prev     uint64
	havePrev bool

	// unfinal maps slot -> root for blocks this process wrote that are not
	// final yet, so an undo can delete exactly its own orphans.
	unfinal     map[uint64]string
	finalHeight uint64

	lastWritten uint64 // highest slot written by this process, for logs and tests
}

func New(s *sink.Sinker, st store.Store, cfg Config, logger *zap.Logger) (*Sinker, error) {
	cfg = cfg.withDefaults()
	if err := store.ValidateWriter(cfg.Writer); err != nil {
		return nil, err
	}
	return &Sinker{
		Shutter: shutter.New(),
		Sinker:  s,
		store:   st,
		cfg:     cfg,
		logger:  logger,
		unfinal: map[uint64]string{},
	}, nil
}

// Run loads the writer's cursor and streams until the context ends or the
// stop block is reached.
func (s *Sinker) Run(ctx context.Context) {
	cursor, err := s.start(ctx)
	if err != nil {
		s.Shutdown(err)
		return
	}

	s.Sinker.OnTerminating(s.Shutdown)
	s.OnTerminating(func(err error) {
		s.logger.Info("blobs sinker terminating", zap.Uint64("last_slot_written", s.lastWritten))
		s.Sinker.Shutdown(err)
	})

	s.logger.Info("starting blobs sink",
		zap.String("writer", s.cfg.Writer),
		zap.Stringer("restarting_at", cursor.Block()),
		zap.Int("batch_size", s.cfg.BatchSize),
		zap.Int("parallelism", s.cfg.Parallelism),
	)
	s.Sinker.Run(ctx, cursor, s)
}

// start loads the cursor and anchors missed-slot tracking at its block. It
// deletes nothing: slots above the cursor may be valid blocks another writer
// wrote, and leftovers of this writer's own crash are either rewritten by the
// stream or removed as missed slots when the stream passes them.
func (s *Sinker) start(ctx context.Context) (*sink.Cursor, error) {
	cursor, err := s.loadCursor(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading cursor: %w", err)
	}
	if !cursor.IsBlank() {
		s.prev, s.havePrev = cursor.Block().Num(), true
	}
	return cursor, nil
}

func (s *Sinker) loadCursor(ctx context.Context) (*sink.Cursor, error) {
	raw, err := s.store.GetCursor(ctx, s.cfg.Writer)
	if errors.Is(err, store.ErrNotFound) {
		s.logger.Info("no cursor found for writer, starting from the configured block range", zap.String("writer", s.cfg.Writer))
		return sink.NewBlankCursor(), nil
	}
	if err != nil {
		return nil, err
	}
	return sink.NewCursor(raw)
}

func (s *Sinker) HandleBlockScopedData(ctx context.Context, data *pbsubstreamsrpc.BlockScopedData, isLive *bool, cursor *sink.Cursor) error {
	output := data.GetOutput().GetMapOutput()
	if output == nil {
		return fmt.Errorf("block %d: no map output", data.GetClock().GetNumber())
	}

	slot := &pbbl.Slot{}
	if err := proto.Unmarshal(output.Value, slot); err != nil {
		return fmt.Errorf("block %d: unmarshal slot: %w", data.Clock.Number, err)
	}
	if slot.Slot != data.Clock.Number {
		// The beacon block number is the slot; anything else is a broken package.
		return fmt.Errorf("block %d carries slot %d", data.Clock.Number, slot.Slot)
	}

	var missed []uint64
	if s.havePrev {
		if slot.Slot <= s.prev {
			return fmt.Errorf("block %d arrived after block %d without an undo signal", slot.Slot, s.prev)
		}
		for m := s.prev + 1; m < slot.Slot; m++ {
			missed = append(missed, m)
		}
	}
	s.prev, s.havePrev = slot.Slot, true
	if data.FinalBlockHeight > s.finalHeight {
		s.finalHeight = data.FinalBlockHeight
	}

	var blockTime time.Time
	if slot.Timestamp != nil {
		blockTime = slot.Timestamp.AsTime()
	}
	s.pending = append(s.pending, pendingBlock{
		slot:   slot.Slot,
		root:   store.RootHex(slot.Root),
		time:   blockTime,
		data:   output.Value,
		blobs:  len(slot.Blobs),
		cursor: cursor,
		missed: missed,
	})

	// Live when the liveness checker says so, or when the block is not yet
	// irreversible: both mean we are at the head and must not hold blocks.
	live := isLive != nil && *isLive
	if !live && cursor != nil && cursor.Cursor != nil && cursor.Step != bstream.StepNewIrreversible {
		live = true
	}
	if live || len(s.pending) >= s.cfg.BatchSize {
		return s.flush(ctx)
	}
	return nil
}

// flush writes every pending slot and root and deletes the missed slots
// concurrently, then the head and finally the cursor of the last pending
// block, so a reader never sees a head or cursor ahead of the data. A failed
// batch is retried whole: every write and delete is idempotent.
func (s *Sinker) flush(ctx context.Context) error {
	if len(s.pending) == 0 {
		return nil
	}
	start := time.Now()

	if err := s.retry(ctx, fmt.Sprintf("writing batch of %d slots", len(s.pending)), func() error {
		return s.writeBatch(ctx, s.pending)
	}); err != nil {
		return err
	}

	last := s.pending[len(s.pending)-1]
	if err := s.publishHead(ctx, last.slot, last.root, last.time); err != nil {
		return err
	}
	if err := s.putCursor(ctx, last.cursor); err != nil {
		return err
	}

	var blobs, bytes, missed int
	for _, p := range s.pending {
		blobs += p.blobs
		bytes += len(p.data)
		missed += len(p.missed)
		if p.slot > s.finalHeight {
			s.unfinal[p.slot] = p.root
		}
	}
	for slot := range s.unfinal {
		if slot <= s.finalHeight {
			delete(s.unfinal, slot)
		}
	}
	SlotsWritten.AddInt(len(s.pending))
	BlobsWritten.AddInt(blobs)
	BytesWritten.AddInt(bytes)
	SlotsDeleted.AddInt(missed)
	FlushCount.Inc()
	LastFlushDuration.SetFloat64(time.Since(start).Seconds())

	s.logger.Debug("flushed",
		zap.Int("slots", len(s.pending)),
		zap.Int("blobs", blobs),
		zap.Int("bytes", bytes),
		zap.Int("missed_slots_deleted", missed),
		zap.Uint64("first_slot", s.pending[0].slot),
		zap.Uint64("last_slot", last.slot),
		zap.Duration("took", time.Since(start)),
	)

	s.lastWritten = last.slot
	clear(s.pending) // drop the payload references, keep the backing array
	s.pending = s.pending[:0]
	return nil
}

// retry runs a store write up to FlushRetries times with quadratic backoff.
// Every store operation the sink performs is idempotent.
func (s *Sinker) retry(ctx context.Context, what string, fn func() error) error {
	var err error
	for attempt := 1; attempt <= s.cfg.FlushRetries; attempt++ {
		if err = fn(); err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return err
		}
		delay := time.Duration(attempt*attempt) * time.Second
		s.logger.Warn("store write failed, retrying", zap.String("op", what), zap.Int("attempt", attempt), zap.Duration("in", delay), zap.Error(err))
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fmt.Errorf("%s after %d attempts: %w", what, s.cfg.FlushRetries, err)
}

func (s *Sinker) putCursor(ctx context.Context, cursor *sink.Cursor) error {
	return s.retry(ctx, "writing cursor", func() error {
		return s.store.PutCursor(ctx, s.cfg.Writer, cursor.String())
	})
}

func (s *Sinker) writeBatch(ctx context.Context, batch []pendingBlock) error {
	var missed []uint64
	for _, p := range batch {
		missed = append(missed, p.missed...)
	}
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(s.cfg.Parallelism)
	if len(missed) > 0 {
		g.Go(func() error { return s.store.DeleteSlots(gctx, missed) })
	}
	for _, p := range batch {
		g.Go(func() error {
			if err := s.store.PutSlot(gctx, p.slot, p.data); err != nil {
				return err
			}
			return s.store.PutRoot(gctx, p.root, p.slot)
		})
	}
	return g.Wait()
}

func (s *Sinker) publishHead(ctx context.Context, slot uint64, root string, blockTime time.Time) error {
	head := store.Head{Slot: slot, Root: root}
	if !blockTime.IsZero() {
		head.Timestamp = blockTime.Unix()
	}
	if err := s.retry(ctx, "writing head", func() error { return s.store.PutHead(ctx, s.cfg.Writer, head) }); err != nil {
		return err
	}
	HeadSlot.SetUint64(slot)
	return nil
}

// HandleBlockUndoSignal rolls this writer back to the last valid block: drop
// pending blocks above it, delete the orphans this process wrote above it,
// republish the head there and save the undo's cursor. Orphans written before
// a restart are not known here; the stream overwrites those that the new chain
// has a block for and deletes the rest as missed slots.
func (s *Sinker) HandleBlockUndoSignal(ctx context.Context, undo *pbsubstreamsrpc.BlockUndoSignal, cursor *sink.Cursor) error {
	lastValid := undo.LastValidBlock.Number
	UndoCount.Inc()
	s.logger.Info("handling undo signal", zap.Uint64("last_valid_slot", lastValid), zap.Uint64("last_written", s.lastWritten))

	// Pending blocks at or below the last valid block are still good; the
	// rest never reached the store and are dropped with their missed slots.
	kept := s.pending[:0]
	for _, p := range s.pending {
		if p.slot <= lastValid {
			kept = append(kept, p)
		}
	}
	clear(s.pending[len(kept):]) // drop the payloads of the blocks never written
	s.pending = kept
	if err := s.flush(ctx); err != nil {
		return err
	}

	orphans, err := s.ownOrphansAbove(ctx, lastValid)
	if err != nil {
		return fmt.Errorf("undo: %w", err)
	}
	if len(orphans) > 0 {
		if err := s.retry(ctx, "undo: deleting orphans", func() error { return s.store.DeleteSlots(ctx, orphans) }); err != nil {
			return err
		}
		SlotsDeleted.AddInt(len(orphans))
		s.logger.Info("deleted orphaned slots", zap.Uint64("above", lastValid), zap.Uint64s("slots", orphans))
	}

	head, err := s.headFor(ctx, lastValid, undo.LastValidBlock.Id)
	if err != nil {
		return err
	}
	if err := s.publishHead(ctx, head.Slot, head.Root, time.Unix(head.Timestamp, 0)); err != nil {
		return err
	}
	if err := s.putCursor(ctx, cursor); err != nil {
		return err
	}
	s.prev, s.havePrev = lastValid, true
	s.lastWritten = lastValid
	return nil
}

// ownOrphansAbove returns the slots above lastValid that this process wrote
// and that still hold the block it wrote. A slot already refilled with another
// block (a faster writer on the new chain) is not ours to delete.
func (s *Sinker) ownOrphansAbove(ctx context.Context, lastValid uint64) ([]uint64, error) {
	var orphans []uint64
	for slot, root := range s.unfinal {
		if slot <= lastValid {
			continue
		}
		delete(s.unfinal, slot)
		data, err := s.store.GetSlot(ctx, slot)
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("reading slot %d: %w", slot, err)
		}
		pb := &pbbl.Slot{}
		if err := proto.Unmarshal(data, pb); err != nil {
			return nil, fmt.Errorf("slot %d: unmarshal: %w", slot, err)
		}
		if store.RootHex(pb.Root) == root {
			orphans = append(orphans, slot)
		}
	}
	return orphans, nil
}

// headFor rebuilds the head for a slot already in the store, falling back to
// what the undo signal carries when the slot predates this writer's data.
func (s *Sinker) headFor(ctx context.Context, slot uint64, id string) (store.Head, error) {
	data, err := s.store.GetSlot(ctx, slot)
	if errors.Is(err, store.ErrNotFound) {
		root := id
		if len(root) >= 2 && root[:2] != "0x" {
			root = "0x" + root
		}
		return store.Head{Slot: slot, Root: root}, nil
	}
	if err != nil {
		return store.Head{}, fmt.Errorf("reading slot %d for head: %w", slot, err)
	}
	pb := &pbbl.Slot{}
	if err := proto.Unmarshal(data, pb); err != nil {
		return store.Head{}, fmt.Errorf("slot %d: unmarshal: %w", slot, err)
	}
	head := store.Head{Slot: pb.Slot, Root: store.RootHex(pb.Root)}
	if pb.Timestamp != nil {
		head.Timestamp = pb.Timestamp.AsTime().Unix()
	}
	return head, nil
}

// HandleBlockRangeCompletion flushes what is still pending when a stop block
// ends the stream. The library's completion cursor is ignored: it is the last
// cursor received, which need not belong to a block that was delivered here.
// flush saves the cursor of the last pending block instead.
func (s *Sinker) HandleBlockRangeCompletion(ctx context.Context, _ *sink.Cursor) error {
	return s.flush(ctx)
}
