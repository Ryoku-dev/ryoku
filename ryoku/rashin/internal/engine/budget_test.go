package engine

import (
	"context"
	"testing"

	"ryoku-rashin/internal/llm"
)

func TestBudgetReserveAndTriggerPerWindow(t *testing.T) {
	cases := []struct {
		window, reserve, trigger int
	}{
		{16384, 4096, 9830},    // 25% hits the 4096 floor
		{32768, 8192, 19660},   // 25% of a small window
		{131072, 19660, 89129}, // omp: 15%, at least 16384
		{65536, 16384, 39321},  // omp's floor wins over 15%
	}
	for _, c := range cases {
		b := NewBudget(c.window)
		if b.Reserve != c.reserve || b.Trigger != c.trigger {
			t.Errorf("window %d: reserve %d trigger %d, want %d %d", c.window, b.Reserve, b.Trigger, c.reserve, c.trigger)
		}
		if b.Over(c.trigger) || !b.Over(c.trigger+1) {
			t.Errorf("window %d: Over must fire just past the trigger", c.window)
		}
	}
	if b := NewBudget(0); b.Window != 16384 {
		t.Errorf("unknown window sized as %d, want 16384", b.Window)
	}
}

func TestFitOutputKeepsPromptPlusOutputInsideWindow(t *testing.T) {
	cases := []struct {
		name                      string
		window, prompt, requested int
		want                      int
	}{
		{"fits", 16384, 2000, 4096, 4096},
		{"shrinks to the room left", 16384, 15000, 4096, 1320},
		{"floor near a full window", 16384, 16000, 4096, 1024},
		{"no cap asks for the room", 32768, 10000, 0, 22704},
		{"large window", 131072, 100000, 8192, 8192},
	}
	for _, c := range cases {
		if got := FitOutput(c.window, c.prompt, c.requested); got != c.want {
			t.Errorf("%s: FitOutput(%d, %d, %d) = %d, want %d", c.name, c.window, c.prompt, c.requested, got, c.want)
		}
	}
}

type fixedWindow int

func (w fixedWindow) Window(context.Context, llm.Request) int { return int(w) }

func TestWindowResolutionOrder(t *testing.T) {
	ctx := context.Background()
	if got := Window(ctx, fixedWindow(40000), llm.Request{}, llm.Final{Context: 8192}); got != 40000 {
		t.Errorf("gateway window: got %d", got)
	}
	if got := Window(ctx, fixedWindow(0), llm.Request{}, llm.Final{Context: 8192}); got != 8192 {
		t.Errorf("last answer's window: got %d", got)
	}
	if got := Window(ctx, nil, llm.Request{}, llm.Final{}); got != 16384 {
		t.Errorf("unknown window: got %d", got)
	}
}

func TestPromptTokensAnchorsOnProviderCount(t *testing.T) {
	req := llm.Request{Messages: []llm.Message{
		{Role: llm.RoleSystem, Content: "system"},
		{Role: llm.RoleUser, Content: "hello"},
		{Role: llm.RoleAssistant, Content: "hi"},
		{Role: llm.RoleUser, Content: "0123456789012345678901234567890123456789"},
	}}
	tail := llm.EstimateMessage(req.Messages[3])
	if got, want := PromptTokens(req, Anchor{Tokens: 5000, Tail: 1}), 5000+tail+tail/10; got != want {
		t.Errorf("anchored prompt %d, want %d", got, want)
	}
	if got, want := PromptTokens(req, Anchor{}), llm.EstimateRequest(req); got != want {
		t.Errorf("unanchored prompt %d, want %d", got, want)
	}
	if got, want := PromptTokens(req, Anchor{Tokens: 5000, Tail: 9}), llm.EstimateRequest(req); got != want {
		t.Errorf("stale anchor must estimate whole: %d, want %d", got, want)
	}
}
