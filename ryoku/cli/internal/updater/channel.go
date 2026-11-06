package updater

import (
	"bufio"
	"context"
	"encoding/json"
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

type channelReport struct {
	Channel string `json:"channel"`
	Display string `json:"display"`
	Stable  bool   `json:"stable"`
}

// Channel reports the effective source or package channel without contacting
// the network.
func Channel(args []string) error {
	asJSON := false
	switch {
	case len(args) == 0:
	case len(args) == 1 && args[0] == "--json":
		asJSON = true
	default:
		return fmt.Errorf(i18n.T("usage: ryoku channel [--json]"))
	}

	channel := ryokuChannel()
	display := sys.DisplayChannel(channel)
	if channel == "unstable-dev" {
		display = sys.ChannelUnstable
	}
	report := channelReport{
		Channel: channel,
		Display: display,
		Stable: channel == "main" || channel == sys.ChannelStable ||
			strings.HasPrefix(channel, "v"),
	}
	if asJSON {
		return json.NewEncoder(os.Stdout).Encode(report)
	}
	_, err := fmt.Fprintln(os.Stdout, report.Display)
	return err
}

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

// channelUpdate advances a checkout only as far as the testing gate permits,
// then redeploys it unless that would roll the desktop back. Stable source
// tracking follows main directly.
func channelUpdate() error {
	repo := sys.ResolveRepo()
	if repo == "" {
		return fmt.Errorf(i18n.T("no Ryoku checkout to update"))
	}
	ch := ryokuChannel()

	progress.at("channel")
	progress.detailf(i18n.T("Fetching origin/%s"), ch)
	gitFetch(repo, ch)

	gated := ch == "unstable-dev" && !updateForce
	var pointer checkedPointer
	pointerOK := false
	if gated {
		pointer, pointerOK = readCheckedPointer()
		if updateWait {
			pointer, pointerOK = waitForChannelCheck(repo, ch, pointer, pointerOK)
		}
	}
	target, note := resolveChannelTarget(repo, ch, pointer, pointerOK, updateForce)
	if note != "" {
		progress.logf("%s", note)
	}
	// Without the gate nothing new may be taken, and the checkout can sit far
	// behind the desktop that runs: deploying it anyway once rolled a box back a
	// week while it was offline, removing the default bar.
	if gated && !pointerOK {
		progress.logf(i18n.T("Keeping the desktop that is running; nothing was deployed"))
		return nil
	}

	// Report what the sync actually did; "Update complete" alone hid a box that
	// redeployed the same commit every time.
	before := gitShort(repo, "HEAD")
	if err := syncChannel(repo, ch, target); err != nil {
		return err
	}
	if after := gitShort(repo, "HEAD"); after != before {
		progress.logf(i18n.T("Advanced %s → %s (v%s %s)"), before, after, readVersion(repo), ch)
	} else if sameCommit(repo, target, "refs/remotes/origin/"+ch) {
		progress.logf(i18n.T("Already on the latest %s (v%s, %s)"), ch, readVersion(repo), before)
	} else {
		progress.logf(i18n.T("Already on the latest checked %s commit (v%s, %s)"), ch, readVersion(repo), before)
	}

	// The last checked commit can trail one already deployed, from another
	// checkout or by hand. Laying it down would roll that work back, so the
	// desktop waits until the gate reaches what it runs.
	if gated {
		if running := deployedBase(repo); running != "HEAD" && !sameCommit(repo, "HEAD", running) &&
			isAncestor(repo, "HEAD", running) {
			progress.logf(i18n.T("The desktop already runs %s, newer than %s; nothing was deployed"),
				gitShort(repo, running), gitShort(repo, "HEAD"))
			return nil
		}
	}

	progress.at("deploy")
	progress.detailf(i18n.T("Deploying the desktop from the checkout"))
	if err := sys.Run(filepath.Join(repo, "ryoku", "shell", "deploy.sh")); err != nil {
		return fmt.Errorf(i18n.T("deploy from %s failed: %w"), repo, err)
	}
	return nil
}

// resolveChannelTarget chooses a local git target without changing the checkout.
// Only unstable-dev is gate-backed; stable source tracking follows its tip.
func resolveChannelTarget(repo, ch string, p checkedPointer, pointerOK, force bool) (target string, note string) {
	tip := "refs/remotes/origin/" + ch
	if force || ch != "unstable-dev" {
		return tip, ""
	}
	if !pointerOK {
		return "HEAD", fmt.Sprintf(i18n.T("cannot read the testing gate; staying on %s"), gitShort(repo, "HEAD"))
	}

	if p.State == "passed" && (sameCommit(repo, p.Commit, tip) || onlyReleaseBumps(repo, p.Commit, tip)) {
		return tip, ""
	}

	target = lastCheckedTarget(repo, tip, p)
	checked := gitShort(repo, target)
	switch {
	case p.State == "checking" && sameCommit(repo, p.Commit, tip):
		minutes := pointerAgeMinutes(p.At)
		return target, fmt.Sprintf(i18n.T("unstable-dev moved to %s %d min ago; its checks are still running (%s). Deploying the last checked commit %s"),
			gitShort(repo, tip), minutes, p.Run, checked)
	case p.State == "failed" && sameCommit(repo, p.Commit, tip):
		return target, fmt.Sprintf(i18n.T("%s failed its checks (%s); staying on the last checked commit %s"),
			gitShort(repo, tip), p.Run, checked)
	default:
		return target, fmt.Sprintf(i18n.T("the testing gate does not cover unstable-dev at %s; staying on the last checked commit %s"),
			gitShort(repo, tip), checked)
	}
}

func waitForChannelCheck(repo, ch string, p checkedPointer, ok bool) (checkedPointer, bool) {
	tip := "refs/remotes/origin/" + ch
	if !ok || p.State != "checking" || !sameCommit(repo, p.Commit, tip) {
		return p, ok
	}

	deadline := time.Now().Add(25 * time.Minute)
	nextLog := time.Now().Add(time.Minute)
	for time.Now().Before(deadline) {
		delay := buildPollInterval
		if remaining := time.Until(deadline); delay > remaining {
			delay = remaining
		}
		time.Sleep(delay)

		p, ok = readCheckedPointer()
		if !ok || p.State != "checking" || !sameCommit(repo, p.Commit, tip) {
			return p, ok
		}
		now := time.Now()
		if !now.Before(nextLog) {
			progress.logf(i18n.T("Checks for unstable-dev at %s are still running (%s)"), gitShort(repo, tip), p.Run)
			for !nextLog.After(now) {
				nextLog = nextLog.Add(time.Minute)
			}
		}
	}
	return p, ok
}

func sameCommit(repo, a, b string) bool {
	left, err := sys.RunOut("git", "-C", repo, "rev-parse", "--verify", a+"^{commit}")
	if err != nil {
		return false
	}
	right, err := sys.RunOut("git", "-C", repo, "rev-parse", "--verify", b+"^{commit}")
	return err == nil && strings.TrimSpace(left) == strings.TrimSpace(right)
}

func onlyReleaseBumps(repo, checked, tip string) bool {
	if checked == "" || !isAncestor(repo, checked, tip) {
		return false
	}
	out, err := sys.RunOut("git", "-C", repo, "log", "--format=%an%x00%s", checked+".."+tip)
	if err != nil || strings.TrimSpace(out) == "" {
		return false
	}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		fields := strings.SplitN(sc.Text(), "\x00", 2)
		if len(fields) != 2 || fields[0] != "github-actions[bot]" ||
			!strings.HasPrefix(fields[1], "release: bump unstable to") {
			return false
		}
	}
	return sc.Err() == nil
}

func lastCheckedTarget(repo, tip string, p checkedPointer) string {
	checked := p.Passed.Commit
	if p.State == "passed" {
		checked = p.Commit
	}
	if checked == "" || !isAncestor(repo, checked, tip) ||
		!isAncestor(repo, "HEAD", checked) || sameCommit(repo, "HEAD", checked) {
		return "HEAD"
	}
	return checked
}

func pointerAgeMinutes(at string) int {
	started, err := time.Parse(time.RFC3339, at)
	if err != nil {
		return 0
	}
	minutes := int(time.Since(started).Minutes())
	if minutes < 0 {
		return 0
	}
	return minutes
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

// syncChannel advances a clean checkout onto target when that is a lossless
// fast-forward, whatever branch is checked out. A branch that already contains
// the target is left alone; a diverged branch keeps its own commits, except the
// channel branch, which mirrors upstream and is reset onto the same target.
// Local edits to tracked files are never touched. Every path that declines to
// advance logs why, so a skip is not mistaken for a successful update.
func syncChannel(repo, ch, target string) error {
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
	// HEAD already contains the selected target (current or ahead): nothing to pull.
	if isAncestor(repo, target, "HEAD") {
		return nil
	}
	// HEAD is strictly behind on a straight line: fast-forward onto the target.
	if isAncestor(repo, "HEAD", target) {
		if err := sys.Run("git", "-C", repo, "merge", "--ff-only", target); err != nil {
			// usually an untracked file the incoming commits also add; git names it above.
			return fmt.Errorf(i18n.T("fast-forward to origin/%s failed (see git's message above; "+
				"move the colliding file out of %s, then update again): %w"), ch, repo, err)
		}
		return nil
	}
	// Diverged: HEAD holds commits the selected target lacks. The channel branch
	// mirrors upstream, so reset it; any other branch keeps its work.
	head, _ := sys.RunOut("git", "-C", repo, "symbolic-ref", "--short", "--quiet", "HEAD")
	if strings.TrimSpace(head) == ch {
		progress.logf(i18n.T("Channel history diverged; reconciling %s onto its checked target"), ch)
		if err := sys.Run("git", "-C", repo, "reset", "--hard", target); err != nil {
			return fmt.Errorf(i18n.T("reconcile %s to its checked target failed: %w"), ch, err)
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
