package mcpserve

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

// fakeDaemon answers the calls the bridge makes as the daemon would, and
// records each one.
type fakeDaemon struct {
	mu     sync.Mutex
	status protocol.OniMCPStatus
	calls  []string
	params []json.RawMessage
	answer func(method string, params json.RawMessage) (any, error)
	done   chan struct{}
	closed bool
}

func (f *fakeDaemon) Call(ctx context.Context, method string, params, result any) error {
	raw, _ := json.Marshal(params)
	f.mu.Lock()
	f.calls = append(f.calls, method)
	f.params = append(f.params, raw)
	status := f.status
	answer := f.answer
	f.mu.Unlock()
	var out any
	var err error
	if method == protocol.MOniMCP {
		out = status
	} else if answer != nil {
		out, err = answer(method, raw)
	}
	if err != nil || result == nil {
		return err
	}
	b, _ := json.Marshal(out)
	return json.Unmarshal(b, result)
}

func (f *fakeDaemon) Done() <-chan struct{} { return f.done }

func (f *fakeDaemon) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.closed {
		f.closed = true
		close(f.done)
	}
	return nil
}

func (f *fakeDaemon) sent() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.DeleteFunc(slices.Clone(f.calls), func(m string) bool { return m == protocol.MOniMCP })
}

// agent is the program's end of the pipes.
type agent struct {
	t      *testing.T
	in     io.WriteCloser
	out    *bufio.Scanner
	nextID int
	served chan error
	logs   *[]string
}

func serve(t *testing.T, dial Dial) *agent {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	var mu sync.Mutex
	logs := []string{}
	a := &agent{t: t, in: inW, out: bufio.NewScanner(outR), served: make(chan error, 1), logs: &logs}
	a.out.Buffer(make([]byte, 0, 1<<20), 1<<20)
	go func() {
		a.served <- Serve(context.Background(), inR, outW, Options{Dial: dial, Version: "test", Log: func(code string) {
			mu.Lock()
			logs = append(logs, code)
			mu.Unlock()
		}})
		outW.Close()
	}()
	t.Cleanup(func() { inW.Close() })
	return a
}

type rpcReply struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (a *agent) call(method string, params any) rpcReply {
	a.t.Helper()
	a.nextID++
	line, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": a.nextID, "method": method, "params": params})
	if _, err := a.in.Write(append(line, '\n')); err != nil {
		a.t.Fatal(err)
	}
	if !a.out.Scan() {
		a.t.Fatalf("no reply to %s: %v", method, a.out.Err())
	}
	var r rpcReply
	if err := json.Unmarshal(a.out.Bytes(), &r); err != nil {
		a.t.Fatal(err)
	}
	return r
}

func (a *agent) initialize(name string) rpcReply {
	return a.call("initialize", map[string]any{"protocolVersion": "2025-06-18", "clientInfo": map[string]string{"name": name}, "capabilities": map[string]any{}})
}

func (a *agent) tools() []string {
	a.t.Helper()
	r := a.call("tools/list", map[string]any{})
	if r.Error != nil {
		a.t.Fatalf("tools/list: %s", r.Error.Message)
	}
	var list struct {
		Tools []struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(r.Result, &list); err != nil {
		a.t.Fatal(err)
	}
	var names []string
	for _, tool := range list.Tools {
		if tool.Description == "" || tool.InputSchema["type"] != "object" {
			a.t.Fatalf("tool %s is not described: %+v", tool.Name, tool)
		}
		names = append(names, tool.Name)
	}
	return names
}

type toolOutcome struct {
	text    string
	isError bool
	rpc     string
}

func (a *agent) tool(name string, args any) toolOutcome {
	a.t.Helper()
	r := a.call("tools/call", map[string]any{"name": name, "arguments": args})
	if r.Error != nil {
		return toolOutcome{rpc: r.Error.Message}
	}
	var res struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(r.Result, &res); err != nil || len(res.Content) != 1 || res.Content[0].Type != "text" {
		a.t.Fatalf("tools/call %s: %s", name, r.Result)
	}
	return toolOutcome{text: res.Content[0].Text, isError: res.IsError}
}

func (a *agent) close() error {
	a.t.Helper()
	a.in.Close()
	select {
	case err := <-a.served:
		return err
	case <-time.After(5 * time.Second):
		a.t.Fatal("the server did not end when its input closed")
	}
	return nil
}

func approvedDaemon(tools ...string) *fakeDaemon {
	return &fakeDaemon{done: make(chan struct{}), status: protocol.OniMCPStatus{Enabled: true, Tools: tools,
		Clients: []protocol.OniMCPClient{{Name: "agent", State: protocol.OniMCPClientApproved, Connected: true}}}}
}

func dialTo(d *fakeDaemon, names *[]string) Dial {
	return func(ctx context.Context, name string) (Daemon, error) {
		if names != nil {
			*names = append(*names, name)
		}
		return d, nil
	}
}

func TestMcpHandshakeListsExactlyTheToolsTurnedOn(t *testing.T) {
	d := approvedDaemon(protocol.OniMCPNotify, protocol.OniMCPRemindersList)
	var names []string
	a := serve(t, dialTo(d, &names))
	if r := a.call("tools/list", map[string]any{}); r.Error == nil {
		t.Fatal("tools listed before initialize")
	}
	r := a.initialize("agent")
	if r.Error != nil {
		t.Fatal(r.Error.Message)
	}
	var init struct {
		ProtocolVersion string            `json:"protocolVersion"`
		ServerInfo      map[string]string `json:"serverInfo"`
		Capabilities    map[string]any    `json:"capabilities"`
	}
	if err := json.Unmarshal(r.Result, &init); err != nil || init.ProtocolVersion != "2025-06-18" || init.ServerInfo["name"] != "hyakki" || init.Capabilities["tools"] == nil {
		t.Fatalf("initialize = %s", r.Result)
	}
	if !slices.Equal(names, []string{"agent"}) {
		t.Fatalf("dialled as %v", names)
	}
	if r := a.call("ping", nil); r.Error != nil {
		t.Fatal(r.Error.Message)
	}
	if got := a.tools(); !slices.Equal(got, []string{protocol.OniMCPRemindersList, protocol.OniMCPNotify}) {
		t.Fatalf("tools = %v", got)
	}
	if r := a.call("resources/list", map[string]any{}); r.Error == nil || r.Error.Code != -32601 {
		t.Fatalf("an unknown method = %+v", r.Error)
	}
	if out := a.tool("tasks_add", map[string]any{}); out.rpc == "" {
		t.Fatal("an unknown tool was called")
	}
	if out := a.tool(protocol.OniMCPNotesAdd, map[string]any{"text": "x"}); out.rpc == "" {
		t.Fatal("a tool turned off was called")
	}
	if len(d.sent()) != 0 {
		t.Fatalf("refused calls reached the daemon: %v", d.sent())
	}
	if err := a.close(); err != nil {
		t.Fatalf("served = %v", err)
	}
	if !d.closed {
		t.Fatal("the daemon connection outlived the server")
	}
}

func TestMcpRefusesWhenOffAndWhenUnapproved(t *testing.T) {
	off := serve(t, func(ctx context.Context, name string) (Daemon, error) {
		return nil, &protocol.Error{Code: protocol.CodeForbidden, Message: "The MCP bridge is off."}
	})
	r := off.initialize("agent")
	if r.Error == nil || r.Error.Message != "The MCP bridge is off." {
		t.Fatalf("initialize while off = %+v", r)
	}
	if r := off.call("tools/list", map[string]any{}); r.Error == nil {
		t.Fatal("tools listed while off")
	}
	if r := off.initialize("bad\nname"); r.Error == nil {
		t.Fatal("a name with a newline was taken")
	}

	d := approvedDaemon(protocol.OniMCPNotify)
	d.status.Clients = []protocol.OniMCPClient{{Name: "stranger", State: protocol.OniMCPClientWaiting, Connected: true}}
	waiting := serve(t, dialTo(d, nil))
	if r := waiting.initialize("stranger"); r.Error != nil {
		t.Fatal(r.Error.Message)
	}
	if got := waiting.tools(); len(got) != 0 {
		t.Fatalf("an unapproved program sees %v", got)
	}
	if out := waiting.tool(protocol.OniMCPNotify, map[string]any{"text": "hello"}); out.rpc == "" {
		t.Fatal("an unapproved program called a tool")
	}
	if len(d.sent()) != 0 {
		t.Fatalf("an unapproved program reached %v", d.sent())
	}
}

func TestMcpNotesBecomeSuggestionsAndSecretsNeverLeave(t *testing.T) {
	d := approvedDaemon(protocol.OniMCPNotesAdd, protocol.OniMCPNotesSearch)
	d.answer = func(method string, raw json.RawMessage) (any, error) {
		switch method {
		case protocol.MMemoryImport:
			return protocol.MemoryChange{Items: []protocol.MemoryItem{{ID: "m1", Op: protocol.MemoryOpSuggested}}}, nil
		case protocol.MMemorySearch:
			return protocol.MemorySearchResult{Memories: []protocol.Memory{{Text: "Prefers tea."}, {Text: "a private matter", Sensitive: true}}}, nil
		}
		return nil, errors.New("unexpected " + method)
	}
	a := serve(t, dialTo(d, nil))
	a.initialize("agent")
	out := a.tool(protocol.OniMCPNotesAdd, map[string]any{"text": "Likes short answers."})
	if out.isError || !strings.Contains(out.text, "Inbox") {
		t.Fatalf("notes_add = %+v", out)
	}
	var p protocol.MemoryImportParams
	if err := json.Unmarshal(d.params[len(d.params)-1], &p); err != nil || p.Text != "Likes short answers." || p.Scope != protocol.MemoryYou {
		t.Fatalf("sent %s", d.params[len(d.params)-1])
	}
	for _, secret := range []string{"My card is 4111 1111 1111 1111", "my password is hunter2"} {
		if out := a.tool(protocol.OniMCPNotesAdd, map[string]any{"text": secret}); !out.isError {
			t.Fatalf("%q was taken: %+v", secret, out)
		}
	}
	if got := d.sent(); !slices.Equal(got, []string{protocol.MMemoryImport}) {
		t.Fatalf("calls = %v", got)
	}
	if out := a.tool(protocol.OniMCPNotesAdd, map[string]any{"text": "ok", "scope": "team:x"}); !out.isError {
		t.Fatal("an unknown argument was taken")
	}
	out = a.tool(protocol.OniMCPNotesSearch, map[string]any{"query": "drinks"})
	if out.isError || !strings.Contains(out.text, "Prefers tea.") || strings.Contains(out.text, "private matter") {
		t.Fatalf("notes_search = %+v", out)
	}
	// the log holds codes, never the words
	for _, line := range *a.logs {
		if strings.Contains(line, "short answers") || strings.Contains(line, "4111") || strings.Contains(line, "tea") {
			t.Fatalf("the log holds text: %q", line)
		}
	}
}

func TestMcpScreenAskReturnsWordsOnlyAndNothingOnANo(t *testing.T) {
	d := approvedDaemon(protocol.OniMCPScreenAsk)
	answer := protocol.OniAskResult{Status: protocol.OniAskDeclined}
	d.answer = func(method string, raw json.RawMessage) (any, error) {
		if method != protocol.MOniAsk {
			return nil, errors.New("unexpected " + method)
		}
		return answer, nil
	}
	a := serve(t, dialTo(d, nil))
	a.initialize("agent")
	if out := a.tool(protocol.OniMCPScreenAsk, map[string]any{"question": "What is this?"}); !out.isError || !strings.Contains(out.text, "Nothing was sent") {
		t.Fatalf("after a no = %+v", out)
	}
	answer = protocol.OniAskResult{Status: protocol.OniAskAnswered, Answer: &protocol.OniAnswer{Text: "A text editor.", Route: "fixture/eyes"}}
	if out := a.tool(protocol.OniMCPScreenAsk, map[string]any{"question": "What is this?"}); out.isError || out.text != "A text editor." {
		t.Fatalf("after a yes = %+v", out)
	}
	// a refusal from the daemon is the tool's error, in its words
	d.mu.Lock()
	d.answer = func(string, json.RawMessage) (any, error) {
		return nil, &protocol.Error{Code: protocol.CodeConflict, Message: "Perch Oni on a window first."}
	}
	d.mu.Unlock()
	if out := a.tool(protocol.OniMCPScreenAsk, map[string]any{"question": "What is this?"}); !out.isError || out.text != "Perch Oni on a window first." {
		t.Fatalf("with no perch = %+v", out)
	}
}

func TestMcpRemindersAddIsBoundedBeforeTheDaemon(t *testing.T) {
	d := approvedDaemon(protocol.OniMCPRemindersAdd)
	d.answer = func(method string, raw json.RawMessage) (any, error) { return protocol.OniRemindersResult{}, nil }
	a := serve(t, dialTo(d, nil))
	a.initialize("agent")
	due := time.Now().Add(time.Hour).Format(time.RFC3339)
	for _, bad := range []map[string]any{
		{"due": due, "text": strings.Repeat("a", TextMax+1)},
		{"due": "tomorrow", "text": "call"},
		{"due": due},
		{"due": due, "text": "pay with 4111 1111 1111 1111"},
		{"due": due, "text": "x", "kind": "alarm"},
	} {
		if out := a.tool(protocol.OniMCPRemindersAdd, bad); !out.isError {
			t.Fatalf("%v was taken", bad)
		}
	}
	if len(d.sent()) != 0 {
		t.Fatalf("bad arguments reached the daemon: %v", d.sent())
	}
	if out := a.tool(protocol.OniMCPRemindersAdd, map[string]any{"due": due, "text": "Call the bank"}); out.isError {
		t.Fatalf("a good reminder = %+v", out)
	}
	var p protocol.OniRemindersParams
	if err := json.Unmarshal(d.params[len(d.params)-1], &p); err != nil || p.Action != protocol.OniRemindersAdd || p.Kind != protocol.OniReminderKindReminder || p.Text != "Call the bank" {
		t.Fatalf("sent %s", d.params[len(d.params)-1])
	}
}

func TestMcpBoundsArgumentsAndConcurrentCalls(t *testing.T) {
	d := approvedDaemon(protocol.OniMCPNotify)
	a := serve(t, dialTo(d, nil))
	a.initialize("agent")
	if out := a.tool(protocol.OniMCPNotify, map[string]any{"text": strings.Repeat("a", ArgsLimit)}); out.rpc == "" {
		t.Fatal("oversized arguments were taken")
	}
	if out := a.tool(protocol.OniMCPNotify, map[string]any{"text": "see https://example.com"}); !out.isError {
		t.Fatal("a link was taken")
	}
	// only InFlight calls at once; the rest are refused, not queued
	release := make(chan struct{})
	d.mu.Lock()
	d.answer = func(string, json.RawMessage) (any, error) { <-release; return protocol.Empty{}, nil }
	d.mu.Unlock()
	for id := range InFlight + 2 {
		line, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 100 + id, "method": "tools/call", "params": map[string]any{"name": protocol.OniMCPNotify, "arguments": map[string]any{"text": "hi " + strconv.Itoa(id)}}})
		if _, err := a.in.Write(append(line, '\n')); err != nil {
			t.Fatal(err)
		}
	}
	refused := 0
	for range 2 {
		if !a.out.Scan() {
			t.Fatal("no reply")
		}
		if strings.Contains(a.out.Text(), `"error"`) {
			refused++
		}
	}
	close(release)
	for range InFlight {
		a.out.Scan()
	}
	if refused != 2 {
		t.Fatalf("%d of the extra calls refused", refused)
	}
}

func TestMcpRedialsAfterTheDaemonStopped(t *testing.T) {
	first := approvedDaemon(protocol.OniMCPFocusStatus)
	second := approvedDaemon(protocol.OniMCPFocusStatus)
	second.answer = func(string, json.RawMessage) (any, error) {
		return protocol.OniFocusResult{Armed: true, LeftMs: 90_000}, nil
	}
	daemons := []*fakeDaemon{first, second}
	var names []string
	a := serve(t, func(ctx context.Context, name string) (Daemon, error) {
		names = append(names, name)
		d := daemons[0]
		daemons = daemons[1:]
		return d, nil
	})
	a.initialize("agent")
	first.Close()
	if out := a.tool(protocol.OniMCPFocusStatus, map[string]any{}); out.isError || !strings.Contains(out.text, "2 more minutes") {
		t.Fatalf("focus_status = %+v", out)
	}
	if !slices.Equal(names, []string{"agent", "agent"}) {
		t.Fatalf("dialled %v", names)
	}
}

// The bridge reaches the daemon only through the wire: it imports the
// protocol, the pipe, the sensitive judge, i18n and the standard library.
func TestMcpServeImportsOnlyTheWire(t *testing.T) {
	allowed := []string{"ryoku-rashin/internal/protocol", "ryoku-rashin/internal/rpcpipe", "ryoku-rashin/internal/sensitive", "ryoku-i18n"}
	files, _ := filepath.Glob("*.go")
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(token.NewFileSet(), name, src, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if strings.Contains(path, ".") || strings.Contains(path, "ryoku") {
				if !slices.Contains(allowed, path) {
					t.Errorf("%s imports %s", name, path)
				}
			}
		}
	}
}
