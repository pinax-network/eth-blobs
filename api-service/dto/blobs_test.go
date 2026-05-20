package dto

import (
	pbbl "blob-service/pb/pinax/ethereum/blobs/v1"
	"encoding/json"
	"strings"
	"testing"
)

func TestHexBytesMarshalJSON(t *testing.T) {
	cases := []struct {
		name string
		in   HexBytes
		want string
	}{
		{"nil", nil, `"0x"`},
		{"empty", HexBytes{}, `"0x"`},
		{"single byte", HexBytes{0xab}, `"0xab"`},
		{"multi byte", HexBytes{0xde, 0xad, 0xbe, 0xef}, `"0xdeadbeef"`},
		{"leading zero preserved", HexBytes{0x00, 0x01}, `"0x0001"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != tc.want {
				t.Errorf("got %s want %s", b, tc.want)
			}
		})
	}
}

func TestStrU64MarshalJSON(t *testing.T) {
	cases := []struct {
		in   StrU64
		want string
	}{
		{0, `"0"`},
		{1, `"1"`},
		{12345, `"12345"`},
		{1<<64 - 1, `"18446744073709551615"`},
	}
	for _, tc := range cases {
		b, err := json.Marshal(tc.in)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != tc.want {
			t.Errorf("%d: got %s want %s", tc.in, b, tc.want)
		}
	}
}

func TestNewBlobCopiesFields(t *testing.T) {
	slot := &pbbl.Slot{
		Slot:          1234,
		ProposerIndex: 99,
		ParentRoot:    []byte{0x11},
		StateRoot:     []byte{0x22},
		BodyRoot:      []byte{0x33},
		Signature:     []byte{0x44},
	}
	blob := &pbbl.Blob{
		Index:                       7,
		Blob:                        []byte{0xaa, 0xbb},
		KzgCommitment:               []byte{0xcc},
		KzgProof:                    []byte{0xdd},
		KzgCommitmentInclusionProof: [][]byte{{0x01}, {0x02}, {0x03}},
	}
	got := NewBlob(blob, slot)
	if got.Index != 7 {
		t.Errorf("Index: got %d want 7", got.Index)
	}
	if string(got.Blob) != "\xaa\xbb" {
		t.Errorf("Blob bytes mismatch")
	}
	if got.SignedBlockHeader.Message.Slot != 1234 {
		t.Errorf("Slot: got %d want 1234", got.SignedBlockHeader.Message.Slot)
	}
	if got.SignedBlockHeader.Message.ProposerIndex != 99 {
		t.Errorf("ProposerIndex: got %d want 99", got.SignedBlockHeader.Message.ProposerIndex)
	}
	if len(got.KzgCommitmentInclusionProof) != 3 {
		t.Errorf("KzgCommitmentInclusionProof: got %d entries want 3", len(got.KzgCommitmentInclusionProof))
	}
}

func TestBlobsResponseJSONShape(t *testing.T) {
	resp := BlobsResponse{
		ExecutionOptimistic: false,
		Finalized:           true,
		Data:                []HexBytes{{0xab}, {0xcd, 0xef}},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	// Field presence and the hex-string encoding for HexBytes inside an array.
	for _, want := range []string{
		`"execution_optimistic":false`,
		`"finalized":true`,
		`"data":["0xab","0xcdef"]`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in %s", want, s)
		}
	}
}

func TestBlobSidecarsResponseJSONShape(t *testing.T) {
	resp := BlobSidecarsResponse{
		ExecutionOptimistic: false,
		Finalized:           true,
		Data: []*Blob{{
			Index:         5,
			Blob:          HexBytes{0xab},
			KzgCommitment: HexBytes{0xcd},
			KzgProof:      HexBytes{0xef},
			SignedBlockHeader: SignedBlockHeader{
				Message:   &Message{Slot: 1, ProposerIndex: 2},
				Signature: HexBytes{0x11},
			},
		}},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		`"execution_optimistic":false`,
		`"finalized":true`,
		`"index":"5"`, // StrU64 → JSON string
		`"slot":"1"`,  // nested in signed_block_header
		`"proposer_index":"2"`,
		`"blob":"0xab"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in %s", want, s)
		}
	}
}
