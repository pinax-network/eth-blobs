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
	if !bytes.HasPrefix(stored, zstdMagic) {
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

// Objects written before compression are raw serialized Slots and must read
// back unchanged, including a slot-0 Slot whose first field is omitted.
func TestDecodeSlotPassesRawThrough(t *testing.T) {
	cases := map[string]*pbbl.Slot{
		"slot zero, no blobs": {Root: []byte{1, 2, 3}},
		"empty":               {},
		"with blobs":          {Slot: 42, Root: make([]byte, 32), Blobs: []*pbbl.Blob{{Index: 1, Blob: []byte{9}}}},
		"parent root first":   {ParentRoot: []byte{0x28, 0xb5, 0x2f, 0xfd}},
	}
	for name, pb := range cases {
		raw, err := proto.Marshal(pb)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.HasPrefix(raw, zstdMagic) {
			t.Fatalf("%s: a serialized Slot must never look like a zstd frame", name)
		}
		got, err := DecodeSlot(raw)
		if err != nil || !bytes.Equal(got, raw) {
			t.Errorf("%s: raw object must pass through: err=%v", name, err)
		}
	}
}

func TestDecodeSlotRejectsCorruptFrame(t *testing.T) {
	stored := EncodeSlot(realisticSlot(t, 1, 1))
	stored = stored[:len(stored)/2]
	if _, err := DecodeSlot(stored); err == nil {
		t.Fatal("a truncated zstd frame must fail")
	}
}

// Slots go to the backend compressed; objects written before compression was
// introduced still read back.
func TestDStoreCompressesSlotsAndReadsLegacy(t *testing.T) {
	st, err := Open("file://"+t.TempDir(), 0)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	raw := realisticSlot(t, 100, 3)
	if err := st.PutSlot(ctx, 100, raw); err != nil {
		t.Fatal(err)
	}
	r, err := st.store.OpenObject(ctx, SlotKey(100))
	if err != nil {
		t.Fatal(err)
	}
	onDisk, _ := io.ReadAll(r)
	_ = r.Close()
	if !bytes.HasPrefix(onDisk, zstdMagic) || len(onDisk) >= len(raw) {
		t.Fatalf("slot object on disk must be compressed: %d bytes for %d raw", len(onDisk), len(raw))
	}
	if got, err := st.GetSlot(ctx, 100); err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("GetSlot must return the raw Slot: err=%v", err)
	}

	legacy := realisticSlot(t, 101, 1)
	if err := st.store.WriteObject(ctx, SlotKey(101), bytes.NewReader(legacy)); err != nil {
		t.Fatal(err)
	}
	if got, err := st.GetSlot(ctx, 101); err != nil || !bytes.Equal(got, legacy) {
		t.Fatalf("legacy uncompressed slot object must read back: err=%v", err)
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
