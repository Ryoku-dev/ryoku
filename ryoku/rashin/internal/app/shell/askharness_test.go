package shell

import (
	"image"
	"testing"
)

// Answer with in Settings > Ask picks what a new ask runs on: the demo daemon
// records ask.harness, the next ask's composer names the harness and its
// model menu lists what that harness offers.
func TestAnswerWithPutsTheNextAskOnAHarnessAndItsModels(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("settings/ask")
	if harness := stage.runtime.store.Snapshot().Settings.Ask.Harness; harness != "" {
		t.Fatalf("the demo starts with Ask on %q, not on Rashin's providers", harness)
	}
	stage.click("Rashin's providers")
	stage.click("Claude Code")
	stage.await("the pick becomes ask.harness", func() bool {
		return stage.runtime.store.Snapshot().Settings.Ask.Harness == "claude"
	})

	stage.open("ask/new")
	if _, shown := stage.named("Answered by Claude Code"); !shown {
		t.Fatalf("a new ask does not say Claude Code answers it; on screen: %v", stage.names())
	}
	stage.click("Model: Opus 5.5")
	for _, model := range []string{"Sonnet 5", "Haiku 5"} {
		if _, listed := stage.named(model); !listed {
			t.Fatalf("the model menu does not list %s; on screen: %v", model, stage.names())
		}
	}
}

// An ask a harness answered keeps saying so when it is opened again, with
// the model it ran on, and shows what Read only refused.
func TestAnAskAHarnessAnsweredSaysSoWhenOpened(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("ask/ask-20261110-101000-harn")
	for _, name := range []string{"Answered by Claude Code", "Model: Haiku 5"} {
		if _, shown := stage.named(name); !shown {
			t.Fatalf("the harness ask does not show %q; on screen: %v", name, stage.names())
		}
	}
}
