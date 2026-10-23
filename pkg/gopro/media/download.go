package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"

	"github.com/kazilotus/gopro9-cli/pkg/gopro/transport"
)

// Progress reports bytes written so far and the total size (-1 when unknown).
type Progress func(path string, done, total int64)

// DownloadOptions tune a transfer.
type DownloadOptions struct {
	// Resume continues a partial file found at the destination.
	Resume bool
	// Verify fails when the final size differs from the camera's Content-Range total.
	Verify bool
	// Overwrite replaces a complete existing file instead of skipping it.
	Overwrite bool
	Progress  Progress
}

// DownloadResult describes one finished transfer.
type DownloadResult struct {
	Source  string `json:"source" yaml:"source"`
	Dest    string `json:"dest" yaml:"dest"`
	Bytes   int64  `json:"bytes" yaml:"bytes"`
	Resumed bool   `json:"resumed" yaml:"resumed"`
	Skipped bool   `json:"skipped" yaml:"skipped"`
}

// ErrSizeMismatch is returned by Verify when the downloaded size is wrong.
var ErrSizeMismatch = errors.New("media: downloaded size does not match the camera's reported size")

var contentRangeRe = regexp.MustCompile(`bytes \d+-\d+/(\d+)`)

// Size returns the byte length of a media file.
//
// The camera forbids HEAD on media files, so this issues a one-byte Range
// request and reads the Content-Range total.
func (c *Client) Size(ctx context.Context, p string) (int64, error) {
	if err := validatePath(p); err != nil {
		return 0, err
	}
	h := http.Header{}
	h.Set("Range", "bytes=0-0")
	resp, body, err := c.t.Open(ctx, transport.Request{Path: "/videos/DCIM/" + p, Headers: h, Idempotent: true, Accept: []int{http.StatusPartialContent, http.StatusOK}}) //nolint:bodyclose // closed via body below
	if err != nil {
		return 0, err
	}
	defer func() { _ = body.Close() }()
	if resp.StatusCode == http.StatusOK {
		return resp.ContentLength, nil
	}
	m := contentRangeRe.FindStringSubmatch(resp.Header.Get("Content-Range"))
	if m == nil {
		return 0, fmt.Errorf("%w: missing Content-Range on %s", transport.ErrBadResponse, p)
	}
	return strconv.ParseInt(m[1], 10, 64)
}

// Download fetches "DIR/NAME" to dest, resuming a partial file when asked.
func (c *Client) Download(ctx context.Context, p, dest string, opts DownloadOptions) (DownloadResult, error) {
	res := DownloadResult{Source: p, Dest: dest}
	if err := validatePath(p); err != nil {
		return res, err
	}
	total, err := c.Size(ctx, p)
	if err != nil {
		return res, err
	}
	var offset int64
	if st, serr := os.Stat(dest); serr == nil {
		switch {
		case st.Size() == total && !opts.Overwrite:
			res.Bytes, res.Skipped = total, true
			return res, nil
		case opts.Resume && st.Size() < total:
			offset, res.Resumed = st.Size(), true
		}
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return res, err
	}
	flags := os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	if res.Resumed {
		flags = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	}
	f, err := os.OpenFile(dest, flags, 0o644) //nolint:gosec // media files are meant to be readable
	if err != nil {
		return res, err
	}
	defer func() { _ = f.Close() }()

	h := http.Header{}
	accept := []int{http.StatusOK}
	if res.Resumed {
		h.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
		accept = append(accept, http.StatusPartialContent)
	}
	resp, body, err := c.t.Open(ctx, transport.Request{Path: "/videos/DCIM/" + p, Headers: h, Accept: accept}) //nolint:bodyclose // closed via body below
	if err != nil {
		return res, err
	}
	defer func() { _ = body.Close() }()
	if res.Resumed && resp.StatusCode == http.StatusOK {
		// Camera ignored the range: start over.
		if err := f.Truncate(0); err != nil {
			return res, err
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return res, err
		}
		offset, res.Resumed = 0, false
	}
	done := offset
	if opts.Progress != nil {
		opts.Progress(p, done, total)
	}
	buf := make([]byte, 256<<10)
	for {
		n, rerr := body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return res, werr
			}
			done += int64(n)
			if opts.Progress != nil {
				opts.Progress(p, done, total)
			}
		}
		if rerr != nil {
			if errors.Is(rerr, io.EOF) {
				break
			}
			return res, rerr
		}
	}
	res.Bytes = done
	if opts.Verify && done != total {
		return res, fmt.Errorf("%w: %s got %d want %d", ErrSizeMismatch, p, done, total)
	}
	return res, nil
}

// DownloadMany fetches several files into dir with bounded parallelism.
// It stops at the first error and returns the results completed so far.
func (c *Client) DownloadMany(ctx context.Context, paths []string, dir string, parallel int, opts DownloadOptions) ([]DownloadResult, error) {
	if parallel < 1 {
		parallel = 1
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		mu      sync.Mutex
		results []DownloadResult
		firstEr error
		wg      sync.WaitGroup
		sem     = make(chan struct{}, parallel)
	)
	for _, p := range paths {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(p string) {
			defer wg.Done()
			defer func() { <-sem }()
			r, err := c.Download(ctx, p, filepath.Join(dir, filepath.FromSlash(p)), opts)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstEr == nil {
					firstEr = fmt.Errorf("%s: %w", p, err)
					cancel()
				}
				return
			}
			results = append(results, r)
		}(p)
	}
	wg.Wait()
	return results, firstEr
}
