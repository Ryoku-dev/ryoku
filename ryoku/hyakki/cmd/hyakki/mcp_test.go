package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/client"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/mcpserve"
	"ryoku-rashin/internal/protocol"
)

// mcpPipe runs hyakki mcp on pipes against the real daemon and returns
// the agent's ends and the log.
func mcpPipe(t *testing.T) (in io.WriteCloser, out *bufio.Scanner, log *bytes.Buffer, done chan int) {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	log = &bytes.Buffer{}
	done = make(chan int, 1)
	go func() {
		done <- serveMCP(context.Background(), inR, outW, log, func(ctx context.Context, name string) (mcpserve.Daemon, error) {
			return client.Dial(ctx, client.Options{Client: protocol.ClientMCP, Name: name})
		})
		outW.Close()
	}()
	t.Cleanup(func() { inW.Close() })
	return inW, bufio.NewScanner(outR), log, done
}

func mcpRequest(t *testing.T, in io.Writer, out *bufio.Scanner, id int, method string, params any) map[string]json.RawMessage {
	t.Helper()
	line, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if _, err := in.Write(append(line, '\n')); err != nil {
		t.Fatal(err)
	}
	if !out.Scan() {
		t.Fatalf("no reply to %s", method)
	}
	var reply map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &reply); err != nil {
		t.Fatal(err)
	}
	return reply
}

func mcpSettings(t *testing.T, mcp protocol.OniMCPSettings) {
	t.Helper()
	settings, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	settings.Oni.MCP = mcp
	if err := config.Save(settings); err != nil {
		t.Fatal(err)
	}
}

var mcpInit = map[string]any{"protocolVersion": "2025-06-18", "clientInfo": map[string]string{"name": "agent"}, "capabilities": map[string]any{}}

// Off by default: hyakki mcp's initialize is refused by the daemon, and the
// command ends cleanly when the program closes its input.
func TestMcpCommandIsRefusedWhileTheBridgeIsOff(t *testing.T) {
	realOniDaemon(t, func() {})
	in, out, log, done := mcpPipe(t)
	reply := mcpRequest(t, in, out, 1, "initialize", mcpInit)
	if reply["error"] == nil || !strings.Contains(string(reply["error"]), "off") {
		t.Fatalf("initialize while off = %s log %q", out.Bytes(), log.String())
	}
	in.Close()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit %d", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("hyakki mcp did not end with its input")
	}
	if !strings.Contains(log.String(), "hyakki mcp: initialize refused forbidden") {
		t.Fatalf("log = %q", log.String())
	}
}

// On and approved, the real daemon serves exactly the tools turned on; an
// unapproved program sees none, and every refusal leaves only codes in the
// log.
func TestMcpCommandServesTheApprovedProgramOverAPipe(t *testing.T) {
	realOniDaemon(t, func() {
		mcpSettings(t, protocol.OniMCPSettings{Enabled: true, Tools: map[string]bool{protocol.OniMCPNotify: true, protocol.OniMCPFocusStatus: true}, Clients: []string{"agent"}})
	})
	in, out, log, _ := mcpPipe(t)
	if reply := mcpRequest(t, in, out, 1, "initialize", mcpInit); reply["error"] != nil {
		t.Fatalf("initialize = %s", out.Bytes())
	}
	reply := mcpRequest(t, in, out, 2, "tools/list", map[string]any{})
	var list struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(reply["result"], &list); err != nil || len(list.Tools) != 2 || list.Tools[0].Name != protocol.OniMCPFocusStatus || list.Tools[1].Name != protocol.OniMCPNotify {
		t.Fatalf("tools = %s", out.Bytes())
	}
	// Oni is off here: the daemon refuses the notice, and says so
	reply = mcpRequest(t, in, out, 3, "tools/call", map[string]any{"name": protocol.OniMCPNotify, "arguments": map[string]any{"text": "Build finished."}})
	if !strings.Contains(string(reply["result"]), `"isError":true`) {
		t.Fatalf("notify with Oni off = %s", out.Bytes())
	}
	reply = mcpRequest(t, in, out, 4, "tools/call", map[string]any{"name": protocol.OniMCPScreenAsk, "arguments": map[string]any{"question": "What is this?"}})
	if reply["error"] == nil {
		t.Fatalf("a tool turned off = %s", out.Bytes())
	}
	if strings.Contains(log.String(), "Build finished") || strings.Contains(log.String(), "What is this") {
		t.Fatalf("the log holds text: %q", log.String())
	}

	stranger, strangerOut, _, _ := mcpPipe(t)
	init := map[string]any{"protocolVersion": "2025-06-18", "clientInfo": map[string]string{"name": "stranger"}}
	if reply := mcpRequest(t, stranger, strangerOut, 1, "initialize", init); reply["error"] != nil {
		t.Fatalf("initialize = %s", strangerOut.Bytes())
	}
	reply = mcpRequest(t, stranger, strangerOut, 2, "tools/list", map[string]any{})
	if string(reply["result"]) != `{"tools":[]}` {
		t.Fatalf("an unapproved program sees %s", reply["result"])
	}
}
