package projects

import (
	"errors"
	"testing"

	appstate "ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

type fakeProjectEnvironment struct {
	store         *appstate.Store
	worktreeLoads int
	taskLoads     int
}

func (e *fakeProjectEnvironment) Snapshot() appstate.Snapshot {
	return e.store.Snapshot()
}

func (e *fakeProjectEnvironment) LoadWorktrees(
	project string,
	done func(error),
) {
	e.worktreeLoads++
	if e.store.Snapshot().Connection != appstate.Ready {
		done(errors.New("not connected"))
		return
	}
	e.store.SetWorktrees(protocol.WorktreesResult{Project: project})
	done(nil)
}

func (e *fakeProjectEnvironment) LoadTasks(
	project string,
	done func(error),
) {
	e.taskLoads++
	if e.store.Snapshot().Connection != appstate.Ready {
		done(errors.New("not connected"))
		return
	}
	e.store.SetTasks(project, protocol.TasksResult{})
	done(nil)
}

func (e *fakeProjectEnvironment) WatchProject(string) {}

func (e *fakeProjectEnvironment) UnwatchProject(string) {}

func (e *fakeProjectEnvironment) OnGitChanged(string, func()) func() {
	return func() {}
}

func TestProjectLoadsRetryAfterConnectionBecomesReady(t *testing.T) {
	store := appstate.New(nil)
	store.SetConnection(appstate.Connecting)
	e := &fakeProjectEnvironment{store: store}
	var page pageState
	page.init()

	page.open(e, "hyperion", TabGraph, "")
	if e.worktreeLoads != 1 || e.taskLoads != 1 {
		t.Fatalf(
			"pre-connect loads = worktrees %d, tasks %d; want 1 each",
			e.worktreeLoads,
			e.taskLoads,
		)
	}

	store.SetConnection(appstate.Ready)
	page.ensureProjectData(e, "hyperion")
	if e.worktreeLoads != 2 || e.taskLoads != 2 {
		t.Fatalf(
			"ready loads = worktrees %d, tasks %d; want 2 each",
			e.worktreeLoads,
			e.taskLoads,
		)
	}
	epoch := store.Snapshot().Epoch
	if page.worktreeLoads["hyperion"].successEpoch != epoch {
		t.Fatalf(
			"worktree success epoch = %d, want %d",
			page.worktreeLoads["hyperion"].successEpoch,
			epoch,
		)
	}
	if page.taskLoads["hyperion"].successEpoch != epoch {
		t.Fatalf(
			"task success epoch = %d, want %d",
			page.taskLoads["hyperion"].successEpoch,
			epoch,
		)
	}

	page.ensureProjectData(e, "hyperion")
	if e.worktreeLoads != 2 || e.taskLoads != 2 {
		t.Fatalf(
			"duplicate ready loads = worktrees %d, tasks %d",
			e.worktreeLoads,
			e.taskLoads,
		)
	}

	store.SetConnection(appstate.Connecting)
	store.SetConnection(appstate.Ready)
	page.ensureProjectData(e, "hyperion")
	if e.worktreeLoads != 3 || e.taskLoads != 3 {
		t.Fatalf(
			"reconnect loads = worktrees %d, tasks %d; want 3 each",
			e.worktreeLoads,
			e.taskLoads,
		)
	}
}

func TestGroupTasksUsesBoardStatus(t *testing.T) {
	tasks := []protocol.TaskInfo{
		{ID: "one", Status: protocol.TaskBacklog},
		{ID: "two", Status: protocol.TaskWorking},
		{ID: "three", Status: protocol.TaskBacklog},
		{ID: "four", Status: protocol.TaskNeedsYou},
	}
	groups := groupTasks(tasks)
	if len(groups[protocol.TaskBacklog]) != 2 || groups[protocol.TaskBacklog][0].ID != "one" || groups[protocol.TaskBacklog][1].ID != "three" {
		t.Fatalf("backlog = %#v", groups[protocol.TaskBacklog])
	}
	if len(groups[protocol.TaskWorking]) != 1 || groups[protocol.TaskWorking][0].ID != "two" {
		t.Fatalf("working = %#v", groups[protocol.TaskWorking])
	}
	if len(groups[protocol.TaskNeedsYou]) != 1 || groups[protocol.TaskNeedsYou][0].ID != "four" {
		t.Fatalf("needs you = %#v", groups[protocol.TaskNeedsYou])
	}
	for _, status := range boardStatuses {
		if _, ok := groups[status]; !ok {
			t.Fatalf("missing board column %q", status)
		}
	}
}

func TestRemoveParamsRequiresForceFromPreview(t *testing.T) {
	clean := removeParams(protocol.RemovePreview{Path: "/work/clean", NeedsForce: false}, false, false)
	if clean.Force {
		t.Fatal("clean preview unexpectedly forces removal")
	}
	dirty := removeParams(protocol.RemovePreview{Path: "/work/dirty", NeedsForce: true}, true, true)
	if !dirty.Force {
		t.Fatal("preview that needs force did not set Force")
	}
	if !dirty.DeleteBranch || !dirty.ForceBranch {
		t.Fatalf("branch choices were lost: %#v", dirty)
	}
}
