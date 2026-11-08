package main

import (
	"encoding/json"
	"testing"
	"time"
)

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatal(what)
}

func TestIdleAgentRetiresAndResumesItsSession(t *testing.T) {
	quickTestEnv(t)
	prevIdle, prevStart := agentIdleTimeout, startLaneACP
	agentIdleTimeout = 20 * time.Millisecond
	t.Cleanup(func() { agentIdleTimeout, startLaneACP = prevIdle, prevStart })

	h := newChatHub(ryokuLane())
	first, _ := newTestPair(t)
	first.mu.Lock()
	first.sessionID = "s1"
	first.sessionReady = true
	first.mu.Unlock()
	h.mu.Lock()
	h.conn = first
	h.transcript = []wsOut{{Type: "user_text", Text: "hi"}}
	h.last = wsOut{Type: "state", State: "busy"}
	h.armIdleLocked()
	h.mu.Unlock()

	time.Sleep(100 * time.Millisecond)
	h.mu.Lock()
	kept := h.conn == first
	h.last = wsOut{Type: "state", State: "ready"}
	h.armIdleLocked()
	h.mu.Unlock()
	if !kept {
		t.Fatal("an agent in the middle of a turn was retired")
	}

	waitFor(t, "an idle agent with no surface was not retired", func() bool {
		h.mu.Lock()
		defer h.mu.Unlock()
		return h.conn == nil
	})

	second, agent := newTestPair(t)
	startLaneACP = func(string) (*acpConn, error) { return second, nil }
	h.mu.Lock()
	h.ensureConnLocked()
	h.mu.Unlock()

	initialize := agent.read()
	if initialize.Method != "initialize" {
		t.Fatalf("method = %q, want initialize", initialize.Method)
	}
	agent.respond(*initialize.ID, map[string]any{"protocolVersion": 1, "agentCapabilities": map[string]any{"loadSession": true}})
	load := agent.read()
	var params struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.Unmarshal(load.Params, &params)
	if load.Method != "session/load" || params.SessionID != "s1" {
		t.Fatalf("restart sent %s %q, want session/load of the retired session s1", load.Method, params.SessionID)
	}
	agent.respond(*load.ID, map[string]any{})
	waitFor(t, "the resumed session never became ready", func() bool {
		second.mu.Lock()
		defer second.mu.Unlock()
		return second.sessionReady && second.sessionID == "s1"
	})
	h.resetConn()
}

func TestSurfaceKeepsAgentAlive(t *testing.T) {
	quickTestEnv(t)
	prevIdle := agentIdleTimeout
	agentIdleTimeout = 20 * time.Millisecond
	t.Cleanup(func() { agentIdleTimeout = prevIdle })

	h := newChatHub(ryokuLane())
	conn, _ := newTestPair(t)
	h.mu.Lock()
	h.conn = conn
	h.clients[&chatClient{}] = true
	h.armIdleLocked()
	h.mu.Unlock()
	time.Sleep(100 * time.Millisecond)
	h.mu.Lock()
	alive := h.conn == conn
	h.mu.Unlock()
	if !alive {
		t.Fatal("the agent was retired while a surface still showed it")
	}
	h.resetConn()
}
