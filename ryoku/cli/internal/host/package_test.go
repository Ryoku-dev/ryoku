package host

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func packageFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "void.tsv")
	body := "arch\tvoid\tlanes\tnotes\ncombo\tvoid-a void-b\tdesktop\t\nrepo-pkg\t@repo\tdesktop\tbuilt here\nfont-pkg\t@fetch\tdesktop\tdownloaded\nmissing-pkg\t-\tdesktop\tcovered elsewhere\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestXBPSPackageTranslationAndMissingOnlyInstall(t *testing.T) {
	table := packageFixture(t)
	runner := &fakeRunner{answer: func(command Command) Result {
		if argv(command) == "xbps-query void-a" {
			return Result{}
		}
		if command.Name == "xbps-query" {
			return Result{Code: 1}
		}
		return Result{}
	}}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "xbps", "RYOKU_HOST_PKG_TABLE": table})
	if code := app.Package([]string{"install", "--aur", "combo", "identity"}); code != ExitOK {
		t.Fatal(code)
	}
	if got := argv(runner.commands[len(runner.commands)-1]); got != "xbps-install -Sy void-b identity" {
		t.Fatalf("install argv = %q", got)
	}

	runner.commands = nil
	if code := app.Package([]string{"install", "--upgrade", "combo"}); code != ExitOK {
		t.Fatal(code)
	}
	if got := argv(runner.commands[len(runner.commands)-1]); got != "xbps-install -Syu void-a void-b" {
		t.Fatalf("upgrade argv = %q", got)
	}
}

func TestXBPSSpecialMappingsAndIdentityFallback(t *testing.T) {
	table := packageFixture(t)
	for _, name := range []string{"repo-pkg", "font-pkg", "missing-pkg"} {
		runner := &fakeRunner{}
		app, stdout, stderr := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "xbps", "RYOKU_HOST_PKG_TABLE": table})
		if code := app.Package([]string{"installed", name}); code != ExitFalse {
			t.Fatalf("%s installed exit = %d", name, code)
		}
		if stdout.Len() != 0 || stderr.Len() != 0 {
			t.Fatalf("%s condition wrote stdout=%q stderr=%q", name, stdout.String(), stderr.String())
		}
		if code := app.Package([]string{"available", name}); code != ExitNotProvided {
			t.Fatalf("%s available exit = %d", name, code)
		}
		if stdout.Len() != 0 || stderr.Len() != 0 {
			t.Fatalf("%s availability wrote stdout=%q stderr=%q", name, stdout.String(), stderr.String())
		}
		if code := app.Package([]string{"install", name}); code != ExitNotProvided {
			t.Fatalf("%s install exit = %d", name, code)
		}
		if len(runner.commands) != 0 {
			t.Fatalf("%s executed a package command", name)
		}
	}

	runner := &fakeRunner{}
	app, _, stderr := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "xbps", "RYOKU_HOST_PKG_TABLE": filepath.Join(t.TempDir(), "absent")})
	if code := app.Package([]string{"local", "unlisted"}); code != ExitOK {
		t.Fatal(code)
	}
	if stderr.Len() == 0 {
		t.Fatal("missing table warning not written")
	}
}

func TestPacmanConfigUpgradeAndAURSelection(t *testing.T) {
	conf := filepath.Join(t.TempDir(), "pacman.conf")
	if err := os.WriteFile(conf, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{paths: map[string]bool{"pacman": true, "yay": true, "paru": true}}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "pacman", "RYOKU_PACMAN_CONF": conf})
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"available", "foo"}, "pacman --config " + conf + " -Si foo"},
		{[]string{"installed", "foo"}, "pacman -Q foo"},
		{[]string{"install", "foo"}, "pacman --config " + conf + " -S --needed --noconfirm foo"},
		{[]string{"install", "--upgrade", "foo"}, "pacman --config " + conf + " -Syu --needed --noconfirm foo"},
		{[]string{"install", "--aur", "foo"}, "yay -S --needed --noconfirm foo"},
	}
	for _, tc := range cases {
		runner.commands = nil
		if code := app.Package(tc.args); code != ExitOK {
			t.Fatalf("%v exit = %d", tc.args, code)
		}
		if got := argv(runner.commands[0]); got != tc.want {
			t.Fatalf("%v argv = %q, want %q", tc.args, got, tc.want)
		}
	}

	runner = &fakeRunner{paths: map[string]bool{"paru": true}}
	app, _, _ = testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "pacman"})
	if code := app.Package([]string{"install", "--aur", "foo"}); code != ExitOK {
		t.Fatal(code)
	}
	if got := argv(runner.commands[0]); got != "paru -S --needed --noconfirm foo" {
		t.Fatalf("paru argv = %q", got)
	}

	runner = &fakeRunner{}
	app, _, _ = testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "pacman"})
	if code := app.Package([]string{"install", "--aur", "foo"}); code != ExitAbsent {
		t.Fatalf("no helper exit = %d", code)
	}
}

func TestPacmanConditionsAreSilentAndVersionIsBare(t *testing.T) {
	runner := &fakeRunner{answer: func(command Command) Result {
		switch argv(command) {
		case "pacman -Q niri":
			return Result{Output: "niri 26.04-1\n"}
		case "pacman -Q missing":
			return Result{Code: 1, Output: "error output must stay private"}
		case "pacman -Si niri":
			return Result{Output: "Repository : extra\nName : niri\n"}
		default:
			return Result{}
		}
	}}
	app, stdout, stderr := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "pacman"})

	if code := app.Package([]string{"installed", "niri"}); code != ExitOK {
		t.Fatalf("installed exit = %d", code)
	}
	if code := app.Package([]string{"installed", "missing"}); code != ExitFalse {
		t.Fatalf("missing exit = %d", code)
	}
	if code := app.Package([]string{"available", "niri"}); code != ExitOK {
		t.Fatalf("available exit = %d", code)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("conditions wrote stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	for _, command := range runner.commands {
		if command.Stdout != io.Discard || command.Stderr != io.Discard {
			t.Fatalf("condition command did not discard output: %#v", command)
		}
	}

	runner.commands = nil
	if code := app.Package([]string{"version", "niri"}); code != ExitOK {
		t.Fatalf("version exit = %d", code)
	}
	if got := stdout.String(); got != "26.04-1\n" {
		t.Fatalf("version output = %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("version stderr = %q", stderr.String())
	}
}

func TestXBPSVersionIsBare(t *testing.T) {
	table := packageFixture(t)
	runner := &fakeRunner{answer: func(command Command) Result {
		if argv(command) == "xbps-query -p pkgver niri" {
			return Result{Output: "niri-26.04_1\n"}
		}
		return Result{Code: 1}
	}}
	app, stdout, stderr := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "xbps", "RYOKU_HOST_PKG_TABLE": table})
	if code := app.Package([]string{"version", "niri"}); code != ExitOK {
		t.Fatalf("version exit = %d", code)
	}
	if got := stdout.String(); got != "26.04_1\n" {
		t.Fatalf("version output = %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("version stderr = %q", stderr.String())
	}
}

func TestXBPSOwnershipAndExplicitStateUseNativeTools(t *testing.T) {
	table := packageFixture(t)
	runner := &fakeRunner{answer: func(command Command) Result {
		if strings.HasPrefix(argv(command), "xbps-query -o ") {
			return Result{Output: "ryogami-2.4.1_3: /usr/bin/ryogami\n"}
		}
		return Result{}
	}}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "xbps", "RYOKU_HOST_PKG_TABLE": table})
	if owner, err := app.PackageOwner("/usr/bin/ryogami"); err != nil || owner != "ryogami" {
		t.Fatalf("owner = %q, %v", owner, err)
	}
	if code := app.Package([]string{"explicit", "identity"}); code != ExitOK {
		t.Fatalf("explicit exit = %d", code)
	}
	if got := argv(runner.commands[len(runner.commands)-1]); got != "xbps-pkgdb -m manual identity" {
		t.Fatalf("explicit command = %q", got)
	}
}

func TestOrphansUsesHostPackageManager(t *testing.T) {
	tests := []struct {
		manager string
		command string
		output  string
		want    []string
	}{
		{"pacman", "pacman -Qdtq", "old-lib\nunused-tool\n", []string{"old-lib", "unused-tool"}},
		{"xbps", "xbps-query -O", "old-lib-1.0_1\nunused-tool-2.0_3\n", []string{"old-lib-1.0_1", "unused-tool-2.0_3"}},
	}
	for _, tc := range tests {
		t.Run(tc.manager, func(t *testing.T) {
			runner := &fakeRunner{answer: func(command Command) Result {
				return Result{Output: tc.output}
			}}
			app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": tc.manager})
			got, err := app.Orphans()
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("orphans = %v", got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("orphans = %v", got)
				}
			}
			if command := argv(runner.commands[0]); command != tc.command {
				t.Fatalf("command = %q, want %q", command, tc.command)
			}
		})
	}
}

func TestOrphansTreatsEmptyQueryAsNone(t *testing.T) {
	runner := &fakeRunner{answer: func(command Command) Result {
		return Result{Code: 1}
	}}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "pacman"})
	got, err := app.Orphans()
	if err != nil || len(got) != 0 {
		t.Fatalf("orphans = %v, err = %v", got, err)
	}
}

func TestOrphansReportsQueryFailure(t *testing.T) {
	runner := &fakeRunner{answer: func(command Command) Result {
		return Result{Code: 2}
	}}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "xbps"})
	if _, err := app.Orphans(); err == nil {
		t.Fatal("query failure returned no error")
	}
}
