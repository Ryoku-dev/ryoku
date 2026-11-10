package harness

import (
	"context"
	"testing"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

func TestTerminalLargeOutputIsCompleteOrTruncated(t *testing.T) {
	r, sink := helperRunner(t, "terminal-large", protocol.ApprovalsAuto, nil)
	for run := 1; run <= 30; run++ {
		if _, err := r.Prompt(context.Background(), core.PromptInput{Turn: run, Text: "terminal"}, sink); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		texts := sink.waitKind(t, protocol.EvText, run)
		status := texts[run-1].data.(protocol.TextData).Delta
		if status != "200000:false" && status != "1048576:true" {
			t.Fatalf("run %d returned incomplete unmarked output: %s", run, status)
		}
	}
}
