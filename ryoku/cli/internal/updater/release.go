package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"
	i18n "ryoku-i18n"
)

// channelRelease is release.json as build-repo.sh writes it beside a channel's
// db: which release that channel serves right now.
type channelRelease struct {
	Release string `json:"release"`
	Name    string `json:"name"`
	Channel string `json:"channel"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// releaseLedger is releases/index.json, the list publish-repo.yml appends a
// stable release to; `ryoku rollback` offers its entries.
type releaseLedger struct {
	Latest   string          `json:"latest"`
	Releases []ledgerRelease `json:"releases"`
}

type ledgerRelease struct {
	Tag     string `json:"tag"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	Repo    string `json:"repo"`
}

// releaseFetchTTL bounds how often `ryoku status` (polled by the Hub and the
// update island) re-reads a channel's release.json.
const releaseFetchTTL = 10 * time.Minute

// RYOKU_RELEASE_BASE overrides RepoBase for tests and a local mirror.
func repoBase() string {
	if base := strings.TrimSpace(os.Getenv("RYOKU_RELEASE_BASE")); base != "" {
		return strings.TrimSuffix(base, "/")
	}
	if updatePackageManager() == host.XBPS {
		return host.XBPSRepoBase
	}
	return sys.RepoBase
}

// fetchCached GETs url into a state-dir cache keyed by name, re-fetching
// after ttl; offline or on error it serves the last cached body, or nil.
func fetchCached(name, url string, ttl time.Duration) []byte {
	dir := filepath.Join(sys.StateDir(), "release-cache")
	path := filepath.Join(dir, name)
	if st, err := os.Stat(path); err == nil && time.Since(st.ModTime()) < ttl {
		if b, err := os.ReadFile(path); err == nil {
			return b
		}
	}
	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Get(url)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			if b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)); err == nil {
				_ = os.MkdirAll(dir, 0o755)
				_ = os.WriteFile(path, b, 0o644)
				return b
			}
		}
	}
	if b, err := os.ReadFile(path); err == nil {
		return b
	}
	return nil
}

// channelServes reads what a channel currently serves, or a zero value when
// the channel is unreachable and nothing is cached.
func channelServes(channel string) channelRelease {
	var release channelRelease
	url := repoReleaseURL(channel)
	managerBase := host.PacmanRepoBase
	if updatePackageManager() == host.XBPS {
		managerBase = host.XBPSRepoBase
	}
	url = strings.Replace(url, managerBase, repoBase(), 1)
	if url == "" {
		return release
	}
	if body := fetchCached("channel-"+sanitize(channel)+".json", url+"/release.json", releaseFetchTTL); body != nil {
		_ = json.Unmarshal(body, &release)
	}
	return release
}

// ledger reads the release ledger, newest first.
func ledger() releaseLedger {
	var l releaseLedger
	if b := fetchCached("releases-index.json", repoBase()+"/releases/index.json", releaseFetchTTL); b != nil {
		_ = json.Unmarshal(b, &l)
	}
	return l
}

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' {
			return r
		}
		return '_'
	}, s)
}

// Track moves a box onto a package channel: stable (the pointer every install
// starts on), testing (rebuilt on every push to unstable-dev), or a release
// tag (pinned to that frozen release until tracked away). It rewrites the
// [ryoku] Server line and runs an update, which moves the Ryoku set to whatever
// the channel serves, down as well as up. A box whose updates currently come
// from a source checkout is migrated onto packages first: the checkout is
// retired as the update source (the ~/ryoku-arch clone stays on disk but no
// longer drives updates). Building from a checkout is `ryoku track ... --source`.
func Track(channel string) error {
	if channelRepoURL(channel) == "" {
		return fmt.Errorf(i18n.T("unknown channel %q: stable, unstable, or a release tag (see `ryoku rollback` for the list)"), channel)
	}
	source := sys.SourceTracked()
	install := host.Default().Package([]string{"installed", "ryoku-desktop"}) != host.ExitOK
	if install && packagedRepoURL() == "" {
		return fmt.Errorf(i18n.T("no ryoku-desktop package and no Ryoku package repository to install it from; run `ryoku doctor` to add the repo, then `ryoku track %s`"), sys.TrackName(channel))
	}
	if !source && packagedChannel() == "" && packagedRepoURL() != "" {
		return fmt.Errorf(i18n.T("the Ryoku package repository points at %s, a mirror Ryoku does not publish; edit its package-manager configuration by hand"), packagedRepoURL())
	}
	// Already on the channel, package box, nothing to migrate: only move the set
	// if the channel now serves something newer than what is installed.
	if !source && !install && packagedChannel() == channel {
		if serves := channelServes(channel).Release; serves == "" || serves == sys.ReadRelease().Release {
			// A box moved here by a retired name (unstable-dev, main) is already
			// on the right channel; record the choice so the doctor treats it as
			// deliberate, and say so plainly instead of a bare "already on".
			recordChannelIntent(channel)
			if channel == sys.ChannelTesting {
				fmt.Println(i18n.T("already on unstable (the channel `ryoku track unstable-dev` used to select); nothing to move, `ryoku update` keeps it current"))
			} else {
				fmt.Printf(i18n.T("already on %s; nothing to move, `ryoku update` keeps it current\n"), sys.DisplayChannel(channel))
			}
			return nil
		}
		fmt.Printf(i18n.T("==> Already tracking %s; moving the Ryoku set to what it serves\n"), sys.DisplayChannel(channel))
		recordChannelIntent(channel)
		return retargetChannel(channel, runChannelUpdate)
	}

	migrated, err := migrateOffCheckout()
	if err != nil {
		return err
	}
	if migrated {
		clearSessionChannelEnv()
		fmt.Println(i18n.T("==> Retired the source checkout as the update source; the ~/ryoku-arch clone stays on disk but no longer drives updates."))
	}
	switch {
	case channel == sys.ChannelTesting:
		fmt.Println(i18n.T("==> Now tracking unstable packages: rebuilt on every push. `ryoku track stable` returns to stable releases."))
	case sys.IsReleaseTag(channel):
		fmt.Printf(i18n.T("==> Pinned to release %s. `ryoku update` keeps this release; `ryoku track stable` follows releases again.\n"), channel)
	default:
		fmt.Println(i18n.T("==> Now tracking stable packages: named releases as they are published."))
	}
	if install {
		fmt.Println(i18n.T("==> ryoku-desktop is not installed here; the channel switch installs it from the selected channel."))
	}
	manager := "pacman"
	if updatePackageManager() == host.XBPS {
		manager = "xbps-install"
	}
	fmt.Printf(i18n.T("==> Updates now come from packages: `ryoku update` runs %s.\n"), manager)
	// Record the deliberate choice before the move: it is the signal the doctor
	// uses to tell a pin the user asked for from one a failed boot-guard revert
	// left behind (#291). retargetChannel makes the pin transactional -- a failed
	// move restores the previous [ryoku] Server exactly and re-syncs.
	recordChannelIntent(channel)
	return retargetChannel(channel, runChannelUpdate)
}

// recordChannelIntent writes the channel Track is moving to as the deliberate
// choice. Best effort: a box that could not record it just defaults to stable
// when the doctor later reconciles the pin.
func recordChannelIntent(channel string) {
	// Skip the root write (and its sudo prompt) when nothing changes.
	if sys.ReadChannelIntent() == channel {
		return
	}
	if err := sys.WriteChannelIntent(channel); err != nil {
		fmt.Fprintf(os.Stderr, i18n.T("note: could not record the tracked channel: %v\n"), err)
	}
}

// migrateOffCheckout retires a source-tracked box's checkout as the update
// source (the ~/ryoku-arch clone stays on disk) so `ryoku track` can move it
// onto packages. Returns whether it migrated. It touches no pacman and no
// network, so it is unit-testable; the pin and the move are the caller's
// transactional job (retargetChannel).
func migrateOffCheckout() (migrated bool, err error) {
	if !sys.SourceTracked() {
		return false, nil
	}
	if err := sys.RetireSourceTracking(); err != nil {
		return false, fmt.Errorf(i18n.T("could not retire the source checkout: %w"), err)
	}
	return true, nil
}

// runChannelUpdate is the pacman side of a track (the forced -Syyu plus an
// explicit -S ryoku-desktop, which installs it when absent and moves the set in
// either direction). A var so a test exercises the switch without running pacman.
var runChannelUpdate = func() error { return Update([]string{"--channel-switch"}) }

// clearSessionChannelEnv drops RYOKU_CHANNEL from the running user manager and
// the D-Bus activation env (the reverse of bin/ryoku-track), so units and apps
// launched after a migration no longer carry the stale channel. Best-effort and
// a var so a test never touches the real session.
var clearSessionChannelEnv = func() {
	_ = host.Default().Service([]string{"env", "RYOKU_CHANNEL="})
	os.Unsetenv("RYOKU_CHANNEL")
}
