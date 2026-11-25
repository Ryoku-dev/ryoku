package account

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/gateway"
	"ryoku-rashin/internal/protocol"
)

// memStore stands in for the gateway store.
type memStore struct {
	mu      sync.Mutex
	record  *gateway.AccountRecord
	issuers map[string]string
	failing bool
}

func (m *memStore) AccountTokens() (*gateway.AccountRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.record == nil {
		return nil, nil
	}
	r := *m.record
	return &r, nil
}

func (m *memStore) SetAccountTokens(r *gateway.AccountRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failing {
		return errors.New("disk full")
	}
	if r == nil {
		m.record = nil
		return nil
	}
	c := *r
	m.record = &c
	return nil
}

func (m *memStore) AccountIssuer(server string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.issuers[server], nil
}

func (m *memStore) SetAccountIssuer(server, issuer string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.issuers == nil {
		m.issuers = map[string]string{}
	}
	if issuer == "" {
		delete(m.issuers, server)
		return nil
	}
	m.issuers[server] = issuer
	return nil
}

func (m *memStore) kept() *gateway.AccountRecord {
	r, _ := m.AccountTokens()
	return r
}

func (m *memStore) fail(failing bool) {
	m.mu.Lock()
	m.failing = failing
	m.mu.Unlock()
}

// fakeIdP is an OAuth server with the device grant, scripted per test.
type fakeIdP struct {
	srv          *httptest.Server
	noDevice     bool
	answers      []string // device-code poll answers in order; "" grants tokens, "503" fails
	polls        atomic.Int32
	deviceCalls  atomic.Int32
	refreshCalls atomic.Int32
	refreshErr   string
	issued       atomic.Int32
	revoked      atomic.Int32

	mu             sync.Mutex
	deviceGate     chan struct{} // when set, the device endpoint waits on it
	scopes         []string      // the scope of each device authorization request
	invalidScope   bool          // refuse the device authorization as invalid_scope
	refreshGate    chan struct{} // when set, the refresh grant waits on it
	redirectDevice string
	verification   string // overrides the approval page
	complete       string
	expiresIn      int
	onRevoke       func()
}

func (f *fakeIdP) set(fn func()) {
	f.mu.Lock()
	fn()
	f.mu.Unlock()
}

func newIdP(t *testing.T) *fakeIdP {
	f := &fakeIdP{expiresIn: 3600}
	mux := http.NewServeMux()
	f.srv = httptest.NewTLSServer(mux)
	t.Cleanup(f.srv.Close)
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, _ *http.Request) {
		grants := []string{"authorization_code", "refresh_token"}
		if !f.noDevice {
			grants = append(grants, deviceGrant)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": f.srv.URL, "device_authorization_endpoint": f.srv.URL + "/oauth/device_authorization",
			"token_endpoint": f.srv.URL + "/oauth/token", "revocation_endpoint": f.srv.URL + "/oauth/revoke", "grant_types_supported": grants,
		})
	})
	mux.HandleFunc("/oauth/device_authorization", func(w http.ResponseWriter, r *http.Request) {
		f.deviceCalls.Add(1)
		_ = r.ParseForm()
		f.mu.Lock()
		f.scopes = append(f.scopes, r.Form.Get("scope"))
		refuse := f.invalidScope
		f.mu.Unlock()
		if refuse {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_scope"}`))
			return
		}
		f.mu.Lock()
		gate, redirect, verification, complete := f.deviceGate, f.redirectDevice, f.verification, f.complete
		f.mu.Unlock()
		if gate != nil {
			<-gate
		}
		if redirect != "" {
			http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)
			return
		}
		if r.Form.Get("client_id") != "client-1" || r.Header.Get("Authorization") != "" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
			return
		}
		if verification == "" {
			verification = f.srv.URL + "/device"
		}
		if complete == "" {
			complete = f.srv.URL + "/device?user_code=BCDF-GHJK"
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"device_code": "secret-device-code", "user_code": "BCDF-GHJK", "verification_uri": verification, "verification_uri_complete": complete, "expires_in": 600, "interval": 1})
	})
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		switch r.Form.Get("grant_type") {
		case deviceGrant:
			n := int(f.polls.Add(1)) - 1
			answer := ""
			if n < len(f.answers) {
				answer = f.answers[n]
			}
			if answer == "503" {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte(`{}`))
				return
			}
			if answer != "" {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": answer})
				return
			}
		case "refresh_token":
			f.refreshCalls.Add(1)
			f.mu.Lock()
			gate := f.refreshGate
			f.mu.Unlock()
			if gate != nil {
				<-gate
			}
			if f.refreshErr != "" {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": f.refreshErr})
				return
			}
		}
		f.mu.Lock()
		expires := f.expiresIn
		f.mu.Unlock()
		i := f.issued.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "access-" + string(rune('0'+i)), "refresh_token": "refresh-" + string(rune('0'+i)), "expires_in": expires, "token_type": "bearer"})
	})
	mux.HandleFunc("/oauth/revoke", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		hook := f.onRevoke
		f.mu.Unlock()
		if hook != nil {
			hook()
		}
		f.revoked.Add(1)
	})
	return f
}

// fakeWorker is the Ryoku Accounts server.
type fakeWorker struct {
	srv      *httptest.Server
	accounts map[string]string // token -> account id
	mu       sync.Mutex
	gate     chan struct{} // when set, /v1/account waits on it
	lastAuth atomic.Value
	reads    atomic.Int32

	issuer     string
	accountURL string
	notice     string
	scopes     string
	// refuse answers /v1/account with this status and message
	refuse        int
	refuseMessage string
	redirect      string
}

func newWorker(t *testing.T, issuer string) *fakeWorker {
	f := &fakeWorker{issuer: issuer, accounts: map[string]string{}, accountURL: "https://accounts.example.com/account", notice: "Hosted models send what you write off this computer.", scopes: "openid offline_access"}
	mux := http.NewServeMux()
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	mux.HandleFunc("/v1/config", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		issuer, accountURL, notice, scopes := f.issuer, f.accountURL, f.notice, f.scopes
		f.mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"service": "ryoku-accounts", "version": "1",
			"auth":             map[string]string{"issuer": issuer, "client_id": "client-1", "scopes": scopes},
			"units_per_credit": 1_000_000_000,
			"models":           []map[string]string{{"id": "rashin-fast", "name": "Qwen3.8 Flash", "logo": "qwen"}},
			"notice":           notice,
			"account_url":      accountURL,
		})
	})
	mux.HandleFunc("/v1/account", func(w http.ResponseWriter, r *http.Request) {
		f.reads.Add(1)
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		f.lastAuth.Store(token)
		f.mu.Lock()
		id, ok := f.accounts[token]
		gate, refuse, message, redirect := f.gate, f.refuse, f.refuseMessage, f.redirect
		f.mu.Unlock()
		if gate != nil {
			<-gate
		}
		if redirect != "" {
			http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)
			return
		}
		if refuse != 0 {
			w.WriteHeader(refuse)
			body, _ := json.Marshal(map[string]any{"error": map[string]string{"message": message}})
			_, _ = w.Write(body)
			return
		}
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":{"type":"unauthorized","message":"Sign in to your Ryoku Account again."}}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"account": map[string]any{"id": id, "display": id + "@example.com"},
			"balance": map[string]int64{"available": 12_345_678_901, "held": 2_000_000_000}, "units_per_credit": 1_000_000_000,
		})
	})
	return f
}

func (f *fakeWorker) allow(token, account string) {
	f.mu.Lock()
	f.accounts[token] = account
	f.mu.Unlock()
}

func (f *fakeWorker) with(fn func()) {
	f.mu.Lock()
	fn()
	f.mu.Unlock()
}

func newService(t *testing.T, idp *fakeIdP, worker *fakeWorker, store *memStore) *Service {
	t.Setenv(EnvServer, worker.srv.URL)
	s := New(func() protocol.Settings { return protocol.Settings{} }, store)
	s.client = idp.srv.Client()
	s.second = 5 * time.Millisecond
	return s
}

func await(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestDeviceSignInHonoursPollingAndKeepsTokensOutOfState(t *testing.T) {
	idp := newIdP(t)
	idp.answers = []string{"authorization_pending", "slow_down", "authorization_pending"}
	worker := newWorker(t, idp.srv.URL)
	worker.allow("access-1", "ra_a")
	store := &memStore{}
	s := newService(t, idp, worker, store)

	st, err := s.SignIn(context.Background())
	if err != nil || st.State != protocol.AccountSigningIn || st.Code == nil || st.Code.UserCode != "BCDF-GHJK" {
		t.Fatalf("sign-in start = %+v, %v", st, err)
	}
	await(t, "signed in with a balance", func() bool {
		st := s.Status()
		return st.State == protocol.AccountSignedIn && st.Balance != nil
	})
	if got := idp.polls.Load(); got != 4 {
		t.Fatalf("polls = %d, want 4 (pending, slow_down, pending, granted)", got)
	}
	st = s.Status()
	if st.Account.ID != "ra_a" || protocol.AccountCredits(st.Balance.Available, st.UnitsPerCredit) != "12.34" || st.Balance.Held != 2_000_000_000 {
		t.Fatalf("signed-in state = %+v", st)
	}
	raw, _ := json.Marshal(st)
	for _, secret := range []string{"access-1", "refresh-1", "secret-device-code"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("state carries %q: %s", secret, raw)
		}
	}
	for _, gone := range []string{"plans", "pending", "checkout", "order"} {
		if strings.Contains(string(raw), gone) {
			t.Fatalf("state still carries %q: %s", gone, raw)
		}
	}
	if kept := store.kept(); kept == nil || kept.Access != "access-1" || kept.Refresh != "refresh-1" {
		t.Fatalf("stored record = %+v", kept)
	}
}

func TestSignedInStateNamesTheManagePageOnlyWhenItCanBeOpened(t *testing.T) {
	idp := newIdP(t)
	worker := newWorker(t, idp.srv.URL)
	s := signedIn(t, idp, worker, &memStore{}, "ra_a")
	if st := s.Status(); st.Manage != "https://accounts.example.com/account" || st.Account.Display != "ra_a@example.com" {
		t.Fatalf("signed-in state = %+v", st)
	}
	for raw, want := range map[string]string{
		"http://127.0.0.1:3100/account":      "http://127.0.0.1:3100/account",
		"http://localhost:3100/account":      "http://localhost:3100/account",
		"http://accounts.example.com":        "",
		"https://u:p@accounts.example.com/a": "",
		"javascript:alert(1)":                "",
		"file:///etc/passwd":                 "",
		"//accounts.example.com/account":     "",
		"":                                   "",
	} {
		worker.with(func() { worker.accountURL = raw })
		st, _ := s.Refresh(context.Background())
		if st.Manage != want {
			t.Errorf("account_url %q gave Manage %q, want %q", raw, st.Manage, want)
		}
	}
}

func TestDeniedAndExpiredCodesEndSignIn(t *testing.T) {
	for _, answer := range []string{"access_denied", "expired_token"} {
		idp := newIdP(t)
		idp.answers = []string{"authorization_pending", answer}
		worker := newWorker(t, idp.srv.URL)
		store := &memStore{}
		s := newService(t, idp, worker, store)
		if _, err := s.SignIn(context.Background()); err != nil {
			t.Fatal(err)
		}
		await(t, answer, func() bool { return s.Status().State == protocol.AccountSignedOut })
		st := s.Status()
		if st.Error == "" || st.Code != nil || store.kept() != nil {
			t.Fatalf("%s left %+v, record %+v", answer, st, store.kept())
		}
		polls := idp.polls.Load()
		time.Sleep(50 * time.Millisecond)
		if idp.polls.Load() != polls {
			t.Fatalf("%s: polling went on after the flow ended", answer)
		}
	}
}

func TestCancelStopsPolling(t *testing.T) {
	idp := newIdP(t)
	idp.answers = make([]string, 1000)
	for i := range idp.answers {
		idp.answers[i] = "authorization_pending"
	}
	worker := newWorker(t, idp.srv.URL)
	s := newService(t, idp, worker, &memStore{})
	if _, err := s.SignIn(context.Background()); err != nil {
		t.Fatal(err)
	}
	await(t, "a poll", func() bool { return idp.polls.Load() > 0 })
	if st := s.Cancel(); st.State != protocol.AccountSignedOut || st.Code != nil {
		t.Fatalf("after cancel %+v", st)
	}
	time.Sleep(20 * time.Millisecond)
	polls := idp.polls.Load()
	time.Sleep(80 * time.Millisecond)
	if idp.polls.Load() != polls {
		t.Fatal("polling went on after Cancel")
	}
}

func TestMissingDeviceGrantIsReportedNotFaked(t *testing.T) {
	idp := newIdP(t)
	idp.noDevice = true
	worker := newWorker(t, idp.srv.URL)
	s := newService(t, idp, worker, &memStore{})
	st, err := s.SignIn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.State != protocol.AccountSignedOut || !strings.Contains(st.Error, "Device Authorization Grant") || idp.deviceCalls.Load() != 0 {
		t.Fatalf("blocked grant gave %+v (device calls %d)", st, idp.deviceCalls.Load())
	}
}

// L9: nothing the user reads names the services behind the open client.
func TestUserTextNamesNoProvider(t *testing.T) {
	idp := newIdP(t)
	idp.noDevice = true
	worker := newWorker(t, idp.srv.URL)
	s := newService(t, idp, worker, &memStore{})
	st, _ := s.SignIn(context.Background())
	for _, word := range []string{"Clerk", "Stripe", "clerk", "stripe"} {
		if strings.Contains(st.Error, word) {
			t.Fatalf("user text names %s: %q", word, st.Error)
		}
	}
}

func signedIn(t *testing.T, idp *fakeIdP, worker *fakeWorker, store *memStore, account string) *Service {
	t.Helper()
	worker.allow("access-1", account)
	s := newService(t, idp, worker, store)
	if _, err := s.SignIn(context.Background()); err != nil {
		t.Fatal(err)
	}
	await(t, "signed in", func() bool { st := s.Status(); return st.State == protocol.AccountSignedIn && st.Balance != nil })
	return s
}

// expire makes the access token due for renewal.
func expire(s *Service) {
	s.mu.Lock()
	next := *s.record
	next.Expires = time.Now().Add(-time.Minute)
	s.record = &next
	s.mu.Unlock()
}

func TestRejectedTokenIsRenewedOnceThenSignsOut(t *testing.T) {
	idp := newIdP(t)
	worker := newWorker(t, idp.srv.URL)
	store := &memStore{}
	s := signedIn(t, idp, worker, store, "ra_a")

	// the server stops taking access-1; the refresh grant gives access-2
	worker.mu.Lock()
	delete(worker.accounts, "access-1")
	worker.accounts["access-2"] = "ra_a"
	worker.mu.Unlock()
	st, _ := s.Refresh(context.Background())
	if st.State != protocol.AccountSignedIn || st.Error != "" || worker.lastAuth.Load() != "access-2" || store.kept().Access != "access-2" {
		t.Fatalf("after renewal %+v (last auth %v)", st, worker.lastAuth.Load())
	}

	// the refresh token itself is refused: the account is forgotten
	worker.mu.Lock()
	delete(worker.accounts, "access-2")
	worker.mu.Unlock()
	idp.refreshErr = "invalid_grant"
	st, _ = s.Refresh(context.Background())
	if st.State != protocol.AccountSignedOut || st.Account != nil || st.Balance != nil || store.kept() != nil {
		t.Fatalf("after a refused refresh %+v, record %+v", st, store.kept())
	}
}

func TestSignOutCancelsHostedRequestsAndDropsLateReplies(t *testing.T) {
	idp := newIdP(t)
	worker := newWorker(t, idp.srv.URL)
	store := &memStore{}
	s := signedIn(t, idp, worker, store, "ra_a")

	if _, _, _, err := s.Endpoint(context.Background(), "ra_b"); err == nil {
		t.Fatal("a decision reviewed for B received A's credentials")
	}
	base, token, bound, err := s.Endpoint(context.Background(), "ra_a")
	if err != nil || token != "access-1" || base != worker.srv.URL+"/v1" {
		t.Fatalf("endpoint %q %q %v", base, token, err)
	}

	// A's balance read is slow; B signs in while it is in flight
	gate := make(chan struct{})
	worker.with(func() { worker.gate = gate })
	late := make(chan protocol.AccountState, 1)
	go func() { st, _ := s.Refresh(context.Background()); late <- st }()
	await(t, "A's read in flight", func() bool { return s.Status().Busy })

	s.SignOut(context.Background())
	if bound.Err() == nil {
		t.Fatal("sign-out left A's hosted request running")
	}
	worker.with(func() {
		worker.gate = nil
		worker.accounts["access-2"] = "ra_b"
	})
	idp.answers = nil
	close(gate)
	<-late
	if st := s.Status(); st.Account != nil || st.Balance != nil {
		t.Fatalf("A's late reply landed after sign-out: %+v", st)
	}
	if _, err := s.SignIn(context.Background()); err != nil {
		t.Fatal(err)
	}
	await(t, "B signed in", func() bool { st := s.Status(); return st.Account != nil && st.Account.ID == "ra_b" })
	if _, _, _, err := s.Endpoint(context.Background(), "ra_a"); err == nil {
		t.Fatal("a stale decision reviewed for A received B's credentials")
	}
	if idp.revoked.Load() != 1 {
		t.Fatalf("the refresh token was revoked %d times", idp.revoked.Load())
	}
}

func TestAccountAtAnotherServerIsNotUsed(t *testing.T) {
	idp := newIdP(t)
	worker := newWorker(t, idp.srv.URL)
	store := &memStore{record: &gateway.AccountRecord{Server: "https://old.example.com", Access: "access-old"}}
	s := newService(t, idp, worker, store)
	if st := s.Status(); st.State != protocol.AccountSignedOut {
		t.Fatalf("an account kept for another server was taken as signed in: %+v", st)
	}
	if _, _, _, err := s.Endpoint(context.Background(), ""); err == nil {
		t.Fatal("a token for another server was handed out")
	}
}

func TestServerAddressRules(t *testing.T) {
	for raw, want := range map[string]string{
		"https://Accounts.Example.com/": "https://accounts.example.com",
		"http://127.0.0.1:8787":         "http://127.0.0.1:8787",
		"http://localhost:8787":         "http://localhost:8787",
		"":                              "",
	} {
		if got, ok := protocol.AccountServer(raw); !ok || got != want {
			t.Errorf("AccountServer(%q) = %q %v, want %q", raw, got, ok, want)
		}
	}
	for _, raw := range []string{"http://example.com", "https://x.example/v1", "ftp://x", "https://u:p@x.example", "https://x.example/?a=1"} {
		if _, ok := protocol.AccountServer(raw); ok {
			t.Errorf("AccountServer(%q) accepted", raw)
		}
	}
	for units, want := range map[int64]string{0: "0.00", 9_999_999: "0.00", 10_000_000: "0.01", 1_999_999_999: "1.99", 230_000_000_000: "230.00"} {
		if got := protocol.AccountCredits(units, 1_000_000_000); got != want {
			t.Errorf("AccountCredits(%d) = %s, want %s", units, got, want)
		}
	}
}

// M1: the identity provider a server first signed in with is kept per
// server, survives sign-out, and a later change is refused until the server
// address is removed and added again.
func TestIssuerIsPinnedPerServerAndKeptAcrossSignOut(t *testing.T) {
	idp := newIdP(t)
	worker := newWorker(t, idp.srv.URL)
	store := &memStore{}
	s := signedIn(t, idp, worker, store, "ra_a")
	if got, _ := store.AccountIssuer(worker.srv.URL); got != idp.srv.URL {
		t.Fatalf("pinned issuer = %q, want %q", got, idp.srv.URL)
	}
	s.SignOut(context.Background())
	if got, _ := store.AccountIssuer(worker.srv.URL); got != idp.srv.URL {
		t.Fatalf("sign-out dropped the pin: %q", got)
	}

	// the server now names another issuer: the real one's public id would
	// get a token the hostile server could use
	other := newIdP(t)
	worker.with(func() { worker.issuer = other.srv.URL })
	st, _ := s.SignIn(context.Background())
	if st.State != protocol.AccountSignedOut || !strings.Contains(st.Error, "another identity provider") || !strings.Contains(st.Error, "add it again") {
		t.Fatalf("sign-in at a changed issuer gave %+v", st)
	}
	if other.deviceCalls.Load() != 0 {
		t.Fatal("the new issuer was asked for a code before the user agreed to it")
	}

	// removing the address and adding it again is the user's yes
	t.Setenv(EnvServer, "http://127.0.0.1:1")
	s.SettingsChanged()
	if got, _ := store.AccountIssuer(worker.srv.URL); got != "" {
		t.Fatalf("leaving the server kept its pin %q", got)
	}
	t.Setenv(EnvServer, worker.srv.URL)
	s.SettingsChanged()
	st, _ = s.SignIn(context.Background())
	if st.State != protocol.AccountSigningIn || other.deviceCalls.Load() != 1 {
		t.Fatalf("sign-in after re-adding gave %+v (device calls %d)", st, other.deviceCalls.Load())
	}
	await(t, "signed in at the new issuer", func() bool { return s.Status().State == protocol.AccountSignedIn })
	if got, _ := store.AccountIssuer(worker.srv.URL); got != other.srv.URL {
		t.Fatalf("pin after re-adding = %q", got)
	}
}

// M4: a caller that gives up must not lose the rotated refresh token.
func TestRenewalOutlivesACancelledCaller(t *testing.T) {
	idp := newIdP(t)
	worker := newWorker(t, idp.srv.URL)
	store := &memStore{}
	s := signedIn(t, idp, worker, store, "ra_a")
	expire(s)
	gate := make(chan struct{})
	idp.set(func() { idp.refreshGate = gate })
	ctx, cancel := context.WithCancel(context.Background())
	failed := make(chan error, 1)
	go func() { _, _, _, err := s.token(ctx, false); failed <- err }()
	await(t, "the refresh in flight", func() bool { return idp.refreshCalls.Load() == 1 })
	cancel()
	if err := <-failed; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled caller got %v", err)
	}
	close(gate)
	await(t, "the rotated pair kept", func() bool { kept := store.kept(); return kept != nil && kept.Refresh == "refresh-2" })
	if st := s.Status(); st.State != protocol.AccountSignedIn {
		t.Fatalf("a cancelled caller signed the account out: %+v", st)
	}
	if _, token, _, err := s.token(context.Background(), false); err != nil || token != "access-2" || idp.refreshCalls.Load() != 1 {
		t.Fatalf("next caller got %q, %v after %d refreshes", token, err, idp.refreshCalls.Load())
	}
}

func TestRotatedPairSurvivesAStoreWriteFailureAndIsSavedLater(t *testing.T) {
	idp := newIdP(t)
	worker := newWorker(t, idp.srv.URL)
	store := &memStore{}
	s := signedIn(t, idp, worker, store, "ra_a")
	expire(s)
	store.fail(true)
	_, token, _, err := s.token(context.Background(), false)
	if err != nil || token != "access-2" {
		t.Fatalf("renewal with a failing store = %q, %v", token, err)
	}
	s.mu.Lock()
	inMemory := s.record.Refresh
	s.mu.Unlock()
	if inMemory != "refresh-2" || store.kept().Refresh != "refresh-1" {
		t.Fatalf("memory holds %q and the store %q", inMemory, store.kept().Refresh)
	}
	store.fail(false)
	if _, _, _, err := s.token(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if kept := store.kept(); kept.Refresh != "refresh-2" || kept.Access != "access-2" {
		t.Fatalf("the next save did not retry: %+v", kept)
	}
	if idp.refreshCalls.Load() != 1 {
		t.Fatalf("the pair was renewed %d times", idp.refreshCalls.Load())
	}
}

// L1: a redirect from the server or the provider is an answer, never a
// place to go.
func TestRedirectsAreNotFollowed(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"error":"x"}`))
	}))
	defer target.Close()

	idp := newIdP(t)
	worker := newWorker(t, idp.srv.URL)
	idp.set(func() { idp.redirectDevice = target.URL })
	s := newService(t, idp, worker, &memStore{})
	st, _ := s.SignIn(context.Background())
	if st.State != protocol.AccountSignedOut || st.Error == "" || hits.Load() != 0 {
		t.Fatalf("a redirected device request gave %+v, target hits %d", st, hits.Load())
	}

	idp.set(func() { idp.redirectDevice = "" })
	s = signedIn(t, idp, worker, &memStore{}, "ra_a")
	worker.with(func() { worker.redirect = target.URL })
	st, _ = s.Refresh(context.Background())
	if st.Error == "" || hits.Load() != 0 {
		t.Fatalf("a redirected account read gave %+v, target hits %d", st, hits.Load())
	}
}

func TestIdentityProviderOnTheLocalNetworkIsRefusedForARemoteServer(t *testing.T) {
	idp := newIdP(t)
	worker := newWorker(t, idp.srv.URL)
	s := newService(t, idp, worker, &memStore{})
	for _, issuer := range []string{"https://127.0.0.1", "https://192.168.1.5", "https://10.0.0.7:8443", "https://169.254.169.254", "https://[::1]", "https://localhost", "https://printer.local", "https://[::ffff:127.0.0.1]"} {
		_, err := s.metadata(context.Background(), "https://accounts.example.com", issuer)
		var refused *fail
		if !errors.As(err, &refused) || !strings.Contains(refused.text, "on this computer or its network") {
			t.Errorf("issuer %s for a remote server: %v", issuer, err)
		}
	}
	// a server run here may use a provider here
	if _, err := s.metadata(context.Background(), worker.srv.URL, idp.srv.URL); err != nil {
		t.Fatalf("local server with a local provider: %v", err)
	}
	for host, want := range map[string]bool{"accounts.example.com": false, "8.8.8.8": false, "2606:4700::1111": false, "172.16.0.1": true, "fe80::1": true, "0.0.0.0": true} {
		if internalHost(host) != want {
			t.Errorf("internalHost(%q) = %v", host, !want)
		}
	}
}

// L2: what a server says reaches the user clean and short.
func TestServerWordsReachTheUserCleanAndShort(t *testing.T) {
	idp := newIdP(t)
	worker := newWorker(t, idp.srv.URL)
	s := signedIn(t, idp, worker, &memStore{}, "ra_a")
	hostile := "\u202eSign-in ended\u202c, open \x1b[31mevil.example\x07 now\u200b " + strings.Repeat("blah ", 200)
	worker.with(func() {
		worker.refuse, worker.refuseMessage = http.StatusForbidden, hostile
		worker.notice = hostile
	})
	st, _ := s.Refresh(context.Background())
	for _, text := range []string{st.Error, st.Notice} {
		if text == "" || utf8.RuneCountInString(text) > noticeMax {
			t.Fatalf("server text %q is empty or not cut", text)
		}
		for _, r := range text {
			if r == '\u202e' || r == '\u202c' || r == '\u200b' || r < ' ' || r == 0x7f {
				t.Fatalf("server text keeps %U: %q", r, text)
			}
		}
	}
	if utf8.RuneCountInString(st.Error) > protocol.ServerTextMax {
		t.Fatalf("error has %d runes", utf8.RuneCountInString(st.Error))
	}
}

func TestServiceUnavailableIsToldAndDoesNotSignOut(t *testing.T) {
	idp := newIdP(t)
	worker := newWorker(t, idp.srv.URL)
	store := &memStore{}
	s := signedIn(t, idp, worker, store, "ra_a")
	worker.with(func() { worker.refuse, worker.refuseMessage = http.StatusServiceUnavailable, "" })
	st, _ := s.Refresh(context.Background())
	if st.State != protocol.AccountSignedIn || !strings.Contains(st.Error, "right now") || store.kept() == nil || idp.revoked.Load() != 0 {
		t.Fatalf("a 503 gave %+v", st)
	}
}

// L3: the approval page is parsed, has no credentials, and is opened in the
// form the parser gives back.
func TestVerificationPagesAreParsed(t *testing.T) {
	for _, bad := range []string{"https://user:pw@login.example.com/device", "http://login.example.com/device", "javascript:alert(1)", "https:///device", "https://x.example/%zz"} {
		idp := newIdP(t)
		worker := newWorker(t, idp.srv.URL)
		idp.set(func() { idp.verification = bad })
		s := newService(t, idp, worker, &memStore{})
		st, _ := s.SignIn(context.Background())
		if st.State != protocol.AccountSignedOut || st.Code != nil || st.Error == "" {
			t.Errorf("approval page %q gave %+v", bad, st)
		}
	}
	idp := newIdP(t)
	worker := newWorker(t, idp.srv.URL)
	idp.set(func() { idp.complete = "https://u:p@login.example.com/device?c=1" })
	s := newService(t, idp, worker, &memStore{})
	st, _ := s.SignIn(context.Background())
	if st.Code == nil || st.Code.VerificationURIComplete != "" || st.Code.VerificationURI != idp.srv.URL+"/device" {
		t.Fatalf("code = %+v", st.Code)
	}
}

// L4: signing out and changing server revoke first and wipe after.
func TestSignOutRevokesBeforeItReturnsAndBeforeTheRecordIsWiped(t *testing.T) {
	idp := newIdP(t)
	worker := newWorker(t, idp.srv.URL)
	store := &memStore{}
	s := signedIn(t, idp, worker, store, "ra_a")
	var keptWhileRevoking atomic.Bool
	idp.set(func() { idp.onRevoke = func() { keptWhileRevoking.Store(store.kept() != nil) } })
	s.SignOut(context.Background())
	if idp.revoked.Load() != 1 || !keptWhileRevoking.Load() || store.kept() != nil {
		t.Fatalf("revoked %d, record kept while revoking %v, record after %+v", idp.revoked.Load(), keptWhileRevoking.Load(), store.kept())
	}
}

func TestChangingServerRevokesTheOldAccountFirst(t *testing.T) {
	idp := newIdP(t)
	worker := newWorker(t, idp.srv.URL)
	store := &memStore{}
	s := signedIn(t, idp, worker, store, "ra_a")
	t.Setenv(EnvServer, "http://127.0.0.1:1")
	s.SettingsChanged()
	if idp.revoked.Load() != 1 || store.kept() != nil || s.Status().Server != "http://127.0.0.1:1" || s.Status().State != protocol.AccountSignedOut {
		t.Fatalf("revoked %d, record %+v, state %+v", idp.revoked.Load(), store.kept(), s.Status())
	}
}

// L5: a sign-in that began for one server never ends at another.
func TestSignInDoesNotSurviveAServerChange(t *testing.T) {
	idp := newIdP(t)
	worker := newWorker(t, idp.srv.URL)
	store := &memStore{}
	s := newService(t, idp, worker, store)
	gate := make(chan struct{})
	idp.set(func() { idp.deviceGate = gate })
	done := make(chan protocol.AccountState, 1)
	go func() { st, _ := s.SignIn(context.Background()); done <- st }()
	await(t, "the code requested", func() bool { return idp.deviceCalls.Load() == 1 })
	t.Setenv(EnvServer, "http://127.0.0.1:1")
	s.SettingsChanged()
	close(gate)
	st := <-done
	if st.State == protocol.AccountSigningIn || st.Code != nil || st.Server != "http://127.0.0.1:1" {
		t.Fatalf("sign-in went on for the old server: %+v", st)
	}
	time.Sleep(50 * time.Millisecond)
	if idp.polls.Load() != 0 || store.kept() != nil || s.Status().Code != nil {
		t.Fatalf("polls %d, record %+v, state %+v", idp.polls.Load(), store.kept(), s.Status())
	}
}

func TestApprovalForAStaleAccountIsNotKept(t *testing.T) {
	idp := newIdP(t)
	worker := newWorker(t, idp.srv.URL)
	store := &memStore{}
	s := newService(t, idp, worker, store)
	s.mu.Lock()
	stale := s.gen
	s.gen++
	s.mu.Unlock()
	s.finishSignIn(context.Background(), stale, worker.srv.URL, &gateway.AccountRecord{Server: worker.srv.URL, Issuer: idp.srv.URL, Access: "late"})
	if store.kept() != nil || s.Status().State == protocol.AccountSignedIn {
		t.Fatalf("a stale approval was kept: %+v, %+v", store.kept(), s.Status())
	}
	s.mu.Lock()
	gen := s.gen
	s.mu.Unlock()
	s.finishSignIn(context.Background(), gen, "https://elsewhere.example.com", &gateway.AccountRecord{Server: "https://elsewhere.example.com", Access: "late"})
	if store.kept() != nil {
		t.Fatal("an approval for another server was kept")
	}
}

// L6: hosted requests cannot start a read or a refresh per request.
func TestUsedAndRejectedAreCoalesced(t *testing.T) {
	idp := newIdP(t)
	worker := newWorker(t, idp.srv.URL)
	s := signedIn(t, idp, worker, &memStore{}, "ra_a")
	for _, token := range []string{"access-2", "access-3", "access-4", "access-5", "access-6", "access-7", "access-8", "access-9"} {
		worker.allow(token, "ra_a")
	}
	s.reads.gap = 150 * time.Millisecond
	s.renewals.gap = 150 * time.Millisecond
	reads := worker.reads.Load()
	for range 60 {
		s.Used()
		s.Rejected()
	}
	time.Sleep(450 * time.Millisecond)
	if got := worker.reads.Load() - reads; got < 1 || got > 4 {
		t.Fatalf("60 requests made %d balance reads", got)
	}
	if got := idp.refreshCalls.Load(); got < 1 || got > 4 {
		t.Fatalf("60 rejections made %d forced refreshes", got)
	}
}

func TestCoalescerRunsOneAtATimeAndOnceMoreAfterABurst(t *testing.T) {
	var running, peak, runs atomic.Int32
	c := &coalescer{gap: 20 * time.Millisecond}
	job := func() {
		peak.Store(max(peak.Load(), running.Add(1)))
		runs.Add(1)
		time.Sleep(10 * time.Millisecond)
		running.Add(-1)
	}
	c.run(job)
	await(t, "the first run", func() bool { return running.Load() == 1 })
	for range 100 {
		c.run(job)
	}
	await(t, "the burst to settle", func() bool { c.mu.Lock(); defer c.mu.Unlock(); return !c.running })
	if peak.Load() != 1 || runs.Load() != 2 {
		t.Fatalf("peak %d concurrent runs, %d runs for one burst", peak.Load(), runs.Load())
	}
}

// L7: numbers a server sends cannot wrap.
func TestServerNumbersAreClamped(t *testing.T) {
	idp := newIdP(t)
	idp.set(func() { idp.expiresIn = 9_000_000_000_000 })
	worker := newWorker(t, idp.srv.URL)
	store := &memStore{}
	signedIn(t, idp, worker, store, "ra_a")
	left := time.Until(store.kept().Expires)
	if left <= time.Hour || left > 24*time.Hour+time.Minute {
		t.Fatalf("a huge expires_in gave a token good for %s", left)
	}
	for seconds, want := range map[int]time.Duration{-5: time.Minute, 0: time.Minute, 30: time.Minute, 3600: time.Hour, math.MaxInt: 24 * time.Hour} {
		if got := tokenTTL(seconds); got != want {
			t.Errorf("tokenTTL(%d) = %s, want %s", seconds, got, want)
		}
	}
	for _, c := range []struct {
		units, per int64
		want       string
	}{
		{math.MaxInt64, 1, "9223372036854775807.00"},
		{math.MaxInt64, math.MaxInt64, "1.00"},
		{math.MaxInt64 - 1, math.MaxInt64, "0.99"},
		{math.MinInt64, 1, "-9223372036854775808.00"},
		{math.MinInt64, math.MaxInt64, "-1.00"},
		{-1_500_000_000, 1_000_000_000, "-1.50"},
		{5, 0, "5.00"},
		{5, -3, "5.00"},
	} {
		if got := protocol.AccountCredits(c.units, c.per); got != c.want {
			t.Errorf("AccountCredits(%d, %d) = %s, want %s", c.units, c.per, got, c.want)
		}
	}
}

// L8: a failure that names no error is not an approval.
func TestPollRetriesAnAnswerThatNamesNoError(t *testing.T) {
	idp := newIdP(t)
	idp.answers = []string{"503", "authorization_pending", "503"}
	worker := newWorker(t, idp.srv.URL)
	worker.allow("access-1", "ra_a")
	store := &memStore{}
	s := newService(t, idp, worker, store)
	if _, err := s.SignIn(context.Background()); err != nil {
		t.Fatal(err)
	}
	await(t, "signed in after the failures", func() bool { return s.Status().State == protocol.AccountSignedIn })
	if idp.polls.Load() != 4 || store.kept() == nil || store.kept().Access != "access-1" {
		t.Fatalf("polls %d, record %+v", idp.polls.Load(), store.kept())
	}
}

// The scope names the server by its origin; the example is the contract the
// Worker derives the same value from.
func TestServerScopeIsDerivedFromTheOrigin(t *testing.T) {
	const local = "ryoku-server-b1a61bf29a38ff3642af0dad0785e167"
	if got := ServerScope("http://127.0.0.1:8787"); got != local {
		t.Fatalf("ServerScope(local) = %q, want %q", got, local)
	}
	same := map[string][]string{
		"https://accounts.example.com": {"https://accounts.example.com/", "HTTPS://Accounts.Example.COM", "https://accounts.example.com:443", " https://accounts.example.com "},
		"http://127.0.0.1:80":          {"http://127.0.0.1", "http://127.0.0.1:80/"},
		"https://[::1]:8443":           {"https://[::1]:8443/", "HTTPS://[::1]:8443"},
	}
	for base, others := range same {
		want := ServerScope(base)
		if !strings.HasPrefix(want, "ryoku-server-") || len(want) != len("ryoku-server-")+32 {
			t.Fatalf("ServerScope(%q) = %q is not prefix plus 32 hex digits", base, want)
		}
		for _, other := range others {
			if got := ServerScope(other); got != want {
				t.Errorf("ServerScope(%q) = %q, want %q like %q", other, got, want, base)
			}
		}
	}
	// the port, the scheme and the host each make another server
	distinct := []string{"https://accounts.example.com", "https://accounts.example.com:8443", "http://accounts.example.com", "https://accounts.example.org", "https://[::1]:8443", "https://[::2]:8443"}
	seen := map[string]string{}
	for _, server := range distinct {
		scope := ServerScope(server)
		if before, dup := seen[scope]; dup {
			t.Errorf("%q and %q share the scope %q", before, server, scope)
		}
		seen[scope] = server
	}
	if got := ServerScope(""); got != "" {
		t.Errorf("ServerScope(\"\") = %q, want none", got)
	}
}

// The device authorization asks for the server's scopes and the binding of
// the address Rashin was given; a server cannot name its own binding, and
// another address binds to another scope.
func TestDeviceAuthorizationRequestsTheServerBinding(t *testing.T) {
	idp := newIdP(t)
	worker := newWorker(t, idp.srv.URL)
	// a server that names the binding of another server, to be heard as one
	worker.scopes = "openid offline_access " + ServerScope("https://victim.example.com")
	s := newService(t, idp, worker, &memStore{})
	if _, err := s.SignIn(context.Background()); err != nil {
		t.Fatal(err)
	}
	idp.mu.Lock()
	scopes := append([]string(nil), idp.scopes...)
	idp.mu.Unlock()
	want := "openid offline_access " + ServerScope(worker.srv.URL)
	if len(scopes) != 1 || scopes[0] != want {
		t.Fatalf("requested scopes %q, want [%q]", scopes, want)
	}
	if ServerScope(worker.srv.URL) == ServerScope("https://victim.example.com") {
		t.Fatal("the test servers share a scope")
	}

	// another address, another binding
	other := newWorker(t, idp.srv.URL)
	s2 := newService(t, idp, other, &memStore{})
	if _, err := s2.SignIn(context.Background()); err != nil {
		t.Fatal(err)
	}
	idp.mu.Lock()
	scopes = append([]string(nil), idp.scopes...)
	idp.mu.Unlock()
	if len(scopes) != 2 || scopes[1] == scopes[0] || !strings.HasSuffix(scopes[1], ServerScope(other.srv.URL)) {
		t.Fatalf("a second server asked for %q after %q", scopes[1:], scopes[:1])
	}
}

// An issuer that does not know the server's scope refuses at the start, and
// the user is told to use the address from ryoku.dev.
func TestUnknownServerScopeSaysTheServerIsNotRegistered(t *testing.T) {
	idp := newIdP(t)
	idp.set(func() { idp.invalidScope = true })
	worker := newWorker(t, idp.srv.URL)
	s := newService(t, idp, worker, &memStore{})
	st, err := s.SignIn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := "This server is not registered for Ryoku sign-in. Use the server address from ryoku.dev."
	if st.State != protocol.AccountSignedOut || st.Error != want || st.Code != nil {
		t.Fatalf("an unregistered server gave %+v", st)
	}
}

// countingTransport fails and counts every request the service makes.
type countingTransport struct{ calls atomic.Int32 }

func (c *countingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	c.calls.Add(1)
	return nil, errors.New("no network in this test")
}

// A fresh install points at Ryoku's own server, binds sign-in to its scope,
// hands Ember its /v1 base, and reads nothing from the network until the
// user signs in.
func TestFreshInstallUsesRyokuServerWithoutContactingIt(t *testing.T) {
	t.Setenv(EnvServer, "")
	settings := func() protocol.Settings { return config.Defaults() }
	s := New(settings, &memStore{})
	transport := &countingTransport{}
	s.client = &http.Client{Transport: transport}
	if got := s.Server(); got != "https://ai.ryoku.dev" {
		t.Fatalf("Server() = %q, want https://ai.ryoku.dev", got)
	}
	if got := ServerScope(s.Server()); got != "ryoku-server-88a4e8aff84f775d543d899aa62b4bb1" {
		t.Fatalf("ServerScope(production) = %q", got)
	}
	st, err := s.Refresh(context.Background())
	if err != nil || st.State != protocol.AccountSignedOut || st.Server != "https://ai.ryoku.dev" {
		t.Fatalf("fresh state %+v, %v", st, err)
	}
	if s.Models() != nil {
		t.Fatal("hosted models appeared without an account")
	}
	if _, _, _, err := s.Endpoint(context.Background(), ""); err == nil {
		t.Fatal("Ember was reachable without an account")
	}
	if n := transport.calls.Load(); n != 0 {
		t.Fatalf("a signed-out install made %d requests", n)
	}

	store := &memStore{record: &gateway.AccountRecord{Server: "https://ai.ryoku.dev", Access: "access-1", Expires: time.Now().Add(time.Hour)}}
	kept := New(settings, store)
	kept.client = &http.Client{Transport: transport}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	base, token, _, err := kept.Endpoint(ctx, "")
	if err != nil || base != "https://ai.ryoku.dev/v1" || token != "access-1" {
		t.Fatalf("Ember endpoint %q %q %v", base, token, err)
	}
	if n := transport.calls.Load(); n != 0 {
		t.Fatalf("loading a kept account made %d requests", n)
	}
}

// RYOKU_ACCOUNTS_URL wins over the default and over a saved server; a user
// who turned the service off stays unconfigured without it.
func TestEnvironmentServerOverridesTheSettings(t *testing.T) {
	const custom = "https://accounts.example.com"
	t.Setenv(EnvServer, custom)
	for _, saved := range []string{config.DefaultAccountServer, "https://other.example.com", ""} {
		settings := config.Defaults()
		settings.Providers.Account = saved
		s := New(func() protocol.Settings { return settings }, &memStore{})
		if got := s.Server(); got != custom {
			t.Errorf("saved %q: Server() = %q, want %q", saved, got, custom)
		}
	}
	t.Setenv(EnvServer, "")
	off := config.Defaults()
	off.Providers.Account = ""
	if st := New(func() protocol.Settings { return off }, &memStore{}).Status(); st.State != protocol.AccountUnconfigured || st.Server != "" {
		t.Fatalf("a cleared server gave %+v", st)
	}
}
