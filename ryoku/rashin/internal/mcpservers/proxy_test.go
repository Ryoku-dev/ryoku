package mcpservers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/vault"
)

func TestProxyRefreshesWithoutReplacingSessionAndRefusesWrongCapability(t *testing.T) {
	var calls, refreshes atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			if r.Form.Get("refresh_token") != "private-refresh" {
				t.Error("refresh credential missing")
			}
			refreshes.Add(1)
			_, _ = io.WriteString(w, `{"access_token":"fresh-private-access","refresh_token":"rotated-refresh","token_type":"Bearer","expires_in":3600}`)
			return
		}
		index := calls.Add(1)
		expected := "Bearer first-private-access"
		if index > 1 {
			expected = "Bearer fresh-private-access"
		}
		if r.Header.Get("Authorization") != expected || r.Header.Get("X-Private") != "private-header" {
			t.Error("proxy did not add current vault credentials")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":1,"result":{}}`)
	}))
	defer upstream.Close()
	dir := t.TempDir()
	v := vault.OpenWith(filepath.Join(dir, "vault.db"), testKeeper{})
	defer v.Close()
	s, err := Open(filepath.Join(dir, "servers.json"), v)
	if err != nil {
		t.Fatal(err)
	}
	defer s.CancelSignIns()
	info, err := s.Save(protocol.MCPServer{Name: "Authenticated", Type: "http", URL: upstream.URL + "/mcp", Enabled: true, Headers: map[string]string{"X-Private": "private-header"}, OAuth: &protocol.MCPOAuth{ClientID: "client", AuthURL: upstream.URL + "/authorize", TokenURL: upstream.URL + "/token"}})
	if err != nil {
		t.Fatal(err)
	}
	token := oauthToken{AccessToken: "first-private-access", RefreshToken: "private-refresh", Expires: time.Now().Add(time.Hour)}
	raw, _ := json.Marshal(token)
	s.mu.Lock()
	credential, err := s.secret("", info.ID, string(raw))
	s.records[0].Token, s.records[0].Info.State = credential, "ready"
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	servers := s.Ready(context.Background())
	if len(servers) != 1 || len(servers[0].Headers) != 0 || !strings.HasPrefix(servers[0].URL, "http://127.0.0.1:") {
		t.Fatalf("session was not given a secret-free proxy: %+v", servers)
	}
	encoded, _ := json.Marshal(servers)
	if strings.Contains(string(encoded), "private-") || strings.Contains(string(encoded), "private-header") {
		t.Fatal("credentials escaped into session configuration")
	}
	client := &http.Client{Timeout: 3 * time.Second}
	post := func(address string) int {
		t.Helper()
		response, err := client.Post(address, "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		_, _ = io.Copy(io.Discard, response.Body)
		return response.StatusCode
	}
	if post(servers[0].URL) != http.StatusOK {
		t.Fatal("first proxied request failed")
	}
	token.Expires = time.Now().Add(-time.Minute)
	raw, _ = json.Marshal(token)
	s.mu.Lock()
	_, err = s.secret(credential, info.ID, string(raw))
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if post(servers[0].URL) != http.StatusOK || refreshes.Load() != 1 || calls.Load() != 2 {
		t.Fatal("session did not refresh its credential in place")
	}
	wrong, _ := url.Parse(servers[0].URL)
	parts := strings.Split(wrong.Path, "/")
	parts[1] = "wrong-token"
	wrong.Path = strings.Join(parts, "/")
	if post(wrong.String()) != http.StatusNotFound || calls.Load() != 2 {
		t.Fatal("wrong proxy capability reached upstream")
	}
	req, _ := http.NewRequest(http.MethodPost, servers[0].URL, strings.NewReader("{}"))
	req.Host = "outside.example"
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Fatal("wrong Host was accepted")
	}
	s.CancelSignIns()
	if len(s.Ready(context.Background())) != 0 {
		t.Fatal("stopped proxy restarted for an old session")
	}
	if response, err := client.Get(servers[0].URL); err == nil {
		response.Body.Close()
		t.Fatal("proxy still listened after stop")
	}
}

func TestSSEProxyKeepsEndpointMessagesOnLoopback(t *testing.T) {
	var posts atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Private") != "private-header" {
			t.Error("SSE request did not use vault header")
		}
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "event: endpoint\ndata: /messages?session=remote-session\n\n")
			return
		}
		if r.URL.Path != "/messages" || r.URL.Query().Get("session") != "remote-session" {
			t.Error("wrong upstream SSE message endpoint")
		}
		posts.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer upstream.Close()
	dir := t.TempDir()
	v := vault.OpenWith(filepath.Join(dir, "vault.db"), testKeeper{})
	defer v.Close()
	s, err := Open(filepath.Join(dir, "servers.json"), v)
	if err != nil {
		t.Fatal(err)
	}
	defer s.CancelSignIns()
	_, err = s.Save(protocol.MCPServer{Name: "Events", Type: "sse", URL: upstream.URL + "/sse", Enabled: true, Headers: map[string]string{"X-Private": "private-header"}})
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.records[0].Info.State = "ready"
	s.mu.Unlock()
	servers := s.Ready(context.Background())
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get(servers[0].URL)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	endpoint := ""
	for line := range strings.SplitSeq(string(raw), "\n") {
		if strings.HasPrefix(line, "data: ") {
			endpoint = strings.TrimPrefix(line, "data: ")
		}
	}
	if !strings.HasPrefix(endpoint, servers[0].URL+"/endpoint/") || strings.Contains(endpoint, "remote-session") {
		t.Fatal("SSE endpoint bypassed the proxy")
	}
	response, err = client.Post(endpoint, "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusAccepted || posts.Load() != 1 {
		t.Fatal("SSE messages were not proxied")
	}
}
