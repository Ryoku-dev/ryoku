package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ryoget/internal/hls"
	"ryoget/internal/plan"

	i18n "ryoku-i18n"
)

// FallbackFunc resolves a link through an installed yt-dlp.
type FallbackFunc func(ctx context.Context, u *url.URL, env *plan.Env) (*Media, error)

var fallback FallbackFunc

// RegisterFallback is called by the ytdlp package's init so media does not
// import it (ytdlp builds Media values).
func RegisterFallback(f FallbackFunc) {
	registryMu.Lock()
	fallback = f
	registryMu.Unlock()
}

func Fallback() FallbackFunc {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return fallback
}

type ytdlpKey struct{}

// YTDLPAllowed reports whether the Resolve call that reached a service may
// hand the link to yt-dlp.
func YTDLPAllowed(ctx context.Context) bool {
	v, _ := ctx.Value(ytdlpKey{}).(bool)
	return v
}

// WithYTDLP marks ctx as allowing the yt-dlp fallback.
func WithYTDLP(ctx context.Context, allowed bool) context.Context {
	return context.WithValue(ctx, ytdlpKey{}, allowed)
}

type settled struct{ err error }

func (s settled) Error() string { return s.err.Error() }
func (s settled) Unwrap() error { return s.err }

// Settled marks a service error as final: the service already tried
// everything it knows, yt-dlp included, so Resolve must not try again.
func Settled(err error) error {
	if err == nil {
		return nil
	}
	return settled{err}
}

func isSettled(err error) bool {
	var s settled
	return errors.As(err, &s)
}

const (
	probeTimeout = 20 * time.Second
	pageLimit    = 2 << 20
)

// Resolve turns a pasted link into something the engine can fetch. A page with
// nothing to download comes back as a KindPage Media together with an error
// wrapping ErrUnsupported, so the caller can still show its title.
func Resolve(ctx context.Context, raw string, env *plan.Env, allowYTDLP bool) (*Media, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("%w: %s", ErrInvalid, i18n.T("Nothing to download."))
	}
	if strings.HasPrefix(strings.ToLower(raw), "magnet:") {
		return magnetMedia(raw)
	}
	if filepath.IsAbs(raw) && strings.EqualFold(filepath.Ext(raw), ".torrent") {
		if _, err := os.Stat(raw); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
		}
		return &Media{Kind: plan.KindTorrent, URL: raw, Source: torrentSource(), Name: filepath.Base(raw),
			Title: strings.TrimSuffix(filepath.Base(raw), filepath.Ext(raw))}, nil
	}
	if !strings.Contains(raw, "://") && looksLikeHost(raw) {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("%w: %s", ErrInvalid, i18n.T("This is not a link."))
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return nil, fmt.Errorf("%w: %s", ErrInvalid, i18n.T("Only http and https links can be downloaded."))
	}
	u.Fragment = ""
	ctx = WithYTDLP(ctx, allowYTDLP)

	if strings.EqualFold(path.Ext(u.Path), ".torrent") {
		return &Media{Kind: plan.KindTorrent, URL: u.String(), Source: torrentSource(), Name: safeBase(u)}, nil
	}

	if s := Lookup(u); s != nil {
		m, err := s.Resolve(ctx, u, env)
		if err == nil {
			return finish(m, u), nil
		}
		if !allowYTDLP || isSettled(err) || errors.Is(err, ErrInvalid) || errors.Is(err, ErrPrivate) {
			return nil, err
		}
		if fb := Fallback(); fb != nil {
			if m, ferr := fb(ctx, u, env); ferr == nil {
				return finish(m, u), nil
			}
		}
		return nil, err
	}
	return probeWith(ctx, u, env, allowYTDLP, "", true)
}

func finish(m *Media, u *url.URL) *Media {
	if m.URL == "" {
		m.URL = u.String()
	}
	return m
}

func torrentSource() Source { return Source{ID: "torrent", Name: "BitTorrent"} }

var hostish = regexp.MustCompile(`^[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+(:\d+)?(/.*)?$`)

func looksLikeHost(s string) bool { return !strings.ContainsAny(s, " \t") && hostish.MatchString(s) }

func magnetMedia(raw string) (*Media, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	q := u.Query()
	if !strings.HasPrefix(strings.ToLower(q.Get("xt")), "urn:btih:") && !strings.HasPrefix(strings.ToLower(q.Get("xt")), "urn:btmh:") {
		return nil, fmt.Errorf("%w: %s", ErrInvalid, i18n.T("This magnet link has no torrent hash."))
	}
	name := q.Get("dn")
	return &Media{Kind: plan.KindTorrent, URL: raw, Source: torrentSource(), Title: name, Name: SafeName(name, "")}, nil
}

// Some file hosts drop browser user agents and some sites refuse anything
// else, so the probe tries both and the media keeps the one that worked.
var probeAgents = []string{browserUA, plainUA}

func probeGet(ctx context.Context, u *url.URL, env *plan.Env, referer string) (*http.Response, string, error) {
	var lastErr error
	var last *http.Response
	for i, ua := range probeAgents {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, "", fmt.Errorf("%w: %v", ErrInvalid, err)
		}
		req.Header.Set("Range", "bytes=0-")
		req.Header.Set("User-Agent", ua)
		req.Header.Set("Accept", "*/*")
		if referer != "" {
			req.Header.Set("Referer", referer)
		}
		resp, err := httpClient(env).Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, "", ctx.Err()
			}
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusForbidden || i == len(probeAgents)-1 {
			if last != nil {
				last.Body.Close()
			}
			return resp, ua, nil
		}
		if last != nil {
			last.Body.Close()
		}
		last = resp
	}
	if last != nil {
		return last, probeAgents[0], nil
	}
	return nil, "", fmt.Errorf("%w: %v", ErrNetwork, lastErr)
}

// probeWith reads the start of a link, scan lets a page without media offer its files or download form.
func probeWith(ctx context.Context, u *url.URL, env *plan.Env, allowYTDLP bool, referer string, scan bool) (*Media, error) {
	if r := ctxReferer(ctx); r != "" {
		if referer == "" {
			referer = r
		}
		// what a page leads to is sent that page as referer, never the one this link came with
		ctx = WithReferer(ctx, "")
	}
	pctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	resp, ua, err := probeGet(pctx, u, env, referer)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, err
	}
	defer resp.Body.Close()
	ctype := strings.ToLower(resp.Header.Get("Content-Type"))
	mt, _, _ := mime.ParseMediaType(ctype)
	if browserCheckHeader(resp.Header) || (resp.StatusCode >= 400 && (mt == "text/html" || mt == "application/xhtml+xml")) {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, challengeLimit+1))
		if ChallengePage(resp.Header, body) {
			if allowYTDLP {
				if m, ferr := tryFallback(ctx, u, env); ferr == nil {
					return m, nil
				}
			}
			return nil, BrowserCheckError()
		}
	}
	switch {
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, resp.Status)
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		if allowYTDLP {
			if m, ferr := tryFallback(ctx, u, env); ferr == nil {
				return m, nil
			}
		}
		return nil, fmt.Errorf("%w: %s", ErrPrivate, resp.Status)
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, fmt.Errorf("%w: %s", ErrRateLimited, resp.Status)
	case resp.StatusCode >= 400:
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, resp.Status)
	}
	withHeaders := func(m *Media, err error) (*Media, error) {
		if m != nil && m.Source.ID != "ytdlp" {
			if m.Headers == nil {
				m.Headers = map[string]string{}
			}
			if _, set := m.Headers["User-Agent"]; !set {
				m.Headers["User-Agent"] = ua
			}
			if _, set := m.Headers["Referer"]; !set && referer != "" {
				m.Headers["Referer"] = referer
			}
		}
		return m, err
	}

	head := make([]byte, 512)
	n, _ := io.ReadFull(resp.Body, head)
	head = head[:n]
	if mt == "" {
		mt = strings.ToLower(http.DetectContentType(head))
		mt, _, _ = mime.ParseMediaType(mt)
	}

	if isHLS(mt, u, head) {
		h := http.Header{"User-Agent": {ua}}
		if referer != "" {
			h.Set("Referer", referer)
		}
		return withHeaders(hlsMedia(ctx, u, env, h, "", ""))
	}
	if mt == "text/html" || mt == "application/xhtml+xml" {
		rest, _ := io.ReadAll(io.LimitReader(resp.Body, pageLimit))
		page := append(head, rest...)
		if ChallengePage(resp.Header, page) {
			if allowYTDLP {
				if m, ferr := tryFallback(ctx, u, env); ferr == nil {
					return m, nil
				}
			}
			return nil, BrowserCheckError()
		}
		return withHeaders(pageMedia(ctx, u, resp.Request.URL, env, page, ua, allowYTDLP, scan))
	}
	if mt == "application/x-bittorrent" {
		return withHeaders(&Media{Kind: plan.KindTorrent, URL: u.String(), Source: torrentSource(), Name: safeBase(u)}, nil)
	}

	size, resumable := contentSize(resp)
	name := dispositionName(resp.Header.Get("Content-Disposition"))
	if name == "" {
		name = path.Base(resp.Request.URL.Path)
		if uq, err := url.PathUnescape(name); err == nil {
			name = uq
		}
		if name == "/" || name == "." {
			name = ""
		}
	}
	name = SafeName(name, "")
	if name == "" {
		name = SafeName(u.Hostname(), "")
	}
	if filepath.Ext(name) == "" && mt != "application/octet-stream" && mt != "binary/octet-stream" {
		if exts, _ := mime.ExtensionsByType(mt); len(exts) > 0 {
			name += preferredExt(mt, exts)
		}
	}
	return withHeaders(&Media{
		Kind:      plan.KindFile,
		URL:       u.String(),
		Source:    Source{ID: "direct", Name: u.Hostname()},
		Title:     strings.TrimSuffix(name, filepath.Ext(name)),
		Name:      name,
		Size:      size,
		Mime:      mt,
		Resumable: resumable,
	}, nil)
}

func tryFallback(ctx context.Context, u *url.URL, env *plan.Env) (*Media, error) {
	fb := Fallback()
	if fb == nil {
		return nil, ErrUnsupported
	}
	m, err := fb(ctx, u, env)
	if err != nil {
		return nil, err
	}
	return finish(m, u), nil
}

func httpClient(env *plan.Env) *http.Client {
	if env != nil && env.HTTP != nil {
		return env.HTTP
	}
	return http.DefaultClient
}

const (
	browserUA = "Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0"
	plainUA   = "ryoget/1 (+https://ryoku.dev)"
)

func isHLS(mt string, u *url.URL, head []byte) bool {
	switch mt {
	case "application/vnd.apple.mpegurl", "application/x-mpegurl", "audio/mpegurl", "audio/x-mpegurl":
		return true
	}
	if bytes.HasPrefix(bytes.TrimLeft(head, "\ufeff \r\n\t"), []byte("#EXTM3U")) {
		return true
	}
	return strings.EqualFold(path.Ext(u.Path), ".m3u8") && !strings.HasPrefix(mt, "video/")
}

func contentSize(resp *http.Response) (int64, bool) {
	if resp.StatusCode == http.StatusPartialContent {
		cr := resp.Header.Get("Content-Range")
		if i := strings.LastIndexByte(cr, '/'); i >= 0 {
			if n, err := strconv.ParseInt(strings.TrimSpace(cr[i+1:]), 10, 64); err == nil {
				return n, true
			}
		}
		return -1, true
	}
	size := int64(-1)
	if resp.ContentLength >= 0 {
		size = resp.ContentLength
	}
	return size, strings.EqualFold(resp.Header.Get("Accept-Ranges"), "bytes") && size > 0
}

func dispositionName(cd string) string {
	if cd == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(cd)
	if err != nil {
		if i := strings.Index(strings.ToLower(cd), "filename="); i >= 0 {
			return strings.Trim(strings.TrimSpace(strings.SplitN(cd[i+9:], ";", 2)[0]), `"'`)
		}
		return ""
	}
	return filepath.Base(params["filename"])
}

func preferredExt(mt string, exts []string) string {
	want := map[string]string{
		"video/mp4": ".mp4", "audio/mpeg": ".mp3", "image/jpeg": ".jpg", "text/plain": ".txt",
		"application/zip": ".zip", "application/pdf": ".pdf", "audio/mp4": ".m4a",
		"application/x-iso9660-image": ".iso", "application/gzip": ".gz",
	}
	if e, ok := want[mt]; ok {
		return e
	}
	return exts[0]
}

func safeBase(u *url.URL) string {
	b := path.Base(u.Path)
	if s, err := url.PathUnescape(b); err == nil {
		b = s
	}
	return SafeName(b, "")
}

var (
	metaTag  = regexp.MustCompile(`(?is)<meta\s[^>]*>`)
	attrRe   = regexp.MustCompile(`(?is)([a-z:_-]+)\s*=\s*("([^"]*)"|'([^']*)'|([^\s>]+))`)
	titleTag = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
)

func metaProps(page []byte) map[string]string {
	props := map[string]string{}
	for _, tag := range metaTag.FindAll(page, -1) {
		attrs := tagAttrs(tag)
		key := attrs["property"]
		if key == "" {
			key = attrs["name"]
		}
		key = strings.ToLower(key)
		if key == "" {
			continue
		}
		if _, seen := props[key]; !seen {
			props[key] = attrs["content"]
		}
	}
	return props
}

// pageMedia reads a page fetched from u, doc is where redirects ended: the referer, and what links resolve against unless a <base href> says otherwise.
func pageMedia(ctx context.Context, u, doc *url.URL, env *plan.Env, page []byte, ua string, allowYTDLP, scan bool) (*Media, error) {
	base := pageBase(doc, page)
	props := metaProps(page)
	title := props["og:title"]
	if title == "" {
		if m := titleTag.FindSubmatch(page); m != nil {
			title = strings.TrimSpace(html.UnescapeString(string(m[1])))
		}
	}
	thumb := absURL(base, props["og:image"])
	site := props["og:site_name"]
	if site == "" {
		site = u.Hostname()
	}
	src := Source{ID: "web", Name: site}

	for _, kind := range []string{"video", "audio"} {
		ref := firstOf(props["og:"+kind+":secure_url"], props["og:"+kind+":url"], props["og:"+kind])
		if ref == "" {
			continue
		}
		typ := strings.ToLower(props["og:"+kind+":type"])
		if strings.HasPrefix(typ, "text/html") || strings.Contains(typ, "shockwave") {
			continue
		}
		mu, err := url.Parse(absURL(base, ref))
		if err != nil || (mu.Scheme != "http" && mu.Scheme != "https") {
			continue
		}
		if typ == "application/x-mpegurl" || typ == "application/vnd.apple.mpegurl" || strings.EqualFold(path.Ext(mu.Path), ".m3u8") {
			m, err := hlsMedia(ctx, mu, env, http.Header{"Referer": {doc.String()}, "User-Agent": {ua}}, title, thumb)
			if err == nil {
				m.URL = u.String()
				m.Source = src
				return m, nil
			}
			continue
		}
		ext := strings.TrimPrefix(path.Ext(mu.Path), ".")
		if ext == "" {
			if exts, _ := mime.ExtensionsByType(typ); len(exts) > 0 {
				ext = strings.TrimPrefix(exts[0], ".")
			}
		}
		if ext == "" {
			ext = map[string]string{"video": "mp4", "audio": "mp3"}[kind]
		}
		f := Format{
			ID: "og-" + kind, Kind: kind, Ext: ext, Label: strings.ToUpper(ext),
			HasVideo: kind == "video", HasAudio: true,
			Height: atoi(props["og:video:height"]),
			Parts: []plan.Part{{Kind: plan.PartHTTP, URL: mu.String(), Ext: ext, Size: -1,
				Headers: http.Header{"Referer": {doc.String()}, "User-Agent": {ua}}}},
		}
		return &Media{Kind: plan.KindMedia, URL: u.String(), Source: src, Title: title, Thumb: thumb,
			Formats: []Format{f}, Default: f.ID}, nil
	}

	if allowYTDLP {
		if m, err := tryFallback(ctx, u, env); err == nil {
			return m, nil
		} else if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	pageOnly := &Media{Kind: plan.KindPage, URL: u.String(), Source: src, Title: title, Thumb: thumb}
	if scan {
		switch files := pageFiles(doc, base, page); len(files) {
		case 0:
		case 1:
			fu, _ := url.Parse(files[0].URL)
			m, err := probeFile(ctx, fu, env, files[0].Referer)
			if err != nil {
				return pageOnly, err
			}
			return m, nil
		default:
			return &Media{Kind: plan.KindMedia, URL: u.String(), Source: src, Title: title, Thumb: thumb, Items: files}, nil
		}
		if f, ok := downloadForm(base, page); ok {
			if f.browser {
				return pageOnly, needsBrowserErr()
			}
			// a preview names the page as a file and leaves the post to the download itself
			if f.method == http.MethodPost && previewing(ctx) {
				return &Media{Kind: plan.KindFile, URL: u.String(), Source: src, Title: title, Thumb: thumb}, nil
			}
			m, err := followForm(ctx, doc, env, f, ua)
			if err != nil {
				return pageOnly, err
			}
			return m, nil
		}
	}
	return pageOnly, fmt.Errorf("%w: %s", ErrUnsupported, i18n.T("No video, audio or file was found on this page."))
}

func hlsMedia(ctx context.Context, u *url.URL, env *plan.Env, headers http.Header, title, thumb string) (*Media, error) {
	vs, err := hls.Variants(ctx, env, u.String(), headers)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	if title == "" {
		title = strings.TrimSuffix(safeBase(u), ".m3u8")
	}
	m := &Media{Kind: plan.KindMedia, URL: u.String(), Source: Source{ID: "direct", Name: u.Hostname()},
		Title: title, Thumb: thumb}
	m.Formats = HLSFormats(vs, headers)
	if len(m.Formats) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrUnavailable, i18n.T("The stream has no playable variants."))
	}
	return m, nil
}

// HLSFormats turns the variants of a playlist into formats: one per variant,
// paired with its separate audio rendition when the master names one, plus the
// audio renditions on their own.
func HLSFormats(vs []hls.Variant, headers http.Header) []Format {
	var out []Format
	seenAudio := map[string]bool{}
	for _, v := range vs {
		if v.AudioOnly {
			if seenAudio[v.URL] {
				continue
			}
			seenAudio[v.URL] = true
			out = append(out, Format{
				ID: "hls-audio-" + strconv.Itoa(len(out)), Kind: "audio", Label: firstOf(v.Name, "Audio"),
				Ext: "m4a", Codec: v.AudioCodec, Bitrate: v.Bandwidth, HasAudio: true,
				Parts: []plan.Part{{Kind: plan.PartHLS, URL: v.URL, Ext: "ts", Size: -1, Headers: headers.Clone(), Role: "audio"}},
			})
			continue
		}
		label := "HLS"
		if v.Height > 0 {
			label = strconv.Itoa(v.Height) + "p"
		} else if v.Bandwidth > 0 {
			label = strconv.Itoa(v.Bandwidth/1000) + " kbps"
		}
		f := Format{
			ID: "hls-" + strconv.Itoa(len(out)), Kind: "video", Label: label, Ext: "mp4",
			Codec: v.VideoCodec, Height: v.Height, FPS: int(v.FPS + 0.5), Bitrate: v.Bandwidth,
			HasVideo: !v.NoVideo, HasAudio: v.AudioURL == "" && !v.NoAudio,
		}
		role := ""
		if v.AudioURL != "" {
			role = "video"
		}
		f.Parts = []plan.Part{{Kind: plan.PartHLS, URL: v.URL, Ext: "ts", Size: -1, Headers: headers.Clone(), Role: role}}
		if v.AudioURL != "" {
			f.HasAudio = true
			f.Parts = append(f.Parts, plan.Part{Kind: plan.PartHLS, URL: v.AudioURL, Ext: "ts", Size: -1, Headers: headers.Clone(), Role: "audio"})
		}
		if !f.HasVideo {
			f.Kind = "audio"
		}
		if v.AudioCodec != "" {
			f.Codec = strings.Trim(f.Codec+","+v.AudioCodec, ",")
		}
		out = append(out, f)
	}
	return out
}

func absURL(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	r, err := base.Parse(ref)
	if err != nil {
		return ""
	}
	return r.String()
}

func firstOf(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
