package daemon

import (
	"context"
	"errors"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/dictate/target"
	"ryoku-rashin/internal/oni/window"
)

// rewriteReady never focuses a window. An app button may wait for the person
// to return focus, bounded by the existing target timeout and window events.
// Any identity/title/output/geometry change fails closed, not a new target.
func (d *Daemon) rewriteReady(parent context.Context, rec *rewriteRecord, handback bool) error {
	ctx, cancel := context.WithTimeout(parent, oniTargetTimeout)
	defer cancel()
	check := func() error {
		w, ok := rewriteWindow(ctx, rec.windows, rec.target.Window)
		sameBox := w.Box == nil && rec.window.Box == nil || w.Box != nil && rec.window.Box != nil && *w.Box == *rec.window.Box
		if !ok || w.App != rec.window.App || w.Title != rec.window.Title || w.Output != rec.window.Output || !sameBox {
			return errors.New("rewrite: original target changed")
		}
		return rec.target.Verify(ctx, rec.windows)
	}
	err := check()
	if err == nil || !handback || target.ReasonOf(err) != target.ReasonUnfocused || !rec.windows.Caps().Events {
		return err
	}
	d.rewriteSay("answer", i18n.T("Return to the original window now. Hyakki will not focus it for you."))
	ready := false
	watchErr := rec.windows.Watch(ctx, func(window.Snapshot) {
		err = check()
		if err == nil {
			ready = true
			cancel()
		} else if target.ReasonOf(err) != target.ReasonUnfocused {
			cancel()
		}
	})
	if ready {
		return nil
	}
	if err != nil {
		return err
	}
	return watchErr
}
