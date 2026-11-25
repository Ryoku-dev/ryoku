package projects

import (
	"errors"
	"testing"

	"ryoku-rashin/internal/app/env"
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

func TestTaskListPutsWhatNeedsTheReaderFirstAndDropsEmptyGroups(t *testing.T) {
	items := taskItems([]protocol.TaskInfo{
		{ID: "one", Status: protocol.TaskBacklog},
		{ID: "two", Status: protocol.TaskWorking},
		{ID: "three", Status: protocol.TaskBacklog},
		{ID: "four", Status: protocol.TaskNeedsYou},
	})
	var order []string
	for _, item := range items {
		if item.header != "" {
			order = append(order, "#"+item.header)
			if item.count == 0 {
				t.Fatalf("group %q is shown with nothing in it", item.header)
			}
			continue
		}
		order = append(order, item.task.ID)
	}
	want := []string{"#Needs you", "four", "#Working", "two", "#Backlog", "one", "three"}
	if len(order) != len(want) {
		t.Fatalf("task list = %v, want %v", order, want)
	}
	for index := range want {
		if order[index] != want[index] {
			t.Fatalf("task list = %v, want %v", order, want)
		}
	}
}

func TestRemovingAWorktreeThatHoldsWorkTakesItsName(t *testing.T) {
	clean := protocol.RemovePreview{Path: "/work/clean", CanRemove: true}
	dirty := protocol.RemovePreview{Path: "/work/dirty", CanRemove: true, NeedsForce: true}
	blocked := protocol.RemovePreview{Path: "/work/main"}
	if !removeReady(clean, "clean", "") {
		t.Fatal("a worktree with nothing to lose asked for its name")
	}
	if removeReady(dirty, "dirty", "") || removeReady(dirty, "dirty", "dirt") {
		t.Fatal("a worktree that holds work could be removed without typing its name")
	}
	if !removeReady(dirty, "dirty", " dirty ") {
		t.Fatal("typing the name did not allow the removal")
	}
	if removeReady(blocked, "main", "main") {
		t.Fatal("a worktree that cannot be removed was allowed to be")
	}

	if removeParams(clean, false, false).Force {
		t.Fatal("a clean removal was forced")
	}
	params := removeParams(dirty, true, true)
	if !params.Force || !params.DeleteBranch || !params.ForceBranch {
		t.Fatalf("a forced removal with its branch lost a choice: %#v", params)
	}
	if removeParams(dirty, false, true).ForceBranch {
		t.Fatal("a branch that stays was marked to be force deleted")
	}
}

func TestProjectsOpenAtTheProjectLastShownWhileItStillExists(t *testing.T) {
	store := appstate.New(nil)
	e := &env.Env{Store: store}
	page := New()
	if _, _, _, ok := page.Home(e); ok {
		t.Fatal("Projects named a home with no project to show")
	}
	store.ApplySnapshot(protocol.Snapshot{Projects: []protocol.ProjectInfo{{ID: "hyperion"}, {ID: "ryotunes"}}})
	if project, view, _, ok := page.Home(e); !ok || project != "hyperion" || view != TabOverview {
		t.Fatalf("first home = %q %q %v, want the first project's overview", project, view, ok)
	}
	page.state.project, page.state.tab, page.state.worktree = "ryotunes", TabChanges, "/w/tune"
	if project, view, worktree, ok := page.Home(e); !ok || project != "ryotunes" || view != TabChanges || worktree != "/w/tune" {
		t.Fatalf("home = %q %q %q %v, want where the reader left", project, view, worktree, ok)
	}
	store.ApplySnapshot(protocol.Snapshot{Projects: []protocol.ProjectInfo{{ID: "hyperion"}}})
	if project, _, _, ok := page.Home(e); !ok || project != "hyperion" {
		t.Fatalf("home after the last project was forgotten = %q %v, want the one that is left", project, ok)
	}
}
