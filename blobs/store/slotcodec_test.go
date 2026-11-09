package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"testing"

	"google.golang.org/protobuf/proto"

	pbbl "github.com/pinax-network/blob-service/blobs/pb/pinax/ethereum/blobs/v1"
)

// realisticSlot builds a serialized Slot with n blobs, each half random and
// half zero padding, like a rollup batch that does not fill its blob.
func realisticSlot(t testing.TB, slot uint64, n int) []byte {
	t.Helper()
	pb := &pbbl.Slot{Slot: slot, Root: make([]byte, 32), ParentRoot: make([]byte, 32), Signature: make([]byte, 96)}
	_, _ = rand.Read(pb.Root)
	for i := range n {
		blob := make([]byte, 131072)
		_, _ = rand.Read(blob[:65536])
		commitment := make([]byte, 48)
		_, _ = rand.Read(commitment)
		pb.Blobs = append(pb.Blobs, &pbbl.Blob{Index: uint32(i), Blob: blob, KzgCommitment: commitment, KzgProof: commitment})
	}
	raw, err := proto.Marshal(pb)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestSlotCodecRoundTrip(t *testing.T) {
	raw := realisticSlot(t, 15600000, 6)
	stored := EncodeSlot(raw)
	if !bytes.HasPrefix(stored, []byte{0x28, 0xb5, 0x2f, 0xfd}) {
		t.Fatal("stored slot must be a zstd frame")
	}
	if len(stored) >= len(raw)*3/4 {
		t.Errorf("half-zero blobs should compress well: %d -> %d bytes", len(raw), len(stored))
	}
	got, err := DecodeSlot(stored)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("round trip: equal=%v err=%v", bytes.Equal(got, raw), err)
	}
}

// A raw serialized Slot is not a valid slot object.
func TestDecodeSlotRejectsRawSlot(t *testing.T) {
	raw, err := proto.Marshal(&pbbl.Slot{Slot: 42, Root: make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeSlot(raw); err == nil {
		t.Fatal("an uncompressed Slot must not decode")
	}
}

func TestDecodeSlotRejectsCorruptFrame(t *testing.T) {
	stored := EncodeSlot(realisticSlot(t, 1, 1))
	stored = stored[:len(stored)/2]
	if _, err := DecodeSlot(stored); err == nil {
		t.Fatal("a truncated zstd frame must fail")
	}
}

// Slots go to the backend compressed, under the .pb.zst key.
func TestDStoreCompressesSlots(t *testing.T) {
	st, err := Open("file://"+t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	raw := realisticSlot(t, 100, 3)
	if err := st.PutSlot(ctx, 100, raw); err != nil {
		t.Fatal(err)
	}
	if SlotKey(100) != "slots/100.pb.zst" {
		t.Fatalf("slot key: %s", SlotKey(100))
	}
	r, err := st.store.OpenObject(ctx, SlotKey(100))
	if err != nil {
		t.Fatal(err)
	}
	onDisk, _ := io.ReadAll(r)
	_ = r.Close()
	if !bytes.HasPrefix(onDisk, []byte{0x28, 0xb5, 0x2f, 0xfd}) || len(onDisk) >= len(raw) {
		t.Fatalf("slot object on disk must be a zstd frame: %d bytes for %d raw", len(onDisk), len(raw))
	}
	if got, err := st.GetSlot(ctx, 100); err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("GetSlot must return the raw Slot: err=%v", err)
	}
}

func BenchmarkEncodeSlot(b *testing.B) {
	raw := realisticSlot(b, 1, 6)
	b.SetBytes(int64(len(raw)))
	for b.Loop() {
		EncodeSlot(raw)
	}
}

func BenchmarkDecodeSlot(b *testing.B) {
	stored := EncodeSlot(realisticSlot(b, 1, 6))
	for b.Loop() {
		if _, err := DecodeSlot(stored); err != nil {
			b.Fatal(err)
		}
	}
}
