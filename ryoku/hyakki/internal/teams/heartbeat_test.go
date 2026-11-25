package teams

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/vault"
)

// Tests drive the unbound controller's single clock. Closing it joins wake
// workers and timer callbacks before a cold restart.
func heartbeatHarness(t *testing.T) (*harness, protocol.Team) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	h := &harness{dir: t.TempDir(), settings: config.Defaults(), clock: &fakeClock{at: at(10, 0, 0), frozen: true}}
	v := vault.OpenWith(filepath.Join(t.TempDir(), "vault.db"), fixedKeeper{key: bytes.Repeat([]byte{7}, 32)})
	s, err := open(h.dir, v, func(bool) {}, nil, h.currentSettings)
	if err != nil {
		t.Fatal(err)
	}
	h.s = s
	s.now = h.clock.now
	t.Cleanup(func() { _ = s.Close() })
	team := h.newTeam(t, "Heartbeat", nil)
	team.Purpose = "Ship the team goal"
	team.Members[0].Heartbeat = &protocol.MemberHeartbeat{Modes: []string{protocol.HeartbeatInterval, protocol.HeartbeatMessage}, EveryMinutes: 60, Instructions: "Inspect current work and report blockers."}
	if err := s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	team, _ = s.state.team(team.ID)
	return h, team
}

func heartbeatEntry(t *testing.T, h *harness, team protocol.Team) clockEntry {
	t.Helper()
	h.s.clock.mu.Lock()
	h.s.clock.running = true
	h.s.clock.mu.Unlock()
	h.s.clock.refresh(false)
	h.s.clock.mu.Lock()
	entry, ok := h.s.clock.next[heartbeatKey(team.ID, team.Members[0].ID)]
	h.s.clock.mu.Unlock()
	if !ok {
		t.Fatal("interval was not armed")
	}
	return entry
}

func heartbeatTasks(s *Service, origin string) []protocol.TeamTask {
	return s.state.tasks(func(task protocol.TeamTask) bool { return task.Origin == origin })
}

func heartbeatSource(t *testing.T, h *harness, team protocol.Team) protocol.TeamTask {
	t.Helper()
	started := h.clock.now()
	session := newID("session")
	source := protocol.TeamTask{ID: newID("task"), Team: team.ID, Member: team.Members[1].ID, Goal: "Original goal snapshot", Title: "Because task", Brief: "An active agent needs a manager decision.", Status: protocol.TaskWorking, Session: session, Sessions: []string{session}, Started: &started, Created: started, Updated: started}
	if err := h.s.state.putTask(source); err != nil {
		t.Fatal(err)
	}
	h.s.mu.Lock()
	h.s.runs[source.ID] = &run{Task: source.ID, Team: source.Team, Member: source.Member, Session: session, Token: "heartbeat-source-token"}
	h.s.mu.Unlock()
	return source
}

func heartbeatMessage(t *testing.T, h *harness, team protocol.Team, kind, to, peer string) protocol.TeamMessage {
	t.Helper()
	source := heartbeatSource(t, h, team)
	message, err := h.s.recordTeamMessage(WithAgentSession(context.Background(), source.Session), protocol.TeamMessage{Team: team.ID, Task: source.ID, From: source.Member, To: to, Member: peer, Kind: kind, Title: "Decision needed", Body: "Resolve the blocker without assigning a peer directly."})
	if err != nil {
		t.Fatal(err)
	}
	return message
}

func TestHeartbeatIntervalUsesSingleClockWithoutOverlapOrCatchup(t *testing.T) {
	h, team := heartbeatHarness(t)
	entry := heartbeatEntry(t, h, team)
	if !entry.at.Equal(at(11, 0, 0)) || !h.s.Armed() {
		t.Fatal("interval did not arm from configuration time")
	}
	h.clock.set(entry.at.Add(-time.Nanosecond))
	h.s.clock.tick()
	if len(heartbeatTasks(h.s, protocol.TaskFromHeartbeat)) != 0 {
		t.Fatal("interval fired early")
	}
	h.clock.set(entry.at)
	h.s.clock.tick()
	tasks := heartbeatTasks(h.s, protocol.TaskFromHeartbeat)
	if len(tasks) != 1 || tasks[0].Member != team.Members[0].ID || tasks[0].Routine != "" || tasks[0].Trigger == "" || tasks[0].Goal != team.Purpose || tasks[0].Status != protocol.TaskReady || !tasks[0].Automatic {
		t.Fatalf("interval task lost identity or lineage: %+v", tasks)
	}
	h.clock.set(entry.at.Add(48 * time.Hour))
	h.s.clock.tick()
	if len(heartbeatTasks(h.s, protocol.TaskFromHeartbeat)) != 1 {
		t.Fatal("overlapping interval work was created")
	}
	task := tasks[0]
	task.Status = protocol.TaskDone
	if err := h.s.state.putTask(task); err != nil {
		t.Fatal(err)
	}
	h.clock.advance(48 * time.Hour)
	h.s.clock.tick()
	if len(heartbeatTasks(h.s, protocol.TaskFromHeartbeat)) != 2 {
		t.Fatal("downtime was not coalesced to one due task")
	}
}

func TestHeartbeatIntervalRestartAndConcurrentDueAreIdempotent(t *testing.T) {
	h, team := heartbeatHarness(t)
	entry := heartbeatEntry(t, h, team)
	restartHeartbeatHarness(t, h)
	h.s.clock.running = true
	h.clock.set(entry.at)
	h.s.clock.refresh(true)
	if got := h.s.clock.due(heartbeatKey(team.ID, team.Members[0].ID)); !got.Equal(entry.at) {
		t.Fatalf("restart moved durable due time: %v", got)
	}
	var workers sync.WaitGroup
	for range 16 {
		workers.Add(1)
		go func() { defer workers.Done(); h.s.checkHeartbeat(entry, entry.at) }()
	}
	workers.Wait()
	if len(heartbeatTasks(h.s, protocol.TaskFromHeartbeat)) != 1 {
		t.Fatal("concurrent due callbacks duplicated a task")
	}
	// Simulate exit after task commit but before advancing the scheduling file.
	if err := h.s.state.putHeartbeatSchedule(heartbeatSchedule{Team: team.ID, Member: team.Members[0].ID, EveryMinutes: 60, Next: entry.at}); err != nil {
		t.Fatal(err)
	}
	restartHeartbeatHarness(t, h)
	h.s.checkHeartbeat(entry, entry.at)
	if len(heartbeatTasks(h.s, protocol.TaskFromHeartbeat)) != 1 {
		t.Fatal("restart forgot the committed trigger")
	}
}

func TestHeartbeatIntervalPauseResumeAndDisarming(t *testing.T) {
	h, team := heartbeatHarness(t)
	entry := heartbeatEntry(t, h, team)
	team.Paused = true
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	h.s.clock.rearm()
	if h.s.Armed() || !h.s.clock.due(heartbeatKey(team.ID, team.Members[0].ID)).IsZero() {
		t.Fatal("paused interval remained armed")
	}
	h.clock.advance(24 * time.Hour)
	h.s.clock.tick()
	if len(heartbeatTasks(h.s, protocol.TaskFromHeartbeat)) != 0 {
		t.Fatal("paused clock created work")
	}
	team.Paused = false
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	h.s.clock.rearm()
	resumed := h.s.clock.due(heartbeatKey(team.ID, team.Members[0].ID))
	if !resumed.Equal(h.clock.now().Add(time.Hour)) || !resumed.After(entry.at) {
		t.Fatal("resume attempted catch-up instead of a fresh interval")
	}
	team.Members[0].Heartbeat = &protocol.MemberHeartbeat{Modes: []string{}}
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	h.s.clock.rearm()
	h.s.clock.mu.Lock()
	armedTimer := h.s.clock.timer != nil
	h.s.clock.mu.Unlock()
	if h.s.Armed() || armedTimer {
		t.Fatal("last disabled automatic wake left the clock armed")
	}
	// Explicit owner assignment remains available while all automatic modes park.
	manual := h.newTask(t, team, "Owner intervention")
	if manual.Member != team.Members[0].ID {
		t.Fatal("manual owner intervention was blocked")
	}
}

func TestHeartbeatDueRechecksEveryExecutionBarrier(t *testing.T) {
	for _, barrier := range []string{"global", "team", "member", "budget", "edit", "task-edit", "disabled"} {
		t.Run(barrier, func(t *testing.T) {
			h, team := heartbeatHarness(t)
			entry := heartbeatEntry(t, h, team)
			switch barrier {
			case "global":
				h.settings.Teams.Paused = true
			case "team":
				team.Paused = true
			case "member":
				team.Members[0].Paused = true
			case "budget":
				team.Members[0].Budget = &protocol.TeamBudget{MonthTokens: 1}
				if err := h.s.addSpend(team.ID, team.Members[0].ID, "", "", h.clock.now(), protocol.TokenUsage{Input: 1}); err != nil {
					t.Fatal(err)
				}
			case "edit":
				h.s.editing[team.ID] = map[string]bool{team.Members[0].ID: true}
			case "task-edit":
				task := memberBudgetTask(t, h, team, team.Members[0].ID, "")
				h.s.taskEdits[task.ID] = true
			case "disabled":
				team.Members[0].Heartbeat = &protocol.MemberHeartbeat{Modes: []string{}}
			}
			if err := h.s.state.putTeam(team); err != nil {
				t.Fatal(err)
			}
			h.s.checkHeartbeat(entry, entry.at)
			if len(heartbeatTasks(h.s, protocol.TaskFromHeartbeat)) != 0 {
				t.Fatal("due callback bypassed an execution barrier")
			}
		})
	}
}

func TestHeartbeatMessageAuthorityContextAndRestartDeduplication(t *testing.T) {
	h, team := heartbeatHarness(t)
	message := heartbeatMessage(t, h, team, "escalate", team.Members[0].ID, "")
	var workers sync.WaitGroup
	for range 16 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, accepted, err := h.s.wakeMessage(context.Background(), message)
			if err != nil || !accepted {
				t.Errorf("wake: accepted=%v err=%v", accepted, err)
			}
		}()
	}
	workers.Wait()
	tasks := heartbeatTasks(h.s, protocol.TaskFromMessage)
	if len(tasks) != 1 || tasks[0].Member != message.To || tasks[0].Parent != message.Task || tasks[0].Goal != "Original goal snapshot" || tasks[0].Trigger != message.ID || tasks[0].Routine != "" || !tasks[0].Automatic {
		t.Fatalf("message lost authority or because-context: %+v", tasks)
	}
	restartHeartbeatHarness(t, h)
	got, accepted, err := h.s.wakeMessage(context.Background(), message)
	if err != nil || !accepted || got.ID != tasks[0].ID {
		t.Fatalf("restart duplicated an undelivered event: %+v %v %v", got, accepted, err)
	}
	if err := h.s.state.deliverMessage(message.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.s.state.removeTask(tasks[0].ID); err != nil {
		t.Fatal(err)
	}
	_, accepted, err = h.s.wakeMessage(context.Background(), message)
	if err != nil || !accepted || len(heartbeatTasks(h.s, protocol.TaskFromMessage)) != 0 {
		t.Fatal("delivered event was recreated after its wake task was pruned")
	}
}

func TestHeartbeatHelpWakesManagerNeverRequestedPeer(t *testing.T) {
	h, team := heartbeatHarness(t)
	peer := team.Members[1]
	peer.ID, peer.Name = "peer", "Peer"
	team.Members = append(team.Members, peer)
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	message := heartbeatMessage(t, h, team, "help", team.Members[0].ID, peer.ID)
	task, accepted, err := h.s.wakeMessage(context.Background(), message)
	if err != nil || !accepted || task.Member != team.Members[0].ID || task.Member == peer.ID {
		t.Fatalf("peer request became lateral assignment: %+v %v %v", task, accepted, err)
	}
}

func TestHeartbeatUndeliveredRecoveryHonorsModesRoutingAndShutdown(t *testing.T) {
	h, team := heartbeatHarness(t)
	team.Members[0].Heartbeat = &protocol.MemberHeartbeat{Modes: []string{}}
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	team, _ = h.s.state.team(team.ID)
	message := heartbeatMessage(t, h, team, "escalate", team.Members[0].ID, "")
	summoner := heartbeatMessage(t, h, team, "escalate", "", "")
	h.s.clock.running = true
	h.s.clock.rearm()
	if len(heartbeatTasks(h.s, protocol.TaskFromMessage)) != 0 {
		t.Fatal("disabled message mode woke work")
	}
	pending := h.s.state.messages(func(m protocol.TeamMessage) bool { return m.ID == message.ID })
	if len(pending) != 1 || pending[0].Delivered {
		t.Fatal("parked request was falsely marked delivered")
	}
	team.Members[0].Heartbeat = &protocol.MemberHeartbeat{Modes: []string{protocol.HeartbeatMessage}}
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	h.s.clock.rearm()
	if len(heartbeatTasks(h.s, protocol.TaskFromMessage)) != 1 {
		t.Fatal("rearm did not recover the pending addressed request")
	}
	pending = h.s.state.messages(func(m protocol.TeamMessage) bool { return m.ID == message.ID || m.ID == summoner.ID })
	for _, m := range pending {
		if m.Delivered != (m.ID == message.ID) {
			t.Fatal("Summoner request was auto-delivered or accepted manager request stayed pending")
		}
	}
	h.s.clock.stop()
	h.clock.advance(48 * time.Hour)
	h.s.clock.tick()
	h.s.clock.rearm()
	if h.s.clock.timer != nil || len(heartbeatTasks(h.s, protocol.TaskFromMessage)) != 1 {
		t.Fatal("stopped scheduler armed or created work")
	}
	h.s.cancel()
	another := heartbeatMessage(t, h, team, "escalate", team.Members[0].ID, "")
	if _, accepted, err := h.s.wakeMessage(context.Background(), another); err != nil || accepted {
		t.Fatal("shutdown admitted an automatic wake")
	}
}

func TestHeartbeatRoutineAutomationParksButOwnerCanRun(t *testing.T) {
	h, team := heartbeatHarness(t)
	team.Members[0].Heartbeat = &protocol.MemberHeartbeat{Modes: []string{protocol.HeartbeatMessage}}
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	team, _ = h.s.state.team(team.ID)
	routine := do[protocol.Routine](t, h, protocol.MTeamsRoutineCreate, hourly(team, "Owner check", true))
	h.s.checkRoutine(routine.ID, at(11, 0, 0), at(11, 0, 0))
	if len(heartbeatTasks(h.s, protocol.TaskFromRoutine)) != 0 || h.s.Armed() {
		t.Fatal("routine ignored assigned seat's disabled routine mode")
	}
	manual, err := h.s.runRoutine(context.Background(), routine.ID, protocol.InitiatorYou)
	if err != nil || manual.Routine != routine.ID || manual.Automatic {
		t.Fatalf("owner could not explicitly run a parked routine: %+v %v", manual, err)
	}
	team.Members[0].Heartbeat = nil
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	if !h.s.routineAutomaticAllowed(routine) {
		t.Fatal("nil heartbeat did not inherit ordinary routine wakes")
	}
}

func TestHeartbeatDamagedScheduleFailsClosed(t *testing.T) {
	h, team := heartbeatHarness(t)
	heartbeatEntry(t, h, team)
	name := filepath.Join(h.dir, "heartbeats", heartbeatKey(team.ID, team.Members[0].ID)+".json")
	if err := os.WriteFile(name, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.s.clock.refresh(false)
	if !h.s.clock.due(heartbeatKey(team.ID, team.Members[0].ID)).IsZero() || len(heartbeatTasks(h.s, protocol.TaskFromHeartbeat)) != 0 {
		t.Fatal("damaged scheduling state was silently reset")
	}
}

func TestHeartbeatMessageRechecksCurrentManagerAndNeverAcceptsPeerAddress(t *testing.T) {
	for _, scenario := range []string{"manager-changed", "peer-address", "member-paused", "edit-barrier", "global-pause"} {
		t.Run(scenario, func(t *testing.T) {
			h, team := heartbeatHarness(t)
			to := team.Members[0].ID
			if scenario == "peer-address" {
				to = team.Members[1].ID
			}
			message := heartbeatMessage(t, h, team, "escalate", to, "")
			switch scenario {
			case "manager-changed":
				manager := team.Members[1]
				manager.ID, manager.Name = "new-manager", "New manager"
				team.Members[1].ReportsTo = manager.ID
				team.Members = append(team.Members, manager)
			case "member-paused":
				team.Members[0].Paused = true
			case "edit-barrier":
				h.s.editing[team.ID] = map[string]bool{team.Members[0].ID: true}
			case "global-pause":
				h.settings.Teams.Paused = true
			}
			if err := h.s.state.putTeam(team); err != nil {
				t.Fatal(err)
			}
			_, accepted, err := h.s.wakeMessage(context.Background(), message)
			if err != nil || accepted || len(heartbeatTasks(h.s, protocol.TaskFromMessage)) != 0 {
				t.Fatalf("parked or wrongly addressed message woke work: %v %v", accepted, err)
			}
			messages := h.s.state.messages(func(m protocol.TeamMessage) bool { return m.ID == message.ID })
			if len(messages) != 1 || messages[0].Delivered {
				t.Fatal("rejected request was consumed")
			}
		})
	}
}

func TestHeartbeatRecoveryQueuesOneManagerDecisionAtATime(t *testing.T) {
	h, team := heartbeatHarness(t)
	first := heartbeatMessage(t, h, team, "escalate", team.Members[0].ID, "")
	h.clock.advance(time.Second)
	second := heartbeatMessage(t, h, team, "escalate", team.Members[0].ID, "")
	h.s.clock.running = true
	h.s.recoverHeartbeatMessages()
	tasks := heartbeatTasks(h.s, protocol.TaskFromMessage)
	if len(tasks) != 1 || tasks[0].Trigger != first.ID {
		t.Fatal("recovery flooded a manager or ignored oldest-first delivery")
	}
	messages := h.s.state.messages(func(m protocol.TeamMessage) bool { return m.ID == second.ID })
	if len(messages) != 1 || messages[0].Delivered {
		t.Fatal("queued manager decision was discarded")
	}
	task := tasks[0]
	task.Status = protocol.TaskDone
	if err := h.s.state.putTask(task); err != nil {
		t.Fatal(err)
	}
	h.s.clock.rearm()
	tasks = heartbeatTasks(h.s, protocol.TaskFromMessage)
	if len(tasks) != 2 {
		t.Fatal("rearm did not recover the next manager decision")
	}
}
