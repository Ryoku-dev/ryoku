package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fakeHarness(id, bin string) fixHarness {
	return fixHarness{ID: id, Name: id, bin: func() (string, bool) { return bin, bin != "" }}
}

func TestFixOpensTheChatAgentFirstThenAnyInstalled(t *testing.T) {
	hs := []fixHarness{fakeHarness("hermes", "/usr/bin/hermes"), fakeHarness("omp", "/usr/bin/omp"), fakeHarness("claude", "")}
	if h, _, _ := pickFixHarness("omp", hs); h.ID != "omp" {
		t.Fatalf("the chosen chat agent must open the fix, got %s", h.ID)
	}
	if h, _, _ := pickFixHarness("claude", hs); h.ID != "hermes" {
		t.Fatalf("a chosen agent that is not installed falls back to the first installed, got %s", h.ID)
	}
	if _, _, ok := pickFixHarness("", []fixHarness{fakeHarness("omp", "")}); ok {
		t.Fatal("no installed harness must not pick one")
	}
}

// Hermes cannot start interactively with a first message, so its recipe runs
// the brief as one query and then resumes that session. A failed first query
// must not resume whatever session happened to be last.
func TestHermesFixResumesOnlyItsOwnFirstTurn(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	bin := filepath.Join(dir, "hermes")
	script := "#!/bin/sh\nprintf '%s|' \"$@\" >> " + log + "\necho >> " + log + "\n[ \"$2\" = -q ] && exit ${FIRST_EXIT:-0}\nexit 0\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	var hermes fixHarness
	for _, h := range fixHarnesses() {
		if h.ID == "hermes" {
			hermes = h
		}
	}
	argv := hermes.argv(bin, "persona", "Fix with AI: x\n\nbrief with 'quotes' and $(no expansion)", FixSettings{})
	for _, c := range []struct {
		exit string
		want []string
	}{
		{"0", []string{"chat|-q|persona\n\nFix with AI: x\n\nbrief with 'quotes' and $(no expansion)|", "chat|--continue|"}},
		{"1", []string{"chat|-q|persona\n\nFix with AI: x\n\nbrief with 'quotes' and $(no expansion)|"}},
	} {
		_ = os.Remove(log)
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Env = append(os.Environ(), "FIRST_EXIT="+c.exit)
		_ = cmd.Run()
		got, _ := os.ReadFile(log)
		calls := strings.Split(strings.TrimSuffix(string(got), "\n"), "|\n")
		for i := range calls {
			calls[i] = strings.TrimSuffix(calls[i], "|") + "|"
		}
		if strings.Join(calls, "\n") != strings.Join(c.want, "\n") {
			t.Errorf("first query exit %s: calls %q, want %q", c.exit, calls, c.want)
		}
	}
}

// A harness that dies at once (no key, no network) must leave its message on
// screen instead of closing the window it opened.
func TestFixTerminalHoldsOnlyAFailure(t *testing.T) {
	for _, c := range []struct {
		argv     []string
		code     int
		holdText bool
	}{
		{[]string{"true"}, 0, false},
		{[]string{"sh", "-c", "exit 3"}, 3, true},
	} {
		argv := holdOnFailure("Oh My Pi", c.argv)
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stdin = strings.NewReader("\n")
		out, _ := cmd.CombinedOutput()
		if cmd.ProcessState.ExitCode() != c.code {
			t.Errorf("%v: exit %d, want %d", c.argv, cmd.ProcessState.ExitCode(), c.code)
		}
		if strings.Contains(string(out), "Oh My Pi stopped (exit 3)") != c.holdText {
			t.Errorf("%v: output %q", c.argv, out)
		}
	}
}

func TestLaunchTerminalFallsBackWithoutSystemd(t *testing.T) {
	oldLookPath, oldRun, oldStart := terminalLookPath, terminalRun, terminalStart
	t.Cleanup(func() {
		terminalLookPath, terminalRun, terminalStart = oldLookPath, oldRun, oldStart
	})
	terminalLookPath = func(name string) (string, error) {
		if name == "ryoku-app" {
			return "/test/bin/ryoku-app", nil
		}
		return "", exec.ErrNotFound
	}
	terminalRun = func(string, []string) ([]byte, error) {
		t.Fatal("systemd-run path used without systemd-run")
		return nil, nil
	}
	var gotName, gotCwd string
	var gotArgs []string
	terminalStart = func(name, cwd string, args []string) error {
		gotName, gotCwd = name, cwd
		gotArgs = append([]string(nil), args...)
		return nil
	}

	if err := launchTerminal("/tmp/vault", "ryoku-chat-", []string{"codex", "resume", "s1"}); err != nil {
		t.Fatal(err)
	}
	if gotName != "/test/bin/ryoku-app" || gotCwd != "/tmp/vault" {
		t.Fatalf("start = %q in %q", gotName, gotCwd)
	}
	if got := strings.Join(gotArgs, "\x00"); got != "terminal\x00--\x00codex\x00resume\x00s1" {
		t.Fatalf("args = %q", gotArgs)
	}
}

func TestCleanTitleDropsATitleCutInsideANote(t *testing.T) {
	cases := map[string]string{
		"plain question":                     "plain question",
		"[system: you are the Needle and so": "",
		"[system: x":                         "",
		"[system: x] why is my disk full":    "why is my disk full",

		noteOpen + "you are the Needle and": "",

		identityNote("x") + "why is my disk full": "why is my disk full",
	}
	for in, want := range cases {
		if got := cleanTitle(in); got != want {
			t.Errorf("cleanTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseFixArgs(t *testing.T) {
	cases := []struct {
		args    []string
		want    fixRequest
		wantErr bool
	}{
		{[]string{"doctor"}, fixRequest{Kind: "doctor"}, false},
		{[]string{"doctor", "failed", "services"}, fixRequest{Kind: "doctor", Name: "failed services"}, false},
		{[]string{"tip", "disk-/"}, fixRequest{Kind: "tip", ID: "disk-/"}, false},
		{[]string{"app", "firefox", "crashes", "on", "start"}, fixRequest{Kind: "app", App: "firefox", Note: "crashes on start"}, false},
		{[]string{"tip"}, fixRequest{}, true},
		{[]string{"app"}, fixRequest{}, true},
		{[]string{"reboot"}, fixRequest{}, true},
		{nil, fixRequest{}, true},
	}
	for _, c := range cases {
		got, err := parseFixArgs(c.args)
		if (err != nil) != c.wantErr {
			t.Errorf("%v: err = %v, want error %v", c.args, err, c.wantErr)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("%v: got %+v, want %+v", c.args, got, c.want)
		}
	}
}

func TestFixAppRejectsAnythingButAName(t *testing.T) {
	for _, bad := range []string{"", "../etc/passwd", "a b", "$(rm -rf ~)", "x;y", "/usr/bin/firefox", strings.Repeat("a", 65)} {
		if _, _, err := buildFix(fixRequest{Kind: "app", App: bad}); err == nil {
			t.Errorf("app %q was accepted", bad)
		}
	}
	for _, ok := range []string{"firefox", "org.gnome.Nautilus", "ryoku-shell", "wireplumber@", "c++"} {
		if _, _, err := buildFix(fixRequest{Kind: "app", App: ok}); err != nil {
			t.Errorf("app %q rejected: %v", ok, err)
		}
	}
}

func TestDoctorIssuesAreWhatDoctorCouldNotSettle(t *testing.T) {
	scan := DoctorScan{Findings: []DoctorFinding{
		{Name: "a", Status: "ok"}, {Name: "b", Status: "note"}, {Name: "c", Status: "fixed"},
		{Name: "d", Status: "todo"}, {Name: "e", Status: "warn"}, {Name: "f", Status: "fail"},
	}}
	var names []string
	for _, f := range scan.Issues() {
		names = append(names, f.Name)
	}
	if strings.Join(names, ",") != "d,e,f" {
		t.Fatalf("issues = %v, want d,e,f", names)
	}
}

func TestFixEndpointRefusesOtherSites(t *testing.T) {
	cases := []struct {
		name, origin, ctype string
		want                int
	}{
		{"another site", "https://evil.example", "application/json", http.StatusForbidden},
		{"a form post", "", "text/plain", http.StatusForbidden},
		{"the dashboard, bad kind", "http://127.0.0.1:3600", "application/json", http.StatusUnprocessableEntity},
		{"the CLI, bad kind", "", "application/json; charset=utf-8", http.StatusUnprocessableEntity},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodPost, "/api/fix", strings.NewReader(`{"kind":"nope"}`))
		if c.origin != "" {
			req.Header.Set("Origin", c.origin)
		}
		req.Header.Set("Content-Type", c.ctype)
		rec := httptest.NewRecorder()
		handleFix(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s: status %d, want %d", c.name, rec.Code, c.want)
		}
	}
}

// Fix with AI runs the agent, model and effort the fix setting names, even
// when another harness is installed and the chat agent is unset.
func TestFixOpensTheConfiguredCodexModelAndEffort(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	bin := t.TempDir()
	for _, name := range []string{"claude", "codex"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	cfg := `{"enabled":true,"port":3600,"fix":{"agent":"codex","model":"gpt-6.1-sol","effort":"low"}}`
	if err := os.MkdirAll(filepath.Join(cfgHome, "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgHome, "ryoku", "rashin.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	l, err := planFix("Fix with AI: x", "brief")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(bin, "codex"), "-m", "gpt-6.1-sol", "-c", `model_reasoning_effort="low"`, needlePersona + "\n\nFix with AI: x\n\nbrief"}
	if l.Harness != "Codex" || strings.Join(l.Argv, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("launch = %s %q, want Codex %q", l.Harness, l.Argv, want)
	}
}

// Fix with AI pressed outside a terminal (the Settings button) must answer at
// once: it opens the terminal running the fix itself, so the slow health check
// and any error it hits happen on screen, not in a hidden process.
func TestFixOffATerminalOpensTheTerminalBeforeTheHealthCheck(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfgHome)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	if err := os.MkdirAll(filepath.Join(cfgHome, "ryoku"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgHome, "ryoku", "rashin.json"), []byte(`{"fix":{"agent":"codex"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTerm, oldLookPath, oldRun := onTerminal, terminalLookPath, terminalRun
	t.Cleanup(func() { onTerminal, terminalLookPath, terminalRun = oldTerm, oldLookPath, oldRun })
	onTerminal = func() bool { return false }
	terminalLookPath = func(name string) (string, error) { return "/test/bin/" + name, nil }
	var got []string
	terminalRun = func(_ string, args []string) ([]byte, error) {
		got = append([]string(nil), args...)
		return nil, nil
	}

	if err := cmdFix([]string{"doctor"}); err != nil {
		t.Fatalf("no ryoku on PATH, so a health check before the terminal would fail: %v", err)
	}
	self, _ := os.Executable()
	tail := strings.Join(got, "\x00")
	if !strings.HasSuffix(tail, "\x00"+self+"\x00fix\x00doctor") || !strings.Contains(tail, "/test/bin/ryoku-app\x00terminal\x00--\x00sh\x00-c") {
		t.Fatalf("terminal args = %q, want the held fix command %s fix doctor", got, self)
	}

	t.Setenv("PATH", t.TempDir())
	got = nil
	if err := cmdFix([]string{"doctor"}); err == nil || got != nil {
		t.Fatalf("with no harness installed: err %v, terminal %q; want an error and no terminal", err, got)
	}
}
