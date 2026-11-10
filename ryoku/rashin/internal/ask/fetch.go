package ask

import (
	"context"
	"crypto/tls"
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
)

const fetchLimit = 200 * 1024

var (
	htmlNoise = regexp.MustCompile(`(?is)<(script|style|noscript)\b[^>]*>.*?</[a-z]+[ \t\r\n]*>`)
	htmlTag   = regexp.MustCompile(`(?s)<[^>]+>`)
	htmlSpace = regexp.MustCompile(`[ \t\r\f\v]+`)
	htmlLines = regexp.MustCompile(`\n\s*\n+`)
)

func fetchPublic(parent context.Context, rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" || parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("%s", i18n.T("Only public HTTP and HTTPS URLs can be fetched."))
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	if _, err := publicAddresses(ctx, parsed.Hostname()); err != nil {
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
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("%s", i18n.T("The URL redirected too many times."))
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("%s", i18n.T("The URL redirected outside HTTP or HTTPS."))
			}
			_, checkErr := publicAddresses(req.Context(), req.URL.Hostname())
			return checkErr
		},
	}
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

func publicAddress(address netip.Addr) bool {
	if !address.IsValid() || address.IsUnspecified() || address.IsLoopback() || address.IsPrivate() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsMulticast() {
		return false
	}
	if prefix, err := netip.ParsePrefix("100.64.0.0/10"); err == nil && prefix.Contains(address) {
		return false
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
