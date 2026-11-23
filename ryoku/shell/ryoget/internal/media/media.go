// Package media turns a link into something the engine can fetch: a direct
// file, a media post with its formats, or a torrent. Services resolve the sites
// they know from public endpoints; anything else goes through the generic
// probes and, when allowed, an installed yt-dlp.
package media

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"strings"
	"sync"

	"ryoget/internal/plan"
	"ryoget/internal/proto"
)

var (
	ErrUnsupported = errors.New("unsupported")
	ErrUnavailable = errors.New("unavailable")
	ErrPrivate     = errors.New("private")
	ErrRateLimited = errors.New("rate_limited")
	ErrNetwork     = errors.New("network")
	ErrInvalid     = errors.New("invalid")
	// a bot check or script-driven hand-out that only a real browser gets past
	ErrBrowserCheck = errors.New("browser_check")
)

type browserCheck struct{ msg string }

func (e browserCheck) Error() string        { return ErrBrowserCheck.Error() + ": " + e.msg }
func (e browserCheck) Is(target error) bool { return target == ErrBrowserCheck }

// As lets the engine's error mapping read the wire code without a case of its own.
func (e browserCheck) As(target any) bool {
	if p, ok := target.(**proto.Error); ok {
		*p = &proto.Error{Code: proto.CodeBrowserCheck, Message: e.msg}
		return true
	}
	return false
}

// BrowserCheck is the error for a page or file that only a browser can open.
func BrowserCheck(msg string) error { return browserCheck{msg} }

type previewKey struct{}

// Preview marks a resolve that only shows a link before it is added: it reads the site but never
// sends a form, since a post may change something there.
func Preview(ctx context.Context) context.Context {
	return context.WithValue(ctx, previewKey{}, true)
}

func previewing(ctx context.Context) bool {
	on, _ := ctx.Value(previewKey{}).(bool)
	return on
}

type refererKey struct{}

// WithReferer marks a resolve of a file found on a page: its probe sends referer and the media keeps it.
func WithReferer(ctx context.Context, referer string) context.Context {
	return context.WithValue(ctx, refererKey{}, referer)
}

func ctxReferer(ctx context.Context) string {
	r, _ := ctx.Value(refererKey{}).(string)
	return r
}

type Source struct {
	ID   string
	Name string
}

type Format struct {
	ID       string
	Kind     string
	Label    string
	Ext      string
	Codec    string
	Height   int
	FPS      int
	Bitrate  int
	Size     int64
	HasVideo bool
	HasAudio bool
	// what fetching this format alone takes; a video-only format carries only video
	Parts []plan.Part
}

type Item struct {
	Title string
	Thumb string
	Kind  string
	URL   string
	// the page the file was found on, cut to what that page may tell the file's host
	Referer string
}

type Media struct {
	Kind     plan.Kind
	URL      string
	Source   Source
	Title    string
	Author   string
	Name     string
	Thumb    string
	Duration float64
	Size     int64
	Mime     string
	// a single resumable file (KindFile) that the engine may split across connections
	Resumable bool
	Formats   []Format
	Items     []Item
	Default   string
	Note      string
	// request headers every part of this media needs (referer, user agent, cookies)
	Headers map[string]string
}

type Options struct {
	Mode        string
	Quality     string
	AudioFormat string
	Format      string
	// selected Items of a multi-item post; empty means all
	Items []int
	// an audio-only download gets Media.Thumb embedded as its cover picture
	CoverArt bool
}

type Service interface {
	ID() string
	Name() string
	Match(u *url.URL) bool
	Resolve(ctx context.Context, u *url.URL, env *plan.Env) (*Media, error)
}

var (
	registryMu sync.RWMutex
	registry   []Service
)

func Register(s Service) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry = append(registry, s)
	sort.SliceStable(registry, func(i, j int) bool { return registry[i].ID() < registry[j].ID() })
}

func Services() []Service {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return append([]Service(nil), registry...)
}

func Lookup(u *url.URL) Service {
	for _, s := range Services() {
		if s.Match(u) {
			return s
		}
	}
	return nil
}

// HostIs reports whether host is domain or one of its subdomains.
func HostIs(host string, domains ...string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	for _, d := range domains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}
