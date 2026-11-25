package teams

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// This mirrors the downstream webhook decoder's typed report boundary, then
// sends its decoded fields through the real controller's durable report path.
// It is a local HTTP peer, not a third-party runtime interoperability claim.
type runtimePrivacyResponse struct {
	Version int    `json:"version"`
	Session string `json:"session"`
	Task    string `json:"task"`
	Turn    int    `json:"turn"`
	Status  string `json:"status"`
	Report  struct {
		Title string   `json:"title"`
		Body  string   `json:"body"`
		Files []string `json:"files"`
	} `json:"report"`
}

func runtimePrivacyEnvelope(t *testing.T, info protocol.SessionInfo, report string) string {
	t.Helper()
	return `{"version":1,"session":` + string(mustJSON(t, info.ID)) +
		`,"task":` + string(mustJSON(t, info.TeamTask)) +
		`,"turn":1,"status":"completed","report":` + report + `}`
}

func TestRuntimeHTTPRedactsDecodedCredentialBeforeDurableReport(t *testing.T) {
	for _, test := range []struct {
		name, secret, escaped, prefix string
	}{
		{"escaped-slash", "tenant/key-7", `tenant\/key-7`, ""},
		{"unicode-slash-and-letter", "tenant/key-7", `\u0074enant\u002fkey-7`, ""},
		{"mixed-unicode", "tenant/key-7", `tenant/\u006bey-7`, ""},
		{"prefixed-unicode", "tenant/key-7", `Bearer\u0020tenant\u002fkey-7`, "Bearer "},
		{"short-host-bound", "~!", `\u007e\u0021`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			response := ""
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodPost || r.Header.Get("Authorization") != test.prefix+test.secret || r.Header.Get("Accept-Encoding") != "identity" || r.Header.Get("Content-Type") != "application/json" {
					t.Error("request lost the exact credential or transport headers")
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, response)
			}))
			defer server.Close()
			h, info := runtimeFixture(t, protocol.MemberRuntime{Kind: protocol.MemberRuntimeWebhook, URL: server.URL, Credential: "PRIVACY_VALUE"})
			_, err := h.s.vault.Add(protocol.VaultAddParams{
				Name: "PRIVACY_VALUE", Use: protocol.CredentialHeader, Value: test.secret, Prefix: test.prefix,
				Hosts: []string{strings.TrimPrefix(server.URL, "http://")}, Teams: []string{info.Team},
			})
			if err != nil {
				t.Fatal(err)
			}
			response = runtimePrivacyEnvelope(t, info, `{"title":"Checked `+test.escaped+` result","body":"Evidence `+test.escaped+` preserved","files":["artifact-`+test.escaped+`.txt","unchanged.txt"]}`)
			result, err := h.s.RuntimeHTTP(context.Background(), info, []byte(`{}`))
			if err != nil || calls.Load() != 1 || result.Status != http.StatusOK || result.Truncated {
				t.Fatalf("local webhook failed: %v", err)
			}
			var decoded runtimePrivacyResponse
			decoder := json.NewDecoder(strings.NewReader(result.Body))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&decoded); err != nil {
				t.Fatal(err)
			}
			var trailing any
			if decoder.Decode(&trailing) != io.EOF {
				t.Fatal("redacted response is not exactly one JSON value")
			}
			if decoded.Version != 1 || decoded.Session != info.ID || decoded.Task != info.TeamTask || decoded.Turn != 1 || decoded.Status != "completed" {
				t.Fatal("nonsecret envelope changed")
			}
			if !strings.HasPrefix(decoded.Report.Title, "Checked ") || !strings.HasSuffix(decoded.Report.Title, " result") ||
				!strings.HasPrefix(decoded.Report.Body, "Evidence ") || !strings.HasSuffix(decoded.Report.Body, " preserved") ||
				len(decoded.Report.Files) != 2 || !strings.HasPrefix(decoded.Report.Files[0], "artifact-") || !strings.HasSuffix(decoded.Report.Files[0], ".txt") || decoded.Report.Files[1] != "unchanged.txt" {
				t.Fatal("nonsecret report content changed")
			}
			for _, text := range append([]string{decoded.Report.Title, decoded.Report.Body}, decoded.Report.Files...) {
				if strings.Contains(text, test.secret) || strings.Contains(text, test.escaped) {
					t.Fatal("ordinary JSON decoding reconstructed a request credential")
				}
			}
			h.s.mu.Lock()
			run := h.s.runs[info.TeamTask]
			ref := protocol.AgentRef{Session: run.Session, Token: run.Token}
			h.s.mu.Unlock()
			_, err = h.s.agentReport(WithAgentSession(context.Background(), info.ID), protocol.AgentReportParams{
				AgentRef: ref, Title: decoded.Report.Title, Body: decoded.Report.Body, Files: decoded.Report.Files,
			})
			if err != nil {
				t.Fatalf("decoded report was not persisted: %v", err)
			}
			task := h.task(t, info.TeamTask)
			if task.Status != protocol.TaskDone || task.Report == nil || task.Report.Title != decoded.Report.Title || task.Report.Body != decoded.Report.Body || len(task.Report.Files) != 2 || filepath.Base(task.Report.Files[0]) != decoded.Report.Files[0] || filepath.Base(task.Report.Files[1]) != "unchanged.txt" {
				t.Fatal("sanitized report fields did not survive durable storage")
			}
			for _, text := range append([]string{task.Report.Title, task.Report.Body}, task.Report.Files...) {
				if strings.Contains(text, test.secret) {
					t.Fatal("durable decoded report contains the credential")
				}
			}
			runtimeNoSecret(t, h, test.secret, result, decoded, task.Report)
		})
	}
}

func TestRuntimeHTTPPrivacyPreservesCredentialCollidingControlEnvelope(t *testing.T) {
	for _, collision := range []string{"status", "session", "task"} {
		t.Run(collision, func(t *testing.T) {
			response, secret := "", ""
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Header.Get("Authorization") != secret {
					t.Error("request lost its configured credential")
				}
				_, _ = io.WriteString(w, response)
			}))
			defer server.Close()
			h, info := runtimeFixture(t, protocol.MemberRuntime{Kind: protocol.MemberRuntimeWebhook, URL: server.URL, Credential: "PRIVACY_VALUE"})
			switch collision {
			case "status":
				secret = "completed"
			case "session":
				secret = info.ID[:2]
			case "task":
				secret = info.TeamTask[:2]
			}
			if _, err := h.s.vault.Add(protocol.VaultAddParams{Name: "PRIVACY_VALUE", Use: protocol.CredentialHeader, Value: secret, Hosts: []string{strings.TrimPrefix(server.URL, "http://")}, Teams: []string{info.Team}}); err != nil {
				t.Fatal(err)
			}
			report := map[string]any{"title": secret, "body": secret, "files": []string{secret + ".txt"}}
			response = runtimePrivacyEnvelope(t, info, string(mustJSON(t, report)))
			// The decoder accepts escaped, case-insensitive tagged field names.
			// They must identify report output without changing the wire keys.
			response = strings.Replace(response, `"report":`, `"Re\u0070ort":`, 1)
			result, err := h.s.RuntimeHTTP(context.Background(), info, []byte(`{}`))
			if err != nil || calls.Load() != 1 || result.Status != http.StatusOK || result.Truncated {
				t.Fatalf("credential collision broke the local webhook: %v", err)
			}
			var decoded runtimePrivacyResponse
			decoder := json.NewDecoder(strings.NewReader(result.Body))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&decoded); err != nil {
				t.Fatal(err)
			}
			var trailing any
			if decoder.Decode(&trailing) != io.EOF || decoded.Version != 1 || decoded.Session != info.ID || decoded.Task != info.TeamTask || decoded.Turn != 1 || decoded.Status != "completed" {
				t.Fatal("credential collision changed valid completion metadata")
			}
			if decoded.Report.Title == "" || decoded.Report.Body == "" || len(decoded.Report.Files) != 1 {
				t.Fatal("credential collision removed report fields")
			}
			for _, text := range []string{decoded.Report.Title, decoded.Report.Body, decoded.Report.Files[0]} {
				if strings.Contains(text, secret) {
					t.Fatal("control-envelope exception leaked into decoded report output")
				}
			}
			h.s.mu.Lock()
			run := h.s.runs[info.TeamTask]
			ref := protocol.AgentRef{Session: run.Session, Token: run.Token}
			h.s.mu.Unlock()
			if _, err := h.s.agentReport(WithAgentSession(context.Background(), info.ID), protocol.AgentReportParams{
				AgentRef: ref, Title: decoded.Report.Title, Body: decoded.Report.Body, Files: decoded.Report.Files,
			}); err != nil {
				t.Fatalf("collision-safe completion report was not persisted: %v", err)
			}
			task := h.task(t, info.TeamTask)
			if task.Status != protocol.TaskDone || task.Report == nil || task.Report.Title != decoded.Report.Title || task.Report.Body != decoded.Report.Body || len(task.Report.Files) != 1 || filepath.Base(task.Report.Files[0]) != decoded.Report.Files[0] {
				t.Fatal("collision-safe decoded report did not survive durable storage")
			}
			// IDs/status intentionally retain coincident bytes. Inspect durable
			// report content, not unrelated metadata or the team's folder path.
			for _, text := range []string{task.Report.Title, task.Report.Body, filepath.Base(task.Report.Files[0])} {
				if strings.Contains(text, secret) {
					t.Fatal("durable report output retained a colliding credential")
				}
			}
		})
	}
}

func TestRuntimeHTTPPrivacyRejectsInvalidJSONWithoutRepair(t *testing.T) {
	for _, body := range []string{
		`{"report":{"body":"tenant\/key-7"}`,
		`{"report":{"body":"tenant\/key-7"}} {}`,
		`{"report":{"body":"tenant\/key-7"}} trailing`,
		`{"report":{"body":"tenant\qkey-7"}}`,
		`not JSON tenant/key-7`,
		``,
	} {
		t.Run(body, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			h, info := runtimeFixture(t, protocol.MemberRuntime{Kind: protocol.MemberRuntimeWebhook, URL: server.URL, Credential: "PRIVACY_VALUE"})
			if _, err := h.s.vault.Add(protocol.VaultAddParams{Name: "PRIVACY_VALUE", Use: protocol.CredentialHeader, Value: "tenant/key-7", Hosts: []string{strings.TrimPrefix(server.URL, "http://")}, Teams: []string{info.Team}}); err != nil {
				t.Fatal(err)
			}
			result, err := h.s.RuntimeHTTP(context.Background(), info, []byte(`{}`))
			if err == nil || result.Body != "" || calls.Load() != 1 {
				t.Fatal("invalid JSON was returned as a usable response or request retried")
			}
			if h.task(t, info.TeamTask).Report != nil {
				t.Fatal("invalid response produced a durable report")
			}
			runtimeNoSecret(t, h, "tenant/key-7", result, err)
		})
	}
}

func TestRuntimeHTTPPrivacyPreservesInvalidTypedAndDuplicateFields(t *testing.T) {
	for _, report := range []string{
		`{"title":7,"title":"Checked tenant\/key-7","body":"Evidence","files":[]}`,
		`{"title":"Checked","body":"Evidence tenant\/key-7","files":[7]}`,
		`{"title":"Checked","body":"Evidence tenant\/key-7","unknown":true}`,
	} {
		t.Run(report, func(t *testing.T) {
			response := ""
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, response) }))
			defer server.Close()
			h, info := runtimeFixture(t, protocol.MemberRuntime{Kind: protocol.MemberRuntimeWebhook, URL: server.URL, Credential: "PRIVACY_VALUE"})
			if _, err := h.s.vault.Add(protocol.VaultAddParams{Name: "PRIVACY_VALUE", Use: protocol.CredentialHeader, Value: "tenant/key-7", Hosts: []string{strings.TrimPrefix(server.URL, "http://")}, Teams: []string{info.Team}}); err != nil {
				t.Fatal(err)
			}
			response = runtimePrivacyEnvelope(t, info, report)
			result, err := h.s.RuntimeHTTP(context.Background(), info, []byte(`{}`))
			if err != nil {
				t.Fatal(err)
			}
			var decoded runtimePrivacyResponse
			decoder := json.NewDecoder(strings.NewReader(result.Body))
			decoder.DisallowUnknownFields()
			if decoder.Decode(&decoded) == nil {
				t.Fatal("JSON redaction repaired an invalid typed/duplicate/unknown report field")
			}
		})
	}
}

func TestRuntimeHTTPPrivacyBoundsRedactionExpansion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"report":{"body":"`+strings.Repeat("~!", maxRuntimeHTTP/3)+`"}}`)
	}))
	defer server.Close()
	h, info := runtimeFixture(t, protocol.MemberRuntime{Kind: protocol.MemberRuntimeWebhook, URL: server.URL, Credential: "PRIVACY_VALUE"})
	if _, err := h.s.vault.Add(protocol.VaultAddParams{Name: "PRIVACY_VALUE", Use: protocol.CredentialHeader, Value: "~!", Hosts: []string{strings.TrimPrefix(server.URL, "http://")}, Teams: []string{info.Team}}); err != nil {
		t.Fatal(err)
	}
	result, err := h.s.RuntimeHTTP(context.Background(), info, []byte(`{}`))
	if err != nil || !result.Truncated || len(result.Body) > maxRuntimeHTTP {
		t.Fatalf("redaction expansion escaped the bounded non-completion result: %v", err)
	}
	runtimeNoSecret(t, h, "~!", result, err)
}
