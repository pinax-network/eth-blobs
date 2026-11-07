package store

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

// testEndpoint returns an S3 endpoint to test against: BLOBS_S3_TEST_ENDPOINT
// when set (a real RGW or MinIO, with AWS credentials in the environment),
// otherwise an in-process gofakes3 server.
func testEndpoint(t *testing.T) string {
	t.Helper()
	if endpoint := os.Getenv("BLOBS_S3_TEST_ENDPOINT"); endpoint != "" {
		return endpoint
	}
	srv := httptest.NewServer(gofakes3.New(s3mem.New()).Server())
	t.Cleanup(srv.Close)
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	return srv.URL
}

func TestS3Store(t *testing.T) {
	endpoint := testEndpoint(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	bucket := fmt.Sprintf("blobs-test-%d", time.Now().UnixNano())
	st, err := NewS3(ctx, S3Config{Endpoint: endpoint, Bucket: bucket, Prefix: "eth", PathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("create bucket: %v", err)
	}

	// Slots: missing, then present, then deleted (including a key that never existed).
	if _, err := st.GetSlot(ctx, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing slot: want ErrNotFound, got %v", err)
	}
	payload := make([]byte, 256*1024)
	for i := range payload {
		payload[i] = byte(i)
	}
	if err := st.PutSlot(ctx, 1, payload); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetSlot(ctx, 1)
	if err != nil || len(got) != len(payload) || got[12345] != payload[12345] {
		t.Fatalf("round trip: len=%d err=%v", len(got), err)
	}
	if err := st.DeleteSlots(ctx, []uint64{1, 2, 3}); err != nil {
		t.Fatalf("delete with missing keys must succeed: %v", err)
	}
	if _, err := st.GetSlot(ctx, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted slot: want ErrNotFound, got %v", err)
	}

	// Roots.
	root := "0x" + "ab" + "00000000000000000000000000000000000000000000000000000000000000"
	if _, err := st.GetRoot(ctx, root); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing root: want ErrNotFound, got %v", err)
	}
	if err := st.PutRoot(ctx, root, 77); err != nil {
		t.Fatal(err)
	}
	if slot, err := st.GetRoot(ctx, root); err != nil || slot != 77 {
		t.Fatalf("root: slot=%d err=%v", slot, err)
	}

	// Heads: none, one, max across writers.
	if _, err := st.GetHead(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("no head: want ErrNotFound, got %v", err)
	}
	if err := st.PutHead(ctx, "riv", Head{Slot: 100, Root: root, Timestamp: 1700000000}); err != nil {
		t.Fatal(err)
	}
	if err := st.PutHead(ctx, "kan", Head{Slot: 103, Root: root, Timestamp: 1700000036}); err != nil {
		t.Fatal(err)
	}
	head, err := st.GetHead(ctx)
	if err != nil || head.Slot != 103 || head.Writer != "kan" || head.Timestamp != 1700000036 {
		t.Fatalf("head: %+v err=%v", head, err)
	}

	// Cursor.
	if _, err := st.GetCursor(ctx, "riv"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing cursor: want ErrNotFound, got %v", err)
	}
	if err := st.PutCursor(ctx, "riv", "opaque-cursor"); err != nil {
		t.Fatal(err)
	}
	if cur, err := st.GetCursor(ctx, "riv"); err != nil || cur != "opaque-cursor" {
		t.Fatalf("cursor: %q err=%v", cur, err)
	}

	// Prefix isolation: a store on another prefix sees none of it.
	other, _ := NewS3(ctx, S3Config{Endpoint: endpoint, Bucket: bucket, Prefix: "gnosis/", PathStyle: true})
	if _, err := other.GetHead(ctx); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other prefix must not see heads, got %v", err)
	}
}
