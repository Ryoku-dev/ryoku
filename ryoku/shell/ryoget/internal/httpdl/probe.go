// Package httpdl is ryoget's segmented HTTP(S) downloader: one ranged GET whose
// body is the first connection's stream, more connections that split the rest
// in bounded requests, a JSON sidecar to resume from, and a single stream for
// servers that do not take ranges.
package httpdl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const UserAgent = "ryoget/1 (+https://ryoku.dev)"

const (
	connectTimeout = 15 * time.Second
	idleTimeout    = 30 * time.Second
)

// NewClient speaks HTTP/1.1 only: several ranged requests multiplexed over one
// HTTP/2 connection would share one congestion window and gain nothing.
func NewClient() *http.Client {
	d := &net.Dialer{Timeout: connectTimeout, KeepAlive: 30 * time.Second}
	t := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           d.DialContext,
		TLSHandshakeTimeout:   connectTimeout,
		ResponseHeaderTimeout: idleTimeout,
		MaxIdleConnsPerHost:   32,
		IdleConnTimeout:       60 * time.Second,
	}
	t.Protocols = new(http.Protocols)
	t.Protocols.SetHTTP1(true)
	return &http.Client{Transport: t}
}

type StatusError struct {
	Code   int
	Status string
	// from Retry-After (seconds or an HTTP date), capped, 0 when absent
	RetryAfter time.Duration
}

const maxRetryAfter = 300 * time.Second

func statusError(resp *http.Response) *StatusError {
	return &StatusError{Code: resp.StatusCode, Status: resp.Status, RetryAfter: retryAfter(resp.Header.Get("Retry-After"), time.Now())}
}

func retryAfter(v string, now time.Time) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	var d time.Duration
	if s, err := strconv.ParseInt(v, 10, 64); err == nil {
		if s <= 0 {
			return 0
		}
		d = time.Duration(min(s, int64(maxRetryAfter/time.Second))) * time.Second
	} else if at, err := http.ParseTime(v); err == nil {
		d = at.Sub(now)
	}
	return max(0, min(d, maxRetryAfter))
}

func (e *StatusError) Error() string { return "http " + e.Status }

type Info struct {
	// URL after redirects
	URL          string
	Name         string
	Mime         string
	Size         int64
	Ranges       bool
	ETag         string
	LastModified string
}

func newRequest(ctx context.Context, method, raw string, h http.Header) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, raw, nil)
	if err != nil {
		return nil, err
	}
	for k, vs := range h {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", UserAgent)
	}
	return req, nil
}

func checkURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("not an http(s) link: %q", raw)
	}
	return nil
}

// Probe asks with a one-byte ranged GET and not HEAD, which many servers answer wrongly
func Probe(ctx context.Context, c *http.Client, raw string, h http.Header) (*Info, error) {
	if c == nil {
		c = NewClient()
	}
	if err := checkURL(raw); err != nil {
		return nil, err
	}
	req, err := newRequest(ctx, http.MethodGet, raw, h)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", "bytes=0-0")
	req.Header.Set("Accept-Encoding", "identity")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	info := infoFrom(resp, raw)
	switch resp.StatusCode {
	case http.StatusPartialContent:
		if _, _, total, ok := contentRange(resp.Header.Get("Content-Range")); ok && total >= 0 {
			info.Size = total
			info.Ranges = true
		}
	case http.StatusOK:
		info.Size = bodySize(resp)
	case http.StatusRequestedRangeNotSatisfiable:
		if _, _, total, ok := contentRange(resp.Header.Get("Content-Range")); ok && total == 0 {
			info.Size = 0
		}
	default:
		return nil, statusError(resp)
	}
	return info, nil
}

// size and ranges depend on the status, so infoFrom leaves them to the caller
func infoFrom(resp *http.Response, orig string) *Info {
	info := &Info{URL: resp.Request.URL.String(), Size: -1}
	info.Mime = mediaType(resp.Header.Get("Content-Type"))
	info.ETag = resp.Header.Get("ETag")
	info.LastModified = resp.Header.Get("Last-Modified")
	info.Name = FileName(resp.Header.Get("Content-Disposition"), info.URL, orig, info.Mime)
	return info
}

// bodySize is a 200's length, or -1 when it is unknown or compressed.
func bodySize(resp *http.Response) int64 {
	if resp.ContentLength >= 0 && (resp.Header.Get("Content-Encoding") == "" || strings.EqualFold(resp.Header.Get("Content-Encoding"), "identity")) {
		return resp.ContentLength
	}
	return -1
}

func mediaType(ct string) string {
	t, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return ""
	}
	return t
}

// contentRange parses "bytes a-b/total"; total is -1 when the server sent "*".
func contentRange(v string) (start, end, total int64, ok bool) {
	v, found := strings.CutPrefix(strings.TrimSpace(v), "bytes ")
	if !found {
		return 0, 0, 0, false
	}
	rng, tot, found := strings.Cut(v, "/")
	if !found {
		return 0, 0, 0, false
	}
	total = -1
	if tot != "*" {
		t, err := strconv.ParseInt(tot, 10, 64)
		if err != nil {
			return 0, 0, 0, false
		}
		total = t
	}
	if rng == "*" {
		return 0, -1, total, true
	}
	a, b, found := strings.Cut(rng, "-")
	if !found {
		return 0, 0, 0, false
	}
	s, err1 := strconv.ParseInt(a, 10, 64)
	e, err2 := strconv.ParseInt(b, 10, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, 0, false
	}
	return s, e, total, true
}

var looseFilename = regexp.MustCompile(`(?i)filename\s*=\s*"?([^";]+)"?`)

// FileName picks the name a download lands under: Content-Disposition (RFC 6266,
// filename* first), then the last URL path segment, then the host; an extension
// comes from Content-Type when the name has none.
func FileName(disposition, finalURL, origURL, contentType string) string {
	name := ""
	if disposition != "" {
		if _, params, err := mime.ParseMediaType(disposition); err == nil {
			name = params["filename"]
		} else if m := looseFilename.FindStringSubmatch(disposition); m != nil {
			name = m[1]
		}
		name = Sanitize(name)
	}
	if name == "" {
		final, orig := urlBase(finalURL), urlBase(origURL)
		switch {
		case path.Ext(final) != "":
			name = final
		case path.Ext(orig) != "":
			name = orig
		case final != "":
			name = final
		default:
			name = orig
		}
	}
	fromHost := false
	if name == "" {
		if u, err := url.Parse(origURL); err == nil {
			name = Sanitize(u.Hostname())
			fromHost = name != ""
		}
	}
	if name == "" {
		name = "download"
	}
	if path.Ext(name) == "" || fromHost {
		name += extFor(contentType)
	}
	return name
}

func urlBase(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	p := u.Path
	if strings.HasSuffix(p, "/") || p == "" {
		return ""
	}
	return Sanitize(path.Base(p))
}

var commonExt = map[string]string{
	"application/pdf":                         ".pdf",
	"application/zip":                         ".zip",
	"application/gzip":                        ".gz",
	"application/x-gzip":                      ".gz",
	"application/x-xz":                        ".xz",
	"application/x-tar":                       ".tar",
	"application/zstd":                        ".zst",
	"application/x-7z-compressed":             ".7z",
	"application/x-iso9660-image":             ".iso",
	"application/x-bittorrent":                ".torrent",
	"application/json":                        ".json",
	"application/vnd.android.package-archive": ".apk",
	"text/html":                               ".html",
	"text/plain":                              ".txt",
	"image/jpeg":                              ".jpg",
	"image/png":                               ".png",
	"image/gif":                               ".gif",
	"image/webp":                              ".webp",
	"image/avif":                              ".avif",
	"image/svg+xml":                           ".svg",
	"video/mp4":                               ".mp4",
	"video/webm":                              ".webm",
	"video/x-matroska":                        ".mkv",
	"video/quicktime":                         ".mov",
	"audio/mpeg":                              ".mp3",
	"audio/mp4":                               ".m4a",
	"audio/ogg":                               ".ogg",
	"audio/opus":                              ".opus",
	"audio/flac":                              ".flac",
	"audio/wav":                               ".wav",
}

func extFor(ct string) string {
	if ct == "" || ct == "application/octet-stream" {
		return ""
	}
	if e, ok := commonExt[ct]; ok {
		return e
	}
	if exts, err := mime.ExtensionsByType(ct); err == nil && len(exts) > 0 {
		return exts[0]
	}
	return ""
}

// Sanitize makes a single safe path element, or "" when nothing usable is left.
func Sanitize(name string) string {
	if dec, err := url.PathUnescape(name); err == nil {
		name = dec
	}
	name = strings.Map(func(r rune) rune {
		switch {
		case r == '/' || r == '\\' || r == 0:
			return '_'
		case unicode.IsControl(r):
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	name = strings.TrimLeft(name, ".")
	if name == "" {
		return ""
	}
	const max = 200
	if len(name) > max {
		ext := path.Ext(name)
		if len(ext) > 16 {
			ext = ""
		}
		base := name[:len(name)-len(ext)]
		cut := max - len(ext)
		for cut > 0 && !utf8Start(base[cut]) {
			cut--
		}
		name = base[:cut] + ext
	}
	return name
}

func utf8Start(b byte) bool { return b&0xC0 != 0x80 }

var errIdle = errors.New("connection stalled")

// ported from prism-download-manager crates/prism-engine/src/error.rs (MIT)
func transientStatus(code int) bool {
	return code >= 500 || code == http.StatusRequestTimeout || code == http.StatusTooEarly || code == http.StatusTooManyRequests
}

func retryable(err error) bool {
	var se *StatusError
	if errors.As(err, &se) {
		return transientStatus(se.Code)
	}
	if errors.Is(err, errChanged) {
		return false
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, errIdle) || errors.Is(err, io.EOF) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return true
	}
	var oe *net.OpError
	return errors.As(err, &oe)
}
