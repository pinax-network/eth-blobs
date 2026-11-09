package dto

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"testing"

	pbbl "github.com/pinax-network/blob-service/blobs/pb/pinax/ethereum/blobs/v1"
)

// The direct encoder must produce exactly what the original string-based one did.
func TestHexBytesMatchesStringEncoding(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, n := range []int{0, 1, 31, 32, 48, 96, 131072} {
		b := make([]byte, n)
		rng.Read(b)
		want, err := json.Marshal("0x" + hex.EncodeToString(b))
		if err != nil {
			t.Fatal(err)
		}
		got, err := json.Marshal(HexBytes(b))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("len %d: encodings differ", n)
		}
	}
}

func randBytes(rng *rand.Rand, n int) []byte {
	b := make([]byte, n)
	rng.Read(b)
	return b
}

// A 6-blob slot is about 1.6 MB of JSON, close to an average mainnet response.
func benchSidecars(nblobs int) BlobSidecarsResponse {
	rng := rand.New(rand.NewSource(1))
	slot := &pbbl.Slot{
		Slot: 15600000, ProposerIndex: 1,
		ParentRoot: randBytes(rng, 32), StateRoot: randBytes(rng, 32), BodyRoot: randBytes(rng, 32),
		Signature: randBytes(rng, 96),
	}
	resp := BlobSidecarsResponse{}
	for i := 0; i < nblobs; i++ {
		proof := make([][]byte, 17)
		for j := range proof {
			proof[j] = randBytes(rng, 32)
		}
		resp.Data = append(resp.Data, NewBlob(&pbbl.Blob{
			Index: uint32(i), Blob: randBytes(rng, 131072),
			KzgCommitment: randBytes(rng, 48), KzgProof: randBytes(rng, 48),
			KzgCommitmentInclusionProof: proof,
		}, slot))
	}
	return resp
}

func BenchmarkBlobSidecarsMarshal(b *testing.B) {
	resp := benchSidecars(6)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := json.Marshal(resp); err != nil {
			b.Fatal(err)
		}
	}
}
