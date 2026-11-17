package ryokuapps

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"ryoku-cli/internal/apprelease"
	"ryoku-cli/internal/host"
)

type fakePackages struct {
	manager   host.PackageManager
	installed map[string]string
	sizes     map[string]int64
	repo      []host.RepoPackage
	repoErr   error
}

func (f fakePackages) PackageManager() (host.PackageManager, error) { return f.manager, nil }
func (f fakePackages) PackageVersion(name string) (string, error) {
	if v, ok := f.installed[name]; ok {
		return v, nil
	}
	return "", host.ErrRepoAbsent
}
func (f fakePackages) InstalledSize(name string) (int64, bool) {
	size, ok := f.sizes[name]
	return size, ok
}
func (f fakePackages) RepoPackages() ([]host.RepoPackage, error) { return f.repo, f.repoErr }
func (f fakePackages) CompareVersions(_ host.PackageManager, a, b string) int {
	return strings.Compare(a, b)
}

func rowByID(t *testing.T, report Report, id string) Row {
	t.Helper()
	for _, row := range report.Apps {
		if row.ID == id {
			return row
		}
	}
	t.Fatalf("no %s row in %+v", id, report)
	return Row{}
}

// On pacman the GitHub-released apps take the apprelease lane and the hyperion
// line marks Ryotunes as following prereleases; the other apps come from the
// repository; Ryostore is built in.
func TestListOnPacmanHyperion(t *testing.T) {
	var asked []string
	lister := Lister{
		Packages: fakePackages{
			manager:   host.Pacman,
			installed: map[string]string{"ryotunes": "1:1.2.6-1", "ryomanager": "0.1.0-1", "ryoku-desktop": "1.1.0-1"},
			sizes:     map[string]int64{"ryotunes": 40 << 20, "ryomanager": 9 << 20},
			repo:      []host.RepoPackage{{Name: "ryomanager", Version: "0.2.0-1"}, {Name: "ryotunes", Version: "1:1.2.6-1"}},
		},
		Line: apprelease.HyperionLine,
		Latest: func(_ context.Context, app apprelease.App) (string, error) {
			asked = append(asked, app.Name+"@"+app.Line)
			if app.Name == "ryotunes" {
				return "1:2.0.0-1", nil
			}
			return "", errors.New("offline")
		},
	}
	report, err := lister.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Schema != 1 || len(report.Apps) != 4 {
		t.Fatalf("report = %+v", report)
	}
	tunes := rowByID(t, report, "ryotunes")
	want := Row{ID: "ryotunes", Name: "Ryotunes", Summary: tunes.Summary, Package: "ryotunes", Homepage: tunes.Homepage,
		Installed: true, Version: "1:1.2.6-1", Latest: "1:2.0.0-1", Update: true, SizeBytes: 40 << 20,
		Removable: true, DesktopID: "ryotunes.desktop", Lane: LaneAppRelease, Channel: "prerelease"}
	if tunes != want {
		t.Fatalf("ryotunes = %+v\nwant %+v", tunes, want)
	}
	port := rowByID(t, report, "ryoport")
	if port.Installed || port.Latest != "" || port.Lane != LaneAppRelease || port.Channel != "stable" || port.Update {
		t.Fatalf("ryoport = %+v", port)
	}
	manager := rowByID(t, report, "ryomanager")
	if manager.Lane != LaneRepo || manager.Latest != "0.2.0-1" || !manager.Update || manager.SizeBytes != 9<<20 {
		t.Fatalf("ryomanager = %+v", manager)
	}
	store := rowByID(t, report, "ryostore")
	if !store.Builtin || store.Removable || !store.Installed || store.Update || store.SizeBytes != 0 || store.Version != "1.1.0-1" {
		t.Fatalf("ryostore = %+v", store)
	}
	if strings.Join(asked, ",") != "ryotunes@hyperion,ryoport@" {
		t.Fatalf("release lookups = %v", asked)
	}
}

// Fedora and Void take every app from Ryoku's repository and never ask GitHub;
// an unreachable repository leaves latest empty instead of failing the list.
func TestListOnNativeEditions(t *testing.T) {
	for _, manager := range []host.PackageManager{host.DNF, host.XBPS} {
		lister := Lister{
			Packages: fakePackages{manager: manager, installed: map[string]string{"ryotunes": "1.2.6-1.fc44"},
				repoErr: errors.New("offline")},
			Line: apprelease.HyperionLine,
			Latest: func(context.Context, apprelease.App) (string, error) {
				t.Fatalf("%s asked GitHub for a release", manager)
				return "", nil
			},
		}
		report, err := lister.List(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range report.Apps {
			if row.Lane != LaneRepo || row.Channel != "stable" || row.Latest != "" || row.Update {
				t.Fatalf("%s: %+v", manager, row)
			}
		}
		if !rowByID(t, report, "ryotunes").Installed {
			t.Fatalf("%s: ryotunes not installed", manager)
		}
	}
}

// The JSON keys are the contract the Settings and Ryostore pages read.
func TestReportJSONShape(t *testing.T) {
	data, err := json.Marshal(Report{Schema: 1, Apps: []Row{{ID: "ryotunes", Lane: LaneRepo, Channel: "stable"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"schema":1`, `"id":"ryotunes"`, `"installed":false`, `"update":false`,
		`"removable":false`, `"builtin":false`, `"lane":"repo"`, `"channel":"stable"`} {
		if !bytes.Contains(data, []byte(key)) {
			t.Errorf("%s missing from %s", key, data)
		}
	}
	for _, key := range []string{`"version"`, `"latest"`, `"size_bytes"`, `"desktop_id"`} {
		if bytes.Contains(data, []byte(key)) {
			t.Errorf("empty %s present in %s", key, data)
		}
	}
}

func TestCatalogue(t *testing.T) {
	seen := map[string]bool{}
	for _, app := range All() {
		if app.ID == "" || app.Name == "" || app.Summary == "" || app.Package == "" || app.DesktopID == "" || app.Homepage == "" {
			t.Errorf("incomplete entry %+v", app)
		}
		if seen[app.ID] {
			t.Errorf("duplicate id %s", app.ID)
		}
		seen[app.ID] = true
		if strings.ContainsRune(app.Summary, '\u2014') {
			t.Errorf("%s summary has an em-dash", app.ID)
		}
		if app.Builtin && app.Package != desktopRoot {
			t.Errorf("built-in %s does not ship in %s", app.ID, desktopRoot)
		}
	}
	for _, id := range []string{"ryotunes", "ryoport", "ryomanager", "ryostore"} {
		if _, ok := Lookup(id); !ok {
			t.Errorf("%s missing", id)
		}
	}
	if _, ok := Lookup("firefox"); ok {
		t.Error("Lookup found an app outside the catalogue")
	}
}

func TestMainUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	for _, args := range [][]string{nil, {"install"}, {"list", "--yaml"}, {"list", "--json", "x"}} {
		if code := Main(args, &out, &errOut); code != 2 {
			t.Errorf("Main(%v) = %d, want 2", args, code)
		}
	}
}
