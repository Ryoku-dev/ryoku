// Package youtube reads YouTube from its innertube API: video metadata and
// direct stream URLs through the visionOS player context (the one client that
// still hands out plain URLs without a PO token or the JS player), playlists
// as their first video, and search.
package youtube

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client contexts as yt-dlp (public domain) sends them; see
// yt_dlp/extractor/youtube/_base.py INNERTUBE_CLIENTS.
type clientContext struct {
	name    string
	id      int
	version string
	ua      string
	extra   map[string]string
}

var (
	visionOS = clientContext{
		name: "VISIONOS", id: 101, version: "1.02",
		ua: "Mozilla/5.0 (Macintosh; Intel Mac OS X 15_7_3) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.0 Safari/605.1.15",
		extra: map[string]string{
			"deviceMake": "Apple", "deviceModel": "RealityDevice17,1",
			"osName": "visionOS", "osVersion": "26.5.23O471",
		},
	}
	webClient = clientContext{
		name: "WEB", id: 1, version: "2.20260708.00.00",
		ua: "Mozilla/5.0 (X11; Linux x86_64; rv:140.0) Gecko/20100101 Firefox/140.0",
	}
)

const (
	origin        = "https://www.youtube.com"
	visitorTTL    = 6 * time.Hour
	refreshCooldn = 5 * time.Minute
	apiTimeout    = 20 * time.Second
	responseLimit = 16 << 20
)

// Client talks to innertube. The zero value is not usable; use New.
type Client struct {
	http     *http.Client
	base     string
	cacheDir string

	mu          sync.Mutex
	visitor     string
	visitorAt   time.Time
	lastTry     time.Time
	lastRefresh time.Time
}

// New returns a client on hc (nil: a default with a timeout). cacheDir, when
// set, keeps the anonymous visitor identity across restarts.
func New(hc *http.Client, cacheDir string) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: apiTimeout}
	}
	return &Client{http: hc, base: origin, cacheDir: cacheDir}
}

var (
	sharedMu sync.Mutex
	shared   = map[*http.Client]*Client{}
)

// For returns the process-wide client for hc, so the visitor identity is
// fetched once and reused by every resolve and search.
func For(hc *http.Client, cacheDir string) *Client {
	sharedMu.Lock()
	defer sharedMu.Unlock()
	if c, ok := shared[hc]; ok {
		return c
	}
	c := New(hc, cacheDir)
	shared[hc] = c
	return c
}

var (
	errNoVisitor = errors.New("youtube: no visitor data")
	visitorRe    = regexp.MustCompile(`"(Cg[st][A-Za-z0-9_%=-]{10,})"`)
)

// Visitor returns the anonymous visitor identity, fetching it on first use.
func (c *Client) Visitor(ctx context.Context) string {
	c.mu.Lock()
	v, at := c.visitor, c.visitorAt
	c.mu.Unlock()
	if v != "" && time.Since(at) < visitorTTL {
		return v
	}
	if v == "" {
		if cv, cat := c.loadVisitor(); cv != "" && time.Since(cat) < visitorTTL {
			c.setVisitor(cv, cat, false)
			return cv
		}
	}
	c.mu.Lock()
	recent := time.Since(c.lastTry) < time.Minute
	if !recent {
		c.lastTry = time.Now()
	}
	c.mu.Unlock()
	if recent {
		return v
	}
	if nv, err := c.fetchVisitor(ctx); err == nil {
		return nv
	}
	return v
}

// RefreshVisitor replaces the visitor identity after a bot gate, at most once
// every five minutes.
func (c *Client) RefreshVisitor(ctx context.Context) bool {
	c.mu.Lock()
	if time.Since(c.lastRefresh) < refreshCooldn {
		c.mu.Unlock()
		return false
	}
	c.lastRefresh = time.Now()
	c.mu.Unlock()
	_, err := c.fetchVisitor(ctx)
	return err == nil
}

func (c *Client) fetchVisitor(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/sw.js_data", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", webClient.ua)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("youtube: sw.js_data: %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	v := ParseVisitor(data)
	if v == "" {
		return "", errNoVisitor
	}
	c.setVisitor(v, time.Now(), true)
	return v, nil
}

func (c *Client) setVisitor(v string, at time.Time, persist bool) {
	c.mu.Lock()
	c.visitor, c.visitorAt = v, at
	c.mu.Unlock()
	if persist && c.cacheDir != "" {
		data, _ := json.Marshal(map[string]any{"visitor": v, "at": at.Unix()})
		if os.MkdirAll(c.cacheDir, 0o700) == nil {
			tmp := filepath.Join(c.cacheDir, ".youtube-visitor.json.tmp")
			if os.WriteFile(tmp, data, 0o600) == nil {
				os.Rename(tmp, filepath.Join(c.cacheDir, "youtube-visitor.json"))
			}
		}
	}
}

func (c *Client) loadVisitor() (string, time.Time) {
	if c.cacheDir == "" {
		return "", time.Time{}
	}
	data, err := os.ReadFile(filepath.Join(c.cacheDir, "youtube-visitor.json"))
	if err != nil {
		return "", time.Time{}
	}
	var s struct {
		Visitor string `json:"visitor"`
		At      int64  `json:"at"`
	}
	if json.Unmarshal(data, &s) != nil {
		return "", time.Time{}
	}
	return s.Visitor, time.Unix(s.At, 0)
}

// ParseVisitor finds the visitor identity in a sw.js_data body (or any page
// carrying ytcfg VISITOR_DATA).
func ParseVisitor(data []byte) string {
	if i := bytes.IndexByte(data, '['); i >= 0 {
		var root any
		if json.Unmarshal(data[i:], &root) == nil {
			if v := findVisitor(root); v != "" {
				return v
			}
		}
	}
	if m := visitorRe.FindSubmatch(data); m != nil {
		return string(m[1])
	}
	return ""
}

func findVisitor(v any) string {
	switch x := v.(type) {
	case string:
		if len(x) > 16 && (strings.HasPrefix(x, "Cgs") || strings.HasPrefix(x, "Cgt")) {
			return x
		}
	case []any:
		for _, e := range x {
			if s := findVisitor(e); s != "" {
				return s
			}
		}
	case map[string]any:
		for _, e := range x {
			if s := findVisitor(e); s != "" {
				return s
			}
		}
	}
	return ""
}

type apiError struct {
	status int
}

func (e apiError) Error() string { return "youtube: innertube " + strconv.Itoa(e.status) }

// call posts body to /youtubei/v1/<endpoint> as cc and decodes the answer.
func (c *Client) call(ctx context.Context, cc clientContext, endpoint string, body map[string]any, hl string, out any) error {
	visitor := c.Visitor(ctx)
	client := map[string]any{
		"clientName": cc.name, "clientVersion": cc.version, "userAgent": cc.ua,
		"hl": firstOf(hl, "en"), "timeZone": "UTC", "utcOffsetMinutes": 0,
	}
	for k, v := range cc.extra {
		client[k] = v
	}
	if visitor != "" {
		client["visitorData"] = visitor
	}
	body["context"] = map[string]any{"client": client}
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/youtubei/v1/"+endpoint+"?prettyPrint=false", bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", cc.ua)
	req.Header.Set("X-YouTube-Client-Name", strconv.Itoa(cc.id))
	req.Header.Set("X-YouTube-Client-Version", cc.version)
	req.Header.Set("Origin", origin)
	if visitor != "" {
		req.Header.Set("X-Goog-Visitor-Id", visitor)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return apiError{resp.StatusCode}
	}
	return json.NewDecoder(io.LimitReader(resp.Body, responseLimit)).Decode(out)
}

func firstOf(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
