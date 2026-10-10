package host

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRepoChannelAndSetChannelPacman(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "pacman.conf")
	body := "[options]\nHoldPkg = pacman\n\n[ryoku]\nSigLevel = Required\nServer = " + RepoURLFor(Pacman, "stable") + "\n"
	if err := os.WriteFile(conf, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	syncDir := filepath.Join(dir, "sync")
	if err := os.MkdirAll(syncDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(syncDir, "ryoku.db"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	app, _, _ := testApp(&fakeRunner{}, map[string]string{"RYOKU_HOST_PKGMGR": "pacman"})
	app.cfg.PacmanConf = conf
	app.cfg.PacmanSyncDir = syncDir

	if got, err := app.RepoChannel(); err != nil || got != "stable" {
		t.Fatalf("channel = %q, %v", got, err)
	}
	if err := app.RepoSetChannel("testing"); err != nil {
		t.Fatal(err)
	}
	if got, err := app.RepoChannel(); err != nil || got != "testing" {
		t.Fatalf("channel after set = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(syncDir, "ryoku.db")); !os.IsNotExist(err) {
		t.Fatalf("stale sync database survived: %v", err)
	}
}

func TestRepoChannelAndSetChannelXBPS(t *testing.T) {
	dir := t.TempDir()
	shipped := filepath.Join(dir, "usr", "20-ryoku.conf")
	if err := os.MkdirAll(filepath.Dir(shipped), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shipped, []byte("repository="+RepoURLFor(XBPS, "stable")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app, _, _ := testApp(&fakeRunner{}, map[string]string{"RYOKU_HOST_PKGMGR": "xbps"})
	app.cfg.XBPSConfigDir = filepath.Join(dir, "etc")
	app.cfg.XBPSShippedConfig = shipped

	if got, err := app.RepoChannel(); err != nil || got != "stable" {
		t.Fatalf("shipped channel = %q, %v", got, err)
	}
	if err := app.RepoSetChannel("v1.2.3"); err != nil {
		t.Fatal(err)
	}
	if got, err := app.RepoChannel(); err != nil || got != "v1.2.3" {
		t.Fatalf("override channel = %q, %v", got, err)
	}
	body, err := os.ReadFile(filepath.Join(app.cfg.XBPSConfigDir, xbpsRepoConfig))
	if err != nil {
		t.Fatal(err)
	}
	if want := "repository=" + RepoURLFor(XBPS, "v1.2.3") + "\n"; string(body) != want {
		t.Fatalf("override = %q, want %q", body, want)
	}
}

func TestRepoReleaseBaseOverrideForBothManagers(t *testing.T) {
	t.Setenv("RYOKU_RELEASE_BASE", "http://127.0.0.1:8080/bucket/")
	for _, manager := range []PackageManager{Pacman, XBPS} {
		t.Run(string(manager), func(t *testing.T) {
			arch := "$arch"
			if manager == XBPS {
				arch = "x86_64"
			}
			for _, channel := range []string{"stable", "testing", "v1.2.3"} {
				url := RepoURLFor(manager, channel)
				var suffix string
				switch channel {
				case "stable":
					suffix = "/" + arch
				case "testing":
					suffix = "/channels/testing/" + arch
				default:
					suffix = "/releases/" + channel + "/" + arch
				}
				want := "http://127.0.0.1:8080/bucket" + suffix
				if url != want {
					t.Fatalf("RepoURLFor(%s, %q) = %q, want %q", manager, channel, url, want)
				}
				if got := RepoChannelOf(manager, url); got != channel {
					t.Fatalf("RepoChannelOf(%s, %q) = %q, want %q", manager, url, got, channel)
				}
			}
		})
	}
}

func TestRepoProductionBasesWithoutOverride(t *testing.T) {
	t.Setenv("RYOKU_RELEASE_BASE", "")
	if got := RepoURLFor(Pacman, "stable"); got != PacmanRepoBase+"/$arch" {
		t.Fatalf("Pacman stable URL = %q", got)
	}
	if got := RepoURLFor(XBPS, "stable"); got != XBPSRepoBase+"/x86_64" {
		t.Fatalf("XBPS stable URL = %q", got)
	}
}

func TestRepoCommandExitAndSync(t *testing.T) {
	runner := &fakeRunner{}
	app, stdout, _ := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "xbps"})
	app.cfg.XBPSConfigDir = t.TempDir()
	app.cfg.XBPSShippedConfig = filepath.Join(t.TempDir(), "missing")
	if code := app.Repo([]string{"channel"}); code != ExitAbsent {
		t.Fatalf("missing channel exit = %d", code)
	}
	if code := app.Repo([]string{"sync"}); code != ExitOK {
		t.Fatalf("sync exit = %d", code)
	}
	if got := argv(runner.commands[0]); got != "xbps-install -S" {
		t.Fatalf("sync argv = %q", got)
	}
	if stdout.Len() != 0 {
		t.Fatalf("missing channel wrote %q", stdout.String())
	}
}

func TestParseXBPSRepoPackagesAndInstalledSet(t *testing.T) {
	listing := "[-] ryoku-desktop-0.75.0_1 Desktop\n[-] ryoku-desktop-niri-0.75.0_1 Desktop\n[-] ryogami-2.4.1_3 Wallpaper\n"
	packages := parseXBPSRepoPackages(listing, "/repo")
	if got := []string{packages[0].Name, packages[1].Name, packages[2].Name}; !reflect.DeepEqual(got, []string{"ryogami", "ryoku-desktop", "ryoku-desktop-niri"}) {
		t.Fatalf("packages = %v", got)
	}

	dir := t.TempDir()
	conf := filepath.Join(dir, "20-ryoku.conf")
	if err := os.WriteFile(conf, []byte("repository=/repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{answer: func(command Command) Result {
		joined := argv(command)
		switch {
		case strings.Contains(joined, "--repository /repo -Rs"):
			return Result{Output: listing}
		case joined == "xbps-query -p pkgver ryoku-desktop":
			return Result{Output: "ryoku-desktop-0.74.0_1\n"}
		case joined == "xbps-query -p pkgver ryoku-desktop-niri":
			return Result{Output: "ryoku-desktop-niri-0.76.0_1\n"}
		case joined == "xbps-query -p pkgver ryogami":
			return Result{Code: 2}
		case strings.HasPrefix(joined, "xbps-uhelper cmpver 0.74.0_1"):
			return Result{Output: "-1\n"}
		case strings.HasPrefix(joined, "xbps-uhelper cmpver 0.76.0_1"):
			return Result{Output: "1\n"}
		default:
			return Result{Code: 1}
		}
	}}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "xbps"})
	app.cfg.XBPSConfigDir = dir
	app.cfg.XBPSShippedConfig = filepath.Join(t.TempDir(), "missing")
	set, held, err := app.RepoInstalledSet(false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(set, []string{"ryoku-desktop"}) || held != 1 {
		t.Fatalf("set = %v, held = %d", set, held)
	}
}
