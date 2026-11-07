package controllers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/pinax-network/golang-base/middleware"
	"google.golang.org/protobuf/proto"

	pbbl "github.com/pinax-network/blob-service/blobs/pb/pinax/ethereum/blobs/v1"
	"github.com/pinax-network/blob-service/blobs/services"
	"github.com/pinax-network/blob-service/blobs/store"
)

// versionedHashHex returns the 0x-prefixed EIP-4844 versioned hash of a
// commitment, derived independently of the production helper so the test
// pins the contract rather than the implementation.
func versionedHashHex(commitment []byte) string {
	digest := sha256.Sum256(commitment)
	digest[0] = 0x01
	return "0x" + hex.EncodeToString(digest[:])
}

// setupRouter returns a gin engine with both blobs routes mounted and the
// middleware needed to render structured error responses, backed by an
// in-memory store holding one slot at head. Tests call it once per case and
// exercise the handler via httptest.
func setupRouter(t *testing.T, headSlot uint64, finalityLag uint64, blobs []*pbbl.Blob) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	st := store.NewMemStore()
	ctx := context.Background()
	slotProto := &pbbl.Slot{Slot: headSlot, Blobs: blobs}
	raw, err := proto.Marshal(slotProto)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.PutSlot(ctx, headSlot, raw); err != nil {
		t.Fatal(err)
	}
	if err := st.PutHead(ctx, "test", store.Head{Slot: headSlot}); err != nil {
		t.Fatal(err)
	}

	svc := services.NewBlobsService(st, finalityLag, 0)
	bc := NewBlobsController(svc)

	r := gin.New()
	r.Use(middleware.Errors())
	v1 := r.Group("/eth/v1")
	v1.GET("beacon/blob_sidecars/:block_id", bc.BlobsByBlockId)
	v1.GET("beacon/blobs/:block_id", bc.BlobsByBlockIdV2)
	return r
}

func doGet(t *testing.T, r *gin.Engine, url string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestBlobsByBlockIdV2_ResponseShape(t *testing.T) {
	r := setupRouter(t, 1000, 64, []*pbbl.Blob{
		{Index: 0, Blob: []byte{0xaa}, KzgCommitment: []byte{0x01}},
	})
	w := doGet(t, r, "/eth/v1/beacon/blobs/head")
	if w.Code != 200 {
		t.Fatalf("status: got %d want 200, body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"execution_optimistic", "finalized", "data"} {
		if _, ok := body[key]; !ok {
			t.Errorf("missing field %q in response: %s", key, w.Body.String())
		}
	}
	// blob at "head" slot is at distance 0 from head; lag=64 → not finalized.
	finalized, ok := body["finalized"].(bool)
	if !ok {
		t.Fatalf("finalized: not a bool, got %T (%v)", body["finalized"], body["finalized"])
	}
	if finalized {
		t.Errorf("expected finalized=false for head request with lag=64")
	}
	optimistic, ok := body["execution_optimistic"].(bool)
	if !ok {
		t.Fatalf("execution_optimistic: not a bool, got %T (%v)", body["execution_optimistic"], body["execution_optimistic"])
	}
	if optimistic {
		t.Errorf("expected execution_optimistic=false")
	}
}

func TestBlobsByBlockIdV2_VersionedHashFilter_CommaStyle(t *testing.T) {
	r := setupRouter(t, 1000, 64, []*pbbl.Blob{
		{Index: 0, Blob: []byte{0xaa}, KzgCommitment: []byte{0x01}},
		{Index: 1, Blob: []byte{0xbb}, KzgCommitment: []byte{0x02}},
		{Index: 2, Blob: []byte{0xcc}, KzgCommitment: []byte{0x03}},
	})
	url := fmt.Sprintf("/eth/v1/beacon/blobs/head?versioned_hashes=%s,%s",
		versionedHashHex([]byte{0x01}), versionedHashHex([]byte{0x03}))
	w := doGet(t, r, url)
	if w.Code != 200 {
		t.Fatalf("status: got %d want 200, body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Data []string `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 2 {
		t.Fatalf("got %d blobs, want 2: %v", len(body.Data), body.Data)
	}
	if !strings.HasPrefix(body.Data[0], "0xaa") || !strings.HasPrefix(body.Data[1], "0xcc") {
		t.Errorf("filter order/contents wrong: %v", body.Data)
	}
}

func TestBlobsByBlockIdV2_VersionedHashFilter_RepeatedStyle(t *testing.T) {
	r := setupRouter(t, 1000, 64, []*pbbl.Blob{
		{Index: 0, Blob: []byte{0xaa}, KzgCommitment: []byte{0x01}},
		{Index: 1, Blob: []byte{0xbb}, KzgCommitment: []byte{0x02}},
	})
	url := fmt.Sprintf("/eth/v1/beacon/blobs/head?versioned_hashes=%s&versioned_hashes=%s",
		versionedHashHex([]byte{0x02}), versionedHashHex([]byte{0x99}))
	w := doGet(t, r, url)
	if w.Code != 200 {
		t.Fatalf("status: got %d want 200, body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Data []string `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 1 || !strings.HasPrefix(body.Data[0], "0xbb") {
		t.Errorf("expected just 0xbb, got %v", body.Data)
	}
}

func TestBlobsByBlockIdV2_InvalidVersionedHashReturns400(t *testing.T) {
	r := setupRouter(t, 1000, 64, nil)
	w := doGet(t, r, "/eth/v1/beacon/blobs/head?versioned_hashes=0xnothex")
	if w.Code != 400 {
		t.Errorf("status: got %d want 400, body=%s", w.Code, w.Body.String())
	}
}

func TestBlobsByBlockId_IndicesFilterBothStyles(t *testing.T) {
	makeRouter := func() *gin.Engine {
		return setupRouter(t, 1000, 64, []*pbbl.Blob{
			{Index: 0, Blob: []byte{0xaa}, KzgCommitment: []byte{0x01}},
			{Index: 1, Blob: []byte{0xbb}, KzgCommitment: []byte{0x02}},
			{Index: 2, Blob: []byte{0xcc}, KzgCommitment: []byte{0x03}},
		})
	}
	dataCount := func(body []byte) int {
		var resp struct {
			Data []json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			t.Fatal(err)
		}
		return len(resp.Data)
	}

	cases := []struct {
		name string
		url  string
		want int
	}{
		{"comma style", "/eth/v1/beacon/blob_sidecars/head?indices=0,2", 2},
		{"repeated style", "/eth/v1/beacon/blob_sidecars/head?indices=0&indices=2", 2},
		{"no filter returns all", "/eth/v1/beacon/blob_sidecars/head", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doGet(t, makeRouter(), tc.url)
			if w.Code != 200 {
				t.Fatalf("status: got %d want 200", w.Code)
			}
			if n := dataCount(w.Body.Bytes()); n != tc.want {
				t.Errorf("got %d entries want %d", n, tc.want)
			}
		})
	}
}

func TestBlobsByBlockId_InvalidIndexReturns400(t *testing.T) {
	r := setupRouter(t, 1000, 64, nil)
	w := doGet(t, r, "/eth/v1/beacon/blob_sidecars/head?indices=notanumber")
	if w.Code != 400 {
		t.Errorf("status: got %d want 400, body=%s", w.Code, w.Body.String())
	}
}

func TestBlobsByBlockId_UnknownSlotReturns404(t *testing.T) {
	r := setupRouter(t, 1000, 64, nil)
	w := doGet(t, r, "/eth/v1/beacon/blob_sidecars/999")
	if w.Code != 404 {
		t.Errorf("status: got %d want 404, body=%s", w.Code, w.Body.String())
	}
}

func TestBlobsByBlockId_EmptySlotReturns200WithNoData(t *testing.T) {
	r := setupRouter(t, 1000, 64, nil)
	w := doGet(t, r, "/eth/v1/beacon/blob_sidecars/1000")
	if w.Code != 200 {
		t.Fatalf("status: got %d want 200, body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data == nil || len(body.Data) != 0 {
		t.Errorf("expected empty data array, got %s", w.Body.String())
	}
}
