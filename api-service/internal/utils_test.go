package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestVersionedHashFromCommitment(t *testing.T) {
	// Known fixture: versioned hash of the empty commitment is the SHA-256
	// of the empty byte string with the first byte forced to 0x01.
	got := VersionedHashFromCommitment(nil)

	empty := sha256.Sum256(nil)
	want := empty
	want[0] = 0x01

	if got != want {
		t.Errorf("empty commitment: got %x want %x", got, want)
	}
	if got[0] != 0x01 {
		t.Errorf("first byte must be 0x01, got 0x%02x", got[0])
	}

	// Determinism + dependence on input.
	a := VersionedHashFromCommitment([]byte{1, 2, 3})
	b := VersionedHashFromCommitment([]byte{1, 2, 3})
	c := VersionedHashFromCommitment([]byte{1, 2, 4})
	if a != b {
		t.Error("same input produced different hashes")
	}
	if a == c {
		t.Error("different inputs produced the same hash")
	}
}

func TestParseVersionedHash(t *testing.T) {
	valid := "0x01" + strings.Repeat("ab", 31)
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"valid", valid, false},
		{"valid uppercase prefix", "0X01" + strings.Repeat("ab", 31), false},
		{"no 0x prefix", strings.TrimPrefix(valid, "0x"), true},
		{"bad hex", "0x01zz" + strings.Repeat("ab", 30), true},
		{"too short", "0x0100", true},
		{"too long", valid + "ab", true},
		{"wrong version byte 0x00", "0x00" + strings.Repeat("ab", 31), true},
		{"wrong version byte 0x02", "0x02" + strings.Repeat("ab", 31), true},
		{"empty", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseVersionedHash(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			// Round-trip: decoded bytes should match the hex input.
			want, _ := hex.DecodeString(tc.in[2:])
			for i := range got {
				if got[i] != want[i] {
					t.Errorf("byte %d: got 0x%02x want 0x%02x", i, got[i], want[i])
				}
			}
		})
	}
}

func TestContains(t *testing.T) {
	if !Contains([]int{1, 2, 3}, 2) {
		t.Error("expected to find 2")
	}
	if Contains([]int{1, 2, 3}, 4) {
		t.Error("did not expect to find 4")
	}
	if !Contains([]string{"a", "b"}, "b") {
		t.Error("expected to find b")
	}
	if Contains([]int{}, 1) {
		t.Error("empty slice should not contain anything")
	}
}
