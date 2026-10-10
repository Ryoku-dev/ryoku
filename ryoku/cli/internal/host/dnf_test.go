package host

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fedoraPackageFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fedora.tsv")
	body := "arch\tfedora\tlanes\tnotes\nfd\tfd-find\tdesktop\t\ngithub-cli\tgh\tdesktop\t\nmatugen\tryoku-extras\tdesktop\tBundled by the Ryoku extras RPM.\nmissing\t-\tdesktop\tNot in Fedora.\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDNFPackageCommandsForDNF5AndDNF4(t *testing.T) {
	table := fedoraPackageFixture(t)
	tests := []struct {
		name          string
		paths         map[string]bool
		command       string
		available     string
		explicit      string
		remove        string
		installAdvice string
	}{
		{
			name:          "dnf5",
			paths:         map[string]bool{"dnf5": true, "rpm": true},
			command:       "dnf5",
			available:     "dnf5 repoquery -y --available --qf %{name}\n fd-find",
			explicit:      "dnf5 mark user -y fd-find",
			remove:        "dnf5 remove -y --setopt=clean_requirements_on_remove=False fd-find",
			installAdvice: "sudo dnf5 install fd-find gh",
		},
		{
			name:          "dnf4",
			paths:         map[string]bool{"dnf": true, "rpm": true},
			command:       "dnf",
			available:     "dnf repoquery -y --qf %{name}\n fd-find",
			explicit:      "dnf mark install -y fd-find",
			remove:        "dnf remove -y --setopt=clean_requirements_on_remove=False fd-find",
			installAdvice: "sudo dnf install fd-find gh",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeRunner{paths: tc.paths, answer: func(command Command) Result {
				if strings.HasPrefix(argv(command), tc.command+" repoquery") {
					return Result{Output: "fd-find\n"}
				}
				if command.Name == "rpm" {
					return Result{}
				}
				return Result{}
			}}
			app, _, stderr := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "dnf", "RYOKU_HOST_PKG_TABLE": table})

			if got := app.InstallAdvice("fd", "github-cli"); got != tc.installAdvice {
				t.Fatalf("InstallAdvice = %q, want %q", got, tc.installAdvice)
			}
			if code := app.Package([]string{"install", "fd", "github-cli"}); code != ExitOK {
				t.Fatalf("install exit = %d, stderr = %q", code, stderr.String())
			}
			if got := argv(runner.commands[len(runner.commands)-1]); got != tc.command+" install -y --setopt=install_weak_deps=False fd-find gh" {
				t.Fatalf("install argv = %q", got)
			}

			runner.commands = nil
			if code := app.Package([]string{"available", "fd"}); code != ExitOK {
				t.Fatalf("available exit = %d", code)
			}
			if got := argv(runner.commands[0]); got != tc.available {
				t.Fatalf("available argv = %q, want %q", got, tc.available)
			}

			runner.commands = nil
			if code := app.Package([]string{"available", "--aur", "fd"}); code != ExitNotProvided {
				t.Fatalf("AUR availability exit = %d, want %d", code, ExitNotProvided)
			}
			if code := app.Package([]string{"install", "--aur", "fd"}); code != ExitNotProvided {
				t.Fatalf("AUR install exit = %d, want %d", code, ExitNotProvided)
			}
			if len(runner.commands) != 0 {
				t.Fatalf("unsupported AUR operations ran commands: %v", runner.commands)
			}

			runner.commands = nil
			if code := app.Package([]string{"explicit", "fd"}); code != ExitOK {
				t.Fatalf("explicit exit = %d", code)
			}
			if got := argv(runner.commands[len(runner.commands)-1]); got != tc.explicit {
				t.Fatalf("explicit argv = %q, want %q", got, tc.explicit)
			}

			runner.commands = nil
			if code := app.Package([]string{"remove", "fd"}); code != ExitOK {
				t.Fatalf("remove exit = %d", code)
			}
			if got := argv(runner.commands[len(runner.commands)-1]); got != tc.remove {
				t.Fatalf("remove argv = %q, want %q", got, tc.remove)
			}

			runner.commands = nil
			if code := app.Package([]string{"install", "missing"}); code != ExitNotProvided {
				t.Fatalf("unavailable install exit = %d", code)
			}
			if len(runner.commands) != 0 {
				t.Fatalf("unavailable install ran commands: %v", runner.commands)
			}
		})
	}
}

func TestDNF4AvailabilityFallsBackWithoutRepoquery(t *testing.T) {
	table := fedoraPackageFixture(t)
	runner := &fakeRunner{paths: map[string]bool{"dnf": true}, answer: func(command Command) Result {
		switch argv(command) {
		case "dnf repoquery -y --qf %{name}\n fd-find":
			return Result{Code: 1}
		case "dnf list -y --available fd-find":
			return Result{}
		default:
			return Result{Code: 1}
		}
	}}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "dnf", "RYOKU_HOST_PKG_TABLE": table})
	if code := app.Package([]string{"available", "fd"}); code != ExitOK {
		t.Fatalf("available exit = %d", code)
	}
	got := []string{argv(runner.commands[0]), argv(runner.commands[1])}
	want := []string{"dnf repoquery -y --qf %{name}\n fd-find", "dnf list -y --available fd-find"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("availability commands = %v, want %v", got, want)
	}
}

func TestDNFPackageWhyAndTableFallback(t *testing.T) {
	table := fedoraPackageFixture(t)
	app, stdout, stderr := testApp(&fakeRunner{}, map[string]string{"RYOKU_HOST_PKGMGR": "dnf", "RYOKU_HOST_PKG_TABLE": table})
	if code := app.Package([]string{"why", "missing"}); code != ExitOK {
		t.Fatalf("why exit = %d", code)
	}
	if got := stdout.String(); got != "Not in Fedora.\n" {
		t.Fatalf("why output = %q", got)
	}
	stdout.Reset()
	if code := app.Package([]string{"why", "matugen"}); code != ExitFalse {
		t.Fatalf("available mapped why exit = %d, want %d", code, ExitFalse)
	}
	if stdout.Len() != 0 {
		t.Fatalf("available mapped why output = %q, want empty", stdout.String())
	}

	missing := filepath.Join(t.TempDir(), "missing.tsv")
	app, stdout, stderr = testApp(&fakeRunner{}, map[string]string{"RYOKU_HOST_PKGMGR": "dnf", "RYOKU_HOST_PKG_TABLE": missing})
	if code := app.Package([]string{"local", "unlisted"}); code != ExitOK {
		t.Fatalf("identity fallback exit = %d", code)
	}
	if stdout.String() != "unlisted\n" || !strings.Contains(stderr.String(), "using identity package names") {
		t.Fatalf("identity fallback stdout=%q stderr=%q", stdout.String(), stderr.String())
	}

	malformed := filepath.Join(t.TempDir(), "malformed.tsv")
	if err := os.WriteFile(malformed, []byte("arch\tfedora\tlanes\tnotes\nbroken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app, _, stderr = testApp(&fakeRunner{}, map[string]string{"RYOKU_HOST_PKGMGR": "dnf", "RYOKU_HOST_PKG_TABLE": malformed})
	if code := app.Package([]string{"local", "broken"}); code != ExitFailure {
		t.Fatalf("malformed table exit = %d", code)
	}
	if !strings.Contains(stderr.String(), "malformed package row") {
		t.Fatalf("malformed table stderr = %q", stderr.String())
	}
}

func TestDNFOrphansForDNF5AndDNF4(t *testing.T) {
	tests := []struct {
		name    string
		paths   map[string]bool
		command string
	}{
		{"dnf5", map[string]bool{"dnf5": true}, "dnf5 repoquery -y --unneeded --qf %{name}\n"},
		{"dnf4", map[string]bool{"dnf": true}, "dnf repoquery -y --unneeded --qf %{name}\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeRunner{paths: tc.paths, answer: func(command Command) Result {
				output := "jq\n"
				if tc.name == "dnf4" {
					output = "jq\n\n"
				}
				return Result{Output: output}
			}}
			app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "dnf"})
			got, err := app.Orphans()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, []string{"jq"}) {
				t.Fatalf("orphans = %v, want [jq]", got)
			}
			if command := argv(runner.commands[0]); command != tc.command {
				t.Fatalf("orphan argv = %q, want %q", command, tc.command)
			}
		})
	}
}

func TestDNFForeignCountHandlesDNF5AndDNF4Rows(t *testing.T) {
	for _, tc := range []struct {
		name    string
		paths   map[string]bool
		output  string
		command string
	}{
		{"dnf5", map[string]bool{"dnf5": true}, "bash\n", "dnf5 repoquery -y --extras --qf %{name}\n"},
		{"dnf4", map[string]bool{"dnf": true}, "bash\n\n", "dnf repoquery -y --extras --qf %{name}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeRunner{paths: tc.paths, answer: func(command Command) Result {
				return Result{Output: tc.output}
			}}
			app, stdout, _ := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "dnf"})
			if code := app.Package([]string{"count", "--foreign"}); code != ExitOK {
				t.Fatalf("count exit = %d", code)
			}
			if stdout.String() != "1\n" {
				t.Fatalf("count output = %q, want 1", stdout.String())
			}
			if got := argv(runner.commands[0]); got != tc.command {
				t.Fatalf("count argv = %q, want %q", got, tc.command)
			}
		})
	}
}

func TestDNF4OrphansFallbackParsesAutoremovePlan(t *testing.T) {
	listing := "Dependencies resolved.\nRemoving:\n old-app x86_64 1.0-1 @commandline 1 M\nRemoving unused dependencies:\n old-lib noarch 2.0-1 fedora 2 M\nTransaction Summary\nRemove 2 Packages\nOperation aborted.\n"
	runner := &fakeRunner{paths: map[string]bool{"dnf": true}, answer: func(command Command) Result {
		if strings.HasPrefix(argv(command), "dnf repoquery") {
			return Result{Code: 1}
		}
		return Result{Code: 1, Output: listing}
	}}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "dnf"})
	got, err := app.Orphans()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"old-app", "old-lib"}) {
		t.Fatalf("orphans = %v", got)
	}
	if command := argv(runner.commands[1]); command != "dnf autoremove -y --assumeno" {
		t.Fatalf("fallback argv = %q", command)
	}
}

func TestDNFCommandPrefersDNF5(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"dnf", "dnf5"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	if got := DNFCommand(); got != "dnf5" {
		t.Fatalf("DNFCommand = %q", got)
	}
	if err := os.Remove(filepath.Join(dir, "dnf5")); err != nil {
		t.Fatal(err)
	}
	if got := DNFCommand(); got != "dnf" {
		t.Fatalf("DNFCommand without dnf5 = %q", got)
	}
}

func TestDNFDetectionRequiresRPM(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paths map[string]bool
		want  PackageManager
	}{
		{"dnf5", map[string]bool{"dnf5": true, "rpm": true}, DNF},
		{"dnf4", map[string]bool{"dnf": true, "rpm": true}, DNF},
		{"missing-rpm", map[string]bool{"dnf5": true}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app, _, _ := testApp(&fakeRunner{paths: tc.paths}, nil)
			got, err := app.PackageManager()
			if tc.want == "" {
				if err == nil {
					t.Fatalf("PackageManager = %q without rpm", got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("PackageManager = %q, %v, want %q", got, err, tc.want)
			}
		})
	}
}
