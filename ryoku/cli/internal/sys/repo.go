package sys

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	wm "ryoku-wm"
)

// repoPathFile records where the live-mirror checkout sits. The deployed
// `ryoku` binary lives on PATH with no path back to the repo, so the dev
// deploy (ryoku/shell/deploy.sh) writes the checkout root here.
func repoPathFile() string { return filepath.Join(StateDir(), "repo") }

func recordedRepo() string {
	b, err := os.ReadFile(repoPathFile())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// ResolveRepo returns the Ryoku checkout root to track, or "" when there is
// none (a packaged install). RYOKU_REPO wins (so `ryoku deploy` and tests can
// point it explicitly); else the path the last deploy recorded. Anything that
// is not a git work tree is ignored.
func ResolveRepo() string {
	for _, p := range []string{strings.TrimSpace(os.Getenv("RYOKU_REPO")), recordedRepo()} {
		if p == "" {
			continue
		}
		if _, err := RunOut("git", "-C", p, "rev-parse", "--git-dir"); err == nil {
			return p
		}
	}
	// Fallback to the ~/ryoku-arch clone for a lost or broken recorded pointer,
	// but only when the box still opts into source tracking (a recorded
	// RYOKU_CHANNEL): a box migrated onto packages leaves the clone on disk
	// deliberately and must never have it resurrected as the update source.
	if TrackedChannel() == "" {
		return ""
	}
	track := filepath.Join(Home(), "ryoku-arch")
	if _, err := RunOut("git", "-C", track, "rev-parse", "--git-dir"); err == nil {
		if url, e := RunOut("git", "-C", track, "remote", "get-url", "origin"); e == nil && strings.Contains(url, "ryoku-arch") {
			return track
		}
	}
	return ""
}

// TrackedChannel is the update channel `ryoku track` recorded in
// ~/.config/environment.d/ryoku.conf (a RYOKU_CHANNEL=<branch> line), or "" when
// absent. environment.d is read into the session only at the next login, so the
// live RYOKU_CHANNEL is unset on a just-switched box; reading the file keeps the
// CLI on the tracked channel without waiting for a relogin.
func TrackedChannel() string {
	return envFileChannel(filepath.Join(ConfigHome(), "environment.d", "ryoku.conf"))
}

func envFileChannel(path string) string {
	for _, line := range strings.Split(readSmall(path), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "RYOKU_CHANNEL=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "RYOKU_CHANNEL="))
		}
	}
	return ""
}

// readSmall reads at most 4 KiB of a small state file. The release-line files
// live in a user's home and a root helper reads them too, so a file swapped for
// something endless costs nothing.
func readSmall(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	b, _ := io.ReadAll(io.LimitReader(f, 4<<10))
	return string(b)
}

// SourceLine is the branch this account's source checkout follows, "" on a box
// on packages. Independently released apps choose their release line from it.
func SourceLine() string {
	return sourceLineIn(ConfigHome(), StateDir())
}

// SourceLineAt is SourceLine for the account whose home is home. A root helper
// started through pkexec uses it for the calling account, whose environment
// pkexec does not pass on; it reads the same two files SourceLine does.
func SourceLineAt(home string) string {
	if home == "" {
		return ""
	}
	return sourceLineIn(filepath.Join(home, ".config"), filepath.Join(home, ".local", "state", "ryoku"))
}

// sourceLineIn decides the line from what the account keeps on disk, never
// from the session environment, so the user's report and the root install
// agree: the channel `ryoku track` recorded in environment.d, else the branch
// checked out in the checkout the last deploy recorded.
func sourceLineIn(configHome, stateDir string) string {
	if channel := envFileChannel(filepath.Join(configHome, "environment.d", "ryoku.conf")); channel != "" {
		return channel
	}
	repo := strings.TrimSpace(readSmall(filepath.Join(stateDir, "repo")))
	if repo == "" {
		return ""
	}
	return checkoutBranch(repo)
}

// checkoutBranch reads the branch HEAD names in a checkout, a linked worktree
// included, from git's files rather than by running git, which refuses a
// repository another account owns.
func checkoutBranch(repo string) string {
	gitDir := filepath.Join(repo, ".git")
	if info, err := os.Stat(gitDir); err == nil && !info.IsDir() {
		dir, ok := strings.CutPrefix(strings.TrimSpace(readSmall(gitDir)), "gitdir:")
		if !ok {
			return ""
		}
		gitDir = strings.TrimSpace(dir)
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Join(repo, gitDir)
		}
	}
	branch, ok := strings.CutPrefix(strings.TrimSpace(readSmall(filepath.Join(gitDir, "HEAD"))), "ref: refs/heads/")
	if !ok {
		return ""
	}
	return branch
}

// KeepLocalCommits saves the commits HEAD holds that upstream lacks under
// refs/ryoku-backup/<name>/<UTC time> before a caller resets HEAD onto upstream,
// so a reset never loses work nobody pushed. It reports the ref and how many
// commits it kept (none: no ref); an error means nothing was saved and the
// caller must not reset.
func KeepLocalCommits(repo, name, upstream string) (string, int, error) {
	out, err := RunOut("git", "-C", repo, "rev-list", "--count", upstream+"..HEAD")
	if err != nil {
		return "", 0, fmt.Errorf("count the commits %s lacks: %w", upstream, err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil || n == 0 {
		return "", 0, err
	}
	ref := "refs/ryoku-backup/" + name + "/" + time.Now().UTC().Format("20060102T150405Z")
	if _, err := RunOut("git", "-C", repo, "update-ref", ref, "HEAD"); err != nil {
		return "", 0, fmt.Errorf("save %d local commit(s) as %s: %w", n, ref, err)
	}
	return ref, n, nil
}

// SourceTracked reports whether this box takes its updates from a source
// checkout rather than packages: an explicit RYOKU_REPO, a recorded repo
// pointer (deploy.sh), or a tracked RYOKU_CHANNEL in environment.d
// (bin/ryoku-track). `ryoku track <channel>` migrates such a box onto packages.
func SourceTracked() bool {
	if strings.TrimSpace(os.Getenv("RYOKU_REPO")) != "" {
		return true
	}
	if recordedRepo() != "" {
		return true
	}
	return TrackedChannel() != ""
}

// RetireSourceTracking stops a box taking its updates from a source checkout:
// it drops the recorded repo pointer and the RYOKU_CHANNEL line from both
// environment.d and the compositor's user config. The ~/ryoku-arch clone is left on disk (the
// user's data); ResolveRepo's fallback is gated on the tracked channel this
// clears, so the clone is not re-adopted. Clearing the running session env is
// the caller's job: it needs a live user manager this package must not assume.
func RetireSourceTracking() error {
	if err := os.Remove(repoPathFile()); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := dropEnvChannel(filepath.Join(ConfigHome(), "environment.d", "ryoku.conf")); err != nil {
		return err
	}
	dir := wm.ConfigDir(wm.Detect().Name)
	if dir == "" {
		return nil
	}
	return dropLuaChannel(filepath.Join(ConfigHome(), dir, "user.lua"))
}

// dropEnvChannel removes the RYOKU_CHANNEL= line from an environment.d file,
// deleting the file when that leaves it empty (bin/ryoku-track writes nothing
// else into it). Absent file: nothing to do.
func dropEnvChannel(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var kept []string
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "RYOKU_CHANNEL=") {
			continue
		}
		kept = append(kept, line)
	}
	if strings.TrimSpace(strings.Join(kept, "\n")) == "" {
		return os.Remove(path)
	}
	return os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o644)
}

// dropLuaChannel removes the hl.env("RYOKU_CHANNEL", ...) line bin/ryoku-track
// appends to the compositor's user config, leaving the rest of the user's file intact.
func dropLuaChannel(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var kept []string
	for _, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, `hl.env("RYOKU_CHANNEL"`) {
			continue
		}
		kept = append(kept, line)
	}
	return os.WriteFile(path, []byte(strings.Join(kept, "\n")), 0o644)
}
