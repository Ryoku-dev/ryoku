package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func isolateProwlGatewayTest(t *testing.T) string {
	t.Helper()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(homeDir, ".local", "share"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(homeDir, ".local", "state"))
	t.Setenv("RYOKU_CONFIG_BASE", filepath.Join(homeDir, "absent-packaged-base"))
	t.Setenv("RYOKU_RASHIN_VAULT", filepath.Join(homeDir, "vault"))
	t.Setenv("PATH", filepath.Join(homeDir, "bin"))
	t.Setenv("RYOKU_PROWL_BIN", "")
	t.Setenv("RYOKU_RASHIN_REPO", "")
	return homeDir
}

func pointGatewayAt(t *testing.T, serverURL string) {
	t.Helper()
	parsed, err := url.Parse(serverURL)
	if err != nil {
		t.Fatal(err)
	}
	_, port, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_PROWL_PORT", port)
}

func writeGatewayToken(t *testing.T, token string) {
	t.Helper()
	path := prowlGatewayTokenPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func proxyRequest(t *testing.T, mux http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = "127.0.0.1:3600"
	req.Header.Set("X-Rashin-Client", "console")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)
	return recorder
}

func TestProwlProxyForwardsRequestAndProtectsCredentials(t *testing.T) {
	homeDir := isolateProwlGatewayTest(t)
	repo := filepath.Join(homeDir, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".prowl"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_RASHIN_REPO", repo)
	writeGatewayToken(t, "machine-token")

	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %s, want PATCH", r.Method)
		}
		if r.URL.Path != "/api/code/search" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.Query().Get("q") != "needle" || r.URL.Query().Get("repo") != repo {
			t.Errorf("query = %v", r.URL.Query())
		}
		if !strings.HasPrefix(r.URL.RawQuery, "q=needle&repo=") {
			t.Errorf("raw query was not preserved before repo injection: %q", r.URL.RawQuery)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if string(body) != `{"scope":"all"}` {
			t.Errorf("body = %q", body)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer machine-token" {
			t.Errorf("authorization = %q", got)
		}
		for _, name := range []string{"Accept-Encoding", "Cookie", "Origin", "Referer", "Sec-Fetch-Site"} {
			if got := r.Header.Get(name); got != "" {
				t.Errorf("%s leaked upstream: %q", name, got)
			}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer gateway.Close()
	pointGatewayAt(t, gateway.URL)

	mux := http.NewServeMux()
	registerProwlProxy(mux, 3600)
	req := httptest.NewRequest(http.MethodPatch, "/api/prowl/code/search?q=needle", strings.NewReader(`{"scope":"all"}`))
	req.Host = "127.0.0.1:3600"
	req.Header.Set("X-Rashin-Client", "console")
	req.Header.Set("Authorization", "Bearer browser-token")
	req.Header.Set("Accept-Encoding", "br")
	req.Header.Set("Cookie", "session=browser")
	req.Header.Set("Origin", "http://127.0.0.1:3600")
	req.Header.Set("Referer", "http://127.0.0.1:3600/")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
	if got := recorder.Body.String(); got != `{"ok":true}` {
		t.Fatalf("body = %q", got)
	}
}

func TestProwlProxyKeepsExplicitRepoAndLimitsInjectionToCode(t *testing.T) {
	isolateProwlGatewayTest(t)
	writeGatewayToken(t, "token")
	queries := make(chan url.Values, 2)
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries <- r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer gateway.Close()
	pointGatewayAt(t, gateway.URL)
	mux := http.NewServeMux()
	registerProwlProxy(mux, 3600)

	if got := proxyRequest(t, mux, http.MethodGet, "/api/prowl/code/status?repo=%2Fchosen", "").Code; got != http.StatusOK {
		t.Fatalf("code status = %d", got)
	}
	if got := (<-queries).Get("repo"); got != "/chosen" {
		t.Fatalf("explicit repo = %q", got)
	}
	if got := proxyRequest(t, mux, http.MethodGet, "/api/prowl/providers/directory", "").Code; got != http.StatusOK {
		t.Fatalf("provider status = %d", got)
	}
	if values := <-queries; values.Has("repo") {
		t.Fatalf("non-code request gained repo: %v", values)
	}
}

func TestProwlProxyRejectsCrossSiteAndUnknownRequests(t *testing.T) {
	isolateProwlGatewayTest(t)
	mux := http.NewServeMux()
	registerProwlProxy(mux, 3600)

	cases := []struct {
		name   string
		host   string
		origin string
		client string
	}{
		{name: "bad host", host: "evil.example:3600", client: "console"},
		{name: "foreign origin", host: "127.0.0.1:3600", origin: "https://evil.example", client: "console"},
		{name: "missing client", host: "127.0.0.1:3600"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/prowl/health", nil)
			req.Host = test.host
			req.Header.Set("Origin", test.origin)
			req.Header.Set("X-Rashin-Client", test.client)
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, req)
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || body.Error.Code != "cross_site" {
				t.Fatalf("body = %s, err = %v", recorder.Body.String(), err)
			}
		})
	}

	unknown := proxyRequest(t, mux, http.MethodGet, "/api/prowl/admin/secrets", "")
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown segment status = %d, body = %s", unknown.Code, unknown.Body.String())
	}
}

func TestProwlProxyGatewayDownStartsAtMostOncePerWindow(t *testing.T) {
	homeDir := isolateProwlGatewayTest(t)
	bin := filepath.Join(homeDir, "prowl")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_PROWL_BIN", bin)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	t.Setenv("RYOKU_PROWL_PORT", strconv.Itoa(port))

	oldSpawn, oldSystemd, oldNow := gatewaySpawn, gatewaySystemdPresent, gatewayNow
	gatewayLifecycleMu.Lock()
	oldLast := gatewayLastEnsure
	gatewayLastEnsure = time.Time{}
	gatewayLifecycleMu.Unlock()
	t.Cleanup(func() {
		gatewaySpawn, gatewaySystemdPresent, gatewayNow = oldSpawn, oldSystemd, oldNow
		gatewayLifecycleMu.Lock()
		gatewayLastEnsure = oldLast
		gatewayLifecycleMu.Unlock()
	})
	calls := 0
	gatewaySystemdPresent = func() bool { return false }
	gatewayNow = func() time.Time { return time.Unix(1000, 0) }
	gatewaySpawn = func(name string, args ...string) error {
		calls++
		if name != bin || strings.Join(args, " ") != "gateway up --port "+strconv.Itoa(port) {
			t.Errorf("spawn = %s %v", name, args)
		}
		return nil
	}

	mux := http.NewServeMux()
	registerProwlProxy(mux, 3600)
	for range 2 {
		recorder := proxyRequest(t, mux, http.MethodGet, "/api/prowl/health", "")
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), `"code":"gateway_down"`) {
			t.Fatalf("body = %s", recorder.Body.String())
		}
	}
	if calls != 1 {
		t.Fatalf("gateway spawn calls = %d, want 1", calls)
	}
}

func TestEnsureGatewayDoesNotSpawnWithoutProwl(t *testing.T) {
	isolateProwlGatewayTest(t)
	oldSpawn := gatewaySpawn
	t.Cleanup(func() { gatewaySpawn = oldSpawn })
	calls := 0
	gatewaySpawn = func(string, ...string) error {
		calls++
		return nil
	}
	if err := ensureGateway(); err != nil {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatalf("spawn calls = %d, want 0", calls)
	}
}

func TestProwlGatewayJSONDecodesErrorEnvelope(t *testing.T) {
	isolateProwlGatewayTest(t)
	writeGatewayToken(t, "token")
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"error":{"type":"validation","code":"bad_route","message":"route is invalid"}}`))
	}))
	defer gateway.Close()
	pointGatewayAt(t, gateway.URL)

	err := prowlGatewayJSON(context.Background(), http.MethodGet, "/api/code/status", nil, nil)
	gatewayErr, ok := err.(*prowlGatewayError)
	if !ok {
		t.Fatalf("error type = %T, value = %v", err, err)
	}
	if gatewayErr.Status != http.StatusUnprocessableEntity || gatewayErr.Type != "validation" || gatewayErr.Code != "bad_route" || gatewayErr.Message != "route is invalid" {
		t.Fatalf("decoded error = %+v", gatewayErr)
	}
}

func TestProwlGatewayJSONMapsUnreachableGateway(t *testing.T) {
	isolateProwlGatewayTest(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	t.Setenv("RYOKU_PROWL_PORT", strconv.Itoa(port))

	err = prowlGatewayJSON(context.Background(), http.MethodGet, "/api/health", nil, nil)
	gatewayErr, ok := err.(*prowlGatewayError)
	if !ok || gatewayErr.Status != http.StatusServiceUnavailable || gatewayErr.Type != "unavailable" || gatewayErr.Code != "gateway_down" {
		t.Fatalf("gateway error = %#v", err)
	}
}

func TestBuildStatusCarriesProwlState(t *testing.T) {
	homeDir := isolateProwlGatewayTest(t)
	bin := filepath.Join(homeDir, "prowl")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho prowl v4.5.6\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_PROWL_BIN", bin)
	repo := filepath.Join(homeDir, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".prowl"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_RASHIN_REPO", repo)
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/ping" {
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer gateway.Close()
	pointGatewayAt(t, gateway.URL)

	cfg := defaultConfig()
	cfg.Port = 1
	got := BuildStatus(cfg).Prowl
	if !got.Installed || !got.Running || got.Bin != bin || got.Version != "v4.5.6" {
		t.Fatalf("prowl status = %+v", got)
	}
	if got.Port != prowlGatewayPort() || got.URL != prowlGatewayBase() || got.Repo != repo || got.Error != "" {
		t.Fatalf("prowl status endpoints = %+v", got)
	}
}
