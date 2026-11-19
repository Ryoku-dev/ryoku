package shell

import (
	"encoding/json"
	"image"
	"testing"
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"

	"ryoku-rashin/internal/protocol"
)

// idleFrames follows requested frames through a settling interval and then
// counts the idle interval without losing a wakeup at their boundary.
func (s *stage) idleFrames(settle, duration time.Duration) int {
	s.t.Helper()
	start := s.now.Add(settle)
	end := start.Add(duration)
	frames := 0
	for s.now.Before(end) {
		at, wake := s.router.WakeupTime()
		if !wake || at.After(end) {
			break
		}
		if at.After(s.now) {
			s.now = at
		}
		s.frame()
		if !at.Before(start) {
			frames++
		}
		if frames > 10000 {
			s.t.Fatal("continuous invalidations did not settle")
		}
	}
	return frames
}

func (s *stage) finishDemoWork(except string) {
	s.t.Helper()
	snapshot := s.runtime.store.Snapshot()
	apply := func(kind string, value any) {
		s.t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			s.t.Fatal(err)
		}
		if err := s.runtime.store.Apply(protocol.Event{Event: kind, Data: data}); err != nil {
			s.t.Fatal(err)
		}
	}
	for _, info := range snapshot.Sessions {
		if info.ID == except || info.State != protocol.StateWorking && info.State != protocol.StateWaiting {
			continue
		}
		if info.Goal != nil {
			goal := *info.Goal
			goal.Status = protocol.GoalDone
			info.Goal = &goal
		}
		info.State, info.Alive = protocol.StateIdle, false
		apply(protocol.EvtSessionInfo, info)
		if attached, ok := snapshot.Attached[info.ID]; ok {
			end, err := json.Marshal(protocol.TurnEndData{Turn: info.Turns, Stop: protocol.StopEnd})
			if err != nil {
				s.t.Fatal(err)
			}
			apply(protocol.EvtSessionEvent, protocol.SessionEventData{
				Session: info.ID, SessionEvent: protocol.SessionEvent{
					Seq: attached.Model.LastSeq() + 1, Kind: protocol.EvTurnEnd, Data: end,
				},
			})
		}
	}
	for _, task := range snapshot.TeamTasks {
		if task.Status == protocol.TaskWorking {
			task.Status = protocol.TaskDone
			apply(protocol.EvtTeamTask, task)
		}
	}
	for _, team := range snapshot.Teams {
		if team.Stats.Working > 0 {
			team.Stats.Working = 0
			apply(protocol.EvtTeam, team)
		}
	}
	s.frame()
}

func TestMainRoutesRequestNoFramesAtRest(t *testing.T) {
	for _, route := range []string{
		"ask/new", "ask/ask-20261110-094200-a17c",
		"agent", "agent/agent-20261110-101500-b82d", "agent/agent-working", "agent/agent-done",
		"settings/providers", "learning", "teams", "memory",
	} {
		t.Run(route, func(t *testing.T) {
			s := newMovingStage(t, image.Pt(1440, 900))
			s.open(route)
			if route == "settings/providers" && !s.provider(protocol.ProviderOllamaLocal).Running {
				t.Fatal("the local provider is not running in the demo fixture")
			}
			s.finishDemoWork("")
			// Gio's focused caret stops blinking after ten seconds; this
			// interval starts after that finite animation has settled.
			frames := s.idleFrames(11*time.Second, 10*time.Second)
			if frames != 0 {
				t.Errorf("%s requested %d frames over ten idle seconds", route, frames)
			}
		})
	}
}

func TestWaitingAgentDoesNotAnimateWhileAwaitingPermission(t *testing.T) {
	const waiting = "agent-20261110-101500-b82d"
	s := newMovingStage(t, image.Pt(1440, 900))
	s.open("agent/" + waiting)
	s.finishDemoWork(waiting)
	if info := s.runtime.store.Snapshot().Sessions[waiting]; info.State != protocol.StateWaiting {
		t.Fatalf("waiting agent unexpectedly changed to %s", info.State)
	}
	if frames := s.idleFrames(11*time.Second, 10*time.Second); frames != 0 {
		t.Errorf("agent waiting for permission requested %d frames in ten seconds", frames)
	}
}

func TestWorkingMarksAreBoundedAndStopWithWork(t *testing.T) {
	for _, route := range []string{"agent/agent-working", "teams"} {
		t.Run(route, func(t *testing.T) {
			s := newMovingStage(t, image.Pt(1440, 900))
			s.open(route)
			frames := s.idleFrames(2*time.Second, 10*time.Second)
			t.Logf("active work: %d frames over ten seconds", frames)
			if frames < 100 || frames > 160 {
				t.Errorf("active work requested %d frames in ten seconds, want a shared low-rate mark", frames)
			}
			s.finishDemoWork("")
			if frames := s.idleFrames(0, time.Second); frames != 0 {
				t.Errorf("completed work requested %d further frames in one second", frames)
			}
		})
	}
}

func TestReducedMotionKeepsWorkingMarkStill(t *testing.T) {
	s := newStage(t, image.Pt(1440, 900))
	s.open("teams")
	if frames := s.idleFrames(2*time.Second, 10*time.Second); frames != 0 {
		t.Errorf("reduced motion requested %d frames in ten seconds", frames)
	}
}

func TestOpenPreviewDoesNotPollForever(t *testing.T) {
	s := openStage(t, image.Pt(1440, 2600), true, Options{Route: protocol.TabAsk, Session: "ask-artifacts"})
	s.open("ask/ask-artifacts")
	s.finishDemoWork("")
	s.click("Preview")
	s.await("preview opens", func() bool { _, ok := s.named("Close preview"); return ok })
	if _, ok := s.named("Check preview"); !ok {
		t.Fatal("preview has no manual error refresh")
	}
	if frames := s.idleFrames(11*time.Second, 10*time.Second); frames != 0 {
		t.Errorf("open preview requested %d frames over ten idle seconds", frames)
	}
}

func TestIndeterminateProgressStopsOnCompletionAndReducedMotion(t *testing.T) {
	check := func(t *testing.T, moving bool) {
		t.Helper()
		var s *stage
		if moving {
			s = newMovingStage(t, image.Pt(1440, 900))
		} else {
			s = newStage(t, image.Pt(1440, 900))
		}
		s.open("settings/providers")
		active := true
		s.surface = func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Exact(image.Pt(400, 6))
			return kit.BladeProgress{Fraction: -1, Active: active}.Layout(gtx, s.shell.environment().Theme.Theme)
		}
		s.frame()
		frames := s.idleFrames(0, 10*time.Second)
		if moving {
			t.Logf("active sweep: %d frames over ten seconds", frames)
		}
		if moving && (frames < 120 || frames > 130) {
			t.Errorf("indeterminate progress requested %d frames in ten seconds", frames)
		} else if !moving && frames != 0 {
			t.Errorf("reduced motion requested %d progress frames", frames)
		}
		active = false
		s.frame()
		if frames := s.idleFrames(0, time.Second); frames != 0 {
			t.Errorf("completed progress requested %d more frames", frames)
		}
	}
	t.Run("moving", func(t *testing.T) { check(t, true) })
	t.Run("reduced", func(t *testing.T) { check(t, false) })
}
