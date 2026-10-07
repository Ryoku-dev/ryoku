package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func putFile(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanHarnessSkillsFollowsSymlinkedDirs(t *testing.T) {
	home := t.TempDir()
	real := filepath.Join(home, "elsewhere", "ryoku")
	putFile(t, filepath.Join(real, "SKILL.md"), "---\nname: ryoku\ndescription: the desktop skill\n---\nbody\n")
	// wire lays skills as symlinks; the scan must count them, not skip them.
	skillsDir := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(skillsDir, "ryoku")); err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(skillsDir, "plain", "SKILL.md"), "---\nname: plain\ndescription: d\n---\n")

	skills, n := scanHarnessSkills("claude", filepath.Join(home, ".claude"))
	if n != 2 || len(skills) != 2 {
		t.Fatalf("n = %d len = %d, want 2: %+v", n, len(skills), skills)
	}
	var shipped, agent bool
	for _, s := range skills {
		if s.Name == "ryoku" && s.Origin == "shipped" {
			shipped = true
		}
		if s.Name == "plain" && s.Origin == "agent" {
			agent = true
			if s.Description != "d" {
				t.Errorf("desc = %q", s.Description)
			}
		}
	}
	if !shipped || !agent {
		t.Fatalf("origin mapping wrong: %+v", skills)
	}
}

func TestScanHarnessMemories(t *testing.T) {
	home := t.TempDir()
	putFile(t, filepath.Join(home, ".hermes", "memories", "MEMORY.md"), "durable notes\n")
	putFile(t, filepath.Join(home, ".hermes", "state.db"), "sqlite")
	putFile(t, filepath.Join(home, ".codex", "AGENTS.md"), "# rules\n")

	notes := scanHarnessMemories("hermes", filepath.Join(home, ".hermes"))
	if len(notes) < 2 {
		t.Fatalf("hermes notes = %+v", notes)
	}
	var foundMd, foundDb bool
	for _, nt := range notes {
		if nt.Name == "MEMORY.md" && nt.Kind == "file" && nt.Bytes > 0 {
			foundMd = true
		}
		if nt.Name == "state.db" && nt.Kind == "db" {
			foundDb = true
		}
	}
	if !foundMd || !foundDb {
		t.Fatalf("missing file or db note: %+v", notes)
	}
	codex := scanHarnessMemories("codex", filepath.Join(home, ".codex"))
	if len(codex) != 1 || codex[0].Name != "AGENTS.md" {
		t.Fatalf("codex notes = %+v", codex)
	}
}

func TestHarnessModelChoice(t *testing.T) {
	home := t.TempDir()
	putFile(t, filepath.Join(home, "config.toml"), "model = \"gpt-5.6-sol\"\nmodel_reasoning_effort = \"high\"\n")
	m, p := harnessModelChoice("codex", home)
	if m != "gpt-5.6-sol" || p != "openai" {
		t.Fatalf("codex model = %q provider = %q", m, p)
	}
	putFile(t, filepath.Join(home, "settings.json"), `{"model":"opus"}`)
	m, _ = harnessModelChoice("claude", home)
	if m != "opus" {
		t.Fatalf("claude model = %q", m)
	}
	m, p = harnessModelChoice("gemini", home) // no model block in settings.json above
	if m != "" {
		t.Fatalf("gemini should find no model, got %q %q", m, p)
	}
}

func TestScanHarnessCredsNamesNotValues(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "super-secret-value")
	t.Setenv("KITTY_PUBLIC_KEY", "not-a-model-key")
	t.Setenv("STARSHIP_SESSION_KEY", "not-a-model-key")
	home := t.TempDir()
	putFile(t, filepath.Join(home, ".env"), "OPENAI_API_KEY=sk-real-thing\nPLAIN=ok\n")
	putFile(t, filepath.Join(home, "auth.json"), `{"OPENAI_API_KEY":"k","tokens":{"access":"x"}}`)

	creds := scanHarnessCreds("hermes", home)
	labels := map[string]string{}
	for _, c := range creds {
		labels[c.Label] = c.Kind
		if c.Label == "super-secret-value" || c.Label == "sk-real-thing" {
			t.Fatalf("credential value leaked as label: %+v", c)
		}
	}
	if _, ok := labels["OPENAI_API_KEY"]; !ok {
		t.Fatalf("env/file key not reported: %+v", creds)
	}
	if _, ok := labels["GEMINI_API_KEY"]; !ok {
		t.Fatalf("process provider key not reported: %+v", creds)
	}
	// session/tooling variables are not model credentials and must stay out
	for _, noise := range []string{"PLAIN", "KITTY_PUBLIC_KEY", "STARSHIP_SESSION_KEY"} {
		if _, ok := labels[noise]; ok {
			t.Fatalf("non-credential key reported: %s", noise)
		}
	}
	// dedupe: OPENAI_API_KEY appears in file and env; each kind once at most.
	seen := map[string]int{}
	for _, c := range creds {
		seen[c.Kind+c.Label]++
	}
	for k, v := range seen {
		if v > 1 {
			t.Fatalf("duplicate cred %s x%d", k, v)
		}
	}
}

func TestScanHarnessesShape(t *testing.T) {
	fake := t.TempDir()
	putFile(t, filepath.Join(fake, ".omp", "agent", "AGENTS.md"), "# omp\n")
	putFile(t, filepath.Join(fake, ".omp", "agent", "agent.db"), "db")
	t.Setenv("HOME", fake)
	rows := ScanHarnesses()
	if len(rows) != 6 {
		t.Fatalf("rows = %d, want 6 known harnesses", len(rows))
	}
	var omp *Harness
	for i := range rows {
		if rows[i].ID == "omp" {
			omp = &rows[i]
		}
	}
	if omp == nil || !omp.Present {
		t.Fatalf("omp not present: %+v", rows)
	}
	// sorting puts present rows first
	if !rows[0].Present {
		t.Fatal("present rows must sort first")
	}
}

func resetHarnessScan() {
	harnessCache.mu.Lock()
	harnessCache.data = nil
	harnessCache.at = time.Time{}
	harnessCache.mu.Unlock()
}

func TestHarnessCacheIsCopiedAndInvalidatedAfterWire(t *testing.T) {
	home := quickTestEnv(t)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	resetHarnessScan()
	first := HarnessesNow()
	if len(first) == 0 {
		t.Fatal("harness list is empty")
	}
	first[0].Name = "corrupted caller copy"
	again := HarnessesNow()
	if again[0].Name == "corrupted caller copy" {
		t.Fatal("caller mutation changed the cached harness list")
	}
	if err := Wire("claude"); err != nil {
		t.Fatal(err)
	}
	for _, row := range HarnessesNow() {
		if row.ID == "claude" && !row.Wired {
			t.Fatal("cached harness wiring did not refresh")
		}
	}
}

func TestConnectHarnessKeepsWirePendingWhenProwlIsNotRoutable(t *testing.T) {
	home := quickTestEnv(t)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	var activate, inject bool
	quickGateway(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/setup/harnesses":
			fmt.Fprint(w, `{"routable":false,"reason":"no provider","harnesses":[{"id":"claude","skills":"current"}]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/setup/harnesses/claude":
			var body map[string]bool
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["activate"] {
				activate = true
				w.WriteHeader(http.StatusConflict)
				fmt.Fprint(w, `{"error":{"type":"conflict","code":"not_routable","message":"no provider"}}`)
				return
			}
			inject = true
			fmt.Fprint(w, `{"harness":{"id":"claude","injected":true,"skills":"current"}}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/setup/skills":
			fmt.Fprint(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	})
	pending, reason, err := connectHarness(context.Background(), "claude")
	if err != nil {
		t.Fatal(err)
	}
	if !pending || reason != "no provider" || !activate || !inject {
		t.Fatalf("pending=%v reason=%q activate=%v inject=%v", pending, reason, activate, inject)
	}
	if !fileHasBlock(filepath.Join(home, ".claude", "CLAUDE.md")) {
		t.Fatal("Rashin pointer was not wired")
	}
	if !LoadConfig().HasProwlHarness("claude") {
		t.Fatal("pending harness was not recorded as connected")
	}
}

func TestDisconnectHarnessDeletesProwlInjection(t *testing.T) {
	home := quickTestEnv(t)
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Wire("codex"); err != nil {
		t.Fatal(err)
	}
	cfg := LoadConfig()
	cfg.AddProwlHarness("codex")
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	deleted := false
	activated := false
	quickGateway(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && r.URL.Path == "/api/setup/harnesses/codex":
			deleted = true
			fmt.Fprint(w, `{}`)
		case r.Method == http.MethodGet && r.URL.Path == "/api/setup/harnesses":
			fmt.Fprint(w, `{"routable":true,"harnesses":[{"id":"codex","injected":false,"active":false,"skills":"current"}]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/setup/harnesses/codex":
			activated = true
			fmt.Fprint(w, `{"harness":{"active":true}}`)
		default:
			http.NotFound(w, r)
		}
	})
	if err := disconnectHarness(context.Background(), "codex"); err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("Prowl injection was not deleted")
	}
	if LoadConfig().HasProwlHarness("codex") {
		t.Fatal("disconnected harness remained in the connected set")
	}
	if fileHasBlock(filepath.Join(home, ".codex", "AGENTS.md")) {
		t.Fatal("Rashin pointer remained after disconnect")
	}
	routeConnectedHarnesses(context.Background())
	if activated {
		t.Fatal("a later routing pass reconnected the disconnected harness")
	}
}

func TestDisconnectHarnessKeepsConnectionWhenGatewayDeleteFails(t *testing.T) {
	home := quickTestEnv(t)
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Wire("codex"); err != nil {
		t.Fatal(err)
	}
	cfg := LoadConfig()
	cfg.AddProwlHarness("codex")
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	quickGateway(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"type":"internal","message":"delete failed"}}`, http.StatusInternalServerError)
	})
	if err := disconnectHarness(context.Background(), "codex"); err == nil {
		t.Fatal("failed gateway delete must fail disconnect")
	}
	if !LoadConfig().HasProwlHarness("codex") {
		t.Fatal("failed disconnect removed the connected harness")
	}
	if !fileHasBlock(filepath.Join(home, ".codex", "AGENTS.md")) {
		t.Fatal("failed disconnect removed the Rashin pointer")
	}
}

func TestRouteConnectedHarnessesOnlyActivatesRecordedConnections(t *testing.T) {
	home := quickTestEnv(t)
	for _, dir := range []string{".claude", ".codex"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := Wire("claude"); err != nil {
		t.Fatal(err)
	}
	cfg := LoadConfig()
	cfg.ProwlHarnesses = []string{"codex"}
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	var activated []string
	quickGateway(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/setup/harnesses":
			fmt.Fprint(w, `{"routable":true,"harnesses":[{"id":"claude","injected":true,"active":false,"skills":"current"},{"id":"codex","injected":true,"active":false,"skills":"current"}]}`)
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api/setup/harnesses/"):
			activated = append(activated, strings.TrimPrefix(r.URL.Path, "/api/setup/harnesses/"))
			fmt.Fprint(w, `{"harness":{"active":true}}`)
		default:
			http.NotFound(w, r)
		}
	})
	result := routeConnectedHarnesses(context.Background())
	if len(activated) != 1 || activated[0] != "codex" {
		t.Fatalf("activated = %v, want only recorded codex connection", activated)
	}
	if len(result.Routed) != 1 || result.Routed[0] != "codex" || len(result.Pending) != 0 {
		t.Fatalf("result = %+v", result)
	}
}

func TestWireAllDoesNotRouteDetectedHarness(t *testing.T) {
	home := quickTestEnv(t)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	activated := false
	quickGateway(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/setup/harnesses":
			fmt.Fprint(w, `{"routable":true,"harnesses":[{"id":"claude","injected":true,"active":false,"skills":"current"}]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/setup/harnesses/claude":
			activated = true
			fmt.Fprint(w, `{"harness":{"active":true}}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/setup/skills":
			fmt.Fprint(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	})
	WireAll()
	if activated {
		t.Fatal("WireAll routed a detected harness the user never connected")
	}
}

func TestRouteConnectedHarnessesRecordsActiveChatAgent(t *testing.T) {
	home := quickTestEnv(t)
	// The chat agent is the one actually answering, so only the claude ACP
	// adapter may resolve: a real hermes on the developer's PATH would win.
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "claude-code-acp"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	cfg := LoadConfig()
	cfg.ChatAgent = "claude"
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	activated := false
	quickGateway(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/setup/harnesses":
			fmt.Fprint(w, `{"routable":true,"harnesses":[{"id":"claude","injected":false,"active":false,"skills":"current"}]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/api/setup/harnesses/claude":
			activated = true
			fmt.Fprint(w, `{"harness":{"injected":true,"active":true}}`)
		default:
			http.NotFound(w, r)
		}
	})
	result := routeConnectedHarnesses(context.Background())
	if !activated || len(result.Routed) != 1 || result.Routed[0] != "claude" {
		t.Fatalf("active chat agent was not routed: activated=%v result=%+v", activated, result)
	}
	if !LoadConfig().HasProwlHarness("claude") {
		t.Fatal("routed chat agent was not recorded as connected")
	}
}

func TestHarnessRoutingConnectionFlags(t *testing.T) {
	home := quickTestEnv(t)
	for _, dir := range []string{".claude", ".codex", ".omp"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cfg := LoadConfig()
	cfg.ProwlHarnesses = []string{"claude"}
	if err := SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	resetHarnessScan()
	quickGateway(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"routable":false,"reason":"no provider","harnesses":[{"id":"claude","injected":true,"active":false,"skills":"current"},{"id":"codex","injected":true,"active":true,"skills":"current"},{"id":"omp","injected":true,"active":false,"skills":"current"}]}`)
	})
	rows := HarnessesWithRouting(context.Background())
	byID := make(map[string]Harness, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	if got := byID["claude"].Routing; !got.Connected || !got.Pending || got.Active {
		t.Fatalf("connected pending routing = %+v", got)
	}
	if got := byID["codex"].Routing; got.Connected || got.Pending || !got.Active {
		t.Fatalf("manually active routing = %+v", got)
	}
	if got := byID["omp"].Routing; got.Connected || got.Pending || got.Active {
		t.Fatalf("injected-only routing = %+v", got)
	}
}

func TestGeminiRoutingIsUnsupported(t *testing.T) {
	quickTestEnv(t)
	resetHarnessScan()
	quickGateway(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"routable":true,"harnesses":[{"id":"gemini","injected":true,"active":true,"skills":"unsupported"}]}`)
	})
	rows := HarnessesWithRouting(context.Background())
	for _, row := range rows {
		if row.ID == "gemini" {
			if row.Routing.Supported || row.Routing.Note != geminiRoutingReason {
				t.Fatalf("routing = %+v", row.Routing)
			}
			return
		}
	}
	t.Fatal("Gemini row missing")
}
