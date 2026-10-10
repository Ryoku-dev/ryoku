package host

import (
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

func packageFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "void.tsv")
	body := "arch\tvoid\tlanes\tnotes\ncombo\tvoid-a void-b\tdesktop\t\nrepo-pkg\t@repo\tdesktop\tbuilt here\nmissing-pkg\t-\tdesktop\tcovered elsewhere\nempty-note\t-\tdesktop\t\n"
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
	if code := app.Package([]string{"install", "--aur", "--overwrite", "usr/lib/ryoku/*", "combo", "identity"}); code != ExitOK {
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

func TestXBPSRepoMappingsAndUnavailablePackages(t *testing.T) {
	table := packageFixture(t)
	runner := &fakeRunner{answer: func(command Command) Result {
		switch argv(command) {
		case "xbps-query repo-pkg", "xbps-query -R repo-pkg":
			return Result{}
		case "xbps-query -p pkgver repo-pkg":
			return Result{Output: "repo-pkg-0.1.0_1\n"}
		default:
			if command.Name == "xbps-query" {
				return Result{Code: 1}
			}
			return Result{}
		}
	}}
	app, stdout, stderr := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "xbps", "RYOKU_HOST_PKG_TABLE": table})
	if code := app.Package([]string{"installed", "repo-pkg"}); code != ExitOK {
		t.Fatalf("repo package installed exit = %d", code)
	}
	if code := app.Package([]string{"available", "repo-pkg"}); code != ExitOK {
		t.Fatalf("repo package available exit = %d", code)
	}
	if code := app.Package([]string{"installed", "missing-pkg"}); code != ExitFalse {
		t.Fatalf("unavailable package installed exit = %d", code)
	}
	if code := app.Package([]string{"available", "missing-pkg"}); code != ExitNotProvided {
		t.Fatalf("unavailable package available exit = %d", code)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("conditions wrote stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if code := app.Package([]string{"version", "repo-pkg"}); code != ExitOK {
		t.Fatalf("repo package version exit = %d", code)
	}
	if got := stdout.String(); got != "0.1.0_1\n" {
		t.Fatalf("repo package version = %q", got)
	}
	stdout.Reset()

	runner.commands = nil
	if code := app.Package([]string{"install", "repo-pkg"}); code != ExitOK {
		t.Fatalf("repo package install exit = %d", code)
	}
	if len(runner.commands) != 1 || argv(runner.commands[0]) != "xbps-query repo-pkg" {
		t.Fatalf("repo package install commands = %v", runner.commands)
	}

	runner.commands = nil
	if code := app.Package([]string{"remove", "repo-pkg"}); code != ExitOK {
		t.Fatalf("repo package remove exit = %d", code)
	}
	if len(runner.commands) != 2 || argv(runner.commands[1]) != "xbps-remove -y repo-pkg" {
		t.Fatalf("repo package remove commands = %v", runner.commands)
	}

	runner.commands = nil
	if code := app.Package([]string{"install", "missing-pkg"}); code != ExitNotProvided {
		t.Fatalf("unavailable package install exit = %d", code)
	}
	if len(runner.commands) != 0 {
		t.Fatalf("unavailable package executed commands: %v", runner.commands)
	}

	runner = &fakeRunner{}
	app, _, stderr = testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "xbps", "RYOKU_HOST_PKG_TABLE": filepath.Join(t.TempDir(), "absent")})
	if code := app.Package([]string{"local", "unlisted"}); code != ExitOK {
		t.Fatal(code)
	}
	if stderr.Len() == 0 {
		t.Fatal("missing table warning not written")
	}
}

func TestPackageTableRejectsRetiredFetchMappings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "void.tsv")
	body := "arch\tvoid\tlanes\tnotes\nfont-pkg\t@fetch\tdesktop\tdownloaded\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readPackageTable(path); err == nil || !strings.Contains(err.Error(), "@fetch mappings are retired") {
		t.Fatalf("readPackageTable error = %v", err)
	}
}

func TestPackageWhyExplainsOnlyVoidDashMappings(t *testing.T) {
	table := packageFixture(t)
	runner := &fakeRunner{}
	app, stdout, stderr := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "xbps", "RYOKU_HOST_INIT": "runit", "RYOKU_HOST_PKG_TABLE": table})
	cases := []struct {
		name string
		code int
		want string
	}{
		{"missing-pkg", ExitOK, "covered elsewhere\n"},
		{"empty-note", ExitOK, "Not packaged for Void.\n"},
		{"combo", ExitFalse, ""},
		{"identity", ExitFalse, ""},
		{"repo-pkg", ExitFalse, ""},
	}
	for _, tc := range cases {
		stdout.Reset()
		if code := app.Package([]string{"why", tc.name}); code != tc.code {
			t.Fatalf("%s exit = %d, want %d", tc.name, code, tc.code)
		}
		if got := stdout.String(); got != tc.want {
			t.Fatalf("%s output = %q, want %q", tc.name, got, tc.want)
		}
	}
	if stderr.Len() != 0 || len(runner.commands) != 0 {
		t.Fatalf("why wrote stderr=%q or ran commands=%v", stderr.String(), runner.commands)
	}

	app, stdout, stderr = testApp(&fakeRunner{}, map[string]string{"RYOKU_HOST_PKGMGR": "pacman", "RYOKU_HOST_INIT": "systemd"})
	if code := app.Package([]string{"why", "missing-pkg"}); code != ExitFalse {
		t.Fatalf("pacman why exit = %d", code)
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("pacman why wrote stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if code := app.Package([]string{"why"}); code != ExitUsage {
		t.Fatalf("why without name exit = %d", code)
	}
}

func TestPackageAdviceUsesHostInstallCommand(t *testing.T) {
	table := packageFixture(t)
	tests := []struct {
		manager string
		env     map[string]string
		want    string
	}{
		{"pacman", map[string]string{"RYOKU_HOST_PKGMGR": "pacman", "RYOKU_HOST_INIT": "systemd"}, "sudo pacman -S combo identity\n"},
		{"xbps", map[string]string{"RYOKU_HOST_PKGMGR": "xbps", "RYOKU_HOST_INIT": "runit", "RYOKU_HOST_PKG_TABLE": table}, "sudo xbps-install -S void-a void-b identity\n"},
	}
	for _, tc := range tests {
		t.Run(tc.manager, func(t *testing.T) {
			app, stdout, _ := testApp(&fakeRunner{}, tc.env)
			if code := app.Package([]string{"advice", "combo", "identity"}); code != ExitOK {
				t.Fatalf("advice exit = %d", code)
			}
			if got := stdout.String(); got != tc.want {
				t.Fatalf("advice output = %q, want %q", got, tc.want)
			}
			if code := app.Package([]string{"advice"}); code != ExitUsage {
				t.Fatalf("advice without names exit = %d", code)
			}
		})
	}
}

func TestPackageAvailableAURUsesHostPolicy(t *testing.T) {
	pacmanRunner := &fakeRunner{}
	app, _, _ := testApp(pacmanRunner, map[string]string{"RYOKU_HOST_PKGMGR": "pacman", "RYOKU_HOST_INIT": "systemd"})
	if code := app.Package([]string{"available", "--aur", "voxtype"}); code != ExitOK {
		t.Fatalf("pacman AUR availability exit = %d", code)
	}
	if len(pacmanRunner.commands) != 0 {
		t.Fatalf("pacman AUR availability ran commands: %v", pacmanRunner.commands)
	}

	table := packageFixture(t)
	xbpsRunner := &fakeRunner{}
	app, _, _ = testApp(xbpsRunner, map[string]string{"RYOKU_HOST_PKGMGR": "xbps", "RYOKU_HOST_INIT": "runit", "RYOKU_HOST_PKG_TABLE": table})
	if code := app.Package([]string{"available", "--aur", "combo"}); code != ExitOK {
		t.Fatalf("XBPS mapped AUR availability exit = %d", code)
	}
	if len(xbpsRunner.commands) != 2 || argv(xbpsRunner.commands[0]) != "xbps-query -R void-a" || argv(xbpsRunner.commands[1]) != "xbps-query -R void-b" {
		t.Fatalf("XBPS mapped availability commands = %v", xbpsRunner.commands)
	}
	xbpsRunner.commands = nil
	if code := app.Package([]string{"available", "--aur", "missing-pkg"}); code != ExitNotProvided {
		t.Fatalf("XBPS unavailable AUR exit = %d", code)
	}
	if len(xbpsRunner.commands) != 0 {
		t.Fatalf("XBPS unavailable mapping ran commands: %v", xbpsRunner.commands)
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
		{[]string{"install", "--overwrite", "usr/lib/ryoku/*", "--overwrite", "etc/ryoku/*", "foo"}, "pacman --config " + conf + " -S --needed --noconfirm --overwrite usr/lib/ryoku/* --overwrite etc/ryoku/* foo"},
		{[]string{"install", "--aur", "--overwrite", "usr/lib/ryoku/*", "foo"}, "yay -S --needed --noconfirm --overwrite usr/lib/ryoku/* foo"},
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

func TestAURInstallDropsToInvokingUserUnderSudo(t *testing.T) {
	lookupUser = func(name string) (*user.User, error) {
		return &user.User{Username: name, HomeDir: "/home/" + name}, nil
	}
	t.Cleanup(func() { lookupUser = user.Lookup })
	runner := &fakeRunner{paths: map[string]bool{"yay": true}}
	app, _, _ := testApp(runner, map[string]string{
		"RYOKU_HOST_PKGMGR": "pacman", "SUDO_USER": "hilda",
	})
	app.cfg.UID = 0
	if code := app.Package([]string{"install", "--aur", "apple_cursor"}); code != ExitOK {
		t.Fatalf("exit = %d", code)
	}
	want := "runuser -u hilda -- env HOME=/home/hilda USER=hilda LOGNAME=hilda yay -S --needed --noconfirm apple_cursor"
	if got := argv(runner.commands[0]); got != want {
		t.Fatalf("argv = %q, want %q", got, want)
	}

	runner.commands = nil
	app.cfg.UID = 1000
	if code := app.Package([]string{"install", "--aur", "apple_cursor"}); code != ExitOK {
		t.Fatalf("user-side exit = %d", code)
	}
	if got := argv(runner.commands[0]); got != "yay -S --needed --noconfirm apple_cursor" {
		t.Fatalf("user-side argv = %q", got)
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
