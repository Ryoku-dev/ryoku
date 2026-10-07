package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultProwlGatewayPort = 8788
	prowlProxyRequestLimit  = 1 << 20
	prowlProxyResponseLimit = 8 << 20
)

var prowlGatewayHTTPClient = newProwlGatewayHTTPClient()

func newProwlGatewayHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableCompression = true
	return &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func prowlGatewayPort() int {
	if raw := os.Getenv("RYOKU_PROWL_PORT"); raw != "" {
		if port, err := strconv.Atoi(raw); err == nil && port > 0 && port <= 65535 {
			return port
		}
	}
	return defaultProwlGatewayPort
}

func prowlGatewayBase() string {
	return "http://127.0.0.1:" + strconv.Itoa(prowlGatewayPort())
}

func prowlGatewayTokenPath() string {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		base = filepath.Join(home(), ".local", "share")
	}
	// Prowl owns the historical prowl-agent state directory name.
	return filepath.Join(base, "prowl-agent", "gateway", "token")
}

func prowlGatewayToken() string {
	body, err := os.ReadFile(prowlGatewayTokenPath())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(body))
}

type prowlGatewayError struct {
	Status  int
	Type    string
	Code    string
	Message string
}

func (e *prowlGatewayError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Code != "" {
		return e.Code
	}
	return "Prowl gateway request failed"
}

func prowlGatewayJSON(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		blob, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(blob)
	}
	req, err := http.NewRequestWithContext(ctx, method, prowlGatewayBase()+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+prowlGatewayToken())
	resp, err := prowlGatewayHTTPClient.Do(req)
	if err != nil {
		_ = ensureGateway()
		return &prowlGatewayError{
			Status:  http.StatusServiceUnavailable,
			Type:    "unavailable",
			Code:    "gateway_down",
			Message: "Prowl's gateway is not running",
		}
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, prowlProxyResponseLimit+1))
	if err != nil {
		return err
	}
	if len(payload) > prowlProxyResponseLimit {
		return &prowlGatewayError{
			Status:  http.StatusBadGateway,
			Type:    "upstream",
			Code:    "response_too_large",
			Message: "Prowl's gateway response exceeded the limit",
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		gatewayErr := &prowlGatewayError{Status: resp.StatusCode, Type: "upstream", Code: "gateway_error"}
		var envelope struct {
			Error struct {
				Type    string `json:"type"`
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(payload, &envelope) == nil {
			if envelope.Error.Type != "" {
				gatewayErr.Type = envelope.Error.Type
			}
			if envelope.Error.Code != "" {
				gatewayErr.Code = envelope.Error.Code
			}
			gatewayErr.Message = envelope.Error.Message
		}
		if gatewayErr.Message == "" {
			gatewayErr.Message = http.StatusText(resp.StatusCode)
		}
		return gatewayErr
	}
	if out == nil || len(bytes.TrimSpace(payload)) == 0 {
		return nil
	}
	return json.Unmarshal(payload, out)
}

var (
	gatewayLifecycleMu    sync.Mutex
	gatewayLastEnsure     time.Time
	gatewayEnsureWindow   = 30 * time.Second
	gatewayNow            = time.Now
	gatewaySystemdPresent = systemdUserManagerPresent
	gatewaySpawn          = runGatewayCommand
)

func runGatewayCommand(name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func systemdUserManagerPresent() bool {
	if !haveSystemd() {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "systemctl", "--user", "show-environment").Run() == nil
}

func prowlGatewayRunning() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, prowlGatewayBase()+"/api/ping", nil)
	if err != nil {
		return false
	}
	resp, err := prowlGatewayHTTPClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func ensureGateway() error {
	bin, installed := findProwl()
	if !installed || prowlGatewayRunning() {
		return nil
	}

	gatewayLifecycleMu.Lock()
	defer gatewayLifecycleMu.Unlock()
	now := gatewayNow()
	if !gatewayLastEnsure.IsZero() && now.Sub(gatewayLastEnsure) < gatewayEnsureWindow {
		return nil
	}
	gatewayLastEnsure = now
	port := strconv.Itoa(prowlGatewayPort())
	if gatewaySystemdPresent() {
		return gatewaySpawn("systemctl", "--user", "start", "ryoku-prowl.service")
	}
	return gatewaySpawn(bin, "gateway", "up", "--port", port)
}

func stopGateway() error {
	bin, installed := findProwl()
	if !installed {
		return nil
	}
	return gatewaySpawn(bin, "gateway", "down", "--port", strconv.Itoa(prowlGatewayPort()))
}

var prowlProxySegments = map[string]bool{
	"keys": true, "providers": true, "profiles": true, "fallback": true,
	"models": true, "usage": true, "logs": true, "logins": true,
	"signin": true, "settings": true, "health": true, "catalog": true,
	"free-tier": true, "cache": true, "update": true, "projects": true,
	"setup": true, "code": true,
}

func registerProwlProxy(mux *http.ServeMux, rashinPort int) {
	handler := prowlProxyHandler(rashinPort)
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		mux.Handle(method+" /api/prowl/{rest...}", handler)
	}
	notFound := func(w http.ResponseWriter, _ *http.Request) {
		writeProwlProxyError(w, http.StatusNotFound, "not_found", "not_found", "no such endpoint")
	}
	mux.HandleFunc("/api/prowl", notFound)
	mux.HandleFunc("/api/prowl/{rest...}", notFound)
}

func prowlProxyHandler(rashinPort int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validProwlProxyRequest(r, rashinPort) {
			writeProwlProxyError(w, http.StatusForbidden, "forbidden", "cross_site", "Prowl requests are limited to Rashin's local console")
			return
		}
		rest := strings.Trim(r.PathValue("rest"), "/")
		segment, _, _ := strings.Cut(rest, "/")
		if rest == "" || !prowlProxySegments[segment] {
			writeProwlProxyError(w, http.StatusNotFound, "not_found", "not_found", "no such endpoint")
			return
		}
		if r.ContentLength > prowlProxyRequestLimit {
			writeProwlProxyError(w, http.StatusRequestEntityTooLarge, "request", "body_too_large", "request body exceeds 1 MiB")
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, prowlProxyRequestLimit+1))
		if err != nil {
			writeProwlProxyError(w, http.StatusBadRequest, "request", "bad_body", "could not read request body")
			return
		}
		if len(body) > prowlProxyRequestLimit {
			writeProwlProxyError(w, http.StatusRequestEntityTooLarge, "request", "body_too_large", "request body exceeds 1 MiB")
			return
		}

		rawQuery := r.URL.RawQuery
		if segment == "code" && !r.URL.Query().Has("repo") {
			repoQuery := url.Values{"repo": []string{prowlRepo()}}.Encode()
			if rawQuery == "" {
				rawQuery = repoQuery
			} else {
				rawQuery += "&" + repoQuery
			}
		}
		escapedRest := strings.TrimPrefix(r.URL.EscapedPath(), "/api/prowl/")
		upstreamURL := prowlGatewayBase() + "/api/" + escapedRest
		if rawQuery != "" {
			upstreamURL += "?" + rawQuery
		}
		ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
		defer cancel()
		upstream, err := http.NewRequestWithContext(ctx, r.Method, upstreamURL, bytes.NewReader(body))
		if err != nil {
			writeProwlProxyError(w, http.StatusBadGateway, "upstream", "gateway_error", "could not create Prowl request")
			return
		}
		copyProwlProxyHeaders(upstream.Header, r.Header)
		upstream.Header.Set("Authorization", "Bearer "+prowlGatewayToken())
		resp, err := prowlGatewayHTTPClient.Do(upstream)
		if err != nil {
			_ = ensureGateway()
			writeProwlProxyError(w, http.StatusServiceUnavailable, "unavailable", "gateway_down", "Prowl's gateway is not running; Rashin is starting it")
			return
		}
		defer resp.Body.Close()
		payload, err := io.ReadAll(io.LimitReader(resp.Body, prowlProxyResponseLimit+1))
		if err != nil {
			writeProwlProxyError(w, http.StatusBadGateway, "upstream", "gateway_error", "could not read Prowl's response")
			return
		}
		if len(payload) > prowlProxyResponseLimit {
			writeProwlProxyError(w, http.StatusBadGateway, "upstream", "response_too_large", "Prowl's response exceeds 8 MiB")
			return
		}
		if contentType := resp.Header.Get("Content-Type"); contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(payload)
	}
}

func validProwlProxyRequest(r *http.Request, port int) bool {
	portText := strconv.Itoa(port)
	switch r.Host {
	case "127.0.0.1:" + portText, "localhost:" + portText, "[::1]:" + portText:
	default:
		return false
	}
	if r.Header.Get("X-Rashin-Client") != "console" {
		return false
	}
	origin := r.Header.Get("Origin")
	return origin == "" || origin == "http://127.0.0.1:"+portText || origin == "http://localhost:"+portText
}

func copyProwlProxyHeaders(dst, src http.Header) {
	for name, values := range src {
		lower := strings.ToLower(name)
		if lower == "accept-encoding" || lower == "cookie" || lower == "authorization" ||
			lower == "origin" || lower == "referer" || strings.HasPrefix(lower, "sec-fetch-") ||
			lower == "connection" || lower == "proxy-connection" || lower == "keep-alive" ||
			lower == "proxy-authenticate" || lower == "proxy-authorization" || lower == "te" ||
			lower == "trailer" || lower == "transfer-encoding" || lower == "upgrade" {
			continue
		}
		for _, value := range values {
			dst.Add(name, value)
		}
	}
}

func writeProwlProxyError(w http.ResponseWriter, status int, errorType, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"type": errorType, "code": code, "message": message},
	})
}
