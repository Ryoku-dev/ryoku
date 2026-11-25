package teams

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func lifecycleArchive(t *testing.T, state *state, task string) protocol.TeamTask {
	t.Helper()
	messages := state.messages(func(message protocol.TeamMessage) bool { return message.Task == task && message.Kind == "reassign" })
	if len(messages) == 0 {
		t.Fatal("the previous execution was not archived")
	}
	slices.SortFunc(messages, func(a, b protocol.TeamMessage) int { return strings.Compare(a.ID, b.ID) })
	var body strings.Builder
	for _, message := range messages {
		body.WriteString(message.Body)
	}
	var archived protocol.TeamTask
	if err := json.Unmarshal([]byte(body.String()), &archived); err != nil {
		t.Fatal(err)
	}
	return archived
}

func TestReassignAccountsFinalTurnBeforeArchivingWithoutOverwritingOwnerEdits(t *testing.T) {
	for _, edit := range []bool{false, true} {
		t.Run(map[bool]string{false: "final usage", true: "newer owner edit"}[edit], func(t *testing.T) {
			h, team, task := taskControlsFixture(t)
			task = seedControlExecution(t, h, task, "session-final")
			_, result, release := beginControlledReassign(t, h, task, team.Members[1].ID)
			info, _ := h.sessions.Info(task.Session)
			info.Steps = 7
			usage := protocol.TokenUsage{Input: 31, Output: 17}
			h.s.AfterTurn(context.Background(), info, protocol.TurnEndData{Usage: &usage})
			if edit {
				current, _ := h.s.state.task(task.ID)
				current.Brief = "Preserve the newer owner request"
				if err := h.s.state.putTask(current); err != nil {
					t.Fatal(err)
				}
			}
			release()
			finished := receiveReassignment(t, result)
			reopened, err := openState(h.dir)
			if err != nil {
				t.Fatal(err)
			}
			if edit {
				controlsErrorCode(t, finished.err, protocol.CodeConflict)
				current, _ := reopened.task(task.ID)
				if current.Brief != "Preserve the newer owner request" || current.Member != task.Member || current.Status != protocol.TaskStopped || current.Turns != 1 || current.Usage != usage {
					t.Fatalf("final accounting overwrote an owner edit: %+v", current)
				}
				return
			}
			if finished.err != nil {
				t.Fatal(finished.err)
			}
			archived := lifecycleArchive(t, reopened, task.ID)
			if archived.Session != task.Session || archived.Member != task.Member || archived.Turns != 1 || archived.Steps != info.Steps || archived.Usage != usage {
				t.Fatalf("final old-session accounting was not archived: %+v", archived)
			}
			restarted := &Service{state: reopened, now: h.clock.now}
			if used, err := restarted.budgetTokens(team.ID, task.Member, h.clock.now()); err != nil || used != 48 {
				t.Fatalf("old seat durable spend = %d, %v", used, err)
			}
			if used, err := restarted.budgetTokens(team.ID, team.Members[1].ID, h.clock.now()); err != nil || used != 0 {
				t.Fatalf("replacement seat was charged for the old execution: %d, %v", used, err)
			}
		})
	}
}

func TestFinalAccountingErrorDuringStopRetainsReplacementBarrier(t *testing.T) {
	h, team, task := taskControlsFixture(t)
	task = seedControlExecution(t, h, task, "session-final-write")
	_, result, release := beginControlledReassign(t, h, task, team.Members[1].ID)
	path := h.s.state.taskPath(task.ID)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	info, _ := h.sessions.Info(task.Session)
	usage := protocol.TokenUsage{Input: 9}
	h.s.AfterTurn(context.Background(), info, protocol.TurnEndData{Usage: &usage})
	h.s.mu.Lock()
	accountingErr := h.s.taskEditErrors[task.ID]
	h.s.mu.Unlock()
	if accountingErr == nil {
		t.Fatal("lost final task accounting did not revoke replacement")
	}
	release()
	finished := receiveReassignment(t, result)
	if !errors.Is(finished.err, accountingErr) {
		t.Fatalf("replacement lost the actual final-write error: %v", finished.err)
	}
	h.s.mu.Lock()
	fenced, live := h.s.taskEdits[task.ID], h.s.runs[task.ID]
	h.s.mu.Unlock()
	current, _ := h.s.state.task(task.ID)
	if !fenced || live != nil || current.Member != task.Member || len(h.s.state.messages(nil)) != 0 {
		t.Fatal("failed final accounting published or archived a runnable replacement")
	}
}

func TestOldDeadlineCannotStopReplacementExecution(t *testing.T) {
	h, team, task := taskControlsFixture(t)
	team.Members[0].Budget = &protocol.TeamBudget{TaskMinutes: 1}
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	started := h.clock.now().Add(-2 * time.Minute)
	task.Started = &started
	task = seedControlExecution(t, h, task, "session-replaced")
	oldToken := "old-token"
	if _, err := h.call(protocol.MTeamsTaskReassign, protocol.TeamTaskReassignParams{ID: task.ID, Member: task.Member}); err != nil {
		t.Fatal(err)
	}
	replacement, _ := h.s.state.task(task.ID)
	replacement.Started = &started
	replacement = seedControlExecution(t, h, replacement, "session-replacement")
	h.s.mu.Lock()
	h.s.runs[task.ID].Token = "replacement-token"
	h.s.mu.Unlock()
	h.s.overrun(task.ID, oldToken)
	current, _ := h.s.state.task(task.ID)
	if !reflect.DeepEqual(current, replacement) || h.sessions.wasStopped(replacement.Session) {
		t.Fatal("an old deadline ended or rewrote the replacement")
	}
}

func TestOwnerStopAndDeleteWaitForCompleteFamilyBelowClosedAncestors(t *testing.T) {
	for _, method := range []string{protocol.MTeamsTaskStop, protocol.MTeamsTaskDelete} {
		t.Run(method, func(t *testing.T) {
			h, _, root := taskControlsFixture(t)
			root = seedControlExecution(t, h, root, "session-root")
			child := cloneTask(root)
			child.ID, child.Parent = "task-middle", root.ID
			child = seedControlExecution(t, h, child, "session-middle")
			leaf := cloneTask(child)
			leaf.ID, leaf.Parent = "task-leaf", child.ID
			leaf = seedControlExecution(t, h, leaf, "session-leaf")
			for _, task := range []protocol.TeamTask{root, child} {
				task.Status = protocol.TaskDone
				if err := h.s.state.putTask(task); err != nil {
					t.Fatal(err)
				}
			}
			for _, task := range []protocol.TeamTask{root, child, leaf} {
				home := paths.ToolHome(task.Team, task.ID)
				if err := os.MkdirAll(home, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(home, "result"), []byte(task.ID), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			sessions := &controlledTaskSessions{fakeSessions: h.sessions, blockID: leaf.Session, entered: make(chan struct{}), release: make(chan struct{})}
			h.s.sessions = sessions
			var once sync.Once
			release := func() { once.Do(func() { close(sessions.release) }) }
			t.Cleanup(release)
			result := make(chan error, 1)
			go func() {
				_, err := h.call(method, protocol.TeamTaskRef{ID: root.ID})
				result <- err
			}()
			select {
			case <-sessions.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("handler did not reach the grandchild's synchronous Stop")
			}
			select {
			case err := <-result:
				t.Fatalf("handler completed before descendant Stop: %v", err)
			default:
			}
			for _, task := range []protocol.TeamTask{root, child, leaf} {
				if _, ok := h.s.state.task(task.ID); !ok {
					t.Fatal("task files disappeared before all descendants stopped")
				}
				if _, err := os.Stat(filepath.Join(paths.ToolHome(task.Team, task.ID), "result")); err != nil {
					t.Fatal("tool output disappeared before all descendants stopped", err)
				}
				h.s.mu.Lock()
				fenced, live := h.s.taskEdits[task.ID], h.s.runs[task.ID]
				h.s.mu.Unlock()
				if !fenced || live != nil {
					t.Fatal("descendant authority survived while Stop was blocked")
				}
			}
			release()
			select {
			case err := <-result:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("handler did not finish after all descendants stopped")
			}
			for _, task := range []protocol.TeamTask{root, child, leaf} {
				if !h.sessions.wasStopped(task.Session) {
					t.Fatalf("descendant session %s was left live", task.Session)
				}
				if _, ok := h.sessions.goal(task.Session, protocol.GoalStop); !ok {
					t.Fatalf("descendant goal %s was left active", task.Session)
				}
				if method == protocol.MTeamsTaskDelete {
					if _, ok := h.s.state.task(task.ID); ok {
						t.Fatal("deleted descendant remained in storage")
					}
					if _, err := os.Stat(paths.ToolHome(task.Team, task.ID)); !os.IsNotExist(err) {
						t.Fatalf("deleted descendant retained tool files: %v", err)
					}
				}
			}
			if method == protocol.MTeamsTaskStop {
				current, _ := h.s.state.task(leaf.ID)
				if current.Status != protocol.TaskStopped {
					t.Fatal("closed ancestors left their descendant runnable")
				}
			}
		})
	}
}

func TestOwnerStopAndDeleteFailuresKeepEveryTaskFenced(t *testing.T) {
	for _, method := range []string{protocol.MTeamsTaskStop, protocol.MTeamsTaskDelete} {
		for _, failure := range []string{"storage", "stop"} {
			t.Run(method+"/"+failure, func(t *testing.T) {
				h, _, root := taskControlsFixture(t)
				root = seedControlExecution(t, h, root, "session-root-fails")
				child := cloneTask(root)
				child.ID, child.Parent = "task-child-survives", root.ID
				child = seedControlExecution(t, h, child, "session-child-stops")
				stopErr := errors.New("process did not acknowledge Stop")
				if failure == "stop" {
					h.s.sessions = &controlledTaskSessions{fakeSessions: h.sessions, failID: root.Session, stopErr: stopErr}
				} else {
					path := h.s.state.taskPath(root.ID)
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(path, 0o700); err != nil {
						t.Fatal(err)
					}
				}
				_, err := h.call(method, protocol.TeamTaskRef{ID: root.ID})
				if err == nil || failure == "stop" && !errors.Is(err, stopErr) {
					t.Fatalf("handler lost the real failure: %v", err)
				}
				if !h.sessions.wasStopped(child.Session) {
					t.Fatal("one failure skipped another descendant's Stop")
				}
				for _, task := range []protocol.TeamTask{root, child} {
					h.s.mu.Lock()
					fenced, live := h.s.taskEdits[task.ID], h.s.runs[task.ID]
					h.s.mu.Unlock()
					if !fenced || live != nil {
						t.Fatal("failure released descendant authority")
					}
					if _, ok := h.s.state.task(task.ID); !ok {
						t.Fatal("failed deletion removed a family record")
					}
				}
				refused(t, h, protocol.MTeamsTaskRetry, protocol.TeamTaskRetryParams{ID: child.ID}, protocol.CodeConflict)
			})
		}
	}
}

func resumeCapsFixture(t *testing.T) (*harness, protocol.Team, protocol.TeamTask) {
	t.Helper()
	h, team, task := taskControlsFixture(t)
	team.Paused, team.Running = false, 1
	team.Members[0].Budget = &protocol.TeamBudget{TaskTurns: 3, TaskMinutes: 2}
	if err := h.s.state.putTeam(team); err != nil {
		t.Fatal(err)
	}
	// A real occupied slot keeps the resumed task queued without pausing its
	// team or bypassing the production resume readiness checks.
	occupier := cloneTask(task)
	occupier.ID = "task-occupier"
	seedControlExecution(t, h, occupier, "session-occupier")
	task = seedControlExecution(t, h, task, "session-standing")
	started := h.clock.now().Add(-time.Minute)
	task.Status, task.Started, task.Turns, task.Automatic = protocol.TaskBlocked, &started, 1, true
	task.Plan = &protocol.TeamPlan{Summary: "Approved strategy", Approved: true, Revision: 1}
	if err := h.s.state.putTask(task); err != nil {
		t.Fatal(err)
	}
	h.s.mu.Lock()
	delete(h.s.runs, task.ID)
	h.s.mu.Unlock()
	return h, team, task
}

func TestResumePreservesAttemptWhileExplicitRetryArchivesAndRefreshesIt(t *testing.T) {
	h, team, task := resumeCapsFixture(t)
	resumed, err := h.s.changeTaskExecution(context.Background(), protocol.TeamTaskReassignParams{ID: task.ID}, "", taskChangeResume)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Status != protocol.TaskReady || resumed.Turns != task.Turns || resumed.Started == nil || !resumed.Started.Equal(*task.Started) || !resumed.Automatic || !reflect.DeepEqual(resumed.Plan, task.Plan) {
		t.Fatalf("resume refreshed the attempt or lost approval/origin: %+v", resumed)
	}
	if left := taskGoalTurns(team, team.Members[0], resumed); left != 2 {
		t.Fatalf("resume granted %d turns instead of the remaining two", left)
	}
	reopened, err := openState(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	archived := lifecycleArchive(t, reopened, task.ID)
	if archived.Turns != task.Turns || archived.Started == nil || !archived.Started.Equal(*task.Started) {
		t.Fatal("resume discarded the old attempt counters from history")
	}
	resumed.Status = protocol.TaskBlocked
	if err := h.s.state.putTask(resumed); err != nil {
		t.Fatal(err)
	}
	retried := do[protocol.TeamTask](t, h, protocol.MTeamsTaskRetry, protocol.TeamTaskRetryParams{ID: task.ID})
	if retried.Turns != 0 || retried.Started != nil || retried.Automatic || retried.Plan != nil {
		t.Fatalf("explicit retry failed to start a fresh archived attempt: %+v", retried)
	}
}

func TestResumeRefusesAlreadyExhaustedAttemptWithoutChangingStorage(t *testing.T) {
	for _, cap := range []string{"turns", "minutes"} {
		t.Run(cap, func(t *testing.T) {
			h, _, task := resumeCapsFixture(t)
			if cap == "turns" {
				task.Turns = 3
			} else {
				started := h.clock.now().Add(-2 * time.Minute)
				task.Started = &started
			}
			if err := h.s.state.putTask(task); err != nil {
				t.Fatal(err)
			}
			_, err := h.s.changeTaskExecution(context.Background(), protocol.TeamTaskReassignParams{ID: task.ID}, "", taskChangeResume)
			controlsErrorCode(t, err, protocol.CodeConflict)
			current, _ := h.s.state.task(task.ID)
			if !reflect.DeepEqual(current, task) || len(h.s.state.messages(nil)) != 0 {
				t.Fatal("exhausted resume changed storage or reset its attempt")
			}
		})
	}
}

func TestResumeRechecksLimitsAfterQuiescentFinalTurn(t *testing.T) {
	for _, cap := range []string{"turns", "minutes"} {
		t.Run(cap, func(t *testing.T) {
			h, _, task := resumeCapsFixture(t)
			task.Turns = 2
			if err := h.s.state.putTask(task); err != nil {
				t.Fatal(err)
			}
			sessions := &controlledTaskSessions{fakeSessions: h.sessions, blockID: task.Session, entered: make(chan struct{}), release: make(chan struct{})}
			h.s.sessions = sessions
			var once sync.Once
			release := func() { once.Do(func() { close(sessions.release) }) }
			t.Cleanup(release)
			result := make(chan taskReassignResult, 1)
			go func() {
				resumed, err := h.s.changeTaskExecution(context.Background(), protocol.TeamTaskReassignParams{ID: task.ID}, "", taskChangeResume)
				result <- taskReassignResult{task: resumed, err: err}
			}()
			select {
			case <-sessions.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("resume did not wait for the old session")
			}
			if cap == "turns" {
				info, _ := h.sessions.Info(task.Session)
				usage := protocol.TokenUsage{Input: 1}
				h.s.AfterTurn(context.Background(), info, protocol.TurnEndData{Usage: &usage})
			} else {
				h.clock.set(task.Started.Add(2 * time.Minute))
			}
			release()
			finished := receiveReassignment(t, result)
			controlsErrorCode(t, finished.err, protocol.CodeConflict)
			current, _ := h.s.state.task(task.ID)
			h.s.mu.Lock()
			fenced := h.s.taskEdits[task.ID]
			h.s.mu.Unlock()
			if taskIsOpen(current) || !fenced || current.Session != task.Session || current.Started == nil || !current.Started.Equal(*task.Started) {
				t.Fatalf("Stop refreshed an exhausted attempt: %+v", current)
			}
			if cap == "turns" && current.Turns != 3 {
				t.Fatal("the final turn was lost instead of exhausting the attempt")
			}
			if len(h.s.state.messages(nil)) != 0 {
				t.Fatal("a refused resume archived or published a new attempt")
			}
		})
	}
}

func TestReassignmentCannotConfirmStopWithoutSessionManager(t *testing.T) {
	h, team, task := taskControlsFixture(t)
	task = seedControlExecution(t, h, task, "session-unavailable-manager")
	h.s.sessions = nil
	_, err := h.call(protocol.MTeamsTaskReassign, protocol.TeamTaskReassignParams{ID: task.ID, Member: team.Members[1].ID})
	controlsErrorCode(t, err, protocol.CodeConflict)
	current, _ := h.s.state.task(task.ID)
	h.s.mu.Lock()
	fenced, live := h.s.taskEdits[task.ID], h.s.runs[task.ID]
	h.s.mu.Unlock()
	if !fenced || live != nil || current.Member != task.Member || taskIsOpen(current) || len(h.s.state.messages(nil)) != 0 {
		t.Fatal("an unavailable session manager was treated as a successful Stop")
	}
}
