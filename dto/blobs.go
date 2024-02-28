package dto

import (
	pbbl "blob-service/pb/pinax/ethereum/blobs/v1"
	"encoding/hex"
	"encoding/json"
	"strconv"
)

type Blob struct {
	Index                       StrU64            `json:"index"`
	Blob                        HexBytes          `json:"blob"`
	KzgCommitment               HexBytes          `json:"kzg_commitment"`
	KzgProof                    HexBytes          `json:"kzg_proof"`
	SignedBlockHeader           SignedBlockHeader `json:"signed_block_header"`
	KzgCommitmentInclusionProof []HexBytes        `json:"kzg_commitment_inclusion_proof"`
}

type SignedBlockHeader struct {
	Message   *Message `json:"message"`
	Signature HexBytes `json:"signature"`
}

type Message struct {
	Slot          StrU64   `json:"slot"`
	ProposerIndex StrU64   `json:"proposer_index"`
	ParentRoot    HexBytes `json:"parent_root"`
	StateRoot     HexBytes `json:"state_root"`
	BodyRoot      HexBytes `json:"body_root"`
}

type HexBytes []byte

func (h HexBytes) MarshalJSON() ([]byte, error) {
	return json.Marshal("0x" + hex.EncodeToString(h))
}

type StrU64 uint64

func (s StrU64) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatUint(uint64(s), 10))
}

func NewBlob(blob *pbbl.Blob) *Blob {
	return &Blob{
		Index:         StrU64(blob.Index),
		Blob:          blob.Blob,
		KzgCommitment: blob.KzgCommitment,
		KzgProof:      blob.KzgProof,
		SignedBlockHeader: SignedBlockHeader{
			Message: &Message{
				Slot:          StrU64(blob.Slot),
				ProposerIndex: StrU64(blob.ProposerIndex),
				ParentRoot:    blob.ParentRoot,
				StateRoot:     blob.StateRoot,
				BodyRoot:      blob.BodyRoot,
			},
			Signature: blob.Signature,
		},
		KzgCommitmentInclusionProof: convertToHexBytesSlice(blob.KzgCommitmentInclusionProof),
	}
}

func convertToHexBytesSlice(b [][]byte) []HexBytes {
	var res []HexBytes
	for _, v := range b {
		res = append(res, v)
	}
	return res
}
