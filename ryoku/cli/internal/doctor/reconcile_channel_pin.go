package doctor

import (
	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"
	"ryoku-cli/internal/updater"

	i18n "ryoku-i18n"
)

// ---- reconciler: ryoku channel pin -------------------------------------------
//
// reconcileChannelPin heals a packaged box whose [ryoku] pin drifted onto a
// frozen release OLDER than the release actually installed -- the #291 wedge a
// failed boot-guard revert leaves behind: the pin was rewritten to the previous
// release but the downgrade could not complete, so the box runs newer packages
// than the pinned channel serves and can take no further updates.
//
// Deliberate vs accidental is decided by the channel-intent file `ryoku track`
// records (and nothing else writes): a pin that matches the recorded intent is
// the user's own choice and is left alone; a pin that does not is repaired by
// re-tracking the intended channel (stable when none was recorded), which moves
// the pin, the sync db, and the installed set back into agreement through the
// same transactional path `ryoku track` uses.
func reconcileChannelPin(checkOnly bool) recResult {
	manager, err := doctorPackageManager()
	if err != nil || manager != host.Pacman && manager != host.XBPS {
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
		return okRes(i18n.T("release %s is pinned on purpose (matches `ryoku track`)"), pin)
	case channelPinStale:
		if checkOnly {
			return wouldRes(i18n.T("the [ryoku] pin %s is older than the installed release %s, so this box can take no updates"), pin, installed).
				withFix(i18n.T("ryoku track %s"), sys.TrackName(want))
		}
		if err := updater.RetargetChannel(want); err != nil {
			return failRes(i18n.T("could not restore the %s channel from the stale pin %s: %v"), sys.DisplayChannel(want), pin, err).
				withFix(i18n.T("ryoku track %s"), sys.TrackName(want))
		}
		return fixedRes(i18n.T("restored the %s channel; the [ryoku] pin was stuck on %s while %s is installed"), sys.DisplayChannel(want), pin, installed)
	default:
		return okRes(i18n.T("channel pin matches the installed release"))
	}
}

// channelPinOutcome is what planChannelPin decides.
type channelPinOutcome int

const (
	channelPinFine       channelPinOutcome = iota // pin is a channel, or a tag no older than installed
	channelPinDeliberate                          // pin is an old tag the user chose (matches intent)
	channelPinStale                               // pin is an old tag no one chose: repair to want
)

// planChannelPin decides, purely, whether the [ryoku] pin is an accidental stale
// release lock. It fires only when the pin is a frozen release tag strictly
// older than the installed release (stable/testing/a mirror are never "stale"),
// and only calls it accidental when it does not match the recorded intent. The
// second return is the channel to restore (the intent, or stable by default).
func planChannelPin(pin, installed, intent string) (channelPinOutcome, string) {
	if !sys.IsReleaseTag(pin) || !sys.IsReleaseTag(installed) {
		return channelPinFine, "" // stable/testing/mirror, or nothing to compare
	}
	if sys.CompareReleaseTags(pin, installed) >= 0 {
		return channelPinFine, "" // pinned release is current or ahead: fine
	}
	want := intent
	if want == "" {
		want = sys.ChannelStable
	}
	if pin == want {
		return channelPinDeliberate, want // the user pinned this release on purpose
	}
	return channelPinStale, want
}
