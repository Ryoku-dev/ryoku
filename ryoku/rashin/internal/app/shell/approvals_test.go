package shell

import (
	"context"
	"image"
	"testing"

	"gioui.org/io/key"

	"ryoku-rashin/internal/protocol"
)

func (s *stage) settings() protocol.Settings { return s.runtime.store.Snapshot().Settings }

func (s *stage) sessionInfo(id string) protocol.SessionInfo {
	for _, info := range s.runtime.store.Snapshot().Sessions {
		if info.ID == id {
			return info
		}
	}
	return protocol.SessionInfo{}
}

// choose opens the select called from and picks the row called pick
func (s *stage) choose(from, pick string) {
	s.t.Helper()
	s.click(from)
	s.click(pick)
}

// What Ask may do is chosen in its composer. With no conversation open the
// choice is what the next one starts with, and it is the setting, so it is
// still the choice after a restart.
func TestAskIsToldWhatItMayDoFromItsComposer(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("ask/new")
	if _, ok := stage.named("Asking: Read only"); !ok {
		t.Fatalf("a new ask does not start read-only; on screen: %v", stage.names())
	}

	stage.choose("Asking: Read only", "YOLO")
	stage.await("the setting follows the choice", func() bool { return stage.settings().Ask.Approvals == protocol.ApprovalsAuto })
	if _, ok := stage.named("Asking: YOLO"); !ok {
		t.Fatalf("the composer does not show the choice; on screen: %v", stage.names())
	}
	if !stage.typedArrives("what is using my disk") {
		t.Fatal("picking what Ask may do did not hand the keyboard back to the composer")
	}

	known := make(map[string]bool)
	for _, info := range stage.runtime.store.Snapshot().Sessions {
		known[info.ID] = true
	}
	var started protocol.SessionInfo
	stage.press(key.NameReturn, 0)
	stage.await("the question starts a conversation", func() bool {
		for _, info := range stage.runtime.store.Snapshot().Sessions {
			if !known[info.ID] {
				started = info
			}
		}
		return started.ID != ""
	})
	if started.Kind != protocol.KindAsk || started.Mode != protocol.ApprovalsAuto {
		t.Fatalf("the conversation started as %s with mode %q", started.Kind, started.Mode)
	}
}

// In a conversation the choice is that conversation's own, and one made
// before modes existed reads as read-only until it is changed.
func TestAConversationKeepsItsOwnChoiceOfWhatAskMayDo(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	const id = "ask-20261110-094200-a17c"
	stage.open("ask/" + id)
	stage.choose("Asking: Read only", "Ask first")
	stage.await("the conversation takes the choice", func() bool { return stage.sessionInfo(id).Mode == protocol.ApprovalsAsk })
	if _, ok := stage.named("Asking: Ask first"); !ok {
		t.Fatalf("the composer does not show the choice; on screen: %v", stage.names())
	}

	// another conversation is not moved by it
	var other protocol.SessionInfo
	if err := stage.runtime.client.Call(context.Background(), protocol.MSessionNew, protocol.SessionNewParams{Kind: protocol.KindAsk}, &other); err != nil {
		t.Fatal(err)
	}
	// the list hears of a conversation when it is attached, and a place it has not heard of opens as the home
	if err := stage.runtime.client.Attach(context.Background(), other.ID); err != nil {
		t.Fatal(err)
	}
	stage.open("ask/" + other.ID)
	if _, ok := stage.named("Ask a follow-up"); !ok {
		t.Fatalf("the other conversation did not open; on screen: %v", stage.names())
	}
	if _, ok := stage.named("Asking: Read only"); !ok {
		t.Fatalf("another conversation took the first one's choice; on screen: %v", stage.names())
	}
}

// For agents the choice is one setting every session follows, so it reads the
// same on the board, in a session and in Settings, and YOLO is said in words
// under the board's composer.
func TestTheAgentsAskingChoiceIsOneEverywhere(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("agent")
	stage.choose("Asking: Ask before changes", "YOLO")
	stage.await("the setting follows the choice", func() bool { return stage.settings().Agent.Approvals == protocol.ApprovalsAuto })
	if _, ok := stage.named("Asking: YOLO"); !ok {
		t.Fatalf("the board does not show the choice; on screen: %v", stage.names())
	}

	stage.open("agent/agent-done")
	if _, ok := stage.named("Asking: YOLO"); !ok {
		t.Fatalf("a session does not show the board's choice; on screen: %v", stage.names())
	}
	stage.choose("Asking: YOLO", "Ask every time")
	stage.await("a session's choice is the setting too", func() bool { return stage.settings().Agent.Approvals == protocol.ApprovalsAsk })

	stage.open("settings/agents")
	if _, ok := stage.named("Before every action"); !ok {
		t.Fatalf("Settings does not show the choice made in a session; on screen: %v", stage.names())
	}
}
