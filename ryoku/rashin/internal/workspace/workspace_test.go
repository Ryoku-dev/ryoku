package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

type fakeBroadcaster struct {
	mu     sync.Mutex
	events []string
}

func (f *fakeBroadcaster) Broadcast(event string, _ any) {
	f.mu.Lock()
	f.events = append(f.events, event)
	f.mu.Unlock()
}

func eventCount(events *fakeBroadcaster, event string) int {
	events.mu.Lock()
	defer events.mu.Unlock()
	count := 0
	for _, current := range events.events {
		if current == event {
			count++
		}
	}
	return count
}

func waitForEvent(t *testing.T, events *fakeBroadcaster, event string, after int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for eventCount(events, event) <= after && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if eventCount(events, event) <= after {
		t.Fatalf("event %s was not broadcast", event)
	}
}

type fakeSessions struct {
	mu      sync.Mutex
	infos   map[string]protocol.SessionInfo
	prompts []protocol.PromptParams
	next    int
}

func newFakeSessions() *fakeSessions {
	return &fakeSessions{infos: make(map[string]protocol.SessionInfo)}
}

func (f *fakeSessions) Info(id string) (protocol.SessionInfo, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	info, ok := f.infos[id]
	return info, ok
}

func (f *fakeSessions) List(kind protocol.SessionKind) []protocol.SessionInfo {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []protocol.SessionInfo
	for _, info := range f.infos {
		if info.Kind == kind {
			result = append(result, info)
		}
	}
	return result
}

func (f *fakeSessions) New(_ context.Context, params protocol.SessionNewParams) (protocol.SessionInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	now := time.Now()
	info := protocol.SessionInfo{
		ID: "agent-test-" + intString(f.next), Kind: params.Kind, Harness: params.Harness,
		Cwd: params.Cwd, Project: params.Project, Worktree: params.Worktree, Task: params.Task,
		Title: params.Title, State: protocol.StateIdle, Created: now, Updated: now,
	}
	f.infos[info.ID] = info
	return info, nil
}

func (f *fakeSessions) Prompt(_ context.Context, params protocol.PromptParams) (protocol.PromptResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prompts = append(f.prompts, params)
	info := f.infos[params.ID]
	info.State = protocol.StateWorking
	info.Updated = time.Now()
	info.Turns++
	f.infos[params.ID] = info
	return protocol.PromptResult{Turn: info.Turns}, nil
}

type fakeStream struct {
	parts []string
	index int
	final llm.Final
}

func (f *fakeStream) Next() (llm.Delta, error) {
	if f.index >= len(f.parts) {
		return llm.Delta{}, io.EOF
	}
	part := f.parts[f.index]
	f.index++
	return llm.Delta{Text: part}, nil
}
func (f *fakeStream) Final() llm.Final { return f.final }
func (f *fakeStream) Close() error     { return nil }

type fakeClient struct{ text string }

func (f fakeClient) Stream(_ context.Context, _ llm.Request) (llm.Stream, error) {
	return &fakeStream{parts: []string{f.text}, final: llm.Final{Provider: "free", Model: "planner"}}, nil
}

func testWork(t *testing.T, client llm.Client) (*Work, string, *fakeSessions) {
	t.Helper()
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", filepath.Join(state, "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(state, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(state, "cache"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(state, "config"))
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	repo := filepath.Join(state, "repo")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.name", "Workspace Test")
	runGit(t, repo, "config", "user.email", "workspace@example.invalid")
	if err := os.WriteFile(filepath.Join(repo, ".gitignore"), []byte(".env\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "file.txt"), []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "initial")
	work, err := Open(&fakeBroadcaster{}, client, func() protocol.Settings {
		return protocol.Settings{Ask: protocol.AskSettings{Model: "auto"}, Agent: protocol.AgentSettings{Harness: "omp"}}
	})
	if err != nil {
		t.Fatal(err)
	}
	sessions := newFakeSessions()
	work.Bind(sessions)
	t.Cleanup(func() { _ = work.Close() })
	return work, repo, sessions
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func addTestProject(t *testing.T, work *Work, repo string) protocol.ProjectInfo {
	t.Helper()
	result, err := work.addProject(context.Background(), protocol.ProjectAddParams{Path: repo})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestWorktreeCollisionsCopyIgnoredAndSetupTrust(t *testing.T) {
	work, repo, _ := testWork(t, fakeClient{})
	project := addTestProject(t, work, repo)
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "visible.txt"), []byte("visible\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := work.updateProjectSettings(context.Background(), protocol.ProjectSettingsParams{
		Project: project.ID, Patch: json.RawMessage(`{"copyIgnored":[".env","visible.txt"]}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := work.createWorktree(context.Background(), protocol.WorktreeCreateParams{Project: project.ID, Name: "My Task"}, "you")
	if err != nil {
		t.Fatal(err)
	}
	second, err := work.createWorktree(context.Background(), protocol.WorktreeCreateParams{Project: project.ID, Name: "My Task"}, "you")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(first.Path) != "my-task" || filepath.Base(second.Path) != "my-task-2" {
		t.Fatalf("unexpected paths: %s %s", first.Path, second.Path)
	}
	if first.Branch != "rashin/my-task" || second.Branch != "rashin/my-task-2" {
		t.Fatalf("unexpected branches: %s %s", first.Branch, second.Branch)
	}
	if _, err := os.Stat(filepath.Join(first.Path, ".env")); err != nil {
		t.Fatalf("ignored file was not copied: %v", err)
	}
	if _, err := os.Stat(filepath.Join(first.Path, "visible.txt")); !os.IsNotExist(err) {
		t.Fatalf("non-ignored configured file was copied: %v", err)
	}

	scriptDir := filepath.Join(first.Path, ".rashin")
	if err := os.MkdirAll(scriptDir, 0o700); err != nil {
		t.Fatal(err)
	}
	script := []byte("printf ran >> setup-ran\n")
	if err := os.WriteFile(filepath.Join(scriptDir, "setup"), script, 0o700); err != nil {
		t.Fatal(err)
	}
	setup, err := work.setupWorktree(context.Background(), first.Path, "")
	if err != nil {
		t.Fatal(err)
	}
	if setup.State != protocol.SetupUntrusted {
		t.Fatalf("setup state = %s", setup.State)
	}
	if _, err := os.Stat(filepath.Join(first.Path, "setup-ran")); !os.IsNotExist(err) {
		t.Fatal("untrusted script ran")
	}
	sum := sha256.Sum256(script)
	trusted, err := work.trustSetup(context.Background(), protocol.TrustParams{Path: first.Path, Hash: hex.EncodeToString(sum[:])})
	if err != nil {
		t.Fatal(err)
	}
	if trusted.State != protocol.SetupDone {
		t.Fatalf("trusted setup state = %s", trusted.State)
	}
	if _, err := os.Stat(filepath.Join(first.Path, "setup-ran")); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(first.Path, "setup-ran"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "ran" {
		t.Fatalf("setup output = %q", content)
	}
	repeated, err := work.setupWorktree(context.Background(), first.Path, "")
	if err != nil {
		t.Fatal(err)
	}
	if repeated.State != protocol.SetupDone {
		t.Fatalf("repeated setup state = %s", repeated.State)
	}
	content, _ = os.ReadFile(filepath.Join(first.Path, "setup-ran"))
	if string(content) != "ran" {
		t.Fatalf("setup reran without request: %q", content)
	}
	if _, err := work.setupWorktreeMode(context.Background(), first.Path, "", true); err != nil {
		t.Fatal(err)
	}
	content, _ = os.ReadFile(filepath.Join(first.Path, "setup-ran"))
	if string(content) != "ranran" {
		t.Fatalf("explicit setup rerun output = %q", content)
	}
}

func TestPreviewCleanupAndWatchLifecycle(t *testing.T) {
	work, repo, sessions := testWork(t, fakeClient{})
	project := addTestProject(t, work, repo)
	created, err := work.createWorktree(context.Background(), protocol.WorktreeCreateParams{Project: project.ID, Name: "cleanup"}, "you")
	if err != nil {
		t.Fatal(err)
	}
	sessions.mu.Lock()
	sessions.infos["agent-running"] = protocol.SessionInfo{ID: "agent-running", Kind: protocol.KindAgent, Worktree: created.Path, Project: project.ID, State: protocol.StateWorking, Alive: true}
	sessions.mu.Unlock()
	preview, err := work.previewRemove(context.Background(), created.Path)
	if err != nil {
		t.Fatal(err)
	}
	if preview.CanRemove || len(preview.Running) != 1 {
		t.Fatalf("running preview = %#v", preview)
	}
	if err := work.removeWorktree(context.Background(), protocol.WorktreeRemoveParams{Path: created.Path, Force: true}, "you"); err == nil {
		t.Fatal("forced removal was allowed with running session")
	}
	sessions.mu.Lock()
	delete(sessions.infos, "agent-running")
	sessions.mu.Unlock()
	if err := os.WriteFile(filepath.Join(created.Path, "local.tmp"), []byte("local"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = work.updateProjectSettings(context.Background(), protocol.ProjectSettingsParams{
		Project: project.ID, Patch: json.RawMessage(`{"cleanup":{"inactiveDays":1,"protect":"any"}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	work.mu.Lock()
	for i := range work.projects[project.ID].Worktrees {
		work.projects[project.ID].Worktrees[i].Activity = time.Now().Add(-48 * time.Hour)
	}
	work.mu.Unlock()
	cleanup, err := work.cleanupWorktrees(context.Background(), protocol.CleanupParams{Project: project.ID, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(cleanup.Items) != 1 || !strings.Contains(cleanup.Items[0].Reason, "protected local changes") {
		t.Fatalf("cleanup = %#v", cleanup)
	}
	work.mu.RLock()
	watchersBefore := len(work.watchers)
	work.mu.RUnlock()
	if watchersBefore != 0 {
		t.Fatal("watcher exists before interest")
	}
	work.Watch(project.ID, true)
	work.mu.RLock()
	watchersOn := len(work.watchers)
	refsOn := work.watchRef[project.ID]
	work.mu.RUnlock()
	if watchersOn != 1 || refsOn != 1 {
		t.Fatalf("watch on: watchers=%d refs=%d", watchersOn, refsOn)
	}
	events := work.events.(*fakeBroadcaster)
	beforeEdit := eventCount(events, protocol.EvtGitChanged)
	if err := os.WriteFile(filepath.Join(created.Path, "file.txt"), []byte("watched\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitForEvent(t, events, protocol.EvtGitChanged, beforeEdit)
	nested := filepath.Join(created.Path, "generated")
	beforeDirectory := eventCount(events, protocol.EvtGitChanged)
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	waitForEvent(t, events, protocol.EvtGitChanged, beforeDirectory)
	beforeNested := eventCount(events, protocol.EvtGitChanged)
	if err := os.WriteFile(filepath.Join(nested, "new.txt"), []byte("nested\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitForEvent(t, events, protocol.EvtGitChanged, beforeNested)
	work.Watch(project.ID, false)
	work.mu.RLock()
	watchersOff := len(work.watchers)
	refsOff := work.watchRef[project.ID]
	work.mu.RUnlock()
	if watchersOff != 0 || refsOff != 0 {
		t.Fatalf("watch off: watchers=%d refs=%d", watchersOff, refsOff)
	}
}

func TestTasksGatesPlanCheckpointsAndRevertGuard(t *testing.T) {
	plan := "Before {not json}.\n```json\n{\"tasks\":[{\"title\":\"First\",\"goal\":\"Do first\"},{\"title\":\"Second\",\"goal\":\"Do second\",\"deps\":[0]}]}\n```\nAfter {\"note\":\"done\"}."
	work, repo, sessions := testWork(t, fakeClient{text: plan})
	project := addTestProject(t, work, repo)
	dep, err := work.taskCreate(protocol.TaskCreateParams{Project: project.ID, Title: "Dependency"})
	if err != nil {
		t.Fatal(err)
	}
	task, err := work.taskCreate(protocol.TaskCreateParams{Project: project.ID, Title: "Feature", Goal: "Build it", Deps: []string{dep.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := work.taskStart(context.Background(), protocol.TaskStartParams{ID: task.ID}); err == nil {
		t.Fatal("task started with unfinished dependency")
	}
	_, err = work.taskUpdate(protocol.TaskUpdateParams{ID: dep.ID, Patch: json.RawMessage(`{"status":"done"}`)})
	if err != nil {
		t.Fatal(err)
	}
	started, err := work.taskStart(context.Background(), protocol.TaskStartParams{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if started.Task.Status != protocol.TaskWorking || started.Session.ID == "" || started.Worktree.Path == "" {
		t.Fatalf("start result = %#v", started)
	}
	sessions.mu.Lock()
	promptCount := len(sessions.prompts)
	sessions.mu.Unlock()
	if promptCount != 1 {
		t.Fatalf("task prompts = %d", promptCount)
	}
	gated, err := work.taskGate(protocol.GateParams{Task: task.ID, Question: "Pick one", Options: []string{"A", "B"}})
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := work.resolveGate(context.Background(), protocol.GateResolveParams{Task: task.ID, Gate: gated.Gates[0].ID, Resolution: "B"})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Status != protocol.TaskWorking || resolved.Gates[0].Resolution != "B" {
		t.Fatalf("resolved = %#v", resolved)
	}
	sessions.mu.Lock()
	gatePrompt := sessions.prompts[len(sessions.prompts)-1].Text
	sessions.mu.Unlock()
	if !strings.Contains(gatePrompt, "Pick one") || !strings.Contains(gatePrompt, "B") {
		t.Fatalf("gate prompt = %q", gatePrompt)
	}
	planned, err := work.taskPlan(context.Background(), protocol.TaskPlanParams{Project: project.ID, Goal: "A project"})
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Drafts) != 2 || len(planned.Drafts[1].Deps) != 1 || planned.Via != "free/planner" {
		t.Fatalf("plan = %#v", planned)
	}

	session := started.Session
	session.Worktree = started.Worktree.Path
	if ok, err := work.BeforeTurn(context.Background(), session, 1); err != nil || !ok {
		t.Fatalf("checkpoint one: %v %v", ok, err)
	}
	if err := os.WriteFile(filepath.Join(started.Worktree.Path, "file.txt"), []byte("two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if ok, err := work.BeforeTurn(context.Background(), session, 2); err != nil || !ok {
		t.Fatalf("checkpoint two: %v %v", ok, err)
	}
	diff, err := work.gitDiff(context.Background(), protocol.DiffParams{Worktree: started.Worktree.Path, Mode: protocol.DiffCheckpoint, Session: session.ID, Turn: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Files) == 0 {
		t.Fatal("checkpoint diff is empty")
	}
	sessions.mu.Lock()
	other := protocol.SessionInfo{ID: "agent-other", Kind: protocol.KindAgent, Worktree: started.Worktree.Path, State: protocol.StateWorking}
	sessions.infos[other.ID] = other
	sessions.mu.Unlock()
	if _, err := work.RevertTurn(context.Background(), session, 1); err == nil {
		t.Fatal("revert was allowed while another session worked")
	}
}

func TestReviewResolveReturnsAndBroadcasts(t *testing.T) {
	work, repo, _ := testWork(t, fakeClient{})
	project := addTestProject(t, work, repo)
	created, err := work.createWorktree(context.Background(), protocol.WorktreeCreateParams{Project: project.ID, Name: "review"}, "you")
	if err != nil {
		t.Fatal(err)
	}
	comment, err := work.reviewAdd(context.Background(), protocol.ReviewAddParams{
		Worktree: created.Path, Path: "file.txt", Line: 1, Side: "new", Text: "Change this.",
	})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, resolveErr := work.reviewResolve(context.Background(), protocol.ReviewRef{
			Worktree: created.Path, ID: comment.ID, Resolved: true,
		})
		done <- resolveErr
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("review resolve deadlocked")
	}
	if comments := work.reviewList(created.Path).Comments; len(comments) != 1 || !comments[0].Resolved {
		t.Fatalf("resolved comments = %#v", comments)
	}
}

func TestSetupKillsBackgroundProcess(t *testing.T) {
	work, repo, _ := testWork(t, fakeClient{})
	project := addTestProject(t, work, repo)
	created, err := work.createWorktree(context.Background(), protocol.WorktreeCreateParams{Project: project.ID, Name: "setup-process"}, "you")
	if err != nil {
		t.Fatal(err)
	}
	_, err = work.updateProjectSettings(context.Background(), protocol.ProjectSettingsParams{
		Project: project.ID,
		Patch:   json.RawMessage(`{"setup":"sleep 30 & echo $! > child.pid"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if _, err := work.setupWorktree(context.Background(), created.Path, ""); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("setup waited for background child: %v", elapsed)
	}
	pidText, err := os.ReadFile(filepath.Join(created.Path, "child.pid"))
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidText)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for syscall.Kill(pid, 0) == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if err := syscall.Kill(pid, 0); err == nil {
		t.Fatalf("setup child %d is still alive", pid)
	}
}

func TestSubdirectoryCheckpointRevertUsesWorktreeRoot(t *testing.T) {
	work, repo, _ := testWork(t, fakeClient{})
	_ = addTestProject(t, work, repo)
	subdir := filepath.Join(repo, "sub")
	if err := os.MkdirAll(subdir, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(subdir, "nested.txt")
	if err := os.WriteFile(file, []byte("before\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	session := protocol.SessionInfo{ID: "agent-subdir", Kind: protocol.KindAgent, Cwd: subdir, State: protocol.StateIdle}
	if ok, err := work.BeforeTurn(context.Background(), session, 1); err != nil || !ok {
		t.Fatalf("checkpoint: ok=%v err=%v", ok, err)
	}
	if err := os.WriteFile(file, []byte("after\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	work.AfterTurn(context.Background(), session, 1)
	external := filepath.Join(repo, "external-after.txt")
	if err := os.WriteFile(external, []byte("outside\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := work.RevertTurn(context.Background(), session, 1); err == nil {
		t.Fatal("revert removed changes made after the turn")
	} else if protocol.AsError(err).Code != protocol.CodeConflict || !strings.Contains(err.Error(), "external-after.txt") {
		t.Fatalf("revert guard error = %v", err)
	}
	content, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "after\n" {
		t.Fatalf("guard changed turn output = %q", content)
	}
	if err := os.Remove(external); err != nil {
		t.Fatal(err)
	}
	if _, err := work.RevertTurn(context.Background(), session, 1); err != nil {
		t.Fatal(err)
	}
	content, err = os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "before\n" {
		t.Fatalf("reverted content = %q", content)
	}
	if _, err := os.Stat(filepath.Join(subdir, "sub")); !os.IsNotExist(err) {
		t.Fatalf("nested worktree copy appeared: %v", err)
	}
}

func TestTaskDeleteRemovesDependenciesAndCyclesAreRejected(t *testing.T) {
	work, repo, _ := testWork(t, fakeClient{})
	project := addTestProject(t, work, repo)
	first, err := work.taskCreate(protocol.TaskCreateParams{Project: project.ID, Title: "First"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := work.taskCreate(protocol.TaskCreateParams{Project: project.ID, Title: "Second", Deps: []string{first.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := work.taskUpdate(protocol.TaskUpdateParams{
		ID: first.ID, Patch: json.RawMessage(`{"deps":["` + second.ID + `"]}`),
	}); err == nil {
		t.Fatal("dependency cycle was accepted")
	} else if protocol.AsError(err).Code != protocol.CodeBadRequest {
		t.Fatalf("cycle error = %v", err)
	}
	if err := work.taskDelete(first.ID); err != nil {
		t.Fatal(err)
	}
	listed, err := work.tasksList(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tasks) != 1 || listed.Tasks[0].ID != second.ID || len(listed.Tasks[0].Deps) != 0 {
		t.Fatalf("tasks after delete = %#v", listed.Tasks)
	}
	if _, err := work.taskUpdate(protocol.TaskUpdateParams{
		ID: second.ID, Patch: json.RawMessage(`{"status":"ready"}`),
	}); err != nil {
		t.Fatalf("dependent task remained stranded: %v", err)
	}
}

func TestStatusCacheObservesWorkingTreeEdits(t *testing.T) {
	work, repo, _ := testWork(t, fakeClient{})
	project := addTestProject(t, work, repo)
	created, err := work.createWorktree(context.Background(), protocol.WorktreeCreateParams{Project: project.ID, Name: "status"}, "you")
	if err != nil {
		t.Fatal(err)
	}
	listed, err := work.listWorktrees(context.Background(), project.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	var initial *protocol.WorktreeStatus
	for _, current := range listed.Worktrees {
		if samePath(current.Path, created.Path) {
			initial = current.Status
		}
	}
	if initial == nil || initial.Modified != 0 {
		t.Fatalf("initial status = %#v", initial)
	}
	if err := os.WriteFile(filepath.Join(created.Path, "file.txt"), []byte("edited\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	listed, err = work.listWorktrees(context.Background(), project.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	var refreshed *protocol.WorktreeStatus
	for _, current := range listed.Worktrees {
		if samePath(current.Path, created.Path) {
			refreshed = current.Status
		}
	}
	if refreshed == nil || refreshed.Modified == 0 {
		t.Fatalf("stale status = %#v", refreshed)
	}
	if err := os.WriteFile(filepath.Join(created.Path, "wip.txt"), []byte("new\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	graph, err := work.gitLog(context.Background(), protocol.GitLogParams{Project: project.ID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	foundWip := false
	for _, row := range graph.Wip {
		if samePath(row.Worktree, created.Path) && row.Untracked > 0 {
			foundWip = true
		}
	}
	if !foundWip {
		t.Fatalf("graph WIP rows = %#v", graph.Wip)
	}
}

func TestRepairAndPruneOwnedMovedWorktree(t *testing.T) {
	work, repo, _ := testWork(t, fakeClient{})
	project := addTestProject(t, work, repo)
	created, err := work.createWorktree(context.Background(), protocol.WorktreeCreateParams{Project: project.ID, Name: "repair"}, "you")
	if err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(filepath.Dir(created.Path), "repair-moved")
	if err := os.Rename(created.Path, moved); err != nil {
		t.Fatal(err)
	}
	repaired, err := work.repairWorktrees(context.Background(), project.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range repaired.Worktrees {
		if samePath(item.Path, moved) {
			found = true
		}
	}
	if !found {
		t.Fatalf("repaired worktrees = %#v", repaired.Worktrees)
	}
	if err := os.RemoveAll(moved); err != nil {
		t.Fatal(err)
	}
	preview, err := work.pruneWorktrees(context.Background(), protocol.PruneParams{Project: project.ID, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Pruned) != 1 || !samePath(preview.Pruned[0], moved) {
		t.Fatalf("prune preview = %#v", preview.Pruned)
	}
	work.mu.RLock()
	ownedBefore := len(work.projects[project.ID].Worktrees)
	work.mu.RUnlock()
	if ownedBefore != 1 {
		t.Fatalf("dry run removed owned record: %d", ownedBefore)
	}
	pruned, err := work.pruneWorktrees(context.Background(), protocol.PruneParams{Project: project.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(pruned.Pruned) != 1 || !samePath(pruned.Pruned[0], moved) {
		t.Fatalf("pruned = %#v", pruned.Pruned)
	}
	work.mu.RLock()
	ownedAfter := len(work.projects[project.ID].Worktrees)
	work.mu.RUnlock()
	if ownedAfter != 0 {
		t.Fatalf("prune kept owned record: %d", ownedAfter)
	}
}

func TestGateWithoutSessionAndDependencyReadiness(t *testing.T) {
	work, repo, _ := testWork(t, fakeClient{})
	project := addTestProject(t, work, repo)
	dependency, err := work.taskCreate(protocol.TaskCreateParams{Project: project.ID, Title: "Dependency"})
	if err != nil {
		t.Fatal(err)
	}
	dependent, err := work.taskCreate(protocol.TaskCreateParams{Project: project.ID, Title: "Dependent", Deps: []string{dependency.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := work.taskStart(context.Background(), protocol.TaskStartParams{ID: dependent.ID}); err == nil {
		t.Fatal("dependent task started before dependency")
	}
	if _, err := work.taskUpdate(protocol.TaskUpdateParams{ID: dependency.ID, Patch: json.RawMessage(`{"status":"done"}`)}); err != nil {
		t.Fatal(err)
	}
	listed, err := work.tasksList(project.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range listed.Tasks {
		if task.ID == dependent.ID && task.Status != protocol.TaskReady {
			t.Fatalf("dependent status = %s", task.Status)
		}
	}
	gated, err := work.taskGate(protocol.GateParams{Task: dependent.ID, Question: "Choose", Options: []string{"A", "B"}})
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := work.resolveGate(context.Background(), protocol.GateResolveParams{Task: dependent.ID, Gate: gated.Gates[0].ID, Resolution: "A"})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Status != protocol.TaskReady || resolved.Gates[0].Resolution != "A" {
		t.Fatalf("resolved gate task = %#v", resolved)
	}
}

func TestMutationActivityCoverageExactlyOnce(t *testing.T) {
	work, repo, _ := testWork(t, fakeClient{})
	project := addTestProject(t, work, repo)
	if _, err := work.updateProjectSettings(context.Background(), protocol.ProjectSettingsParams{Project: project.ID, Patch: json.RawMessage(`{"branchPrefix":"bad..ref"}`)}); err == nil {
		t.Fatal("invalid settings update succeeded")
	}
	if _, err := work.updateProjectSettings(context.Background(), protocol.ProjectSettingsParams{Project: project.ID, Patch: json.RawMessage(`{"branchPrefix":"qa/"}`)}); err != nil {
		t.Fatal(err)
	}
	tree, err := work.createWorktree(context.Background(), protocol.WorktreeCreateParams{Project: project.ID, Name: "review-activity"}, "you")
	if err != nil {
		t.Fatal(err)
	}
	comment, err := work.reviewAdd(context.Background(), protocol.ReviewAddParams{Worktree: tree.Path, Path: "file.txt", Line: 1, Side: "new", Text: "Review"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := work.reviewResolve(context.Background(), protocol.ReviewRef{Worktree: tree.Path, ID: comment.ID, Resolved: true}); err != nil {
		t.Fatal(err)
	}
	if err := work.reviewDelete(context.Background(), protocol.ReviewRef{Worktree: tree.Path, ID: comment.ID}); err != nil {
		t.Fatal(err)
	}
	task, err := work.taskCreate(protocol.TaskCreateParams{Project: project.ID, Title: "Activity task", Goal: "Do it"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := work.taskUpdate(protocol.TaskUpdateParams{ID: task.ID, Patch: json.RawMessage(`{"title":"Activity task updated"}`)}); err != nil {
		t.Fatal(err)
	}
	gated, err := work.taskGate(protocol.GateParams{Task: task.ID, Question: "Proceed?", Options: []string{"yes"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := work.resolveGate(context.Background(), protocol.GateResolveParams{Task: task.ID, Gate: gated.Gates[0].ID, Resolution: "yes"}); err != nil {
		t.Fatal(err)
	}
	if _, err := work.taskStart(context.Background(), protocol.TaskStartParams{ID: task.ID}); err != nil {
		t.Fatal(err)
	}
	if err := work.taskDelete(task.ID); err != nil {
		t.Fatal(err)
	}
	activity, err := work.listActivity(protocol.ActivityParams{Project: project.ID, Limit: 5000})
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]int{"project.settings": 2, "review.add": 1, "review.resolve": 1, "review.delete": 1, "task.create": 1, "task.update": 1, "task.gate.open": 1, "task.gate.resolve": 1, "task.start": 1, "task.delete": 1}
	counts := make(map[string]int)
	settingsFailures := 0
	for _, entry := range activity.Activity {
		counts[entry.Action]++
		if entry.Action == "project.settings" && !entry.OK {
			settingsFailures++
		}
	}
	for action, expectedCount := range expected {
		if counts[action] != expectedCount {
			t.Fatalf("activity %s count = %d, all = %#v", action, counts[action], counts)
		}
	}
	if settingsFailures != 1 {
		t.Fatalf("settings failure activity count = %d", settingsFailures)
	}
}

func TestProjectAddIsIdempotent(t *testing.T) {
	work, repo, _ := testWork(t, fakeClient{})
	first := addTestProject(t, work, repo)
	nested := filepath.Join(repo, "nested")
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	second, err := work.addProject(context.Background(), protocol.ProjectAddParams{Path: nested})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || !samePath(first.Root, second.Root) {
		t.Fatalf("projects differ: %#v %#v", first, second)
	}
	activity, err := work.listActivity(protocol.ActivityParams{Project: first.ID, Limit: 5000})
	if err != nil {
		t.Fatal(err)
	}
	adds := 0
	for _, entry := range activity.Activity {
		if entry.Action == "project.add" {
			adds++
		}
	}
	if adds != 1 {
		t.Fatalf("project.add activity count = %d", adds)
	}
}

var _ core.SessionControl = (*fakeSessions)(nil)
