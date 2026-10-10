package main

import (
	"fmt"
	"os"
	"path/filepath"
	"ryoku-cli/internal/sys"
	"ryoku-cli/internal/updater"
	i18n "ryoku-i18n"
	"strings"
)

// where the track script lives, for boxes with no local checkout that ask to
// build from source (`--source`). Always the main copy: the stable script moves
// a box in either direction, so a packaged box can still reach it.
const trackURL = "https://raw.githubusercontent.com/ryoku-dev/ryoku-arch/main/bin/ryoku-track"

// sourceBranches maps a `ryoku track ... --source` channel to the git branch
// bin/ryoku-track builds from: stable is the main branch, unstable the
// unstable-dev branch (testing is the internal synonym for unstable).
var sourceBranches = map[string]string{
	sys.ChannelStable:   "main",
	sys.ChannelUnstable: "unstable-dev",
	sys.ChannelTesting:  "unstable-dev",
}

// cmdTrack points the box at an update channel. By default a track selects a
// package channel and hands off to updater.Track (which rewrites the package
// repository and runs an update): `stable`, `unstable` (the testing channel),
// stable release tags, and unstable build names pass through; `testing` is a
// quiet synonym for `unstable`. The old `main`/`unstable-dev` arguments are
// retired and fail with the replacement. `--source` (either order) instead
// builds the box from a git checkout via bin/ryoku-track, the developer path.
func cmdTrack(args []string) error {
	channel, source, err := parseTrackArgs(args)
	if err != nil {
		return err
	}
	if repl := retiredTrackName(channel); repl != "" {
		return fmt.Errorf(i18n.T("`ryoku track %s` was retired; use `ryoku track %s`"), channel, repl)
	}
	if source {
		branch := sourceBranches[channel]
		if branch == "" {
			return fmt.Errorf(i18n.T("`--source` builds from a checkout and takes stable or unstable, not %q"), channel)
		}
		return trackFromSource(branch)
	}
	pkg := packageChannelFor(channel)
	if pkg == "" {
		return fmt.Errorf(i18n.T("unknown channel %q\n"+
			"  packaged: stable, unstable, a release tag, or an unstable build\n"+
			"  source:   stable or unstable, with --source"), channel)
	}
	return updater.Track(pkg)
}

// retiredTrackName maps a retired track argument to its replacement, or "" when
// the name is still accepted. `main` and `unstable-dev` named the git branches a
// tester once tracked; the channels they selected are now `stable`/`unstable`.
func retiredTrackName(name string) string {
	switch name {
	case "main":
		return sys.ChannelStable
	case "unstable-dev":
		return sys.ChannelUnstable
	}
	return ""
}

// parseTrackArgs pulls the one channel and the optional --source flag (either
// order) out of the track arguments.
func parseTrackArgs(args []string) (channel string, source bool, err error) {
	for _, a := range args {
		switch {
		case a == "--source":
			source = true
		case strings.HasPrefix(a, "-"):
			return "", false, fmt.Errorf(i18n.T("unknown flag %q (only --source is accepted)"), a)
		case channel != "":
			return "", false, fmt.Errorf(i18n.T("track takes one channel, got %q and %q"), channel, a)
		default:
			channel = a
		}
	}
	if channel == "" {
		return "", false, fmt.Errorf(i18n.T("usage: ryoku track <stable|unstable|v<release-or-build>>\n" +
			"       ryoku track <stable|unstable> --source   (build from a git checkout)"))
	}
	return channel, source, nil
}

// packageChannelFor maps a track argument to the package channel it selects:
// unstable is the moving testing channel; stable, testing (the quiet synonym),
// stable release tags, and unstable builds pass through. It returns "" for
// anything that names no package channel.
func packageChannelFor(name string) string {
	if name == sys.ChannelUnstable {
		return sys.ChannelTesting
	}
	if sys.ChannelServer(name) != "" {
		return name
	}
	return ""
}

// trackFromSource hands off to bin/ryoku-track, which builds and deploys the box
// from a git checkout of the branch (main or unstable-dev, already translated
// from the stable/unstable track name). It prefers a local checkout's copy and
// otherwise fetches the canonical one, so `--source` works from a packaged
// install with no checkout. The script does the real work and does not lean on
// this binary.
func trackFromSource(branch string) error {
	if repo := sys.ResolveRepo(); repo != "" {
		if script := filepath.Join(repo, "bin", "ryoku-track"); sys.Exists(script) {
			return sys.Run("bash", script, branch)
		}
	}
	if !sys.Has("curl") {
		return fmt.Errorf(i18n.T("no local track script and curl is missing; run it by hand:\n  curl -fsSL %s | bash -s -- %s"), trackURL, branch)
	}
	tmp, err := os.CreateTemp("", "ryoku-track-*.sh")
	if err != nil {
		return err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	if err := sys.Run("curl", "-fsSL", trackURL, "-o", tmp.Name()); err != nil {
		return fmt.Errorf(i18n.T("fetch track script from %s: %w"), trackURL, err)
	}
	return sys.Run("bash", tmp.Name(), branch)
}
