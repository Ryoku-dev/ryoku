package mcpservers

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/vault"
)

func TestRegistryPublicShapeCannotReturnSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "servers.json")
	store, err := Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	info, err := store.Save(protocol.MCPServer{Name: "Tools", Type: "stdio", Command: "tool-server", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.records[0].Headers["Authorization"] = "vault-reference-only"
	store.records[0].Token = "token-reference-only"
	store.mu.Unlock()
	raw, err := json.Marshal(store.List())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "reference-only") || strings.Contains(string(raw), "access_token") || strings.Contains(string(raw), "refresh_token") {
		t.Fatalf("list exposed private metadata: %s", raw)
	}
	if info.ID == "" {
		t.Fatal("server has no ID")
	}
}

func TestSecretNeedsVaultAndNeverReachesRegistry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "servers.json")
	store, err := Open(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Save(protocol.MCPServer{Name: "Tools", Type: "http", URL: "https://tools.example/mcp", Enabled: true, Headers: map[string]string{"Authorization": "private-value"}})
	if err == nil {
		t.Fatal("secret accepted without vault")
	}
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "private-value") {
		t.Fatal("secret reached disk")
	}
}

func TestHTTPCheckInitializesAndListsTools(t *testing.T) {
	initialized := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case "initialize":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"tools":{}}}}`))
		case "notifications/initialized":
			initialized = true
			w.WriteHeader(http.StatusAccepted)
		case "tools/list":
			if !initialized {
				t.Error("tools requested before initialized")
			}
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"read","inputSchema":{"type":"object"}},{"name":"search","inputSchema":{"type":"object"}}]}}`))
		default:
			t.Errorf("unexpected method %s", request.Method)
		}
	}))
	defer server.Close()
	store, err := Open(filepath.Join(t.TempDir(), "servers.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	info, err := store.Save(protocol.MCPServer{Name: "Local", Type: "http", URL: server.URL, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	checked, err := store.Check(context.Background(), info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if checked.State != "ready" || checked.Tools != 2 {
		t.Fatalf("unexpected check: %+v", checked)
	}
}

func TestDisabledServersNeverStart(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "servers.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	info, err := store.Save(protocol.MCPServer{Name: "Off", Type: "stdio", Command: "this-command-must-not-run"})
	if err != nil {
		t.Fatal(err)
	}
	checked, err := store.Check(context.Background(), info.ID)
	if err != nil {
		t.Fatal(err)
	}
	if checked.State != "off" {
		t.Fatal(checked.State)
	}
	if len(store.Tools(context.Background(), tool.Env{Settings: func() protocol.Settings {
		return protocol.Settings{Features: map[string]bool{protocol.FeatureMCP: true}}
	}})) != 0 {
		t.Fatal("disabled server offered to agent")
	}
}

type testKeeper struct{}

func (testKeeper) Key(context.Context, bool, bool) ([]byte, error) { return make([]byte, 32), nil }

func TestVaultHeadersStayWriteOnlyAndSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	v := vault.OpenWith(filepath.Join(dir, "vault.db"), testKeeper{})
	defer v.Close()
	path := filepath.Join(dir, "servers.json")
	store, err := Open(path, v)
	if err != nil {
		t.Fatal(err)
	}
	const secret = "a-private-header-value"
	info, err := store.Save(protocol.MCPServer{Name: "Tools", Type: "http", URL: "https://tools.example/mcp", Enabled: true, Headers: map[string]string{"Authorization": secret}})
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path, v)
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []any{info, reopened.List()} {
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), secret) {
			t.Fatal("public result exposed a header")
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) {
		t.Fatal("registry exposed a header")
	}
	reopened.mu.Lock()
	headers, err := reopened.headers(context.Background(), &reopened.records[0])
	reopened.mu.Unlock()
	if err != nil || headers["Authorization"] != secret {
		t.Fatalf("vault restoration failed: %v", err)
	}
}

func TestPKCELoopbackAndRefresh(t *testing.T) {
	dir := t.TempDir()
	v := vault.OpenWith(filepath.Join(dir, "vault.db"), testKeeper{})
	defer v.Close()
	var expectedChallenge string
	refreshed := make(chan struct{}, 1)
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Form.Get("grant_type") == "refresh_token" {
			if r.Form.Get("refresh_token") != "private-refresh" {
				t.Error("wrong refresh token")
			}
			_, _ = w.Write([]byte(`{"access_token":"fresh-access","token_type":"Bearer","expires_in":3600}`))
			refreshed <- struct{}{}
			return
		}
		digest := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if base64.RawURLEncoding.EncodeToString(digest[:]) != expectedChallenge {
			t.Error("PKCE challenge mismatch")
		}
		if r.Form.Get("code") != "code" || !strings.HasPrefix(r.Form.Get("redirect_uri"), "http://127.0.0.1:") {
			t.Error("wrong code or redirect")
		}
		_, _ = w.Write([]byte(`{"access_token":"private-access","refresh_token":"private-refresh","token_type":"Bearer","expires_in":1}`))
	}))
	defer service.Close()
	store, err := Open(filepath.Join(dir, "servers.json"), v)
	if err != nil {
		t.Fatal(err)
	}
	defer store.CancelSignIns()
	info, err := store.Save(protocol.MCPServer{Name: "Signed in", Type: "http", URL: service.URL + "/mcp", Enabled: true, OAuth: &protocol.MCPOAuth{ClientID: "client", AuthURL: service.URL + "/authorize", TokenURL: service.URL + "/token"}})
	if err != nil {
		t.Fatal(err)
	}
	address, err := store.SignIn(context.Background(), info.ID)
	if err != nil {
		t.Fatal(err)
	}
	auth, err := url.Parse(address)
	if err != nil {
		t.Fatal(err)
	}
	expectedChallenge = auth.Query().Get("code_challenge")
	if auth.Query().Get("code_challenge_method") != "S256" || expectedChallenge == "" {
		t.Fatal("PKCE missing")
	}
	callback, _ := url.Parse(auth.Query().Get("redirect_uri"))
	callback.RawQuery = url.Values{"state": {auth.Query().Get("state")}, "code": {"code"}}.Encode()
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get(callback.String())
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatal(response.StatusCode)
	}
	store.mu.Lock()
	token, err := store.token(context.Background(), &store.records[0])
	store.mu.Unlock()
	if err != nil || token.AccessToken != "fresh-access" {
		t.Fatalf("refresh failed: %v", err)
	}
	select {
	case <-refreshed:
	default:
		t.Fatal("refresh endpoint was not used")
	}
	raw, _ := json.Marshal(store.List())
	if strings.Contains(string(raw), "private-") || strings.Contains(string(raw), "fresh-access") {
		t.Fatal("list exposed OAuth token")
	}
	registry, _ := os.ReadFile(store.path)
	if strings.Contains(string(registry), "private-access") || strings.Contains(string(registry), "private-refresh") {
		t.Fatal("token escaped the vault")
	}
}
