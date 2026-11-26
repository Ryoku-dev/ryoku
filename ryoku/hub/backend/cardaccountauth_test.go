package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCardAccountDiscoveryPinsIssuerAndMetadataOrigin(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	var issuer string
	var metadataIssuer string
	var endpointOverride string
	service := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/config":
			json.NewEncoder(w).Encode(map[string]any{"auth": map[string]string{"issuer": issuer, "client_id": "test-public-client", "scopes": "openid profile email offline_access"}})
		case "/.well-known/oauth-authorization-server":
			device := issuer + "/oauth/device_authorization"
			if endpointOverride != "" {
				device = endpointOverride
			}
			json.NewEncoder(w).Encode(map[string]any{"issuer": metadataIssuer, "device_authorization_endpoint": device, "token_endpoint": issuer + "/oauth/token", "grant_types_supported": []string{cardAccountGrant}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer service.Close()
	issuer = service.URL
	metadataIssuer = issuer
	old := cardAccountHTTP
	cardAccountHTTP = service.Client()
	cardAccountHTTP.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	defer func() { cardAccountHTTP = old }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, e := cardAccountDiscover(ctx, service.URL); e != nil {
		t.Fatal("valid issuer metadata refused", e)
	}
	endpointOverride = "https://another.invalid/oauth/device_authorization"
	if _, _, e := cardAccountDiscover(ctx, service.URL); e == nil {
		t.Fatal("cross-origin OAuth device endpoint accepted")
	}
	endpointOverride = ""
	metadataIssuer = "https://another.invalid"
	if _, _, e := cardAccountDiscover(ctx, service.URL); e == nil {
		t.Fatal("metadata issuer mismatch accepted")
	}
	metadataIssuer = issuer
	issuer = "https://another.invalid"
	if _, _, e := cardAccountDiscover(ctx, service.URL); e == nil || !strings.Contains(e.Error(), "provider changed") {
		t.Fatal("pinned issuer change accepted", e)
	}
}

func TestCardAccountNoPlaintextCredentialFallback(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+filepath.Join(t.TempDir(), "no-bus"))
	token := "must-not-be-used-as-account-bearer"
	t.Setenv("RYOKU_ACCOUNTS_TOKEN", token)
	t.Setenv("RYOKU_ACCOUNTS_ACCESS_TOKEN", token)
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Write([]byte(`{"account":{"id":"ra_abcdefgh12345678"}}`))
	}))
	defer server.Close()
	t.Setenv("RYOKU_ACCOUNTS_URL", server.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, bearer, e := cardAccountAccess(ctx)
	if e == nil || bearer != "" || requests.Load() != 0 {
		t.Fatal("missing keyring used an environment/plaintext fallback", e)
	}
	if strings.Contains(e.Error(), token) {
		t.Fatal("secret appeared in diagnostic")
	}
	dir, e := cardAccountDir()
	if e != nil {
		t.Fatal(e)
	}
	e = filepath.WalkDir(dir, func(path string, entry os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if entry.IsDir() {
			return nil
		}
		raw, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		if strings.Contains(string(raw), token) {
			t.Fatal("secret written to local Account state")
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
}
