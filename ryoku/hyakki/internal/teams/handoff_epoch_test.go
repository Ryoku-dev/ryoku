package teams

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// Drive the authenticated request handler while a prior decision owns
// manager attention. Closing the unbound controller joins its wake workers.
func handoffEpochQueued(t *testing.T, kind string) (*harness, protocol.Team, protocol.TeamTask, protocol.TeamTask, protocol.TeamMessage) {
	t.Helper()
	h, team := heartbeatHarness(t)
	peer := team.Members[1]
	peer.ID, peer.Name = "handoff-peer", "Requested peer"
	team.Members = append(team.Members, peer)
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	busyMessage := heartbeatMessage(t, h, team, "escalate", team.Members[0].ID, "")
	busy, accepted, err := h.s.wakeMessage(context.Background(), busyMessage)
	if err != nil || !accepted {
		t.Fatalf("occupy manager: %v %v", accepted, err)
	}
	if err := h.s.state.deliverMessage(busyMessage.ID); err != nil {
		t.Fatal(err)
	}
	source := heartbeatSource(t, h, team)
	ref := protocol.AgentRef{Session: source.Session, Token: "heartbeat-source-token"}
	ctx := WithAgentSession(context.Background(), source.Session)
	if kind == "help" {
		_, err = h.s.agentHelp(ctx, protocol.AgentHelpParams{AgentRef: ref, Member: peer.ID, Title: "Queued help", Brief: "Obtain a governed decision, not a lateral assignment."})
	} else {
		_, err = h.s.agentEscalate(ctx, protocol.AgentEscalateParams{AgentRef: ref, Title: "Queued escalation", Detail: "Resolve the original execution's blocker."})
	}
	if err != nil {
		t.Fatal(err)
	}
	messages := h.s.state.messages(func(m protocol.TeamMessage) bool { return m.Task == source.ID && m.Kind == kind })
	if len(messages) != 1 || messages[0].Delivered || messages[0].Retired || messages[0].SourceSession != source.Session {
		t.Fatalf("busy manager did not preserve the authenticated pending request: %+v", messages)
	}
	if current, _ := h.s.state.task(source.ID); current.Waiting != protocol.WaitingOnMembers {
		t.Fatal("handler failed to hold the original source execution")
	}
	h.s.mu.Lock()
	current, _ := h.s.state.task(source.ID)
	pending := h.s.pendingTaskRequestsLocked(current)
	h.s.mu.Unlock()
	if !pending {
		t.Fatal("queued request did not preserve the source's waiting hold")
	}
	raw, err := os.ReadFile(filepath.Join(h.dir, "messages", messages[0].ID+".json"))
	if err != nil || bytes.Contains(raw, []byte(ref.Token)) {
		t.Fatalf("message persisted a secret run token: %v", err)
	}
	return h, team, source, busy, messages[0]
}

func handoffEpochFinishManager(t *testing.T, h *harness, busy protocol.TeamTask) {
	t.Helper()
	busy.Status = protocol.TaskDone
	if err := h.s.state.putTask(busy); err != nil {
		t.Fatal(err)
	}
	h.s.clock.mu.Lock()
	h.s.clock.running = true
	h.s.clock.mu.Unlock()
	h.s.clock.rearm()
}

func handoffEpochRetired(t *testing.T, h *harness, message protocol.TeamMessage) {
	t.Helper()
	history, err := h.s.teamMessages(protocol.TeamMessagesParams{Task: message.Task})
	if err != nil {
		t.Fatal(err)
	}
	for _, current := range history.Messages {
		if current.ID != message.ID {
			continue
		}
		if !current.Retired || current.Delivered || current.Body != message.Body || current.SourceSession != message.SourceSession {
			t.Fatalf("retirement erased history or claimed delivery: %+v", current)
		}
		if err := h.s.state.deliverMessage(message.ID); err == nil {
			t.Fatal("a retired request was falsely acknowledged as delivered")
		}
		return
	}
	t.Fatal("queued request disappeared from owner-visible history")
}

func TestHandoffEpochOwnerStopRetiresBusyManagerQueue(t *testing.T) {
	for _, kind := range []string{"help", "escalate"} {
		t.Run(kind, func(t *testing.T) {
			h, team, source, busy, message := handoffEpochQueued(t, kind)
			// Provide only the synchronous session-control seam; no Bind or pump
			// runs while the owner parks the team for intervention.
			team.Paused = true
			if err := h.s.state.putTeam(team); err != nil {
				t.Fatal(err)
			}
			h.s.mu.Lock()
			h.s.sessions = newFakeSessions()
			h.s.mu.Unlock()
			stopped, err := h.s.stopTaskByOwner(context.Background(), source.ID, "You stopped it.")
			h.s.mu.Lock()
			h.s.sessions = nil
			h.s.mu.Unlock()
			if err != nil || stopped.Status != protocol.TaskStopped {
				t.Fatalf("stop: %+v %v", stopped, err)
			}
			team.Paused = false
			if err := h.s.state.putTeam(team); err != nil {
				t.Fatal(err)
			}
			handoffEpochFinishManager(t, h, busy)
			if tasks := heartbeatTasks(h.s, protocol.TaskFromMessage); len(tasks) != 1 {
				t.Fatalf("manager completion woke stopped source work: %+v", tasks)
			}
			restartHeartbeatHarness(t, h)
			handoffEpochRetired(t, h, message)
		})
	}
}

func TestHandoffEpochRetryAndReassignmentRetireOriginalRequest(t *testing.T) {
	for _, change := range []string{"retry", "reassign"} {
		t.Run(change, func(t *testing.T) {
			h, team, source, busy, message := handoffEpochQueued(t, "escalate")
			team.Paused = true // Keep the real replacement queued without worker activity.
			if err := h.s.state.putTeam(team); err != nil {
				t.Fatal(err)
			}
			h.s.mu.Lock()
			h.s.sessions = newFakeSessions()
			h.s.mu.Unlock()
			var replacement protocol.TeamTask
			var err error
			if change == "retry" {
				// Model an already-ended prior attempt whose queued message has
				// not yet been examined by recovery (including legacy state).
				current, _ := h.s.state.task(source.ID)
				current.Status = protocol.TaskBlocked
				if err := h.s.state.putTask(current); err != nil {
					t.Fatal(err)
				}
				replacement, err = h.s.changeTaskExecution(context.Background(), protocol.TeamTaskReassignParams{ID: source.ID}, "", taskChangeRetry)
			} else {
				replacement, err = h.s.taskReassign(context.Background(), protocol.TeamTaskReassignParams{ID: source.ID, Member: team.Members[2].ID})
			}
			h.s.mu.Lock()
			h.s.sessions = nil
			h.s.mu.Unlock()
			if err != nil || replacement.Session != "" || replacement.Status != protocol.TaskReady {
				t.Fatalf("replacement: %+v %v", replacement, err)
			}
			team.Paused = false
			if err := h.s.state.putTeam(team); err != nil {
				t.Fatal(err)
			}
			// Even a source with an active replacement session cannot inherit
			// the prior execution's pending authority.
			replacement.Session, replacement.Status = "replacement-session", protocol.TaskWorking
			if err := h.s.state.putTask(replacement); err != nil {
				t.Fatal(err)
			}
			handoffEpochFinishManager(t, h, busy)
			if tasks := heartbeatTasks(h.s, protocol.TaskFromMessage); len(tasks) != 1 {
				t.Fatalf("old request replayed into replacement: %+v", tasks)
			}
			handoffEpochRetired(t, h, message)
		})
	}
}

func TestHandoffEpochPendingSameSessionRestartSurvivesTokenRotation(t *testing.T) {
	h, _, source, busy, message := handoffEpochQueued(t, "help")
	restartHeartbeatHarness(t, h)
	// The fresh controller has no live run. Recovery relies on persisted
	// source identity, never the old token or an inherited process.
	handoffEpochFinishManager(t, h, busy)
	tasks := h.s.state.tasks(func(task protocol.TeamTask) bool { return task.Trigger == message.ID })
	if len(tasks) != 1 || tasks[0].Parent != source.ID || tasks[0].Member != message.To {
		t.Fatalf("valid pending restart request was lost or reassigned: %+v", tasks)
	}
	h.s.mu.Lock()
	h.s.runs[source.ID] = &run{Task: source.ID, Team: source.Team, Member: source.Member, Session: source.Session, Token: "rotated-token"}
	h.s.mu.Unlock()
	for range 3 {
		got, accepted, err := h.s.wakeMessage(context.Background(), message)
		if err != nil || !accepted || got.ID != tasks[0].ID {
			t.Fatalf("same-session restart duplicated or rejected delivered request: %+v %v %v", got, accepted, err)
		}
		h.s.clock.rearm()
	}
	if current := h.s.state.messages(func(m protocol.TeamMessage) bool { return m.ID == message.ID }); len(current) != 1 || !current[0].Delivered || current[0].Retired {
		t.Fatalf("valid restart delivery lifecycle: %+v", current)
	}
}

func TestHandoffEpochRecoveryFailsClosedForUnverifiableSource(t *testing.T) {
	for _, fault := range []string{"legacy", "closed", "session", "assignee"} {
		t.Run(fault, func(t *testing.T) {
			h, team := heartbeatHarness(t)
			message := heartbeatMessage(t, h, team, "escalate", team.Members[0].ID, "")
			source, _ := h.s.state.task(message.Task)
			switch fault {
			case "legacy":
				// A real old-format file has no durable execution identity.
				h.s.state.mu.Lock()
				message.SourceSession = ""
				err := h.s.state.writeMessageLocked(message)
				h.s.state.mu.Unlock()
				if err != nil {
					t.Fatal(err)
				}
				restartHeartbeatHarness(t, h)
			case "closed":
				source.Status = protocol.TaskDone
			case "session":
				source.Session = "another-attempt"
			case "assignee":
				source.Member = team.Members[0].ID
			}
			if err := h.s.state.putTask(source); err != nil {
				t.Fatal(err)
			}
			_, accepted, err := h.s.wakeMessage(context.Background(), message)
			if err != nil || accepted || len(heartbeatTasks(h.s, protocol.TaskFromMessage)) != 0 {
				t.Fatalf("unverifiable request woke work: %v %v", accepted, err)
			}
			handoffEpochRetired(t, h, message)
		})
	}
}

func TestHandoffEpochRetirementWriteFailureFencesOwnerReplacement(t *testing.T) {
	h, team, source, _, message := handoffEpochQueued(t, "escalate")
	path := filepath.Join(h.dir, "messages", message.ID+".json")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	blockMessageWrite(t, h.dir, message.ID)
	h.s.sessions = newFakeSessions()
	_, err := h.s.taskReassign(context.Background(), protocol.TeamTaskReassignParams{ID: source.ID, Member: team.Members[2].ID})
	h.s.sessions = nil
	if err == nil {
		t.Fatal("retirement storage failure published a replacement")
	}
	h.s.mu.Lock()
	fenced := h.s.taskEdits[source.ID]
	h.s.mu.Unlock()
	current, _ := h.s.state.task(source.ID)
	if !fenced || current.Session != source.Session || current.Member != source.Member {
		t.Fatalf("retirement failure lost execution fence: %+v fenced=%v", current, fenced)
	}
	stored := h.s.state.messages(func(m protocol.TeamMessage) bool { return m.ID == message.ID })
	if len(stored) != 1 || stored[0].Retired || stored[0].Delivered {
		t.Fatalf("failed retirement published an in-memory lifecycle change: %+v", stored)
	}
	if _, accepted, err := h.s.wakeMessage(context.Background(), message); err == nil || accepted {
		t.Fatalf("retirement write failure allowed obsolete wake: %v %v", accepted, err)
	}
}

func TestHandoffEpochPendingHoldMatchesOnlyCurrentExecution(t *testing.T) {
	for _, changed := range []string{"session", "assignee", "team", "closed", "delivered", "retired"} {
		t.Run(changed, func(t *testing.T) {
			h, team := heartbeatHarness(t)
			message := heartbeatMessage(t, h, team, "escalate", team.Members[0].ID, "")
			source, _ := h.s.state.task(message.Task)
			switch changed {
			case "session":
				source.Session = "replacement-session"
			case "assignee":
				source.Member = team.Members[0].ID
			case "team":
				source.Team = "replacement-team"
			case "closed":
				source.Status = protocol.TaskStopped
			case "delivered":
				if err := h.s.state.deliverMessage(message.ID); err != nil {
					t.Fatal(err)
				}
			case "retired":
				if err := h.s.state.retireMessage(message.ID); err != nil {
					t.Fatal(err)
				}
			}
			h.s.mu.Lock()
			pending := h.s.pendingTaskRequestsLocked(source)
			h.s.mu.Unlock()
			if pending {
				t.Fatal("obsolete or consumed request held a replacement execution")
			}
		})
	}
}
