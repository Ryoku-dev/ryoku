package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func openFirstSession(t *testing.T, conn *acpConn, fa *fakeAgent) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- conn.Initialize("/tmp/vault") }()
	init := fa.read()
	fa.respond(*init.ID, map[string]any{"protocolVersion": 1})
	first := fa.read()
	fa.respond(*first.ID, map[string]any{"sessionId": "s1"})
	if err := <-done; err != nil {
		t.Fatalf("Initialize: %v", err)
	}
}

func openNextSession(t *testing.T, conn *acpConn, fa *fakeAgent, id string) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- conn.NewSession() }()
	req := fa.read()
	if req.Method != "session/new" {
		t.Fatalf("got %q, want session/new", req.Method)
	}
	fa.respond(*req.ID, map[string]any{"sessionId": id})
	if err := <-done; err != nil {
		t.Fatalf("NewSession: %v", err)
	}
}

func TestNewChatDoesNotReceiveTheOldTurn(t *testing.T) {
	conn, fa := newTestPair(t)
	defer conn.Close()
	h := newChatHub(lane{Name: laneRyoku})
	client := &chatClient{out: make(chan wsOut, 64)}
	h.clients[client] = true
	h.conn = conn
	go h.pump(conn)

	openFirstSession(t, conn, fa)
	conn.Prompt("slow question", nil)
	prompt := fa.read()
	fa.update("s1", map[string]any{
		"sessionUpdate": "agent_thought_chunk",
		"content":       map[string]string{"type": "text", "text": "first chat thinking"},
	})
	for seen := false; !seen; {
		select {
		case m := <-client.out:
			seen = m.Type == "agent_thought"
		case <-time.After(3 * time.Second):
			t.Fatal("the first chat never streamed its thinking")
		}
	}
	h.mu.Lock()
	h.resetConversationLocked()
	h.mu.Unlock()
	openNextSession(t, conn, fa, "s2")

	fa.update("s1", map[string]any{
		"sessionUpdate": "agent_message_chunk",
		"content":       map[string]string{"type": "text", "text": "answer for the first chat"},
	})
	fa.write(rpcMsg{
		JSONRPC: "2.0", ID: new(int64), Method: "session/request_permission",
		Params: json.RawMessage(`{"sessionId":"s1","toolCall":{"title":"rm"},"options":[{"optionId":"allow","name":"Allow","kind":"allow_once"}]}`),
	})
	cancelled := fa.read()
	if cancelled.ID == nil || string(cancelled.Result) == "" || !strings.Contains(string(cancelled.Result), "cancelled") {
		t.Fatalf("stale permission was not cancelled: %+v", cancelled)
	}
	fa.respond(*prompt.ID, map[string]any{"stopReason": "end_turn"})
	fa.update("s2", map[string]any{
		"sessionUpdate": "agent_message_chunk",
		"content":       map[string]string{"type": "text", "text": "fresh chat answer"},
	})

	deadline := time.After(3 * time.Second)
	for got := false; !got; {
		select {
		case m := <-client.out:
			switch {
			case m.Type == "agent_text" && m.Text == "fresh chat answer":
				got = true
			case m.Type == "agent_thought", m.Type == "agent_text", m.Type == "turn_end", m.Type == "permission":
				t.Fatalf("frame from the abandoned chat reached the new one: %+v", m)
			}
		case <-deadline:
			t.Fatal("the new chat never received its own answer")
		}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, m := range h.transcript {
		if m.Text != "fresh chat answer" {
			t.Fatalf("transcript holds a frame from the old chat: %+v", m)
		}
	}
}

func TestLoadedChatKeepsItsOwnReplay(t *testing.T) {
	conn, fa := newTestPair(t)
	defer conn.Close()
	openFirstSession(t, conn, fa)
	conn.loadSession = true
	openNextSession(t, conn, fa, "s2")

	done := make(chan error, 1)
	go func() { done <- conn.LoadSession("s1") }()
	req := fa.read()
	if req.Method != "session/load" {
		t.Fatalf("got %q, want session/load", req.Method)
	}
	fa.update("s2", map[string]any{
		"sessionUpdate": "agent_message_chunk",
		"content":       map[string]string{"type": "text", "text": "late text from the chat we left"},
	})
	fa.update("s1", map[string]any{
		"sessionUpdate": "user_message_chunk",
		"content":       map[string]string{"type": "text", "text": "replayed question"},
	})
	fa.respond(*req.ID, map[string]any{"sessionId": "s1"})
	if err := <-done; err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	var texts []string
	for ev := range conn.Events() {
		if ev.Type == "agent_text" {
			t.Fatalf("late frame from the left chat leaked into the replay: %+v", ev)
		}
		if ev.Type == "user_text" {
			texts = append(texts, ev.Text)
		}
		if ev.Type == "replay_end" {
			break
		}
	}
	if len(texts) != 1 || texts[0] != "replayed question" {
		t.Fatalf("replayed user text = %v", texts)
	}
}
