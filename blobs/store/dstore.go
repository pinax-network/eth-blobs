package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/streamingfast/dstore"
	"golang.org/x/sync/errgroup"
)

const (
	// DefaultTimeout bounds a single store operation.
	DefaultTimeout = 30 * time.Second

	// deleteParallelism bounds concurrent deletes: dstore deletes one object per call.
	deleteParallelism = 16

	// tmpSuffix marks the local backend's in-flight writes (written next to the
	// target, then renamed over it). They must never be read as heads.
	tmpSuffix = ".tmp"
)

// DStore implements Store on any github.com/streamingfast/dstore backend.
type DStore struct {
	store   dstore.Store
	timeout time.Duration
}

var _ Store = (*DStore)(nil)

// Open connects to the store at storeURL, which names a bucket or directory and
// an optional prefix inside it:
//
//	s3://host:port/bucket[/prefix]?region=none&insecure=true   S3-compatible endpoint (Ceph RGW, MinIO, R2)
//	s3://bucket[/prefix]?region=us-east-1                       AWS S3
//	gs://bucket[/prefix]                                        Google Cloud Storage
//	az://account.container[/prefix]                             Azure Blob Storage
//	file:///var/lib/blobs                                       local directory (created if missing)
//
// dstore's memory:// backend cannot list, so it cannot serve heads; tests use MemStore.
// Credentials come from each backend's standard environment (AWS_ACCESS_KEY_ID /
// AWS_SECRET_ACCESS_KEY, GOOGLE_APPLICATION_CREDENTIALS, ...), so they stay out of
// flags and config files. timeout bounds each operation; zero means DefaultTimeout.
func Open(storeURL string, timeout time.Duration) (*DStore, error) {
	if storeURL == "" {
		return nil, errors.New("store url is required")
	}
	if strings.HasPrefix(storeURL, "memory:") {
		return nil, errors.New("memory:// stores cannot list heads; use store.NewMemStore in tests")
	}
	if strings.HasPrefix(storeURL, "s3:") {
		s3ChecksumDefaults()
	}
	// dstore wants a directory URL without the trailing slash.
	for strings.HasSuffix(storeURL, "/") && !strings.HasSuffix(storeURL, "://") {
		storeURL = strings.TrimSuffix(storeURL, "/")
	}
	// No extension, no compression (blobs do not compress), overwrite on: heads
	// and cursors are rewritten on every flush, slots after a reorg.
	st, err := dstore.NewStore(storeURL, "", "", true)
	if err != nil {
		return nil, fmt.Errorf("open store %s: %w", RedactURL(storeURL), err)
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &DStore{store: st, timeout: timeout}, nil
}

// s3ChecksumDefaults makes the AWS SDK compute and validate checksums only when
// an operation requires them, unless the operator chose otherwise. Ceph RGW and
// MinIO do not return the checksums the SDK adds by default, and the SDK then
// warns on every GET; older S3-compatible servers also reject the trailing
// checksums it would send on PUT. dstore builds its client from the default AWS
// config, which reads these variables.
func s3ChecksumDefaults() {
	for _, k := range []string{"AWS_REQUEST_CHECKSUM_CALCULATION", "AWS_RESPONSE_CHECKSUM_VALIDATION"} {
		if _, set := os.LookupEnv(k); !set {
			_ = os.Setenv(k, "when_required")
		}
	}
}

// RedactURL hides credentials a store URL may carry in its query string, for logging.
func RedactURL(storeURL string) string {
	u, err := url.Parse(storeURL)
	if err != nil {
		return "<unparseable store url>"
	}
	q := u.Query()
	for _, k := range []string{"access_key_id", "secret_access_key"} {
		if q.Has(k) {
			q.Set(k, "REDACTED")
		}
	}
	u.RawQuery = q.Encode()
	if u.User != nil {
		u.User = url.User("REDACTED")
	}
	return u.String()
}

func (s *DStore) get(ctx context.Context, key string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	r, err := s.store.OpenObject(ctx, key)
	if err != nil {
		if errors.Is(err, dstore.ErrNotFound) {
			return nil, fmt.Errorf("%s: %w", key, ErrNotFound)
		}
		return nil, fmt.Errorf("get %s: %w", key, err)
	}
	defer func() { _ = r.Close() }()
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", key, err)
	}
	return data, nil
}

func (s *DStore) put(ctx context.Context, key string, data []byte) error {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	if err := s.store.WriteObject(ctx, key, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("put %s: %w", key, err)
	}
	return nil
}

// GetSlot returns the serialized Slot, decompressing the stored object.
func (s *DStore) GetSlot(ctx context.Context, slot uint64) ([]byte, error) {
	stored, err := s.get(ctx, SlotKey(slot))
	if err != nil {
		return nil, err
	}
	raw, err := DecodeSlot(stored)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", SlotKey(slot), err)
	}
	return raw, nil
}

// PutSlot stores a serialized Slot as a zstd frame.
func (s *DStore) PutSlot(ctx context.Context, slot uint64, data []byte) error {
	return s.put(ctx, SlotKey(slot), EncodeSlot(data))
}

func (s *DStore) DeleteSlots(ctx context.Context, slots []uint64) error {
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(deleteParallelism)
	for _, slot := range slots {
		key := SlotKey(slot)
		g.Go(func() error {
			ctx, cancel := context.WithTimeout(gctx, s.timeout)
			defer cancel()
			if err := s.store.DeleteObject(ctx, key); err != nil && !errors.Is(err, dstore.ErrNotFound) {
				return fmt.Errorf("delete %s: %w", key, err)
			}
			return nil
		})
	}
	return g.Wait()
}

func (s *DStore) GetRoot(ctx context.Context, root string) (uint64, error) {
	data, err := s.get(ctx, RootKey(root))
	if err != nil {
		return 0, err
	}
	slot, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("root %s: malformed slot %q: %w", root, string(data), err)
	}
	return slot, nil
}

func (s *DStore) PutRoot(ctx context.Context, root string, slot uint64) error {
	return s.put(ctx, RootKey(root), []byte(strconv.FormatUint(slot, 10)))
}

func (s *DStore) GetHead(ctx context.Context) (Head, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	var keys []string
	err := s.store.Walk(ctx, headPrefix, func(name string) error {
		if !strings.HasSuffix(name, tmpSuffix) {
			keys = append(keys, name)
		}
		return nil
	})
	if err != nil {
		return Head{}, fmt.Errorf("list heads: %w", err)
	}
	var best Head
	found := false
	for _, key := range keys {
		data, err := s.get(ctx, key)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue // raced with a writer rewriting it
			}
			return Head{}, err
		}
		var head Head
		if err := json.Unmarshal(data, &head); err != nil {
			return Head{}, fmt.Errorf("head %s: %w", key, err)
		}
		if !found || head.Slot > best.Slot {
			best, found = head, true
		}
	}
	if !found {
		return Head{}, fmt.Errorf("head: %w", ErrNotFound)
	}
	return best, nil
}

func (s *DStore) PutHead(ctx context.Context, writer string, head Head) error {
	head.Writer = writer
	data, err := json.Marshal(head)
	if err != nil {
		return err
	}
	return s.put(ctx, HeadKey(writer), data)
}

func (s *DStore) GetCursor(ctx context.Context, writer string) (string, error) {
	data, err := s.get(ctx, CursorKey(writer))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (s *DStore) PutCursor(ctx context.Context, writer string, cursor string) error {
	return s.put(ctx, CursorKey(writer), []byte(cursor))
}
