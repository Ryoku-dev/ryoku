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
	if err := app.RepoSetChannel("stable"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(app.cfg.XBPSConfigDir, xbpsRepoConfig)); !os.IsNotExist(err) {
		t.Fatalf("production stable override survived: %v", err)
	}
}

func TestRepoReleaseBaseOverrideForBothManagers(t *testing.T) {
	t.Setenv("RYOKU_RELEASE_BASE", "http://127.0.0.1:8080/bucket/")
	build := "v0.94.1-beta.20.dev.12+g1234567"
	buildDir := "v0.94.1-beta.20.dev.12_g1234567"
	for _, manager := range []PackageManager{Pacman, XBPS, DNF} {
		t.Run(string(manager), func(t *testing.T) {
			arch := "$arch"
			if manager == XBPS || manager == DNF {
				arch = "x86_64"
			}
			for _, channel := range []string{"stable", "testing", "v1.2.3", build} {
				url := RepoURLFor(manager, channel)
				var suffix string
				switch channel {
				case "stable":
					suffix = "/" + arch
				case "testing":
					suffix = "/channels/testing/" + arch
				case build:
					suffix = "/channels/testing/builds/" + buildDir + "/" + arch
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

func TestRepoSetStableHonorsReleaseBaseOverride(t *testing.T) {
	const base = "http://127.0.0.1:8080/bucket"
	t.Setenv("RYOKU_RELEASE_BASE", base)

	t.Run("pacman", func(t *testing.T) {
		dir := t.TempDir()
		conf := filepath.Join(dir, "pacman.conf")
		if err := os.WriteFile(conf, []byte("[ryoku]\nServer = "+PacmanRepoBase+"/$arch\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		app, _, _ := testApp(&fakeRunner{}, map[string]string{"RYOKU_HOST_PKGMGR": "pacman"})
		app.cfg.PacmanConf = conf
		app.cfg.PacmanSyncDir = filepath.Join(dir, "sync")
		if err := app.RepoSetChannel("stable"); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(conf)
		if err != nil {
			t.Fatal(err)
		}
		want := "Server = " + base + "/$arch"
		if !strings.Contains(string(raw), want) {
			t.Fatalf("pacman stable override = %q, want %q", raw, want)
		}
	})

	t.Run("xbps", func(t *testing.T) {
		dir := t.TempDir()
		shipped := filepath.Join(dir, "shipped.conf")
		if err := os.WriteFile(shipped, []byte("repository="+XBPSRepoBase+"/x86_64\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		app, _, _ := testApp(&fakeRunner{}, map[string]string{"RYOKU_HOST_PKGMGR": "xbps"})
		app.cfg.XBPSConfigDir = filepath.Join(dir, "etc")
		app.cfg.XBPSShippedConfig = shipped
		if err := app.RepoSetChannel("stable"); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join(app.cfg.XBPSConfigDir, xbpsRepoConfig))
		if err != nil {
			t.Fatal(err)
		}
		want := "repository=" + base + "/x86_64\n"
		if string(raw) != want {
			t.Fatalf("XBPS stable override = %q, want %q", raw, want)
		}
	})
}

func TestRepoProductionBasesWithoutOverride(t *testing.T) {
	t.Setenv("RYOKU_RELEASE_BASE", "")
	if got := RepoURLFor(Pacman, "stable"); got != PacmanRepoBase+"/$arch" {
		t.Fatalf("Pacman stable URL = %q", got)
	}
	if got := RepoURLFor(XBPS, "stable"); got != XBPSRepoBase+"/x86_64" {
		t.Fatalf("XBPS stable URL = %q", got)
	}
	if got := RepoURLFor(DNF, "stable"); got != DNFRepoBase+"/x86_64" {
		t.Fatalf("DNF stable URL = %q", got)
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

func TestDNFRepoURLRoundTrips(t *testing.T) {
	t.Setenv("RYOKU_RELEASE_BASE", "")
	build := "v0.94.1-beta.20.dev.12+g1234567"
	channels := []string{"stable", "testing", "v1.2.3", build}
	for _, channel := range channels {
		url := RepoURLFor(DNF, channel)
		if got := RepoChannelOf(DNF, url); got != channel {
			t.Errorf("RepoChannelOf(DNF, %q) = %q, want %q", url, got, channel)
		}
		for _, arch := range []string{"x86_64", "$basearch", "$arch"} {
			concrete := strings.Replace(url, "$releasever", "44", 1)
			concrete = strings.TrimSuffix(concrete, "x86_64") + arch
			if got := RepoChannelOf(DNF, concrete); got != channel {
				t.Errorf("RepoChannelOf(DNF, %q) = %q, want %q", concrete, got, channel)
			}
		}
	}
	if got := RepoURLFor(DNF, "stable"); got != DNFRepoBase+"/x86_64" {
		t.Fatalf("DNF stable URL = %q", got)
	}

	t.Setenv("RYOKU_RELEASE_BASE", "http://127.0.0.1:8080/fedora/$releasever/")
	for _, channel := range channels {
		url := RepoURLFor(DNF, channel)
		if !strings.HasPrefix(url, "http://127.0.0.1:8080/fedora/$releasever/") {
			t.Fatalf("override URL = %q", url)
		}
		if got := RepoChannelOf(DNF, url); got != channel {
			t.Errorf("override channel = %q, want %q", got, channel)
		}
	}
}

func TestExpandDNFRepoURL(t *testing.T) {
	osRelease := filepath.Join(t.TempDir(), "os-release")
	if err := os.WriteFile(osRelease, []byte("ID=fedora\nVERSION_ID=\"44\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_OS_RELEASE", osRelease)
	raw := "https://repo.example/fedora/$releasever/channels/testing/$basearch/$arch"
	want := "https://repo.example/fedora/44/channels/testing/x86_64/x86_64"
	if got := ExpandRepoURL(raw); got != want {
		t.Fatalf("ExpandRepoURL = %q, want %q", got, want)
	}
}

func TestDNFRepoConfigRewritesOnlyRyokuBaseURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ryoku.repo")
	old := "[other]\nbaseurl=https://other.invalid/repo\n\n[ryoku]\nname=Ryoku\nbaseurl = https://old.invalid/repo\nenabled=1\ngpgcheck=1\nrepo_gpgcheck=1\ngpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-ryoku\n"
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	app, _, _ := testApp(&fakeRunner{}, map[string]string{
		"RYOKU_HOST_PKGMGR":     "dnf",
		"RYOKU_DNF_REPO_CONFIG": path,
	})
	wantURL := RepoURLFor(DNF, "testing")
	if err := app.RepoSetChannel("testing"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(old, "baseurl = https://old.invalid/repo", "baseurl = "+wantURL, 1)
	if string(got) != want {
		t.Fatalf("rewritten repo file = %q, want %q", got, want)
	}
	if url, err := app.RepoURL(); err != nil || url != wantURL {
		t.Fatalf("RepoURL = %q, %v", url, err)
	}
	if channel, err := app.RepoChannel(); err != nil || channel != "testing" {
		t.Fatalf("RepoChannel = %q, %v", channel, err)
	}

	missing := filepath.Join(t.TempDir(), "missing.repo")
	app, _, _ = testApp(&fakeRunner{}, map[string]string{
		"RYOKU_HOST_PKGMGR":     "dnf",
		"RYOKU_DNF_REPO_CONFIG": missing,
	})
	if err := app.RepoSetURL(wantURL); err != ErrRepoAbsent {
		t.Fatalf("missing RepoSetURL error = %v, want ErrRepoAbsent", err)
	}
}

func TestParseDNFRepoPackagesHandlesRealDNF5AndDNF4Rows(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{
			name:   "dnf5",
			output: "bash\t0:5.3.9-3.fc44\nglibc\t0:2.43-2.fc44\nglibc\t0:2.43-9.fc44\n",
		},
		{
			name:   "dnf4",
			output: "bash\t0:5.1.8-2.el9\n\nbash\t0:5.1.8-9.el9\n\nglibc\t0:2.34-274.el9\n\nglibc\t0:2.34-285.el9\n\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			packages := parseDNFRepoPackages(tc.output)
			if len(packages) != 2 {
				t.Fatalf("packages = %v, want two rows", packages)
			}
			got := []RepoPackage{
				{Name: "bash", Version: packages[0].Version, Repository: packages[0].Repository},
				{Name: "glibc", Version: packages[1].Version, Repository: packages[1].Repository},
			}
			want := []RepoPackage{
				{Name: "bash", Version: map[string]string{"dnf5": "0:5.3.9-3.fc44", "dnf4": "0:5.1.8-9.el9"}[tc.name], Repository: "ryoku"},
				{Name: "glibc", Version: map[string]string{"dnf5": "0:2.43-9.fc44", "dnf4": "0:2.34-285.el9"}[tc.name], Repository: "ryoku"},
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("packages = %v, want %v", got, want)
			}
		})
	}
}

func TestDNFRepoArgvForDNF5AndDNF4(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paths map[string]bool
	}{
		{"dnf5", map[string]bool{"dnf5": true}},
		{"dnf4", map[string]bool{"dnf": true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeRunner{paths: tc.paths, answer: func(command Command) Result {
				if command.Args[0] == "repoquery" {
					output := "ryoku\t0:1.0-1.fc44\n"
					if tc.name == "dnf4" {
						output += "\n"
					}
					return Result{Output: output}
				}
				return Result{}
			}}
			app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "dnf"})
			command := "dnf"
			query := " repoquery -y --repo=ryoku --qf %{name}\t%{epoch}:%{version}-%{release}\n"
			if tc.name == "dnf5" {
				command = "dnf5"
				query = " repoquery -y --available --repo=ryoku --qf %{name}\t%{epoch}:%{version}-%{release}\n"
			}
			if err := app.RepoSync(false); err != nil {
				t.Fatal(err)
			}
			if _, err := app.RepoPackages(); err != nil {
				t.Fatal(err)
			}
			got := []string{argv(runner.commands[0]), argv(runner.commands[1])}
			want := []string{
				command + " makecache -y --refresh --repo=ryoku",
				command + query,
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("repo argv = %v, want %v", got, want)
			}
		})
	}
}

func TestDNFRepoSyncAndInstalledSet(t *testing.T) {
	listing := "up\t0:2.0-1.fc44\ndown\t0:2.0-1.fc44\nequal\t(none):2.0-1.fc44\nabsent\t0:1.0-1.fc44\n"
	runner := &fakeRunner{paths: map[string]bool{"dnf5": true}, answer: func(command Command) Result {
		switch argv(command) {
		case "dnf5 repoquery -y --available --repo=ryoku --qf %{name}\t%{epoch}:%{version}-%{release}\n":
			return Result{Output: listing}
		case "rpm -q --qf %{EPOCHNUM}:%{VERSION}-%{RELEASE}\n up":
			return Result{Output: "0:1.0-1.fc44\n"}
		case "rpm -q --qf %{EPOCHNUM}:%{VERSION}-%{RELEASE}\n down":
			return Result{Output: "0:3.0-1.fc44\n"}
		case "rpm -q --qf %{EPOCHNUM}:%{VERSION}-%{RELEASE}\n equal":
			return Result{Output: "0:2.0-1.fc44\n"}
		case "rpm -q --qf %{EPOCHNUM}:%{VERSION}-%{RELEASE}\n absent":
			return Result{Code: 1}
		default:
			return Result{}
		}
	}}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "dnf"})
	if err := app.RepoSync(false); err != nil {
		t.Fatal(err)
	}
	if got := argv(runner.commands[0]); got != "dnf5 makecache -y --refresh --repo=ryoku" {
		t.Fatalf("RepoSync argv = %q", got)
	}
	runner.commands = nil

	set, skipped, err := app.RepoInstalledSet(false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(set, []string{"equal-0:2.0-1.fc44", "up-0:2.0-1.fc44"}) || skipped != 1 {
		t.Fatalf("ordinary set = %v, skipped = %d", set, skipped)
	}
	for _, command := range runner.commands {
		if strings.Contains(argv(command), "unserved") {
			t.Fatalf("unserved package was queried: %q", argv(command))
		}
	}

	set, skipped, err = app.RepoInstalledSet(true, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"down-0:2.0-1.fc44", "equal-0:2.0-1.fc44", "up-0:2.0-1.fc44"}
	if !reflect.DeepEqual(set, want) || skipped != 0 {
		t.Fatalf("channel-move set = %v, skipped = %d, want %v", set, skipped, want)
	}
}

func TestDNFRepoInstalledSetIncludesUpToDatePackages(t *testing.T) {
	const version = "0:2.0-1.fc44"
	runner := &fakeRunner{paths: map[string]bool{"dnf5": true}, answer: func(command Command) Result {
		switch argv(command) {
		case "dnf5 repoquery -y --available --repo=ryoku --qf %{name}\t%{epoch}:%{version}-%{release}\n":
			return Result{Output: "ryoku\t" + version + "\n"}
		case "rpm -q --qf %{EPOCHNUM}:%{VERSION}-%{RELEASE}\n ryoku":
			return Result{Output: version + "\n"}
		default:
			return Result{}
		}
	}}
	app, _, _ := testApp(runner, map[string]string{"RYOKU_HOST_PKGMGR": "dnf"})
	set, skipped, err := app.RepoInstalledSet(false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(set, []string{"ryoku-" + version}) || skipped != 0 {
		t.Fatalf("up-to-date set = %v, skipped = %d", set, skipped)
	}
}
