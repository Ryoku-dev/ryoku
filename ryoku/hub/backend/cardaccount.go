package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

const cardAccountGrant = "urn:ietf:params:oauth:grant-type:device_code"

var cardAccountHTTP = &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func cardAccountOrigin(raw string) (string, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Hostname() == "" || u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", errors.New("Configure an exact Account server origin")
	}
	host := strings.ToLower(u.Hostname())
	scheme := strings.ToLower(u.Scheme)
	loop := host == "localhost"
	if ip := net.ParseIP(host); ip != nil {
		loop = ip.IsLoopback()
	}
	if scheme != "https" && !(scheme == "http" && loop) {
		return "", errors.New("Account server requires HTTPS (HTTP only on loopback)")
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if rawPort := u.Port(); rawPort != "" {
		port, err := strconv.Atoi(rawPort)
		if err != nil || port < 0 || port > 65535 {
			return "", errors.New("Invalid Account server port")
		}
		if !(scheme == "https" && port == 443) && !(scheme == "http" && port == 80) {
			host += ":" + strconv.Itoa(port)
		}
	}
	return scheme + "://" + host, nil
}
func cardAccountServer() (string, error) { return cardAccountOrigin(os.Getenv("RYOKU_ACCOUNTS_URL")) }
func cardAccountScope(server string) string {
	sum := sha256.Sum256([]byte(server))
	return "ryoku-server-" + hex.EncodeToString(sum[:16])
}
func cardAccountJSON(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if e := enc.Encode(v); e != nil {
		return nil, e
	}
	return bytes.TrimSuffix(b.Bytes(), []byte{'\n'}), nil
}
func cardAccountDir() (string, error) {
	dir, e := badgeDataDir()
	if e != nil {
		return "", e
	}
	dir = filepath.Join(dir, "account")
	if e = os.MkdirAll(dir, 0700); e != nil {
		return "", e
	}
	st, e := os.Lstat(dir)
	if e != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("Unsafe Account state directory")
	}
	return dir, nil
}
func cardAccountWrite(name string, v any) error {
	data, e := cardAccountJSON(v)
	if e != nil {
		return e
	}
	return cardAccountWriteBytes(name, data)
}
func cardAccountWriteBytes(name string, data []byte) error {
	dir, e := cardAccountDir()
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(dir, ".write-")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(0600); e == nil {
		_, e = f.Write(data)
	}
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(f.Name(), filepath.Join(dir, name))
}
func cardAccountRead(name string, out any) error {
	dir, e := cardAccountDir()
	if e != nil {
		return e
	}
	f, e := os.OpenFile(filepath.Join(dir, name), os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if e != nil {
		return e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil || !st.Mode().IsRegular() || st.Mode().Perm()&0077 != 0 {
		return errors.New("Unsafe Account state file")
	}
	raw, e := readBounded(f, 64<<10)
	if e != nil {
		return e
	}
	return json.Unmarshal(raw, out)
}
func cardAccountRemove(name string) error {
	dir, e := cardAccountDir()
	if e != nil {
		return e
	}
	e = os.Remove(filepath.Join(dir, name))
	if os.IsNotExist(e) {
		return nil
	}
	return e
}

// another hub process must not spend the same rotating refresh token
func cardAccountLock(ctx context.Context) (func(), error) {
	return cardAccountNamedLock(ctx, "account.lock")
}
func cardAccountNamedLock(ctx context.Context, name string) (func(), error) {
	dir, e := cardAccountDir()
	if e != nil {
		return nil, e
	}
	f, e := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return nil, e
	}
	for {
		e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if e == nil {
			return func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }, nil
		}
		if e != syscall.EWOULDBLOCK {
			f.Close()
			return nil, e
		}
		select {
		case <-ctx.Done():
			f.Close()
			return nil, ctx.Err()
		case <-time.After(25 * time.Millisecond):
		}
	}
}

type cardAccountBuffer struct{ bytes.Buffer }

func (b *cardAccountBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 64<<10 {
		return 0, errors.New("Account credential exceeds limit")
	}
	return b.Buffer.Write(p)
}

// secret attributes identify the server, never the credential
func cardAccountSecret(ctx context.Context, server, action, slot string, value any, out any) error {
	bus, e := dbus.ConnectSessionBus()
	if e != nil {
		return errors.New("Secret Service is unavailable; start and unlock a keyring")
	}
	defer bus.Close()
	call := bus.Object("org.freedesktop.secrets", "/org/freedesktop/secrets").CallWithContext(ctx, "org.freedesktop.DBus.Peer.Ping", 0)
	if call.Err != nil {
		return errors.New("Secret Service is unavailable; start and unlock a keyring")
	}
	args := []string{action}
	if action == "store" {
		args = append(args, "--label=Ryoku Account")
	}
	args = append(args, "service", "ryoku-card-account", "server", server, "slot", slot)
	cmd := exec.CommandContext(ctx, "secret-tool", args...)
	cmd.WaitDelay = time.Second
	var input []byte
	if action == "store" {
		input, e = cardAccountJSON(value)
		if e != nil {
			return e
		}
		defer clear(input)
		cmd.Stdin = bytes.NewReader(input)
	}
	var output cardAccountBuffer
	defer func() { clear(output.Bytes()) }()
	cmd.Stdout = &output
	if e = cmd.Run(); e != nil {
		if (action == "lookup" || action == "clear") && output.Len() == 0 {
			if exit, ok := e.(*exec.ExitError); ok && exit.ExitCode() == 1 {
				if action == "clear" {
					return nil
				}
				return os.ErrNotExist
			}
		}
		return errors.New("Account keyring operation failed; unlock the keyring")
	}
	if action == "lookup" {
		if output.Len() == 0 {
			return os.ErrNotExist
		}
		if json.Unmarshal(output.Bytes(), out) != nil {
			return errors.New("Account keyring record is invalid")
		}
	}
	return nil
}
func cardAccountRequest(ctx context.Context, method, endpoint, token, contentType string, body []byte, out any) (int, error) {
	req, e := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if e != nil {
		return 0, errors.New("Invalid Account request")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")
	res, e := cardAccountHTTP.Do(req)
	if e != nil {
		return 0, errors.New("The Account service could not be reached")
	}
	defer res.Body.Close()
	raw, e := readBounded(res.Body, 256<<10)
	if e != nil {
		return res.StatusCode, errors.New("The Account response exceeds its limit")
	}
	if out != nil && len(raw) > 0 {
		if json.Unmarshal(raw, out) != nil {
			return res.StatusCode, errors.New("The Account service returned unreadable data")
		}
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return res.StatusCode, fmt.Errorf("Account request refused (HTTP %d)", res.StatusCode)
	}
	return res.StatusCode, nil
}
func cardAccountAPI(ctx context.Context, server, token, method, path string, body, out any) error {
	var raw []byte
	var e error
	if body != nil {
		raw, e = cardAccountJSON(body)
		if e != nil {
			return e
		}
	}
	_, e = cardAccountRequest(ctx, method, server+path, token, "application/json", raw, out)
	return e
}

type cardAccountConfig struct {
	Auth struct {
		Issuer   string `json:"issuer"`
		ClientID string `json:"client_id"`
		Scopes   string `json:"scopes"`
	} `json:"auth"`
	AccountURL string `json:"account_url"`
}
type cardAccountMetadata struct {
	Issuer     string   `json:"issuer"`
	Device     string   `json:"device_authorization_endpoint"`
	Token      string   `json:"token_endpoint"`
	Revocation string   `json:"revocation_endpoint"`
	Grants     []string `json:"grant_types_supported"`
}
type cardAccountRecord struct {
	Server     string
	Issuer     string
	ClientID   string
	Endpoint   string
	Revocation string
	Access     string
	Refresh    string
	Expires    int64
	AccountID  string
}
type cardAccountPending struct {
	Server       string
	Issuer       string
	ClientID     string
	Endpoint     string
	Revocation   string
	Device       string
	UserCode     string
	Verification string
	Expires      int64
	Next         int64
	Interval     int64
}
type cardAccountToken struct {
	Access  string `json:"access_token"`
	Refresh string `json:"refresh_token"`
	Expires int    `json:"expires_in"`
	Error   string `json:"error"`
}
type cardAccountIdentity struct {
	Account struct {
		ID string `json:"id"`
	} `json:"account"`
}

func cardAccountEndpoint(raw, issuer string) bool {
	a, e := url.Parse(raw)
	b, f := url.Parse(issuer)
	return e == nil && f == nil && a.Scheme == "https" && a.Host == b.Host && a.User == nil && a.Fragment == "" && a.Opaque == ""
}
func cardAccountDiscover(ctx context.Context, server string) (cardAccountConfig, cardAccountMetadata, error) {
	var cfg cardAccountConfig
	var meta cardAccountMetadata
	if e := cardAccountAPI(ctx, server, "", "GET", "/v1/config", nil, &cfg); e != nil {
		return cfg, meta, e
	}
	issuer := strings.TrimRight(cfg.Auth.Issuer, "/")
	u, e := url.Parse(issuer)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || cfg.Auth.ClientID == "" {
		return cfg, meta, errors.New("Account sign-in metadata is invalid")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	ip := net.ParseIP(host)
	s, _ := url.Parse(server)
	sip := net.ParseIP(s.Hostname())
	localServer := s.Hostname() == "localhost" || (sip != nil && sip.IsLoopback())
	if !localServer && (host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") || (ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified()))) {
		return cfg, meta, errors.New("Account issuer cannot be a private-network identity provider")
	}
	var pinned struct{ Issuer string }
	if e = cardAccountRead("issuer-"+cardAccountScope(server)+".json", &pinned); e == nil && pinned.Issuer != issuer {
		return cfg, meta, errors.New("Account identity provider changed; refusing sign-in")
	} else if e != nil && !os.IsNotExist(e) {
		return cfg, meta, e
	}
	if _, e = cardAccountRequest(ctx, "GET", issuer+"/.well-known/oauth-authorization-server", "", "", nil, &meta); e != nil {
		return cfg, meta, e
	}
	grant := false
	for _, g := range meta.Grants {
		grant = grant || g == cardAccountGrant
	}
	if strings.TrimRight(meta.Issuer, "/") != issuer || !grant || !cardAccountEndpoint(meta.Device, issuer) || !cardAccountEndpoint(meta.Token, issuer) || (meta.Revocation != "" && !cardAccountEndpoint(meta.Revocation, issuer)) {
		return cfg, meta, errors.New("Account issuer endpoints or device grant do not match")
	}
	if e = cardAccountWrite("issuer-"+cardAccountScope(server)+".json", map[string]string{"Issuer": issuer}); e != nil {
		return cfg, meta, e
	}
	return cfg, meta, nil
}
func cardAccountForm(ctx context.Context, endpoint string, form url.Values, out any) (int, error) {
	raw := []byte(form.Encode())
	defer clear(raw)
	return cardAccountRequest(ctx, "POST", endpoint, "", "application/x-www-form-urlencoded", raw, out)
}
func cardAccountTTL(seconds int) int64 {
	if seconds <= 0 || seconds > 86400 {
		return 0
	}
	return time.Now().Add(time.Duration(seconds) * time.Second).Unix()
}

// ask the configured service, not a cached entitlement or a browser token
func cardAccountAccess(ctx context.Context) (string, string, error) {
	server, e := cardAccountServer()
	if e != nil {
		return "", "", e
	}
	unlock, e := cardAccountLock(ctx)
	if e != nil {
		return "", "", e
	}
	defer unlock()
	var r cardAccountRecord
	if e = cardAccountSecret(ctx, server, "lookup", "tokens", nil, &r); e != nil {
		return "", "", errors.New("Sign in to Ryoku Account (an unlocked Secret Service keyring is required)")
	}
	if r.Server != server || !cardAccountEndpoint(r.Endpoint, r.Issuer) || r.Access == "" || !cardAccountIDPattern.MatchString(r.AccountID) {
		return "", "", errors.New("Account credential binding is invalid")
	}
	renew := func() error {
		if r.Refresh == "" {
			return errors.New("Account sign-in expired; sign in again")
		}
		renewCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 40*time.Second)
		defer stop()
		var answer cardAccountToken
		status, err := cardAccountForm(renewCtx, r.Endpoint, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {r.Refresh}, "client_id": {r.ClientID}}, &answer)
		if err != nil || status != 200 || answer.Access == "" || cardAccountTTL(answer.Expires) == 0 {
			if answer.Error == "invalid_grant" || answer.Error == "invalid_client" {
				_ = cardAccountSecret(renewCtx, server, "clear", "tokens", nil, nil)
			}
			return errors.New("Account sign-in could not be renewed; sign in again")
		}
		r.Access = answer.Access
		if answer.Refresh != "" {
			r.Refresh = answer.Refresh
		}
		r.Expires = cardAccountTTL(answer.Expires)
		// keep a rotated pair even if the caller closed the window
		if err = cardAccountSecret(renewCtx, server, "store", "tokens", r, nil); err != nil {
			return errors.New("The renewed sign-in could not be saved in Secret Service; unlock it and sign in again")
		}
		return nil
	}
	if r.Expires <= time.Now().Add(30*time.Second).Unix() {
		if e = renew(); e != nil {
			return "", "", e
		}
	}
	var identity cardAccountIdentity
	status, e := cardAccountRequest(ctx, "GET", server+"/v1/account", r.Access, "", nil, &identity)
	if status == 401 {
		if e = renew(); e != nil {
			return "", "", e
		}
		_, e = cardAccountRequest(ctx, "GET", server+"/v1/account", r.Access, "", nil, &identity)
	}
	if e != nil {
		return "", "", e
	}
	if identity.Account.ID == "" || (r.AccountID != "" && identity.Account.ID != r.AccountID) {
		return "", "", errors.New("Account identity changed; sign in again")
	}
	return server, r.Access, nil
}
func cardAccountOpen(ctx context.Context, raw string) error {
	u, e := url.Parse(raw)
	if e != nil || u.User != nil || u.Host == "" || u.Opaque != "" {
		return errors.New("Invalid browser destination")
	}
	if u.Scheme != "https" {
		origin, e := cardAccountOrigin(u.Scheme + "://" + u.Host)
		if e != nil || !strings.HasPrefix(origin, "http://") {
			return errors.New("Browser destination requires HTTPS")
		}
	}
	cmd := exec.CommandContext(ctx, "xdg-open", u.String())
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.WaitDelay = time.Second
	if e = cmd.Run(); e != nil {
		return errors.New("The system browser could not be opened")
	}
	return nil
}
