// Package account is the Ryoku Account on this computer: signing in with a
// code approved in the browser (OAuth 2.0 Device Authorization Grant, RFC
// 8628, as a public client with no secret), keeping its tokens sealed in the
// gateway store, and reading the account and its credits from the Ryoku
// Accounts server. The server owns every balance; this package only reads
// it, and buying, usage and the subscription live on the website the server
// names. Tokens never leave the daemon: the app sees AccountState, which
// holds none.
package account

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/gateway"
	"ryoku-rashin/internal/protocol"
)

// EnvServer overrides the server address in the settings.
const EnvServer = "RYOKU_ACCOUNTS_URL"

const (
	requestTimeout = 20 * time.Second
	// revokeTimeout is how long signing out waits for the identity provider
	revokeTimeout = 3 * time.Second
	// refreshEarly renews an access token this long before it expires
	refreshEarly = 2 * time.Minute
	maxBody      = 1 << 20
	deviceGrant  = "urn:ietf:params:oauth:grant-type:device_code"
	// the longest a polling interval or a slow_down can make the wait
	maxInterval = 60
	// hostedModelsMax bounds the models a server can list
	hostedModelsMax = 32
	noticeMax       = 400
)

// Store keeps the sealed account record and, per server, the identity
// provider it first signed in with; the gateway implements it.
type Store interface {
	AccountTokens() (*gateway.AccountRecord, error)
	SetAccountTokens(*gateway.AccountRecord) error
	AccountIssuer(server string) (string, error)
	SetAccountIssuer(server, issuer string) error
}

// Service is the one Ryoku Account of this daemon.
type Service struct {
	settings func() protocol.Settings
	store    Store
	client   *http.Client
	now      func() time.Time
	// second is the provider's polling unit; tests shorten it
	second time.Duration

	mu        sync.Mutex
	state     protocol.AccountState
	record    *gateway.AccountRecord
	gen       uint64
	flow      context.CancelFunc
	listeners []func()
	bound     []boundRequest
	// unsaved is a record that is newer in memory than in the store
	unsaved bool
	// refreshMu makes one token refresh at a time
	refreshMu sync.Mutex
	// reads and renewals are what hosted requests ask for in the background,
	// each at most once at a time and not again within its gap
	reads    coalescer
	renewals coalescer
}

type boundRequest struct {
	gen    uint64
	ctx    context.Context
	cancel context.CancelFunc
}

// New loads a kept account. Nothing is read from the network until the app
// asks or a hosted request needs a token.
func New(settings func() protocol.Settings, store Store) *Service {
	s := &Service{
		settings: settings,
		store:    store,
		client:   &http.Client{Timeout: requestTimeout, Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, MaxIdleConnsPerHost: 2, IdleConnTimeout: 60 * time.Second}},
		now:      time.Now,
		second:   time.Second,
	}
	s.reads.gap = 5 * time.Second
	s.renewals.gap = 30 * time.Second
	s.load()
	return s
}

// Server is the configured server address, "" for none.
func (s *Service) Server() string {
	if v, ok := protocol.AccountServer(os.Getenv(EnvServer)); ok && v != "" {
		return v
	}
	v, _ := protocol.AccountServer(s.settings().Providers.Account)
	return v
}

func (s *Service) load() {
	record, _ := s.store.AccountTokens()
	s.mu.Lock()
	defer s.mu.Unlock()
	server := s.Server()
	s.state = protocol.AccountState{State: protocol.AccountSignedOut, Server: server}
	if server == "" {
		s.state.State = protocol.AccountUnconfigured
	}
	if record != nil && record.Server == server && server != "" {
		s.record = record
		s.state.State = protocol.AccountSignedIn
	}
}

// OnChange is called after every state change, on no particular goroutine.
func (s *Service) OnChange(fn func()) {
	s.mu.Lock()
	s.listeners = append(s.listeners, fn)
	s.mu.Unlock()
}

func (s *Service) changed() {
	s.mu.Lock()
	listeners := append([]func(){}, s.listeners...)
	s.mu.Unlock()
	for _, fn := range listeners {
		fn()
	}
}

// Status is the state as last read.
func (s *Service) Status() protocol.AccountState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}

func (s *Service) snapshotLocked() protocol.AccountState {
	st := s.state
	st.Models = append([]protocol.AccountModel(nil), st.Models...)
	if st.Code != nil {
		code := *st.Code
		st.Code = &code
	}
	if st.Account != nil {
		a := *st.Account
		st.Account = &a
	}
	if st.Balance != nil {
		b := *st.Balance
		st.Balance = &b
	}
	return st
}

// SettingsChanged follows a new server address: an account signed in at
// another server is revoked and forgotten here, and the identity provider
// remembered for the old address goes with it, which is how a user accepts
// a server that changed its provider.
func (s *Service) SettingsChanged() {
	server := s.Server()
	s.mu.Lock()
	old := s.state.Server
	s.mu.Unlock()
	if old == server {
		return
	}
	s.forget("", true)
	if old != "" {
		_ = s.store.SetAccountIssuer(old, "")
	}
	s.mu.Lock()
	s.state = protocol.AccountState{State: protocol.AccountSignedOut, Server: server}
	if server == "" {
		s.state.State = protocol.AccountUnconfigured
	}
	s.mu.Unlock()
	s.changed()
}

// serverScopePrefix starts the one scope that binds a sign-in to a server.
const serverScopePrefix = "ryoku-server-"

// ServerScope is the scope that binds a sign-in to one server: a fixed
// prefix and the first 32 hex digits of the SHA-256 of the server's origin
// (lowercase scheme://host[:port], default port left out, IPv6 in brackets).
// The identity provider only knows the scopes its owner created, so a
// look-alike server's scope does not exist and the sign-in is refused at the
// start. Rashin derives it from the address it was given, never from what a
// server says about itself. "" for an address with no host.
func ServerScope(server string) string {
	u, err := url.Parse(strings.TrimSpace(server))
	if err != nil || u.Hostname() == "" {
		return ""
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port := u.Port(); port != "" && !(scheme == "http" && port == "80") && !(scheme == "https" && port == "443") {
		host += ":" + port
	}
	sum := sha256.Sum256([]byte(scheme + "://" + host))
	return serverScopePrefix + hex.EncodeToString(sum[:16])
}

// requestedScopes is what the server asks for, minus any server binding it
// names itself, plus the binding Rashin derived.
func requestedScopes(offered, server string) (string, bool) {
	bound := ServerScope(server)
	if bound == "" {
		return "", false
	}
	var scopes []string
	for _, scope := range strings.Fields(offered) {
		if !strings.HasPrefix(scope, serverScopePrefix) {
			scopes = append(scopes, scope)
		}
	}
	return strings.Join(append(scopes, bound), " "), true
}

// serverConfig is the server's public description of itself.
type serverConfig struct {
	Auth struct {
		Issuer   string `json:"issuer"`
		ClientID string `json:"client_id"`
		Scopes   string `json:"scopes"`
	} `json:"auth"`
	UnitsPerCredit int64                   `json:"units_per_credit"`
	Models         []protocol.AccountModel `json:"models"`
	Notice         string                  `json:"notice"`
	AccountURL     string                  `json:"account_url"`
}

// shown is what the server's description gives the dialog, as a user may
// read it.
func (c serverConfig) shown() (models []protocol.AccountModel, notice, manage string) {
	for _, m := range c.Models {
		id := protocol.ServerText(m.ID, 100)
		if id == "" || id != m.ID || strings.ContainsAny(id, " ") || len(models) == hostedModelsMax {
			continue
		}
		name := protocol.ServerText(m.Name, 60)
		if name == "" {
			name = id
		}
		// a mark is a short lowercase word the app's logo set knows; anything
		// else falls back to the provider's own
		logo := m.Logo
		if len(logo) > 32 || strings.IndexFunc(logo, func(r rune) bool { return (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' }) >= 0 {
			logo = ""
		}
		models = append(models, protocol.AccountModel{ID: id, Name: name, Logo: logo})
	}
	return models, protocol.ServerText(c.Notice, noticeMax), protocol.AccountManage(c.AccountURL)
}

type authMetadata struct {
	Issuer              string   `json:"issuer"`
	DeviceAuthorization string   `json:"device_authorization_endpoint"`
	Token               string   `json:"token_endpoint"`
	Revocation          string   `json:"revocation_endpoint"`
	GrantTypes          []string `json:"grant_types_supported"`
}

// fail is an error whose text is already for the user.
type fail struct{ text string }

func (f *fail) Error() string { return f.text }

func failf(format string, args ...any) error { return &fail{text: i18n.Tf(format, args...)} }

func userText(err error) string {
	var f *fail
	if errors.As(err, &f) {
		return f.text
	}
	if errors.Is(err, context.Canceled) {
		return ""
	}
	return i18n.T("The Ryoku Accounts server could not be reached. Check your connection and try again.")
}

func (s *Service) http() *http.Client { return noRedirects(s.client) }

func (s *Service) getJSON(ctx context.Context, rawURL, token string, out any) (int, error) {
	return s.do(ctx, http.MethodGet, rawURL, token, nil, out)
}

func (s *Service) do(ctx context.Context, method, rawURL, token string, body any, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = strings.NewReader(string(data))
	}
	request, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
	if err != nil {
		return 0, err
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := s.http().Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxBody))
	if err != nil {
		return response.StatusCode, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return response.StatusCode, serverError(response.StatusCode, data)
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return response.StatusCode, failf("The Ryoku Accounts server answered with something unreadable.")
		}
	}
	return response.StatusCode, nil
}

// serverError keeps only the server's own message, which the Ryoku server
// writes for users, cleaned and cut; anything else becomes a plain sentence.
func serverError(status int, data []byte) error {
	var envelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &envelope) == nil {
		if text := protocol.ServerText(envelope.Error.Message, protocol.ServerTextMax); text != "" {
			return &fail{text: text}
		}
	}
	if status == http.StatusServiceUnavailable {
		return failf("The Ryoku Accounts server cannot check your account right now. Try again in a moment.")
	}
	return failf("The Ryoku Accounts server answered %d.", status)
}

func (s *Service) config(ctx context.Context, server string) (serverConfig, error) {
	var cfg serverConfig
	if _, err := s.getJSON(ctx, server+"/v1/config", "", &cfg); err != nil {
		return cfg, err
	}
	if cfg.UnitsPerCredit <= 0 {
		cfg.UnitsPerCredit = 1
	}
	return cfg, nil
}

// checkIssuer holds a server to the identity provider it first signed in
// with. Trust is on first use: the first sign-in keeps the issuer, and a
// server that later names another is refused until its address is removed
// from the settings and added again.
func (s *Service) checkIssuer(server, issuer string) error {
	pinned, _ := s.store.AccountIssuer(server)
	if pinned == "" || pinned == issuerKey(issuer) {
		return nil
	}
	return failf("Sign-in is blocked: this server now names another identity provider (%s) than the one you first signed in with (%s). If you expected that, remove the server address in Settings and add it again.", displayHost(issuer), displayHost(pinned))
}

func (s *Service) metadata(ctx context.Context, server, issuer string) (authMetadata, error) {
	var meta authMetadata
	u, err := url.Parse(issuer)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return meta, failf("Sign-in is blocked: the server names an identity provider that is not https (%s).", displayHost(issuer))
	}
	if internalHost(u.Hostname()) && !serverIsLoopback(server) {
		return meta, failf("Sign-in is blocked: the server names an identity provider on this computer or its network (%s).", displayHost(issuer))
	}
	if _, err := s.getJSON(ctx, issuerKey(issuer)+"/.well-known/oauth-authorization-server", "", &meta); err != nil {
		return meta, err
	}
	if issuerKey(meta.Issuer) != issuerKey(issuer) {
		return meta, failf("Sign-in is blocked: %s describes itself as another issuer.", displayHost(issuer))
	}
	supported := false
	for _, grant := range meta.GrantTypes {
		supported = supported || grant == deviceGrant
	}
	if !supported || meta.DeviceAuthorization == "" {
		return meta, failf("Sign-in is blocked: %s does not offer the OAuth Device Authorization Grant. Turn on the device authorization grant for the Ryoku Account application at the identity provider.", displayHost(issuer))
	}
	for _, endpoint := range []string{meta.DeviceAuthorization, meta.Token} {
		if !sameHost(endpoint, issuer) {
			return meta, failf("Sign-in is blocked: %s names endpoints on another host.", displayHost(issuer))
		}
	}
	return meta, nil
}

func (s *Service) setBusy(busy bool, errText string) {
	s.mu.Lock()
	s.state.Busy = busy
	if errText != "" || !busy {
		s.state.Error = errText
	}
	s.mu.Unlock()
	s.changed()
}

// currentLocked is whether the account is still the one a flow began for:
// the same generation at the same server.
func (s *Service) currentLocked(gen uint64, server string) bool {
	return s.gen == gen && s.Server() == server
}

// SignIn asks the identity provider for a code and waits in the background
// for the user to approve it in the browser. It returns once the code is
// there to show, or why it is not.
func (s *Service) SignIn(ctx context.Context) (protocol.AccountState, error) {
	server := s.Server()
	if server == "" {
		return s.Status(), protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Set the Ryoku Accounts server first."))
	}
	s.mu.Lock()
	if s.state.State == protocol.AccountSigningIn {
		st := s.snapshotLocked()
		s.mu.Unlock()
		return st, nil
	}
	signedIn := s.state.State == protocol.AccountSignedIn
	s.mu.Unlock()
	if signedIn {
		s.forget("", true)
	}
	s.mu.Lock()
	gen := s.gen
	s.mu.Unlock()
	s.setBusy(true, "")
	cfg, err := s.config(ctx, server)
	if err == nil {
		err = s.checkIssuer(server, cfg.Auth.Issuer)
	}
	var meta authMetadata
	if err == nil {
		meta, err = s.metadata(ctx, server, cfg.Auth.Issuer)
	}
	var started deviceStart
	if err == nil {
		started, err = s.startDevice(ctx, meta, cfg, server)
	}
	if err != nil {
		s.mu.Lock()
		if s.currentLocked(gen, server) {
			s.state.Busy = false
			s.state.Error = userText(err)
		}
		s.mu.Unlock()
		s.changed()
		return s.Status(), nil
	}
	flowCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	if !s.currentLocked(gen, server) {
		// the server or the account changed while the provider answered
		s.mu.Unlock()
		cancel()
		return s.Status(), nil
	}
	if s.flow != nil {
		s.flow()
	}
	s.flow = cancel
	expires := s.now().Add(time.Duration(started.ExpiresIn) * time.Second)
	models, notice, manage := cfg.shown()
	s.state = protocol.AccountState{
		State: protocol.AccountSigningIn, Server: server,
		Code:   &protocol.AccountCode{UserCode: started.UserCode, VerificationURI: started.VerificationURI, VerificationURIComplete: started.VerificationURIComplete, ExpiresAt: expires},
		Models: models, UnitsPerCredit: cfg.UnitsPerCredit, Notice: notice, Manage: manage,
	}
	st := s.snapshotLocked()
	s.mu.Unlock()
	s.changed()
	go s.poll(flowCtx, gen, server, cfg, meta, started, expires)
	return st, nil
}

type deviceStart struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type tokenAnswer struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
}

func (s *Service) postForm(ctx context.Context, endpoint string, form url.Values, out any) (int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return 0, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := s.http().Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxBody))
	if err != nil {
		return response.StatusCode, err
	}
	if json.Unmarshal(data, out) != nil {
		return response.StatusCode, failf("The identity provider answered %d with something unreadable.", response.StatusCode)
	}
	return response.StatusCode, nil
}

func (s *Service) startDevice(ctx context.Context, meta authMetadata, cfg serverConfig, server string) (deviceStart, error) {
	var started deviceStart
	var problem struct {
		Error string `json:"error"`
	}
	scopes, ok := requestedScopes(cfg.Auth.Scopes, server)
	if !ok {
		return started, failf("Set the Ryoku Accounts server first.")
	}
	form := url.Values{"client_id": {cfg.Auth.ClientID}, "scope": {scopes}}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, meta.DeviceAuthorization, strings.NewReader(form.Encode()))
	if err != nil {
		return started, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := s.http().Do(request)
	if err != nil {
		return started, err
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, maxBody))
	if err != nil {
		return started, err
	}
	if response.StatusCode != http.StatusOK {
		_ = json.Unmarshal(data, &problem)
		reason := protocol.ServerText(problem.Error, 60)
		if problem.Error == "invalid_scope" {
			return started, failf("This server is not registered for Ryoku sign-in. Use the server address from ryoku.dev.")
		}
		if problem.Error == "unauthorized_client" || problem.Error == "invalid_client" {
			return started, failf("Sign-in is blocked: Ryoku Account refused this app (%s). Check that the Ryoku Account application is public and has Device authorization grant on.", reason)
		}
		return started, failf("Sign-in is blocked: the identity provider answered %d (%s).", response.StatusCode, firstNonEmpty(reason, i18n.T("no reason")))
	}
	if json.Unmarshal(data, &started) != nil || started.DeviceCode == "" || started.UserCode == "" || started.VerificationURI == "" {
		return started, failf("The identity provider did not give a sign-in code.")
	}
	uri, ok := cleanVerification(started.VerificationURI)
	if !ok {
		return started, failf("Sign-in is blocked: the approval page is not a plain https address.")
	}
	started.VerificationURI = uri
	started.VerificationURIComplete, _ = cleanVerification(started.VerificationURIComplete)
	started.UserCode = protocol.ServerText(started.UserCode, 40)
	if started.ExpiresIn <= 0 || started.ExpiresIn > 3600 {
		started.ExpiresIn = 600
	}
	if started.Interval <= 0 {
		started.Interval = 5
	}
	started.Interval = min(started.Interval, maxInterval)
	return started, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// poll waits for the user's answer at the interval the provider asked for,
// slowing down when told to, and ends on approval, denial, expiry or Cancel.
func (s *Service) poll(ctx context.Context, gen uint64, server string, cfg serverConfig, meta authMetadata, started deviceStart, expires time.Time) {
	interval := time.Duration(started.Interval) * s.second
	backoff := interval
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if s.now().After(expires) {
			s.endFlow(ctx, i18n.T("The sign-in code expired. Sign in again for a new one."))
			return
		}
		var answer tokenAnswer
		status, err := s.postForm(ctx, meta.Token, url.Values{"client_id": {cfg.Auth.ClientID}, "grant_type": {deviceGrant}, "device_code": {started.DeviceCode}}, &answer)
		if ctx.Err() != nil {
			return
		}
		if err != nil || status != http.StatusOK && (answer.Error == "" || status >= 500) {
			// a failure that says nothing about the code retries while it is still good
			backoff = min(2*backoff, 30*s.second)
			continue
		}
		backoff = interval
		switch answer.Error {
		case "":
		case "authorization_pending":
			continue
		case "slow_down":
			interval = min(interval+5*s.second, maxInterval*s.second)
			backoff = interval
			continue
		case "access_denied":
			s.endFlow(ctx, i18n.T("Sign-in was denied in the browser."))
			return
		case "expired_token":
			s.endFlow(ctx, i18n.T("The sign-in code expired. Sign in again for a new one."))
			return
		default:
			s.endFlow(ctx, i18n.Tf("Sign-in failed (%s). Sign in again.", protocol.ServerText(answer.Error, 60)))
			return
		}
		if answer.AccessToken == "" {
			s.endFlow(ctx, i18n.T("The identity provider approved the sign-in but gave no token."))
			return
		}
		record := &gateway.AccountRecord{
			Server: server, Issuer: meta.Issuer, ClientID: cfg.Auth.ClientID, TokenEndpoint: meta.Token, RevocationEndpoint: meta.Revocation,
			Access: answer.AccessToken, Refresh: answer.RefreshToken, Expires: s.now().Add(tokenTTL(answer.ExpiresIn)),
		}
		s.finishSignIn(ctx, gen, server, record)
		return
	}
}

// finishSignIn keeps the approved account, unless the server or the account
// changed while the user was in the browser: then the tokens are dropped.
func (s *Service) finishSignIn(ctx context.Context, gen uint64, server string, record *gateway.AccountRecord) {
	s.mu.Lock()
	if ctx.Err() != nil || !s.currentLocked(gen, server) {
		s.mu.Unlock()
		return
	}
	err := s.store.SetAccountIssuer(server, issuerKey(record.Issuer))
	if err == nil {
		err = s.store.SetAccountTokens(record)
	}
	if err != nil {
		s.mu.Unlock()
		s.endFlow(ctx, i18n.T("The sign-in could not be kept on this computer."))
		return
	}
	s.flow = nil
	s.record = record
	s.unsaved = false
	s.gen++
	s.state.State = protocol.AccountSignedIn
	s.state.Code = nil
	s.state.Error = ""
	s.mu.Unlock()
	s.changed()
	_, _ = s.Refresh(context.Background())
}

func (s *Service) endFlow(ctx context.Context, text string) {
	s.mu.Lock()
	if ctx.Err() != nil {
		s.mu.Unlock()
		return
	}
	if s.flow != nil {
		s.flow()
		s.flow = nil
	}
	s.state.State = protocol.AccountSignedOut
	s.state.Code = nil
	s.state.Busy = false
	s.state.Error = text
	s.mu.Unlock()
	s.changed()
}

// Cancel stops a sign-in that waits for approval.
func (s *Service) Cancel() protocol.AccountState {
	s.mu.Lock()
	if s.flow != nil {
		s.flow()
		s.flow = nil
	}
	if s.state.State == protocol.AccountSigningIn {
		s.state.State = protocol.AccountSignedOut
		s.state.Code = nil
		s.state.Busy = false
		s.state.Error = ""
	}
	s.mu.Unlock()
	s.changed()
	return s.Status()
}

// saveLocked keeps a record that only memory has, after a store write failed.
func (s *Service) saveLocked() {
	if s.unsaved && s.record != nil && s.store.SetAccountTokens(s.record) == nil {
		s.unsaved = false
	}
}

// token returns a token valid now, refreshing it first when it is about to
// expire, and the account generation it belongs to. The refresh is not tied
// to the caller: a caller that gives up must not lose a rotated token.
func (s *Service) token(ctx context.Context, force bool) (string, string, uint64, error) {
	s.mu.Lock()
	s.saveLocked()
	record, gen := s.record, s.gen
	s.mu.Unlock()
	if record == nil {
		return "", "", 0, failf("Sign in to Ryoku to use Ryoku Ember.")
	}
	if !force && s.now().Add(refreshEarly).Before(record.Expires) {
		return record.Server, record.Access, gen, nil
	}
	type renewed struct {
		server, token string
		gen           uint64
		err           error
	}
	done := make(chan renewed, 1)
	go func() {
		var r renewed
		r.server, r.token, r.gen, r.err = s.renew(record)
		done <- r
	}()
	select {
	case r := <-done:
		return r.server, r.token, r.gen, r.err
	case <-ctx.Done():
		return "", "", 0, ctx.Err()
	}
}

// renew trades the refresh token for a new pair, once at a time, on a
// context of its own.
func (s *Service) renew(seen *gateway.AccountRecord) (string, string, uint64, error) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	s.mu.Lock()
	current, gen := s.record, s.gen
	s.mu.Unlock()
	if current == nil {
		return "", "", 0, failf("Sign in to Ryoku to use Ryoku Ember.")
	}
	if current.Access != seen.Access && s.now().Add(refreshEarly).Before(current.Expires) {
		return current.Server, current.Access, gen, nil
	}
	if current.Refresh == "" {
		s.forget(i18n.T("Your sign-in ended. Sign in again."), false)
		return "", "", 0, failf("Your sign-in ended. Sign in again.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	var answer tokenAnswer
	status, err := s.postForm(ctx, current.TokenEndpoint, url.Values{"client_id": {current.ClientID}, "grant_type": {"refresh_token"}, "refresh_token": {current.Refresh}}, &answer)
	if err != nil {
		return "", "", 0, err
	}
	if status != http.StatusOK || answer.AccessToken == "" {
		if answer.Error == "invalid_grant" || answer.Error == "invalid_client" || status == http.StatusUnauthorized || status == http.StatusBadRequest {
			s.forget(i18n.T("Your sign-in ended. Sign in again."), false)
			return "", "", 0, failf("Your sign-in ended. Sign in again.")
		}
		return "", "", 0, failf("The identity provider could not renew the sign-in now.")
	}
	next := *current
	next.Access = answer.AccessToken
	if answer.RefreshToken != "" {
		next.Refresh = answer.RefreshToken
	}
	next.Expires = s.now().Add(tokenTTL(answer.ExpiresIn))
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gen != gen || s.record == nil {
		return "", "", 0, failf("The Ryoku Account changed.")
	}
	// the new pair is the account's from here on even if the store failed to
	// keep it: the old refresh token is spent, and the next save retries
	s.record = &next
	s.unsaved = s.store.SetAccountTokens(&next) != nil
	return next.Server, next.Access, gen, nil
}

type accountAnswer struct {
	Account struct {
		ID      string  `json:"id"`
		Display *string `json:"display"`
	} `json:"account"`
	Balance struct {
		Available int64 `json:"available"`
		Held      int64 `json:"held"`
	} `json:"balance"`
	UnitsPerCredit int64 `json:"units_per_credit"`
}

// authed calls the server with the account's token, renewing it once when
// the server refuses it, and reports the generation it ran for.
func (s *Service) authed(ctx context.Context, method, path string, body, out any) (uint64, error) {
	server, token, gen, err := s.token(ctx, false)
	if err != nil {
		return 0, err
	}
	status, err := s.do(ctx, method, server+path, token, body, out)
	if status != http.StatusUnauthorized {
		return gen, err
	}
	server, token, gen, err = s.token(ctx, true)
	if err != nil {
		return 0, err
	}
	status, err = s.do(ctx, method, server+path, token, body, out)
	if status == http.StatusUnauthorized {
		s.forget(i18n.T("Your sign-in ended. Sign in again."), false)
		return 0, failf("Your sign-in ended. Sign in again.")
	}
	return gen, err
}

// Refresh reads the account, its balance and the server's description again.
// A reply for an account that was signed out meanwhile is dropped.
func (s *Service) Refresh(ctx context.Context) (protocol.AccountState, error) {
	st := s.Status()
	if st.State != protocol.AccountSignedIn {
		return st, nil
	}
	server := s.Server()
	s.setBusy(true, "")
	s.mu.Lock()
	started := s.gen
	s.mu.Unlock()
	cfg, cfgErr := s.config(ctx, server)
	var answer accountAnswer
	gen, err := s.authed(ctx, http.MethodGet, "/v1/account", nil, &answer)
	if err != nil && gen == 0 {
		gen = started
	}
	s.mu.Lock()
	if s.gen != gen || s.state.State != protocol.AccountSignedIn {
		s.state.Busy = false
		s.mu.Unlock()
		s.changed()
		return s.Status(), nil
	}
	s.state.Busy = false
	if err != nil {
		s.state.Error = userText(err)
	} else {
		s.state.Error = ""
		display := ""
		if answer.Account.Display != nil {
			display = protocol.ServerText(*answer.Account.Display, protocol.ServerTextMax)
		}
		s.state.Account = &protocol.AccountIdentity{ID: protocol.ServerText(answer.Account.ID, protocol.ServerTextMax), Display: display}
		s.state.Balance = &protocol.AccountBalance{Available: answer.Balance.Available, Held: answer.Balance.Held, Read: s.now()}
		if answer.UnitsPerCredit > 0 {
			s.state.UnitsPerCredit = answer.UnitsPerCredit
		}
	}
	if cfgErr == nil {
		s.state.Models, s.state.Notice, s.state.Manage = cfg.shown()
		if cfg.UnitsPerCredit > 0 {
			s.state.UnitsPerCredit = cfg.UnitsPerCredit
		}
	}
	s.mu.Unlock()
	s.changed()
	return s.Status(), nil
}

// SignOut forgets the account on this computer, asking the identity
// provider to revoke its refresh token first, for a few seconds at most.
func (s *Service) SignOut(context.Context) protocol.AccountState {
	s.forget("", true)
	return s.Status()
}

// forget drops the tokens and everything shown of the account, and cancels
// every hosted request still running for it, so a late answer cannot land
// after another account signed in. With revoke the refresh token is revoked
// before the record is wiped, so a revoke that did not finish leaves the
// account signed in on the next start and not valid for anyone else.
func (s *Service) forget(errText string, revoke bool) {
	s.mu.Lock()
	record := s.record
	s.record = nil
	s.unsaved = false
	s.gen++
	gen := s.gen
	if s.flow != nil {
		s.flow()
		s.flow = nil
	}
	bound := s.bound
	s.bound = nil
	server := s.Server()
	keep := protocol.AccountState{State: protocol.AccountSignedOut, Server: server, Error: errText}
	if server == "" {
		keep.State = protocol.AccountUnconfigured
	}
	s.state = keep
	s.mu.Unlock()
	for _, b := range bound {
		b.cancel()
	}
	s.changed()
	if revoke && record != nil {
		s.revoke(record)
	}
	s.mu.Lock()
	if s.gen == gen {
		_ = s.store.SetAccountTokens(nil)
	}
	s.mu.Unlock()
}

// revoke asks the identity provider to void the refresh token, within a few
// seconds and only at the provider the account signed in with.
func (s *Service) revoke(record *gateway.AccountRecord) {
	if record.RevocationEndpoint == "" || record.Refresh == "" || !sameHost(record.RevocationEndpoint, record.Issuer) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), revokeTimeout)
	defer cancel()
	var ignored map[string]any
	_, _ = s.postForm(ctx, record.RevocationEndpoint, url.Values{"client_id": {record.ClientID}, "token": {record.Refresh}, "token_type_hint": {"refresh_token"}}, &ignored)
}

// Models are the hosted models while signed in.
func (s *Service) Models() []protocol.AccountModel {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.record == nil {
		return nil
	}
	return append([]protocol.AccountModel(nil), s.state.Models...)
}

// Endpoint gives a hosted request the server's OpenAI base, a valid token
// and a context that ends when this account signs out or another signs in.
func (s *Service) Endpoint(ctx context.Context, expectedAccount string) (string, string, context.Context, error) {
	server, token, gen, err := s.token(ctx, false)
	if err != nil {
		return "", "", nil, errors.New(userText(err))
	}
	bound, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	if s.gen != gen || expectedAccount != "" && (s.state.Account == nil || s.state.Account.ID != expectedAccount) {
		s.mu.Unlock()
		cancel()
		return "", "", nil, errors.New(i18n.T("The Ryoku Account changed."))
	}
	live := s.bound[:0]
	for _, b := range s.bound {
		if b.ctx.Err() == nil {
			live = append(live, b)
		}
	}
	s.bound = append(live, boundRequest{gen: gen, ctx: bound, cancel: cancel})
	s.mu.Unlock()
	return server + "/v1", token, bound, nil
}

// Rejected renews the token after the server refused it, in the background,
// once at a time and not more often than every half minute.
func (s *Service) Rejected() {
	s.renewals.run(func() {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		_, _, _, _ = s.token(ctx, true)
	})
}

// Used reads the balance again after a hosted request ended, in the
// background, once at a time and not more often than every few seconds.
func (s *Service) Used() {
	s.reads.run(func() {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		_, _ = s.Refresh(ctx)
	})
}

var _ gateway.Hosted = (*Service)(nil)
