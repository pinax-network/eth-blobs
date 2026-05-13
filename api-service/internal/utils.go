package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func Contains[T comparable](slice []T, target T) bool {
	for _, value := range slice {
		if value == target {
			return true
		}
	}
	return false
}

// VersionedHashFromCommitment computes the EIP-4844 versioned hash for a
// KZG commitment: sha256(commitment) with the first byte replaced by the
// blob-commitment version byte (0x01).
func VersionedHashFromCommitment(commitment []byte) [32]byte {
	h := sha256.Sum256(commitment)
	h[0] = 0x01
	return h
}

// ParseVersionedHash decodes a "0x"-prefixed 32-byte versioned hash string.
// Returns ErrInvalidVersionedHash if the input is malformed.
func ParseVersionedHash(s string) ([32]byte, error) {
	var out [32]byte
	if !strings.HasPrefix(s, "0x") && !strings.HasPrefix(s, "0X") {
		return out, ErrInvalidVersionedHash
	}
	b, err := hex.DecodeString(s[2:])
	if err != nil || len(b) != 32 {
		return out, ErrInvalidVersionedHash
	}
	copy(out[:], b)
	return out, nil
}
