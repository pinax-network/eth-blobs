// Package sink streams map_blobs from a Substreams endpoint and writes one
// object per beacon block into a store.Store, plus the root index and the
// writer's head and cursor objects.
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
	// CleanupWindow is how many slots above a cut point (an undo's last valid
	// block, or the cursor on startup) get their slot objects deleted, so a
	// slot a reorg emptied does not keep serving a stale block.
	CleanupWindow uint64
	// FlushRetries is how many times a failed batch write is retried before
	// the sink gives up. Writes are idempotent, so a retry is always safe.
	FlushRetries int
}

func (c Config) withDefaults() Config {
	if c.BatchSize <= 0 {
		c.BatchSize = 200
	}
	if c.Parallelism <= 0 {
		c.Parallelism = 16
	}
	if c.CleanupWindow == 0 {
		c.CleanupWindow = 64
	}
	if c.FlushRetries <= 0 {
		c.FlushRetries = 5
	}
	return c
}

type pendingBlock struct {
	slot  uint64
	root  string
	time  time.Time
	data  []byte
	blobs int
}

// Sinker adapts the substreams-sink library to a store.Store.
type Sinker struct {
	*shutter.Shutter
	*sink.Sinker

	store  store.Store
	cfg    Config
	logger *zap.Logger

	pending     []pendingBlock
	lastWritten uint64 // highest slot written by this process
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
	}, nil
}

// Run loads the writer's cursor, clears any leftovers above it, and streams
// until the context ends or the stop block is reached.
func (s *Sinker) Run(ctx context.Context) {
	cursor, err := s.loadCursor(ctx)
	if err != nil {
		s.Shutdown(fmt.Errorf("loading cursor: %w", err))
		return
	}

	if !cursor.IsBlank() {
		// A crash between the slot writes of a flush and its cursor write
		// leaves objects above the cursor. The stream rewrites those slots,
		// except the ones a reorg has since emptied, so clear the window.
		from := cursor.Block().Num()
		if err := s.cleanupAbove(ctx, from, from+s.cfg.CleanupWindow); err != nil {
			s.Shutdown(fmt.Errorf("startup cleanup: %w", err))
			return
		}
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
		zap.Uint64("cleanup_window", s.cfg.CleanupWindow),
	)
	s.Sinker.Run(ctx, cursor, s)
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

	var blockTime time.Time
	if slot.Timestamp != nil {
		blockTime = slot.Timestamp.AsTime()
	}
	s.pending = append(s.pending, pendingBlock{
		slot:  slot.Slot,
		root:  store.RootHex(slot.Root),
		time:  blockTime,
		data:  output.Value,
		blobs: len(slot.Blobs),
	})

	// Live when the liveness checker says so, or when the block is not yet
	// irreversible: both mean we are at the head and must not hold blocks.
	live := isLive != nil && *isLive
	if !live && cursor != nil && cursor.Cursor != nil && cursor.Step != bstream.StepNewIrreversible {
		live = true
	}
	if live || len(s.pending) >= s.cfg.BatchSize {
		return s.flush(ctx, cursor)
	}
	return nil
}

// flush writes every pending slot and root concurrently, then the head and
// finally the cursor, so a reader never sees a head or cursor ahead of the
// data. A failed batch is retried whole: every write is idempotent.
func (s *Sinker) flush(ctx context.Context, cursor *sink.Cursor) error {
	if len(s.pending) == 0 {
		return nil
	}
	start := time.Now()

	var err error
	for attempt := 1; attempt <= s.cfg.FlushRetries; attempt++ {
		if err = s.writeBatch(ctx, s.pending); err == nil {
			break
		}
		if ctx.Err() != nil {
			return err
		}
		delay := time.Duration(attempt*attempt) * time.Second
		s.logger.Warn("batch write failed, retrying", zap.Int("attempt", attempt), zap.Duration("in", delay), zap.Error(err))
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err != nil {
		return fmt.Errorf("writing batch of %d slots after %d attempts: %w", len(s.pending), s.cfg.FlushRetries, err)
	}

	last := s.pending[len(s.pending)-1]
	if err := s.publishHead(ctx, last.slot, last.root, last.time); err != nil {
		return err
	}
	if err := s.store.PutCursor(ctx, s.cfg.Writer, cursor.String()); err != nil {
		return fmt.Errorf("writing cursor: %w", err)
	}

	var blobs, bytes int
	for _, p := range s.pending {
		blobs += p.blobs
		bytes += len(p.data)
	}
	SlotsWritten.AddInt(len(s.pending))
	BlobsWritten.AddInt(blobs)
	BytesWritten.AddInt(bytes)
	FlushCount.Inc()
	LastFlushDuration.SetFloat64(time.Since(start).Seconds())

	s.logger.Debug("flushed",
		zap.Int("slots", len(s.pending)),
		zap.Int("blobs", blobs),
		zap.Int("bytes", bytes),
		zap.Uint64("first_slot", s.pending[0].slot),
		zap.Uint64("last_slot", last.slot),
		zap.Duration("took", time.Since(start)),
	)

	s.lastWritten = last.slot
	s.pending = s.pending[:0]
	return nil
}

func (s *Sinker) writeBatch(ctx context.Context, batch []pendingBlock) error {
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(s.cfg.Parallelism)
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
	if err := s.store.PutHead(ctx, s.cfg.Writer, head); err != nil {
		return fmt.Errorf("writing head: %w", err)
	}
	HeadSlot.SetUint64(slot)
	return nil
}

func (s *Sinker) HandleBlockUndoSignal(ctx context.Context, undo *pbsubstreamsrpc.BlockUndoSignal, cursor *sink.Cursor) error {
	lastValid := undo.LastValidBlock.Number
	UndoCount.Inc()
	s.logger.Info("handling undo signal", zap.Uint64("last_valid_slot", lastValid), zap.Uint64("last_written", s.lastWritten))

	// Pending blocks at or below the last valid block are still good; the
	// rest never reached the store and are simply dropped.
	kept := s.pending[:0]
	for _, p := range s.pending {
		if p.slot <= lastValid {
			kept = append(kept, p)
		}
	}
	s.pending = kept
	if err := s.flush(ctx, cursor); err != nil {
		return err
	}

	upTo := lastValid + s.cfg.CleanupWindow
	if s.lastWritten > upTo {
		upTo = s.lastWritten
	}
	if err := s.cleanupAbove(ctx, lastValid, upTo); err != nil {
		return fmt.Errorf("undo cleanup: %w", err)
	}

	head, err := s.headFor(ctx, lastValid, undo.LastValidBlock.Id)
	if err != nil {
		return err
	}
	if err := s.publishHead(ctx, head.Slot, head.Root, time.Unix(head.Timestamp, 0)); err != nil {
		return err
	}
	if err := s.store.PutCursor(ctx, s.cfg.Writer, cursor.String()); err != nil {
		return fmt.Errorf("writing cursor after undo: %w", err)
	}
	s.lastWritten = lastValid
	return nil
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

// cleanupAbove deletes the slot objects in (from, to]. Root objects of the
// deleted slots are left in place: the API verifies that a slot fetched by
// root really carries that root, so a stale root resolves to not found.
func (s *Sinker) cleanupAbove(ctx context.Context, from, to uint64) error {
	if to <= from {
		return nil
	}
	slots := make([]uint64, 0, to-from)
	for slot := from + 1; slot <= to; slot++ {
		slots = append(slots, slot)
	}
	if err := s.store.DeleteSlots(ctx, slots); err != nil {
		return err
	}
	SlotsDeleted.AddInt(len(slots))
	s.logger.Info("deleted slot objects above cut point", zap.Uint64("above", from), zap.Uint64("up_to", to))
	return nil
}

// HandleBlockRangeCompletion flushes what is still pending when a stop block
// ends the stream.
func (s *Sinker) HandleBlockRangeCompletion(ctx context.Context, cursor *sink.Cursor) error {
	return s.flush(ctx, cursor)
}
