package updater

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"ryoku-cli/internal/sys"
	i18n "ryoku-i18n"
	"strconv"
	"strings"
	"time"
)

// Update channel = a git branch (main for everyone) a Ryoku checkout tracks.
// `ryoku status` reports how far the deployed commit (the running system) is
// behind origin/<channel>; `ryoku update` pulls the channel in and redeploys.
// A packaged install has no checkout, so these report "no channel" and the
// caller falls back to the pacman view of the [ryoku] repo.

// ryokuChannel: the channel update tracks. A packaged box's channel is
// authoritative -- the [ryoku] Server it points at (stable, testing, or a
// pinned release) -- and wins over a RYOKU_CHANNEL a source box left in
// environment.d before it was migrated onto packages, so a migrated box
// reports testing, not the stale unstable-dev. A source checkout follows the
// branch `ryoku track --source` persisted to environment.d, which wins over the
// stale login env; a box that never tracked follows main.
func ryokuChannel() string {
	if sys.ResolveRepo() == "" {
		if channel := packagedChannel(); channel != "" {
			return channel
		}
	}
	if c := sys.TrackedChannel(); c != "" {
		return c
	}
	if c := strings.TrimSpace(os.Getenv("RYOKU_CHANNEL")); c != "" {
		return c
	}
	return "main"
}

// deployedFile records the commit the last deploy laid down. The channel is
// measured from that, not from whatever branch happens to be checked out, so
// a commit pushed upstream shows as an update until the machine redeploys onto it.
func deployedFile() string {
	return filepath.Join(sys.Xdg("XDG_STATE_HOME", ".local/state"), "ryoku", "deployed")
}

// deployedBase returns the recorded deployed commit if it still resolves in
// repo, else HEAD (a checkout that has never deployed through this CLI).
// Baseline for the channel comparison: what is running.
func deployedBase(repo string) string {
	if b, err := os.ReadFile(deployedFile()); err == nil {
		if c := strings.TrimSpace(string(b)); c != "" {
			if _, err := sys.RunOut("git", "-C", repo, "rev-parse", "--verify", "--quiet", c+"^{commit}"); err == nil {
				return c
			}
		}
	}
	return "HEAD"
}

// channelStatus reports how far the deployed commit is behind the channel:
// the commits origin/<channel> has that the running system doesn't. ok=false
// when there's no checkout to track or the remote has no such branch, so the
// caller falls back to the pacman view. Fetch is best-effort and bounded so
// an offline or auth-walled remote never hangs a status query; on a fetch
// failure the cached remote-tracking ref stands.
func channelStatus() (statusReport, bool) {
	repo := sys.ResolveRepo()
	if repo == "" {
		return statusReport{}, false
	}
	ch := ryokuChannel()
	remote := "refs/remotes/origin/" + ch

	gitFetch(repo, ch)

	if _, err := sys.RunOut("git", "-C", repo, "rev-parse", "--verify", "--quiet", remote); err != nil {
		return statusReport{}, false
	}
	base := deployedBase(repo)
	behind := gitCount(repo, base+".."+remote)

	// Version = the channel's latest commit, so the Hub matches what main
	// shows. When behind, surface the running commit too, so the header reads
	// "running X -> latest Y"; when current, both are the channel tip.
	latest := gitShort(repo, remote)
	installed := latest
	if behind > 0 {
		installed = gitShort(repo, base)
	}
	snaps, snapsKnown := snapshotCount()
	return statusReport{
		Installed:      installed,
		Latest:         latest,
		Available:      behind > 0,
		Behind:         behind,
		Updates:        gitLog(repo, base+".."+remote),
		Channel:        ch,
		Snapshots:      snaps,
		SnapshotsKnown: snapsKnown,
	}, true
}

// channelUpdate brings the checkout up to origin/<channel> and redeploys: the
// git equivalent of a package upgrade. The caller only reaches here on a
// checkout (a packaged install has no repo to track).
func channelUpdate() error {
	repo := sys.ResolveRepo()
	if repo == "" {
		return fmt.Errorf(i18n.T("no Ryoku checkout to update"))
	}
	ch := ryokuChannel()

	progress.at("channel")
	progress.detailf(i18n.T("Fetching origin/%s"), ch)
	gitFetch(repo, ch)
	// report what the sync actually did; "Update complete" alone hid a box that
	// redeployed the same commit every time.
	before := gitShort(repo, "HEAD")
	if err := syncChannel(repo, ch); err != nil {
		return err
	}
	if after := gitShort(repo, "HEAD"); after != before {
		progress.logf(i18n.T("Advanced %s → %s (v%s %s)"), before, after, readVersion(repo), ch)
	} else {
		progress.logf(i18n.T("Already on the latest %s (v%s, %s)"), ch, readVersion(repo), before)
	}

	progress.at("deploy")
	progress.detailf(i18n.T("Deploying the desktop from the checkout"))
	if err := sys.Run(filepath.Join(repo, "ryoku", "shell", "deploy.sh")); err != nil {
		return fmt.Errorf(i18n.T("deploy from %s failed: %w"), repo, err)
	}
	return nil
}

// readVersion reads the checkout's VERSION file (e.g. 0.50.8-beta.19), the
// release bump every push carries, so the update names the version, not just the
// commit. "?" when absent.
func readVersion(repo string) string {
	b, err := os.ReadFile(filepath.Join(repo, "VERSION"))
	if err != nil {
		return "?"
	}
	return strings.TrimSpace(string(b))
}

// syncChannel advances a clean checkout onto origin/<ch> when that is a lossless
// fast-forward, whatever branch is checked out, so `ryoku update` clears "behind"
// on a dev box tracking unstable-dev exactly as it does on main, not only when
// the branch is named <ch>. A branch that already contains the channel tip
// (current or ahead) is left alone; a diverged branch keeps its own commits,
// except the channel branch, which mirrors upstream and is reset onto it. Local
// edits to tracked files are never touched: the deploy runs on what is checked
// out. Every path that declines to advance logs why, so a skip is not mistaken
// for a successful update.
func syncChannel(repo, ch string) error {
	remote := "refs/remotes/origin/" + ch
	// No channel ref to track (offline first run, or the branch is gone): deploy
	// what is checked out rather than guess.
	if _, err := sys.RunOut("git", "-C", repo, "rev-parse", "--verify", "--quiet", remote); err != nil {
		progress.logf(i18n.T("No origin/%s to track (offline, or the branch is gone); deploying the checkout as-is"), ch)
		return nil
	}
	// untracked files don't block a fast-forward (git refuses one that would
	// overwrite them, which surfaces as a real error below). Counting them as
	// dirty froze boxes: one stray build artifact and update redeployed the same
	// commit forever.
	if dirty, _ := sys.RunOut("git", "-C", repo, "status", "--porcelain", "--untracked-files=no"); strings.TrimSpace(dirty) != "" {
		progress.logf(i18n.T("Uncommitted changes in %s; staying on %s (commit or stash them, then update again)"),
			repo, gitShort(repo, "HEAD"))
		return nil
	}
	// HEAD already contains the channel tip (current or ahead): nothing to pull.
	if isAncestor(repo, remote, "HEAD") {
		return nil
	}
	// HEAD is strictly behind on a straight line: fast-forward onto the channel.
	if isAncestor(repo, "HEAD", remote) {
		if err := sys.Run("git", "-C", repo, "merge", "--ff-only", remote); err != nil {
			// usually an untracked file the incoming commits also add; git names it above.
			return fmt.Errorf(i18n.T("fast-forward to origin/%s failed (see git's message above; "+
				"move the colliding file out of %s, then update again): %w"), ch, repo, err)
		}
		return nil
	}
	// Diverged: HEAD holds commits origin/<ch> lacks. The channel branch mirrors
	// upstream, so reset it; any other branch keeps its work (a maintainer mid-dev).
	head, _ := sys.RunOut("git", "-C", repo, "symbolic-ref", "--short", "--quiet", "HEAD")
	if strings.TrimSpace(head) == ch {
		progress.logf(i18n.T("Channel history diverged; reconciling %s onto origin/%s"), ch, ch)
		if err := sys.Run("git", "-C", repo, "reset", "--hard", remote); err != nil {
			return fmt.Errorf(i18n.T("reconcile to origin/%s failed: %w"), ch, err)
		}
	}
	return nil
}

// isAncestor reports whether commit a is contained in b's history. A bad ref
// reports false, so the caller falls through safely.
func isAncestor(repo, a, b string) bool {
	_, err := sys.RunOut("git", "-C", repo, "merge-base", "--is-ancestor", a, b)
	return err == nil
}

// gitFetch updates the remote-tracking ref for one branch, best-effort.
// Never prompts for credentials and is bounded, so a private or unreachable
// remote fails fast and the cached ref stands.
func gitFetch(repo, branch string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", repo, "fetch", "--quiet", "--no-tags", "origin", branch)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	_ = cmd.Run()
}

func gitCount(repo, rng string) int {
	out, err := sys.RunOut("git", "-C", repo, "rev-list", "--count", rng)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(out))
	return n
}

// gitShort: abbreviated commit hash of ref, the bare identifier the Hub
// shows as the version. 7-char floor matches GitHub's short hashes (git
// extends if a collision ever appears, as GitHub does).
func gitShort(repo, ref string) string {
	out, err := sys.RunOut("git", "-C", repo, "rev-parse", "--short=7", ref)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// gitLog: commits in rng newest-first as display rows. Subject in Name,
// short hash in New (a commit has no from/to pair, so Old stays empty).
func gitLog(repo, rng string) []updateItem {
	ups := []updateItem{}
	out, err := sys.RunOut("git", "-C", repo, "log", "--abbrev=7", "--format=%h%x1f%s", rng)
	if err != nil {
		return ups
	}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		f := strings.SplitN(sc.Text(), "\x1f", 2)
		if len(f) == 2 {
			ups = append(ups, updateItem{Name: f[1], New: f[0]})
		}
	}
	return ups
}
