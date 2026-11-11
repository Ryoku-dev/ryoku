package teamsui

import (
	"testing"
	"time"

	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// after a restart the finished task's session is not loaded, so its kept steps tell how it went
func TestWorkedKeepsItsStepsWithoutTheSession(t *testing.T) {
	started := time.Date(2026, time.November, 11, 15, 0, 0, 0, time.UTC)
	ended := started.Add(52 * time.Second)
	task := protocol.TeamTask{
		ID: "task-a", Status: protocol.TaskDone, Session: "agent-a", Started: &started, Ended: &ended, Steps: 4,
	}
	if words, _ := Worked(state.Snapshot{}, task); words != "Worked for 52s · 4 steps" {
		t.Errorf("without its session the task reads %q, want %q", words, "Worked for 52s · 4 steps")
	}
	loaded := state.Snapshot{Sessions: map[string]protocol.SessionInfo{"agent-a": {ID: "agent-a", Steps: 6}}}
	if words, _ := Worked(loaded, task); words != "Worked for 52s · 6 steps" {
		t.Errorf("with its session the task reads %q, want %q", words, "Worked for 52s · 6 steps")
	}
}
