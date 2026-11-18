package artifacts

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestPreviewTokenAndOrigin(t *testing.T) {
	v := &preview{token: strings.Repeat("a", 48), contentToken: strings.Repeat("c", 48), origin: "http://127.0.0.1:4242", entry: "index.html", code: "<h1>Private preview</h1>", kind: "html"}
	for _, tc := range []struct {
		name, method, token, origin, header, host string
		status                                    int
	}{
		{"page", "GET", v.token, "", "", "", 200},
		{"missing token", "GET", "", "", "", "", 404},
		{"token prefix", "GET", v.token[:20], "", "", "", 404},
		{"wrong token", "GET", strings.Repeat("b", 48), "", "", "", 404},
		{"foreign page", "GET", v.token, "https://example.org", "", "", 403},
		{"rebound host", "GET", v.token, "", "", "example.org:4242", 403},
		{"report", "POST", v.token, v.origin, v.token, "", 204},
		{"report no origin", "POST", v.token, "", v.token, "", 403},
		{"report null origin", "POST", v.token, "null", v.token, "", 403},
		{"report wrong origin", "POST", v.token, "http://127.0.0.1:4243", v.token, "", 403},
		{"report no header", "POST", v.token, v.origin, "", "", 403},
		{"report header prefix", "POST", v.token, v.origin, v.token[:20], "", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := "__view"
			if tc.method == "POST" {
				name = "__errors"
			}
			r := httptest.NewRequest(tc.method, v.origin+"/"+tc.token+"/"+name, strings.NewReader(`{"message":"broken button","line":3}`))
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("X-Artifact-Token", tc.header)
			r.Header.Set("Content-Type", "application/json")
			if tc.host != "" {
				r.Host = tc.host
			}
			w := httptest.NewRecorder()
			v.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d", w.Code, tc.status)
			}
			if tc.status != 200 && strings.Contains(w.Body.String(), v.token) {
				t.Fatal("error response leaked the token")
			}
			if tc.status == 200 && (!strings.Contains(w.Header().Get("Content-Security-Policy"), "default-src 'none'") || w.Header().Get("Referrer-Policy") != "no-referrer") {
				t.Fatal("page is missing preview safety headers")
			}
		})
	}
}

func TestPreviewNeverLeavesArtifactFolder(t *testing.T) {
	base := t.TempDir()
	folder := filepath.Join(base, "artifact")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "secret"), []byte("outside-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "index.html"), []byte("<h1>Page</h1>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(base, "secret"), filepath.Join(folder, "escape")); err != nil {
		t.Fatal(err)
	}
	var m Manager
	opened, err := m.Open(context.Background(), protocol.ArtifactOpenParams{Session: "test", Path: filepath.Join(folder, "index.html")}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close(opened.ID)
	m.mu.Lock()
	v := m.previews[opened.ID]
	prefix := v.origin + "/" + v.contentToken + "/static/"
	m.mu.Unlock()
	for _, suffix := range []string{"../secret", "%2e%2e/secret", "escape", ".", "", "missing"} {
		response, err := http.Get(prefix + suffix)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if response.StatusCode == 200 || strings.Contains(string(body), "outside-secret") {
			t.Fatalf("served forbidden path %q", suffix)
		}
	}
}

func TestPreviewRejectsCrossSiteAndOversizedReports(t *testing.T) {
	v := &preview{token: "token", origin: "http://127.0.0.1:4242", entry: "index.html", code: "page", kind: "html"}
	for _, body := range []string{`{"message":"ok"}` + strings.Repeat(" ", MaxBytes), `{"message":"ok"} {"message":"more"}`} {
		r := httptest.NewRequest("POST", v.origin+"/token/__errors", strings.NewReader(body))
		r.Header.Set("Origin", v.origin)
		r.Header.Set("X-Artifact-Token", v.token)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		v.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest && w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("oversized or trailing report status = %d", w.Code)
		}
	}
	r := httptest.NewRequest("GET", v.origin+"/token/index.html", nil)
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	v.ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-site status = %d", w.Code)
	}
}

func TestPreviewUsesStaticSandboxUntilScriptsAreRequested(t *testing.T) {
	v := &preview{token: "wrapper-token", contentToken: "child-token", origin: "http://127.0.0.1:4242", entry: "index.html", code: `<script>location.replace("https://example.org/?private=page")</script>`, kind: "html"}
	wrapper := httptest.NewRecorder()
	v.ServeHTTP(wrapper, httptest.NewRequest("GET", v.origin+"/wrapper-token/__view", nil))
	if wrapper.Code != http.StatusOK {
		t.Fatalf("wrapper status = %d", wrapper.Code)
	}
	body := wrapper.Body.String()
	start := strings.Index(body, "<iframe ")
	if start < 0 {
		t.Fatal("wrapper has no preview frame")
	}
	end := strings.IndexByte(body[start:], '>')
	if end < 0 {
		t.Fatal("wrapper frame is incomplete")
	}
	frame := body[start : start+end]
	if !strings.Contains(frame, `sandbox=""`) || !strings.Contains(frame, "/child-token/static/index.html") {
		t.Fatalf("preview did not start with scripts off: %s", frame)
	}
	for _, permission := range []string{"allow-top-navigation", "allow-same-origin", "allow-popups"} {
		if strings.Contains(frame, permission) || strings.Contains(body, permission) {
			t.Fatalf("wrapper grants %s", permission)
		}
	}
	if strings.Contains(body, "location.replace") || !strings.Contains(body, "Run scripts") || !strings.Contains(body, `event.source!==frame.contentWindow`) || !strings.Contains(body, `event.origin!=="null"`) || !strings.Contains(body, "Number.isInteger(d.line)") {
		t.Fatal("wrapper contains untrusted code or lacks its relay boundary")
	}
	if csp := wrapper.Header().Get("Content-Security-Policy"); strings.Contains(csp, "script-src 'unsafe-inline'") || !strings.Contains(csp, "script-src 'nonce-") {
		t.Fatalf("wrapper script CSP is not restricted to its trusted script: %s", csp)
	}
	for _, mode := range []string{"static", "run"} {
		r := httptest.NewRequest("GET", v.origin+"/child-token/"+mode+"/index.html", nil)
		r.Header.Set("Origin", "null")
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		w := httptest.NewRecorder()
		v.ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("%s child status = %d", mode, w.Code)
		}
		csp := w.Header().Get("Content-Security-Policy")
		if !strings.Contains(csp, "; sandbox") || !strings.Contains(csp, "connect-src 'none'") || strings.Contains(csp, "allow-same-origin") || strings.Contains(csp, "allow-top-navigation") || strings.Contains(csp, "allow-popups") {
			t.Fatalf("child sandbox missing or too permissive: %s", csp)
		}
		if mode == "static" && (!strings.Contains(csp, "script-src 'none'") || strings.Contains(csp, "sandbox allow-scripts")) {
			t.Fatalf("static response allows scripts: %s", csp)
		}
		if mode == "run" && (!strings.Contains(csp, "sandbox allow-scripts") || !strings.Contains(w.Body.String(), "parent.postMessage")) {
			t.Fatal("requested scripts cannot relay runtime errors")
		}
		if strings.Contains(w.Body.String(), v.token) || strings.Contains(w.Body.String(), "X-Artifact-Token") {
			t.Fatal("untrusted child received the wrapper's report credential")
		}
	}
}

func TestCloseAllReleasesEveryPreview(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.html")
	if err := os.WriteFile(path, []byte("<p>Page</p>"), 0o600); err != nil {
		t.Fatal(err)
	}
	var m Manager
	var urls []string
	var previews []*preview
	for range 2 {
		result, err := m.Open(context.Background(), protocol.ArtifactOpenParams{Session: "test", Path: path}, nil)
		if err != nil {
			t.Fatal(err)
		}
		urls = append(urls, result.URL)
		m.mu.Lock()
		previews = append(previews, m.previews[result.ID])
		m.mu.Unlock()
	}
	m.CloseAll()
	if m.Busy() {
		t.Fatal("closed previews still hold the manager busy")
	}
	for index, v := range previews {
		if _, err := m.Session(v.id); err == nil {
			t.Fatal("closed preview is still registered")
		}
		if !v.closed {
			t.Fatal("closed preview can restart its idle timer")
		}
		if file, err := v.root.Open("index.html"); err == nil {
			_ = file.Close()
			t.Fatal("artifact root remained open")
		}
		if response, err := http.Get(urls[index]); err == nil {
			_ = response.Body.Close()
			t.Fatal("preview listener remained open")
		}
	}
	m.CloseAll()
	result, err := m.Open(context.Background(), protocol.ArtifactOpenParams{Session: "test", Code: "<p>New page</p>", Kind: "html"}, nil)
	if err != nil {
		t.Fatalf("manager cannot reopen after the feature is enabled again: %v", err)
	}
	m.Close(result.ID)
}
