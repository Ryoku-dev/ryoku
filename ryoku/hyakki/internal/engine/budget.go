package engine

// Derived from oh-my-pi (MIT), packages/agent/src/output-budget.ts@c7fff67
// and packages/agent/src/compaction/compaction.ts@c7fff67 (reserve and
// threshold).

import (
	"context"

	"ryoku-rashin/internal/engine/tool"
	"ryoku-rashin/internal/llm"
)

const (
	// MinOutputTokens is the smallest max_tokens FitOutput asks for: near a
	// full window a turn then stops on length instead of failing with a 400.
	MinOutputTokens = 1024
	// OutputHeadwayTokens covers framing no message count sees (generation
	// prompt, reasoning wrappers); omp measured caps landing 11 to 42 tokens
	// over without it.
	OutputHeadwayTokens = 64
	// SmallWindow is the largest window that gets the proportional reserve.
	SmallWindow = 32768
	// smallReserveFloor is the least a small window reserves.
	smallReserveFloor = 4096
	// largeReserveFloor is omp's default reserve.
	largeReserveFloor = 16384
)

// WindowSource tells the window of the model a request routes to first, 0
// when unknown; core.Gateway is one.
type WindowSource interface {
	Window(ctx context.Context, req llm.Request) int
}

// Window is the context window req will run in: what the gateway knows of
// the model it routes to first, else the window the last answer reported,
// else tool.DefaultWindow (the smallest a local model is run with).
func Window(ctx context.Context, gw WindowSource, req llm.Request, last llm.Final) int {
	if gw != nil {
		if w := gw.Window(ctx, req); w > 0 {
			return w
		}
	}
	if last.Context > 0 {
		return last.Context
	}
	return tool.DefaultWindow
}

// Budget is how a window is spent: Reserve is kept for the reply and the
// next prompt's growth, Trigger is the prompt size past which the
// conversation is compacted.
type Budget struct {
	Window  int
	Reserve int
	Trigger int
}

// NewBudget sizes a window (0 or less is tool.DefaultWindow).
func NewBudget(window int) Budget {
	if window <= 0 {
		window = tool.DefaultWindow
	}
	reserve := ReserveFor(window)
	return Budget{Window: window, Reserve: reserve, Trigger: (window - reserve) * 8 / 10}
}

// ReserveFor is the reserve of a window: max(25% of it, 4096) up to
// SmallWindow, where omp's fixed 16384 would leave a 16K model nothing;
// omp's max(15% of it, 16384) above. It never reaches the window.
func ReserveFor(window int) int {
	var r int
	if window <= SmallWindow {
		r = max(window/4, smallReserveFloor)
	} else {
		r = max(window*15/100, largeReserveFloor)
	}
	if r >= window {
		r = max(1, window*15/100)
	}
	return r
}

// Over reports a prompt past the compaction trigger.
func (b Budget) Over(prompt int) bool { return prompt > b.Trigger }

// PromptTokens is req's prompt size: the provider's count at the anchor plus
// an estimate of the messages after it (with omp's tenth margin), or the
// whole request estimated when nothing anchors.
func PromptTokens(req llm.Request, a Anchor) int {
	if a.Tokens <= 0 || a.Tail < 0 || a.Tail > len(req.Messages) {
		return llm.EstimateRequest(req)
	}
	tail := a.Extra
	for _, m := range req.Messages[len(req.Messages)-a.Tail:] {
		tail += llm.EstimateMessage(m) * 11 / 10
	}
	// A changed system prompt or tool schema invalidates a provider anchor.
	return max(llm.EstimateRequest(req), a.Tokens+tail)
}

// FitOutput is max_tokens for a prompt in a window, so prompt plus output
// never exceeds it (llama.cpp rejects that): requested when it fits, else
// the room left minus OutputHeadwayTokens, never below MinOutputTokens.
// requested 0 asks for all the room there is.
func FitOutput(window, prompt, requested int) int {
	if window <= 0 {
		window = tool.DefaultWindow
	}
	room := window - prompt - OutputHeadwayTokens
	if requested > 0 && room >= requested {
		return requested
	}
	return max(MinOutputTokens, room)
}
