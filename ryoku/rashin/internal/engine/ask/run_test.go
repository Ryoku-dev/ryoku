package ask

import (
	"context"
	"strings"
	"testing"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/engine/exec"
	"ryoku-rashin/internal/protocol"
)

func TestUserRunStreamsOutputAndReturnsModelMessage(t *testing.T) {
	sink := &captureSink{}
	x := exec.Host(exec.HostOptions{})
	defer x.Close()
	input := core.PromptInput{Turn: 2, Run: `printf 'line one\n'; printf 'warning\n' >&2`}
	message, ok, err := New(nil).Run(context.Background(), x, input, t.TempDir(), sink)
	if err != nil || !ok {
		t.Fatalf("user run = %v, %v", ok, err)
	}
	if !strings.Contains(message.Content, "line one") || !strings.Contains(message.Content, "warning") || !strings.Contains(message.Content, "exit status 0") || !strings.Contains(message.Content, "Do not propose this command again") {
		t.Fatalf("model received %q", message.Content)
	}
	var output string
	var ended bool
	for _, event := range sink.events {
		if event.kind != protocol.EvTerminal {
			continue
		}
		terminal := event.data.(protocol.TerminalData)
		output += terminal.Delta
		if terminal.Done && terminal.Exit != nil && *terminal.Exit == 0 {
			ended = true
		}
	}
	if !ended || !strings.Contains(output, "line one") || !strings.Contains(output, "warning") {
		t.Fatalf("terminal output = %q, ended = %v", output, ended)
	}
}

func TestUserRunRefusesPasswordTerminal(t *testing.T) {
	sink := &captureSink{}
	_, _, err := New(nil).Run(context.Background(), nil, core.PromptInput{Turn: 2, Run: "sudo true"}, t.TempDir(), sink)
	if err == nil || !strings.Contains(err.Error(), "needs a terminal") || len(sink.events) != 0 {
		t.Fatalf("terminal-only run = %v, events %#v", err, sink.events)
	}
}
