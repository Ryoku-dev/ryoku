package ryokuapps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"ryoku-cli/internal/apprelease"
	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// Row is one app in `ryoku apps list --json`.
type Row struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Summary   string `json:"summary"`
	Package   string `json:"package"`
	Homepage  string `json:"homepage"`
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
	Latest    string `json:"latest,omitempty"`
	Update    bool   `json:"update"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	Removable bool   `json:"removable"`
	Builtin   bool   `json:"builtin"`
	DesktopID string `json:"desktop_id,omitempty"`
	Lane      Lane   `json:"lane"`
	Channel   string `json:"channel"`
}

// Report is the whole `ryoku apps list --json` document.
type Report struct {
	Schema int   `json:"schema"`
	Apps   []Row `json:"apps"`
}

// packages is the slice of the host seam the report reads.
type packages interface {
	PackageManager() (host.PackageManager, error)
	PackageVersion(name string) (string, error)
	InstalledSize(name string) (int64, bool)
	RepoPackages() ([]host.RepoPackage, error)
	CompareVersions(manager host.PackageManager, installed, available string) int
}

// Lister builds the report. Latest answers a GitHub-released app's newest
// release on its line.
type Lister struct {
	Packages packages
	Line     string
	Latest   func(context.Context, apprelease.App) (string, error)
}

// releaseLookupTimeout bounds one release lookup, so an offline box still
// answers the list promptly from what it knows.
const releaseLookupTimeout = 8 * time.Second

// List reports every catalogue app on this box. It never changes the system.
func (l Lister) List(ctx context.Context) (Report, error) {
	manager, err := l.Packages.PackageManager()
	if err != nil {
		return Report{}, err
	}
	// The repository list is optional: offline, the report still names what is
	// installed and leaves latest empty.
	repo := map[string]string{}
	if packages, err := l.Packages.RepoPackages(); err == nil {
		for _, pkg := range packages {
			repo[pkg.Name] = pkg.Version
		}
	}
	report := Report{Schema: 1, Apps: make([]Row, 0, len(catalogue))}
	for _, app := range All() {
		row := Row{
			ID: app.ID, Name: app.Name, Summary: i18n.T(app.Summary), Package: app.Package,
			Homepage: app.Homepage, Removable: app.Removable(), Builtin: app.Builtin,
			DesktopID: app.DesktopID, Lane: app.LaneOn(manager), Channel: app.Channel(manager, l.Line),
		}
		version, err := l.Packages.PackageVersion(app.Package)
		row.Installed = err == nil && version != ""
		if row.Installed {
			row.Version = version
		}
		if app.Builtin {
			// Part of the desktop: there whenever Ryoku is, updated with it.
			row.Installed = true
			report.Apps = append(report.Apps, row)
			continue
		}
		if row.Installed {
			if size, ok := l.Packages.InstalledSize(app.Package); ok {
				row.SizeBytes = size
			}
		}
		if row.Lane == LaneAppRelease {
			lookup, cancel := context.WithTimeout(ctx, releaseLookupTimeout)
			row.Latest, _ = l.Latest(lookup, app.ReleaseOn(l.Line))
			cancel()
		} else {
			row.Latest = repo[app.Package]
		}
		row.Update = row.Installed && row.Latest != "" &&
			l.Packages.CompareVersions(manager, row.Version, row.Latest) < 0
		report.Apps = append(report.Apps, row)
	}
	return report, nil
}

// Main is `ryoku apps ...` for the signed-in user.
func Main(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "list" || len(args) > 2 || (len(args) == 2 && args[1] != "--json") {
		fmt.Fprint(stderr, i18n.T("Usage: ryoku apps list [--json]\n"))
		return 2
	}
	lister := Lister{
		Packages: host.Default(),
		Line:     sys.SourceLine(),
		Latest: func(ctx context.Context, app apprelease.App) (string, error) {
			return app.Latest(ctx)
		},
	}
	report, err := lister.List(context.Background())
	if err != nil {
		fmt.Fprintf(stderr, "ryoku apps: %v\n", err)
		return 1
	}
	if len(args) == 2 {
		encoder := json.NewEncoder(stdout)
		if err := encoder.Encode(report); err != nil {
			return 1
		}
		return 0
	}
	for _, row := range report.Apps {
		state := i18n.T("not installed")
		switch {
		case row.Builtin:
			state = i18n.T("built in")
		case row.Update:
			state = i18n.Tf("%s, update to %s", row.Version, row.Latest)
		case row.Installed:
			state = row.Version
		}
		fmt.Fprintf(stdout, "%-11s %s\n", row.Name, state)
	}
	return 0
}
