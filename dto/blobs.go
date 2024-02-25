package dto

import (
	pbbl "blob-service/pb"
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
	Extra                       Extra             `json:"extra"`
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

type Extra struct {
	BlockNumber StrU64     `json:"block_number"`
	Timestamp   *Timestamp `json:"timestamp"`
}

type Timestamp struct {
	Seconds int64 `json:"seconds,omitempty"`
	Nanos   int32 `json:"nanos,omitempty"`
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
				Slot:          StrU64(blob.SignedBlockHeader.Message.Slot),
				ProposerIndex: StrU64(blob.SignedBlockHeader.Message.ProposerIndex),
				ParentRoot:    blob.SignedBlockHeader.Message.ParentRoot,
				StateRoot:     blob.SignedBlockHeader.Message.StateRoot,
				BodyRoot:      blob.SignedBlockHeader.Message.BodyRoot,
			},
			Signature: blob.SignedBlockHeader.Signature,
		},
		KzgCommitmentInclusionProof: convertToHexBytesSlice(blob.KzgCommitmentInclusionProof),
		Extra: Extra{
			BlockNumber: StrU64(blob.Extra.BlockNumber),
			Timestamp: &Timestamp{
				Seconds: blob.Extra.Timestamp.Seconds,
				Nanos:   blob.Extra.Timestamp.Nanos,
			},
		},
	}
}

func convertToHexBytesSlice(b [][]byte) []HexBytes {
	var res []HexBytes
	for _, v := range b {
		res = append(res, v)
	}
	return res
}
