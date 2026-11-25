package state

import (
	"encoding/json"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

func TestStoreFeatureDoesNotWaitForStoreLock(t *testing.T) {
	store := New(nil)
	if store.Feature("compare") {
		t.Fatal("a new store enabled a feature")
	}
	features := map[string]bool{"compare": true}
	store.ApplySnapshot(protocol.Snapshot{Settings: protocol.Settings{Features: features}})
	features["compare"] = false
	if !store.Feature("compare") {
		t.Fatal("feature publication retained the caller's map")
	}
	if err := store.Apply(protocol.Event{Event: protocol.EvtSettingsChanged, Data: json.RawMessage(`{"features":{"history":true}}`)}); err != nil {
		t.Fatal(err)
	}
	if store.Feature("compare") || !store.Feature("history") {
		t.Fatal("settings.changed did not replace published features")
	}

	locked, release, unlocked := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go func() {
		store.mu.Lock()
		close(locked)
		<-release
		store.mu.Unlock()
		close(unlocked)
	}()
	<-locked
	defer func() {
		close(release)
		<-unlocked
	}()
	result := make(chan bool, 1)
	go func() { result <- store.Feature("history") }()
	select {
	case enabled := <-result:
		if !enabled {
			t.Fatal("the published feature was lost while the store was locked")
		}
	case <-time.After(time.Second):
		t.Fatal("Feature waited for the store lock")
	}
}

func TestConnectionEpochAdvancesOnReadyTransitions(t *testing.T) {
	store := New(nil)
	if epoch := store.Snapshot().Epoch; epoch != 0 {
		t.Fatalf("initial epoch = %d, want 0", epoch)
	}
	store.SetConnection(Connecting)
	if epoch := store.Snapshot().Epoch; epoch != 0 {
		t.Fatalf("connecting epoch = %d, want 0", epoch)
	}
	store.SetConnection(Ready)
	if epoch := store.Snapshot().Epoch; epoch != 1 {
		t.Fatalf("first ready epoch = %d, want 1", epoch)
	}
	store.SetConnection(Ready)
	if epoch := store.Snapshot().Epoch; epoch != 1 {
		t.Fatalf("duplicate ready epoch = %d, want 1", epoch)
	}
	store.SetConnection(Connecting)
	store.SetConnection(Ready)
	if epoch := store.Snapshot().Epoch; epoch != 2 {
		t.Fatalf("reconnected epoch = %d, want 2", epoch)
	}
}

func TestFocusDeliveriesAreConsumedOnceAndBoundedByTheFrame(t *testing.T) {
	store := New(nil)
	initial := store.Snapshot()
	if initial.FocusGeneration != 0 || len(store.TakeFocus(initial.FocusGeneration)) != 0 {
		t.Fatal("an initial snapshot produced a focus request")
	}
	deliver := func(raw string) {
		t.Helper()
		if err := store.Apply(protocol.Event{Event: protocol.EvtUIFocus, Data: json.RawMessage(raw)}); err != nil {
			t.Fatal(err)
		}
	}
	deliver(`{"tab":"settings","view":"features","activationToken":"one-shot"}`)
	first := store.Snapshot()
	deliver(`{"tab":"settings","view":"features","activationToken":"one-shot"}`)
	if len(store.TakeFocus(initial.FocusGeneration)) != 0 {
		t.Fatal("an old frame consumed a newer focus request")
	}
	taken := store.TakeFocus(first.FocusGeneration)
	if len(taken) != 1 || taken[0].Generation != 1 || taken[0].Params.ActivationToken != "one-shot" {
		t.Fatal("the first frame did not consume exactly its own activation request")
	}
	if len(store.TakeFocus(first.FocusGeneration)) != 0 {
		t.Fatal("a redraw replayed an activation token")
	}
	store.ApplySnapshot(protocol.Snapshot{})
	latest := store.Snapshot()
	if latest.FocusGeneration != 2 {
		t.Fatal("a daemon snapshot changed the focus delivery generation")
	}
	taken = store.TakeFocus(latest.FocusGeneration)
	if len(taken) != 1 || taken[0].Generation != 2 || taken[0].Params.Tab != protocol.TabSettings {
		t.Fatal("an identical later request was lost")
	}
	if len(store.TakeFocus(latest.FocusGeneration)) != 0 {
		t.Fatal("the latest frame replayed a consumed activation request")
	}
}

func TestMalformedFocusDoesNotProducePartialNavigationOrActivation(t *testing.T) {
	store := New(nil)
	for _, raw := range []string{`{"tab":"settings","view":17}`, `{"tab":`, `null`} {
		_ = store.Apply(protocol.Event{Event: protocol.EvtUIFocus, Data: json.RawMessage(raw)})
	}
	snapshot := store.Snapshot()
	if snapshot.FocusGeneration != 0 || len(store.TakeFocus(snapshot.FocusGeneration)) != 0 {
		t.Fatal("malformed focus input produced a native activation request")
	}
}

func TestFocusActivationRegistrationConsumesPendingTokensOnce(t *testing.T) {
	store := New(nil)
	if err := store.Apply(protocol.Event{Event: protocol.EvtUIFocus, Data: json.RawMessage(`{"tab":"settings","activationToken":"launch-race"}`)}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	activate := func(token string) {
		if token != "launch-race" {
			t.Fatal("pending native activation lost its token")
		}
		// the native callback must run outside the state lock
		_ = store.Snapshot()
		calls++
	}
	store.SetFocusActivation(activate)
	store.SetFocusActivation(activate)
	if calls != 1 {
		t.Fatalf("registration activated a pending request %d times, want 1", calls)
	}
	taken := store.TakeFocus(store.Snapshot().FocusGeneration)
	if len(taken) != 1 || !taken[0].Activated || taken[0].Params.ActivationToken != "" || taken[0].Params.Tab != protocol.TabSettings {
		t.Fatal("native activation did not leave token-free pending navigation")
	}
	store.SetFocusActivation(nil)
	if err := store.Apply(protocol.Event{Event: protocol.EvtUIFocus, Data: json.RawMessage(`{"tab":"settings"}`)}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("a late delivery reached the unregistered window handler")
	}
}

func TestStoreRejectsBrokenLocalEventsWithoutLosingLastModel(t *testing.T) {
	store := New(nil)
	store.ApplySnapshot(protocol.Snapshot{Local: protocol.LocalResult{
		Models: []protocol.LocalModel{{ID: "lfm3", Name: "LFM3 Omni 2B", State: protocol.LocalInstalled}},
	}})
	if err := store.Apply(protocol.Event{Event: protocol.EvtLocalChanged, Data: json.RawMessage(`{`)}); err == nil {
		t.Fatal("invalid local.changed was accepted")
	}
	if got := store.Snapshot().Local.Models; len(got) != 1 || got[0].Name != "LFM3 Omni 2B" {
		t.Fatalf("invalid local.changed erased the installed model: %+v", got)
	}
	if err := store.Apply(protocol.Event{Event: protocol.EvtLocalProgress, Data: json.RawMessage(`{`)}); err == nil {
		t.Fatal("invalid local.progress was accepted")
	}
	if got := store.Snapshot().LocalProgress; len(got) != 0 {
		t.Fatalf("invalid local.progress added a download: %+v", got)
	}
}

func TestStoreSnapshotAndEventsStayOrdered(t *testing.T) {
	store := New(nil)
	older := protocol.SessionInfo{ID: "ask-old", Kind: protocol.KindAsk, Title: "Older", Updated: time.Unix(10, 0)}
	newer := protocol.SessionInfo{ID: "agent-new", Kind: protocol.KindAgent, Title: "Newer", Updated: time.Unix(20, 0)}
	store.ApplySnapshot(protocol.Snapshot{Sessions: []protocol.SessionInfo{older, newer}, Settings: protocol.Settings{Enabled: true}})
	snapshot := store.Snapshot()
	if len(snapshot.Order) != 2 || snapshot.Order[0] != newer.ID || snapshot.Order[1] != older.ID {
		t.Fatalf("session order = %v, want newest first", snapshot.Order)
	}
	changed := older
	changed.Title = "Renamed"
	changed.Updated = time.Unix(30, 0)
	raw, _ := json.Marshal(changed)
	if err := store.Apply(protocol.Event{Event: protocol.EvtSessionInfo, Data: raw}); err != nil {
		t.Fatal(err)
	}
	snapshot = store.Snapshot()
	if snapshot.Order[0] != older.ID || snapshot.Sessions[older.ID].Title != "Renamed" {
		t.Fatalf("session.info was not reduced: %#v", snapshot)
	}
	removed, _ := json.Marshal(protocol.SessionsChanged{Removed: []string{newer.ID}})
	if err := store.Apply(protocol.Event{Event: protocol.EvtSessionsChanged, Data: removed}); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Snapshot().Sessions[newer.ID]; ok {
		t.Fatal("removed session remained in snapshot")
	}
}

func TestStoreAttachResumesWithoutResettingTranscript(t *testing.T) {
	store := New(nil)
	info := protocol.SessionInfo{ID: "ask-one", Kind: protocol.KindAsk, Title: "One"}
	store.ApplySnapshot(protocol.Snapshot{Sessions: []protocol.SessionInfo{info}})
	if since := store.Open(info.ID); since != 0 {
		t.Fatalf("initial since = %d, want 0", since)
	}
	store.ApplyAttach(0, protocol.SessionAttachResult{Info: info, Events: []protocol.SessionEvent{{Seq: 1, Kind: protocol.EvTurnStart}, {Seq: 2, Kind: protocol.EvTurnEnd}}})
	if since := store.OpenSessions()[info.ID]; since != 2 {
		t.Fatalf("resume since = %d, want 2", since)
	}
	store.ApplyAttach(2, protocol.SessionAttachResult{Info: info, Events: []protocol.SessionEvent{{Seq: 3, Kind: protocol.EvTitle}}})
	attached := store.Snapshot().Attached[info.ID]
	if attached.Model.LastSeq() != 3 {
		t.Fatalf("last seq = %d, want 3", attached.Model.LastSeq())
	}
}

func streamed(t *testing.T, store *Store, session string, seq int64, kind string, data any) {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(protocol.SessionEventData{Session: session, SessionEvent: protocol.SessionEvent{Seq: seq, Kind: kind, Data: raw}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Apply(protocol.Event{Event: protocol.EvtSessionEvent, Data: wire}); err != nil {
		t.Fatal(err)
	}
}

// The replay that opens a session is applied by whoever asked for it, and
// what the session streams by the reader of the connection. A new question is
// sent the moment its session exists, so its first events can be read before
// the replay is applied. The replay from zero then wiped them, and the
// question vanished from its own transcript.
func TestWhatStreamsBeforeTheReplayLandsIsKept(t *testing.T) {
	info := protocol.SessionInfo{ID: "ask-new", Kind: protocol.KindAsk}
	asked := protocol.UserData{Text: "Which step failed?"}
	prompts := func(store *Store) []string { return store.Snapshot().Attached[info.ID].Model.Prompts() }

	// attached before the question was asked: an empty replay, then the stream
	store := New(nil)
	store.Open(info.ID)
	streamed(t, store, info.ID, 1, protocol.EvUser, asked)
	streamed(t, store, info.ID, 2, protocol.EvTurnStart, protocol.TurnStartData{Turn: 1})
	store.ApplyAttach(0, protocol.SessionAttachResult{Info: info})
	if got := prompts(store); len(got) != 1 || got[0] != asked.Text {
		t.Fatalf("after an empty replay landed late the transcript holds the prompts %q", got)
	}
	streamed(t, store, info.ID, 3, protocol.EvText, protocol.TextData{Message: "m", Delta: "Step 3."})
	if seq := store.Snapshot().Attached[info.ID].Model.LastSeq(); seq != 3 {
		t.Fatalf("after the replay landed the stream stopped at %d", seq)
	}

	// attached after it was asked: the question is in the replay, the stream begins behind it
	store = New(nil)
	store.Open(info.ID)
	streamed(t, store, info.ID, 2, protocol.EvTurnStart, protocol.TurnStartData{Turn: 1})
	raw, _ := json.Marshal(asked)
	store.ApplyAttach(0, protocol.SessionAttachResult{Info: info, Events: []protocol.SessionEvent{{Seq: 1, Kind: protocol.EvUser, Data: raw}}})
	if got := prompts(store); len(got) != 1 || store.Snapshot().Attached[info.ID].Model.LastSeq() != 2 {
		t.Fatalf("a replay that landed after the stream began left prompts %q at seq %d", got, store.Snapshot().Attached[info.ID].Model.LastSeq())
	}

	// a replay that never comes must not hold the stream back for good
	store = New(nil)
	store.Open(info.ID)
	streamed(t, store, info.ID, 1, protocol.EvUser, asked)
	store.Opened(info.ID)
	if got := prompts(store); len(got) != 1 {
		t.Fatalf("after a failed attach the transcript holds the prompts %q", got)
	}
}

func TestStoreReducesWorkCollectionsAndGitChanged(t *testing.T) {
	store := New(nil)
	project := protocol.ProjectInfo{ID: "hyperion", Name: "Hyperion"}
	task := protocol.TaskInfo{ID: "task-1", Project: project.ID, Title: "Build Work", Status: protocol.TaskWorking}
	store.ApplySnapshot(protocol.Snapshot{Projects: []protocol.ProjectInfo{project}, Tasks: []protocol.TaskInfo{task}})
	worktree := protocol.WorktreeInfo{Project: project.ID, Path: "/tmp/hyperion", Name: "hyperion"}
	comment := protocol.ReviewComment{ID: "review-1", Worktree: worktree.Path, Path: "router.go", Line: 3}
	events := []struct {
		name string
		data any
	}{
		{protocol.EvtProjectsChanged, protocol.ProjectsResult{Projects: []protocol.ProjectInfo{{ID: project.ID, Name: "Renamed"}}}},
		{protocol.EvtWorktreesChanged, protocol.WorktreesResult{Project: project.ID, Worktrees: []protocol.WorktreeInfo{worktree}}},
		{protocol.EvtTasksChanged, protocol.TasksResult{Tasks: []protocol.TaskInfo{{ID: task.ID, Project: project.ID, Title: task.Title, Status: protocol.TaskDone}}}},
		{protocol.EvtReviewChanged, protocol.ReviewResult{Worktree: worktree.Path, Comments: []protocol.ReviewComment{comment}}},
	}
	for _, item := range events {
		raw, err := json.Marshal(item.data)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Apply(protocol.Event{Event: item.name, Data: raw}); err != nil {
			t.Fatal(err)
		}
	}
	called := 0
	unsubscribe := store.OnGitChanged(project.ID, func() { called++ })
	raw, _ := json.Marshal(protocol.GitChanged{Project: project.ID, Worktree: worktree.Path})
	if err := store.Apply(protocol.Event{Event: protocol.EvtGitChanged, Data: raw}); err != nil {
		t.Fatal(err)
	}
	unsubscribe()
	_ = store.Apply(protocol.Event{Event: protocol.EvtGitChanged, Data: raw})
	snapshot := store.Snapshot()
	if len(snapshot.Projects) != 1 || snapshot.Projects[0].Name != "Renamed" {
		t.Fatalf("projects = %#v", snapshot.Projects)
	}
	if len(snapshot.Worktrees[project.ID]) != 1 || snapshot.Worktrees[project.ID][0].Path != worktree.Path {
		t.Fatalf("worktrees = %#v", snapshot.Worktrees)
	}
	if len(snapshot.Tasks[project.ID]) != 1 || snapshot.Tasks[project.ID][0].Status != protocol.TaskDone {
		t.Fatalf("tasks = %#v", snapshot.Tasks)
	}
	if len(snapshot.Reviews[worktree.Path]) != 1 || snapshot.Reviews[worktree.Path][0].ID != comment.ID {
		t.Fatalf("reviews = %#v", snapshot.Reviews)
	}
	if called != 1 {
		t.Fatalf("git listener calls = %d, want 1", called)
	}
}
