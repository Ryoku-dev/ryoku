package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func serve(t *testing.T, server *Server, lines ...string) []map[string]any {
	t.Helper()
	reader, writer := io.Pipe()
	var out strings.Builder
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background(), reader, &out) }()
	for _, line := range lines {
		if _, err := io.WriteString(writer, line+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	writer.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	var replies []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var value map[string]any
		if err := json.Unmarshal([]byte(line), &value); err != nil {
			t.Fatalf("reply %q: %v", line, err)
		}
		replies = append(replies, value)
	}
	return replies
}

func byID(replies []map[string]any, id float64) map[string]any {
	for _, value := range replies {
		if value["id"] == id {
			return value
		}
	}
	return nil
}

func TestHandshakeListAndCall(t *testing.T) {
	server := &Server{Name: "rashin", Version: "1", Tools: []Tool{
		{Name: "echo", Description: "Says it back.", ReadOnly: true, Input: map[string]any{"type": "object"}, Call: func(_ context.Context, arguments json.RawMessage) Result {
			var args struct{ Text string }
			_ = json.Unmarshal(arguments, &args)
			if args.Text == "" {
				return Failure(errors.New("nothing to say"))
			}
			return Text(args.Text)
		}},
	}}
	replies := serve(t, server,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo","arguments":{"text":"hi"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"echo"}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"missing"}}`,
		`{"jsonrpc":"2.0","id":6,"method":"resources/list"}`,
		`not json`,
	)
	if len(replies) != 7 {
		t.Fatalf("replies = %v", replies)
	}
	if version := byID(replies, 1)["result"].(map[string]any)["protocolVersion"]; version != "2025-03-26" {
		t.Fatalf("negotiated %v", version)
	}
	tools := byID(replies, 2)["result"].(map[string]any)["tools"].([]any)
	if tool := tools[0].(map[string]any); tool["name"] != "echo" || tool["annotations"].(map[string]any)["readOnlyHint"] != true {
		t.Fatalf("tools = %v", tools)
	}
	said := byID(replies, 3)["result"].(map[string]any)
	if said["content"].([]any)[0].(map[string]any)["text"] != "hi" || said["isError"] != nil {
		t.Fatalf("call = %v", said)
	}
	if failed := byID(replies, 4)["result"].(map[string]any); failed["isError"] != true {
		t.Fatalf("a failed call reads %v", failed)
	}
	if byID(replies, 5)["error"] == nil || byID(replies, 6)["error"] == nil {
		t.Fatal("an unknown tool or method is not a protocol error")
	}
}

func TestUnknownVersionGetsTheNewestAndCancelledCallsStaySilent(t *testing.T) {
	started := make(chan struct{})
	server := &Server{Name: "rashin", Tools: []Tool{{Name: "wait", Input: map[string]any{"type": "object"}, Call: func(ctx context.Context, _ json.RawMessage) Result {
		close(started)
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
		return Text("late")
	}}}}
	reader, writer := io.Pipe()
	var out strings.Builder
	done := make(chan error, 1)
	go func() { done <- server.Serve(context.Background(), reader, &out) }()
	io.WriteString(writer, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"1999-01-01"}}`+"\n")
	io.WriteString(writer, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"wait"}}`+"\n")
	<-started
	io.WriteString(writer, `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":2}}`+"\n")
	time.Sleep(50 * time.Millisecond)
	writer.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "late") {
		t.Fatalf("a cancelled call still answered: %s", out.String())
	}
	if !strings.Contains(out.String(), Versions[0]) {
		t.Fatalf("an unknown version was not answered with the newest: %s", out.String())
	}
}
