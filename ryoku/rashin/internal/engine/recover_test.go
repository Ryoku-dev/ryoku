package engine

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

func TestEngineRetriesEmptyAnswerWithNextStep(t *testing.T) {
	g := &scriptedGateway{replies: []scriptedReply{scriptedTurn(llm.FinishStop, ""), scriptedTurn(llm.FinishStop, "Done.")}}
	r := engineFixture(t, g)
	result := invokeTurn(t, r, &engineSink{})
	if result.Stop != protocol.StopEnd || len(g.requests) != 2 {
		t.Fatalf("stop=%s calls=%d", result.Stop, len(g.requests))
	}
	found := false
	for _, m := range g.requests[1].Messages {
		if strings.Contains(m.Content, "last reply was empty") {
			found = true
		}
	}
	if !found {
		t.Fatal("empty reply was not nudged")
	}
}

func TestEngineEmptyAnswerFailsAfterThreeNudges(t *testing.T) {
	g := &scriptedGateway{replies: []scriptedReply{scriptedTurn(llm.FinishStop, ""), scriptedTurn(llm.FinishStop, ""), scriptedTurn(llm.FinishStop, ""), scriptedTurn(llm.FinishStop, "")}}
	r := engineFixture(t, g)
	result, err := r.Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "Finish this task."}, &engineSink{})
	if err == nil || result.Stop != protocol.StopError || len(g.requests) != 4 {
		t.Fatalf("stop=%s calls=%d err=%v", result.Stop, len(g.requests), err)
	}
}

func TestEngineReadCallsOverlapButWritesWait(t *testing.T) {
	g := &scriptedGateway{replies: []scriptedReply{scriptedTurn(llm.FinishToolCalls, "", llm.ToolCall{ID: "read1", Name: tool.Read, Arguments: `{"path":"a"}`}, llm.ToolCall{ID: "read2", Name: tool.Grep, Arguments: `{"pattern":"a"}`}, llm.ToolCall{ID: "write", Name: tool.Write, Arguments: `{"path":"a"}`}), scriptedTurn(llm.FinishStop, "Done.")}}
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var mu sync.Mutex
	completed := 0
	read := func(_ context.Context, _ tool.Env) tool.Result {
		started <- struct{}{}
		<-release
		mu.Lock()
		completed++
		mu.Unlock()
		return tool.Result{Text: "read"}
	}
	write := func(_ context.Context, _ tool.Env) tool.Result {
		mu.Lock()
		n := completed
		mu.Unlock()
		if n != 2 {
			return tool.Failed("write ran before both reads")
		}
		return tool.Result{Text: "written"}
	}
	r := engineFixture(t, g, parallelScriptedTool{scriptedTool{name: tool.Read, readonly: true, run: read}}, parallelScriptedTool{scriptedTool{name: tool.Grep, readonly: true, run: read}}, scriptedTool{name: tool.Write, run: write})
	done := make(chan error, 1)
	go func() { _, err := r.Prompt(context.Background(), structPrompt(), &engineSink{}); done <- err }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("first read did not run")
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("second read did not overlap first")
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("turn did not finish")
	}
	for _, m := range g.requests[1].Messages {
		if m.Role == llm.RoleTool && m.Name == tool.Write && m.IsError {
			t.Fatalf("write was premature: %s", m.Content)
		}
	}
}

func structPrompt() core.PromptInput {
	return core.PromptInput{Turn: 1, Text: "Read twice, then write."}
}

func TestEngineSteerJoinsNextRound(t *testing.T) {
	g := &scriptedGateway{replies: []scriptedReply{scriptedTurn(llm.FinishToolCalls, "", llm.ToolCall{ID: "read", Name: tool.Read, Arguments: "{}"}), scriptedTurn(llm.FinishStop, "Done.")}}
	started, release := make(chan struct{}), make(chan struct{})
	r := engineFixture(t, g, scriptedTool{name: tool.Read, readonly: true, run: func(_ context.Context, _ tool.Env) tool.Result {
		close(started)
		<-release
		return tool.Result{Text: "read"}
	}})
	done := make(chan error, 1)
	go func() { _, err := r.Prompt(context.Background(), structPrompt(), &engineSink{}); done <- err }()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("tool did not begin")
	}
	if !r.Steer(core.PromptInput{Text: "Also check the second file."}) {
		t.Fatal("running turn did not accept steering")
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("turn did not finish")
	}
	if r.Steer(core.PromptInput{Text: "Afterwards."}) {
		t.Fatal("completed turn accepted steering")
	}
	found := false
	for _, m := range g.requests[1].Messages {
		if strings.Contains(m.Content, "Also check the second file.") {
			found = true
		}
	}
	if !found {
		t.Fatal("steering was not injected before the next model call")
	}
}

func TestEngineTokenCeilingStopsBeforeRequest(t *testing.T) {
	g := &scriptedGateway{replies: []scriptedReply{scriptedTurn(llm.FinishStop, "Unexpected.")}}
	r := engineFixture(t, g)
	r.ceiling.Tokens = 1
	result := invokeTurn(t, r, &engineSink{})
	if result.Stop != protocol.StopMaxRounds || len(g.requests) != 0 {
		t.Fatalf("stop=%s calls=%d", result.Stop, len(g.requests))
	}
}

func TestEngineContinuesAfterPartialStreamStall(t *testing.T) {
	g := &scriptedGateway{replies: []scriptedReply{{deltas: []llm.Delta{{Text: "I was checking"}}, fail: errors.New("connection reset")}, scriptedTurn(llm.FinishStop, "Done.")}}
	r := engineFixture(t, g)
	result := invokeTurn(t, r, &engineSink{})
	if result.Stop != protocol.StopEnd || len(g.requests) != 2 {
		t.Fatalf("stop=%s calls=%d", result.Stop, len(g.requests))
	}
	found := false
	for _, m := range g.requests[1].Messages {
		if strings.Contains(m.Content, "connection error") {
			found = true
		}
	}
	if !found {
		t.Fatal("partial answer did not get continuation note")
	}
}

func TestEngineMalformedCallIsNotExecuted(t *testing.T) {
	g := &scriptedGateway{replies: []scriptedReply{scriptedTurn(llm.FinishToolCalls, "", llm.ToolCall{ID: "bad", Name: tool.Write, Arguments: `{"path":`}), scriptedTurn(llm.FinishStop, "Done.")}}
	ran := 0
	r := engineFixture(t, g, scriptedTool{name: tool.Write, run: func(_ context.Context, _ tool.Env) tool.Result { ran++; return tool.Result{Text: "changed"} }})
	result := invokeTurn(t, r, &engineSink{})
	if result.Stop != protocol.StopEnd || ran != 0 || len(g.requests) != 2 {
		t.Fatalf("stop=%s ran=%d calls=%d", result.Stop, ran, len(g.requests))
	}
	found := false
	for _, m := range g.requests[1].Messages {
		if strings.Contains(m.Content, "could not be read") {
			found = true
		}
	}
	if !found {
		t.Fatal("malformed tool call did not get a retry note")
	}
}
