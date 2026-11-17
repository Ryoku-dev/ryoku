package main

import (
	"bytes"
	"encoding/json"
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
	reply, e := teamExchange(method, path, contentType, body, header, limit)
	return reply.body, reply.status, e
}

// teamReply carries the gate's clock with its answer, so a record can be stamped in the
// gate's time when this computer's clock is off
type teamReply struct {
	body   []byte
	status int
	date   time.Time
}

func teamExchange(method, path, contentType string, body []byte, header http.Header, limit int64) (teamReply, error) {
	base, e := teamURL()
	if e != nil {
		return teamReply{}, e
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return teamReply{}, errors.New("invalid team path")
	}
	req, e := http.NewRequest(method, base+path, bytes.NewReader(body))
	if e != nil {
		return teamReply{}, e
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
		return teamReply{}, e
	}
	defer resp.Body.Close()
	reply := teamReply{status: resp.StatusCode}
	reply.date, _ = http.ParseTime(resp.Header.Get("Date"))
	// Cloudflare's bot check answers before the gate does, with a page meant for a browser
	if resp.Header.Get("Cf-Mitigated") == "challenge" {
		return reply, errTeamChallenged
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if e != nil {
		return reply, e
	}
	if int64(len(b)) > limit {
		// the gate's own answers are small; a large refusal is a page from in front of it
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return reply, fmt.Errorf("team.ryoku.dev could not be reached (HTTP %d): try again later", resp.StatusCode)
		}
		return reply, fmt.Errorf("team response exceeds %d bytes", limit)
	}
	reply.body = b
	return reply, nil
}

var errTeamChallenged = errors.New("team.ryoku.dev's protection held this request back: try again in a few minutes, and tell the Ryoku team if it keeps happening")

// teamRefusal is the gate's reason for refusing a request, or its HTTP status without one
func teamRefusal(reply teamReply) string {
	var answer struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(reply.body, &answer) == nil && answer.Error != "" {
		return cleanCardText(answer.Error)
	}
	return fmt.Sprintf("HTTP %d", reply.status)
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
