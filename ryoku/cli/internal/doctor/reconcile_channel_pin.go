package doctor

import (
	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"
	"ryoku-cli/internal/updater"

	i18n "ryoku-i18n"
)

// ---- reconciler: ryoku channel pin -------------------------------------------
//
// reconcileChannelPin heals a packaged box whose repository pin drifted onto a
// frozen version older than the version actually installed. A failed boot-guard
// revert can rewrite the pin without completing the package move, leaving the
// box unable to take further updates.
//
// Deliberate vs accidental is decided by the channel-intent file `ryoku track`
// records (and nothing else writes): a pin that matches the recorded intent is
// the user's own choice and is left alone; a pin that does not is repaired by
// re-tracking the intended channel (stable when none was recorded), which moves
// the pin, the sync db, and the installed set back into agreement through the
// same transactional path `ryoku track` uses.
func reconcileChannelPin(checkOnly bool) recResult {
	manager, err := doctorPackageManager()
	if err != nil || manager != host.Pacman && manager != host.XBPS && manager != host.DNF {
		return okRes(i18n.T("release channel pin checks are not available on this host"))
	}
	if sys.ResolveRepo() != "" || !doctorPackageInstalled("ryoku-desktop") {
		if manager == host.Pacman {
			return okRes(i18n.T("not a packaged install; no [ryoku] channel pin to reconcile"))
		}
		return okRes(i18n.T("not a packaged install; no Ryoku channel pin to reconcile"))
	}
	pin, _ := host.Default().RepoChannel()
	installed := sys.ReadRelease().Release
	switch outcome, want := planChannelPin(pin, installed, sys.ReadChannelIntent()); outcome {
	case channelPinDeliberate:
		if sys.IsUnstableBuild(pin) {
			return okRes(i18n.T("unstable build %s is pinned on purpose (matches `ryoku track`)"), pin)
		}
		return okRes(i18n.T("release %s is pinned on purpose (matches `ryoku track`)"), pin)
	case channelPinStale:
		if checkOnly {
			return wouldRes(i18n.T("the Ryoku pin %s is older than the installed version %s, so this box can take no updates"), pin, installed).
				withFix(i18n.T("ryoku track %s"), sys.TrackName(want))
		}
		if err := updater.RetargetChannel(want); err != nil {
			return failRes(i18n.T("could not restore the %s channel from the stale pin %s: %v"), sys.DisplayChannel(want), pin, err).
				withFix(i18n.T("ryoku track %s"), sys.TrackName(want))
		}
		return fixedRes(i18n.T("restored the %s channel; the [ryoku] pin was stuck on %s while %s is installed"), sys.DisplayChannel(want), pin, installed)
	default:
		return okRes(i18n.T("channel pin matches the installed version"))
	}
}

// channelPinOutcome is what planChannelPin decides.
type channelPinOutcome int

const (
	channelPinFine       channelPinOutcome = iota // pin is a channel, current, ahead, or incomparable
	channelPinDeliberate                          // an older frozen pin matches the recorded intent
	channelPinStale                               // an older frozen pin was not chosen: repair to want
)

// planChannelPin decides, purely, whether a frozen package pin is accidental.
// Stable releases compare with stable releases and unstable builds with
// unstable builds; channels, mirrors, and mixed kinds are never called stale.
// The second return is the channel to restore (the intent, or stable by default).
func planChannelPin(pin, installed, intent string) (channelPinOutcome, string) {
	if !sys.IsFrozenVersion(pin) || !sys.IsFrozenVersion(installed) {
		return channelPinFine, ""
	}
	order, comparable := sys.CompareFrozenVersions(pin, installed)
	if !comparable || order >= 0 {
		return channelPinFine, ""
	}
	want := intent
	if want == "" {
		want = sys.ChannelStable
	}
	if pin == want {
		return channelPinDeliberate, want
	}
	return channelPinStale, want
}
