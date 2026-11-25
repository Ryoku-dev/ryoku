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

	"ryoku-rashin/internal/protocol"
)

type controlledTaskSessions struct {
	*fakeSessions
	blockID string
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	failID  string
	stopErr error
}

func (sessions *controlledTaskSessions) Stop(ctx context.Context, id string) error {
	if id == sessions.blockID {
		sessions.once.Do(func() { close(sessions.entered) })
		select {
		case <-sessions.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if id == sessions.failID {
		return sessions.stopErr
	}
	return sessions.fakeSessions.Stop(ctx, id)
}

func seedControlExecution(t *testing.T, h *harness, task protocol.TeamTask, session string) protocol.TeamTask {
	t.Helper()
	task.Status, task.Session, task.Sessions = protocol.TaskWorking, session, []string{session}
	if err := h.s.state.putTask(task); err != nil {
		t.Fatal(err)
	}
	h.sessions.mu.Lock()
	h.sessions.infos[session] = protocol.SessionInfo{ID: session, Team: task.Team, TeamTask: task.ID, Member: task.Member, State: protocol.StateWorking, Goal: &protocol.Goal{Status: protocol.GoalActive}}
	h.sessions.mu.Unlock()
	h.s.mu.Lock()
	h.s.runs[task.ID] = &run{Task: task.ID, Team: task.Team, Member: task.Member, Session: session, Token: "old-token"}
	h.s.mu.Unlock()
	return task
}

type taskReassignResult struct {
	task protocol.TeamTask
	err  error
}

func beginControlledReassign(t *testing.T, h *harness, task protocol.TeamTask, member string) (*controlledTaskSessions, <-chan taskReassignResult, func()) {
	t.Helper()
	sessions := &controlledTaskSessions{fakeSessions: h.sessions, blockID: task.Session, entered: make(chan struct{}), release: make(chan struct{})}
	h.s.sessions = sessions
	var once sync.Once
	release := func() { once.Do(func() { close(sessions.release) }) }
	t.Cleanup(release)
	result := make(chan taskReassignResult, 1)
	go func() {
		reassigned, err := h.s.taskReassign(context.Background(), protocol.TeamTaskReassignParams{ID: task.ID, Member: member})
		result <- taskReassignResult{task: reassigned, err: err}
	}()
	select {
	case <-sessions.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("reassignment did not reach synchronous Stop")
	}
	return sessions, result, release
}

func receiveReassignment(t *testing.T, result <-chan taskReassignResult) taskReassignResult {
	t.Helper()
	select {
	case result := <-result:
		return result
	case <-time.After(5 * time.Second):
		t.Fatal("reassignment did not finish after Stop returned")
		return taskReassignResult{}
	}
}

func TestTaskReassignmentStopsDescendantsBeforePublishing(t *testing.T) {
	h, team, root := taskControlsFixture(t)
	root = seedControlExecution(t, h, root, "session-root")
	child := cloneTask(root)
	child.ID, child.Parent = "task-child", root.ID
	child = seedControlExecution(t, h, child, "session-child")
	grandchild := cloneTask(child)
	grandchild.ID, grandchild.Parent = "task-grandchild", child.ID
	grandchild = seedControlExecution(t, h, grandchild, "session-grandchild")
	closed := cloneTask(child)
	closed.ID, closed.Status = "task-closed", protocol.TaskDone
	closed.Report = &protocol.TeamReport{Title: "Prior result", Body: "Keep this report", At: root.Created}
	if err := h.s.state.putTask(closed); err != nil {
		t.Fatal(err)
	}
	_, result, release := beginControlledReassign(t, h, root, team.Members[1].ID)
	for _, task := range []protocol.TeamTask{root, child, grandchild} {
		current, _ := h.s.state.task(task.ID)
		if current.Status != protocol.TaskStopped || current.Member != task.Member || current.Ended == nil {
			t.Fatalf("while Stop is delayed task changed assignee or stayed runnable: %+v", current)
		}
		h.s.mu.Lock()
		guard, run := h.s.taskEdits[task.ID], h.s.runs[task.ID]
		h.s.mu.Unlock()
		if !guard || run != nil {
			t.Fatal("old execution retained authority during Stop")
		}
	}
	_, err := h.s.caller(context.Background(), protocol.AgentRef{Session: root.Session, Token: "old-token"})
	controlsErrorCode(t, err, protocol.CodeConflict)
	_, err = h.s.taskReassign(context.Background(), protocol.TeamTaskReassignParams{ID: child.ID, Member: team.Members[1].ID})
	controlsErrorCode(t, err, protocol.CodeConflict)
	refused(t, h, protocol.MTeamsTaskStop, protocol.TeamTaskRef{ID: child.ID}, protocol.CodeConflict)
	refused(t, h, protocol.MTeamsTaskDelete, protocol.TeamTaskRef{ID: child.ID}, protocol.CodeConflict)
	select {
	case early := <-result:
		t.Fatalf("replacement returned before Stop: %+v", early)
	default:
	}
	release()
	finished := receiveReassignment(t, result)
	if finished.err != nil || finished.task.ID != root.ID || finished.task.Member != team.Members[1].ID || finished.task.Status != protocol.TaskReady || finished.task.Goal != root.Goal || finished.task.Parent != root.Parent || !slices.Equal(finished.task.Sessions, root.Sessions) {
		t.Fatalf("replacement = %+v, %v", finished.task, finished.err)
	}
	for _, task := range []protocol.TeamTask{root, child, grandchild} {
		if !h.sessions.wasStopped(task.Session) {
			t.Fatalf("old session %s was not stopped", task.Session)
		}
	}
	if _, ok := h.sessions.goal(root.Session, protocol.GoalStop); !ok {
		t.Fatal("the old execution retained an active goal")
	}
	for _, task := range []protocol.TeamTask{child, grandchild} {
		current, _ := h.s.state.task(task.ID)
		if current.Status != protocol.TaskStopped || current.Parent != task.Parent {
			t.Fatalf("descendant was retained as stale delegated work: %+v", current)
		}
	}
	if current, _ := h.s.state.task(closed.ID); !reflect.DeepEqual(current, closed) {
		t.Fatal("closed descendant history changed")
	}
}

func TestTaskReassignmentRejectsTaskOrRosterChangesDuringStop(t *testing.T) {
	for _, change := range []string{"task", "member", "team-pause", "new-descendant"} {
		t.Run(change, func(t *testing.T) {
			h, team, task := taskControlsFixture(t)
			task = seedControlExecution(t, h, task, "session-root")
			_, result, release := beginControlledReassign(t, h, task, team.Members[1].ID)
			switch change {
			case "task":
				current, _ := h.s.state.task(task.ID)
				current.Brief = "A newer owner edit"
				if err := h.s.state.putTask(current); err != nil {
					t.Fatal(err)
				}
			case "member":
				team.Members[1].Revision++
				if err := h.s.state.putTeam(team); err != nil {
					t.Fatal(err)
				}
			case "team-pause":
				team.PauseReason = "Emergency restriction"
				if err := h.s.state.putTeam(team); err != nil {
					t.Fatal(err)
				}
			case "new-descendant":
				child := cloneTask(task)
				child.ID, child.Parent, child.Status = "task-racing-child", task.ID, protocol.TaskReady
				if err := h.s.state.putTask(child); err != nil {
					t.Fatal(err)
				}
			}
			release()
			finished := receiveReassignment(t, result)
			controlsErrorCode(t, finished.err, protocol.CodeConflict)
			current, _ := h.s.state.task(task.ID)
			if current.Member != task.Member || current.Status != protocol.TaskStopped || (change == "task" && current.Brief != "A newer owner edit") {
				t.Fatalf("stale publication replaced newer state: %+v", current)
			}
		})
	}
}

func TestTaskReassignmentStopErrorDoesNotPublishAndStopsOtherSessions(t *testing.T) {
	h, team, task := taskControlsFixture(t)
	task = seedControlExecution(t, h, task, "session-fails")
	task.Sessions = append(task.Sessions, "session-other")
	if err := h.s.state.putTask(task); err != nil {
		t.Fatal(err)
	}
	h.sessions.mu.Lock()
	h.sessions.infos["session-other"] = protocol.SessionInfo{ID: "session-other", State: protocol.StateWorking}
	h.sessions.mu.Unlock()
	failure := errors.New("old process did not acknowledge stop")
	h.s.sessions = &controlledTaskSessions{fakeSessions: h.sessions, failID: task.Session, stopErr: failure}
	_, err := h.s.taskReassign(context.Background(), protocol.TeamTaskReassignParams{ID: task.ID, Member: team.Members[1].ID})
	if !errors.Is(err, failure) {
		t.Fatalf("Stop error was lost: %v", err)
	}
	current, _ := h.s.state.task(task.ID)
	if current.Member != task.Member || current.Status != protocol.TaskStopped || !h.sessions.wasStopped("session-other") {
		t.Fatalf("uncertain Stop published or skipped another session: %+v", current)
	}
	if len(h.s.state.messages(nil)) != 0 {
		t.Fatal("failed Stop described a completed reassignment")
	}
	h.s.mu.Lock()
	fenced, live := h.s.taskEdits[task.ID], h.s.runs[task.ID]
	h.s.mu.Unlock()
	if !fenced || live != nil {
		t.Fatal("failed Stop released the old execution's revocation barrier")
	}
	refused(t, h, protocol.MTeamsTaskRetry, protocol.TeamTaskRetryParams{ID: task.ID}, protocol.CodeConflict)
}

func TestCompletedTaskReassignmentPreservesActualReportAcrossRestart(t *testing.T) {
	h, team, task := taskControlsFixture(t)
	task.Status = protocol.TaskDone
	task.Session, task.Sessions = "session-old", []string{"session-first", "session-old"}
	task.Plan = &protocol.TeamPlan{Summary: "Prior strategy", Steps: []string{"Inspect", "Report"}, Revision: 7, Approved: true}
	task.Report = &protocol.TeamReport{Title: "Complete output", Body: strings.Repeat("界", maxTeamMessageBody), Files: []string{"result.txt"}, At: task.Created}
	task.Usage.Input = 1 << 54
	if err := h.s.state.putTask(task); err != nil {
		t.Fatal(err)
	}
	reassigned, err := h.s.taskReassign(context.Background(), protocol.TeamTaskReassignParams{ID: task.ID, Member: team.Members[1].ID})
	if err != nil || reassigned.Report != nil || reassigned.Plan != nil || reassigned.ID != task.ID || reassigned.Goal != task.Goal || reassigned.Created != task.Created || !slices.Equal(reassigned.Sessions, task.Sessions) {
		t.Fatalf("completed replacement = %+v, %v", reassigned, err)
	}
	reopened, err := openState(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	messages := reopened.messages(func(message protocol.TeamMessage) bool { return message.Task == task.ID && message.Kind == "reassign" })
	if len(messages) < 2 {
		t.Fatal("large report was not durably chunked")
	}
	slices.SortFunc(messages, func(a, b protocol.TeamMessage) int { return strings.Compare(a.ID, b.ID) })
	var body strings.Builder
	for _, message := range messages {
		if len(message.Body) > maxTeamMessageBody {
			t.Fatal("history exceeded the bounded message size")
		}
		body.WriteString(message.Body)
	}
	var archived protocol.TeamTask
	if err := json.Unmarshal([]byte(body.String()), &archived); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(archived.Report, task.Report) || !reflect.DeepEqual(archived.Plan, task.Plan) || archived.Usage != task.Usage || !slices.Equal(archived.Sessions, task.Sessions) {
		t.Fatalf("actual previous execution was lost: %+v", archived)
	}
}

func TestTaskHistoryAppendIsIdempotentAndIOFailureBlocksReplacement(t *testing.T) {
	h, team, task := taskControlsFixture(t)
	task.Status = protocol.TaskDone
	task.Report = &protocol.TeamReport{Title: "Report", Body: "The actual result", At: task.Created}
	if err := h.s.state.putTask(task); err != nil {
		t.Fatal(err)
	}
	h.s.mu.Lock()
	err := h.s.archiveTaskHistoryLocked(task)
	if err == nil {
		err = h.s.archiveTaskHistoryLocked(task)
	}
	h.s.mu.Unlock()
	if err != nil || len(h.s.state.messages(nil)) != 1 {
		t.Fatalf("history append is not idempotent: %v", err)
	}
	// A new report creates a new event, but an unwritable message directory
	// must not permit runAgain to erase the only copy of that report.
	task.Report.Body = "A subsequent result"
	if err := h.s.state.putTask(task); err != nil {
		t.Fatal(err)
	}
	messageDir := filepath.Join(h.dir, "messages")
	if err := os.Rename(messageDir, messageDir+"-saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(messageDir, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = h.s.taskReassign(context.Background(), protocol.TeamTaskReassignParams{ID: task.ID, Member: team.Members[1].ID})
	if err == nil {
		t.Fatal("history IO error was suppressed")
	}
	current, _ := h.s.state.task(task.ID)
	if current.Member != task.Member || !reflect.DeepEqual(current.Report, task.Report) {
		t.Fatal("failed history write erased report or reassigned")
	}
}

func TestTaskReplacementRejectsStaleAndDeletedSnapshots(t *testing.T) {
	h, _, task := taskControlsFixture(t)
	newer := cloneTask(task)
	newer.Brief = "newer"
	if err := h.s.state.putTask(newer); err != nil {
		t.Fatal(err)
	}
	stale := cloneTask(task)
	stale.Member = "stale-member"
	controlsErrorCode(t, h.s.state.replaceTask(task, stale), protocol.CodeConflict)
	if current, _ := h.s.state.task(task.ID); current.Brief != newer.Brief {
		t.Fatal("stale snapshot overwrote a newer task")
	}
	if err := h.s.state.removeTask(task.ID); err != nil {
		t.Fatal(err)
	}
	controlsErrorCode(t, h.s.state.replaceTask(newer, stale), protocol.CodeConflict)
	if _, exists := h.s.state.task(task.ID); exists {
		t.Fatal("CAS resurrected a removed task")
	}
}

func TestTaskPlanStepsAreDeepCloned(t *testing.T) {
	h, _, task := taskControlsFixture(t)
	task.Plan = &protocol.TeamPlan{Summary: "Plan", Steps: []string{"Keep this"}, Revision: 1}
	if err := h.s.state.putTask(task); err != nil {
		t.Fatal(err)
	}
	task.Plan.Steps[0] = "mutate input"
	first, _ := h.s.state.task(task.ID)
	if first.Plan.Steps[0] != "Keep this" {
		t.Fatal("store retained the input plan slice")
	}
	first.Plan.Steps[0], first.Plan.Approved = "mutate output", true
	second := h.s.state.tasks(nil)[0]
	if second.Plan.Steps[0] != "Keep this" || second.Plan.Approved {
		t.Fatal("task reads shared a plan or its steps")
	}
}

func TestTaskReassignmentRequiresExactCurrentMember(t *testing.T) {
	h, team, task := taskControlsFixture(t)
	for _, member := range []string{"", "absent", team.Members[1].Name} {
		_, err := h.s.taskReassign(context.Background(), protocol.TeamTaskReassignParams{ID: task.ID, Member: member})
		controlsErrorCode(t, err, protocol.CodeNotFound)
	}
	current, _ := h.s.state.task(task.ID)
	if !reflect.DeepEqual(current, task) {
		t.Fatal("invalid assignee stopped or changed the task")
	}
}

func TestTaskReassignmentDeniesOldExecutionApprovals(t *testing.T) {
	h, team, task := taskControlsFixture(t)
	approval := approvalRecord{Approval: protocol.Approval{ID: "approval-old", Team: team.ID, Task: task.ID, Status: protocol.ApprovalPending, Created: task.Created}}
	if err := h.s.state.putApproval(approval); err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.taskReassign(context.Background(), protocol.TeamTaskReassignParams{ID: task.ID, Member: team.Members[1].ID}); err != nil {
		t.Fatal(err)
	}
	decided, ok := h.s.state.approval(approval.ID)
	if !ok || decided.Status != protocol.ApprovalDenied || decided.Decided == nil {
		t.Fatalf("old-session approval survived reassignment: %+v", decided)
	}
	reopened, err := openState(h.dir)
	if err != nil {
		t.Fatal(err)
	}
	if persisted, _ := reopened.approval(approval.ID); persisted.Status != protocol.ApprovalDenied {
		t.Fatal("approval denial was not durable")
	}
}

func TestTaskReassignmentStorageFailureRetainsQuiescenceBarrier(t *testing.T) {
	h, team, task := taskControlsFixture(t)
	task = seedControlExecution(t, h, task, "session-root")
	path := h.s.state.taskPath(task.ID)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	_, err := h.s.taskReassign(context.Background(), protocol.TeamTaskReassignParams{ID: task.ID, Member: team.Members[1].ID})
	if err == nil {
		t.Fatal("stop-state persistence error was suppressed")
	}
	h.s.mu.Lock()
	guard, oldRun := h.s.taskEdits[task.ID], h.s.runs[task.ID]
	h.s.mu.Unlock()
	current, _ := h.s.state.task(task.ID)
	if !guard || oldRun != nil || current.Member != task.Member || !h.sessions.wasStopped(task.Session) {
		t.Fatalf("failed write released execution or published a replacement: %+v", current)
	}
}

func TestTaskRetryPreservesPreviousReportInTimeline(t *testing.T) {
	h, _, task := taskControlsFixture(t)
	task.Status = protocol.TaskDone
	task.Report = &protocol.TeamReport{Title: "Previous report", Body: "Do not erase this output", At: task.Created}
	if err := h.s.state.putTask(task); err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.retryTask(context.Background(), protocol.TeamTaskRetryParams{ID: task.ID}); err != nil {
		t.Fatal(err)
	}
	messages := h.s.state.messages(func(message protocol.TeamMessage) bool { return message.Task == task.ID && message.Kind == "reassign" })
	if len(messages) != 1 {
		t.Fatalf("retry kept %d execution history records, want one", len(messages))
	}
	var archived protocol.TeamTask
	if err := json.Unmarshal([]byte(messages[0].Body), &archived); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(archived.Report, task.Report) {
		t.Fatal("ordinary retry lost its actual previous report")
	}
}
