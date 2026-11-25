package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/imports/chats"
	"ryoku-rashin/internal/protocol"
)

// oniActivity reads the actual owners, including when Oni is off. No new
// activity file is made. Only ledger codes, timings and usage cross this API;
// kept text, task goals, titles and imported passages never do.
func (d *Daemon) oniActivity(ctx context.Context) (protocol.OniActivityResult, error) {
	out := protocol.OniActivityResult{Counts: make(map[string]int, len(protocol.OniActivityStores)), Rows: []protocol.OniActivityRow{}}
	for _, store := range protocol.OniActivityStores {
		out.Counts[store] = 0
	}
	r := &d.oni.reflex
	r.mu.Lock()
	out.Counts[protocol.OniActivityReflexLog] = len(r.rows)
	for i := len(r.rows) - 1; i >= 0; i-- {
		row := r.rows[i]
		// Reflex's ledger does not meter model tokens. Unknown is not free.
		out.Rows = append(out.Rows, protocol.OniActivityRow{At: row.At, Store: protocol.OniActivityReflexLog, Action: row.Intent, Route: row.Route, Ms: row.Ms, Unmetered: true, Outcome: row.Outcome})
	}
	r.mu.Unlock()
	h := &d.oni.hands
	h.rowsMu.Lock()
	out.Counts[protocol.OniActivityHandsLog] = len(h.rows)
	for i := len(h.rows) - 1; i >= 0; i-- {
		row := h.rows[i]
		out.Rows = append(out.Rows, protocol.OniActivityRow{At: row.At, Store: protocol.OniActivityHandsLog, Action: row.Tool, Ms: row.Ms, Unmetered: true, Outcome: row.Outcome})
	}
	h.rowsMu.Unlock()
	for _, row := range d.assignLedger().Rows(d.reflexClock().Now()) {
		out.Counts[protocol.OniActivityAssignment]++
		out.Rows = append(out.Rows, protocol.OniActivityRow{At: row.At, Store: protocol.OniActivityAssignment, Action: row.Lane, Route: row.Route, Input: row.Input, Output: row.Output, Estimated: row.Estimated, Unmetered: row.Unmetered, Outcome: row.Outcome})
	}
	keeps, err := d.oniKeeps().List()
	if err == nil {
		err = d.oniHints().Check()
	}
	activityCount(&out, protocol.OniActivityWalkthroughs, len(keeps), err)
	routines, err := d.oniRoutines().List()
	activityCount(&out, protocol.OniActivityRoutines, len(routines), err)
	d.oni.remind.mu.Lock()
	reminders, err := d.remindStoreLocked().List()
	d.oni.remind.mu.Unlock()
	activityCount(&out, protocol.OniActivityReminders, len(reminders), err)
	d.oni.recall.importMu.Lock()
	imports, err := chats.Count(ctx, recallRoot())
	d.oni.recall.importMu.Unlock()
	activityCount(&out, protocol.OniActivityRecallImports, imports, err)
	// Stable ties keep store order, then each owner's newest-first order.
	slices.SortStableFunc(out.Rows, func(a, b protocol.OniActivityRow) int { return b.At.Compare(a.At) })
	if n := len(out.Rows); n > 0 {
		out.Since = out.Rows[n-1].At
	}
	if len(out.Rows) > protocol.OniActivityRowsMax {
		out.Rows = out.Rows[:protocol.OniActivityRowsMax]
	}
	return out, ctx.Err()
}

// A failed owner contributes no invented zero and no raw error that could
// contain file contents, titles or passages. Healthy owners remain usable.
func activityCount(out *protocol.OniActivityResult, store string, count int, err error) {
	if err == nil {
		out.Counts[store] = count
		return
	}
	delete(out.Counts, store)
	if out.Errors == nil {
		out.Errors = make(map[string]string)
	}
	out.Errors[store] = i18n.T("These local records could not be read. Their count is unknown. Repair or remove this store in Optional parts before clearing it.")
}

// oniActivityClear is app-only at dispatch and behind Settings' confirmation.
// Turning Oni off first prevents new work; outstanding workers must finish
// before records go. Settings mutation is excluded throughout the operation.
// A cancelled request deletes nothing before preflight; cancellation between
// owners may leave earlier owners cleared, which the next read reports.
func (d *Daemon) oniActivityClear(ctx context.Context, p protocol.OniActivityClearParams) (protocol.OniActivityResult, error) {
	if !protocol.ValidOniActivityStores(p.Stores) {
		return protocol.OniActivityResult{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Name each record to clear once, or all of them."))
	}
	d.settingsMu.Lock()
	defer d.settingsMu.Unlock()
	if protocol.EffectiveOni(d.Settings()).Master {
		return protocol.OniActivityResult{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Turn Oni off before clearing its records. Nothing has been cleared."))
	}
	d.oni.voice.mu.Lock()
	busy := d.oni.voice.work != 0
	d.oni.voice.mu.Unlock()
	a := &d.oni.assign
	a.mu.Lock()
	busy = busy || a.work != 0 || a.running || a.card != nil
	a.mu.Unlock()
	d.oni.routine.mu.Lock()
	busy = busy || d.oni.routine.work != 0
	d.oni.routine.mu.Unlock()
	if busy || d.oni.hands.active.Load() {
		return protocol.OniActivityResult{}, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Oni is finishing work. Wait for it to stop, then clear the records. Nothing has been cleared."))
	}
	stores := p.Stores
	if stores[0] == protocol.OniActivityAll {
		stores = protocol.OniActivityStores
	}
	// Preflight only the selected owners. An unreadable unrelated owner
	// remains unknown in the result, never a reason to block this clear.
	for _, store := range stores {
		if err := d.activityPreflight(ctx, store); err != nil {
			return protocol.OniActivityResult{}, err
		}
	}
	for _, store := range stores {
		if err := ctx.Err(); err != nil {
			return protocol.OniActivityResult{}, err
		}
		if err := d.activityClearStore(ctx, store); err != nil {
			return protocol.OniActivityResult{}, err
		}
	}
	d.stateChanged()
	return d.oniActivity(ctx)
}

func (d *Daemon) activityClearStore(ctx context.Context, store string) error {
	switch store {
	case protocol.OniActivityReflexLog:
		r := &d.oni.reflex
		r.mu.Lock()
		clear(r.rows)
		r.rows = nil
		r.mu.Unlock()
	case protocol.OniActivityHandsLog:
		// The task journal is the Hands owner's persistent history. Its
		// clear also removes kept task projects, never the browser profile.
		if _, err := d.handsClearCall(); err != nil {
			return err
		}
		h := &d.oni.hands
		h.rowsMu.Lock()
		clear(h.rows)
		h.rows = nil
		h.rowsMu.Unlock()
	case protocol.OniActivityAssignment:
		d.assignLedger().ClearRows(d.reflexClock().Now())
	case protocol.OniActivityWalkthroughs:
		if err := d.oniKeeps().ForgetAll(); err != nil {
			return err
		}
		return d.oniHints().ForgetAll()
	case protocol.OniActivityRoutines:
		return d.oniRoutines().ForgetAll()
	case protocol.OniActivityReminders:
		r := &d.oni.remind
		r.mu.Lock()
		d.remindDisarmLocked()
		store := d.remindStoreLocked()
		store.CancelAll()
		left, err := store.List()
		r.mu.Unlock()
		if err != nil {
			return err
		}
		if len(left) != 0 {
			return protocol.Errorf(protocol.CodeInternal, "%s", i18n.T("The reminders could not be cleared."))
		}
		d.remindHoldWake()
	case protocol.OniActivityRecallImports:
		d.oni.recall.importMu.Lock()
		defer d.oni.recall.importMu.Unlock()
		ids, err := d.activityImportIndexes(ctx)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if err := d.deps.Indexer.Remove(ctx, protocol.IndexRef{ID: id}); err != nil {
				return err
			}
		}
		return chats.ForgetAll(recallRoot())
	}
	return nil
}

// Workers that have not begun when Oni is turned off never send or append
// activity after a clear; workers already sending keep the clear refused.
func (d *Daemon) activityAssignBegin() bool {
	a := &d.oni.assign
	a.mu.Lock()
	defer a.mu.Unlock()
	if assignRefusal(protocol.EffectiveOni(d.Settings())) != nil {
		return false
	}
	a.work++
	return true
}

func (d *Daemon) activityAssignEnd() {
	a := &d.oni.assign
	a.mu.Lock()
	a.work--
	a.mu.Unlock()
}

func (d *Daemon) activityPreflight(ctx context.Context, store string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	switch store {
	case protocol.OniActivityHandsLog:
		journal, err := d.handsJournal()
		if err != nil {
			return err
		}
		_, running, err := journal.Running()
		if err != nil {
			return err
		}
		if running {
			return protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("Hands still has an active task in its journal. Stop it before clearing records. Nothing has been cleared."))
		}
		return nil
	case protocol.OniActivityWalkthroughs:
		if _, err := d.oniKeeps().List(); err != nil {
			return err
		}
		return d.oniHints().Check()
	case protocol.OniActivityRoutines:
		_, err := d.oniRoutines().List()
		return err
	case protocol.OniActivityReminders:
		d.oni.remind.mu.Lock()
		_, err := d.remindStoreLocked().List()
		d.oni.remind.mu.Unlock()
		return err
	case protocol.OniActivityRecallImports:
		d.oni.recall.importMu.Lock()
		defer d.oni.recall.importMu.Unlock()
		if _, err := chats.Count(ctx, recallRoot()); err != nil {
			return err
		}
		_, err := d.activityImportIndexes(ctx)
		return err
	}
	return nil
}

// The index engine registers canonical paths. Resolve only parents for
// identity: the final import-root entry may now be a symlink to somebody
// else's folder, which clearing must neither follow nor claim as its index.
// Missing trailing directories are retained so cached indexes can still be
// removed after their original source directory disappeared.
func activityRootIdentity(root string) (string, error) {
	root = filepath.Clean(root)
	parent := filepath.Dir(root)
	var missing []string
	for {
		canonical, err := filepath.EvalSymlinks(parent)
		if err == nil {
			for i := len(missing) - 1; i >= 0; i-- {
				canonical = filepath.Join(canonical, missing[i])
			}
			return filepath.Join(canonical, filepath.Base(root)), nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		// A dangling existing symlink is not a missing directory whose
		// literal name may safely be appended to a canonical ancestor.
		if info, statErr := os.Lstat(parent); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return "", err
		}
		next := filepath.Dir(parent)
		if next == parent {
			return "", err
		}
		missing = append(missing, filepath.Base(parent))
		parent = next
	}
}

// The caller holds importMu. Match every declared root under the same
// parent-canonical identity before removing anything; a mixed-root index is
// refused even if its primary Root uses a different spelling.
func (d *Daemon) activityImportIndexes(ctx context.Context) ([]string, error) {
	if d.deps.Indexer == nil {
		return nil, nil
	}
	owned, err := activityRootIdentity(recallRoot())
	if err != nil {
		return nil, err
	}
	indexes, err := d.deps.Indexer.List(ctx)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, info := range indexes {
		matched, other := false, false
		classify := func(root string) {
			if root == "" {
				return
			}
			if root == recallRoot() || root == owned {
				matched = true
				return
			}
			identity, err := activityRootIdentity(root)
			if err == nil && identity == owned {
				matched = true
			} else {
				other = true
			}
		}
		classify(info.Root)
		for _, root := range info.Roots {
			classify(root)
		}
		if matched && other {
			return nil, protocol.Errorf(protocol.CodeConflict, "%s", i18n.T("The imported chats share an index with another folder. Separate that index before clearing the imports."))
		}
		if matched {
			ids = append(ids, info.ID)
		}
	}
	return ids, ctx.Err()
}
