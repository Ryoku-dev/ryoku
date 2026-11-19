package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"

	"ryoku-i18n"
)

const (
	maxBody     = 8 << 20
	maxPageBody = 1 << 20
)

var (
	errTooLarge = errors.New("response too large")
	errBadKey   = errors.New("bad key")
)

type statusError struct{ code int }

func (e statusError) Error() string { return "http " + strconv.Itoa(e.code) }

func errorText(err error) string {
	var se statusError
	var ne net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return i18n.T("Timed out")
	case errors.As(err, &se):
		return i18n.Tf("Server answered %d", se.code)
	case errors.As(err, &ne):
		return i18n.T("Unreachable")
	default:
		return i18n.T("Unavailable")
	}
}

// identity names the program honestly on every request (Wikimedia's
// user-agent policy, crates.io and GitHub require it); impersonating a
// browser changed no outcome in the probes and reads as circumvention.
type identity struct {
	base http.RoundTripper
	ua   string
}

func (t identity) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.Header.Set("User-Agent", t.ua)
	return t.base.RoundTrip(r2)
}

func withIdentity(c *http.Client, ua string) *http.Client {
	out := *c
	base := c.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	out.Transport = identity{base: base, ua: ua}
	return &out
}

func newClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 4 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          64,
			MaxIdleConnsPerHost:   4,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   4 * time.Second,
			ResponseHeaderTimeout: 6 * time.Second,
		},
		CheckRedirect: checkRedirect,
	}
}

// newPageClient fetches arbitrary result pages and icons, so it refuses to
// dial anything that is not a public address.
func newPageClient() *http.Client {
	dialer := &net.Dialer{Timeout: 4 * time.Second, KeepAlive: 30 * time.Second, Control: publicOnly}
	return &http.Client{
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConnsPerHost:   2,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   4 * time.Second,
			ResponseHeaderTimeout: 6 * time.Second,
		},
		CheckRedirect: checkRedirect,
	}
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return errors.New("too many redirects")
	}
	if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
		return errors.New("redirect to a non-web url")
	}
	return nil
}

var cgnat = net.IPNet{IP: net.IPv4(100, 64, 0, 0), Mask: net.CIDRMask(10, 32)}

func publicOnly(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !publicIP(ip) {
		return fmt.Errorf("refusing non-public address %s", host)
	}
	return nil
}

func publicIP(ip net.IP) bool {
	return !(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() || cgnat.Contains(ip))
}

// acceptLanguage turns a locale into a browser-like header: "en-US" gives
// "en-US,en;q=0.9", "de" gives "de,en;q=0.8".
func acceptLanguage(locale string) string {
	primary, region, _ := strings.Cut(locale, "-")
	if primary == "" {
		return "en-US,en;q=0.9"
	}
	var b strings.Builder
	if region != "" {
		b.WriteString(primary + "-" + region + "," + primary + ";q=0.9")
	} else {
		b.WriteString(primary)
	}
	if primary != "en" {
		b.WriteString(",en;q=0.8")
	}
	return b.String()
}

type request struct {
	// governed engine id; "" for page and icon fetches
	engine  string
	method  string
	url     string
	body    []byte
	ctype   string
	accept  string
	locale  string
	headers map[string]string
	client  *http.Client
	limit   int64
	// keep the first limit bytes of a longer body instead of failing
	truncate bool
}

type response struct {
	status int
	header http.Header
	body   []byte
	url    *url.URL
}

// fetch sends one request through the engine's budget and health: a paused
// engine is never asked, a block pauses it, and the answer counts as health.
func (e *Service) fetch(ctx context.Context, r request) (*response, error) {
	timeout := defaultWireTimeout
	if r.engine != "" {
		f := flightOf(ctx)
		release, err := e.gov.acquire(ctx, r.engine, f.notifyWaiting)
		if err != nil {
			return nil, err
		}
		defer release()
		f.wire()
		if d := e.gov.def(r.engine); d != nil && d.timeout > 0 {
			timeout = d.timeout
		}
	}
	wctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := e.send(wctx, r)
	if err != nil {
		return nil, err
	}
	if r.engine != "" {
		if blocked, retry := classify(r.engine, resp, e.now()); blocked {
			return nil, &pausedError{engine: r.engine, until: e.gov.strike(r.engine, retry)}
		}
		if resp.status < 400 || resp.status == http.StatusNotFound {
			e.gov.ok(r.engine)
		}
	}
	return resp, nil
}

func (e *Service) send(ctx context.Context, r request) (*response, error) {
	if r.method == "" {
		r.method = http.MethodGet
	}
	var body io.Reader
	if r.body != nil {
		body = bytes.NewReader(r.body)
	}
	req, err := http.NewRequestWithContext(ctx, r.method, r.url, body)
	if err != nil {
		return nil, err
	}
	if r.accept != "" {
		req.Header.Set("Accept", r.accept)
	}
	if r.ctype != "" {
		req.Header.Set("Content-Type", r.ctype)
	}
	req.Header.Set("Accept-Language", acceptLanguage(r.locale))
	for k, v := range r.headers {
		req.Header.Set(k, v)
	}
	client := r.client
	if client == nil {
		client = e.client
	}
	resp, err := client.Do(req)
	// DuckDuckGo often resets a fresh connection; one more try is cheap
	if err != nil && errors.Is(err, syscall.ECONNRESET) && r.body == nil && ctx.Err() == nil {
		resp, err = client.Do(req.Clone(ctx))
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	limit := r.limit
	if limit == 0 {
		limit = maxBody
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		if !r.truncate {
			return nil, errTooLarge
		}
		data = data[:limit]
	}
	return &response{status: resp.StatusCode, header: resp.Header, body: data, url: resp.Request.URL}, nil
}

// getJSON decodes a 2xx body into v.
func (e *Service) getJSON(ctx context.Context, r request, v any) (*response, error) {
	if r.accept == "" {
		r.accept = "application/json"
	}
	resp, err := e.fetch(ctx, r)
	if err != nil {
		return nil, err
	}
	if err := statusErr(resp.status); err != nil {
		return resp, err
	}
	if err := json.Unmarshal(resp.body, v); err != nil {
		return resp, fmt.Errorf("decode %s: %w", r.url, err)
	}
	return resp, nil
}

func statusErr(code int) error {
	if code >= 200 && code < 300 {
		return nil
	}
	return statusError{code}
}
