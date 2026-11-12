package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

// fakeOffered is what the client offered the fake agent at initialize.
func fakeOffered(params json.RawMessage) string {
	var initialize struct {
		ClientCapabilities struct {
			FS struct {
				Read  bool `json:"readTextFile"`
				Write bool `json:"writeTextFile"`
			} `json:"fs"`
			Terminal bool `json:"terminal"`
		} `json:"clientCapabilities"`
	}
	_ = json.Unmarshal(params, &initialize)
	offered := initialize.ClientCapabilities
	return fmt.Sprintf("read=%t write=%t terminal=%t ", offered.FS.Read, offered.FS.Write, offered.Terminal)
}

// fakeSandboxedTurn says what the agent was offered, then asks the client to
// read a file anyway.
func fakeSandboxedTurn(write func(any), offered string) {
	writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": offered}})
	write(map[string]any{"jsonrpc": "2.0", "id": 200, "method": "fs/read_text_file", "params": map[string]any{
		"sessionId": "fake-session", "path": os.Getenv("RASHIN_ACP_TEST_FILE"),
	}})
}

// fakeSandboxedAnswer says whether the client refused the read, and ends
// the turn.
func fakeSandboxedAnswer(write func(any), message rpcMessage, promptID json.RawMessage) {
	answer := "read"
	if message.Error != nil {
		answer = "refused"
	}
	writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": answer}})
	writePromptResult(write, promptID, "end_turn")
}

func TestASandboxedAgentGetsNoFilesOrTerminalFromRashin(t *testing.T) {
	r, sink := helperRunner(t, "sandboxed", protocol.ApprovalsAuto, nil)
	r.sandbox = func(argv []string) ([]string, error) { return argv, nil }
	path := filepath.Join(r.opts.Cwd, "secret.txt")
	if err := os.WriteFile(path, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RASHIN_ACP_TEST_FILE", path)
	if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "read"}, sink); err != nil {
		t.Fatal(err)
	}
	sink.mu.Lock()
	var said strings.Builder
	for _, event := range sink.events {
		if event.kind == protocol.EvText {
			said.WriteString(event.data.(protocol.TextData).Delta)
		}
		if event.kind == protocol.EvPermission {
			t.Error("a sandboxed agent's read reached the permission path")
		}
	}
	sink.mu.Unlock()
	if got := said.String(); got != "read=false write=false terminal=false refused" {
		t.Fatalf("a sandboxed agent was told and answered %q", got)
	}
}

func TestAnUnsandboxedAgentIsOfferedFilesAndTerminal(t *testing.T) {
	r, sink := helperRunner(t, "sandboxed", protocol.ApprovalsAuto, nil)
	path := filepath.Join(r.opts.Cwd, "notes.txt")
	if err := os.WriteFile(path, []byte("notes"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RASHIN_ACP_TEST_FILE", path)
	if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "read"}, sink); err != nil {
		t.Fatal(err)
	}
	texts := sink.waitKind(t, protocol.EvText, 1)
	if got := texts[0].data.(protocol.TextData).Delta; !strings.HasPrefix(got, "read=true write=true terminal=true") {
		t.Fatalf("an agent outside a sandbox was offered %q", got)
	}
}

func TestADefinitionIsFoundByHarnessOrByTheCommandACustomInstanceRuns(t *testing.T) {
	if def, ok := definitionOf("claude", nil); !ok || !slices.Equal(def.state, []string{".claude", ".claude.json", ".config/claude"}) {
		t.Fatalf("claude's state is %v", def.state)
	}
	if def, ok := definitionOf("my-omp", []string{"/opt/tools/omp", "acp"}); !ok || def.id != "omp" {
		t.Fatalf("a custom instance running omp runs on %q", def.id)
	}
	if _, ok := definitionOf("mine", []string{"/opt/tools/unknown-agent"}); ok {
		t.Fatal("an unknown command has a definition")
	}
}

// A team sandbox runs the harness with HOME and the XDG dirs at the team's
// own home for it, its only writable place beside the team's; every team
// home is hidden beneath it, nothing under the user's own harness state is
// writable, the home's guarded paths are read-only, and only the kept
// environment, the instance's own and the daemon's for the session cross in.
func TestATeamSandboxRunsTheHarnessInItsTeamsHome(t *testing.T) {
	isolateHomes(t)
	t.Setenv("OPENAI_API_KEY", "sk-from-the-login-session")
	t.Setenv("LC_TIME", "en_GB.UTF-8")
	for _, rel := range []string{".omp/agent/extensions", ".hermes/hermes-agent"} {
		if err := os.MkdirAll(filepath.Join(paths.Home(), rel), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeHome(t, "omp", "team-a", ".omp/agent/models.yml", "providers: {}\n")
	base := sandbox.Spec{Folder: "/teams/one", Writable: []string{"/kept"}, ReadOnly: []string{"/programs"}, Env: map[string]string{"RASHIN_SOCKET": "/run/rashin.sock"}}
	spec, err := sandboxFor(base, "omp", "team-a", []string{"omp", "acp"}, map[string]string{"HOME": paths.Home(), "OMP_FLAG": "1"})
	if err != nil {
		t.Fatal(err)
	}
	home := paths.Resolved(HomeOf("omp", "team-a"))
	if !slices.Equal(spec.Writable, []string{"/kept", home}) {
		t.Fatalf("writable %v, want the team's and %s", spec.Writable, home)
	}
	if !slices.Contains(spec.Hidden, paths.Resolved(paths.HarnessHomes())) {
		t.Fatalf("the other teams' homes are not hidden: %v", spec.Hidden)
	}
	if !slices.Contains(spec.ReadOnly, filepath.Join(home, ".omp", "agent", "models.yml")) || !slices.Contains(spec.ReadOnly, filepath.Join(home, ".gitconfig")) {
		t.Fatalf("the guarded paths are not read-only: %v", spec.ReadOnly)
	}
	for _, writable := range spec.Writable {
		for _, state := range StatePaths() {
			if paths.Within(paths.Resolved(state), writable) || paths.Within(writable, paths.Resolved(state)) {
				t.Fatalf("%s is writable and holds or sits in the user's harness state %s", writable, state)
			}
		}
	}
	want := map[string]string{
		"HOME": home, "XDG_CONFIG_HOME": home + "/.config", "XDG_DATA_HOME": home + "/.local/share",
		"XDG_STATE_HOME": home + "/.local/state", "XDG_CACHE_HOME": home + "/.cache",
		"RASHIN_SOCKET": "/run/rashin.sock", "OMP_FLAG": "1", "LC_TIME": "en_GB.UTF-8",
	}
	for name, value := range want {
		if spec.Env[name] != value {
			t.Fatalf("%s is %q inside, want %q", name, spec.Env[name], value)
		}
	}
	if !spec.ClearEnv || spec.Env["OPENAI_API_KEY"] != "" {
		t.Fatalf("the login session's key crosses into the sandbox: clear %t, env %v", spec.ClearEnv, spec.Env)
	}
	info, err := os.Stat(home)
	if err != nil || info.Mode().Perm() != 0o700 {
		t.Fatalf("the team home is %v, %v", info, err)
	}
	gitconfig, err := os.ReadFile(filepath.Join(home, ".gitconfig"))
	if err != nil || !strings.Contains(string(gitconfig), filepath.Join(paths.Home(), ".gitconfig")) {
		t.Fatalf("the team home's git config does not include the user's: %q %v", gitconfig, err)
	}
	if !slices.Equal(base.Writable, []string{"/kept"}) || len(base.Env) != 1 || !slices.Equal(base.ReadOnly, []string{"/programs"}) || len(base.Hidden) != 0 {
		t.Fatalf("completing the sandbox changed the daemon's: %+v", base)
	}

	hermes, err := sandboxFor(base, "hermes", "team-a", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(hermes.ReadOnly, paths.Resolved(filepath.Join(paths.Home(), ".hermes", "hermes-agent"))) {
		t.Fatalf("hermes's own program is not shown again: %v", hermes.ReadOnly)
	}
}
