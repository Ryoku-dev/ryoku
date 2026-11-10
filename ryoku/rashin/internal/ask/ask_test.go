package ask

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

type scriptedClient struct {
	mu      sync.Mutex
	streams []*scriptedStream
	calls   []llm.Request
}

func (c *scriptedClient) Stream(_ context.Context, request llm.Request) (llm.Stream, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, request)
	stream := c.streams[0]
	c.streams = c.streams[1:]
	return stream, nil
}

type scriptedStream struct {
	deltas []llm.Delta
	final  llm.Final
	at     int
}

func (s *scriptedStream) Next() (llm.Delta, error) {
	if s.at >= len(s.deltas) {
		return llm.Delta{}, io.EOF
	}
	delta := s.deltas[s.at]
	s.at++
	return delta, nil
}

func (s *scriptedStream) Final() llm.Final { return s.final }
func (s *scriptedStream) Close() error     { return nil }

type recordedEvent struct {
	kind string
	data any
}

type recordingSink struct {
	events []recordedEvent
	info   protocol.SessionInfo
}

func (s *recordingSink) Emit(kind string, data any) {
	s.events = append(s.events, recordedEvent{kind: kind, data: data})
}

func (s *recordingSink) Update(change func(*protocol.SessionInfo)) { change(&s.info) }

func testRunner(t *testing.T, client llm.Client) core.Runner {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	t.Setenv("XDG_RUNTIME_DIR", filepath.Join(root, "runtime"))
	factory := New(client, nil)
	return factory.NewAsk(core.AskOptions{
		Session: protocol.SessionInfo{ID: "ask-test", Model: "auto"},
		Settings: func() protocol.Settings {
			return protocol.Settings{Ask: protocol.AskSettings{Model: "auto"}}
		},
	})
}

func finalStream(deltas ...llm.Delta) *scriptedStream {
	return &scriptedStream{deltas: deltas, final: llm.Final{Provider: "free", Model: "model", Finish: llm.FinishStop}}
}

func TestTurnEventOrderWithToolCall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	call := llm.ToolCall{ID: "call-1", Name: "system_query", Arguments: `{"command":"printf hi"}`}
	client := &scriptedClient{streams: []*scriptedStream{
		finalStream(llm.Delta{ToolCalls: []llm.ToolCall{call}}),
		finalStream(llm.Delta{Text: "The result is hi."}),
	}}
	sink := &recordingSink{}
	result, err := testRunner(t, client).Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Check it"}, sink)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stop != protocol.StopEnd {
		t.Fatalf("stop = %q, want %q", result.Stop, protocol.StopEnd)
	}
	var kinds []string
	for _, event := range sink.events {
		kinds = append(kinds, event.kind)
	}
	want := []string{protocol.EvTool, protocol.EvTool, protocol.EvText, protocol.EvAnswer}
	if strings.Join(kinds, ",") != strings.Join(want, ",") {
		t.Fatalf("events = %v, want %v", kinds, want)
	}
	first := sink.events[0].data.(protocol.ToolData)
	second := sink.events[1].data.(protocol.ToolData)
	if first.Status != protocol.ToolRunning || second.Status != protocol.ToolDone || second.Output != "hi" {
		t.Fatalf("tool transitions = %#v then %#v", first, second)
	}
}

func TestNeedsAgentSplitTailIsNotVisible(t *testing.T) {
	client := &scriptedClient{streams: []*scriptedStream{finalStream(
		llm.Delta{Text: "Open Ryoku Settings.\nNEEDS_"},
		llm.Delta{Text: "AGENT: the setting must be changed"},
	)}}
	sink := &recordingSink{}
	_, err := testRunner(t, client).Prompt(context.Background(), core.PromptInput{Turn: 2, Text: "Change it"}, sink)
	if err != nil {
		t.Fatal(err)
	}
	var visible string
	var answer protocol.AnswerData
	for _, event := range sink.events {
		switch event.kind {
		case protocol.EvText:
			visible += event.data.(protocol.TextData).Delta
		case protocol.EvAnswer:
			answer = event.data.(protocol.AnswerData)
		}
	}
	if visible != "Open Ryoku Settings." {
		t.Fatalf("visible = %q", visible)
	}
	if !answer.NeedsAgent || answer.Reason != "the setting must be changed" {
		t.Fatalf("answer = %#v", answer)
	}
}

func TestProposeBuildsTieredPlan(t *testing.T) {
	call := llm.ToolCall{ID: "plan", Name: "propose", Arguments: `{"summary":"Inspect and install","commands":[{"command":"ls -la","why":"inspect"},{"command":"sudo pacman -S vim","why":"install"}]}`}
	client := &scriptedClient{streams: []*scriptedStream{
		finalStream(llm.Delta{ToolCalls: []llm.ToolCall{call}}),
		finalStream(llm.Delta{Text: "Review the plan."}),
	}}
	sink := &recordingSink{}
	_, err := testRunner(t, client).Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Plan it"}, sink)
	if err != nil {
		t.Fatal(err)
	}
	answer := sink.events[len(sink.events)-1].data.(protocol.AnswerData)
	if answer.Plan == nil || len(answer.Plan.Commands) != 2 {
		t.Fatalf("plan = %#v", answer.Plan)
	}
	if answer.Plan.Commands[0].Tier != protocol.TierRead || answer.Plan.Commands[1].Tier != protocol.TierSystem {
		t.Fatalf("tiers = %#v", answer.Plan.Commands)
	}
}

func TestSystemQueryRefusesWrite(t *testing.T) {
	call := llm.ToolCall{ID: "write", Name: "system_query", Arguments: `{"command":"touch /tmp/rashin-test"}`}
	client := &scriptedClient{streams: []*scriptedStream{
		finalStream(llm.Delta{ToolCalls: []llm.ToolCall{call}}),
		finalStream(llm.Delta{Text: "Use a proposed command instead."}),
	}}
	sink := &recordingSink{}
	_, err := testRunner(t, client).Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Create it"}, sink)
	if err != nil {
		t.Fatal(err)
	}
	done := sink.events[1].data.(protocol.ToolData)
	if done.Status != protocol.ToolDone || !strings.Contains(done.Output, "Use propose") {
		t.Fatalf("refusal = %#v", done)
	}
	if len(client.calls) != 2 || len(client.calls[1].Messages) == 0 {
		t.Fatalf("model requests = %d", len(client.calls))
	}
	last := client.calls[1].Messages[len(client.calls[1].Messages)-1]
	if last.Role != llm.RoleTool || !strings.Contains(last.Content, "Refused") {
		t.Fatalf("tool reply = %#v", last)
	}
}

func TestHistoryBudgetKeepsNewestTurns(t *testing.T) {
	var events []protocol.SessionEvent
	for i := range 20 {
		events = append(events,
			event(protocol.EvUser, protocol.UserData{Text: strings.Repeat(string(rune('a'+i)), 20)}),
			event(protocol.EvText, protocol.TextData{Message: string(rune('A' + i)), Delta: strings.Repeat(string(rune('A'+i)), 20)}),
		)
	}
	messages := historyMessages(events, 85)
	if len(messages) != 4 {
		t.Fatalf("kept %d messages, want 4", len(messages))
	}
	if !strings.Contains(messages[0].Content, "s") || !strings.Contains(messages[len(messages)-1].Content, "T") {
		t.Fatalf("history did not keep newest turns: %#v", messages)
	}
}

func event(kind string, value any) protocol.SessionEvent {
	data, _ := json.Marshal(value)
	return protocol.SessionEvent{Kind: kind, Data: data}
}

func TestExtractActions(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "note.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	text := "See " + file + " and " + root + ", open https://example.com, use `echo hi`, and pick #aabbcc."
	actions := extractActions(text)
	found := make(map[string]bool)
	for _, action := range actions {
		found[action.Kind] = true
	}
	for _, kind := range []string{protocol.ActFile, protocol.ActDir, protocol.ActURL, protocol.ActCmd, protocol.ActColor} {
		if !found[kind] {
			t.Errorf("missing %s action in %#v", kind, actions)
		}
	}
}

// An answer that says which filesystems a command skips used to offer each of
// them twice: as a folder to open and as a command to run.
func TestExtractActionsLeavesNamedSystemRootsAlone(t *testing.T) {
	text := "It excludes virtual filesystems (`/proc`, `/sys`, `/dev`, `/run`), drops errors into /dev/null and reads `/etc/os-release`."
	for _, action := range extractActions(text) {
		if action.Kind == protocol.ActCmd {
			t.Errorf("%q was offered as a command to run", action.Value)
		}
		if action.Kind == protocol.ActDir {
			t.Errorf("%q was offered as a folder to open", action.Value)
		}
		if action.Value == "/dev/null" {
			t.Error("a device was offered as a file to open")
		}
	}
	program := filepath.Join(t.TempDir(), "report")
	if err := os.WriteFile(program, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	actions := extractActions("Run `" + program + " --all` to see it.")
	if len(actions) == 0 || actions[len(actions)-1].Kind != protocol.ActCmd {
		t.Fatalf("a program named by its path is still a command: %#v", actions)
	}
}

func TestReadFileRefusesSecrets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	secret := filepath.Join(home, ".ssh", "id_ed25519")
	if err := os.MkdirAll(filepath.Dir(secret), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secret, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readFileTool(secret, 0, 0, home); err == nil || !strings.Contains(err.Error(), "Refused") {
		t.Fatalf("secret read error = %v", err)
	}
}

func TestFetchURLRefusesPrivateAddress(t *testing.T) {
	_, err := fetchPublic(context.Background(), "http://127.0.0.1/private")
	if err == nil || !strings.Contains(err.Error(), "private or local") {
		t.Fatalf("private fetch error = %v", err)
	}
}

func TestAskModelStrictnessFollowsSessionPin(t *testing.T) {
	t.Run("settings pin may fail over", func(t *testing.T) {
		client := &scriptedClient{streams: []*scriptedStream{finalStream(llm.Delta{Text: "ok"})}}
		runner := New(client, nil).NewAsk(core.AskOptions{
			Session: protocol.SessionInfo{ID: "ask-settings", Model: "provider/model"},
			Settings: func() protocol.Settings {
				return protocol.Settings{Ask: protocol.AskSettings{Model: "provider/model"}}
			},
		})
		if _, err := runner.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "hello"}, &recordingSink{}); err != nil {
			t.Fatal(err)
		}
		if client.calls[0].Strict {
			t.Fatal("settings model pin was marked strict")
		}
	})

	t.Run("set model pin is strict", func(t *testing.T) {
		client := &scriptedClient{streams: []*scriptedStream{finalStream(llm.Delta{Text: "ok"})}}
		runner := testRunner(t, client)
		sink := &recordingSink{}
		if err := runner.SetModel(context.Background(), "provider/model", sink); err != nil {
			t.Fatal(err)
		}
		if _, err := runner.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "hello"}, sink); err != nil {
			t.Fatal(err)
		}
		if !client.calls[0].Strict {
			t.Fatal("session model pin was not marked strict")
		}
	})
}

func TestReadAndListRefuseNonRegularFiles(t *testing.T) {
	root := t.TempDir()
	fifo := filepath.Join(root, "pipe")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readFileTool(fifo, 0, 0, root); err == nil {
		t.Fatal("FIFO read unexpectedly succeeded")
	}
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := listDirTool(file, root); err == nil {
		t.Fatal("file listing unexpectedly succeeded")
	}
}

func TestReadFileChecksResolvedSecretPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	profile := filepath.Join(home, ".mozilla", "profile")
	if err := os.MkdirAll(profile, 0o700); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(profile, "cookies.sqlite")
	if err := os.WriteFile(secret, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(home, "innocent")
	if err := os.Symlink(secret, link); err != nil {
		t.Fatal(err)
	}
	if _, err := readFileTool(link, 0, 0, home); err == nil || !strings.Contains(err.Error(), "Refused") {
		t.Fatalf("symlinked secret read error = %v", err)
	}
}

func TestRunBoundedKillsPipelineProcessGroup(t *testing.T) {
	root := t.TempDir()
	worker := filepath.Join(root, "worker")
	pidFile := filepath.Join(root, "pid")
	body := "#!/bin/sh\nprintf '%s' \"$$\" > \"$1\"\nexec cat /dev/zero\n"
	if err := os.WriteFile(worker, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	commandLine := fmt.Sprintf("%q %q | wc -c", worker, pidFile)
	start := time.Now()
	if _, err := runBounded(context.Background(), time.Second, root, 1024, "sh", "-c", commandLine); err == nil {
		t.Fatal("unbounded pipeline unexpectedly succeeded")
	}
	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Fatalf("pipeline returned after %s, want about one second", elapsed)
	}
	rawPID, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(rawPID))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		err := syscall.Kill(pid, 0)
		if err == syscall.ESRCH {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pipeline child %d remains after timeout: %v", pid, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRunBoundedStopsDevZeroPipeline(t *testing.T) {
	root := t.TempDir()
	pidFile := filepath.Join(root, "pid")
	commandLine := fmt.Sprintf(`printf '%%s' "$$" > %q; cat /dev/zero | wc -c`, pidFile)
	start := time.Now()
	if _, err := runBounded(context.Background(), time.Second, root, 1024, "sh", "-c", commandLine); err == nil {
		t.Fatal("unbounded /dev/zero pipeline unexpectedly succeeded")
	}
	if elapsed := time.Since(start); elapsed < 750*time.Millisecond || elapsed > 3*time.Second {
		t.Fatalf("/dev/zero pipeline returned after %s, want about one second", elapsed)
	}
	rawPID, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(rawPID))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		err := syscall.Kill(-pid, 0)
		if err == syscall.ESRCH {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("pipeline process group %d remains after timeout: %v", pid, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDoctorCacheDoesNotKeepCancelledError(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	count := filepath.Join(root, "count")
	script := "#!/bin/sh\nprintf x >> " + count + "\nif [ \"$(wc -c < " + count + ")\" -eq 1 ]; then sleep 5; fi\nprintf '[]'\n"
	if err := os.WriteFile(filepath.Join(bin, "ryoku"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", root)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	cache := &doctorCache{}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := cache.run(ctx); err == nil {
		t.Fatal("cancelled doctor unexpectedly succeeded")
	}
	if _, err := cache.run(context.Background()); err != nil {
		t.Fatalf("second doctor inherited cancelled error: %v", err)
	}
	body, err := os.ReadFile(count)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != 2 {
		t.Fatalf("doctor ran %d times, want 2", len(body))
	}
}

func TestDoctorSingleFlightWaitRespectsContext(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	started := filepath.Join(root, "started")
	script := "#!/bin/sh\ntouch " + started + "\nsleep 1\nprintf '[]'\n"
	if err := os.WriteFile(filepath.Join(bin, "ryoku"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", root)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	cache := &doctorCache{}
	firstDone := make(chan error, 1)
	go func() {
		_, err := cache.run(context.Background())
		firstDone <- err
	}()
	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("first doctor did not start")
		}
		time.Sleep(5 * time.Millisecond)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := cache.run(ctx); err == nil {
		t.Fatal("waiting doctor ignored cancellation")
	}
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Fatalf("waiting doctor returned after %s", elapsed)
	}
	if err := <-firstDone; err != nil {
		t.Fatalf("first doctor failed: %v", err)
	}
}

// Every ask is told the three things it cannot work out: what Ryoku and its
// vault are, and what Rashin, the system it runs in, is and how it is reached.
func TestAskIsToldAboutRyokuTheVaultAndRashin(t *testing.T) {
	client := &scriptedClient{streams: []*scriptedStream{finalStream(llm.Delta{Text: "Hello."})}}
	runner := testRunner(t, client)
	vault := filepath.Join(os.Getenv("XDG_DATA_HOME"), "ryoku", "vault")
	if err := os.MkdirAll(vault, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "AGENTS.md"), []byte("Read desktop.md before searching."), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "What is Rashin?"}, &recordingSink{}); err != nil {
		t.Fatal(err)
	}
	prompt := client.calls[0].Messages[0].Content
	for _, want := range []string{
		"on a Ryoku Linux desktop",
		"# Ryoku vault", "Read desktop.md before searching.",
		"# Rashin", "Rashin is Ryoku's optional assistant", "rashin search", "Rashin > Settings > Providers",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt does not say %q", want)
		}
	}
}
