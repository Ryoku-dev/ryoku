package mcpservers

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

type oauthToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	ExpiresIn    int64     `json:"expires_in,omitempty"`
	Expires      time.Time `json:"expires,omitempty"`
}

var oauthClient = &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func exchange(ctx context.Context, endpoint string, form url.Values) (oauthToken, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return oauthToken{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := oauthClient.Do(req)
	if err != nil {
		return oauthToken{}, errors.New(i18n.T("The sign-in service could not be reached."))
	}
	defer resp.Body.Close()
	var token oauthToken
	if resp.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&token) != nil || token.AccessToken == "" || (token.TokenType != "" && !strings.EqualFold(token.TokenType, "Bearer")) {
		return oauthToken{}, errors.New(i18n.T("The sign-in service did not issue a token."))
	}
	if token.ExpiresIn > 0 {
		token.Expires = time.Now().Add(time.Duration(min(token.ExpiresIn, 31536000)) * time.Second)
	}
	return token, nil
}

func (s *Store) token(ctx context.Context, r *record) (oauthToken, error) {
	raw, err := s.vault.Reveal(r.Token)
	if err != nil {
		return oauthToken{}, err
	}
	var token oauthToken
	if json.Unmarshal([]byte(raw), &token) != nil {
		return token, errors.New(i18n.T("Sign in to this server again."))
	}
	if token.Expires.IsZero() || time.Now().Add(30*time.Second).Before(token.Expires) {
		return token, nil
	}
	if token.RefreshToken == "" {
		return token, errors.New(i18n.T("Sign in to this server again."))
	}
	fresh, err := exchange(ctx, r.Info.OAuth.TokenURL, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {token.RefreshToken}, "client_id": {r.Info.OAuth.ClientID}})
	if err != nil {
		return fresh, err
	}
	if fresh.RefreshToken == "" {
		fresh.RefreshToken = token.RefreshToken
	}
	encoded, err := json.Marshal(fresh)
	if err != nil {
		return fresh, err
	}
	_, err = s.secret(r.Token, r.Info.ID, string(encoded))
	return fresh, err
}

func (s *Store) SignIn(ctx context.Context, id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var selected *record
	for i := range s.records {
		if s.records[i].Info.ID == id {
			selected = &s.records[i]
			break
		}
	}
	if selected == nil || selected.Info.OAuth == nil || !selected.Info.Enabled {
		return "", errors.New(i18n.T("Enable this server and add its sign-in details first."))
	}
	if s.vault == nil {
		return "", errors.New(i18n.T("The credential vault is unavailable."))
	}
	if s.cancelSignIn[id] != nil {
		return "", errors.New(i18n.T("A sign-in for this server is already open."))
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	redirect := "http://" + listener.Addr().String() + "/callback"
	verifier := randomID() + randomID()
	state := randomID()
	digest := sha256.Sum256([]byte(verifier))
	oauth := *selected.Info.OAuth
	auth, _ := url.Parse(oauth.AuthURL)
	query := auth.Query()
	query.Set("response_type", "code")
	query.Set("client_id", oauth.ClientID)
	query.Set("redirect_uri", redirect)
	query.Set("scope", oauth.Scopes)
	query.Set("state", state)
	query.Set("code_challenge_method", "S256")
	query.Set("code_challenge", base64.RawURLEncoding.EncodeToString(digest[:]))
	auth.RawQuery = query.Encode()
	s.pending[id] = true
	background, cancel := context.WithCancel(context.Background())
	s.cancelSignIn[id] = cancel
	go s.awaitSignIn(background, listener, id, oauth, state, verifier, redirect)
	return auth.String(), nil
}

func (s *Store) awaitSignIn(parent context.Context, listener net.Listener, id string, oauth protocol.MCPOAuth, state, verifier, redirect string) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Minute)
	defer cancel()
	done := make(chan struct{}, 1)
	mux := http.NewServeMux()
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 5 * time.Second}
	mux.HandleFunc("/callback", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		if req.Method != http.MethodGet || req.Host != listener.Addr().String() || req.Header.Get("Origin") != "" || subtle.ConstantTimeCompare([]byte(req.URL.Query().Get("state")), []byte(state)) != 1 {
			http.Error(w, i18n.T("This sign-in link is not valid."), http.StatusBadRequest)
			return
		}
		s.mu.Lock()
		if !s.pending[id] {
			s.mu.Unlock()
			http.Error(w, i18n.T("This sign-in is already complete."), http.StatusConflict)
			return
		}
		s.pending[id] = false
		s.mu.Unlock()
		defer func() {
			select {
			case done <- struct{}{}:
			default:
			}
		}()
		code := req.URL.Query().Get("code")
		if code == "" || len(code) > 8192 {
			http.Error(w, i18n.T("Sign-in was not completed."), http.StatusBadRequest)
			return
		}
		token, err := exchange(ctx, oauth.TokenURL, url.Values{"grant_type": {"authorization_code"}, "code": {code}, "client_id": {oauth.ClientID}, "redirect_uri": {redirect}, "code_verifier": {verifier}})
		if err != nil {
			http.Error(w, i18n.T("Sign-in failed. Return to Rashin and try again."), http.StatusBadGateway)
			return
		}
		raw, err := json.Marshal(token)
		if err != nil {
			http.Error(w, i18n.T("Could not keep this sign-in."), http.StatusInternalServerError)
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if ctx.Err() != nil {
			http.Error(w, i18n.T("Sign-in was cancelled."), http.StatusGone)
			return
		}
		for i := range s.records {
			r := &s.records[i]
			if r.Info.ID != id || !r.Info.Enabled || !sameOAuth(r.Info.OAuth, &oauth) {
				continue
			}
			r.Token, err = s.secret(r.Token, id, string(raw))
			if err == nil {
				r.Info.State, r.Info.Detail = "failing", i18n.T("Signed in. Check the server to read its tools.")
				err = s.persist()
			}
			if err != nil {
				http.Error(w, i18n.T("Could not keep this sign-in. Unlock the vault and try again."), http.StatusInternalServerError)
				return
			}
			_, _ = io.WriteString(w, i18n.T("Signed in. Return to Rashin and check the server."))
			return
		}
		http.Error(w, i18n.T("The server changed while signing in."), http.StatusConflict)
	})
	go func() { _ = server.Serve(listener) }()
	select {
	case <-ctx.Done():
		_ = server.Close()
	case <-done:
		finish, stop := context.WithTimeout(context.Background(), 2*time.Second)
		_ = server.Shutdown(finish)
		stop()
	}
	s.mu.Lock()
	if stop := s.cancelSignIn[id]; stop != nil {
		stop()
	}
	delete(s.pending, id)
	delete(s.cancelSignIn, id)
	s.mu.Unlock()
}
