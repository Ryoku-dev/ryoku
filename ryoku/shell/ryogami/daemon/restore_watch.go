package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	wm "ryoku-wm"
)

// At login the wallpaper file (a late mount) or the outputs (a late monitor)
// can lag the daemon; a one-shot restore then left the grey default for the
// session. retryRestore covers the first, watchOutputs the second.

const (
	restoreRetryWindow   = 30 * time.Second
	restoreRetryInterval = 1 * time.Second
)

func (d *daemon) retryRestore() {
	deadline := time.Now().Add(restoreRetryWindow)
	for time.Now().Before(deadline) {
		time.Sleep(restoreRetryInterval)
		if want, applied := d.restoreOutputs(); want == 0 || applied > 0 {
			return
		}
	}
	// The window covers a late mount or a finishing download; a choice still
	// missing after it is a dead path, and grey for the session is worse than
	// the default.
	d.restoreFallback()
}

// restoreFallback repaints the stored outputs against the default wallpaper and
// records it, so a dead choice heals at login instead of greying every time.
func (d *daemon) restoreFallback() {
	if want, applied := d.restoreOutputs(); want == 0 || applied > 0 {
		return // it landed as the window closed, or nothing was ever recorded
	}
	src := d.defaultWallpaper()
	if src == "" {
		return // nothing installed to fall back to
	}
	path := filepath.Join(d.config().cacheDir(), "outputs.json")
	state := map[string]map[string]interface{}{}
	loadJSON(path, &state)
	if len(state) == 0 {
		return
	}
	for _, e := range state {
		e["path"] = src
	}
	saveJSON(path, state)
	if _, applied := d.restoreOutputs(); applied > 0 {
		fmt.Fprintf(os.Stderr, "ryogami: recorded wallpaper never arrived; painted %s\n",
			filepath.Base(src))
	}
}

// watchOutputs restores the wallpaper onto outputs that appear after startup: a
// monitor plugged in, or a panel that probes late at login. The seam's watch
// carries the full output list on every change, so a frame whose set grew since
// the last is the signal the old monitoradded event was. It also feeds the
// shared output cache the video/upscale sizing reads. watch returns on a
// compositor exit, so it reconnects with the same backoff.
func (d *daemon) watchOutputs() {
	for {
		var previous map[string]bool
		// Outputs only: the provider then skips the window and workspace reads
		// it would otherwise do on every window event.
		_ = wmClient.WatchKinds(context.Background(), []wm.FrameKind{wm.FrameOutputs}, func(f wm.Frame) {
			if f.Kind != wm.FrameOutputs {
				return
			}
			outputs.set(f.Outputs)
			current := map[string]bool{}
			for _, output := range f.Outputs {
				current[output.Name] = true
			}
			if previous != nil && d.config().restoreEnabled() {
				var added []string
				for name := range current {
					if !previous[name] {
						added = append(added, name)
					}
				}
				if len(added) > 0 {
					d.restoreOutputTargets(added)
					for _, name := range added {
						if target, err := d.workspaces.current(name); err == nil {
							_ = d.applyWorkspaceTarget(target, "init")
						}
					}
				}
			}
			previous = current
		})
		time.Sleep(restoreRetryInterval)
	}
}
