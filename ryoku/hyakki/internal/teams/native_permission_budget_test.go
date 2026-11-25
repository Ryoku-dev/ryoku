package teams

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

func TestNativePermissionWaitDoesNotSpendTaskMinutes(t *testing.T) {
	h, team := governedTeam(t)
	team.Budget.TaskMinutes = 1
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	task := h.newTask(t, team, "Ask before changing the assigned file")
	ref := h.agent(t, task.ID)
	current := h.task(t, task.ID)
	if current.Started == nil {
		t.Fatal("the actual controller never started the task")
	}
	h.clock.mu.Lock()
	h.clock.frozen = true
	h.clock.at = *current.Started
	h.clock.mu.Unlock()
	h.clock.advance(10 * time.Second)
	info := h.sessions.info(ref.Session)
	info.State, info.Pending = protocol.StateWaiting, 1
	h.s.SessionChanged(info)
	h.clock.advance(3 * time.Minute)
	info.State, info.Pending = protocol.StateWorking, 0
	h.s.SessionChanged(info)
	current = h.task(t, task.ID)
	remaining := h.s.minutesEnd(current, 1).Sub(h.clock.now())
	if remaining != 50*time.Second {
		t.Fatalf("native permission wait spent active task budget: remaining=%s, want 50s", remaining)
	}
	h.s.mu.Lock()
	run := h.s.runs[task.ID]
	token := ""
	if run != nil {
		token = run.Token
	}
	h.s.mu.Unlock()
	if token == "" {
		t.Fatal("the owner's permission wait retired the task execution")
	}
	h.s.overrun(task.ID, token)
	if got := h.task(t, task.ID).Status; got != protocol.TaskWorking {
		t.Fatalf("permission answer immediately exhausted the task: %s", got)
	}
	deadline := h.s.minutesEnd(current, 1)
	h.clock.set(deadline.Add(-time.Nanosecond))
	h.s.overrun(task.ID, token)
	if got := h.task(t, task.ID).Status; got != protocol.TaskWorking {
		t.Fatalf("task stopped before its active-time boundary: %s", got)
	}
	h.clock.set(deadline)
	h.s.overrun(task.ID, token)
	if got := h.task(t, task.ID).Status; got != protocol.TaskBlocked {
		t.Fatalf("permission wait disabled the finite active-time cap: %s", got)
	}
}

func TestNativePermissionWaitSurvivesModelEventsAndTaskReopen(t *testing.T) {
	h, team := governedTeam(t)
	team.Budget.TaskMinutes = 1
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	task := h.newTask(t, team, "Wait for a reviewed local change")
	ref := h.agent(t, task.ID)
	current := h.task(t, task.ID)
	h.clock.mu.Lock()
	h.clock.frozen = true
	h.clock.at = *current.Started
	h.clock.mu.Unlock()
	h.clock.advance(10 * time.Second)
	info := h.sessions.info(ref.Session)
	info.State, info.Pending = protocol.StateWaiting, 1
	h.s.SessionChanged(info)
	h.clock.advance(30 * time.Second)
	info.Model = "test/switched"
	h.s.SessionChanged(info)
	h.s.SessionChanged(info)
	h.clock.advance(2 * time.Minute)
	info.State, info.Pending = protocol.StateWorking, 0
	h.s.SessionChanged(info)
	reopened, err := openState(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	restored, ok := reopened.task(task.ID)
	if !ok {
		t.Fatal("persisted task disappeared")
	}
	if remaining := h.s.minutesEnd(restored, 1).Sub(h.clock.now()); remaining != 50*time.Second {
		t.Fatalf("model events or reopening spent the owner's wait: remaining=%s", remaining)
	}
}

func TestStandingResumeRetainsNativePermissionWaitWithoutRefreshingTheCap(t *testing.T) {
	h, team, task := resumeCapsFixture(t)
	now := h.clock.now()
	h.clock.mu.Lock()
	h.clock.frozen = true
	h.clock.at = now
	h.clock.mu.Unlock()
	started := now.Add(-4 * time.Minute)
	task.Started = &started
	task.PermissionWaits = []protocol.TaskPermissionWait{{From: started.Add(10 * time.Second), Until: started.Add(3*time.Minute + 10*time.Second)}}
	if err := h.s.state.putTask(task); err != nil {
		t.Fatal(err)
	}
	resumed, err := h.s.changeTaskExecution(context.Background(), protocol.TeamTaskReassignParams{ID: task.ID}, "", taskChangeResume)
	if err != nil {
		t.Fatal(err)
	}
	if remaining := h.s.minutesEnd(resumed, 2).Sub(h.clock.now()); remaining != time.Minute {
		t.Fatalf("standing resume lost the owner's wait or refreshed active time: remaining=%s", remaining)
	}
	if h.s.taskAttemptExhausted(team, team.Members[0], resumed) {
		t.Fatal("resumed task was exhausted before its remaining active minute")
	}
	h.clock.advance(time.Minute)
	if !h.s.taskAttemptExhausted(team, team.Members[0], resumed) {
		t.Fatal("native permission history granted extra active time after standing resume")
	}
}

func TestRecoveryReleasesNativePermissionWaitWithoutRefreshingActiveBudget(t *testing.T) {
	h, team := governedTeam(t)
	team.Budget.TaskMinutes = 1
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	task := h.newTask(t, team, "Resume only the remaining active time after recovery")
	ref := h.agent(t, task.ID)
	current := h.task(t, task.ID)
	h.clock.mu.Lock()
	h.clock.frozen = true
	h.clock.at = *current.Started
	h.clock.mu.Unlock()
	h.clock.advance(10 * time.Second)
	info := h.sessions.info(ref.Session)
	info.State, info.Pending = protocol.StateWaiting, 1
	h.s.SessionChanged(info)
	h.clock.advance(3 * time.Minute)
	reopened, err := openState(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	h.s.mu.Lock()
	for _, running := range h.s.runs {
		if running.timer != nil {
			running.timer.Stop()
		}
	}
	h.s.runs = make(map[string]*run)
	h.s.state = reopened
	h.s.mu.Unlock()
	h.s.recoverRuns()
	current = h.task(t, task.ID)
	if current.Status != protocol.TaskWorking || current.Waiting != "" {
		t.Fatalf("surviving active goal did not recover: %+v", current)
	}
	h.clock.advance(50 * time.Second)
	h.s.mu.Lock()
	running := h.s.runs[task.ID]
	token := ""
	if running != nil {
		token = running.Token
	}
	h.s.mu.Unlock()
	h.s.overrun(task.ID, token)
	if got := h.task(t, task.ID).Status; got != protocol.TaskBlocked {
		t.Fatalf("recovery kept crediting permission time during actual work: %s", got)
	}
}

// These fixtures drive the same controller, delegation RPC and native session
// hooks as a real run; only the clock and session runner are deterministic.
func delegatedMinutesFixture(t *testing.T) (*harness, protocol.Team, protocol.TeamTask, protocol.AgentRef, protocol.TeamTask, protocol.AgentRef) {
	t.Helper()
	h, team := governedTeam(t)
	team.Budget.TaskMinutes = 1
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	h.clock.mu.Lock()
	h.clock.frozen = true
	h.clock.mu.Unlock()
	root := h.newTask(t, team, "Finish within one active minute, excluding owner decisions")
	rootRef := h.agent(t, root.ID)
	h.clock.advance(10 * time.Second)
	child := governedPiece(t, h, root, rootRef, team.Members[1].ID)
	return h, team, h.task(t, root.ID), rootRef, child, h.agent(t, child.ID)
}

func TestDescendantNativePermissionProtectsAncestorRemainingMinutes(t *testing.T) {
	h, team, root, rootRef, child, childRef := delegatedMinutesFixture(t)
	leaf := governedPiece(t, h, child, childRef, team.Members[2].ID)
	leafRef := h.agent(t, leaf.ID)
	info := h.sessions.info(leafRef.Session)
	info.State, info.Pending = protocol.StateWaiting, 1
	h.s.SessionChanged(info)
	h.clock.advance(3 * time.Minute)
	for _, execution := range []struct {
		task string
		ref  protocol.AgentRef
	}{{root.ID, rootRef}, {child.ID, childRef}} {
		h.s.overrun(execution.task, execution.ref.Token)
		if got := h.task(t, execution.task); !taskIsOpen(got) {
			t.Fatalf("a delegated native owner wait exhausted an ancestor: %+v", got)
		}
	}
	info.State, info.Pending = protocol.StateWorking, 0
	h.s.SessionChanged(info)
	current := h.task(t, root.ID)
	if current.Started == nil || !current.Started.Equal(*root.Started) {
		t.Fatal("owner wait refreshed the ancestor's original start")
	}
	if left := h.s.minutesEnd(current, 1).Sub(h.clock.now()); left != 50*time.Second {
		t.Fatalf("ancestor did not resume its original remaining budget: %s", left)
	}
	h.clock.advance(50*time.Second - time.Nanosecond)
	h.s.overrun(root.ID, rootRef.Token)
	if !taskIsOpen(h.task(t, root.ID)) {
		t.Fatal("ancestor ended before its remaining active time")
	}
	h.clock.advance(time.Nanosecond)
	h.s.overrun(root.ID, rootRef.Token)
	if got := h.task(t, root.ID).Status; got != protocol.TaskBlocked {
		t.Fatalf("delegated wait disabled the ancestor's finite cap: %s", got)
	}
}

func TestOwnerWaitDoesNotProtectActiveSiblingOrMemberHandoffAlone(t *testing.T) {
	t.Run("active sibling", func(t *testing.T) {
		h, team, root, rootRef, child, childRef := delegatedMinutesFixture(t)
		sibling := governedPiece(t, h, root, rootRef, team.Members[3].ID)
		siblingRef := h.agent(t, sibling.ID)
		info := h.sessions.info(childRef.Session)
		info.State, info.Pending = protocol.StateWaiting, 1
		h.s.SessionChanged(info)
		h.clock.advance(time.Minute)
		h.s.overrun(sibling.ID, siblingRef.Token)
		if got := h.task(t, sibling.ID).Status; got != protocol.TaskBlocked {
			t.Fatalf("another branch's owner wait suspended active work: %s", got)
		}
		h.s.overrun(root.ID, rootRef.Token)
		if !taskIsOpen(h.task(t, root.ID)) || h.task(t, child.ID).Waiting != protocol.WaitingOnPermission {
			t.Fatal("the genuine descendant owner hold did not protect its ancestor")
		}
	})
	t.Run("member handoff", func(t *testing.T) {
		h, _, root, rootRef, _, _ := delegatedMinutesFixture(t)
		h.clock.advance(50 * time.Second)
		h.s.overrun(root.ID, rootRef.Token)
		if got := h.task(t, root.ID).Status; got != protocol.TaskBlocked {
			t.Fatalf("WaitingOnMembers alone suspended the finite ancestor cap: %s", got)
		}
	})
}

func TestDelegatedOwnerWaitUnionIncludesGovernanceWithoutDoubleCredit(t *testing.T) {
	h, team, root, rootRef, _, childRef := delegatedMinutesFixture(t)
	sibling := governedPiece(t, h, root, rootRef, team.Members[3].ID)
	siblingRef := h.agent(t, sibling.ID)
	info := h.sessions.info(childRef.Session)
	info.State, info.Pending = protocol.StateWaiting, 1
	h.s.SessionChanged(info) // Native hold: t=10 through t=90.
	h.clock.advance(20 * time.Second)
	h.answer(t, protocol.MTeamsAgentAsk, protocol.AgentAskParams{AgentRef: siblingRef, Title: "Choose the reviewed local action"})
	approval := h.pending(t) // Governance hold: t=30 through t=120.
	h.clock.advance(time.Minute)
	info.State, info.Pending = protocol.StateWorking, 0
	h.s.SessionChanged(info)
	h.clock.advance(30 * time.Second)
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: approval.ID, Decision: protocol.DecideOnce})
	current := h.task(t, root.ID)
	if left := h.s.minutesEnd(current, 1).Sub(h.clock.now()); left != 50*time.Second {
		t.Fatalf("overlapping descendant native and governance waits were not one union: %s", left)
	}
	h.clock.advance(50 * time.Second)
	h.s.overrun(root.ID, rootRef.Token)
	if h.task(t, root.ID).Status != protocol.TaskBlocked {
		t.Fatal("overlapping owner waits granted extra active time")
	}
}

func TestDelegatedWaitHistorySurvivesTerminalRemovalAndReassignment(t *testing.T) {
	for _, change := range []string{"terminal", "delete", "reassign", "prune"} {
		t.Run(change, func(t *testing.T) {
			h, _, root, _, child, childRef := delegatedMinutesFixture(t)
			info := h.sessions.info(childRef.Session)
			info.State, info.Pending = protocol.StateWaiting, 1
			h.s.SessionChanged(info)
			h.clock.advance(3 * time.Minute)
			switch change {
			case "terminal":
				if _, err := h.s.stopTask(context.Background(), child.ID, "Owner stopped the delegated execution."); err != nil {
					t.Fatal(err)
				}
			case "delete":
				if err := h.s.deleteTask(context.Background(), child.ID); err != nil {
					t.Fatal(err)
				}
			case "reassign":
				if _, err := h.s.taskReassign(context.Background(), protocol.TeamTaskReassignParams{ID: child.ID, Member: child.Member}); err != nil {
					t.Fatal(err)
				}
				// An old native session event must not create an open hold
				// in the replacement attempt or its ancestor.
				h.s.SessionChanged(info)
			case "prune":
				if _, err := h.s.stopTask(context.Background(), child.ID, "The completed permission wait enters history."); err != nil {
					t.Fatal(err)
				}
				if err := h.s.state.removeTask(child.ID); err != nil {
					t.Fatal(err)
				}
			}
			// Ending a child can still resume its parent asynchronously. Join
			// the old service before opening its durable state as a restart.
			restartHeartbeatHarness(t, h)
			current := h.task(t, root.ID)
			if left := h.s.minutesEnd(current, 1).Sub(h.clock.now()); left != 50*time.Second {
				t.Fatalf("%s lost earned descendant wait history across restart: %s", change, left)
			}
			h.clock.advance(50 * time.Second)
			if !h.s.taskAttemptExhausted(protocol.Team{Budget: protocol.TeamBudget{TaskMinutes: 1}}, protocol.Member{}, h.task(t, root.ID)) {
				t.Fatalf("%s left an unlimited open descendant credit", change)
			}
		})
	}
}

func TestFreshAncestorAttemptCannotInheritOldDescendantWaits(t *testing.T) {
	h, _, root, _, _, childRef := delegatedMinutesFixture(t)
	info := h.sessions.info(childRef.Session)
	info.State, info.Pending = protocol.StateWaiting, 1
	h.s.SessionChanged(info)
	h.clock.advance(3 * time.Minute)
	replacement, err := h.s.taskReassign(context.Background(), protocol.TeamTaskReassignParams{ID: root.ID, Member: root.Member})
	if err != nil {
		t.Fatal(err)
	}
	replacementRef := h.agent(t, replacement.ID)
	h.s.SessionChanged(info)
	h.clock.advance(time.Minute)
	h.s.overrun(replacement.ID, replacementRef.Token)
	if got := h.task(t, replacement.ID).Status; got != protocol.TaskBlocked {
		t.Fatalf("an old descendant execution credited a fresh ancestor attempt: %s", got)
	}
}

func TestRecoveryClosesDescendantNativeWaitAndKeepsAncestorFinite(t *testing.T) {
	h, _, root, _, _, childRef := delegatedMinutesFixture(t)
	info := h.sessions.info(childRef.Session)
	info.State, info.Pending = protocol.StateWaiting, 1
	h.s.SessionChanged(info)
	h.clock.advance(3 * time.Minute)
	reopened, err := openState(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	h.s.mu.Lock()
	for _, running := range h.s.runs {
		if running.timer != nil {
			running.timer.Stop()
		}
	}
	h.s.runs = make(map[string]*run)
	h.s.state = reopened
	h.s.mu.Unlock()
	h.s.recoverRuns()
	current := h.task(t, root.ID)
	if left := h.s.minutesEnd(current, 1).Sub(h.clock.now()); left != 50*time.Second {
		t.Fatalf("recovery lost the descendant owner delay: %s", left)
	}
	h.clock.advance(50 * time.Second)
	h.s.mu.Lock()
	running := h.s.runs[root.ID]
	token := ""
	if running != nil {
		token = running.Token
	}
	h.s.mu.Unlock()
	if token == "" {
		t.Fatal("the ancestor's active execution was not recovered")
	}
	h.s.overrun(root.ID, token)
	if h.task(t, root.ID).Status != protocol.TaskBlocked {
		t.Fatal("recovery kept crediting a descendant's vanished permission card")
	}
}

func TestAncestorWaitHistoryFoldFailsBeforeSourceHistoryIsLost(t *testing.T) {
	for _, change := range []string{"remove", "replace"} {
		t.Run(change, func(t *testing.T) {
			h, _, root, _, child, childRef := delegatedMinutesFixture(t)
			info := h.sessions.info(childRef.Session)
			info.State, info.Pending = protocol.StateWaiting, 1
			h.s.SessionChanged(info)
			h.clock.advance(3 * time.Minute)
			if _, err := h.s.stopTask(context.Background(), child.ID, "Preserve this execution's wait history."); err != nil {
				t.Fatal(err)
			}
			previous := h.task(t, child.ID)
			var err error
			func() {
				// Match owner commits: parent continuations must not consume
				// the one-shot directory fault intended for this history fold.
				h.s.mu.Lock()
				defer h.s.mu.Unlock()
				installTransactionFaults(h.s.state, filepath.Join(h.s.state.dir, "tasks"), "rollback")
				if change == "remove" {
					err = h.s.state.removeTask(child.ID)
				} else {
					replacement := cloneTask(previous)
					runAgain(&replacement, h.clock.now())
					err = h.s.state.replaceTask(previous, replacement)
				}
			}()
			if err == nil {
				t.Fatal("source history was destroyed despite a failed ancestor history write")
			}
			current, exists := h.s.state.task(child.ID)
			if !exists || current.Session != previous.Session || current.Started == nil || current.Status != previous.Status {
				t.Fatal("failed history folding removed or replaced the source execution")
			}
			if left := h.s.minutesEnd(h.task(t, root.ID), 1).Sub(h.clock.now()); left != 50*time.Second {
				t.Fatalf("failed history fold lost the original retained source's earned credit: %s", left)
			}
		})
	}
}

func TestApprovalRetentionCannotEraseDelegatedOwnerWait(t *testing.T) {
	h, _, root, rootRef, child, childRef := delegatedMinutesFixture(t)
	h.answer(t, protocol.MTeamsAgentAsk, protocol.AgentAskParams{AgentRef: childRef, Title: "Approve the delegated action"})
	approval := h.pending(t)
	h.clock.advance(3 * time.Minute)
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: approval.ID, Decision: protocol.DecideOnce})
	h.clock.advance(time.Second)
	now := h.clock.now()
	// Seed the unrelated newer history in memory, then exercise the real
	// bounded persistence/pruning consumer in one transaction.
	h.s.state.mu.Lock()
	var newest approvalRecord
	for range keepDecided {
		newest = approvalRecord{Approval: protocol.Approval{ID: newID("approval"), Status: protocol.ApprovalApproved, Created: now, Decided: &now}}
		h.s.state.approvalSet[newest.ID] = newest
	}
	h.s.state.mu.Unlock()
	if err := h.s.state.putApproval(newest); err != nil {
		t.Fatal(err)
	}
	if _, exists := h.s.state.approval(approval.ID); exists {
		t.Fatal("the bounded approval consumer did not prune the oldest decision")
	}
	if err := h.s.deleteTask(context.Background(), child.ID); err != nil {
		t.Fatal(err)
	}
	reopened, err := openState(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	h.s.mu.Lock()
	h.s.state = reopened
	h.s.mu.Unlock()
	if left := h.s.minutesEnd(h.task(t, root.ID), 1).Sub(h.clock.now()); left != 49*time.Second {
		t.Fatalf("approval pruning or source deletion erased delegated decision time: %s", left)
	}
	h.clock.advance(49 * time.Second)
	h.s.overrun(root.ID, rootRef.Token)
	if h.task(t, root.ID).Status != protocol.TaskBlocked {
		t.Fatal("retained governance wait disabled the ancestor's active-time cap")
	}
}

func TestFailedNativeWaitClosureCannotGrantUnlimitedAncestorCredit(t *testing.T) {
	for _, waited := range []time.Duration{0, 3 * time.Minute} {
		t.Run(waited.String(), func(t *testing.T) {
			h, _, root, rootRef, child, childRef := delegatedMinutesFixture(t)
			info := h.sessions.info(childRef.Session)
			info.State, info.Pending = protocol.StateWaiting, 1
			h.s.SessionChanged(info)
			h.clock.advance(waited)
			installTransactionFaults(h.s.state, filepath.Join(h.s.state.dir, "tasks"), "rollback")
			info.State, info.Pending = protocol.StateWorking, 0
			h.s.SessionChanged(info)
			h.s.mu.Lock()
			fenced, running := h.s.taskEdits[child.ID], h.s.runs[child.ID]
			h.s.mu.Unlock()
			if !fenced || running != nil {
				t.Fatal("failed native wait history publication left its execution authorized")
			}
			// Failed publication must retain the actual wait through revocation,
			// without continuing to add credit while the old runner is stopped.
			if left := h.s.minutesEnd(h.task(t, root.ID), 1).Sub(h.clock.now()); left != 50*time.Second {
				t.Fatalf("failed wait closure lost earned human-delay credit: %s", left)
			}
			h.clock.advance(50 * time.Second)
			h.s.overrun(root.ID, rootRef.Token)
			if h.task(t, root.ID).Status != protocol.TaskBlocked {
				t.Fatal("a fenced stale native wait kept adding inherited active-time credit")
			}
			reopened, err := openState(h.dir)
			if err != nil {
				t.Fatal(err)
			}
			h.s.mu.Lock()
			h.s.state = reopened
			h.s.mu.Unlock()
			if end := h.s.minutesEnd(h.task(t, root.ID), 1); !end.Equal(h.clock.now()) {
				t.Fatalf("restart let a stale descendant open hold revise an ended ancestor's earned budget: %s", end)
			}
		})
	}
}

func TestRecoveryClosesNativeWaitEvenUnderSeparateGovernanceHold(t *testing.T) {
	h, _, root, rootRef, child, childRef := delegatedMinutesFixture(t)
	info := h.sessions.info(childRef.Session)
	info.State, info.Pending = protocol.StateWaiting, 1
	h.s.SessionChanged(info)
	h.answer(t, protocol.MTeamsAgentAsk, protocol.AgentAskParams{AgentRef: childRef, Title: "Choose the reviewed action"})
	approval := h.pending(t)
	h.clock.advance(3 * time.Minute)
	reopened, err := openState(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	h.s.mu.Lock()
	for _, running := range h.s.runs {
		if running.timer != nil {
			running.timer.Stop()
		}
	}
	h.s.runs = make(map[string]*run)
	h.s.state = reopened
	h.s.mu.Unlock()
	h.s.recoverRuns()
	recovered := h.task(t, child.ID)
	if recovered.Waiting != protocol.WaitingOnYou {
		t.Fatal("recovery released a genuine pending governance decision")
	}
	for _, span := range recovered.PermissionWaits {
		if span.Until.IsZero() {
			t.Fatal("a separate governance hold preserved an unknown open native wait")
		}
	}
	do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: approval.ID, Decision: protocol.DecideOnce})
	h.s.mu.Lock()
	if running := h.s.runs[root.ID]; running != nil {
		rootRef.Token = running.Token
	}
	h.s.mu.Unlock()
	h.clock.advance(50 * time.Second)
	h.s.overrun(root.ID, rootRef.Token)
	if h.task(t, root.ID).Status != protocol.TaskBlocked {
		t.Fatal("an unknown native hold survived the governance answer and disabled the ancestor cap")
	}
}

func TestPreSessionStrategyWaitProtectsOnlyAlreadyStartedAncestors(t *testing.T) {
	h, team, root, rootRef, child, _ := delegatedMinutesFixture(t)
	team.PlanApproval = true
	team.Members[3].Runtime = &protocol.MemberRuntime{Kind: protocol.MemberRuntimeCommand, Command: []string{"/bin/true"}}
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	parked, err := h.s.taskReassign(context.Background(), protocol.TeamTaskReassignParams{ID: child.ID, Member: team.Members[3].ID})
	if err != nil {
		t.Fatal(err)
	}
	if parked.Session != "" || parked.Started != nil || parked.Waiting != protocol.WaitingOnYou {
		t.Fatalf("the foreign replacement did not park before session admission: %+v", parked)
	}
	h.clock.advance(3 * time.Minute)
	h.s.overrun(root.ID, rootRef.Token)
	if !taskIsOpen(h.task(t, root.ID)) {
		t.Fatal("a genuine pre-session delegated strategy wait exhausted the ancestor")
	}
	do[protocol.TeamTask](t, h, protocol.MTeamsTaskPlan, protocol.TeamTaskPlanParams{ID: child.ID, Summary: "Run only the configured local command", Steps: []string{"Run the configured argv", "Report its observed result"}})
	h.agent(t, child.ID)
	current := h.task(t, root.ID)
	if left := h.s.minutesEnd(current, 1).Sub(h.clock.now()); left != 50*time.Second {
		t.Fatalf("starting the approved foreign session lost its ancestor's wait credit: %s", left)
	}
	if left := h.s.minutesEnd(h.task(t, child.ID), 1).Sub(h.clock.now()); left != time.Minute {
		t.Fatalf("pre-session owner wait granted extra time to an unstarted child: %s", left)
	}
	h.clock.advance(50 * time.Second)
	h.s.overrun(root.ID, rootRef.Token)
	if h.task(t, root.ID).Status != protocol.TaskBlocked {
		t.Fatal("pre-session strategy approval disabled the finite ancestor cap")
	}
}
