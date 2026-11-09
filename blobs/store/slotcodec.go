package store

import (
	"fmt"

	"github.com/klauspost/compress/zstd"
)

// Slot objects (slots/<slot>.pb.zst) are a zstd frame of the serialized Slot.
// Blobs are often zero-padded, so this saves ~20% on mainnet (more on gnosis,
// little on hoodi). The compression level makes no measurable difference on
// this data, so the fastest one is used.

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

// DecodeSlot returns the serialized Slot held by a stored slot object. Safe for
// concurrent use.
func DecodeSlot(stored []byte) ([]byte, error) {
	raw, err := slotDecoder.DecodeAll(stored, nil)
	if err != nil {
		return nil, fmt.Errorf("decompress slot object: %w", err)
	}
	return raw, nil
}
