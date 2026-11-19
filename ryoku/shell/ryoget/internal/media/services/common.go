// Package services resolves the sites ryoget knows natively from their public,
// keyless endpoints. Each file registers one media.Service; every request goes
// through env.HTTP so tests replay fixtures through its transport.
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"ryoget/internal/media"
	"ryoget/internal/plan"
)

const browserUA = "Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0"

// pages like Facebook's run past a megabyte before the player data
const bodyLimit = 16 << 20

type request struct {
	method string
	url    string
	header http.Header
	body   []byte
	// statuses the caller inspects itself instead of mapping to an error
	accept []int
}

type response struct {
	status  int
	header  http.Header
	body    []byte
	final   *url.URL
	cookies []*http.Cookie
}

func client(env *plan.Env) *http.Client {
	if env != nil && env.HTTP != nil {
		return env.HTTP
	}
	return http.DefaultClient
}

func fetch(ctx context.Context, env *plan.Env, r request) (*response, error) {
	method := r.method
	if method == "" {
		method = http.MethodGet
	}
	var body io.Reader
	if r.body != nil {
		body = bytes.NewReader(r.body)
	}
	req, err := http.NewRequestWithContext(ctx, method, r.url, body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", media.ErrInvalid, err)
	}
	for k, v := range r.header {
		req.Header[k] = v
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", browserUA)
	}
	resp, err := client(env).Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: %v", media.ErrNetwork, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, bodyLimit))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: %v", media.ErrNetwork, err)
	}
	out := &response{status: resp.StatusCode, header: resp.Header, body: data, final: resp.Request.URL, cookies: resp.Cookies()}
	for _, s := range r.accept {
		if s == resp.StatusCode {
			return out, nil
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return out, statusErr(resp.StatusCode)
	}
	return out, nil
}

func statusErr(code int) error {
	switch {
	case code == http.StatusTooManyRequests:
		return fmt.Errorf("%w: HTTP %d", media.ErrRateLimited, code)
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return fmt.Errorf("%w: HTTP %d", media.ErrPrivate, code)
	case code >= 500:
		return fmt.Errorf("%w: HTTP %d", media.ErrNetwork, code)
	default:
		return fmt.Errorf("%w: HTTP %d", media.ErrUnavailable, code)
	}
}

func getBody(ctx context.Context, env *plan.Env, rawURL string, h http.Header) (*response, error) {
	return fetch(ctx, env, request{url: rawURL, header: h})
}

func getJSON(ctx context.Context, env *plan.Env, rawURL string, h http.Header, out any) error {
	r, err := fetch(ctx, env, request{url: rawURL, header: withAccept(h, "application/json")})
	if err != nil {
		return err
	}
	return decode(r.body, out)
}

func postJSON(ctx context.Context, env *plan.Env, rawURL string, h http.Header, in, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("%w: %v", media.ErrInvalid, err)
	}
	h = withAccept(h, "application/json")
	h.Set("Content-Type", "application/json")
	r, err := fetch(ctx, env, request{method: http.MethodPost, url: rawURL, header: h, body: b})
	if err != nil {
		return err
	}
	return decode(r.body, out)
}

func decode(b []byte, out any) error {
	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("%w: unexpected response: %v", media.ErrUnavailable, err)
	}
	return nil
}

func withAccept(h http.Header, accept string) http.Header {
	if h == nil {
		h = http.Header{}
	} else {
		h = h.Clone()
	}
	if h.Get("Accept") == "" {
		h.Set("Accept", accept)
	}
	return h
}

func hdr(kv ...string) http.Header {
	h := http.Header{}
	for i := 0; i+1 < len(kv); i += 2 {
		h.Set(kv[i], kv[i+1])
	}
	return h
}

// cookieHeader joins response cookies into one Cookie value for media hosts
// that check the session the page handed out.
func cookieHeader(cs []*http.Cookie, names ...string) string {
	var parts []string
	for _, c := range cs {
		if len(names) > 0 && !contains(names, c.Name) {
			continue
		}
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// navigate is what a browser sends for a top-level page load; some sites hand
// non-browser clients a shell page without the post data.
func navigate() http.Header {
	return hdr(
		"Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"Accept-Language", "en-US,en;q=0.5",
		"Sec-Fetch-Dest", "document",
		"Sec-Fetch-Mode", "navigate",
		"Sec-Fetch-Site", "none",
		"Sec-Fetch-User", "?1",
	)
}

// scriptJSON returns the JSON text that follows marker inside the page, ending
// at the matching closing brace or bracket.
func scriptJSON(page []byte, marker string) []byte {
	i := bytes.Index(page, []byte(marker))
	if i < 0 {
		return nil
	}
	rest := page[i+len(marker):]
	j := bytes.IndexAny(rest, "{[")
	if j < 0 {
		return nil
	}
	return balanced(rest[j:])
}

func balanced(b []byte) []byte {
	depth := 0
	inStr, esc := false, false
	for i, c := range b {
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{', '[':
			depth++
		case '}', ']':
			depth--
			if depth == 0 {
				return b[:i+1]
			}
		}
	}
	return nil
}

// scriptByID returns the body of <script id="id" ...>...</script>.
func scriptByID(page []byte, id string) []byte {
	re := regexp.MustCompile(`(?s)<script[^>]*\bid="` + regexp.QuoteMeta(id) + `"[^>]*>(.*?)</script>`)
	m := re.FindSubmatch(page)
	if m == nil {
		return nil
	}
	return m[1]
}

var metaRe = regexp.MustCompile(`(?i)<meta\s+[^>]*?(?:property|name)="([^"]+)"[^>]*?content="([^"]*)"`)

func metaTags(page []byte) map[string]string {
	out := map[string]string{}
	for _, m := range metaRe.FindAllSubmatch(page, -1) {
		k := strings.ToLower(string(m[1]))
		if _, ok := out[k]; !ok {
			out[k] = unescapeHTML(string(m[2]))
		}
	}
	return out
}

func unescapeHTML(s string) string { return html.UnescapeString(s) }

func first(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// oneLine trims a post caption into a title: first line, at most n runes.
func oneLine(s string, n int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	r := []rune(s)
	if len(r) > n {
		s = strings.TrimSpace(string(r[:n])) + "…"
	}
	return s
}

func heightLabel(h, fps int) string {
	if h <= 0 {
		return ""
	}
	l := strconv.Itoa(h) + "p"
	if fps > 30 {
		l += strconv.Itoa(fps)
	}
	return l
}

func kbpsLabel(bps int) string {
	if bps <= 0 {
		return ""
	}
	return strconv.Itoa((bps+500)/1000) + " kbps"
}

func extOf(rawURL, fallback string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fallback
	}
	p := u.Path
	if i := strings.LastIndexByte(p, '.'); i >= 0 && i > strings.LastIndexByte(p, '/') {
		ext := strings.ToLower(p[i+1:])
		if len(ext) >= 2 && len(ext) <= 4 {
			return ext
		}
	}
	return fallback
}

// fileVideo is a progressive file that carries picture and sound together.
func fileVideo(id, rawURL, ext string, height, fps, bitrate int, size int64, codec string, h http.Header) media.Format {
	if ext == "" {
		ext = "mp4"
	}
	if size <= 0 {
		size = -1
	}
	return media.Format{
		ID: id, Kind: "video", Label: first(heightLabel(height, fps), kbpsLabel(bitrate), strings.ToUpper(ext)),
		Ext: ext, Codec: codec, Height: height, FPS: fps, Bitrate: bitrate, Size: max(size, 0),
		HasVideo: true, HasAudio: true,
		Parts: []plan.Part{{Kind: plan.PartHTTP, URL: rawURL, Ext: ext, Size: size, Headers: cloneHeader(h)}},
	}
}

func fileAudio(id, rawURL, ext string, bitrate int, size int64, codec string, h http.Header) media.Format {
	if size <= 0 {
		size = -1
	}
	return media.Format{
		ID: id, Kind: "audio", Label: first(kbpsLabel(bitrate), strings.ToUpper(ext)),
		Ext: ext, Codec: codec, Bitrate: bitrate, Size: max(size, 0), HasAudio: true,
		Parts: []plan.Part{{Kind: plan.PartHTTP, URL: rawURL, Ext: ext, Size: size, Role: "audio", Headers: cloneHeader(h)}},
	}
}

func cloneHeader(h http.Header) http.Header {
	if len(h) == 0 {
		return nil
	}
	return h.Clone()
}

// best returns the id of the tallest format, then the highest bitrate.
func best(fs []media.Format) string {
	id, bh, bb := "", -1, -1
	for _, f := range fs {
		if f.Kind != "video" {
			continue
		}
		if f.Height > bh || (f.Height == bh && f.Bitrate > bb) {
			id, bh, bb = f.ID, f.Height, f.Bitrate
		}
	}
	if id == "" {
		for _, f := range fs {
			if f.Bitrate > bb {
				id, bb = f.ID, f.Bitrate
			}
		}
	}
	if id == "" && len(fs) > 0 {
		id = fs[0].ID
	}
	return id
}

// photoFile is a single picture post: a plain file the engine fetches whole.
func photoFile(src media.Source, imageURL, title, author string, h map[string]string) *media.Media {
	ext := extOf(imageURL, "jpg")
	return &media.Media{
		Kind: plan.KindFile, URL: imageURL, Source: src, Title: title, Author: author, Thumb: imageURL,
		Name: media.SafeName(first(title, src.Name), ext), Mime: "image/" + strings.Replace(ext, "jpg", "jpeg", 1),
		Size: -1, Resumable: true, Headers: h,
	}
}

func pathParts(u *url.URL) []string {
	var out []string
	for _, p := range strings.Split(u.Path, "/") {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// follow resolves a short link to where it points without reading the page.
// It stops at the first hop on one of hosts so the post page itself is never
// loaded.
func follow(ctx context.Context, env *plan.Env, rawURL string, hosts ...string) (*url.URL, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", media.ErrInvalid, err)
	}
	for k, v := range navigate() {
		req.Header[k] = v
	}
	req.Header.Set("User-Agent", browserUA)
	c := *client(env)
	var landed *url.URL
	c.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(hosts) > 0 && media.HostIs(next.URL.Hostname(), hosts...) {
			landed = next.URL
			return http.ErrUseLastResponse
		}
		if len(via) >= 10 {
			return errors.New("too many redirects")
		}
		return nil
	}
	resp, err := c.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%w: %v", media.ErrNetwork, err)
	}
	resp.Body.Close()
	if landed != nil {
		return landed, nil
	}
	if resp.StatusCode >= 400 {
		return nil, statusErr(resp.StatusCode)
	}
	return resp.Request.URL, nil
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
