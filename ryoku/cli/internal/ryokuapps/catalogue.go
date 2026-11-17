// Package ryokuapps is the table of Ryoku's own apps and the two seams that
// read it: the user's read-only report (`ryoku apps list --json`) and the
// root helper's package verbs (`ryoku-host apps ...`) that Settings and
// Ryostore call through pkexec.
package ryokuapps

import (
	"ryoku-cli/internal/apprelease"
	"ryoku-cli/internal/host"
)

// Lane is how an app reaches a box.
type Lane string

const (
	// LaneAppRelease is a GitHub-released Arch package Ryoku downloads and
	// verifies itself (Arch and CachyOS).
	LaneAppRelease Lane = "apprelease"
	// LaneRepo is a package from Ryoku's repository: [ryoku] on Arch and
	// CachyOS, the native RPM or XBPS package on Fedora and Void.
	LaneRepo Lane = "repo"
)

// App is one Ryoku-owned app.
type App struct {
	ID        string
	Name      string
	Summary   string
	Package   string
	DesktopID string
	Homepage  string
	// Builtin apps ship inside ryoku-desktop and are never removed on their own.
	Builtin bool
	// Release is the GitHub release lane the app takes on pacman boxes, nil
	// when it only comes from Ryoku's repository.
	Release *apprelease.App
}

var catalogue = [...]App{
	{
		ID: "ryotunes", Name: "Ryotunes", Package: "ryotunes",
		Summary:   "The music player, in your Ryoku style and colours.",
		DesktopID: "ryotunes.desktop", Homepage: "https://github.com/ryoku-dev/ryotunes",
		Release: &apprelease.Ryotunes,
	},
	{
		ID: "ryoport", Name: "Ryoport", Package: "ryoport",
		Summary:   "The fleet dashboard for virtual machines, SSH servers and remote consoles.",
		DesktopID: "dev.ryoku.ryoport.desktop", Homepage: "https://github.com/ryoku-dev/ryoport",
		Release: &apprelease.Ryoport,
	},
	{
		ID: "ryomanager", Name: "Ryomanager", Package: "ryomanager",
		Summary:   "The system monitor and process manager.",
		DesktopID: "ryomanager.desktop", Homepage: "https://github.com/Ryoku-dev/ryomanager",
	},
	{
		ID: "ryostore", Name: "Ryostore", Package: "ryoku-desktop",
		Summary:   "The store for bar styles, plugins, widgets and more. Part of the Ryoku desktop.",
		DesktopID: "ryostore.desktop", Homepage: "https://github.com/ryoku-dev/ryostore-revamped",
		Builtin: true,
	},
}

// All lists the catalogue in display order.
func All() []App { return catalogue[:] }

// Lookup finds an app by id.
func Lookup(id string) (App, bool) {
	for _, app := range catalogue {
		if app.ID == id {
			return app, true
		}
	}
	return App{}, false
}

// Removable reports whether the app may be removed on its own.
func (a App) Removable() bool { return !a.Builtin }

// LaneOn is the lane the app takes on a box with this package manager.
func (a App) LaneOn(manager host.PackageManager) Lane {
	if a.Release != nil && manager == host.Pacman {
		return LaneAppRelease
	}
	return LaneRepo
}

// ReleaseOn is the app's GitHub release as the given Ryoku line follows it.
func (a App) ReleaseOn(line string) apprelease.App {
	return a.Release.OnLine(line)
}

// Channel names the releases the app follows on this box.
func (a App) Channel(manager host.PackageManager, line string) string {
	if a.LaneOn(manager) == LaneAppRelease && a.ReleaseOn(line).Prerelease() {
		return "prerelease"
	}
	return "stable"
}
