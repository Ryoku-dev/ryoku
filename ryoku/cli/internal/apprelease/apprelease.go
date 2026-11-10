// Package apprelease consumes official Arch packages for independently released Ryoku apps.
package apprelease

import (
	"context"
	"fmt"
	"time"

	"ryoku-cli/internal/host"
)

// App describes one independently released app package.
type App struct {
	Name        string
	DisplayName string
	Repo        string
	Epoch       string
	Binary      string
	// LegacyOwner is the package that shipped the app's files before the app
	// had a package of its own.
	LegacyOwner string
}

var (
	Ryotunes = App{
		Name: "ryotunes", DisplayName: "Ryotunes", Repo: "ryoku-dev/ryotunes",
		Epoch: "1", Binary: "/usr/bin/ryotunes",
	}
	Ryoport = App{
		Name: "ryoport", DisplayName: "Ryoport", Repo: "ryoku-dev/ryoport",
		Binary: "/usr/bin/ryoport", LegacyOwner: "ryoku-desktop",
	}
)

// Status is the outcome of a Check, Upgrade or Ensure.
type Status struct {
	Installed string
	Latest    string
	Available bool
	Updated   bool
}

const (
	wantArch      = "x86_64"
	checkCacheTTL = time.Hour
)

func (a App) validate() error {
	switch {
	case a.Name == "ryotunes" && a.DisplayName == "Ryotunes" &&
		a.Repo == "ryoku-dev/ryotunes" && a.Epoch == "1" && a.Binary == "/usr/bin/ryotunes" &&
		a.LegacyOwner == "":
		return nil
	case a.Name == "ryoport" && a.DisplayName == "Ryoport" &&
		a.Repo == "ryoku-dev/ryoport" && a.Epoch == "" && a.Binary == "/usr/bin/ryoport" &&
		a.LegacyOwner == "ryoku-desktop":
		return nil
	default:
		return fmt.Errorf("unsupported app release configuration for %q", a.Name)
	}
}

// Check reports installed and latest versions without mutating packages.
func (a App) Check(ctx context.Context) (Status, error) {
	if err := a.validate(); err != nil {
		return Status{}, err
	}
	if manager, _ := host.Default().PackageManager(); manager != host.Pacman {
		return Status{}, nil
	}
	return defaultClient(a).Check(ctx)
}

// Upgrade installs a strictly newer release and respects an absent app.
func (a App) Upgrade(ctx context.Context) (Status, error) {
	if err := a.validate(); err != nil {
		return Status{}, err
	}
	if manager, _ := host.Default().PackageManager(); manager != host.Pacman {
		return Status{}, nil
	}
	return defaultClient(a).Upgrade(ctx)
}

// Ensure installs an absent app or moves an installed app strictly forward.
func (a App) Ensure(ctx context.Context) (Status, error) {
	if err := a.validate(); err != nil {
		return Status{}, err
	}
	if manager, _ := host.Default().PackageManager(); manager != host.Pacman {
		return Status{}, nil
	}
	return defaultClient(a).Ensure(ctx)
}

// Check is the Client-scoped implementation behind App.Check.
func (c *Client) Check(ctx context.Context) (Status, error) {
	installed := c.installedVersion(c.app.Name)
	if installed == "" {
		return Status{}, nil
	}
	rel, err := c.latestRelease(ctx, false)
	if err != nil {
		return Status{Installed: installed}, err
	}
	newer, err := c.isNewer(rel.Version, installed)
	if err != nil {
		return Status{Installed: installed, Latest: rel.Version}, err
	}
	return Status{Installed: installed, Latest: rel.Version, Available: newer}, nil
}

// Upgrade is the Client-scoped implementation behind App.Upgrade.
func (c *Client) Upgrade(ctx context.Context) (Status, error) {
	installed := c.installedVersion(c.app.Name)
	if installed == "" {
		return Status{}, nil
	}
	rel, err := c.latestRelease(ctx, true)
	if err != nil {
		return Status{Installed: installed}, err
	}
	newer, err := c.isNewer(rel.Version, installed)
	if err != nil {
		return Status{Installed: installed, Latest: rel.Version}, err
	}
	if !newer {
		return Status{Installed: installed, Latest: rel.Version}, nil
	}
	if err := c.installRelease(ctx, rel, installed, false); err != nil {
		return Status{Installed: installed, Latest: rel.Version, Available: true}, err
	}
	return Status{Installed: rel.Version, Latest: rel.Version, Updated: true}, nil
}

// Ensure is the Client-scoped implementation behind App.Ensure.
func (c *Client) Ensure(ctx context.Context) (Status, error) {
	installed := c.installedVersion(c.app.Name)
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
			return Status{Installed: installed, Latest: rel.Version}, nil
		}
	}
	if err := c.installRelease(ctx, rel, installed, true); err != nil {
		return Status{Installed: installed, Latest: rel.Version, Available: true}, err
	}
	return Status{Installed: rel.Version, Latest: rel.Version, Updated: true}, nil
}

func (c *Client) isNewer(latest, installed string) (bool, error) {
	cmp, err := c.vercmp(latest, installed)
	if err != nil {
		return false, err
	}
	return cmp > 0, nil
}
