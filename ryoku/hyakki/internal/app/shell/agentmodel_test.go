package shell

import (
	"image"
	"strings"
	"testing"

	"gioui.org/io/key"

	"ryoku-rashin/internal/protocol"
)

func (s *stage) startedOn(model string) (protocol.SessionInfo, bool) {
	for _, info := range s.runtime.store.Snapshot().Sessions {
		if strings.HasPrefix(info.ID, "agent-demo-") && info.Model == model {
			return info, true
		}
	}
	return protocol.SessionInfo{}, false
}

func TestTheBoardPicksTheModelASessionStartsOn(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("agent")
	stage.await("the native model list loads", func() bool {
		_, shown := stage.named("Model: LFM3 Omni 2B")
		return shown
	})
	stage.click("Model: LFM3 Omni 2B")
	stage.click("Smart")
	if _, shown := stage.named("Model: Smart"); !shown {
		t.Fatalf("the pick did not reach the board's model control; setting=%q, on screen: %v", stage.settings().Agent.Model, stage.names())
	}
	stage.await("the model is the new session default", func() bool {
		return stage.settings().Agent.Model == protocol.RouteSmart
	})
	if _, shown := stage.named("Model: Smart"); !shown {
		t.Fatalf("the board does not show the model; on screen: %v", stage.names())
	}
	stage.typeText("fix the build")
	stage.press(key.NameReturn, 0)
	var info protocol.SessionInfo
	stage.await("session.new carries the model", func() bool {
		var ok bool
		info, ok = stage.startedOn(protocol.RouteSmart)
		return ok
	})
	stage.await("the session opens on that model", func() bool {
		_, shown := stage.named("Model: Smart")
		return shown && stage.shell.router.Current().Session == info.ID
	})
}
