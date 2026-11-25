package teams

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"ryoku-rashin/internal/hostrule"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/vault"
)

const maxRuntimeHTTP = 256 << 10

// runtimeAuthority binds trusted session metadata to the current execution,
// including the run token so replacing a run cannot revive an old callback.
type runtimeAuthority struct {
	token  string
	team   protocol.Team
	member protocol.Member
	task   protocol.TeamTask
}

func runtimeRefused() error {
	return protocol.Errorf(protocol.CodeForbidden, "This runtime's current execution policy does not permit this operation.")
}

func (s *Service) runtimeAuthority(ctx context.Context, info protocol.SessionInfo, before *runtimeAuthority) (runtimeAuthority, error) {
	if err := ctx.Err(); err != nil {
		return runtimeAuthority{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.runs[info.TeamTask]
	if r == nil || r.Token == "" || info.ID == "" || r.Session != info.ID || r.Task != info.TeamTask || r.Team != info.Team || r.Member != info.Member || s.closing || s.taskEdits[r.Task] || s.editing[r.Team][r.Member] {
		return runtimeAuthority{}, runtimeRefused()
	}
	team, known := s.state.team(r.Team)
	task, exists := s.state.task(r.Task)
	member, seated := protocol.MemberByID(team.Members, r.Member)
	if !known || !exists || !seated || !taskIsOpen(task) || task.Team != r.Team || task.Member != r.Member || task.Session != r.Session || team.Paused || member.Paused || s.settings().Teams.Paused || !s.memberBudgetReady(team, member) {
		return runtimeAuthority{}, runtimeRefused()
	}
	if before != nil && (r.Token != before.token || !reflect.DeepEqual(member.Runtime, before.member.Runtime) || taskApprovals(team, member, task) != taskApprovals(before.team, before.member, before.task)) {
		return runtimeAuthority{}, runtimeRefused()
	}
	return runtimeAuthority{token: r.Token, team: team, member: member, task: task}, nil
}

func (s *Service) runtimeCredential(ref, team, use string) (protocol.Credential, error) {
	c, found := s.vault.Get(ref)
	if !found {
		c, found = s.vault.Find(ref)
	}
	if !found || c.Use != use || !slices.Contains(c.Teams, team) {
		return protocol.Credential{}, protocol.Errorf(protocol.CodeForbidden, "The configured runtime credential is unavailable for this team and use.")
	}
	return c, nil
}

// runtimeCredentialCurrent also detects rotation while a keyring unlock waited.
func (s *Service) runtimeCredentialCurrent(c protocol.Credential) bool {
	current, ok := s.vault.Get(c.ID)
	return ok && reflect.DeepEqual(current, c)
}

// RuntimeEnvironment exports only explicitly consented Runtime-use records to
// a currently authorized local runtime. It is never a socket/tool endpoint.
func (s *Service) RuntimeEnvironment(ctx context.Context, info protocol.SessionInfo) (map[string]string, error) {
	authority, err := s.runtimeAuthority(ctx, info, nil)
	if err != nil {
		return nil, err
	}
	kind := memberRuntimeKind(authority.member)
	if kind != protocol.MemberRuntimeCLI && kind != protocol.MemberRuntimeCommand {
		return nil, runtimeRefused()
	}
	result := make(map[string]string, len(authority.member.Runtime.SecretEnv))
	credentials := make([]protocol.Credential, 0, len(authority.member.Runtime.SecretEnv))
	for name, ref := range authority.member.Runtime.SecretEnv {
		if !runtimeEnvName(name) || ref == "" {
			return nil, runtimeRefused()
		}
		c, err := s.runtimeCredential(ref, authority.team.ID, protocol.CredentialRuntime)
		if err != nil {
			return nil, err
		}
		value, err := s.vault.Reveal(c.ID)
		if err != nil {
			return nil, protocol.Errorf(protocol.CodeUnavailable, "The configured runtime secret could not be opened.")
		}
		if _, err := s.runtimeAuthority(ctx, info, &authority); err != nil {
			return nil, err
		}
		if !s.runtimeCredentialCurrent(c) {
			return nil, runtimeRefused()
		}
		credentials = append(credentials, c)
		result[name] = value
	}
	if _, err := s.runtimeAuthority(ctx, info, &authority); err != nil {
		return nil, err
	}
	for _, c := range credentials {
		if !s.runtimeCredentialCurrent(c) {
			return nil, runtimeRefused()
		}
	}
	for _, c := range credentials {
		s.recordCredential(protocol.VaultEvent{Credential: c.ID, Name: c.Name, Team: authority.team.ID, Task: authority.task.ID, Outcome: protocol.VaultUsed, Detail: "Exported to the explicitly configured trusted runtime environment."})
	}
	return result, nil
}

// checkRuntimeEndpoint is shared by storage validation and the execution
// chokepoint. DNS names must resolve publicly at dial time; only an explicitly
// configured literal loopback address may reach the local computer.
func checkRuntimeEndpoint(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 8192 || raw != strings.TrimSpace(raw) || strings.ContainsAny(raw, "\\#\r\n\x00") || u == nil || u.Host == "" || strings.HasSuffix(u.Host, ":") || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.Opaque != "" || u.RawFragment != "" {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "A webhook needs an absolute URL without embedded credentials or fragments.")
	}
	if port := u.Port(); port != "" {
		p, err := strconv.Atoi(port)
		if err != nil || p < 1 || p > 65535 {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "A webhook port must be between 1 and 65535.")
		}
	}
	address, literal := netip.ParseAddr(u.Hostname())
	if literal == nil {
		if address.Zone() != "" || (!address.Unmap().IsLoopback() && !runtimePublicAddress(address)) {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "A webhook cannot reach private, metadata or reserved addresses.")
		}
	} else if !hostrule.Valid(strings.ToLower(u.Hostname())) || strings.HasPrefix(u.Hostname(), "*") {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "A webhook host must be a literal loopback address or a public DNS name.")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && literal == nil && address.Unmap().IsLoopback()) {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "Webhooks use HTTPS, or literal loopback HTTP for a local receiver.")
	}
	return u, nil
}

var runtimeReservedNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("2001::/23"), netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"), netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
}

var runtimeGlobalIPv6 = netip.MustParsePrefix("2000::/3")

func runtimePublicAddress(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.Zone() != "" {
		return false
	}
	for _, network := range runtimeReservedNetworks {
		if network.Contains(address) {
			return false
		}
	}
	return address.Is4() || runtimeGlobalIPv6.Contains(address)
}

// runtimeDial resolves once and dials the vetted IP itself: the transport
// cannot perform a second lookup that rebinds a public host to a private peer.
func runtimeDial(ctx context.Context, network, target string, loopback bool, lookup func(context.Context, string) ([]netip.Addr, error)) (net.Conn, error) {
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		return nil, errors.New("invalid webhook address")
	}
	var addresses []netip.Addr
	literalLoopback := false
	if literal, err := netip.ParseAddr(host); err == nil {
		literalLoopback = literal.Zone() == "" && literal.Unmap().IsLoopback()
		addresses = []netip.Addr{literal.Unmap()}
	} else {
		addresses, err = lookup(ctx, host)
		if err != nil || len(addresses) == 0 {
			return nil, errors.New("webhook DNS lookup failed")
		}
	}
	for _, address := range addresses {
		if !(loopback && literalLoopback && address.Unmap().IsLoopback()) && !runtimePublicAddress(address) {
			return nil, errors.New("webhook DNS resolved to a prohibited address")
		}
	}
	// One connect attempt, never a retry or a second DNS resolution.
	return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(addresses[0].Unmap().String(), port))
}

// RuntimeHTTP sends exactly the configured webhook POST after the engine's
// central action gate. No model-selected endpoint or headers cross this API.
func (s *Service) RuntimeHTTP(ctx context.Context, info protocol.SessionInfo, body []byte) (protocol.AgentHTTPResult, error) {
	authority, err := s.runtimeAuthority(ctx, info, nil)
	if err != nil {
		return protocol.AgentHTTPResult{}, err
	}
	if memberRuntimeKind(authority.member) != protocol.MemberRuntimeWebhook || taskApprovals(authority.team, authority.member, authority.task) == protocol.ApprovalsReadOnly || len(body) > maxRuntimeHTTP {
		return protocol.AgentHTTPResult{}, runtimeRefused()
	}
	u, err := checkRuntimeEndpoint(authority.member.Runtime.URL)
	if err != nil {
		return protocol.AgentHTTPResult{}, err
	}
	host := requestHost(u)
	var credential protocol.Credential
	value := ""
	if ref := authority.member.Runtime.Credential; ref != "" {
		credential, err = s.runtimeCredential(ref, authority.team.ID, protocol.CredentialHeader)
		if err != nil || !hostrule.Allows(credential.Hosts, host) {
			return protocol.AgentHTTPResult{}, runtimeRefused()
		}
		value, err = s.vault.Reveal(credential.ID)
		if err != nil {
			return protocol.AgentHTTPResult{}, protocol.Errorf(protocol.CodeUnavailable, "The configured webhook credential could not be opened.")
		}
	}
	current := func() error {
		if _, err := s.runtimeAuthority(ctx, info, &authority); err != nil {
			return err
		}
		if credential.ID != "" && !s.runtimeCredentialCurrent(credential) {
			return runtimeRefused()
		}
		return nil
	}
	values := map[string]string{value: credential.Name}
	if credential.Prefix != "" {
		values[credential.Prefix+value] = credential.Name
	}
	exact := vault.ExactValuesScrubber(values)
	scrub := func(text string) string { return s.redact(exact(text)) }
	if err := current(); err != nil {
		return protocol.AgentHTTPResult{}, err
	}
	if scrub(u.String()) != u.String() {
		return protocol.AgentHTTPResult{}, runtimeRefused()
	}
	ctx, cancel := context.WithTimeout(ctx, egressTimeout)
	defer cancel()
	literal, _ := netip.ParseAddr(u.Hostname())
	transport := &http.Transport{
		Proxy: nil, DisableCompression: true, DisableKeepAlives: true,
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout: 10 * time.Second, ResponseHeaderTimeout: egressTimeout,
		MaxResponseHeaderBytes: 32 << 10,
		DialContext: func(ctx context.Context, network, target string) (net.Conn, error) {
			if err := current(); err != nil {
				return nil, err
			}
			conn, err := runtimeDial(ctx, network, target, literal.IsValid() && literal.Unmap().IsLoopback(), func(ctx context.Context, host string) ([]netip.Addr, error) {
				return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
			})
			if err != nil {
				return nil, err
			}
			if err := current(); err != nil {
				_ = conn.Close()
				return nil, err
			}
			return conn, nil
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: egressTimeout, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("webhook redirects are refused") }}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return protocol.AgentHTTPResult{}, runtimeRefused()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Encoding", "identity")
	if credential.ID != "" {
		req.Header.Set(credential.Header, credential.Prefix+value)
	}
	resp, err := client.Do(req)
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		if err := ctx.Err(); err != nil {
			return protocol.AgentHTTPResult{}, err
		}
		// Transport errors can embed URLs or peer-controlled text. The caller
		// only needs the outcome, never an upstream diagnostic carrying a key.
		return protocol.AgentHTTPResult{}, protocol.Errorf(protocol.CodeUnavailable, "The configured webhook request failed or redirected.")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode < 400 || resp.Header.Get("Content-Encoding") != "" && !strings.EqualFold(resp.Header.Get("Content-Encoding"), "identity") {
		return protocol.AgentHTTPResult{}, protocol.Errorf(protocol.CodeUnavailable, "The webhook response was redirected or encoded and cannot be shown safely.")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxRuntimeHTTP+1))
	if err != nil || len(raw) > maxRuntimeHTTP {
		if err := ctx.Err(); err != nil {
			return protocol.AgentHTTPResult{}, err
		}
		return protocol.AgentHTTPResult{}, protocol.Errorf(protocol.CodeUnavailable, "The webhook response could not be read within the 256 KiB limit.")
	}
	if err := current(); err != nil {
		return protocol.AgentHTTPResult{}, err
	}
	text, truncated, err := runtimeScrubResponse(raw, scrub)
	if err != nil {
		return protocol.AgentHTTPResult{}, err
	}
	result := protocol.AgentHTTPResult{Status: resp.StatusCode, Body: text, Truncated: truncated, Headers: map[string]string{}}
	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		result.Headers["content-type"] = scrub(contentType)
	}
	if credential.ID != "" {
		s.recordCredential(protocol.VaultEvent{Credential: credential.ID, Name: credential.Name, Team: authority.team.ID, Task: authority.task.ID, Host: host, Outcome: protocol.VaultUsed, Detail: "Configured runtime webhook POST completed."})
	}
	s.record(protocol.Activity{Actor: authority.member.Name, Action: "runtime.webhook", Target: authority.member.ID, OK: resp.StatusCode < 400, Team: authority.team.ID, Task: authority.task.ID, Initiator: protocol.InitiatorAgent, Detail: "Configured runtime webhook POST completed."})
	return result, nil
}

// runtimeScrubResponse redacts decoded report strings, leaving the control
// envelope intact even when a credential collides with an ID or status. It
// preserves keys, duplicate fields and number tokens: normalization could turn
// an invalid report into an accepted completion. Validate the entire input
// first, even if redaction expansion will exceed the bounded output.
func runtimeScrubResponse(raw []byte, scrub func(string) string) (string, bool, error) {
	if !json.Valid(raw) {
		return "", false, protocol.Errorf(protocol.CodeUnavailable, "The webhook response is not one complete JSON value.")
	}
	clean := make([]byte, 0, len(raw))
	appendBounded := func(part []byte) bool {
		remaining := maxRuntimeHTTP - len(clean)
		if len(part) > remaining {
			clean = append(clean, part[:remaining]...)
			return false
		}
		clean = append(clean, part...)
		return true
	}
	copied := 0
	var containers []struct{ output, reportField bool }
	outputValue := func() bool {
		if len(containers) == 0 {
			return false
		}
		parent := containers[len(containers)-1]
		return parent.output || len(containers) == 1 && parent.reportField
	}
	for start := 0; start < len(raw); start++ {
		switch raw[start] {
		case '{', '[':
			containers = append(containers, struct{ output, reportField bool }{output: outputValue()})
			continue
		case '}', ']':
			containers = containers[:len(containers)-1]
			continue
		case ',':
			if len(containers) > 0 {
				containers[len(containers)-1].reportField = false
			}
			continue
		case '"':
		default:
			continue
		}
		end := start + 1
		for raw[end] != '"' {
			if raw[end] == '\\' {
				end++
			}
			end++
		}
		end++
		after := bytes.TrimLeft(raw[end:], " \t\r\n")
		isKey := len(after) > 0 && after[0] == ':'
		if isKey && len(containers) == 1 || !isKey && outputValue() {
			var value string
			if err := json.Unmarshal(raw[start:end], &value); err != nil {
				return "", false, protocol.Errorf(protocol.CodeUnavailable, "The webhook response contains an invalid JSON string.")
			}
			if isKey {
				// encoding/json matches tagged field names case-insensitively.
				containers[0].reportField = strings.EqualFold(value, "report")
				start = end - 1
				continue
			}
			if redacted := scrub(value); redacted != value {
				encoded, _ := json.Marshal(redacted)
				if !appendBounded(raw[copied:start]) || !appendBounded(encoded) {
					return string(clean), true, nil
				}
				copied = end
			}
		}
		start = end - 1
	}
	complete := appendBounded(raw[copied:])
	return string(clean), !complete, nil
}
