package gateway

import (
	"errors"
	"io"

	"ryoku-rashin/internal/gateway/textcalls"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// attempt is one upstream call as the router reads it: the provider's own
// stream, or that stream read through a tool-call dialect.
type attempt interface {
	Next() (llm.Delta, error)
	Close() error
	outcome() attemptOutcome
}

// attemptOutcome is what a finished attempt reports: its finish reason, the
// usage the provider counted, and how many deltas it gave.
type attemptOutcome struct {
	finish     string
	usage      llm.Usage
	usageKnown bool
	emitted    int
}

// wrapDialect scans local native-model text for leaked calls, while a model
// configured with a text dialect always uses that dialect.
func wrapDialect(inner *sseAttempt, p *provider, m *model, request llm.Request) attempt {
	if len(request.Tools) == 0 {
		return inner
	}
	dialect := m.ToolDialect
	if dialect == "" {
		if p.Tier != protocol.TierLocal {
			return inner
		}
		dialect = textcalls.Auto
	}
	scanner := textcalls.ForDialect(dialect, textcalls.Options{
		Tools: request.Tools,
		// a local server may leave <think> in the text where a hosted one
		// would have split it out
		Thinking: p.Tier == protocol.TierLocal,
		NewID:    newCallID,
	})
	if scanner == nil {
		return inner
	}
	return &dialectAttempt{inner: inner, scanner: scanner, scanning: true}
}

type dialectAttempt struct {
	inner    *sseAttempt
	scanner  textcalls.Scanner
	scanning bool
	queue    []llm.Delta
	done     bool
	emitted  int
	// parsed counts calls read from text; fabricated is a model that wrote
	// a tool's result itself, after which nothing it says is kept
	parsed     int
	fabricated bool
}

func (d *dialectAttempt) Next() (llm.Delta, error) {
	for {
		if len(d.queue) > 0 {
			delta := d.queue[0]
			d.queue = d.queue[1:]
			d.emitted++
			return delta, nil
		}
		if d.done {
			return llm.Delta{}, io.EOF
		}
		delta, err := d.inner.Next()
		if errors.Is(err, io.EOF) {
			if d.scanning {
				d.push(d.scanner.Flush(d.inner.finish == llm.FinishLength))
				d.scanning = false
			}
			d.done = true
			continue
		}
		if err != nil {
			return delta, err
		}
		if delta.Reasoning != "" || delta.Signature != "" {
			d.queue = append(d.queue, llm.Delta{Reasoning: delta.Reasoning, Signature: delta.Signature})
		}
		native := delta.ToolStart != nil || len(delta.ToolCalls) > 0
		if d.scanning && native {
			// the provider parses calls itself: what the scanner holds is text
			d.push(d.scanner.Flush(false))
			d.scanning = false
		}
		if delta.Text != "" {
			if d.scanning {
				d.push(d.scanner.Feed(delta.Text))
			} else {
				d.queue = append(d.queue, llm.Delta{Text: delta.Text})
			}
		}
		if d.fabricated {
			_ = d.inner.Close()
			d.done = true
			continue
		}
		if native {
			d.queue = append(d.queue, llm.Delta{ToolStart: delta.ToolStart, ToolCalls: delta.ToolCalls})
		}
	}
}

// push turns scanner events into deltas. A broken partial call is withheld:
// the engine sees a length stop or an empty reply and can ask again.
func (d *dialectAttempt) push(events []textcalls.Event) {
	for _, event := range events {
		if event.Fabricated {
			d.fabricated = true
		}
		if event.Text != "" || event.Reasoning != "" {
			d.queue = append(d.queue, llm.Delta{Text: event.Text, Reasoning: event.Reasoning})
		}
		if event.Start != nil {
			start := *event.Start
			d.queue = append(d.queue, llm.Delta{ToolStart: &start})
		}
		if event.Call != nil {
			d.parsed++
			d.queue = append(d.queue, llm.Delta{ToolCalls: []llm.ToolCall{*event.Call}})
		}
	}
}

func (d *dialectAttempt) outcome() attemptOutcome {
	out := d.inner.outcome()
	out.emitted = d.emitted
	if d.parsed > 0 && (out.finish == "" || out.finish == llm.FinishStop || d.fabricated) {
		out.finish = llm.FinishToolCalls
	}
	return out
}

func (d *dialectAttempt) Close() error { return d.inner.Close() }
