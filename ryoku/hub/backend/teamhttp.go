package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func teamURL() (string, error) {
	s := os.Getenv("RYOKU_TEAM_URL")
	if s == "" {
		return "https://team.ryoku.dev", nil
	}
	u, e := url.Parse(s)
	if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", errors.New("invalid team URL")
	}
	if u.Scheme != "https" && (u.Scheme != "http" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost")) {
		return "", errors.New("team URL requires HTTPS or loopback HTTP")
	}
	return strings.TrimRight(s, "/"), nil
}
func teamGet(path string, limit int64) ([]byte, int, error) {
	return teamSend(http.MethodGet, path, "", nil, nil, limit)
}
func teamSend(method, path, contentType string, body []byte, header http.Header, limit int64) ([]byte, int, error) {
	base, e := teamURL()
	if e != nil {
		return nil, 0, e
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return nil, 0, errors.New("invalid team path")
	}
	req, e := http.NewRequest(method, base+path, bytes.NewReader(body))
	if e != nil {
		return nil, 0, e
	}
	req.Header = header.Clone()
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	client := http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, e := client.Do(req)
	if e != nil {
		return nil, 0, e
	}
	defer resp.Body.Close()
	b, e := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if e != nil {
		return nil, resp.StatusCode, e
	}
	if int64(len(b)) > limit {
		return nil, resp.StatusCode, fmt.Errorf("team response exceeds %d bytes", limit)
	}
	return b, resp.StatusCode, nil
}
func teamCacheDir() (string, error) {
	d, e := os.UserCacheDir()
	if e != nil {
		return "", e
	}
	d = filepath.Join(d, "ryoku", "team")
	e = os.MkdirAll(d, 0700)
	return d, e
}
