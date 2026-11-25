package teams

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/vault"
)

const runtimeFixtureSecret = "an unrecognizable value & punctuation /+"

// The session harness starts a native fake session; configure its live seat
// directly so these tests exercise the controller, not an external runtime.
func runtimeFixture(t *testing.T, runtime protocol.MemberRuntime) (*harness, protocol.SessionInfo) {
	t.Helper()
	h := newHarness(t, 0)
	team := h.newTeam(t, "Runtime team", func(p *protocol.TeamCreateParams) { p.Approvals = protocol.ApprovalsAuto })
	task := h.newTask(t, team, "Exercise a trusted runtime callback")
	ref := h.agent(t, task.ID)
	task = h.task(t, task.ID)
	team, _ = h.s.state.team(team.ID)
	for i := range team.Members {
		if team.Members[i].ID == task.Member {
			team.Members[i].Runtime = &runtime
		}
	}
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	return h, protocol.SessionInfo{ID: ref.Session, Team: team.ID, TeamTask: task.ID, Member: task.Member}
}

func runtimeRecord(t *testing.T, h *harness, info protocol.SessionInfo, use string, hosts ...string) protocol.Credential {
	t.Helper()
	c, err := h.s.vault.Add(protocol.VaultAddParams{Name: "RUNTIME_VALUE", Use: use, RuntimeConsent: use == protocol.CredentialRuntime, Hosts: hosts, Teams: []string{info.Team}, Value: runtimeFixtureSecret})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func editRuntimeSeat(t *testing.T, h *harness, info protocol.SessionInfo, edit func(*protocol.Team, *protocol.Member)) {
	t.Helper()
	team, _ := h.s.state.team(info.Team)
	for i := range team.Members {
		if team.Members[i].ID == info.Member {
			edit(&team, &team.Members[i])
			break
		}
	}
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
}

func runtimeNoSecret(t *testing.T, h *harness, value string, seen ...any) {
	t.Helper()
	for i, item := range seen {
		if err, ok := item.(error); ok {
			seen[i] = err.Error()
		}
	}
	h.noSecret(t, value, seen...)
}

func TestRuntimeEnvironmentExportsOnlyConfiguredRuntimeUseForCurrentTeam(t *testing.T) {
	for _, use := range []string{protocol.CredentialRuntime, protocol.CredentialHeader, protocol.CredentialMail} {
		t.Run(use, func(t *testing.T) {
			h, info := runtimeFixture(t, protocol.MemberRuntime{Kind: protocol.MemberRuntimeCommand, Command: []string{"fixture"}, Env: map[string]string{"PUBLIC": "not an export"}, SecretEnv: map[string]string{"PROCESS_KEY": "RUNTIME_VALUE"}})
			var hosts []string
			if use == protocol.CredentialHeader {
				hosts = []string{"example.org"}
			}
			c := runtimeRecord(t, h, info, use, hosts...)
			before, _ := h.s.vault.Get(c.ID)
			got, err := h.s.RuntimeEnvironment(context.Background(), info)
			if use != protocol.CredentialRuntime {
				if err == nil || len(got) != 0 {
					t.Fatal("host/mail-bound value was exported")
				}
				after, _ := h.s.vault.Get(c.ID)
				if !reflect.DeepEqual(before, after) {
					t.Fatal("refused export altered credential metadata")
				}
			} else {
				if err != nil || !reflect.DeepEqual(got, map[string]string{"PROCESS_KEY": runtimeFixtureSecret}) {
					t.Fatalf("configured export failed: %v", err)
				}
				_, err = h.s.vault.Update(protocol.VaultUpdateParams{ID: c.ID, Patch: json.RawMessage(`{"teams":[]}`)})
				if err != nil {
					t.Fatal(err)
				}
				got, err = h.s.RuntimeEnvironment(context.Background(), info)
				if err == nil || len(got) != 0 {
					t.Fatal("revoked team grant still exported")
				}
			}
			runtimeNoSecret(t, h, runtimeFixtureSecret, err)
		})
	}
}

func TestRuntimeCallbacksRejectForgedAndRevokedExecution(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*harness, *protocol.SessionInfo)
	}{
		{"session", func(h *harness, i *protocol.SessionInfo) { i.ID = "forged" }},
		{"team", func(h *harness, i *protocol.SessionInfo) { i.Team = "other-team" }},
		{"member", func(h *harness, i *protocol.SessionInfo) { i.Member = "other-member" }},
		{"task", func(h *harness, i *protocol.SessionInfo) { i.TeamTask = "other-task" }},
		{"member-edit", func(h *harness, i *protocol.SessionInfo) {
			h.s.mu.Lock()
			h.s.editing[i.Team] = map[string]bool{i.Member: true}
			h.s.mu.Unlock()
		}},
		{"task-edit", func(h *harness, i *protocol.SessionInfo) {
			h.s.mu.Lock()
			h.s.taskEdits[i.TeamTask] = true
			h.s.mu.Unlock()
		}},
		{"ended-run", func(h *harness, i *protocol.SessionInfo) {
			h.s.mu.Lock()
			delete(h.s.runs, i.TeamTask)
			h.s.mu.Unlock()
		}},
		{"team-paused", func(h *harness, i *protocol.SessionInfo) {
			editRuntimeSeat(t, h, *i, func(team *protocol.Team, _ *protocol.Member) { team.Paused = true })
		}},
		{"member-paused", func(h *harness, i *protocol.SessionInfo) {
			editRuntimeSeat(t, h, *i, func(_ *protocol.Team, m *protocol.Member) { m.Paused = true })
		}},
		{"global-paused", func(h *harness, i *protocol.SessionInfo) {
			h.settingsMu.Lock()
			h.settings.Teams.Paused = true
			h.settingsMu.Unlock()
		}},
		{"budget", func(h *harness, i *protocol.SessionInfo) {
			editRuntimeSeat(t, h, *i, func(_ *protocol.Team, m *protocol.Member) { m.Budget = &protocol.TeamBudget{MonthTokens: 1} })
			if err := h.s.addSpend(i.Team, i.Member, "", "", h.clock.now(), protocol.TokenUsage{Input: 1}); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			h, info := runtimeFixture(t, protocol.MemberRuntime{Kind: protocol.MemberRuntimeCLI, Command: []string{"fixture"}, SecretEnv: map[string]string{"PROCESS_KEY": "RUNTIME_VALUE"}})
			runtimeRecord(t, h, info, protocol.CredentialRuntime)
			test.mutate(h, &info)
			got, err := h.s.RuntimeEnvironment(context.Background(), info)
			if err == nil || len(got) != 0 {
				t.Fatal("forged or revoked authority exported a credential")
			}
			if _, err := h.s.RuntimeHTTP(context.Background(), info, []byte(`{}`)); err == nil {
				t.Fatal("forged or revoked authority sent a webhook")
			}
			runtimeNoSecret(t, h, runtimeFixtureSecret, err)
		})
	}
}

func TestRuntimeHTTPCentralChokepointRedactsAndDropsSensitiveHeaders(t *testing.T) {
	var received atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		if r.Method != http.MethodPost || r.URL.RequestURI() != "/exact?mode=one" || r.Header.Get("Authorization") != runtimeFixtureSecret {
			t.Error("request was not the exact configured credentialed POST")
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"task":"fixture"}` {
			t.Error("body changed")
		}
		w.Header().Set("Location", "http://169.254.169.254/"+runtimeFixtureSecret)
		w.Header().Set("WWW-Authenticate", runtimeFixtureSecret)
		w.Header().Set("Set-Cookie", runtimeFixtureSecret)
		w.Header().Set("Content-Type", "text/plain; echo="+runtimeFixtureSecret)
		_ = json.NewEncoder(w).Encode(map[string]any{"report": map[string]string{
			"body": runtimeFixtureSecret + " " + base64.StdEncoding.EncodeToString([]byte(runtimeFixtureSecret)),
		}})
	}))
	defer server.Close()
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	h, info := runtimeFixture(t, protocol.MemberRuntime{Kind: protocol.MemberRuntimeWebhook, URL: server.URL + "/exact?mode=one", Credential: "RUNTIME_VALUE"})
	runtimeRecord(t, h, info, protocol.CredentialHeader, strings.TrimPrefix(server.URL, "http://"))
	result, err := h.s.RuntimeHTTP(context.Background(), info, []byte(`{"task":"fixture"}`))
	if err != nil || received.Load() != 1 || result.Status != http.StatusOK {
		t.Fatalf("configured request failed: %v", err)
	}
	if len(result.Headers) != 1 || result.Headers["content-type"] == "" {
		t.Fatal("unsafe or missing response headers")
	}
	runtimeNoSecret(t, h, runtimeFixtureSecret, result, err)
	runtimeNoSecret(t, h, base64.StdEncoding.EncodeToString([]byte(runtimeFixtureSecret)), result)
}

func TestRuntimeHTTPRefusesWithoutSendingOrChangingCredential(t *testing.T) {
	for _, mode := range []string{"readonly", "plan", "cross-team", "runtime-use", "mail-use", "host", "url-secret", "oversize", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
			defer server.Close()
			h, info := runtimeFixture(t, protocol.MemberRuntime{Kind: protocol.MemberRuntimeWebhook, URL: server.URL, Credential: "RUNTIME_VALUE"})
			use := protocol.CredentialHeader
			var hosts = []string{strings.TrimPrefix(server.URL, "http://")}
			if mode == "runtime-use" {
				use = protocol.CredentialRuntime
				hosts = nil
			}
			if mode == "mail-use" {
				use = protocol.CredentialMail
				hosts = nil
			}
			c := runtimeRecord(t, h, info, use, hosts...)
			if mode == "url-secret" {
				editRuntimeSeat(t, h, info, func(_ *protocol.Team, m *protocol.Member) {
					m.Runtime.URL = server.URL + "?token=" + url.QueryEscape(runtimeFixtureSecret)
				})
			}
			switch mode {
			case "readonly":
				editRuntimeSeat(t, h, info, func(team *protocol.Team, _ *protocol.Member) { team.Approvals = protocol.ApprovalsReadOnly })
			case "plan":
				editRuntimeSeat(t, h, info, func(team *protocol.Team, _ *protocol.Member) { team.PlanApproval = true })
			case "cross-team":
				_, _ = h.s.vault.Update(protocol.VaultUpdateParams{ID: c.ID, Patch: json.RawMessage(`{"teams":["different-team"]}`)})
			case "host":
				_, _ = h.s.vault.Update(protocol.VaultUpdateParams{ID: c.ID, Patch: json.RawMessage(`{"hosts":["example.org"]}`)})
			}
			before, _ := h.s.vault.Get(c.ID)
			body := []byte(`{}`)
			if mode == "oversize" {
				body = bytes.Repeat([]byte("a"), maxRuntimeHTTP+1)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancelled" {
				cancel()
			}
			result, err := h.s.RuntimeHTTP(ctx, info, body)
			if err == nil || calls.Load() != 0 {
				t.Fatal("refused runtime operation contacted a receiver")
			}
			after, _ := h.s.vault.Get(c.ID)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("refused runtime operation changed vault metadata")
			}
			runtimeNoSecret(t, h, runtimeFixtureSecret, result, err)
		})
	}
}

func TestRuntimeHTTPResponseBoundsRedirectsCancellationAndRevocation(t *testing.T) {
	for _, mode := range []string{"oversize", "encoded", "redirect", "cancel", "revoke"} {
		t.Run(mode, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				switch mode {
				case "oversize":
					_, _ = io.WriteString(w, strings.Repeat("x", maxRuntimeHTTP)+runtimeFixtureSecret)
				case "encoded":
					w.Header().Set("Content-Encoding", "deflate")
					_, _ = io.WriteString(w, runtimeFixtureSecret)
				case "redirect":
					w.Header().Set("Location", "http://169.254.169.254/"+runtimeFixtureSecret)
					w.WriteHeader(http.StatusTemporaryRedirect)
				default:
					close(entered)
					select {
					case <-release:
					case <-r.Context().Done():
					}
					_, _ = io.WriteString(w, runtimeFixtureSecret)
				}
			}))
			defer server.Close()
			h, info := runtimeFixture(t, protocol.MemberRuntime{Kind: protocol.MemberRuntimeWebhook, URL: server.URL, Credential: "RUNTIME_VALUE"})
			c := runtimeRecord(t, h, info, protocol.CredentialHeader, strings.TrimPrefix(server.URL, "http://"))
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			type answer struct {
				result protocol.AgentHTTPResult
				err    error
			}
			done := make(chan answer, 1)
			go func() { result, err := h.s.RuntimeHTTP(ctx, info, []byte(`{}`)); done <- answer{result, err} }()
			if mode == "cancel" || mode == "revoke" {
				select {
				case <-entered:
				case <-ctx.Done():
					t.Fatal("receiver did not start")
				}
				if mode == "cancel" {
					cancel()
				} else {
					if _, err := h.s.vault.Update(protocol.VaultUpdateParams{ID: c.ID, Patch: json.RawMessage(`{"teams":[]}`)}); err != nil {
						t.Fatal(err)
					}
				}
				close(release)
			}
			var got answer
			select {
			case got = <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("runtime request ignored cancellation/deadline")
			}
			if got.err == nil || got.result.Body != "" || calls.Load() != 1 {
				t.Fatal("unsafe response returned, redirect followed, or request retried")
			}
			runtimeNoSecret(t, h, runtimeFixtureSecret, got.result, got.err)
		})
	}
}

func TestRuntimeURLAndDNSRejectPrivateRebindingBeforeConnect(t *testing.T) {
	for _, raw := range []string{"http://example.org/hook", "https://user:password@example.org/hook", "https://example.org/#fragment", "https://example.org/#", "https://169.254.169.254/", "https://10.0.0.1/", "https://100.100.100.200/", "https://[fc00::1]/", "https://[::ffff:169.254.169.254]/", "https://[::1%25lo]/", "https://example.org:0/", "https://example.org:65536/", "https://example.org:/", "https://*.example.org/", "https://example.org/\\evil", " https://example.org/"} {
		if _, err := checkRuntimeEndpoint(raw); err == nil {
			t.Fatalf("unsafe URL accepted: %s", raw)
		}
	}
	for _, raw := range []string{"http://127.0.0.1:12345/hook?x=1", "http://[::1]:12345/hook", "https://example.org/hook", "https://8.8.8.8/hook"} {
		if _, err := checkRuntimeEndpoint(raw); err != nil {
			t.Fatalf("valid URL refused: %s: %v", raw, err)
		}
	}
	for _, addresses := range [][]netip.Addr{
		{netip.MustParseAddr("127.0.0.1")},
		{netip.MustParseAddr("8.8.8.8"), netip.MustParseAddr("169.254.169.254")},
		{netip.MustParseAddr("::ffff:10.0.0.1")},
		{netip.MustParseAddr("64:ff9b::a00:1")},
	} {
		lookups := 0
		_, err := runtimeDial(context.Background(), "tcp", "fixture.example:443", false, func(context.Context, string) ([]netip.Addr, error) { lookups++; return addresses, nil })
		if err == nil || lookups != 1 || !strings.Contains(err.Error(), "prohibited") {
			t.Fatal("DNS private/mixed/translated addresses were not rejected before connect")
		}
	}
}

type runtimeUnlockKeeper struct {
	key              []byte
	entered, release chan struct{}
}

func (k runtimeUnlockKeeper) Key(ctx context.Context, create, prompt bool) ([]byte, error) {
	if !prompt {
		return nil, errors.New("test keyring starts locked")
	}
	close(k.entered)
	select {
	case <-k.release:
		return bytes.Clone(k.key), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestRuntimeEnvironmentRechecksRevocationAfterBlockingReveal(t *testing.T) {
	for _, mode := range []string{"grant", "run-token", "seat-edit"} {
		t.Run(mode, func(t *testing.T) {
			h, info := runtimeFixture(t, protocol.MemberRuntime{Kind: protocol.MemberRuntimeCommand, Command: []string{"fixture"}, SecretEnv: map[string]string{"PROCESS_KEY": "RUNTIME_VALUE"}})
			key := bytes.Repeat([]byte{19}, 32)
			path := filepath.Join(t.TempDir(), "runtime-vault.db")
			v := vault.OpenWith(path, fixedKeeper{key: key})
			c, err := v.Add(protocol.VaultAddParams{Name: "RUNTIME_VALUE", Use: protocol.CredentialRuntime, RuntimeConsent: true, Teams: []string{info.Team}, Value: runtimeFixtureSecret})
			if err != nil {
				t.Fatal(err)
			}
			if err := v.Close(); err != nil {
				t.Fatal(err)
			}
			entered, release := make(chan struct{}), make(chan struct{})
			if err := h.s.vault.Close(); err != nil {
				t.Fatal(err)
			}
			h.s.vault = vault.OpenWith(path, runtimeUnlockKeeper{key: key, entered: entered, release: release})
			type answer struct {
				env map[string]string
				err error
			}
			done := make(chan answer, 1)
			go func() { env, err := h.s.RuntimeEnvironment(context.Background(), info); done <- answer{env, err} }()
			select {
			case <-entered:
			case <-time.After(time.Second):
				t.Fatal("reveal did not wait on keyring")
			}
			switch mode {
			case "grant":
				if _, err := h.s.vault.Update(protocol.VaultUpdateParams{ID: c.ID, Patch: json.RawMessage(`{"teams":[]}`)}); err != nil {
					t.Fatal(err)
				}
			case "run-token":
				h.s.mu.Lock()
				h.s.runs[info.TeamTask].Token = "replacement-run"
				h.s.mu.Unlock()
			case "seat-edit":
				h.s.mu.Lock()
				h.s.editing[info.Team] = map[string]bool{info.Member: true}
				h.s.mu.Unlock()
			}
			close(release)
			select {
			case got := <-done:
				if got.err == nil || len(got.env) != 0 {
					t.Fatal("revoked authority received plaintext after keyring unlock")
				}
				runtimeNoSecret(t, h, runtimeFixtureSecret, got.err)
			case <-time.After(time.Second):
				t.Fatal("reveal did not return after unlock")
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(raw, []byte(runtimeFixtureSecret)) {
				t.Fatal("blocked reveal persisted plaintext")
			}
		})
	}
}

func TestUncredentialedWebhookUsesSameRuntimeAuthorityAndReadOnlyRefusal(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "" {
			t.Error("uncredentialed runtime inherited authentication")
		}
		_, _ = io.WriteString(w, `"local receiver completed"`)
	}))
	defer server.Close()
	h, info := runtimeFixture(t, protocol.MemberRuntime{Kind: protocol.MemberRuntimeWebhook, URL: server.URL})
	result, err := h.s.RuntimeHTTP(context.Background(), info, []byte(`{}`))
	if err != nil || result.Body != `"local receiver completed"` || calls.Load() != 1 {
		t.Fatalf("configured uncredentialed webhook failed: %v", err)
	}
	editRuntimeSeat(t, h, info, func(team *protocol.Team, _ *protocol.Member) {
		team.Approvals = protocol.ApprovalsReadOnly
	})
	if _, err := h.s.RuntimeHTTP(context.Background(), info, []byte(`{}`)); err == nil || calls.Load() != 1 {
		t.Fatal("uncredentialed webhook bypassed read-only policy")
	}
}

func TestRuntimeEnvironmentNeverExportsForNativeOrWebhookSeats(t *testing.T) {
	for _, kind := range []string{protocol.MemberRuntimeNative, protocol.MemberRuntimeWebhook} {
		t.Run(kind, func(t *testing.T) {
			h, info := runtimeFixture(t, protocol.MemberRuntime{Kind: kind, SecretEnv: map[string]string{"PROCESS_KEY": "RUNTIME_VALUE"}})
			runtimeRecord(t, h, info, protocol.CredentialRuntime)
			got, err := h.s.RuntimeEnvironment(context.Background(), info)
			if err == nil || len(got) != 0 {
				t.Fatal("a non-process runtime received plaintext environment secrets")
			}
			runtimeNoSecret(t, h, runtimeFixtureSecret, err)
		})
	}
}

func TestRuntimeVaultAddRefusesMissingConsentAndRawTrustedMetadataUnchanged(t *testing.T) {
	h := newHarness(t, 0)
	for _, consent := range []bool{false, true} {
		params := map[string]any{
			"name": "RUNTIME_VALUE", "use": protocol.CredentialRuntime,
			"value": runtimeFixtureSecret, "runtimeConsent": consent,
		}
		if consent {
			params["trusted"] = []protocol.CredentialTrust{{Team: "forged-team", Host: "example.org"}}
		}
		before := h.s.vault.List()
		ledgerBefore := h.ledger(t)
		result, err := h.call(protocol.MVaultAdd, params)
		if err == nil {
			t.Fatal("runtime add accepted missing consent or raw host-trust metadata")
		}
		if !reflect.DeepEqual(before, h.s.vault.List()) || !reflect.DeepEqual(ledgerBefore, h.ledger(t)) {
			t.Fatal("refused runtime add changed the vault or activity")
		}
		runtimeNoSecret(t, h, runtimeFixtureSecret, result, err)
	}
}
