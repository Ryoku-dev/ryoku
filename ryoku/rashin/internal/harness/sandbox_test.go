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

func TestStateIsFoundByHarnessOrByTheCommandACustomInstanceRuns(t *testing.T) {
	if got := stateOf("claude", nil); !slices.Equal(got, []string{".claude", ".claude.json"}) {
		t.Fatalf("claude's state is %v", got)
	}
	if got := stateOf("my-omp", []string{"/opt/tools/omp", "acp"}); !slices.Equal(got, []string{".omp"}) {
		t.Fatalf("a custom instance running omp has state %v", got)
	}
	if got := stateOf("mine", []string{"/opt/tools/unknown-agent"}); got != nil {
		t.Fatalf("an unknown command has state %v", got)
	}
}

func TestSandboxWritesTheStateThatExistsUnderTheInstanceHome(t *testing.T) {
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, ".omp"), 0o700); err != nil {
		t.Fatal(err)
	}
	base := sandbox.Spec{Folder: "/teams/one", Writable: []string{"/kept"}}
	spec, err := sandboxFor(base, []string{".omp", ".absent"}, map[string]string{"HOME": home})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/kept", paths.Resolved(filepath.Join(home, ".cache")), paths.Resolved(filepath.Join(home, ".omp"))}
	if !slices.Equal(spec.Writable, want) {
		t.Fatalf("writable %v, want %v", spec.Writable, want)
	}
	if info, err := os.Stat(filepath.Join(home, ".cache")); err != nil || !info.IsDir() {
		t.Fatalf("the cache was not created: %v", err)
	}
	if len(base.Writable) != 1 {
		t.Fatalf("completing the sandbox changed the daemon's: %v", base.Writable)
	}
}
