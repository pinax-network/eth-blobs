package store

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
)

// storeURLs returns one base URL per backend to run the suite against. Each
// test appends its own prefix. BLOBS_TEST_STORE_URL adds a real store (e.g.
// s3://localhost:9000/blobs-test?region=none&insecure=true with AWS credentials
// in the environment).
func storeURLs(t *testing.T) map[string]string {
	t.Helper()
	backend := s3mem.New()
	bucket := fmt.Sprintf("blobs-test-%d", time.Now().UnixNano())
	if err := backend.CreateBucket(bucket); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(gofakes3.New(backend).Server())
	t.Cleanup(srv.Close)
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	host := strings.TrimPrefix(srv.URL, "http://")

	urls := map[string]string{
		"file": "file://" + t.TempDir(),
		// Same shape as a Ceph RGW / MinIO deployment: endpoint host:port, bucket in the path.
		"s3": fmt.Sprintf("s3://%s/%s?region=none&insecure=true", host, bucket),
	}
	if u := os.Getenv("BLOBS_TEST_STORE_URL"); u != "" {
		urls["env"] = u
	}
	return urls
}

// withPrefix appends a path segment to a store URL, keeping its query string.
func withPrefix(t *testing.T, base, prefix string) string {
	t.Helper()
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/" + prefix
	return u.String()
}

func TestDStore(t *testing.T) {
	for name, base := range storeURLs(t) {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			prefix := fmt.Sprintf("eth-%d", time.Now().UnixNano())
			st, err := Open(withPrefix(t, base, prefix)+trailingSlash(name), 0)
			if err != nil {
				t.Fatal(err)
			}
			testStore(ctx, t, st)

			other, err := Open(withPrefix(t, base, prefix+"-other"), 0)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := other.GetHead(ctx); !errors.Is(err, ErrNotFound) {
				t.Fatalf("a store on another prefix must not see heads, got %v", err)
			}
			again, err := Open(withPrefix(t, base, prefix), 0)
			if err != nil {
				t.Fatal(err)
			}
			if head, err := again.GetHead(ctx); err != nil || head.Slot != 105 {
				t.Fatalf("reopened store: head %+v err %v", head, err)
			}
		})
	}
}

// trailingSlash exercises the trailing-slash normalization on one backend.
func trailingSlash(name string) string {
	if name == "file" {
		return "/"
	}
	return ""
}

func testStore(ctx context.Context, t *testing.T, st Store) {
	t.Helper()

	// Slots: missing, present, overwritten, deleted (including keys that never existed).
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
	if err := st.PutSlot(ctx, 1, []byte("reorged")); err != nil {
		t.Fatal(err)
	}
	if got, err := st.GetSlot(ctx, 1); err != nil || string(got) != "reorged" {
		t.Fatalf("slot overwrite must win: %q err=%v", got, err)
	}
	if err := st.DeleteSlots(ctx, []uint64{1, 2, 3}); err != nil {
		t.Fatalf("delete with missing keys must succeed: %v", err)
	}
	if _, err := st.GetSlot(ctx, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted slot: want ErrNotFound, got %v", err)
	}

	// Roots.
	root := "0x" + "ab" + strings.Repeat("0", 62)
	if _, err := st.GetRoot(ctx, root); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing root: want ErrNotFound, got %v", err)
	}
	if err := st.PutRoot(ctx, root, 77); err != nil {
		t.Fatal(err)
	}
	if slot, err := st.GetRoot(ctx, root); err != nil || slot != 77 {
		t.Fatalf("root: slot=%d err=%v", slot, err)
	}

	// Heads: none, max across writers, and a rewritten head wins.
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
	if err := st.PutHead(ctx, "riv", Head{Slot: 105, Root: root, Timestamp: 1700000060}); err != nil {
		t.Fatal(err)
	}
	if head, err := st.GetHead(ctx); err != nil || head.Slot != 105 || head.Writer != "riv" {
		t.Fatalf("rewritten head must win: %+v err=%v", head, err)
	}

	// Cursor: missing, written, rewritten.
	if _, err := st.GetCursor(ctx, "riv"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing cursor: want ErrNotFound, got %v", err)
	}
	for _, c := range []string{"cursor-1", "cursor-2"} {
		if err := st.PutCursor(ctx, "riv", c); err != nil {
			t.Fatal(err)
		}
		if cur, err := st.GetCursor(ctx, "riv"); err != nil || cur != c {
			t.Fatalf("cursor: %q want %q err=%v", cur, c, err)
		}
	}
}

// The local backend writes through <name>.<random>.tmp; a reader listing
// heads mid-write must skip it rather than parse a partial file.
func TestDStoreSkipsLocalTempFiles(t *testing.T) {
	dir := t.TempDir()
	st, err := Open("file://"+dir, 0)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := st.PutHead(ctx, "riv", Head{Slot: 10}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "head", "kan.x1y2z3.tmp"), []byte(`{"slot":`), 0o644); err != nil {
		t.Fatal(err)
	}
	if head, err := st.GetHead(ctx); err != nil || head.Slot != 10 {
		t.Fatalf("head: %+v err=%v", head, err)
	}
}

func TestOpenRejectsBadURLs(t *testing.T) {
	for _, u := range []string{"", "memory://x", "ftp://host/bucket", "s3://bucket/prefix"} { // the last lacks region=
		if _, err := Open(u, 0); err == nil {
			t.Errorf("Open(%q): want error", u)
		}
	}
}

func TestRedactURL(t *testing.T) {
	got := RedactURL("s3://host:9000/b/p?region=none&access_key_id=AKIA&secret_access_key=s3cr3t")
	if strings.Contains(got, "AKIA") || strings.Contains(got, "s3cr3t") || !strings.Contains(got, "region=none") {
		t.Fatalf("RedactURL: %s", got)
	}
}
