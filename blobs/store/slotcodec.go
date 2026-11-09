package store

import (
	"bytes"
	"fmt"

	"github.com/klauspost/compress/zstd"
)

// Slot objects are stored as a zstd frame of the serialized Slot. Blobs are
// often zero-padded, so this saves ~20% on mainnet (more on gnosis, little on
// hoodi). The compression level makes no measurable difference on this data,
// so the fastest one is used.
//
// Objects written before compression was introduced are the raw serialized
// Slot; DecodeSlot passes them through. The two cannot be confused: a zstd
// frame starts with 28 b5 2f fd, and a serialized Slot never starts with 0x28
// (that would be field 5 with varint wire type, and field 5 is bytes).

var zstdMagic = []byte{0x28, 0xb5, 0x2f, 0xfd}

var (
	slotEncoder = mustEncoder()
	slotDecoder = mustDecoder()
)

func mustEncoder() *zstd.Encoder {
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest))
	if err != nil {
		panic(err)
	}
	return enc
}

func mustDecoder() *zstd.Decoder {
	dec, err := zstd.NewReader(nil, zstd.WithDecoderConcurrency(0))
	if err != nil {
		panic(err)
	}
	return dec
}

// EncodeSlot compresses a serialized Slot for storage. Safe for concurrent use.
func EncodeSlot(raw []byte) []byte {
	return slotEncoder.EncodeAll(raw, make([]byte, 0, len(raw)))
}

// DecodeSlot returns the serialized Slot held by a stored slot object, whether
// it was written compressed or (older objects) raw. Safe for concurrent use.
func DecodeSlot(stored []byte) ([]byte, error) {
	if !bytes.HasPrefix(stored, zstdMagic) {
		return stored, nil
	}
	raw, err := slotDecoder.DecodeAll(stored, nil)
	if err != nil {
		return nil, fmt.Errorf("decompress slot object: %w", err)
	}
	return raw, nil
}
