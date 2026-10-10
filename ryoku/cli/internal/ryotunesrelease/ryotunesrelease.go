// Package ryotunesrelease consumes official Ryotunes GitHub release packages.
// Ryoku's pacman repository mirrors and signs the same packages independently.
//
// Check reports availability without installing. Upgrade updates an existing
// installation but respects absence. Ensure also installs an absent package
// when Doctor is reconciling a managed Ryoku desktop.
//
// All operations are context-bounded. Installation resolves the release fresh,
// verifies its checksum and package metadata, and never downgrades.
package ryotunesrelease

import (
	"context"
	"time"

	"ryoku-cli/internal/host"
)

// Status is the outcome of a Check, Upgrade or Ensure.
//
//	Installed is the pacman version (pkgver-pkgrel, epoch included) currently
//	          installed, or "" when Ryotunes is not installed. After a successful
//	          Upgrade or Ensure it is the version just installed.
//	Latest    is the resolved published version, or "" when discovery was skipped
//	          or failed.
//	Available reports that a strictly newer build exists than what is installed.
//	          It may remain true on installation failure; success clears it.
//	Updated   reports that Upgrade or Ensure installed a build during this call.
type Status struct {
	Installed string
	Latest    string
	Available bool
	Updated   bool
}

// pkgName is the pacman package (and the GitHub asset prefix) this client tracks.
const pkgName = "ryotunes"

// wantArch is the only architecture Ryoku publishes and installs Ryotunes for.
// A package file that reports any other architecture is refused before pacman
// ever sees it.
const wantArch = "x86_64"

// wantEpoch is the pacman epoch every official Ryotunes release carries in its
// .PKGINFO. The release asset FILENAME is deliberately epochless
// (ryotunes-<pkgver>-<pkgrel>-x86_64.pkg.tar.zst, the published contract), but
// the package's real version is 1:<pkgver>-<pkgrel>. The epoch is what lets a
// current build outrank the retired, divergently high-versioned build the
// [ryoku] repo used to ship (2.5.1-1): under pacman ordering 1:x-y beats any
// epoch-0 version, so an old box moves forward instead of seeing a "downgrade".
// It is a fixed part of the publishing contract, not read from the (untrusted)
// asset name; the downloaded package's own .PKGINFO is verified to carry exactly
// this epoch before pacman installs it.
const wantEpoch = "1"

// checkCacheTTL bounds how often Check re-reads the GitHub release. `ryoku
// doctor` (and anything else polling availability) can run repeatedly; a cache
// this side of an hour keeps that off the network and well under GitHub's
// unauthenticated rate limit while staying fresh enough to notice a new release
// the same day. Upgrade ignores it and always fetches fresh.
const checkCacheTTL = time.Hour

// Check reports the installed and latest Ryotunes versions without mutating any
// package. The published asset is an Arch package, so this stays inactive on
// other package managers until a native release lane exists. It performs a
// bounded, cached release lookup; when Ryotunes is not installed it returns a
// zero Status and does no network at all. A lookup that fails with nothing
// cached returns an error rather than a false "up to date".
func Check(ctx context.Context) (Status, error) {
	if manager, _ := host.Default().PackageManager(); manager != host.Pacman {
		return Status{}, nil
	}
	return defaultClient().Check(ctx)
}

// Upgrade installs the latest published Ryotunes when it is strictly newer than
// the installed build, and does nothing otherwise. It is a no-op (Updated false,
// no network, no install) when Ryotunes is not installed, so it never resurrects
// a removed app and never downgrades. The candidate package is verified by
// sha256 and by its own pacman metadata before it is installed through pacman.
func Upgrade(ctx context.Context) (Status, error) {
	if manager, _ := host.Default().PackageManager(); manager != host.Pacman {
		return Status{}, nil
	}
	return defaultClient().Upgrade(ctx)
}

// Ensure installs the latest published Ryotunes on a box that is meant to have
// it (a fresh install, or a reconcile after the user removed it) and moves an
// older installed build forward. Unlike Upgrade it installs when the package is
// absent -- it is the install path, not just the update path -- while still
// verifying the candidate by sha256 and its own pacman metadata, refusing a
// downgrade, and pulling only from the official GitHub release.
func Ensure(ctx context.Context) (Status, error) {
	if manager, _ := host.Default().PackageManager(); manager != host.Pacman {
		return Status{}, nil
	}
	return defaultClient().Ensure(ctx)
}

// Check is the Client-scoped implementation behind the package-level Check.
func (c *Client) Check(ctx context.Context) (Status, error) {
	installed := c.installedVersion(pkgName)
	if installed == "" {
		// Not installed: Ryoku only tracks an app the box already has, so there
		// is nothing to compare and no reason to touch the network.
		return Status{}, nil
	}

	rel, err := c.latestRelease(ctx, false)
	if err != nil {
		// Offline or a failed lookup: report what we know (the installed
		// version) and surface the error. Never claim "up to date".
		return Status{Installed: installed}, err
	}

	newer, err := c.isNewer(rel.Version, installed)
	if err != nil {
		return Status{Installed: installed, Latest: rel.Version}, err
	}
	return Status{Installed: installed, Latest: rel.Version, Available: newer}, nil
}

// Upgrade is the Client-scoped implementation behind the package-level Upgrade.
func (c *Client) Upgrade(ctx context.Context) (Status, error) {
	installed := c.installedVersion(pkgName)
	if installed == "" {
		// Not installed: do not install, do not fetch. Upgrading an app the box
		// does not have would be installing it, which is not this path's job.
		return Status{}, nil
	}

	// Fresh, never a stale cache: an install decision must be made against what
	// GitHub serves right now, not a lookup Check left behind minutes ago.
	rel, err := c.latestRelease(ctx, true)
	if err != nil {
		return Status{Installed: installed}, err
	}

	newer, err := c.isNewer(rel.Version, installed)
	if err != nil {
		return Status{Installed: installed, Latest: rel.Version}, err
	}
	if !newer {
		// Already current or ahead: nothing to install. Never downgrade.
		return Status{Installed: installed, Latest: rel.Version}, nil
	}

	if err := c.installRelease(ctx, rel, false); err != nil {
		return Status{Installed: installed, Latest: rel.Version, Available: true}, err
	}
	// Installed the new build: it is now what is installed, and nothing newer
	// remains to offer.
	return Status{Installed: rel.Version, Latest: rel.Version, Updated: true}, nil
}

// Ensure brings Ryotunes to the latest published build for a box that is meant
// to have it but does not (a fresh install, or a reconcile after the user
// removed it), and moves an older installed build forward. Unlike Upgrade it
// installs when the package is absent -- that is the whole point: it is the
// install path, called by `ryoku doctor` when a desktop box is missing the app.
// It is still safe: it fetches the release fresh, verifies the candidate by
// sha256 and by its own pacman metadata, refuses a downgrade when a newer build
// is already installed, and pulls only from the official GitHub release (never
// the stale [ryoku] repo copy), so it delivers the current native build even
// before a repo re-import has propagated.
func (c *Client) Ensure(ctx context.Context) (Status, error) {
	installed := c.installedVersion(pkgName)

	// Fresh, never a stale cache: an install decision must be made against what
	// GitHub serves right now.
	rel, err := c.latestRelease(ctx, true)
	if err != nil {
		return Status{Installed: installed}, err
	}

	if installed != "" {
		newer, err := c.isNewer(rel.Version, installed)
		if err != nil {
			return Status{Installed: installed, Latest: rel.Version}, err
		}
		if !newer {
			// Already current or ahead: nothing to install. Never downgrade.
			return Status{Installed: installed, Latest: rel.Version}, nil
		}
	}

	if err := c.installRelease(ctx, rel, true); err != nil {
		return Status{Installed: installed, Latest: rel.Version, Available: true}, err
	}
	return Status{Installed: rel.Version, Latest: rel.Version, Updated: true}, nil
}

// isNewer reports whether latest is strictly greater than installed under
// pacman's own version ordering (epoch, pkgver, pkgrel), so an epoch bump or a
// pkgrel-only rebuild is compared the same way pacman would.
func (c *Client) isNewer(latest, installed string) (bool, error) {
	cmp, err := c.vercmp(latest, installed)
	if err != nil {
		return false, err
	}
	return cmp > 0, nil
}
