package engine

// Derived from oh-my-pi (MIT), packages/agent/src/agent-loop.ts@c7fff67

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// reply is one model attempt as it came back.
type reply struct {
	// text is the reply's whole text, visible what the user was shown of it
	// (Ask holds back its NEEDS_AGENT line and memory fences).
	text     string
	visible  string
	thinking []llm.Thinking
	calls    []llm.ToolCall
	final    llm.Final
	// interrupt is a stream rule that stopped the attempt; notices are rules
	// that let it go on and are told after the round.
	interrupt *Interrupt
	notices   []string
	// looped is set when the reply repeated itself and was stopped.
	looped bool
	// shown is set once text or a tool line reached the user: such an
	// attempt is not replayed silently.
	shown bool
	err   error
}

// streamReply runs one model attempt: deltas become text and thought
// events as they come, reasoning is kept for replay, tool calls are
// collected, and the stream guards and the loop detector may stop it early.
func (r *runner) streamReply(ctx context.Context, ts *turnState, req llm.Request) reply {
	var rep reply
	if r.f.deps.Gateway == nil {
		rep.err = errors.New("no model gateway")
		return rep
	}
	ts.attempts++
	message := fmt.Sprintf("%s-%d-%d", ts.prefix, ts.turn, ts.attempts)
	var text, visible, reasoning strings.Builder
	type streamed struct {
		kind string
		data any
	}
	var held []streamed
	send := func(kind string, data any) {
		if len(ts.guards) > 0 {
			held = append(held, streamed{kind, data})
			return
		}
		ts.sink.Emit(kind, data)
	}
	emitText := func(delta string) {
		if delta == "" {
			return
		}
		visible.WriteString(delta)
		rep.shown = len(ts.guards) == 0
		send(protocol.EvText, protocol.TextData{Message: message, Delta: delta})
	}
	emitThought := func(delta string) {
		if delta != "" {
			send(protocol.EvThought, protocol.TextData{Message: message, Delta: delta})
		}
	}
	var filter AskFilter
	if ts.ask && r.f.deps.Ask != nil {
		filter = r.f.deps.Ask.Filter(emitText, emitThought)
	}
	closeBlock := func(signature string) {
		if reasoning.Len() > 0 || signature != "" {
			rep.thinking = append(rep.thinking, llm.Thinking{Text: reasoning.String(), Signature: signature})
			reasoning.Reset()
		}
	}
	ts.loop.reset()

	stream, err := r.f.deps.Gateway.Stream(ctx, req)
	if err != nil {
		rep.err = err
		return rep
	}
	defer stream.Close()
	for {
		delta, err := stream.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			rep.err = err
			break
		}
		if delta.Text != "" {
			text.WriteString(delta.Text)
			if filter != nil {
				filter.Text(delta.Text)
			} else {
				emitText(delta.Text)
			}
		}
		if delta.Reasoning != "" {
			reasoning.WriteString(delta.Reasoning)
			if filter != nil {
				filter.Thought(delta.Reasoning)
			} else {
				emitThought(delta.Reasoning)
			}
		}
		if delta.Signature != "" {
			closeBlock(delta.Signature)
		}
		if stop := ts.observe(delta, &rep); stop {
			break
		}
		if ts.loop.feed(delta.Text) || ts.loop.feed(delta.Reasoning) {
			rep.looped = true
			break
		}
		if start := delta.ToolStart; start != nil && start.ID != "" {
			rep.shown = len(ts.guards) == 0
			send(protocol.EvTool, protocol.ToolData{Call: start.ID, Title: start.Name, Kind: ts.kindOf(start.Name), Status: protocol.ToolPending, Parent: r.parentCall()})
		}
		rep.calls = append(rep.calls, delta.ToolCalls...)
	}
	if filter != nil {
		filter.Close()
	}
	if rep.err == nil && rep.interrupt == nil && !rep.looped {
		for _, event := range held {
			ts.sink.Emit(event.kind, event.data)
		}
		if len(held) > 0 {
			rep.shown = true
		}
	}
	closeBlock("")
	rep.text = text.String()
	rep.visible = visible.String()
	if rep.err == nil && rep.interrupt == nil && !rep.looped {
		rep.final = stream.Final()
	}
	if ctx.Err() != nil && rep.err == nil && rep.interrupt == nil && !rep.looped {
		rep.err = ctx.Err()
	}
	return rep
}

// observe hands a delta to the turn's stream guards: an interrupt stops the
// attempt, notices are kept for after the round.
func (ts *turnState) observe(delta llm.Delta, rep *reply) bool {
	for _, g := range ts.guards {
		hit := g.Observe(delta)
		if hit == nil {
			continue
		}
		if hit.Notice {
			rep.notices = append(rep.notices, hit.Message)
			continue
		}
		rep.interrupt = hit
		return true
	}
	return false
}

// fillCallIDs gives every call a unique id: some local servers send none,
// some send the same one twice.
func fillCallIDs(calls []llm.ToolCall, prefix string, turn, attempt int) {
	seen := make(map[string]bool, len(calls))
	for i := range calls {
		if calls[i].ID == "" || seen[calls[i].ID] {
			calls[i].ID = fmt.Sprintf("%s-%d-%d-tool-%d", prefix, turn, attempt, i+1)
		}
		seen[calls[i].ID] = true
	}
}

// via names the model that answered as provider/model.
func via(final llm.Final) string {
	model := strings.TrimSpace(final.Model)
	provider := strings.TrimSpace(final.Provider)
	if provider == "" || strings.HasPrefix(model, provider+"/") {
		return model
	}
	if model == "" {
		return provider
	}
	return provider + "/" + model
}

// finishStop maps a finish reason to a turn's stop.
func finishStop(finish string) string {
	switch finish {
	case llm.FinishLength:
		return protocol.StopMaxTokens
	case llm.FinishFilter:
		return protocol.StopRefusal
	default:
		return protocol.StopEnd
	}
}
