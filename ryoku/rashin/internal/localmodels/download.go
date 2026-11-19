package localmodels

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/safehttp"
)

const (
	partSuffix = ".part"
	// headerWait bounds the wait for response headers; readIdle the wait
	// for the next bytes of a body. Nothing bounds a whole download: a
	// gigabyte on a slow line takes as long as it takes.
	headerWait = 30 * time.Second
	readIdle   = 60 * time.Second
)

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// newClient is the downloads' and Hugging Face's client: public addresses
// only, bounded header and idle-read waits, no total timeout.
func newClient() *http.Client {
	transport := &http.Transport{
		DialContext:           safehttp.DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: headerWait,
		IdleConnTimeout:       90 * time.Second,
	}
	return &http.Client{Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New(i18n.T("The download was redirected too many times."))
		}
		if req.URL.User != nil || req.URL.Scheme != "https" && req.URL.Scheme != "http" {
			return errors.New(i18n.T("The download was redirected to an address Rashin does not follow."))
		}
		return safehttp.CheckHost(req.Context(), req.URL.Hostname())
	}}
}

// fetchRequest is one pinned file to fetch into Dest.
type fetchRequest struct {
	URL    string
	Dest   string
	Size   int64
	SHA256 string
	// Offline verifies what is already on disk and never connects.
	Offline bool
}

// fetchProgress is what a fetch reports: bytes done of the file while it
// downloads or verifies.
type fetchProgress func(phase string, done int64)

// errMismatch is a file whose bytes do not hash to the pinned sha256; kept
// is set when Rashin left it in place (it did not download it).
type errMismatch struct {
	name, want, got string
	kept            bool
}

func (e *errMismatch) Error() string {
	if e.kept {
		return i18n.Tf("%s does not match its checksum (expected %s, got %s).", e.name, e.want, e.got)
	}
	return i18n.Tf("%s did not match its checksum (expected %s, got %s), so Rashin deleted it.", e.name, e.want, e.got)
}

// fetch makes Dest the pinned file. A complete Dest is verified and kept; a
// part file is hashed and resumed with a Range request; a server that
// ignores the range starts it over. The part is synced and renamed into
// place only after its sha256 matches. A cancelled fetch keeps its part.
// fetched reports whether any byte came over the network.
func fetch(ctx context.Context, client *http.Client, req fetchRequest, progress fetchProgress) (fetched bool, err error) {
	name := filepath.Base(req.Dest)
	if info, err := os.Stat(req.Dest); err == nil && info.Mode().IsRegular() && info.Size() == req.Size {
		sum, err := hashFile(ctx, req.Dest, func(n int64) { progress(protocol.LocalPhaseVerify, n) })
		if err != nil {
			return false, err
		}
		if sum == req.SHA256 {
			return false, nil
		}
		if req.Offline {
			return false, &errMismatch{name: name, want: req.SHA256, got: sum, kept: true}
		}
		// not the pinned file: replace it
		if err := os.Remove(req.Dest); err != nil {
			return false, err
		}
	} else if req.Offline {
		return false, errors.New(i18n.Tf("%s is not complete on disk.", name))
	}
	if err := os.MkdirAll(filepath.Dir(req.Dest), 0o700); err != nil {
		return false, err
	}
	part := req.Dest + partSuffix
	f, err := os.OpenFile(part, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return false, err
	}
	defer func() {
		if f != nil {
			_ = f.Close()
		}
	}()
	info, err := f.Stat()
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, errors.New(i18n.T("The partial download is not a regular file."))
	}
	have := info.Size()
	if have > req.Size {
		if err := f.Truncate(0); err != nil {
			return false, err
		}
		have = 0
	}
	sum := sha256.New()
	if have > 0 {
		// the hash of what is there carries on with the new bytes
		if _, err := copyCtx(ctx, sum, io.NewSectionReader(f, 0, have), func(n int64) { progress(protocol.LocalPhaseVerify, n) }); err != nil {
			return false, err
		}
	}
	if _, err := f.Seek(have, io.SeekStart); err != nil {
		return false, err
	}
	for have < req.Size {
		got, restart, err := fetchRange(ctx, client, req.URL, have, req.Size, f, sum, func(n int64) { progress(protocol.LocalPhaseDownload, n) })
		fetched = fetched || got > 0
		if err != nil {
			return fetched, err
		}
		if restart {
			if err := f.Truncate(0); err != nil {
				return fetched, err
			}
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				return fetched, err
			}
			sum.Reset()
			have = 0
			continue
		}
		have += got
		if got == 0 {
			return fetched, errors.New(i18n.Tf("The download of %s ended early.", name))
		}
	}
	got := hex.EncodeToString(sum.Sum(nil))
	if got != req.SHA256 {
		_ = f.Close()
		f = nil
		_ = os.Remove(part)
		return fetched, &errMismatch{name: name, want: req.SHA256, got: got}
	}
	if err := f.Sync(); err != nil {
		return fetched, err
	}
	if err := f.Close(); err != nil {
		f = nil
		return fetched, err
	}
	f = nil
	if err := os.Rename(part, req.Dest); err != nil {
		return fetched, err
	}
	syncDir(filepath.Dir(req.Dest))
	return fetched, nil
}

// fetchRange asks for the bytes from offset on and appends them to w and sum.
// restart is set when the server sent the whole file instead, or refused
// the range, so the caller starts over.
func fetchRange(ctx context.Context, client *http.Client, url string, offset, size int64, w io.Writer, sum hash.Hash, progress func(int64)) (got int64, restart bool, err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, false, err
	}
	if offset > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(offset, 10)+"-")
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, false, unreachable(err)
	}
	defer resp.Body.Close()
	switch {
	case offset > 0 && resp.StatusCode == http.StatusPartialContent:
		if start, ok := rangeStart(resp.Header.Get("Content-Range")); !ok || start != offset {
			return 0, true, nil
		}
	case offset > 0 && (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusRequestedRangeNotSatisfiable):
		return 0, true, nil
	case offset == 0 && resp.StatusCode == http.StatusOK:
	default:
		return 0, false, errors.New(i18n.Tf("The download server answered %s.", resp.Status))
	}
	body := idleReader(resp.Body, cancel, readIdle)
	defer body.stop()
	limit := io.LimitReader(body, size-offset)
	n, err := copyCtx(ctx, io.MultiWriter(w, sum), limit, func(n int64) { progress(offset + n) })
	if err != nil && n == 0 {
		if body.idled() {
			return 0, false, errors.New(i18n.T("The download stalled for a minute; try again to resume it."))
		}
		return 0, false, unreachable(err)
	}
	if err != nil && body.idled() {
		return n, false, errors.New(i18n.T("The download stalled for a minute; try again to resume it."))
	}
	if err != nil && ctx.Err() == nil {
		// a dropped connection: what arrived is kept and the loop resumes
		return n, false, nil
	}
	return n, false, err
}

func rangeStart(header string) (int64, bool) {
	rest, ok := strings.CutPrefix(header, "bytes ")
	if !ok {
		return 0, false
	}
	start, _, ok := strings.Cut(rest, "-")
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(start, 10, 64)
	return n, err == nil
}

func unreachable(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("%s: %w", i18n.T("Rashin could not reach the download server"), err)
}

// copyCtx copies in 1 MiB steps, stopping when ctx ends, and reports the
// running total.
func copyCtx(ctx context.Context, dst io.Writer, src io.Reader, progress func(int64)) (int64, error) {
	buf := make([]byte, 1<<20)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return total, werr
			}
			total += int64(n)
			progress(total)
		}
		if err == io.EOF {
			return total, nil
		}
		if err != nil {
			return total, err
		}
	}
}

func hashFile(ctx context.Context, name string, progress func(int64)) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sum := sha256.New()
	if _, err := copyCtx(ctx, sum, f, progress); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

func syncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
}

// idle cancels a body read that sees no byte for the wait.
type idle struct {
	r      io.Reader
	timer  *time.Timer
	wait   time.Duration
	mu     sync.Mutex
	fired  bool
	cancel context.CancelFunc
}

func idleReader(r io.Reader, cancel context.CancelFunc, wait time.Duration) *idle {
	i := &idle{r: r, wait: wait, cancel: cancel}
	i.timer = time.AfterFunc(wait, func() {
		i.mu.Lock()
		i.fired = true
		i.mu.Unlock()
		cancel()
	})
	return i
}

func (i *idle) Read(p []byte) (int, error) {
	n, err := i.r.Read(p)
	if n > 0 {
		i.timer.Reset(i.wait)
	}
	return n, err
}

func (i *idle) stop() { i.timer.Stop() }

func (i *idle) idled() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.fired
}

// freeBytes is what an unprivileged user may still write under dir, which
// may not exist yet.
func freeBytes(dir string) (int64, bool) {
	for d := dir; ; d = filepath.Dir(d) {
		var st syscall.Statfs_t
		if err := syscall.Statfs(d, &st); err == nil {
			return int64(st.Bavail) * int64(st.Bsize), true
		}
		if parent := filepath.Dir(d); parent == d {
			return 0, false
		}
	}
}

// checkSpace fails when need bytes plus 5% do not fit under dir.
func checkSpace(dir string, need int64) error {
	if need <= 0 {
		return nil
	}
	free, ok := freeBytes(dir)
	if !ok {
		return nil
	}
	if want := need + need/20; free < want {
		return errors.New(i18n.Tf("Not enough free disk space: this needs %s and %s is free.", humanBytes(want), humanBytes(free)))
	}
	return nil
}

func humanBytes(n int64) string {
	const unit = 1000
	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return strconv.FormatFloat(float64(n)/float64(div), 'f', 1, 64) + " " + string("kMGTPE"[exp]) + "B"
}
