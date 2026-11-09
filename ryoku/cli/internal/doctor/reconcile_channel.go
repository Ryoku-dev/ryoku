package doctor

import (
	"os"
	"path/filepath"
	"strings"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// reconcileUpdateChannel heals a box moved between update channels with
// `ryoku track`. track records the channel in ~/.config/environment.d/ryoku.conf
// and checks that branch out in the update checkout (~/ryoku-arch). A toggled or
// half-finished switch can leave the checkout on a different branch than the
// recorded channel; `ryoku update` then measures the running system against the
// wrong branch and shows updates that never clear (and can redeploy a mismatched
// tree, which is where a "shell.qml: File not found" config load comes from).
// Put the checkout back on the tracked branch so status and update agree. Only a
// clean checkout is moved -- a maintainer's uncommitted work is never touched.
func reconcileUpdateChannel(checkOnly bool) recResult {
	repo := sys.ResolveRepo()
	if repo == "" {
		return okRes(i18n.T("packaged install; no update-channel checkout to reconcile"))
	}
	ch := sys.TrackedChannel()
	if ch == "" {
		return okRes(i18n.T("no tracked update channel recorded"))
	}
	head, _ := sys.RunOut("git", "-C", repo, "symbolic-ref", "--short", "--quiet", "HEAD")
	head = strings.TrimSpace(head)
	if head == ch {
		return okRes(i18n.T("updates come from the source checkout on %q; `ryoku track stable` moves this box onto packages"), ch)
	}
	if checkOnly {
		return wouldRes(i18n.T("the update checkout %s is on %q but the tracked channel is %q; `ryoku update` measures against the wrong branch"), repo, head, ch).
			withFix("ryoku doctor")
	}
	if dirty, _ := sys.RunOut("git", "-C", repo, "status", "--porcelain", "--untracked-files=no"); strings.TrimSpace(dirty) != "" {
		return warnRes(i18n.T("%s is on %q, not the tracked channel %q, and has uncommitted changes, so its branch was left as-is"), repo, head, ch).
			withFix(i18n.T("commit or stash in %s, then run `ryoku doctor`"), repo)
	}
	_, _ = sys.RunOut("git", "-C", repo, "fetch", "origin", ch)
	if _, err := sys.RunOut("git", "-C", repo, "checkout", ch); err != nil {
		return warnRes(i18n.T("could not switch %s onto the tracked channel %q: %v"), repo, ch, err).
			withFix("%s", retrackCommand(ch))
	}
	_, _ = sys.RunOut("git", "-C", repo, "reset", "--hard", "origin/"+ch)
	return fixedRes(i18n.T("switched the update checkout onto the tracked channel %q; run `ryoku update` to redeploy"), ch)
}

// retrackCommand rebuilds a checkout on the branch it tracks when that branch
// has a `ryoku track` name. Any other branch has none, so a checkout that
// cannot follow it moves onto stable packages instead.
func retrackCommand(ch string) string {
	switch ch {
	case "unstable-dev":
		return "ryoku track unstable"
	case "main":
		return "ryoku track stable --source"
	}
	return "ryoku track stable"
}

// reconcileRepoPointer unsticks a box whose update-checkout record broke: it
// converts a symlinked state dir (the retired dev-switch layout) to a real
// directory and repoints a stale repo pointer at the ~/ryoku-arch checkout.
// Packaged installs carry no pointer, so the repoint needs a real checkout.
func reconcileRepoPointer(checkOnly bool) recResult {
	state := sys.StateDir()
	var actions []string

	if fi, err := os.Lstat(state); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		if checkOnly {
			return wouldRes(i18n.T("the state dir %s is a symlink (retired dev-switch layout); it must be a real directory or the update pointer cannot be recorded"), state).
				withFix("ryoku doctor")
		}
		if err := desymlinkStateDir(state); err != nil {
			return failRes(i18n.T("could not convert the state dir symlink %s to a directory: %v"), state, err).
				withFix(i18n.T("remove the symlink and recreate %s as a directory by hand"), state)
		}
		actions = append(actions, i18n.T("converted the state dir symlink to a real directory"))
	}

	repoFile := filepath.Join(state, "repo")
	recorded := ""
	if b, err := os.ReadFile(repoFile); err == nil {
		recorded = strings.TrimSpace(string(b))
	}
	track := filepath.Join(sys.Home(), "ryoku-arch")
	// Only adopt ~/ryoku-arch for a box that still opts into source tracking (a
	// recorded RYOKU_CHANNEL). A box migrated onto packages carries no tracked
	// channel and left its clone on disk deliberately; re-adopting it would undo
	// the migration.
	if sys.TrackedChannel() != "" && !isRyokuArchTree(recorded) && isRyokuArchTree(track) {
		if checkOnly {
			what := i18n.T("missing")
			if recorded != "" {
				what = i18n.Tf("stale (%s)", recorded)
			}
			return wouldRes(i18n.T("the update-checkout pointer is %s but the tracking checkout at %s is healthy; update/status report no channel and never advance"), what, track).
				withFix("ryoku doctor")
		}
		if err := os.MkdirAll(state, 0o755); err != nil {
			return failRes(i18n.T("could not create the state dir %s: %v"), state, err)
		}
		if err := os.WriteFile(repoFile, []byte(track+"\n"), 0o644); err != nil {
			return failRes(i18n.T("could not record the update-checkout pointer at %s: %v"), repoFile, err)
		}
		actions = append(actions, i18n.Tf("repointed the update checkout to %s", track))
	}

	if len(actions) == 0 {
		return okRes(i18n.T("update-checkout pointer and state dir are healthy"))
	}
	return fixedRes(i18n.T("%s; run `ryoku update` to advance"), strings.Join(actions, "; "))
}

// isRyokuArchTree reports whether p is a git work tree whose origin is a
// ryoku-arch remote, so a stray unrelated repo is never adopted as the channel.
func isRyokuArchTree(p string) bool {
	if p == "" {
		return false
	}
	if _, err := sys.RunOut("git", "-C", p, "rev-parse", "--git-dir"); err != nil {
		return false
	}
	url, err := sys.RunOut("git", "-C", p, "remote", "get-url", "origin")
	return err == nil && strings.Contains(url, "ryoku-arch")
}

// desymlinkStateDir turns a symlinked state dir into a real one, migrating the
// (possibly dangling) target's contents so the recorded pointer is not lost.
func desymlinkStateDir(state string) error {
	target, _ := filepath.EvalSymlinks(state)
	if err := os.Remove(state); err != nil {
		return err
	}
	if err := os.MkdirAll(state, 0o755); err != nil {
		return err
	}
	if target == "" || target == state {
		return nil
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		dst := filepath.Join(state, e.Name())
		if sys.Exists(dst) {
			continue
		}
		_ = sys.Run("cp", "-a", filepath.Join(target, e.Name()), dst)
	}
	return nil
}
