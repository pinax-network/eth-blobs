// Package store is the object layout the blobs sink writes and the blobs API
// reads, on any github.com/streamingfast/dstore backend (S3-compatible, GCS,
// Azure, a local directory). Every access path is a point lookup by key;
// nothing lists except the handful of per-writer head objects.
//
// Layout inside the store URL (a bucket or directory, optionally with a prefix):
//
//	slots/<slot>.pb     the serialized pinax.ethereum.blobs.v1.Slot, one per
//	                    beacon block, including blocks that carry no blobs
//	roots/0x<root>      decimal slot number of the block with that root
//	head/<writer>       JSON Head of the newest slot <writer> has written
//	cursor/<writer>     the opaque Substreams cursor of <writer>
//
// Several writers (one per DC) may share a bucket: slot and root objects are
// deterministic for a given block so overwrites are harmless, and each writer
// owns its head and cursor objects. Readers take the max head across writers.
package store

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// ErrNotFound is returned when the requested object does not exist.
var ErrNotFound = errors.New("object not found")

// Head describes the newest slot a writer has fully written (slot object and
// root object both present). Timestamp is the slot's block time as unix
// seconds, so a reader can report drift without fetching the slot.
type Head struct {
	Slot      uint64 `json:"slot"`
	Root      string `json:"root"`
	Timestamp int64  `json:"timestamp"`
	Writer    string `json:"writer,omitempty"`
}

// Store is the minimal object-store contract shared by the sink, the API and
// the migration tool.
type Store interface {
	GetSlot(ctx context.Context, slot uint64) ([]byte, error)
	PutSlot(ctx context.Context, slot uint64, data []byte) error
	// DeleteSlots removes the given slot objects; missing ones are not an error.
	DeleteSlots(ctx context.Context, slots []uint64) error

	// GetRoot resolves a normalized 0x-prefixed block root to its slot.
	GetRoot(ctx context.Context, root string) (uint64, error)
	PutRoot(ctx context.Context, root string, slot uint64) error

	// GetHead returns the highest head across all writers.
	GetHead(ctx context.Context) (Head, error)
	PutHead(ctx context.Context, writer string, head Head) error

	GetCursor(ctx context.Context, writer string) (string, error)
	PutCursor(ctx context.Context, writer string, cursor string) error
}

// Keys. Slots are plain decimal: they are never listed, so there is no need
// for zero padding.
func SlotKey(slot uint64) string   { return fmt.Sprintf("slots/%d.pb", slot) }
func RootKey(root string) string   { return "roots/" + root }
func HeadKey(writer string) string { return "head/" + writer }
func CursorKey(writer string) string {
	return "cursor/" + writer
}

const headPrefix = "head/"

// RootHex renders a raw 32-byte root the way root keys and the API expect it.
func RootHex(root []byte) string { return "0x" + hex.EncodeToString(root) }

// NormalizeRoot validates a user-supplied block root and returns it in the
// canonical lowercase 0x-prefixed form used for keys.
func NormalizeRoot(s string) (string, error) {
	if len(s) != 66 || (s[:2] != "0x" && s[:2] != "0X") {
		return "", fmt.Errorf("invalid block root %q: want 0x-prefixed 32-byte hex", s)
	}
	b, err := hex.DecodeString(s[2:])
	if err != nil || len(b) != 32 {
		return "", fmt.Errorf("invalid block root %q: want 0x-prefixed 32-byte hex", s)
	}
	return "0x" + strings.ToLower(s[2:]), nil
}

// ValidateWriter rejects writer ids that would not make a sane key segment.
func ValidateWriter(writer string) error {
	if writer == "" {
		return errors.New("writer id must not be empty")
	}
	if strings.HasSuffix(writer, tmpSuffix) {
		return fmt.Errorf("writer id %q: must not end in %s", writer, tmpSuffix)
	}
	for _, r := range writer {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.'
		if !ok {
			return fmt.Errorf("writer id %q: only [A-Za-z0-9._-] allowed", writer)
		}
	}
	return nil
}
