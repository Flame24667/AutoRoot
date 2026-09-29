package main

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// rangeServer serves payload, honouring Range so resume can be exercised
// against real HTTP semantics rather than a mock.
func rangeServer(t *testing.T, payload []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHdr := r.Header.Get("Range")
		if rangeHdr == "" {
			w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
			w.Write(payload)
			return
		}
		if !strings.HasPrefix(rangeHdr, "bytes=") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		spec := strings.TrimPrefix(rangeHdr, "bytes=")
		start, err := strconv.Atoi(strings.TrimSuffix(spec, "-"))
		if err != nil || start >= len(payload) {
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		w.Header().Set("Content-Range", "bytes "+strconv.Itoa(start)+"-"+
			strconv.Itoa(len(payload)-1)+"/"+strconv.Itoa(len(payload)))
		w.Header().Set("Content-Length", strconv.Itoa(len(payload)-start))
		w.WriteHeader(http.StatusPartialContent)
		w.Write(payload[start:])
	}))
}

func payloadOf(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i % 251)
	}
	return b
}

func TestDownloadStreamsToDestination(t *testing.T) {
	payload := payloadOf(512 * 1024)
	srv := rangeServer(t, payload)
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "fw.zip")
	res, err := Download(DownloadRequest{
		URL:          srv.URL,
		DestPath:     dest,
		ExpectedSize: int64(len(payload)),
		ExpectedSHA256: func() string {
			s := sha256.Sum256(payload)
			return hex.EncodeToString(s[:])
		}(),
	}, nil)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if res.Resumed {
		t.Error("a fresh download must not be reported as resumed")
	}
	if !res.Verified {
		t.Error("a matching sha256 must be reported as verified")
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(payload) {
		t.Fatalf("downloaded %d bytes, want %d", len(got), len(payload))
	}
	if _, err := os.Stat(dest + partFileSuffix); !os.IsNotExist(err) {
		t.Error("the .part file must be gone after a successful download")
	}
}

func TestDownloadResumesFromPartialFile(t *testing.T) {
	payload := payloadOf(512 * 1024)
	srv := rangeServer(t, payload)
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "fw.zip")
	// Simulate a connection that died after 200 KB.
	const partial = 200 * 1024
	if err := os.WriteFile(dest+partFileSuffix, payload[:partial], 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Download(DownloadRequest{
		URL:          srv.URL,
		DestPath:     dest,
		ExpectedSize: int64(len(payload)),
	}, nil)
	if err != nil {
		t.Fatalf("resumed download failed: %v", err)
	}
	if !res.Resumed {
		t.Error("expected the transfer to be reported as resumed")
	}
	if res.ResumedAt != partial {
		t.Errorf("resumed from %d, want %d", res.ResumedAt, partial)
	}
	if res.HTTPStatus != http.StatusPartialContent {
		t.Errorf("expected HTTP 206, got %d", res.HTTPStatus)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != string(payload) {
		t.Error("resumed file does not match the source payload")
	}
}

func TestDownloadRestartsWhenServerIgnoresRange(t *testing.T) {
	payload := payloadOf(64 * 1024)
	// A server that always answers 200 with the full body, like a naive CDN.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(payload)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "fw.zip")
	if err := os.WriteFile(dest+partFileSuffix, payload[:1024], 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Download(DownloadRequest{
		URL:          srv.URL,
		DestPath:     dest,
		ExpectedSize: int64(len(payload)),
	}, nil)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	got, _ := os.ReadFile(dest)
	// The stale prefix must not survive: the body restarts at zero, so any
	// leftover bytes from the previous attempt would corrupt the result.
	if string(got) != string(payload) {
		t.Error("a server ignoring Range produced a corrupted file")
	}
	if res.Bytes != int64(len(payload)) {
		t.Errorf("wrote %d bytes, want %d", res.Bytes, len(payload))
	}
}

func TestDownloadRejectsSHA256Mismatch(t *testing.T) {
	payload := payloadOf(32 * 1024)
	srv := rangeServer(t, payload)
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "fw.zip")
	_, err := Download(DownloadRequest{
		URL:            srv.URL,
		DestPath:       dest,
		ExpectedSize:   int64(len(payload)),
		ExpectedSHA256: strings.Repeat("0", 64),
	}, nil)
	if err == nil {
		t.Fatal("expected a sha256 mismatch to fail the download")
	}
	if !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(err.Error()) == 0 || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("error should name the failing check, got %v", err)
	}
}

func TestDownloadRejectsSizeMismatch(t *testing.T) {
	payload := payloadOf(32 * 1024)
	srv := rangeServer(t, payload)
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "fw.zip")
	_, err := Download(DownloadRequest{
		URL:          srv.URL,
		DestPath:     dest,
		ExpectedSize: int64(len(payload)) * 2,
	}, nil)
	if err == nil {
		t.Fatal("expected a size mismatch to fail the download")
	}
	if !strings.Contains(err.Error(), "size mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDownloadRejectsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "fw.zip")
	_, err := Download(DownloadRequest{URL: srv.URL, DestPath: dest}, nil)
	if err == nil {
		t.Fatal("expected a 403 to fail the download")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Fatalf("error should carry the HTTP status, got %v", err)
	}
}

func TestDownloadRejectsNonHTTPScheme(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "fw.zip")
	_, err := Download(DownloadRequest{URL: "file:///etc/passwd", DestPath: dest}, nil)
	if err == nil {
		t.Fatal("expected a file:// URL to be rejected")
	}
}

func TestDownloadReportsProgress(t *testing.T) {
	payload := payloadOf(256 * 1024)
	srv := rangeServer(t, payload)
	defer srv.Close()

	var last int64
	res, err := Download(DownloadRequest{
		URL:          srv.URL,
		DestPath:     filepath.Join(t.TempDir(), "fw.zip"),
		ExpectedSize: int64(len(payload)),
	}, func(downloaded, total int64) {
		if downloaded > last {
			last = downloaded
		}
		if total != int64(len(payload)) {
			t.Errorf("progress reported total %d, want %d", total, len(payload))
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if last != res.Bytes {
		t.Errorf("last progress %d != final size %d", last, res.Bytes)
	}
}

func TestFileSHA256MatchesKnownValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.bin")
	payload := payloadOf(1000)
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256(payload)
	got, err := FileSHA256(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != hex.EncodeToString(want[:]) {
		t.Errorf("FileSHA256 = %s, want %s", got, hex.EncodeToString(want[:]))
	}
}
