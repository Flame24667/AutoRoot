package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DownloadRequest describes one firmware fetch.
type DownloadRequest struct {
	URL      string
	DestPath string

	// ExpectedSHA256, when set, is verified after the transfer completes.
	// Empty means "no known-good digest", which is reported as a warning
	// rather than treated as a pass.
	ExpectedSHA256 string

	// ExpectedSize, when non-zero, is checked against Content-Length and the
	// final on-disk size.
	ExpectedSize int64
}

// DownloadResult reports the outcome of a transfer.
type DownloadResult struct {
	Path       string
	Bytes      int64
	Resumed    bool
	ResumedAt  int64
	SHA256     string
	Verified   bool
	Duration   time.Duration
	HTTPStatus int
	FinalURL   string
	Warnings   []string
}

// downloadClient is a plain streaming HTTP client. Go's default transport
// already uses TLS, which is independent of Windows Schannel, so this path is
// not affected by the SChannel credential failures that break Rust's
// schannel-based build of samloader.
var downloadClient = &http.Client{
	Timeout: 0, // no global cap; per-request context handles cancellation
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          4,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   20 * time.Second,
		ExpectContinueTimeout: 5 * time.Second,
	},
}

// partFileSuffix marks the in-progress transfer so a crash leaves a resumable
// artifact instead of a file that looks complete.
const partFileSuffix = ".part"

// Download streams url to destPath, resuming from any existing .part file.
//
// Resumability relies on the server honouring Range requests. When it does not
// (or answers 416 because the local part is already complete), the download
// restarts from zero, so the call is always safe to retry.
//
// The bytes are written straight to the destination volume. Nothing is staged
// in the system drive's temp folder, which is the reason earlier attempts
// failed: the C: volume had only ~10 GB free for a 5.81 GB package plus its
// extracted parts.
func Download(req DownloadRequest, progress func(downloaded, total int64)) (*DownloadResult, error) {
	if strings.TrimSpace(req.URL) == "" {
		return nil, fmt.Errorf("download URL is required")
	}
	if strings.TrimSpace(req.DestPath) == "" {
		return nil, fmt.Errorf("download destination is required")
	}
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		return nil, fmt.Errorf("download URL must be http or https, got %q", req.URL)
	}

	if err := os.MkdirAll(filepath.Dir(req.DestPath), 0o755); err != nil {
		return nil, fmt.Errorf("cannot create destination folder: %w", err)
	}

	partPath := req.DestPath + partFileSuffix
	result := &DownloadResult{Path: req.DestPath, FinalURL: req.URL}
	started := time.Now()

	var offset int64
	if st, err := os.Stat(partPath); err == nil && st.Size() > 0 {
		offset = st.Size()
		result.Resumed = true
		result.ResumedAt = offset
	}

	attempts := 0
	for {
		attempts++
		if attempts > 5 {
			return nil, fmt.Errorf("download did not complete after %d attempts; %s is kept for a later resume", attempts-1, partPath)
		}

		written, status, finalURL, err := downloadOnce(req, partPath, offset, progress)
		result.HTTPStatus = status
		result.FinalURL = finalURL

		if err != nil {
			// A partial transfer is normal on a dropped connection; keep the
			// bytes and let the caller retry or resume later.
			if st, statErr := os.Stat(partPath); statErr == nil {
				result.Bytes = st.Size()
			}
			return result, fmt.Errorf("download interrupted at %s: %w", formatSize(result.Bytes), err)
		}
		offset = written
		if status == http.StatusOK && offset < req.ExpectedSize && req.ExpectedSize > 0 {
			// A 200 to a ranged request means the server ignored Range and we
			// restarted. Loop once more to reach the full length.
			continue
		}
		break
	}

	if req.ExpectedSize > 0 && offset != req.ExpectedSize {
		return result, fmt.Errorf("size mismatch: expected %s, downloaded %s", formatSize(req.ExpectedSize), formatSize(offset))
	}

	if err := os.Rename(partPath, req.DestPath); err != nil {
		return result, fmt.Errorf("cannot finalize download: %w", err)
	}
	result.Bytes = offset
	result.Duration = time.Since(started)

	sum, err := FileSHA256(req.DestPath)
	if err != nil {
		return result, fmt.Errorf("cannot hash downloaded file: %w", err)
	}
	result.SHA256 = sum

	if want := strings.ToLower(strings.TrimSpace(req.ExpectedSHA256)); want != "" {
		if want != sum {
			return result, fmt.Errorf("sha256 mismatch: expected %s, got %s", want, sum)
		}
		result.Verified = true
	} else {
		result.Warnings = append(result.Warnings,
			"no trusted sha256 was supplied for this file; integrity rests on the internal .tar.md5 checks alone")
	}

	return result, nil
}

// downloadOnce performs a single ranged request and returns the new total size
// on disk. status 206 means the server honoured Range; 200 means it did not.
func downloadOnce(req DownloadRequest, partPath string, offset int64, progress func(int64, int64)) (int64, int, string, error) {
	httpReq, err := http.NewRequest(http.MethodGet, req.URL, nil)
	if err != nil {
		return offset, 0, req.URL, err
	}
	httpReq.Header.Set("User-Agent", "AutoRoot/1.0 (+firmware fetcher)")
	if offset > 0 {
		httpReq.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	resp, err := downloadClient.Do(httpReq)
	if err != nil {
		return offset, 0, req.URL, err
	}
	defer resp.Body.Close()

	finalURL := req.URL
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// Fresh transfer, or the server ignored our Range header. Either way
		// the body starts at zero, so truncate any existing partial file.
		offset = 0
	case http.StatusPartialContent:
		// Resumed as requested.
	case http.StatusRequestedRangeNotSatisfiable:
		// The local part is already at or past the end; treat as complete only
		// if it matches the expected size, otherwise restart cleanly.
		if st, statErr := os.Stat(partPath); statErr == nil {
			if req.ExpectedSize > 0 && st.Size() == req.ExpectedSize {
				return st.Size(), resp.StatusCode, finalURL, nil
			}
		}
		if err := os.Remove(partPath); err != nil {
			return offset, resp.StatusCode, finalURL, err
		}
		return 0, resp.StatusCode, finalURL, nil
	default:
		return offset, resp.StatusCode, finalURL,
			fmt.Errorf("server returned %s", resp.Status)
	}

	flags := os.O_WRONLY | os.O_CREATE
	if offset == 0 {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(partPath, flags, 0o644)
	if err != nil {
		return offset, resp.StatusCode, finalURL, err
	}
	defer f.Close()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset, resp.StatusCode, finalURL, err
	}

	total := resp.ContentLength
	if resp.StatusCode == http.StatusPartialContent && total >= 0 {
		total += offset
	}
	if req.ExpectedSize > 0 {
		total = req.ExpectedSize
	}

	written := offset
	buf := make([]byte, 1<<20) // 1 MiB chunks keep memory flat on 5.8 GB
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				return written, resp.StatusCode, finalURL, err
			}
			written += int64(n)
			if progress != nil {
				progress(written, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return written, resp.StatusCode, finalURL, readErr
		}
	}

	// Flush before the caller hashes or renames the file.
	if err := f.Sync(); err != nil {
		return written, resp.StatusCode, finalURL, err
	}
	return written, resp.StatusCode, finalURL, nil
}

// FileSHA256 hashes a file without loading it into memory, so a 5.8 GB
// firmware can be verified on a machine with limited RAM.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	st, err := f.Stat()
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(h, &progressReader{reader: f, total: st.Size(), detail: "SHA-256: " + filepath.Base(path)}); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// DownloadState is the resume record persisted next to a partial download so
// a restart can continue instead of starting over.
type DownloadState struct {
	URL             string `json:"url"`
	DestPath        string `json:"destPath"`
	PartPath        string `json:"partPath"`
	BytesDownloaded int64  `json:"bytesDownloaded"`
	Total           int64  `json:"total"`
	ExpectedSHA256  string `json:"expectedSha256,omitempty"`
	StartedAt       string `json:"startedAt"`
	UpdatedAt       string `json:"updatedAt"`
	Done            bool   `json:"done"`
	SHA256          string `json:"sha256,omitempty"`
}

func (d *DownloadState) save() error {
	dir := cacheDir()
	path := filepath.Join(dir, "download-state.json")
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadDownloadState restores the last transfer record, if any.
func LoadDownloadState() (*DownloadState, error) {
	path := filepath.Join(cacheDir(), "download-state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var st DownloadState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	return &st, nil
}
