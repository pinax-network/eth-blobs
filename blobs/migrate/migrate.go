// Package migrate copies slots out of a running substreams-sink-kv serve
// endpoint into a store.Store. The KV value of `slot:<n>` is the serialized
// Slot proto, which is exactly the slot object the sink writes, so the copy is
// byte for byte and needs no transformation.
package migrate

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	pbkv "github.com/streamingfast/substreams-sink-kv/pb/substreams/sink/kv/v1"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	pbbl "github.com/pinax-network/blob-service/blobs/pb/pinax/ethereum/blobs/v1"
	"github.com/pinax-network/blob-service/blobs/store"
)

// DefaultRetries is how many times a KV read or store operation is attempted.
// Any failure that outlasts it aborts the run, which then starts its range
// over, so it is sized to ride out an object-store stall of several minutes
// (an S3 gateway resharding a large bucket's index blocks its writes): with the
// quadratic backoff and a 30s operation timeout, 8 attempts span ~5 minutes.
const DefaultRetries = 8

type Config struct {
	// KVAddr is the substreams-sink-kv serve gRPC address.
	KVAddr string
	// From and To bound the slot range to copy, inclusive. To == 0 means the
	// KV store's head.
	From, To uint64
	Workers  int
	// Writer, when set, publishes head/<writer> for the highest copied slot
	// so the API can serve before the live sink takes over.
	Writer string
	// Overwrite rewrites slots that already exist in the store. By default
	// existing slots are not copied again (their root index is still checked
	// and repaired) so an interrupted run can simply be rerun.
	Overwrite bool
	Retries   int
}

type Report struct {
	Copied  uint64
	Skipped uint64
	// Repaired counts skipped slots whose root index was missing or wrong
	// (a previous run stopped between the slot write and the root write).
	Repaired  uint64
	Missing   uint64
	Bytes     uint64
	Blobs     uint64
	HighSlot  uint64
	HighRoot  string
	HighTime  int64
	StartedAt time.Time
	Duration  time.Duration
}

// Run copies the configured slot range. It is safe to rerun on the same
// range: the writes are idempotent, existing slots are not copied again but
// have their root index repaired, and they count towards the published head.
func Run(ctx context.Context, cfg Config, st store.Store, logger *zap.Logger) (*Report, error) {
	if cfg.Workers <= 0 {
		cfg.Workers = 16
	}
	if cfg.Retries <= 0 {
		cfg.Retries = DefaultRetries
	}

	conn, err := grpc.NewClient(cfg.KVAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024*1024*1024)),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to kv %s: %w", cfg.KVAddr, err)
	}
	defer func() { _ = conn.Close() }()
	kv := pbkv.NewKvClient(conn)

	kvHead, err := kvHead(ctx, kv)
	if err != nil {
		return nil, err
	}
	to := cfg.To
	if to == 0 || to > kvHead {
		to = kvHead
	}
	if cfg.From > to {
		return nil, fmt.Errorf("from slot %d is above the end slot %d", cfg.From, to)
	}

	report := &Report{StartedAt: time.Now()}
	var (
		copied, skipped, repaired, missing, bytes, blobs atomic.Uint64
		mu                                               sync.Mutex
	)
	noteHigh := func(slot uint64, root string, ts int64) {
		mu.Lock()
		defer mu.Unlock()
		if slot >= report.HighSlot {
			report.HighSlot, report.HighRoot, report.HighTime = slot, root, ts
		}
	}

	logger.Info("starting kv migration",
		zap.String("kv", cfg.KVAddr),
		zap.Uint64("from", cfg.From), zap.Uint64("to", to), zap.Uint64("kv_head", kvHead),
		zap.Int("workers", cfg.Workers), zap.Bool("overwrite", cfg.Overwrite),
	)

	g, gctx := errgroup.WithContext(ctx)
	slots := make(chan uint64)
	g.Go(func() error {
		defer close(slots)
		for slot := cfg.From; slot <= to; slot++ {
			select {
			case slots <- slot:
			case <-gctx.Done():
				return gctx.Err()
			}
		}
		return nil
	})

	progress := time.NewTicker(10 * time.Second)
	defer progress.Stop()
	go func() {
		for {
			select {
			case <-progress.C:
				done := copied.Load() + skipped.Load() + missing.Load()
				elapsed := time.Since(report.StartedAt).Seconds()
				logger.Info("migration progress",
					zap.Uint64("done", done), zap.Uint64("total", to-cfg.From+1),
					zap.Uint64("copied", copied.Load()), zap.Uint64("skipped", skipped.Load()), zap.Uint64("missing", missing.Load()),
					zap.Float64("slots_per_sec", float64(done)/elapsed),
					zap.Float64("mb_per_sec", float64(bytes.Load())/elapsed/1e6),
				)
			case <-gctx.Done():
				return
			}
		}
	}()

	for i := 0; i < cfg.Workers; i++ {
		g.Go(func() error {
			for slot := range slots {
				if !cfg.Overwrite {
					existing, exists, err := getSlotWithRetry(gctx, st, slot, cfg.Retries, logger)
					if err != nil {
						return fmt.Errorf("checking slot %d: %w", slot, err)
					}
					if exists {
						fixed, err := ensureIndexed(gctx, st, slot, existing, cfg.Retries, logger, noteHigh)
						if err != nil {
							return err
						}
						if fixed {
							repaired.Add(1)
						}
						skipped.Add(1)
						continue
					}
				}
				value, found, err := getWithRetry(gctx, kv, fmt.Sprintf("slot:%d", slot), cfg.Retries, logger)
				if err != nil {
					return err
				}
				if !found {
					missing.Add(1)
					continue
				}
				pb := &pbbl.Slot{}
				if err := proto.Unmarshal(value, pb); err != nil {
					return fmt.Errorf("slot %d: unmarshal: %w", slot, err)
				}
				if pb.Slot != slot {
					return fmt.Errorf("kv slot:%d carries slot %d", slot, pb.Slot)
				}
				root := store.RootHex(pb.Root)
				if err := retry(gctx, cfg.Retries, logger, func() error {
					if err := st.PutSlot(gctx, slot, value); err != nil {
						return err
					}
					return st.PutRoot(gctx, root, slot)
				}); err != nil {
					return fmt.Errorf("writing slot %d: %w", slot, err)
				}
				var ts int64
				if pb.Timestamp != nil {
					ts = pb.Timestamp.AsTime().Unix()
				}
				noteHigh(slot, root, ts)
				copied.Add(1)
				bytes.Add(uint64(len(value)))
				blobs.Add(uint64(len(pb.Blobs)))
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	report.Copied, report.Skipped, report.Repaired, report.Missing = copied.Load(), skipped.Load(), repaired.Load(), missing.Load()
	report.Bytes, report.Blobs = bytes.Load(), blobs.Load()
	report.Duration = time.Since(report.StartedAt)

	if cfg.Writer != "" && report.HighSlot > 0 {
		head := store.Head{Slot: report.HighSlot, Root: report.HighRoot, Timestamp: report.HighTime}
		if err := st.PutHead(ctx, cfg.Writer, head); err != nil {
			return report, fmt.Errorf("publishing head: %w", err)
		}
		logger.Info("published head", zap.String("writer", cfg.Writer), zap.Uint64("slot", head.Slot))
	}
	return report, nil
}

// ensureIndexed checks a slot already in the store: its root object must point
// back at it, which a run interrupted between the slot write and the root
// write would have left missing. It reports whether the root was rewritten and
// records the slot for the head either way.
func ensureIndexed(ctx context.Context, st store.Store, slot uint64, data []byte, retries int, logger *zap.Logger, noteHigh func(uint64, string, int64)) (bool, error) {
	pb := &pbbl.Slot{}
	if err := proto.Unmarshal(data, pb); err != nil {
		return false, fmt.Errorf("existing slot %d: unmarshal: %w", slot, err)
	}
	if pb.Slot != slot {
		return false, fmt.Errorf("existing slot object %d carries slot %d", slot, pb.Slot)
	}
	root := store.RootHex(pb.Root)
	var indexed uint64
	found := true
	if err := retry(ctx, retries, logger, func() error {
		var err error
		indexed, err = st.GetRoot(ctx, root)
		if errors.Is(err, store.ErrNotFound) {
			found = false
			return nil
		}
		return err
	}); err != nil {
		return false, fmt.Errorf("checking root of slot %d: %w", slot, err)
	}
	fixed := false
	if !found || indexed != slot {
		if err := retry(ctx, retries, logger, func() error { return st.PutRoot(ctx, root, slot) }); err != nil {
			return false, fmt.Errorf("repairing root of slot %d: %w", slot, err)
		}
		fixed = true
	}
	var ts int64
	if pb.Timestamp != nil {
		ts = pb.Timestamp.AsTime().Unix()
	}
	noteHigh(slot, root, ts)
	return fixed, nil
}

func kvHead(ctx context.Context, kv pbkv.KvClient) (uint64, error) {
	resp, err := kv.Get(ctx, &pbkv.GetRequest{Key: "head"})
	if err != nil {
		return 0, fmt.Errorf("reading kv head: %w", err)
	}
	v := resp.GetValue()
	if len(v) != 8 {
		return 0, fmt.Errorf("kv head: unexpected %d-byte value", len(v))
	}
	var head uint64
	for _, b := range v {
		head = head<<8 | uint64(b)
	}
	return head, nil
}

// getSlotWithRetry reads a slot already in the store. A missing slot is an
// answer, not a failure; anything else (a reset connection, a timeout while the
// backend is busy) is retried, since giving up restarts the whole range.
func getSlotWithRetry(ctx context.Context, st store.Store, slot uint64, retries int, logger *zap.Logger) (data []byte, found bool, err error) {
	err = retry(ctx, retries, logger, func() error {
		var err error
		data, err = st.GetSlot(ctx, slot)
		if errors.Is(err, store.ErrNotFound) {
			found = false
			return nil
		}
		found = err == nil
		return err
	})
	return data, found, err
}

func getWithRetry(ctx context.Context, kv pbkv.KvClient, key string, retries int, logger *zap.Logger) (value []byte, found bool, err error) {
	err = retry(ctx, retries, logger, func() error {
		resp, err := kv.Get(ctx, &pbkv.GetRequest{Key: key})
		if err != nil {
			if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
				found = false
				return nil
			}
			return err
		}
		value, found = resp.GetValue(), true
		return nil
	})
	if err != nil {
		return nil, false, fmt.Errorf("kv get %s: %w", key, err)
	}
	return value, found, nil
}

func retry(ctx context.Context, attempts int, logger *zap.Logger, fn func() error) error {
	var err error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err = fn(); err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return err
		}
		delay := time.Duration(attempt*attempt) * 500 * time.Millisecond
		logger.Warn("operation failed, retrying", zap.Int("attempt", attempt), zap.Duration("in", delay), zap.Error(err))
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}
