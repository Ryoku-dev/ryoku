// Package web exposes bounded, public-address-only text fetching to the engine.
package web

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

const fetchLimit = 200 * 1024

var (
	htmlNoise = regexp.MustCompile(`(?is)<(script|style|noscript)\b[^>]*>.*?</[a-z]+[ \t\r\n]*>`)
	htmlTag   = regexp.MustCompile(`(?s)<[^>]+>`)
	htmlSpace = regexp.MustCompile(`[ \t\r\f\v]+`)
	htmlLines = regexp.MustCompile(`\n\s*\n+`)
)

// Tools returns the public HTTP text fetcher for the engine registry.
func Tools() []tool.Tool { return []tool.Tool{fetchTool{}} }

type fetchTool struct{}

func (fetchTool) Name() string    { return tool.FetchURL }
func (fetchTool) Kind() tool.Kind { return protocol.ToolFetch }
func (fetchTool) ReadOnly() bool  { return true }
func (fetchTool) Spec(tool.Profile) llm.Tool {
	return llm.Tool{Name: tool.FetchURL, Description: "Fetch a public HTTP or HTTPS page as text, at most 200 KiB.", Parameters: json.RawMessage(`{"type":"object","properties":{"url":{"type":"string"}},"required":["url"],"additionalProperties":false}`)}
}
func (fetchTool) Present(raw json.RawMessage) tool.Presentation {
	var a struct {
		URL string `json:"url"`
	}
	_ = json.Unmarshal(raw, &a)
	return tool.Presentation{Title: i18n.T("Fetching a public page"), Input: a.URL}
}
func (fetchTool) Run(ctx context.Context, call tool.Call, env tool.Env) tool.Result {
	var a struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(call.Args, &a); err != nil {
		return tool.Failed(err.Error())
	}
	if strings.TrimSpace(a.URL) == "" {
		return tool.Failed(i18n.T("A URL is required."))
	}
	text, err := fetchPublic(ctx, a.URL)
	if err != nil {
		return tool.Failed(err.Error())
	}
	limit := env.Limits
	if limit.Told <= 0 {
		limit = env.Profile.Limits()
	}
	if len(text) > limit.Told {
		text = tool.HeadTail(text, limit.Head, limit.Tail)
	}
	return tool.Result{Text: text}
}

func fetchPublic(parent context.Context, rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return "", fmt.Errorf("%s", i18n.T("Only public HTTP and HTTPS URLs can be fetched."))
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	if _, err = publicAddresses(ctx, parsed.Hostname()); err != nil {
		return "", err
	}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, splitErr := net.SplitHostPort(address)
			if splitErr != nil {
				return nil, splitErr
			}
			addresses, resolveErr := publicAddresses(ctx, host)
			if resolveErr != nil {
				return nil, resolveErr
			}
			dialer := net.Dialer{Timeout: 5 * time.Second}
			var lastErr error
			for _, ip := range addresses {
				conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if dialErr == nil {
					return conn, nil
				}
				lastErr = dialErr
			}
			return nil, lastErr
		},
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	}
	client := &http.Client{Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("%s", i18n.T("The URL redirected too many times."))
		}
		if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
			return fmt.Errorf("%s", i18n.T("The URL redirected outside HTTP or HTTPS."))
		}
		if req.URL.User != nil {
			return fmt.Errorf("%s", i18n.T("Only public HTTP and HTTPS URLs can be fetched."))
		}
		_, checkErr := publicAddresses(req.Context(), req.URL.Hostname())
		return checkErr
	}}
	defer transport.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/html, text/plain, application/json, application/xml;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf(i18n.T("The page returned HTTP %d."), resp.StatusCode)
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "text/") && !strings.Contains(contentType, "json") && !strings.Contains(contentType, "xml") {
		return "", fmt.Errorf(i18n.T("The page returned unsupported content type %s."), contentType)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, fetchLimit+1))
	if err != nil {
		return "", err
	}
	truncated := len(body) > fetchLimit
	if truncated {
		body = body[:fetchLimit]
	}
	text := string(body)
	if strings.Contains(contentType, "html") || strings.Contains(strings.ToLower(text[:min(len(text), 512)]), "<html") {
		text = htmlText(text)
	}
	text = strings.TrimSpace(text)
	if truncated {
		text += "\n[page trimmed at 200 KiB]"
	}
	return text, nil
}
func publicAddresses(ctx context.Context, host string) ([]netip.Addr, error) {
	if host == "" {
		return nil, fmt.Errorf("%s", i18n.T("The URL has no host."))
	}
	var addresses []netip.Addr
	if literal, err := netip.ParseAddr(strings.Trim(host, "[]")); err == nil {
		addresses = append(addresses, literal.Unmap())
	} else {
		resolved, resolveErr := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if resolveErr != nil {
			return nil, resolveErr
		}
		for _, address := range resolved {
			addresses = append(addresses, address.Unmap())
		}
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("%s", i18n.T("The URL host has no address."))
	}
	for _, address := range addresses {
		if !publicAddress(address) {
			return nil, fmt.Errorf(i18n.T("Refused private or local address %s."), address.String())
		}
	}
	return addresses, nil
}

var excludedPublicRanges = [...]netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001:db8::/32"),
}

func publicAddress(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLinkLocalUnicast() {
		return false
	}
	for _, prefix := range excludedPublicRanges {
		if prefix.Contains(address) {
			return false
		}
	}
	return true
}
func htmlText(body string) string {
	body = htmlNoise.ReplaceAllString(body, "\n")
	body = strings.NewReplacer("<br>", "\n", "<br/>", "\n", "<br />", "\n", "</p>", "\n", "</div>", "\n", "</li>", "\n").Replace(body)
	body = htmlTag.ReplaceAllString(body, " ")
	body = html.UnescapeString(body)
	body = htmlSpace.ReplaceAllString(body, " ")
	return htmlLines.ReplaceAllString(body, "\n\n")
}
