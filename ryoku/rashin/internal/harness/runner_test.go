package harness

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

type recordedEvent struct {
	kind string
	data any
}

type testSink struct {
	mu     sync.Mutex
	events []recordedEvent
	info   protocol.SessionInfo
	wake   chan struct{}
}

func newTestSink() *testSink {
	return &testSink{wake: make(chan struct{}, 64)}
}

func (s *testSink) Emit(kind string, data any) {
	s.mu.Lock()
	s.events = append(s.events, recordedEvent{kind: kind, data: data})
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *testSink) Update(change func(*protocol.SessionInfo)) {
	s.mu.Lock()
	change(&s.info)
	s.mu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *testSink) waitKind(t *testing.T, kind string, count int) []recordedEvent {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		s.mu.Lock()
		var matches []recordedEvent
		for _, event := range s.events {
			if event.kind == kind {
				matches = append(matches, event)
			}
		}
		s.mu.Unlock()
		if len(matches) >= count {
			return matches
		}
		select {
		case <-s.wake:
		case <-deadline.C:
			t.Fatalf("timed out waiting for %d %s events", count, kind)
		}
	}
}

func lastOf[T any](t *testing.T, sink *testSink, kind string) T {
	t.Helper()
	sink.mu.Lock()
	defer sink.mu.Unlock()
	for index := len(sink.events) - 1; index >= 0; index-- {
		if sink.events[index].kind == kind {
			return sink.events[index].data.(T)
		}
	}
	t.Fatalf("no %s event was reported", kind)
	var zero T
	return zero
}

func countOf(sink *testSink, kind string) int {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	count := 0
	for _, event := range sink.events {
		if event.kind == kind {
			count++
		}
	}
	return count
}

func helperRunner(t *testing.T, mode string, approvals string, resume json.RawMessage) (*runner, *testSink) {
	t.Helper()
	t.Setenv("RASHIN_ACP_TEST_MODE", mode)
	cwd := t.TempDir()
	r := newRunner(core.HarnessOptions{
		Cwd: cwd,
		Approvals: func() string {
			return approvals
		},
		Resume: resume,
	}, []string{os.Args[0], "-test.run=^TestACPHelperProcess$"})
	sink := newTestSink()
	t.Cleanup(func() { _ = r.Stop() })
	return r, sink
}

func TestPromptTranslatesACPUpdates(t *testing.T) {
	r, sink := helperRunner(t, "updates", protocol.ApprovalsAsk, nil)
	result, err := r.Prompt(context.Background(), core.PromptInput{Text: "hello"}, sink)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stop != protocol.StopEnd || result.Usage == nil || result.Usage.Input != 12 {
		t.Fatalf("unexpected result: %+v", result)
	}
	for _, kind := range []string{protocol.EvText, protocol.EvThought, protocol.EvTool, protocol.EvPlan, protocol.EvCommands, protocol.EvMode, protocol.EvModel, protocol.EvTitle} {
		if len(sink.waitKind(t, kind, 1)) == 0 {
			t.Fatalf("missing %s", kind)
		}
	}
	texts := sink.waitKind(t, protocol.EvText, 2)
	first := texts[0].data.(protocol.TextData)
	second := texts[1].data.(protocol.TextData)
	if first.Message != second.Message {
		t.Fatalf("contiguous text chunks changed message id: %q != %q", first.Message, second.Message)
	}
}

// handed is the prompt as the agent received it
func handed(t *testing.T, mode string, attachments ...protocol.Attachment) []map[string]any {
	t.Helper()
	r, sink := helperRunner(t, mode, protocol.ApprovalsAsk, nil)
	if _, err := r.Prompt(context.Background(), core.PromptInput{Text: "What went wrong?", Images: attachments}, sink); err != nil {
		t.Fatal(err)
	}
	var blocks []map[string]any
	echoed := sink.waitKind(t, protocol.EvText, 1)[0].data.(protocol.TextData).Delta
	if err := json.Unmarshal([]byte(echoed), &blocks); err != nil {
		t.Fatalf("the agent echoed %q: %v", echoed, err)
	}
	return blocks
}

// The composer attaches any file. Every one used to be sent as a picture,
// and none at all to an agent that takes no pictures.
func TestAnAgentIsHandedPicturesAsPicturesAndFilesAsLinks(t *testing.T) {
	folder := t.TempDir()
	picture, log := filepath.Join(folder, "screenshot"), filepath.Join(folder, "build log.txt")
	png := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...)
	if err := os.WriteFile(picture, png, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(log, []byte("error: linker failed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	attachments := []protocol.Attachment{{Path: picture}, {Path: log}}
	link := map[string]any{"type": "resource_link", "uri": (&url.URL{Scheme: "file", Path: log}).String(), "name": "build log.txt"}

	blocks := handed(t, "echo", attachments...)
	if len(blocks) != 3 || blocks[0]["text"] != "What went wrong?" {
		t.Fatalf("the agent was handed %v", blocks)
	}
	if blocks[1]["type"] != "image" || blocks[1]["mimeType"] != "image/png" || blocks[1]["data"] != base64.StdEncoding.EncodeToString(png) {
		t.Fatalf("the picture went as %v", blocks[1])
	}
	if !reflect.DeepEqual(blocks[2], link) {
		t.Fatalf("the log went as %v, want %v", blocks[2], link)
	}

	blocks = handed(t, "echo-no-pictures", attachments...)
	if len(blocks) != 3 || blocks[1]["type"] != "resource_link" || blocks[1]["name"] != "screenshot" || !reflect.DeepEqual(blocks[2], link) {
		t.Fatalf("an agent that takes no pictures was handed %v", blocks)
	}
}

func TestAnAttachmentThatIsGoneStopsThePrompt(t *testing.T) {
	r, sink := helperRunner(t, "echo", protocol.ApprovalsAsk, nil)
	gone := filepath.Join(t.TempDir(), "gone.txt")
	_, err := r.Prompt(context.Background(), core.PromptInput{Text: "Look", Images: []protocol.Attachment{{Path: gone}}}, sink)
	if err == nil || !strings.Contains(err.Error(), gone) {
		t.Fatalf("a prompt with a missing attachment went on: %v", err)
	}
}

func TestReadOnlyPolicyAndPermissionBeforeTool(t *testing.T) {
	r, sink := helperRunner(t, "permissions", protocol.ApprovalsReadOnly, nil)
	done := make(chan error, 1)
	go func() {
		_, err := r.Prompt(context.Background(), core.PromptInput{Text: "tools"}, sink)
		done <- err
	}()
	permissions := sink.waitKind(t, protocol.EvPermission, 1)
	permission := permissions[0].data.(protocol.PermissionData)
	if permission.Call != "edit-1" {
		t.Fatalf("permission did not retain tool call: %+v", permission)
	}
	events := sink.waitKind(t, protocol.EvTool, 2)
	if events[0].data.(protocol.ToolData).Call != "read-1" || events[1].data.(protocol.ToolData).Call != "edit-1" {
		t.Fatalf("permission tool ordering was not preserved: %+v", events)
	}
	if err := r.Permission(permission.Request, "allow"); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	resolved := sink.waitKind(t, protocol.EvPermissionResolved, 2)
	by := map[string]bool{}
	for _, event := range resolved {
		by[event.data.(protocol.PermissionResolvedData).By] = true
	}
	if !by["policy"] || !by["user"] {
		t.Fatalf("unexpected permission resolution: %+v", resolved)
	}
}

func TestCancelResolvesEveryPermissionOnce(t *testing.T) {
	r, sink := helperRunner(t, "cancel", protocol.ApprovalsAsk, nil)
	done := make(chan struct{})
	go func() {
		_, _ = r.Prompt(context.Background(), core.PromptInput{Text: "cancel"}, sink)
		close(done)
	}()
	sink.waitKind(t, protocol.EvPermission, 2)
	r.Cancel()
	resolved := sink.waitKind(t, protocol.EvPermissionResolved, 2)
	if len(resolved) != 2 {
		t.Fatalf("resolved %d permissions", len(resolved))
	}
	r.Cancel()
	time.Sleep(20 * time.Millisecond)
	sink.mu.Lock()
	count := 0
	for _, event := range sink.events {
		if event.kind == protocol.EvPermissionResolved {
			count++
		}
	}
	sink.mu.Unlock()
	if count != 2 {
		t.Fatalf("cancel resolved permissions more than once: %d", count)
	}
	<-done
}

func TestCancelThenPromptOnSameRunner(t *testing.T) {
	r, sink := helperRunner(t, "cancel-next", protocol.ApprovalsAsk, nil)
	first := make(chan struct {
		result core.TurnResult
		err    error
	}, 1)
	go func() {
		result, err := r.Prompt(context.Background(), core.PromptInput{Text: "first"}, sink)
		first <- struct {
			result core.TurnResult
			err    error
		}{result: result, err: err}
	}()
	sink.waitKind(t, protocol.EvPermission, 1)
	r.Cancel()
	second := make(chan error, 1)
	go func() {
		_, err := r.Prompt(context.Background(), core.PromptInput{Text: "second"}, sink)
		second <- err
	}()
	select {
	case turn := <-first:
		if turn.err != nil {
			t.Fatal(turn.err)
		}
		if turn.result.Stop != protocol.StopCancelled {
			t.Fatalf("cancelled prompt stopped as %q", turn.result.Stop)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled prompt did not return")
	}
	select {
	case err := <-second:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("next prompt did not return")
	}
	texts := sink.waitKind(t, protocol.EvText, 1)
	if texts[len(texts)-1].data.(protocol.TextData).Delta != "second" {
		t.Fatalf("next prompt did not produce its response: %+v", texts)
	}
	r.mu.Lock()
	permissions, terminals := len(r.permissions), len(r.terminals)
	r.mu.Unlock()
	if permissions != 0 || terminals != 0 {
		t.Fatalf("cancelled turn leaked state: permissions=%d terminals=%d", permissions, terminals)
	}
}

func TestFileWriteRecordsDiff(t *testing.T) {
	r, sink := helperRunner(t, "write", protocol.ApprovalsAuto, nil)
	path := filepath.Join(r.opts.Cwd, "file.txt")
	if err := os.WriteFile(path, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RASHIN_ACP_TEST_FILE", path)
	if _, err := r.Prompt(context.Background(), core.PromptInput{Text: "write"}, sink); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil || string(content) != "new" {
		t.Fatalf("file content = %q, %v", content, err)
	}
	for _, event := range sink.waitKind(t, protocol.EvTool, 2) {
		tool := event.data.(protocol.ToolData)
		if tool.Diff != nil && tool.Diff.Old == "old" && tool.Diff.New == "new" {
			return
		}
	}
	t.Fatal("write did not emit its diff")
}

func TestTerminalCreateOutputWaitAndRelease(t *testing.T) {
	r, sink := helperRunner(t, "terminal", protocol.ApprovalsAuto, nil)
	if _, err := r.Prompt(context.Background(), core.PromptInput{Text: "terminal"}, sink); err != nil {
		t.Fatal(err)
	}
	events := sink.waitKind(t, protocol.EvTerminal, 3)
	var output string
	var done bool
	for _, event := range events {
		terminal := event.data.(protocol.TerminalData)
		output += terminal.Delta
		done = done || terminal.Done
	}
	if !strings.Contains(output, "terminal-ok") || !done {
		t.Fatalf("terminal events missing output or completion: %q, done=%t", output, done)
	}
}

func TestResumeUsesLoadOnlyWhenAdvertised(t *testing.T) {
	resume := json.RawMessage(`{"acpSession":"past","loadSession":true}`)
	for _, test := range []struct {
		mode  string
		title string
	}{
		{mode: "resume-load", title: "loaded"},
		{mode: "resume-new", title: "new"},
	} {
		t.Run(test.mode, func(t *testing.T) {
			r, sink := helperRunner(t, test.mode, protocol.ApprovalsAuto, resume)
			if _, err := r.Prompt(context.Background(), core.PromptInput{Text: "resume"}, sink); err != nil {
				t.Fatal(err)
			}
			titles := sink.waitKind(t, protocol.EvTitle, 1)
			if titles[0].data.(protocol.TitleData).Title != test.title {
				t.Fatalf("opened wrong ACP session method: %+v", titles)
			}
		})
	}
}

func TestUnexpectedExitSurfacesStderr(t *testing.T) {
	r, sink := helperRunner(t, "exit", protocol.ApprovalsAuto, nil)
	_, err := r.Prompt(context.Background(), core.PromptInput{Text: "exit"}, sink)
	if err == nil {
		t.Fatal("prompt succeeded after agent exit")
	}
	errors := sink.waitKind(t, protocol.EvError, 1)
	message := errors[0].data.(protocol.ErrorData).Message
	if !strings.Contains(message, "scripted failure") || r.Alive() {
		t.Fatalf("unexpected exit was not surfaced: %q, alive=%t", message, r.Alive())
	}
}

func TestACPHelperProcess(t *testing.T) {
	mode := os.Getenv("RASHIN_ACP_TEST_MODE")
	if mode == "" {
		return
	}
	runFakeAgent(mode)
	os.Exit(0)
}

func runFakeAgent(mode string) {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64<<10), acpLineLimit)
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()
	write := func(value any) {
		encoded, _ := json.Marshal(value)
		_, _ = writer.Write(append(encoded, '\n'))
		_ = writer.Flush()
	}
	var promptID json.RawMessage
	permissionResponses := 0
	prompts := 0
	chosen := map[string]string{"mode": "default", "model": "big", "thinking": "auto"}
	terminalID := ""
	terminalBase := 0
	for scanner.Scan() {
		var message rpcMessage
		if json.Unmarshal(scanner.Bytes(), &message) != nil {
			continue
		}
		switch message.Method {
		case "initialize":
			load := mode == "resume-load" || mode == "capabilities" || mode == "load-replay" || mode == "generation" || mode == "echo-load"
			promptCapabilities := map[string]bool{"image": mode != "echo-no-pictures"}
			if mode == "capabilities" {
				promptCapabilities["audio"] = true
				promptCapabilities["embeddedContext"] = true
			}
			write(map[string]any{"jsonrpc": "2.0", "id": rawID(message.ID), "result": map[string]any{
				"protocolVersion":   1,
				"agentCapabilities": map[string]any{"loadSession": load, "promptCapabilities": promptCapabilities},
			}})
		case "session/new", "session/load":
			title := "new"
			if message.Method == "session/load" {
				title = "loaded"
			}
			if strings.HasPrefix(mode, "resume-") {
				writeUpdate(write, map[string]any{"sessionUpdate": "session_info_update", "title": title})
			}
			if mode == "load-replay" && message.Method == "session/load" {
				writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": "replayed"}})
				writeUpdate(write, map[string]any{"sessionUpdate": "tool_call", "toolCallId": "replayed-tool", "title": "Replay", "kind": "read"})
				writeUpdate(write, map[string]any{"sessionUpdate": "plan", "entries": []map[string]string{{"content": "Replay", "status": "pending"}}})
				writeUpdate(write, map[string]any{"sessionUpdate": "available_commands_update", "availableCommands": []map[string]string{{"name": "replay", "description": "Replay"}}})
				writeUpdate(write, map[string]any{"sessionUpdate": "current_mode_update", "currentModeId": "code"})
				writeUpdate(write, map[string]any{"sessionUpdate": "config_option_update", "configOptions": []map[string]any{{
					"id": "model", "category": "model", "type": "select", "currentValue": "fast",
					"options": []map[string]string{{"value": "fast", "name": "Fast"}},
				}}})
				writeUpdate(write, map[string]any{"sessionUpdate": "current_model_update", "currentModelId": "smart"})
			}
			result := map[string]any{"sessionId": "fake-session"}
			if strings.HasPrefix(mode, "options") {
				result["configOptions"] = fakeConfigOptions(chosen)
			}
			if mode == "capabilities" {
				result["models"] = map[string]any{
					"availableModels": []map[string]string{{"modelId": "smart", "name": "Smart"}},
					"currentModelId":  "smart",
				}
				result["modes"] = map[string]any{
					"availableModes": []map[string]string{{"id": "code", "name": "Code"}},
					"currentModeId":  "code",
				}
			}
			write(map[string]any{"jsonrpc": "2.0", "id": rawID(message.ID), "result": result})
		case "session/prompt":
			prompts++
			promptID = append(json.RawMessage(nil), message.ID...)
			switch mode {
			case "echo", "echo-no-pictures", "echo-load":
				var params struct {
					Prompt json.RawMessage `json:"prompt"`
				}
				_ = json.Unmarshal(message.Params, &params)
				writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": string(params.Prompt)}})
				writePromptResult(write, promptID, "end_turn")
			case "updates":
				fakeUpdates(write)
				writePromptResult(write, promptID, "end_turn")
			case "subagents":
				fakeSubagentsOmp(write)
				writePromptResult(write, promptID, "end_turn")
			case "subagents-claude":
				fakeSubagentsClaude(write)
				writePromptResult(write, promptID, "end_turn")
			case "permissions":
				writePermission(write, 100, "read-1", "read")
			case "cancel":
				writePermission(write, 100, "read-1", "edit")
				writePermission(write, 101, "edit-1", "edit")
			case "write":
				write(map[string]any{"jsonrpc": "2.0", "id": 200, "method": "fs/write_text_file", "params": map[string]any{"sessionId": "fake-session", "path": os.Getenv("RASHIN_ACP_TEST_FILE"), "content": "new"}})
			case "terminal":
				write(map[string]any{"jsonrpc": "2.0", "id": 200, "method": "terminal/create", "params": map[string]any{"sessionId": "fake-session", "command": "printf", "args": []string{"terminal-ok"}}})
			case "resume-load", "resume-new":
				writePromptResult(write, promptID, "end_turn")
			case "environment":
				writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": os.Getenv("RASHIN_CUSTOM_VALUE")}})
				writePromptResult(write, promptID, "end_turn")
			case "capabilities":
				writePromptResult(write, promptID, "end_turn")
			case "cancel-next":
				if prompts == 1 {
					writePermission(write, 100, "edit-1", "edit")
				} else {
					writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": "second"}})
					writePromptResult(write, promptID, "end_turn")
				}
			case "exit":
				_, _ = fmt.Fprintln(os.Stderr, "scripted failure")
				return
			case "load-replay":
				writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": "fresh"}})
				writePromptResult(write, promptID, "end_turn")
			case "generation":
				writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": "generation"}})
				writePermission(write, 100, "edit-1", "edit")
			case "cancel-late":
				if prompts == 1 {
					writeUpdate(write, map[string]any{"sessionUpdate": "plan", "entries": []map[string]string{{"content": "started", "status": "in_progress"}}})
				} else {
					writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": "second"}})
					writePromptResult(write, promptID, "end_turn")
				}
			case "cancel-ignore":
				writeUpdate(write, map[string]any{"sessionUpdate": "plan", "entries": []map[string]string{{"content": "started", "status": "in_progress"}}})
			case "terminal-large":
				terminalBase = 200 + prompts*10
				write(map[string]any{"jsonrpc": "2.0", "id": terminalBase, "method": "terminal/create", "params": map[string]any{
					"sessionId": "fake-session", "command": "sh", "args": []string{"-c", "head -c 200000 /dev/zero | tr '\\0' x"},
				}})
			case "terminal-persist":
				if prompts == 1 {
					write(map[string]any{"jsonrpc": "2.0", "id": 200, "method": "terminal/create", "params": map[string]any{
						"sessionId": "fake-session", "command": "sh", "args": []string{"-c", "sleep 0.1; printf kept"},
					}})
				} else {
					write(map[string]any{"jsonrpc": "2.0", "id": 210, "method": "terminal/wait_for_exit", "params": map[string]any{
						"sessionId": "fake-session", "terminalId": terminalID,
					}})
				}
			case "terminal-cancel":
				write(map[string]any{"jsonrpc": "2.0", "id": 200, "method": "terminal/create", "params": map[string]any{
					"sessionId": "fake-session", "command": "sh", "args": []string{"-c", "sleep 30"},
				}})
			case "synthetic-ids":
				write(map[string]any{"jsonrpc": "2.0", "id": 200, "method": "fs/write_text_file", "params": map[string]any{
					"sessionId": "fake-session", "path": os.Getenv("RASHIN_ACP_TEST_FILE"), "content": "changed",
				}})
			case "file-read-outside":
				write(map[string]any{"jsonrpc": "2.0", "id": 200, "method": "fs/read_text_file", "params": map[string]any{
					"sessionId": "fake-session", "path": os.Getenv("RASHIN_ACP_TEST_FILE"),
				}})
			case "file-write-ask", "file-write-readonly", "file-write-auto", "file-write-outside":
				write(map[string]any{"jsonrpc": "2.0", "id": 200, "method": "fs/write_text_file", "params": map[string]any{
					"sessionId": "fake-session", "path": os.Getenv("RASHIN_ACP_TEST_FILE"), "content": "changed",
				}})
			case "file-relative":
				write(map[string]any{"jsonrpc": "2.0", "id": 200, "method": "fs/read_text_file", "params": map[string]any{
					"sessionId": "fake-session", "path": "relative.txt",
				}})
			case "crash-child":
				marker := os.Getenv("RASHIN_ACP_TEST_MARKER")
				if _, err := os.Stat(marker); errors.Is(err, os.ErrNotExist) {
					_ = os.WriteFile(marker, []byte("crashed"), 0600)
					child := exec.Command("sh", "-c", "sleep 30")
					child.Stderr = os.Stderr
					_ = child.Start()
					return
				}
				writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": "restarted"}})
				writePromptResult(write, promptID, "end_turn")
			}
		case "session/set_config_option":
			var params struct {
				ConfigID string `json:"configId"`
				Value    string `json:"value"`
			}
			_ = json.Unmarshal(message.Params, &params)
			chosen[params.ConfigID] = params.Value
			result := map[string]any{}
			if mode != "options-silent" {
				result["configOptions"] = fakeConfigOptions(chosen)
			}
			write(map[string]any{"jsonrpc": "2.0", "id": rawID(message.ID), "result": result})
		case "session/cancel":
			if mode == "cancel" && permissionResponses == 2 {
				writePromptResult(write, promptID, "cancelled")
			}
			if mode == "cancel-next" {
				writePromptResult(write, promptID, "cancelled")
			}
			if mode == "cancel-late" {
				writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": "late-before-cancelled"}})
				time.Sleep(100 * time.Millisecond)
				writePromptResult(write, promptID, "cancelled")
			}
			if mode == "terminal-cancel" {
				writePromptResult(write, promptID, "cancelled")
			}
		default:
			if len(message.ID) == 0 || message.Method != "" {
				continue
			}
			switch mode {
			case "permissions":
				permissionResponses++
				if permissionResponses == 1 {
					writePermission(write, 101, "edit-1", "edit")
				} else {
					writePromptResult(write, promptID, "end_turn")
				}
			case "cancel":
				permissionResponses++
				if permissionResponses == 2 {
					writePromptResult(write, promptID, "cancelled")
				}
			case "write":
				writePromptResult(write, promptID, "end_turn")
			case "terminal":
				var response struct {
					TerminalID string `json:"terminalId"`
				}
				_ = json.Unmarshal(message.Result, &response)
				if response.TerminalID != "" {
					terminalID = response.TerminalID
					write(map[string]any{"jsonrpc": "2.0", "id": 201, "method": "terminal/output", "params": map[string]any{"sessionId": "fake-session", "terminalId": terminalID}})
				} else if rpcID(message.ID) == "201" {
					write(map[string]any{"jsonrpc": "2.0", "id": 202, "method": "terminal/wait_for_exit", "params": map[string]any{"sessionId": "fake-session", "terminalId": terminalID}})
				} else if rpcID(message.ID) == "202" {
					write(map[string]any{"jsonrpc": "2.0", "id": 203, "method": "terminal/release", "params": map[string]any{"sessionId": "fake-session", "terminalId": terminalID}})
				} else if rpcID(message.ID) == "203" {
					writePromptResult(write, promptID, "end_turn")
				}
			case "generation":
				writePromptResult(write, promptID, "end_turn")
			case "terminal-large":
				id := rpcID(message.ID)
				switch id {
				case fmt.Sprint(terminalBase):
					var response struct {
						TerminalID string `json:"terminalId"`
					}
					_ = json.Unmarshal(message.Result, &response)
					terminalID = response.TerminalID
					write(map[string]any{"jsonrpc": "2.0", "id": terminalBase + 1, "method": "terminal/wait_for_exit", "params": map[string]any{
						"sessionId": "fake-session", "terminalId": terminalID,
					}})
				case fmt.Sprint(terminalBase + 1):
					write(map[string]any{"jsonrpc": "2.0", "id": terminalBase + 2, "method": "terminal/output", "params": map[string]any{
						"sessionId": "fake-session", "terminalId": terminalID,
					}})
				case fmt.Sprint(terminalBase + 2):
					var response struct {
						Output    string `json:"output"`
						Truncated bool   `json:"truncated"`
					}
					_ = json.Unmarshal(message.Result, &response)
					writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{
						"type": "text", "text": fmt.Sprintf("%d:%t", len(response.Output), response.Truncated),
					}})
					write(map[string]any{"jsonrpc": "2.0", "id": terminalBase + 3, "method": "terminal/release", "params": map[string]any{
						"sessionId": "fake-session", "terminalId": terminalID,
					}})
				case fmt.Sprint(terminalBase + 3):
					writePromptResult(write, promptID, "end_turn")
				}
			case "terminal-persist":
				switch rpcID(message.ID) {
				case "200":
					var response struct {
						TerminalID string `json:"terminalId"`
					}
					_ = json.Unmarshal(message.Result, &response)
					terminalID = response.TerminalID
					writePromptResult(write, promptID, "end_turn")
				case "210":
					write(map[string]any{"jsonrpc": "2.0", "id": 211, "method": "terminal/output", "params": map[string]any{
						"sessionId": "fake-session", "terminalId": terminalID,
					}})
				case "211":
					var response struct {
						Output string `json:"output"`
					}
					_ = json.Unmarshal(message.Result, &response)
					writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": response.Output}})
					write(map[string]any{"jsonrpc": "2.0", "id": 212, "method": "terminal/release", "params": map[string]any{
						"sessionId": "fake-session", "terminalId": terminalID,
					}})
				case "212":
					writePromptResult(write, promptID, "end_turn")
				}
			case "terminal-cancel":
				var response struct {
					TerminalID string `json:"terminalId"`
				}
				_ = json.Unmarshal(message.Result, &response)
				if response.TerminalID != "" {
					terminalID = response.TerminalID
					writeUpdate(write, map[string]any{"sessionUpdate": "plan", "entries": []map[string]string{{"content": "started", "status": "in_progress"}}})
				}
			case "synthetic-ids":
				switch rpcID(message.ID) {
				case "200":
					write(map[string]any{"jsonrpc": "2.0", "id": 201, "method": "terminal/create", "params": map[string]any{
						"sessionId": "fake-session", "command": "printf", "args": []string{"id"},
					}})
				case "201":
					var response struct {
						TerminalID string `json:"terminalId"`
					}
					_ = json.Unmarshal(message.Result, &response)
					terminalID = response.TerminalID
					write(map[string]any{"jsonrpc": "2.0", "id": 202, "method": "terminal/release", "params": map[string]any{
						"sessionId": "fake-session", "terminalId": terminalID,
					}})
				case "202":
					writePromptResult(write, promptID, "end_turn")
				}
			case "file-read-outside", "file-write-ask", "file-write-readonly", "file-write-auto", "file-write-outside", "file-relative":
				if len(message.Result) != 0 {
					var response struct {
						Content string `json:"content"`
					}
					_ = json.Unmarshal(message.Result, &response)
					if response.Content != "" {
						writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": response.Content}})
					}
				}
				writePromptResult(write, promptID, "end_turn")
			}
		}
	}
}

func rawID(id json.RawMessage) any {
	var number int64
	if json.Unmarshal(id, &number) == nil {
		return number
	}
	var text string
	_ = json.Unmarshal(id, &text)
	return text
}

func writeUpdate(write func(any), update map[string]any) {
	write(map[string]any{"jsonrpc": "2.0", "method": "session/update", "params": map[string]any{"sessionId": "fake-session", "update": update}})
}

func writePermission(write func(any), id int, call, kind string) {
	write(map[string]any{"jsonrpc": "2.0", "id": id, "method": "session/request_permission", "params": map[string]any{
		"sessionId": "fake-session",
		"toolCall":  map[string]any{"toolCallId": call, "title": call, "kind": kind, "rawInput": map[string]string{"path": "/tmp/value"}},
		"options":   []map[string]string{{"optionId": "allow", "name": "Allow once", "kind": "allow_once"}, {"optionId": "reject", "name": "Reject", "kind": "reject_once"}},
	}})
}

func writePromptResult(write func(any), id json.RawMessage, stop string) {
	write(map[string]any{"jsonrpc": "2.0", "id": rawID(id), "result": map[string]any{"stopReason": stop}})
}

func fakeUpdates(write func(any)) {
	writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": "one"}})
	writeUpdate(write, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": "two"}})
	writeUpdate(write, map[string]any{"sessionUpdate": "agent_thought_chunk", "content": map[string]string{"type": "text", "text": "thinking"}})
	writeUpdate(write, map[string]any{"sessionUpdate": "tool_call", "toolCallId": "tool-1", "title": "Read", "kind": "read", "status": "in_progress", "rawInput": map[string]string{"path": "/tmp/file"}})
	writeUpdate(write, map[string]any{"sessionUpdate": "plan", "entries": []map[string]string{{"content": "Work", "status": "pending"}}})
	writeUpdate(write, map[string]any{"sessionUpdate": "available_commands_update", "availableCommands": []map[string]string{{"name": "help", "description": "Help"}}})
	writeUpdate(write, map[string]any{"sessionUpdate": "current_mode_update", "currentModeId": "code"})
	writeUpdate(write, map[string]any{"sessionUpdate": "config_option_update", "configOptions": []map[string]any{{"id": "model", "category": "model", "type": "select", "currentValue": "fast", "options": []map[string]string{{"value": "fast", "name": "Fast"}}}}})
	writeUpdate(write, map[string]any{"sessionUpdate": "session_info_update", "title": "Session title"})
	writeUpdate(write, map[string]any{"sessionUpdate": "usage_update", "used": 12, "size": 100})
}
