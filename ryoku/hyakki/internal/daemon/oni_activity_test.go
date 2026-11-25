package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/index"
	"ryoku-rashin/internal/oni/hands/task"
	"ryoku-rashin/internal/oni/remind"
	"ryoku-rashin/internal/oni/routine"
	"ryoku-rashin/internal/protocol"
)

func activitySeed(t *testing.T) *Daemon {
	t.Helper()
	d, _ := partsDaemon(t, func(settings *protocol.Settings) { settings.Enabled = true })
	at := d.reflexClock().Now()
	d.oni.reflex.rows = []protocol.OniReflexRow{{At: at, Intent: "volume.up", Route: protocol.DecisionRules, Ms: 12, Outcome: protocol.OniReflexDone}}
	d.oni.hands.rows = []protocol.OniHandsRow{{At: at, Tool: "click", Ms: 21, Outcome: protocol.OniHandsOutcomeDone}}
	l := d.assignLedger()
	if err := l.Attempt(at, false); err != nil {
		t.Fatal(err)
	}
	if err := l.Record(protocol.OniAssignmentRow{At: at, Lane: protocol.OniAssignmentLaneFast, Route: "fixture/eyes", Input: 23, Output: 7, Estimated: true, Outcome: "ok"}, false); err != nil {
		t.Fatal(err)
	}
	putKept(t, keptFile, keptSave)
	if err := d.oniRoutines().Put(routine.Routine{Name: "Privacy fixture routine", Steps: threeSteps}); err != nil {
		t.Fatal(err)
	}
	d.oni.remind.mu.Lock()
	store := d.remindStoreLocked()
	d.oni.remind.mu.Unlock()
	if err := store.Add(remind.Reminder{ID: "0123456789abcdef", Kind: remind.KindReminder, Created: at, Due: at.Add(time.Hour), Text: "PRIVATE_REMINDER_WORDS"}); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(recallRoot(), protocol.OniImportClaude)
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "fixture.md"), []byte("PRIVATE_IMPORTED_TITLE and PRIVATE_IMPORTED_PASSAGE"), 0o600); err != nil {
		t.Fatal(err)
	}
	j, err := d.handsJournal()
	if err != nil {
		t.Fatal(err)
	}
	// Enqueue starts the first journal task immediately. This fixture is
	// retained history, not a live task, so finish it through its owner.
	queued, err := j.Enqueue(task.Task{ID: task.NewID(), Scope: protocol.OniHandsScopeWindow, App: "fixture", Origin: "app"}, at)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := j.Update(queued.ID, task.Stopped, queued.Step, "", at); err != nil {
		t.Fatal(err)
	}
	return d
}

func activityRead(t *testing.T, d *Daemon) protocol.OniActivityResult {
	t.Helper()
	result, err := d.oniActivity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestOniActivityReadsActualOwnersWhileOffWithoutContent(t *testing.T) {
	d := activitySeed(t)
	got := activityRead(t, d)
	for _, store := range protocol.OniActivityStores {
		if got.Counts[store] != 1 {
			t.Errorf("%s count %d, want its owner's one record", store, got.Counts[store])
		}
	}
	if len(got.Rows) != 3 || got.Since.IsZero() {
		t.Fatalf("ledger %+v", got)
	}
	for _, row := range got.Rows {
		if row.Store != protocol.OniActivityAssignment && !row.Unmetered {
			t.Errorf("missing token usage became free: %+v", row)
		}
		if row.Store == protocol.OniActivityAssignment && (!row.Estimated || row.Input != 23 || row.Output != 7) {
			t.Errorf("assignment usage lost: %+v", row)
		}
	}
	wire, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"PRIVATE_", "Privacy fixture routine", "save this file", "Open the File menu", "fixture.md"} {
		if strings.Contains(string(wire), secret) {
			t.Errorf("activity disclosed kept content %q", secret)
		}
	}
}

func TestOniActivityLast50StableAndSinceCoversCountedRows(t *testing.T) {
	d, _ := partsDaemon(t, nil)
	at := d.reflexClock().Now()
	for i := range 60 {
		d.oni.reflex.rows = append(d.oni.reflex.rows, protocol.OniReflexRow{At: at.Add(time.Duration(i) * time.Second), Intent: "volume.up", Ms: int64(i), Outcome: protocol.OniReflexDone})
	}
	d.oni.hands.rows = []protocol.OniHandsRow{{At: at.Add(59 * time.Second), Tool: "click", Outcome: protocol.OniHandsOutcomeDone}}
	first, second := activityRead(t, d), activityRead(t, d)
	if !reflect.DeepEqual(first, second) || len(first.Rows) != protocol.OniActivityRowsMax || first.Counts[protocol.OniActivityReflexLog] != 60 || !first.Since.Equal(at) {
		t.Fatalf("unstable/bounded/count/since result %+v", first)
	}
	if first.Rows[0].Store != protocol.OniActivityReflexLog || first.Rows[1].Store != protocol.OniActivityHandsLog {
		t.Fatalf("equal timestamps lost stable store ordering: %+v", first.Rows[:2])
	}
	if !slices.IsSortedFunc(first.Rows, func(a, b protocol.OniActivityRow) int { return b.At.Compare(a.At) }) {
		t.Fatal("rows are not newest first")
	}
}

func TestOniActivityClearsEachOwnerAndAllWithoutResettingBudget(t *testing.T) {
	for _, selected := range append(slices.Clone(protocol.OniActivityStores), protocol.OniActivityAll) {
		t.Run(selected, func(t *testing.T) {
			d := activitySeed(t)
			before := activityRead(t, d)
			day := d.assignLedger().Day(d.reflexClock().Now())
			// These are safety counters, not display history.
			d.oni.reflex.ran = []time.Time{d.reflexClock().Now()}
			d.oni.reflex.hosted = []time.Time{d.reflexClock().Now()}
			result, err := d.oniActivityClear(context.Background(), protocol.OniActivityClearParams{Stores: []string{selected}})
			if err != nil {
				t.Fatal(err)
			}
			for _, store := range protocol.OniActivityStores {
				want := before.Counts[store]
				if store == selected || selected == protocol.OniActivityAll {
					want = 0
				}
				if result.Counts[store] != want {
					t.Errorf("clearing %s changed %s to %d, want %d", selected, store, result.Counts[store], want)
				}
			}
			if got := d.assignLedger().Day(d.reflexClock().Now()); got != day {
				t.Errorf("privacy reset budget: before %+v after %+v", day, got)
			}
			if len(d.oni.reflex.ran) != 1 || len(d.oni.reflex.hosted) != 1 {
				t.Fatal("privacy reset quick-command safety counters")
			}
			if selected == protocol.OniActivityHandsLog || selected == protocol.OniActivityAll {
				j, _ := d.handsJournal()
				if tasks, err := j.List(); err != nil || len(tasks) != 0 {
					t.Fatalf("Hands task journal survived: %+v %v", tasks, err)
				}
			}
			if selected == protocol.OniActivityRecallImports || selected == protocol.OniActivityAll {
				if _, err := os.Lstat(recallRoot()); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("import files survived: %v", err)
				}
			}
			if selected == protocol.OniActivityAll && (len(result.Rows) != 0 || !result.Since.IsZero()) {
				t.Fatalf("cleared activity survived: %+v", result)
			}
		})
	}
}

func TestOniActivityClearCancellationInUseAndCallerSafety(t *testing.T) {
	d := activitySeed(t)
	params := protocol.OniActivityClearParams{Stores: []string{protocol.OniActivityAll}}
	before := activityRead(t, d)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := d.oniActivityClear(ctx, params); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled clear: %v", err)
	}
	if after := activityRead(t, d); !reflect.DeepEqual(after, before) {
		t.Fatal("cancelled clear deleted records")
	}
	for _, busy := range []string{"on", "voice", "assignment", "routine", "hands"} {
		switch busy {
		case "on":
			d.settings.Oni.Master = true
		case "voice":
			d.oni.voice.work = 1
		case "assignment":
			d.oni.assign.work = 1
		case "routine":
			d.oni.routine.work = 1
		case "hands":
			d.oni.hands.active.Store(true)
		}
		_, err := d.oniActivityClear(context.Background(), params)
		partCode(t, err, protocol.CodeConflict)
		d.settings.Oni.Master = false
		d.oni.voice.work, d.oni.assign.work, d.oni.routine.work = 0, 0, 0
		d.oni.hands.active.Store(false)
		if after := activityRead(t, d); !reflect.DeepEqual(after, before) {
			t.Fatalf("%s in-use clear deleted records", busy)
		}
	}
	for _, stores := range [][]string{nil, {"memory"}, {protocol.OniActivityAll, protocol.OniActivityReminders}, {protocol.OniActivityReminders, protocol.OniActivityReminders}} {
		_, err := d.oniActivityClear(context.Background(), protocol.OniActivityClearParams{Stores: stores})
		partCode(t, err, protocol.CodeBadRequest)
	}
	cli := gateConn(t, d, protocol.ClientCLI, false, "")
	app := gateConn(t, d, protocol.ClientApp, true, "")
	if _, err := d.dispatch(context.Background(), cli, protocol.MOniActivity, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("activity read still unsupported: %v", err)
	}
	if _, err := d.dispatch(context.Background(), cli, protocol.MOniActivityClear, mustMarshal(t, params)); !forbidden(err) {
		t.Fatalf("non-app clear reached a store: %v", err)
	}
	if _, err := d.dispatch(context.Background(), app, protocol.MOniActivityClear, mustMarshal(t, params)); err != nil {
		t.Fatalf("app clear still unsupported: %v", err)
	}
}

type activityIndexer struct {
	core.Indexer
	indexes []protocol.IndexInfo
	removed []string
	fail    error
}

func (i *activityIndexer) List(context.Context) ([]protocol.IndexInfo, error) {
	return slices.Clone(i.indexes), nil
}

func (i *activityIndexer) Remove(_ context.Context, ref protocol.IndexRef) error {
	if i.fail != nil {
		return i.fail
	}
	i.removed = append(i.removed, ref.ID)
	i.indexes = slices.DeleteFunc(i.indexes, func(info protocol.IndexInfo) bool { return info.ID == ref.ID })
	return nil
}

func TestOniActivityImportClearRemovesOwnedIndexesAndNoOtherRoot(t *testing.T) {
	d := activitySeed(t)
	other := t.TempDir()
	indexer := &activityIndexer{indexes: []protocol.IndexInfo{
		{ID: "imports-primary", Root: recallRoot()},
		{ID: "imports-roots", Roots: []string{recallRoot()}},
		{ID: "vault", Root: other},
	}}
	d.deps.Indexer = indexer
	export := filepath.Join(other, "original-export.json")
	if err := os.WriteFile(export, []byte("original export stays"), 0o600); err != nil {
		t.Fatal(err)
	}
	indexer.fail = errors.New("index owner refused")
	params := protocol.OniActivityClearParams{Stores: []string{protocol.OniActivityRecallImports}}
	if _, err := d.oniActivityClear(context.Background(), params); err == nil {
		t.Fatal("index failure was hidden")
	}
	if got := activityRead(t, d).Counts[protocol.OniActivityRecallImports]; got != 1 {
		t.Fatal("failed index removal deleted the imports")
	}
	indexer.fail = nil
	if _, err := d.oniActivityClear(context.Background(), params); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(indexer.removed, []string{"imports-primary", "imports-roots"}) || len(indexer.indexes) != 1 || indexer.indexes[0].ID != "vault" {
		t.Fatalf("clearing imports touched another index: %+v", indexer)
	}
	if _, err := os.Stat(export); err != nil {
		t.Fatal("clearing imports touched an original export", err)
	}
}

// A real index canonicalizes the chosen import path before registering it.
// Clearing through a symlinked XDG parent must remove its searchable store,
// not merely make the Markdown disappear under the lexical path.
func TestOniActivityImportClearRemovesCanonicalIndexThroughSymlinkedDataHome(t *testing.T) {
	d, _ := partsDaemon(t, nil)
	realData := t.TempDir()
	linkedData := filepath.Join(t.TempDir(), "linked-data")
	if err := os.Symlink(realData, linkedData); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", linkedData)
	root := recallRoot()
	if err := os.MkdirAll(filepath.Join(root, protocol.OniImportClaude), 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, protocol.OniImportClaude, "fixture.md")
	if err := os.WriteFile(file, []byte("# Local privacy fixture\n\ncanonicalprivacywitness is a local fixture word.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	engine, err := index.Open(nil, func() protocol.Settings {
		return protocol.Settings{Index: protocol.IndexSettings{Semantic: false}}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	info, err := engine.Add(context.Background(), protocol.IndexAddParams{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil || info.Root != canonical || info.Root == root {
		t.Fatalf("the real index did not canonicalize the fixture: %+v, %v", info, err)
	}
	query := protocol.SearchParams{IndexRef: protocol.IndexRef{ID: info.ID}, Query: "canonicalprivacywitness", Mode: protocol.SearchLexical, Existing: true}
	if before, err := engine.Search(context.Background(), query); err != nil || len(before.Hits) == 0 {
		t.Fatalf("fixture was not actually searchable before clear: %+v, %v", before, err)
	}
	d.deps.Indexer = engine
	if _, err := d.oniActivityClear(context.Background(), protocol.OniActivityClearParams{Stores: []string{protocol.OniActivityRecallImports}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(root); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("import Markdown was not removed: %v", err)
	}
	if after, err := engine.Search(context.Background(), query); err == nil && len(after.Hits) != 0 {
		t.Errorf("cleared imported words remain searchable through the canonical index: %+v", after.Hits)
	}
	registered, err := engine.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(registered, func(row protocol.IndexInfo) bool { return row.ID == info.ID }) {
		t.Errorf("cleared canonical import index remains registered: %+v", registered)
	}
}

func TestOniActivitySelectiveClearSurvivesUnrelatedDamagedStore(t *testing.T) {
	d := activitySeed(t)
	if err := os.WriteFile(d.oniKeeps().Path(), []byte("not a valid kept walkthrough store"), 0o600); err != nil {
		t.Fatal(err)
	}
	partial := activityRead(t, d)
	if _, inventedZero := partial.Counts[protocol.OniActivityWalkthroughs]; inventedZero || partial.Errors[protocol.OniActivityWalkthroughs] == "" {
		t.Fatalf("damaged store was not reported as unknown: %+v", partial)
	}
	metadata, err := json.Marshal(partial)
	if err != nil || strings.Contains(string(metadata), "not a valid kept walkthrough store") {
		t.Fatalf("partial error metadata leaked stored text: %s, %v", metadata, err)
	}
	if _, err := d.oniActivityClear(context.Background(), protocol.OniActivityClearParams{Stores: []string{protocol.OniActivityAll}}); err == nil {
		t.Fatal("all must refuse an unreadable selected owner")
	}
	if still := activityRead(t, d).Counts[protocol.OniActivityReminders]; still != 1 {
		t.Fatal("all partially deleted healthy reminders before detecting a damaged owner")
	}
	result, err := d.oniActivityClear(context.Background(), protocol.OniActivityClearParams{Stores: []string{protocol.OniActivityReminders}})
	if err != nil {
		t.Fatalf("an unrelated damaged walkthrough store blocked the selected reminder clear: %v", err)
	}
	if result.Counts[protocol.OniActivityReminders] != 0 || result.Counts[protocol.OniActivityRoutines] != 1 || result.Counts[protocol.OniActivityRecallImports] != 1 {
		t.Fatalf("selective clear did not preserve healthy unrelated owners: %+v", result)
	}
	if _, inventedZero := result.Counts[protocol.OniActivityWalkthroughs]; inventedZero || result.Errors[protocol.OniActivityWalkthroughs] == "" {
		t.Fatalf("post-clear refresh lost the unrelated unknown count: %+v", result)
	}
	d.oni.remind.mu.Lock()
	reminders, err := d.remindStoreLocked().List()
	d.oni.remind.mu.Unlock()
	if err != nil || len(reminders) != 0 {
		t.Fatalf("selected reminders survived clear: %+v, %v", reminders, err)
	}
}

func TestOniActivityImportClearAbsentAndSymlinkRootBoundaries(t *testing.T) {
	for _, boundary := range []string{"absent-parent", "symlink-root"} {
		t.Run(boundary, func(t *testing.T) {
			d, _ := partsDaemon(t, nil)
			realData := t.TempDir()
			linkedData := filepath.Join(t.TempDir(), "linked-data")
			if err := os.Symlink(realData, linkedData); err != nil {
				t.Fatal(err)
			}
			t.Setenv("XDG_DATA_HOME", linkedData)
			root, foreign := recallRoot(), t.TempDir()
			for _, dir := range []string{root, foreign} {
				if err := os.MkdirAll(dir, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "fixture.md"), []byte("# Fixture\n\nprivacyboundarywitness remains local.\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			engine, err := index.Open(nil, func() protocol.Settings {
				return protocol.Settings{Index: protocol.IndexSettings{Semantic: false}}
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = engine.Close() })
			ownIndex, err := engine.Add(context.Background(), protocol.IndexAddParams{Path: root})
			if err != nil {
				t.Fatal(err)
			}
			foreignIndex, err := engine.Add(context.Background(), protocol.IndexAddParams{Path: foreign})
			if err != nil {
				t.Fatal(err)
			}
			if boundary == "absent-parent" {
				if err := os.RemoveAll(filepath.Dir(root)); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.RemoveAll(root); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(foreign, root); err != nil {
					t.Fatal(err)
				}
			}
			d.deps.Indexer = engine
			if _, err := d.oniActivityClear(context.Background(), protocol.OniActivityClearParams{Stores: []string{protocol.OniActivityRecallImports}}); err != nil {
				t.Fatal(err)
			}
			registered, err := engine.List(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if slices.ContainsFunc(registered, func(row protocol.IndexInfo) bool { return row.ID == ownIndex.ID }) {
				t.Fatal("the cached import index survived its missing or replaced source")
			}
			if !slices.ContainsFunc(registered, func(row protocol.IndexInfo) bool { return row.ID == foreignIndex.ID }) {
				t.Fatal("the import root's symlink target lost its unrelated index")
			}
			query := protocol.SearchParams{IndexRef: protocol.IndexRef{ID: foreignIndex.ID}, Query: "privacyboundarywitness", Mode: protocol.SearchLexical, Existing: true}
			if result, err := engine.Search(context.Background(), query); err != nil || len(result.Hits) == 0 {
				t.Fatalf("clearing imports damaged unrelated searchable content: %+v, %v", result, err)
			}
			if _, err := os.Stat(filepath.Join(foreign, "fixture.md")); err != nil {
				t.Fatal("the import root's symlink target was followed during deletion", err)
			}
		})
	}
}

func TestOniActivityImportClearProtectsCanonicalSharedIndex(t *testing.T) {
	d := activitySeed(t)
	canonical, err := filepath.EvalSymlinks(recallRoot())
	if err != nil {
		t.Fatal(err)
	}
	indexer := &activityIndexer{indexes: []protocol.IndexInfo{{
		ID: "shared-imports", Root: canonical,
		Roots: []string{canonical, t.TempDir()},
	}}}
	d.deps.Indexer = indexer
	if _, err := d.oniActivityClear(context.Background(), protocol.OniActivityClearParams{Stores: []string{protocol.OniActivityRecallImports}}); err == nil {
		t.Fatal("clearing imports removed a shared canonical index")
	}
	if len(indexer.removed) != 0 || activityRead(t, d).Counts[protocol.OniActivityRecallImports] != 1 {
		t.Fatal("shared index refusal changed files or index registration")
	}
}

func TestOniActivityClearPreflightsActiveJournalBeforeDeletingAnyStore(t *testing.T) {
	d := activitySeed(t)
	journal, err := d.handsJournal()
	if err != nil {
		t.Fatal(err)
	}
	queued, err := journal.Enqueue(task.Task{ID: task.NewID(), Scope: protocol.OniHandsScopeWindow, App: "fixture", Origin: "app"}, d.reflexClock().Now())
	if err != nil || !queued.State.Active() {
		t.Fatalf("fixture did not start a real journal task: %+v, %v", queued, err)
	}
	// The persisted journal is authoritative even if no live Hands run or
	// active atomic flag was reconstructed in this isolated daemon.
	before := activityRead(t, d)
	_, err = d.oniActivityClear(context.Background(), protocol.OniActivityClearParams{Stores: []string{protocol.OniActivityAll}})
	partCode(t, err, protocol.CodeConflict)
	if after := activityRead(t, d); !reflect.DeepEqual(after, before) {
		t.Fatalf("an active journal was detected only after deleting earlier stores: before %+v after %+v", before, after)
	}
	if _, running, err := journal.Running(); err != nil || !running {
		t.Fatalf("the refused privacy clear changed the active task: running %t, %v", running, err)
	}
}
