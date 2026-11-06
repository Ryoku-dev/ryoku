package host

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func provisionFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	share := filepath.Join(root, "usr", "share", "ryoku")
	for path, body := range map[string]string{
		"brand/logo.svg":                     "brand\n",
		"ryodecors/decor.png":                "decor\n",
		"npmrc":                              "prefix=${HOME}/.local\n",
		"zsh/zshrc":                          "packaged zsh config\n",
		"../applications/nvim.desktop":       "[Desktop Entry]\nName=Neovim\nExec=nvim\n",
		"../applications/ryoku-nvim.desktop": "[Desktop Entry]\nName=Ryoku Neovim\nExec=ryoku-nvim\n",
		"../applications/hidden.desktop":     "[Desktop Entry]\nName=Hidden\nExec=hidden\nActions=Open;\n\n[Desktop Action Open]\nName=Open\nExec=hidden\n",
	} {
		target := filepath.Join(share, path)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return share, filepath.Join(root, "home")
}

func provisionTestApp(t *testing.T, share, home string) (*App, *fakeRunner, *bytes.Buffer) {
	t.Helper()
	return provisionTestAppWithShell(t, share, home, "/bin/bash")
}

func provisionTestAppWithShell(t *testing.T, share, home, shell string) (*App, *fakeRunner, *bytes.Buffer) {
	t.Helper()
	runner := &fakeRunner{answer: func(command Command) Result {
		if command.Name == "/usr/bin/ryoku-session-intro" {
			return Result{Output: "19\n"}
		}
		return Result{}
	}}
	app, stdout, _ := testApp(runner, map[string]string{
		"HOME":                home,
		"USER":                "student",
		"LOGNAME":             "student",
		"SHELL":               shell,
		"RYOKU_SHARE_ROOT":    share,
		"RYOKU_DROP_PACKAGES": "pkg-one,pkg-two",
	})
	app.cfg.UID, app.cfg.UIDSet = 1000, true
	return app, runner, stdout
}

func TestUsersSeedSelfWritesExpectedTreeAndIsIdempotent(t *testing.T) {
	share, home := provisionFixture(t)
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	app, _, stdout := provisionTestApp(t, share, home)
	args := []string{"seed-self", "--rice", "hacker", "--rashin", "--hide-app", "hidden.desktop", "--skip-welcome"}
	if code := app.Users(args); code != ExitOK {
		t.Fatalf("seed-self exit = %d, output = %s", code, stdout.String())
	}
	var response usersMutationResult
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil || !response.OK {
		t.Fatalf("response = %q, error = %v", stdout.String(), err)
	}

	wantTree := []string{
		".config", ".config/user-dirs.dirs", ".local", ".local/share", ".local/share/applications",
		".local/share/applications/hidden.desktop", ".local/share/applications/nvim.desktop",
		".local/share/ryoku", ".local/share/ryoku/assets", ".local/share/ryoku/assets/brand",
		".local/share/ryoku/assets/brand/logo.svg", ".local/state", ".local/state/ryoku",
		".local/state/ryoku/default-rice-pending", ".local/state/ryoku/launcher-overrides",
		".local/state/ryoku/provisioned", ".local/state/ryoku/rashin-setup-pending",
		".local/state/ryoku/welcome-seen", ".npmrc", "Desktop", "Documents", "Downloads", "Music",
		"Pictures", "Pictures/ryodecors", "Pictures/ryodecors/decor.png", "Public", "Templates", "Videos", "Videos/Recordings",
	}
	sort.Strings(wantTree)
	firstTree, firstFiles := snapshotProvisionHome(t, home)
	if !reflect.DeepEqual(firstTree, wantTree) {
		t.Fatalf("tree = %#v\nwant %#v", firstTree, wantTree)
	}
	for path, want := range map[string]string{
		".local/state/ryoku/default-rice-pending": "hacker\n",
		".local/state/ryoku/rashin-setup-pending": "pending\n",
		".local/state/ryoku/welcome-seen":         "19",
		".local/state/ryoku/provisioned":          "pkg-one\npkg-two\n",
		".local/state/ryoku/launcher-overrides":   "hidden.desktop\nnvim.desktop\n",
	} {
		if got := firstFiles[path]; got != want {
			t.Errorf("%s = %q, want %q", path, got, want)
		}
	}
	for _, path := range []string{".local/share/applications/hidden.desktop", ".local/share/applications/nvim.desktop"} {
		body := firstFiles[path]
		if !strings.Contains(body, "NoDisplay=true\n") || strings.Contains(body, "Hidden=true") {
			t.Errorf("override %s = %q", path, body)
		}
	}

	stdout.Reset()
	if code := app.Users(args); code != ExitOK {
		t.Fatalf("second seed-self exit = %d, output = %s", code, stdout.String())
	}
	secondTree, secondFiles := snapshotProvisionHome(t, home)
	if !reflect.DeepEqual(secondTree, firstTree) || !reflect.DeepEqual(secondFiles, firstFiles) {
		t.Fatalf("second seed changed the home\nfirst tree=%#v\nsecond tree=%#v", firstTree, secondTree)
	}
}

func TestUsersSeedSelfSeedsZshStartupFileOnlyWhenMissing(t *testing.T) {
	share, zshHome := provisionFixture(t)
	if err := os.MkdirAll(zshHome, 0o700); err != nil {
		t.Fatal(err)
	}
	zshApp, _, stdout := provisionTestAppWithShell(t, share, zshHome, "/usr/bin/zsh")
	if code := zshApp.Users([]string{"seed-self"}); code != ExitOK {
		t.Fatalf("zsh seed-self exit = %d, output = %s", code, stdout.String())
	}
	zshrc := filepath.Join(zshHome, ".zshrc")
	body, err := os.ReadFile(zshrc)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(body), "packaged zsh config\n"; got != want {
		t.Fatalf(".zshrc = %q, want %q", got, want)
	}

	const custom = "custom zsh config\n"
	if err := os.WriteFile(zshrc, []byte(custom), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if code := zshApp.Users([]string{"seed-self"}); code != ExitOK {
		t.Fatalf("second zsh seed-self exit = %d, output = %s", code, stdout.String())
	}
	body, err = os.ReadFile(zshrc)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != custom {
		t.Fatalf("existing .zshrc was overwritten with %q", body)
	}

	bashHome := filepath.Join(filepath.Dir(zshHome), "bash-home")
	if err := os.MkdirAll(bashHome, 0o700); err != nil {
		t.Fatal(err)
	}
	bashApp, _, stdout := provisionTestAppWithShell(t, share, bashHome, "/bin/bash")
	if code := bashApp.Users([]string{"seed-self"}); code != ExitOK {
		t.Fatalf("bash seed-self exit = %d, output = %s", code, stdout.String())
	}
	if _, err := os.Stat(filepath.Join(bashHome, ".zshrc")); !os.IsNotExist(err) {
		t.Fatalf("non-zsh seed created .zshrc: %v", err)
	}
}

func TestUsersSeedSelfRefusesSymlinkedDestination(t *testing.T) {
	share, home := provisionFixture(t)
	outside := t.TempDir()
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(home, "Pictures")); err != nil {
		t.Fatal(err)
	}
	app, _, _ := provisionTestApp(t, share, home)
	if code := app.Users([]string{"seed-self"}); code != ExitFailure {
		t.Fatalf("seed-self exit = %d", code)
	}
	entries, err := os.ReadDir(outside)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("symlink target was modified: %#v", entries)
	}
}

func TestUsersProvisionUsesCleanAccountEnvironment(t *testing.T) {
	passwdPath, groupPath := writeUsersFixtures(t)
	runner := &fakeRunner{}
	app, stdout, _ := testApp(runner, map[string]string{
		"LANG":            "en_GB.UTF-8",
		"RYOKU_HOST_INIT": "systemd",
	})
	app.cfg.PasswdPath, app.cfg.GroupPath = passwdPath, groupPath
	app.cfg.UID, app.cfg.UIDSet = 0, true
	if code := app.Users([]string{"provision", "bob", "--rice", "hacker", "--rashin", "--hide-app", "hidden.desktop", "--skip-welcome"}); code != ExitOK {
		t.Fatalf("provision exit = %d, output = %s", code, stdout.String())
	}
	if len(runner.commands) != 1 {
		t.Fatalf("commands = %#v", runner.commands)
	}
	command := runner.commands[0]
	if command.Name != "runuser" {
		t.Fatalf("command = %#v", command)
	}
	joined := strings.Join(command.Args, " ")
	for _, part := range []string{
		"-u bob -- /usr/bin/env -i", "HOME=/home/bob", "USER=bob", "LOGNAME=bob", "SHELL=/bin/zsh",
		"PATH=/usr/local/bin:/usr/bin:/bin", "LANG=en_GB.UTF-8", "RYOKU_HOST_INIT=systemd",
		"/usr/bin/ryoku-host users seed-self --rice hacker --rashin --hide-app hidden.desktop --skip-welcome",
	} {
		if !strings.Contains(joined, part) {
			t.Errorf("argv %q does not contain %q", joined, part)
		}
	}
}

func TestNoProvisionFlagIsInternalAndPositionIndependent(t *testing.T) {
	filtered, noProvision, err := takeNoProvision([]string{"alice", "--no-provision", "--full-name", "Alice", "--role", "standard"})
	if err != nil {
		t.Fatal(err)
	}
	if !noProvision || !reflect.DeepEqual(filtered, []string{"alice", "--full-name", "Alice", "--role", "standard"}) {
		t.Fatalf("filtered = %#v, noProvision = %t", filtered, noProvision)
	}
	if _, _, err := takeNoProvision([]string{"--no-provision", "--no-provision"}); err == nil {
		t.Fatal("duplicate --no-provision succeeded")
	}
}

func snapshotProvisionHome(t *testing.T, home string) ([]string, map[string]string) {
	t.Helper()
	var tree []string
	files := map[string]string{}
	err := filepath.WalkDir(home, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == home {
			return nil
		}
		relative, err := filepath.Rel(home, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		tree = append(tree, relative)
		if entry.Type().IsRegular() {
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			files[relative] = string(body)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(tree)
	return tree, files
}
