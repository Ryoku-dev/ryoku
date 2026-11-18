package mcpservers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/vault"
)

type record struct {
	Info    protocol.MCPServerInfo `json:"info"`
	Headers map[string]string      `json:"headers,omitempty"`
	Token   string                 `json:"token,omitempty"`
}

type Store struct {
	mu                   sync.Mutex
	path                 string
	vault                *vault.Vault
	records              []record
	pending              map[string]bool
	cancelSignIn         map[string]context.CancelFunc
	proxy                *http.Server
	proxyClient          *http.Client
	proxyURL, proxyToken string
	proxyCancel          context.CancelFunc
	proxyEndpoints       map[string]proxyEndpoint
	stopped              bool
}

func Open(path string, v *vault.Vault) (*Store, error) {
	s := &Store{path: path, vault: v, pending: make(map[string]bool), cancelSignIn: make(map[string]context.CancelFunc)}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &s.records); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) persist() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	raw, err := json.Marshal(s.records)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".mcp-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), s.path)
}

func (s *Store) List() []protocol.MCPServerInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]protocol.MCPServerInfo, 0, len(s.records))
	for _, r := range s.records {
		info := r.Info
		info.Args = append([]string(nil), info.Args...)
		info.HeaderNames = append([]string(nil), info.HeaderNames...)
		if info.OAuth != nil {
			oauth := *info.OAuth
			info.OAuth = &oauth
		}
		out = append(out, info)
	}
	return out
}

var headerRule = regexp.MustCompile("^[A-Za-z0-9!#$%&'*+.^_`|~-]+$")

func validURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Host != "" && u.User == nil && u.Fragment == "" && (u.Scheme == "https" || (u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")))
}

func (s *Store) Save(p protocol.MCPServer) (protocol.MCPServerInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(p.Name) == "" || len(p.Name) > 100 {
		return protocol.MCPServerInfo{}, errors.New(i18n.T("Give the server a short name."))
	}
	if p.Type != "stdio" && p.Type != "http" && p.Type != "sse" {
		return protocol.MCPServerInfo{}, errors.New(i18n.T("Choose stdio, http or sse."))
	}
	if p.Type == "stdio" && strings.TrimSpace(p.Command) == "" {
		return protocol.MCPServerInfo{}, errors.New(i18n.T("Give the server a command."))
	}
	if p.Type != "stdio" && !validURL(p.URL) {
		return protocol.MCPServerInfo{}, errors.New(i18n.T("Use an HTTPS address, or HTTP on this computer."))
	}
	if p.OAuth != nil && (p.Type == "stdio" || p.OAuth.ClientID == "" || !validURL(p.OAuth.AuthURL) || !validURL(p.OAuth.TokenURL)) {
		return protocol.MCPServerInfo{}, errors.New(i18n.T("Check the sign-in client and HTTPS addresses."))
	}
	for name, value := range p.Headers {
		if !headerRule.MatchString(name) || len(value) > 12000 || strings.ContainsAny(value, "\r\n") || strings.EqualFold(name, "Host") || strings.EqualFold(name, "Content-Length") || strings.EqualFold(name, "Connection") || strings.EqualFold(name, "Transfer-Encoding") {
			return protocol.MCPServerInfo{}, errors.New(i18n.T("That header cannot be used."))
		}
	}
	index := -1
	for i := range s.records {
		if s.records[i].Info.ID == p.ID {
			index = i
		}
	}
	if p.ID != "" && index < 0 {
		return protocol.MCPServerInfo{}, errors.New(i18n.T("That server no longer exists."))
	}
	if p.ID == "" {
		p.ID = randomID()
	}
	r := record{Headers: make(map[string]string)}
	if index >= 0 {
		r = s.records[index]
	}
	old := r
	r.Headers = maps.Clone(old.Headers)
	if r.Headers == nil {
		r.Headers = make(map[string]string)
	}
	var created []string
	committed := false
	defer func() {
		if !committed && s.vault != nil {
			for _, id := range created {
				_ = s.vault.Remove(id)
			}
		}
	}()
	if p.URL != r.Info.URL || p.Type != r.Info.Type || !sameOAuth(p.OAuth, r.Info.OAuth) {
		r.Token = ""
		if p.URL != r.Info.URL {
			r.Headers = make(map[string]string)
		}
	}
	for name, value := range p.Headers {
		name = http.CanonicalHeaderKey(name)
		if value == "" {
			delete(r.Headers, name)
			continue
		}
		id, err := s.secret("", p.ID, value)
		if err != nil {
			return protocol.MCPServerInfo{}, err
		}
		created = append(created, id)
		r.Headers[name] = id
	}
	names := make([]string, 0, len(r.Headers))
	for name := range r.Headers {
		names = append(names, name)
	}
	sort.Strings(names)
	state := "failing"
	if !p.Enabled {
		state = "off"
	} else if p.OAuth != nil && r.Token == "" {
		state = "needs-signin"
	}
	r.Info = protocol.MCPServerInfo{ID: p.ID, Name: strings.TrimSpace(p.Name), Type: p.Type, Command: p.Command, Args: p.Args, URL: p.URL, HeaderNames: names, OAuth: p.OAuth, Enabled: p.Enabled, State: state}
	if index < 0 {
		s.records = append(s.records, r)
	} else {
		s.records[index] = r
	}
	if err := s.persist(); err != nil {
		if index < 0 {
			s.records = s.records[:len(s.records)-1]
		} else {
			s.records[index] = old
		}
		return protocol.MCPServerInfo{}, err
	}
	committed = true
	if stop := s.cancelSignIn[p.ID]; stop != nil && (!p.Enabled || !sameOAuth(old.Info.OAuth, p.OAuth) || old.Info.URL != p.URL) {
		s.pending[p.ID] = false
		stop()
	}
	if s.vault != nil {
		for name, id := range old.Headers {
			if r.Headers[name] != id {
				_ = s.vault.Remove(id)
			}
		}
		if old.Token != "" && old.Token != r.Token {
			_ = s.vault.Remove(old.Token)
		}
	}
	return r.Info, nil
}

func sameOAuth(a, b *protocol.MCPOAuth) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func randomID() string { var b [16]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }

func (s *Store) secret(id, server, value string) (string, error) {
	if s.vault == nil {
		return "", errors.New(i18n.T("The credential vault is unavailable."))
	}
	if id != "" {
		_, err := s.vault.Update(protocol.VaultUpdateParams{ID: id, Value: value})
		return id, err
	}
	c, err := s.vault.Add(protocol.VaultAddParams{Name: "MCP_" + strings.ToUpper(randomID()), Description: "MCP " + server, Use: protocol.CredentialHeader, Hosts: []string{"localhost"}, Value: value})
	return c.ID, err
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, r := range s.records {
		if r.Info.ID != id {
			continue
		}
		before := append([]record(nil), s.records...)
		s.records = append(s.records[:i], s.records[i+1:]...)
		if err := s.persist(); err != nil {
			s.records = before
			return err
		}
		if stop := s.cancelSignIn[id]; stop != nil {
			s.pending[id] = false
			stop()
		}
		if s.vault != nil {
			for _, credential := range r.Headers {
				_ = s.vault.Remove(credential)
			}
			if r.Token != "" {
				_ = s.vault.Remove(r.Token)
			}
		}
		return nil
	}
	return errors.New(i18n.T("That server no longer exists."))
}

func (s *Store) headers(ctx context.Context, r *record) (map[string]string, error) {
	headers := make(map[string]string, len(r.Headers)+1)
	if (len(r.Headers) > 0 || r.Token != "") && s.vault == nil {
		return nil, errors.New(i18n.T("Unlock the credential vault first."))
	}
	for name, id := range r.Headers {
		value, err := s.vault.Reveal(id)
		if err != nil {
			return nil, err
		}
		headers[name] = value
	}
	if r.Info.OAuth != nil {
		if r.Token == "" {
			return nil, errors.New(i18n.T("Sign in to this server first."))
		}
		token, err := s.token(ctx, r)
		if err != nil {
			return nil, err
		}
		headers["Authorization"] = "Bearer " + token.AccessToken
	}
	return headers, nil
}

func (s *Store) Check(ctx context.Context, id string) (protocol.MCPServerInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.records {
		r := &s.records[i]
		if r.Info.ID != id {
			continue
		}
		r.Info.Checked, r.Info.Tools, r.Info.Detail = time.Now(), 0, ""
		if !r.Info.Enabled {
			r.Info.State = "off"
			return r.Info, nil
		}
		headers, err := s.headers(ctx, r)
		if err != nil {
			r.Info.State, r.Info.Detail = "needs-signin", i18n.T("Sign in or unlock the credential vault.")
		} else {
			r.Info.Tools, err = probe(ctx, r.Info, headers)
			if err != nil {
				r.Info.State, r.Info.Detail = "failing", i18n.T("The server did not complete its tool check.")
			} else {
				r.Info.State = "ready"
			}
			if errors.Is(err, errSignIn) {
				r.Info.State, r.Info.Detail = "needs-signin", i18n.T("Sign in or update the saved headers.")
			}
		}
		if err := s.persist(); err != nil {
			return protocol.MCPServerInfo{}, err
		}
		return r.Info, nil
	}
	return protocol.MCPServerInfo{}, errors.New(i18n.T("That server no longer exists."))
}

func (s *Store) Ready(ctx context.Context) []core.MCPServer {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return nil
	}
	var out []core.MCPServer
	for i := range s.records {
		r := &s.records[i]
		if !r.Info.Enabled || r.Info.State != "ready" {
			continue
		}
		server := core.MCPServer{Name: r.Info.Name, Type: r.Info.Type, Command: r.Info.Command, Args: r.Info.Args, URL: r.Info.URL}
		if (r.Info.Type == "http" || r.Info.Type == "sse") && (len(r.Headers) > 0 || r.Info.OAuth != nil) {
			if err := s.startProxyLocked(); err != nil {
				continue
			}
			server.URL = s.proxyAddressLocked(r.Info.ID)
		}
		out = append(out, server)
	}
	return out
}

func (s *Store) CancelSignIns() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, cancel := range s.cancelSignIn {
		s.pending[id] = false
		cancel()
	}
	s.stopped = true
	s.stopProxyLocked()
}

func (s *Store) Resume() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = false
}
