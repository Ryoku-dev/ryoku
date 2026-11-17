package main

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	installerplan "ryoku-installerplan"
)

func TestFedoraRepositoryChannelsAndOverride(t *testing.T) {
	t.Setenv("RYOKU_RPM_REPO", "")
	for _, test := range []struct {
		ref, channel, repository string
	}{
		{"main", "stable", fedoraStableRepo},
		{"unstable-dev", "testing", fedoraTestingRepo},
		{"v1.2.3", "v1.2.3", "https://repo.ryoku.dev/stable/fedora/$releasever/releases/v1.2.3/x86_64"},
	} {
		channel, repository := fedoraRepository(test.ref)
		if channel != test.channel || repository != test.repository {
			t.Errorf("fedoraRepository(%q) = %q, %q; want %q, %q", test.ref, channel, repository, test.channel, test.repository)
		}
		if !strings.Contains(fedoraRepoContent(repository), "baseurl="+test.repository+"\n") {
			t.Errorf("repo content for %q does not carry %q", test.ref, test.repository)
		}
	}

	t.Setenv("RYOKU_RPM_REPO", "/srv/ryoku-rpms")
	channel, repository := fedoraRepository("unstable-dev")
	if channel != "testing" || repository != "/srv/ryoku-rpms" {
		t.Fatalf("override = %q, %q; want testing, /srv/ryoku-rpms", channel, repository)
	}
	if !strings.Contains(fedoraRepoContent(repository), "baseurl=/srv/ryoku-rpms\n") {
		t.Fatal("override was not written into repo content")
	}
}

func TestFedoraRepoContent(t *testing.T) {
	want := "[ryoku]\n" +
		"name=Ryoku\n" +
		"baseurl=" + fedoraTestingRepo + "\n" +
		"enabled=1\n" +
		"gpgcheck=1\n" +
		"repo_gpgcheck=1\n" +
		"gpgkey=file:///etc/pki/rpm-gpg/RPM-GPG-KEY-ryoku\n"
	if got := fedoraRepoContent(fedoraTestingRepo); got != want {
		t.Fatalf("repo content:\n%s\nwant:\n%s", got, want)
	}

	path := filepath.Join(t.TempDir(), "ryoku.repo")
	t.Setenv("RYOKU_DNF_REPO_CONFIG", path)
	t.Setenv("RYOKU_RPM_REPO", "")
	if err := os.WriteFile(path, []byte(fedoraRepoContent(fedoraStableRepo)), 0o644); err != nil {
		t.Fatal(err)
	}
	if !fedoraRepoIsConfigured(&engine{ref: "main"}) {
		t.Fatal("exact stable repo file was not accepted")
	}
	if fedoraRepoIsConfigured(&engine{ref: "unstable-dev"}) {
		t.Fatal("stable repo file was accepted for testing")
	}
}

func TestFedoraCoprContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "coprs.tsv")
	data := "copr\tincludepkgs\twhy\n" +
		"sdegler/hyprland\thyprland hypridle\tFedora lacks the required versions.\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := readFedoraCoprs(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %v", rows)
	}
	want := "[ryoku-copr-sdegler-hyprland]\n" +
		"name=Ryoku dependency: sdegler/hyprland\n" +
		"baseurl=https://download.copr.fedorainfracloud.org/results/sdegler/hyprland/fedora-$releasever-$basearch/\n" +
		"enabled=1\n" +
		"gpgcheck=1\n" +
		"repo_gpgcheck=0\n" +
		"gpgkey=https://download.copr.fedorainfracloud.org/results/sdegler/hyprland/pubkey.gpg\n" +
		"includepkgs=hyprland hypridle\n" +
		"skip_if_unavailable=1\n"
	if got := fedoraCoprContent(rows[0]); got != want {
		t.Fatalf("COPR content:\n%s\nwant:\n%s", got, want)
	}
}

func TestRPMPrimaryFingerprintRejectsWrongOrMultipleKeys(t *testing.T) {
	listing := func(fingerprints ...string) string {
		var b strings.Builder
		for _, fingerprint := range fingerprints {
			b.WriteString("pub:::::::::\n")
			b.WriteString("fpr:::::::::")
			b.WriteString(fingerprint)
			b.WriteString(":\n")
		}
		return b.String()
	}
	if got, err := rpmPrimaryFingerprint(listing(fedoraKeyFingerprint)); err != nil || got != fedoraKeyFingerprint {
		t.Fatalf("valid release key rejected: %q, %v", got, err)
	}
	if _, err := rpmPrimaryFingerprint(listing("0000000000000000000000000000000000000000")); err == nil {
		t.Fatal("wrong fingerprint was accepted")
	}
	if _, err := rpmPrimaryFingerprint(listing(fedoraKeyFingerprint, "1111111111111111111111111111111111111111")); err == nil {
		t.Fatal("two primary keys were accepted")
	}
}

func TestFedoraRepoTrustProbeRequiresPackageOutput(t *testing.T) {
	bin := t.TempDir()
	sudo := "#!/bin/sh\n[ \"$1\" != -n ] || shift\nexec \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "sudo"), []byte(sudo), 0o755); err != nil {
		t.Fatal(err)
	}
	dnfPath := filepath.Join(bin, "dnf5")
	success := "#!/bin/sh\n[ \"$*\" = \"repoquery --repo=ryoku -q ryoku\" ] || exit 64\nprintf '%s\\n' 'ryoku-1.2.3-1.fc44.x86_64'\n"
	if err := os.WriteFile(dnfPath, []byte(success), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	d := newFedoraLinux()
	e := &engine{f: &facts{distro: d}, events: make(chan any, 8)}
	if err := verifyFedoraRepoReadable(e); err != nil {
		t.Fatalf("published package was rejected: %v", err)
	}

	empty := "#!/bin/sh\n[ \"$*\" = \"repoquery --repo=ryoku -q ryoku\" ] || exit 64\n"
	if err := os.WriteFile(dnfPath, []byte(empty), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := verifyFedoraRepoReadable(e); err == nil {
		t.Fatal("an empty successful repoquery was accepted")
	}
}

func TestFedoraPackagePlanUsesTranslationsAndAllRepos(t *testing.T) {
	payload := t.TempDir()
	table := filepath.Join(payload, "fedora/packages/translations.tsv")
	if err := os.MkdirAll(filepath.Dir(table), 0o755); err != nil {
		t.Fatal(err)
	}
	resolver, err := os.ReadFile(filepath.Join("..", "system", "packages", "resolve"))
	if err != nil {
		t.Fatal(err)
	}
	resolverPath := filepath.Join(payload, "system/packages/resolve")
	if err := os.MkdirAll(filepath.Dir(resolverPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resolverPath, resolver, 0o755); err != nil {
		t.Fatal(err)
	}
	data := "arch\tfedora\tlanes\tnotes\n" +
		"chromium\tchromium\tdesktop\t\n" +
		"ryoku\tryoku\tdesktop\tRyoku-owned RPM.\n" +
		"ryoku-desktop\tryoku-desktop\tdesktop\tRyoku-owned RPM.\n" +
		"ryoku-desktop-hyprland\t-\thyprland\tUnavailable in this fixture.\n" +
		"ryoku-desktop-niri\tryoku-desktop-niri\tdesktop\tRyoku-owned RPM.\n" +
		"ryoku-desktop-wayfire\t-\tdesktop\tUnavailable in this fixture.\n" +
		"zsh\tzsh\tdesktop\t\n" +
		"zsh-autosuggestions\tzsh-autosuggestions\tdesktop\t\n" +
		"zsh-history-substring-search\t-\tdesktop\tNot packaged.\n" +
		"zsh-syntax-highlighting\tzsh-syntax-highlighting\tdesktop\t\n" +
		"ryoku-oh-my-zsh\t-\tdesktop\tFedora uses the packaged shell config.\n"
	if err := os.WriteFile(table, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	d := newFedoraLinux()
	e := &engine{
		f:       &facts{distro: d},
		p:       &plan{browser: "chromium", shell: "zsh", compositor: "niri"},
		dry:     true,
		payload: payload,
		events:  make(chan any, 32),
	}
	if err := stepPackages(e); err != nil {
		t.Fatal(err)
	}
	close(e.events)
	var lines []string
	for event := range e.events {
		if line, ok := event.(evLine); ok {
			lines = append(lines, line.line)
		}
	}
	output := strings.Join(lines, "\n")
	wantCommand := "DRYRUN: sudo -n " + d.installCmd[0] + " install -y chromium ryoku ryoku-desktop ryoku-desktop-niri zsh zsh-autosuggestions zsh-syntax-highlighting"
	if !strings.Contains(output, wantCommand) {
		t.Fatalf("Fedora package command missing:\n%s", output)
	}
	if strings.Contains(output, "--repo") {
		t.Fatalf("Fedora install restricted dependency resolution to one repo:\n%s", output)
	}
}

func TestFedoraPackagePlanUsesDesktopDevHardwareAndHyprlandLanes(t *testing.T) {
	e := &engine{
		f:       &facts{distro: newFedoraLinux(), gpus: []string{"amdgpu"}},
		p:       &plan{browser: "firefox", shell: "fish", compositor: "hyprland", devtools: true},
		payload: filepath.Clean(".."),
	}
	packages, err := e.readEditionPackages("fedora")
	if err != nil {
		t.Fatal(err)
	}
	got := make(map[string]bool, len(packages))
	for _, pkg := range packages {
		got[pkg] = true
	}
	for _, want := range []string{"ryoku-desktop", "ryoku-desktop-hyprland", "firefox", "golang", "amd-ucode-firmware", "hyprland"} {
		if !got[want] {
			t.Errorf("resolved Fedora plan is missing %s", want)
		}
	}
	if got["ryoku-desktop-niri"] {
		t.Fatal("Hyprland package plan also installed the niri compositor variant")
	}
	// a compositor bump in the COPR must not make the install unsolvable
	for _, weak := range []string{"hyprglass", "hypr-dynamic-cursors"} {
		if got[weak] {
			t.Errorf("Fedora plan names %s, which the variant only recommends", weak)
		}
	}
}

type httpDoerFunc func(*http.Request) (*http.Response, error)

func (fn httpDoerFunc) Do(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func httpResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestFedoraReleaseGateChecksChannelBeforeChanges(t *testing.T) {
	osRelease := filepath.Join(t.TempDir(), "os-release")
	if err := os.WriteFile(osRelease, []byte("ID=fedora\nVERSION_ID=44\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_RPM_REPO", "")
	var requests []string
	client := httpDoerFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.Method+" "+req.URL.String())
		return httpResponse(http.StatusOK, ""), nil
	})
	if err := fedoraReleaseGate(client, "unstable-dev", osRelease); err != nil {
		t.Fatal(err)
	}
	want := "HEAD https://repo.ryoku.dev/stable/fedora/44/channels/testing/x86_64/repodata/repomd.xml"
	if len(requests) != 1 || requests[0] != want {
		t.Fatalf("release requests = %v, want [%s]", requests, want)
	}
}

func TestFedoraReleaseGateRefusesUnpublishedAndSkipsOverride(t *testing.T) {
	osRelease := filepath.Join(t.TempDir(), "os-release")
	if err := os.WriteFile(osRelease, []byte("ID=fedora\nVERSION_ID=43\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_RPM_REPO", "")
	calls := 0
	client := httpDoerFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if req.URL.String() == fedoraReleasesURL {
			return httpResponse(http.StatusOK, "44\n45\n"), nil
		}
		return httpResponse(http.StatusNotFound, ""), nil
	})
	err := fedoraReleaseGate(client, "main", osRelease)
	want := "Ryoku does not publish packages for Fedora 43 yet. Supported: 44, 45. Upgrade Fedora first with sudo dnf system-upgrade."
	if err == nil || err.Error() != want {
		t.Fatalf("gate error = %v, want %q", err, want)
	}
	if calls != 2 {
		t.Fatalf("release gate made %d requests, want 2", calls)
	}

	t.Setenv("RYOKU_RPM_REPO", "/srv/ryoku")
	calls = 0
	if err := fedoraReleaseGate(client, "main", osRelease); err != nil {
		t.Fatalf("override was gated: %v", err)
	}
	if calls != 0 {
		t.Fatalf("override made %d HTTP requests", calls)
	}
}

func TestFedoraReleaseGateExplainsStableBeforeTheFirstRelease(t *testing.T) {
	osRelease := filepath.Join(t.TempDir(), "os-release")
	if err := os.WriteFile(osRelease, []byte("ID=fedora\nVERSION_ID=44\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_RPM_REPO", "")
	for _, tc := range []struct {
		name      string
		published string
		wantWait  bool
	}{
		{name: "before the first release", wantWait: true},
		{name: "after the first release", published: fedoraReleaseRepoURL("main", "45")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := httpDoerFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.String() {
				case fedoraReleasesURL:
					return httpResponse(http.StatusOK, "44\n45\n"), nil
				case tc.published:
					return httpResponse(http.StatusOK, ""), nil
				}
				return httpResponse(http.StatusNotFound, ""), nil
			})
			err := fedoraReleaseGate(client, "main", osRelease)
			if err == nil {
				t.Fatal("an unpublished release passed the gate")
			}
			if got := strings.Contains(err.Error(), "first Fedora release"); got != tc.wantWait {
				t.Fatalf("stable gate error = %v", err)
			}
		})
	}
}

func TestFedoraReleaseGateFallsBackToGET(t *testing.T) {
	osRelease := filepath.Join(t.TempDir(), "os-release")
	if err := os.WriteFile(osRelease, []byte("ID=fedora\nVERSION_ID=44\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_RPM_REPO", "")
	var methods []string
	client := httpDoerFunc(func(req *http.Request) (*http.Response, error) {
		methods = append(methods, req.Method)
		if req.Method == http.MethodHead {
			return httpResponse(http.StatusMethodNotAllowed, ""), nil
		}
		return httpResponse(http.StatusOK, "repository metadata"), nil
	})
	if err := fedoraReleaseGate(client, "main", osRelease); err != nil {
		t.Fatalf("GET fallback was rejected: %v", err)
	}
	if got := strings.Join(methods, ","); got != "HEAD,GET" {
		t.Fatalf("release probe methods = %q, want HEAD,GET", got)
	}
}

func TestFedoraReleaseGateReportsNetworkFailure(t *testing.T) {
	osRelease := filepath.Join(t.TempDir(), "os-release")
	if err := os.WriteFile(osRelease, []byte("ID=fedora\nVERSION_ID=44\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_RPM_REPO", "")
	client := httpDoerFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("network offline")
	})
	err := fedoraReleaseGate(client, "main", osRelease)
	if err == nil || !strings.Contains(err.Error(), "repository for Fedora 44 could not be reached") {
		t.Fatalf("network gate error = %v", err)
	}
	if strings.Contains(err.Error(), "does not publish packages") {
		t.Fatalf("network failure was reported as an unpublished release: %v", err)
	}
}

func TestFedoraCompositorsComeFromEditionTable(t *testing.T) {
	table := []byte("arch\tfedora\tlanes\tnotes\n" +
		"chromium\tchromium\tdesktop\tFedora package.\n" +
		"firefox\tfirefox\tdesktop\tFedora package.\n" +
		"zen-browser-bin\t-\textra\tNo Fedora package provides this Arch package.\n" +
		"fish\tfish\tdesktop\tFedora package.\n" +
		"zsh\tzsh\tdesktop\tFedora package.\n" +
		"zsh-autosuggestions\tzsh-autosuggestions\tdesktop\tFedora package.\n" +
		"zsh-history-substring-search\tzsh-history-substring-search\tdesktop\tFedora package.\n" +
		"zsh-syntax-highlighting\tzsh-syntax-highlighting\tdesktop\tFedora package.\n" +
		"ryoku-oh-my-zsh\tryoku-oh-my-zsh\tdesktop\tFedora package.\n" +
		"blesh\tblesh\tdesktop\tFedora package.\n" +
		"nautilus\tnautilus\tdesktop\tFedora package.\n" +
		"nautilus-python\tnautilus-python\tdesktop\tFedora package.\n" +
		"dolphin\tdolphin\toptional\tFedora package.\n" +
		"thunar\tthunar\toptional\tFedora package.\n" +
		"ryoku-desktop-hyprland\tryoku-desktop-hyprland\thyprland\tRyoku-owned RPM.\n" +
		"ryoku-desktop-niri\tryoku-desktop-niri\tdesktop\tRyoku-owned RPM.\n" +
		"ryoku-desktop-wayfire\t-\tdesktop\tFedora ships Wayfire 0.10.1; Ryoku requires 0.11.\n")
	plan, err := parseEditionCompositors(table)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(plan.choices.AvailableIDs(installerplan.Compositor), ","); got != "hyprland,niri" {
		t.Fatalf("Fedora compositors = %q, want hyprland,niri", got)
	}
	// None installs nothing, so no edition table can withhold it.
	if got := strings.Join(plan.choices.AvailableIDs(installerplan.Browser), ","); got != "firefox,chromium,none" {
		t.Fatalf("Fedora browsers = %q, want firefox,chromium", got)
	}
	if got := strings.Join(unavailableChoiceNotices(plan, installerplan.Compositor), "\n"); !strings.Contains(got, "Wayfire unavailable") || !strings.Contains(got, "requires 0.11") {
		t.Fatalf("Wayfire omission was not explained: %s", got)
	}
}

func TestFedoraSessionHandlesGDMKDEAndPowerProfiles(t *testing.T) {
	d := newFedoraLinux()
	f := &facts{distro: d, currentDM: "gdm.service", kdeSddmConf: true, username: "ryoku"}
	p := defaultPlan(f, []string{"niri"})
	if p.greeter {
		t.Fatal("KDE-owned SDDM configuration must keep its greeter by default")
	}
	if got := f.otherDM(); got != "gdm.service" {
		t.Fatalf("display-manager.service -> gdm detected as %q", got)
	}

	events := make(chan any, 128)
	e := &engine{f: f, p: p, dry: true, payload: "/payload", events: events}
	if err := stepSession(e); err != nil {
		t.Fatal(err)
	}
	close(events)
	var lines []string
	for event := range events {
		if line, ok := event.(evLine); ok {
			lines = append(lines, line.line)
		}
	}
	output := strings.Join(lines, "\n")
	if !strings.Contains(output, "systemctl disable gdm.service") {
		t.Fatalf("GDM was not disabled before the SDDM setup:\n%s", output)
	}
	if !strings.Contains(output, "rm -f /etc/sddm.conf.d/99-ryoku.conf /etc/sddm.conf.d/zz-ryoku.conf") {
		t.Fatalf("KDE greeter ownership was not preserved:\n%s", output)
	}
	events = make(chan any, 4)
	e.events = events
	if err := enableFedoraPowerProfiles(e, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	close(events)
	if len(events) != 0 {
		t.Fatal("power-profiles-daemon was enabled without an installed unit")
	}

	unitDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(unitDir, "power-profiles-daemon.service"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	events = make(chan any, 4)
	e.events = events
	if err := enableFedoraPowerProfiles(e, unitDir); err != nil {
		t.Fatal(err)
	}
	close(events)
	output = ""
	for event := range events {
		if line, ok := event.(evLine); ok {
			output += line.line + "\n"
		}
	}
	if !strings.Contains(output, "systemctl enable power-profiles-daemon.service") {
		t.Fatalf("installed power-profiles-daemon unit was not enabled:\n%s", output)
	}
}

func TestEnableFedoraBluetooth(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "systemctl"), []byte("#!/bin/sh\nprintf '%s\\n' \"$UNIT_STATE\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "sudo"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	collect := func(events chan any) string {
		close(events)
		var lines []string
		for event := range events {
			if line, ok := event.(evLine); ok {
				lines = append(lines, line.line)
			}
		}
		return strings.Join(lines, "\n")
	}

	t.Run("missing unit is silent", func(t *testing.T) {
		events := make(chan any, 4)
		e := &engine{dry: true, events: events}
		if err := enableFedoraBluetooth(e, t.TempDir()); err != nil {
			t.Fatal(err)
		}
		if output := collect(events); output != "" {
			t.Fatalf("missing bluetooth unit produced output: %s", output)
		}
	})

	unitDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(unitDir, "bluetooth.service"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Run("already enabled is silent", func(t *testing.T) {
		t.Setenv("UNIT_STATE", "enabled")
		events := make(chan any, 4)
		e := &engine{dry: true, events: events}
		if err := enableFedoraBluetooth(e, unitDir); err != nil {
			t.Fatal(err)
		}
		if output := collect(events); output != "" {
			t.Fatalf("enabled bluetooth unit produced output: %s", output)
		}
	})

	t.Run("installed disabled unit is enabled", func(t *testing.T) {
		t.Setenv("UNIT_STATE", "disabled")
		events := make(chan any, 4)
		e := &engine{dry: true, events: events}
		if err := enableFedoraBluetooth(e, unitDir); err != nil {
			t.Fatal(err)
		}
		if output := collect(events); !strings.Contains(output, "systemctl enable bluetooth.service") {
			t.Fatalf("disabled bluetooth unit was not enabled:\n%s", output)
		}
	})

	t.Run("enable records restore", func(t *testing.T) {
		t.Setenv("UNIT_STATE", "disabled")
		restorePath := filepath.Join(t.TempDir(), "restore.sh")
		if err := os.WriteFile(restorePath, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		events := make(chan any, 4)
		e := &engine{events: events, restorePath: restorePath}
		if err := enableFedoraBluetooth(e, unitDir); err != nil {
			t.Fatal(err)
		}
		collect(events)
		restore, err := os.ReadFile(restorePath)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(restore), "sudo systemctl disable bluetooth.service") {
			t.Fatalf("bluetooth restore was not recorded:\n%s", restore)
		}
	})
}

func TestFedoraKeyUntrustCommandFindsTheImportedKey(t *testing.T) {
	fingerprint := strings.ToLower(fedoraKeyFingerprint)
	legacy := "gpg-pubkey-" + fingerprint[len(fingerprint)-8:]
	missing := errors.New("exit status 1")
	for _, test := range []struct {
		name    string
		listed  string
		listErr error
		legacy  bool
		want    []string
	}{
		{name: "fingerprint keystore", listed: "105ef944 Fedora\n" + strings.ToUpper(fingerprint) + " Ryoku Releases <releases@ryoku.dev> public key\n", want: []string{"rpmkeys", "--delete", fingerprint}},
		{name: "gpg-pubkey package", listErr: missing, legacy: true, want: []string{"rpm", "-e", legacy}},
		{name: "other keys only", listed: "105ef944105ef944105ef944105ef944105ef944 Fedora\n", want: nil},
		{name: "nothing trusted", listErr: missing, want: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			probe := func(name string, args ...string) (string, error) {
				switch strings.Join(append([]string{name}, args...), " ") {
				case "rpmkeys --list":
					return test.listed, test.listErr
				case "rpm -q " + legacy:
					if test.legacy {
						return legacy + "-6a0c5a3b\n", nil
					}
					return "", missing
				}
				t.Fatalf("unexpected probe %s %v", name, args)
				return "", nil
			}
			got := fedoraKeyUntrustCommand(probe)
			if strings.Join(got, " ") != strings.Join(test.want, " ") {
				t.Fatalf("untrust command = %q, want %q", got, test.want)
			}
		})
	}
}
