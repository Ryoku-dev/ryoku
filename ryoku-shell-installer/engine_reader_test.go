package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	installerplan "ryoku-installerplan"
)

// pacman progress repaints end in \r, not \n; they must surface live as
// transient events (throttled), while only newline-terminated lines are final.
func TestCmdReaderSplitsCarriageReturns(t *testing.T) {
	e := &engine{events: make(chan any, 64)}
	err := e.cmd("", nil, "sh", "-c", `printf 'dl 1%%\rdl 2%%\rdl done\n'; printf 'crlf line\r\n'`)
	if err != nil {
		t.Fatalf("cmd: %v", err)
	}
	close(e.events)
	var finals, transients []string
	for msg := range e.events {
		if ln, ok := msg.(evLine); ok {
			s := strings.TrimSpace(ln.line)
			if strings.HasPrefix(s, "$") {
				continue // the echoed command line
			}
			if ln.transient {
				transients = append(transients, s)
			} else {
				finals = append(finals, s)
			}
		}
	}
	if len(transients) != 1 || transients[0] != "dl 1%" {
		t.Errorf("transients = %v, want the first repaint only (throttle eats the second)", transients)
	}
	if len(finals) != 2 || finals[0] != "dl done" || finals[1] != "crlf line" {
		t.Errorf("finals = %v, want [dl done, crlf line] (\\r\\n is one line ending)", finals)
	}
}

// a shell-converted box keeps its own bootloader + initramfs, so the limine
// hook packages -- which pull limine + mkinitcpio back as depends and ship
// pacman hooks that collide with a host's own (Garuda's garuda-hooks, #58) --
// must be filtered out of the install set alongside limine/mkinitcpio/snapper.
func TestReadBasePackagesSkipsBootChain(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "system/packages"), 0o755); err != nil {
		t.Fatal(err)
	}
	base := "# a comment\nlimine\nlimine-mkinitcpio-hook\nlimine-snapper-sync\nmkinitcpio\nsnapper\nkitty\nryoku-desktop\n"
	if err := os.WriteFile(filepath.Join(dir, "system/packages/base.packages"), []byte(base), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := (&engine{payload: dir}).readBasePackages()
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	for _, p := range got {
		set[p] = true
	}
	for _, skip := range []string{"limine", "limine-mkinitcpio-hook", "limine-snapper-sync", "mkinitcpio", "snapper"} {
		if set[skip] {
			t.Errorf("%s must be skipped on a shell-converted box, got %v", skip, got)
		}
	}
	for _, keep := range []string{"kitty", "ryoku-desktop"} {
		if !set[keep] {
			t.Errorf("%s must install, missing from %v", keep, got)
		}
	}
}

func TestVoidPackagePlanMatchesResolver(t *testing.T) {
	root := filepath.Clean("..")
	p := &plan{browser: "chromium", shell: "zsh", compositor: "niri", devtools: true}
	f := &facts{distro: voidLinux, hasNvidia: true}
	e := &engine{f: f, p: p, payload: root}

	got, err := e.readEditionPackages("void")
	if err != nil {
		t.Fatal(err)
	}
	resolver := filepath.Join(root, "system", "packages", "resolve")
	args := []string{
		resolver,
		"--table", filepath.Join(root, "void", "packages", "translations.tsv"),
		"--lane", "desktop",
		"--lane", "dev",
		"--lane", "hardware:amd",
		"--lane", "hardware:intel",
		"--lane", "hardware:nvidia",
		"--session",
		"--drop", "ryoku-desktop-hyprland",
		"--drop", "ryoku-desktop-wayfire",
		"--drop", "firefox",
		"--drop", "zen-browser-bin",
		"--drop", "fish",
		"--drop", "blesh",
	}
	out, err := exec.Command("sh", args...).Output()
	if err != nil {
		t.Fatalf("resolver: %v", err)
	}
	want := strings.Fields(string(out))
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("Void package plan:\n%v\nwant resolver output:\n%v", got, want)
	}
}

func TestVoidPackagedTransactionsContainMetaPackagesWithoutBuildSet(t *testing.T) {
	t.Setenv("RYOKU_XBPS_REPO", "https://repo.test/void")
	e := &engine{
		f:       &facts{distro: voidLinux, hasNvidia: true},
		p:       &plan{browser: "firefox", shell: "fish", compositor: "niri"},
		payload: filepath.Clean(".."),
		dry:     true,
		events:  make(chan any, 16),
	}
	if err := stepPackages(e); err != nil {
		t.Fatal(err)
	}
	close(e.events)
	var commands []string
	for event := range e.events {
		line, ok := event.(evLine)
		if ok && strings.Contains(line.line, "xbps-install") {
			commands = append(commands, line.line)
		}
	}
	if len(commands) != 2 {
		t.Fatalf("xbps transactions = %v, want one enabler and one package transaction", commands)
	}
	if !strings.Contains(commands[0], "void-repo-") {
		t.Fatalf("first transaction does not enable a Void repository: %q", commands[0])
	}
	for _, pkg := range []string{"ryoku-keyring", "ryoku-desktop", "ryoku-desktop-niri"} {
		if !strings.Contains(" "+commands[1]+" ", " "+pkg+" ") {
			t.Errorf("package transaction missing %s: %q", pkg, commands[1])
		}
	}
	for _, buildPkg := range []string{" go ", " cmake "} {
		if strings.Contains(" "+commands[1]+" ", buildPkg) {
			t.Errorf("package transaction contains source-build package %q: %q", strings.TrimSpace(buildPkg), commands[1])
		}
	}
}

func TestVoidRepoSeedsKeyBeforeConfigAndSync(t *testing.T) {
	keyDir := t.TempDir()
	plist := filepath.Join(keyDir, "test-key.plist")
	if err := os.WriteFile(plist, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RYOKU_XBPS_KEYRING_DIR", keyDir)
	t.Setenv("RYOKU_XBPS_REPO", "https://repo.test/void")
	e := &engine{
		f:      &facts{distro: voidLinux},
		ref:    "unstable-dev",
		dry:    true,
		events: make(chan any, 16),
	}
	if err := stepRepo(e); err != nil {
		t.Fatal(err)
	}
	close(e.events)
	var lines []string
	for event := range e.events {
		if line, ok := event.(evLine); ok {
			lines = append(lines, line.line)
		}
	}
	joined := strings.Join(lines, "\n")
	keyAt := strings.Index(joined, "install -Dm644 "+plist)
	configAt := strings.Index(joined, "write "+voidRepoConfig)
	syncAt := strings.LastIndex(joined, "xbps-install -S")
	if keyAt < 0 || configAt < 0 || syncAt < 0 || !(keyAt < configAt && configAt < syncAt) {
		t.Fatalf("repo plan must seed key, write config, then sync:\n%s", joined)
	}
}

func TestVoidHardwareLanesMirrorArchProfiles(t *testing.T) {
	for _, tc := range []struct {
		name string
		f    *facts
		want string
	}{
		{name: "amd", f: &facts{gpus: []string{"amdgpu"}}, want: "amd"},
		{name: "intel", f: &facts{gpus: []string{"i915"}}, want: "intel"},
		{name: "nvidia", f: &facts{hasNvidia: true}, want: "amd intel nvidia"},
		{name: "vm", f: &facts{gpus: []string{"virtio_gpu"}}, want: "vm"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := strings.Join(hardwareLanes(tc.f), " "); got != tc.want {
				t.Fatalf("hardware lanes = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPayloadLocationFollowsDeliveryModel(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", "")

	arch := &engine{f: &facts{homeDir: home, distro: archLinux}}
	arch.resolvePayload()
	if want := filepath.Join(home, ".cache", "ryoku-shell-install", "repo"); arch.payload != want {
		t.Fatalf("Arch payload = %q, want %q", arch.payload, want)
	}

	void := &engine{f: &facts{homeDir: home, distro: voidLinux}}
	void.resolvePayload()
	if void.payload != arch.payload {
		t.Fatalf("Void payload = %q, want packaged cache %q", void.payload, arch.payload)
	}

	source := &engine{f: &facts{homeDir: home, distro: debianLinux}}
	source.resolvePayload()
	if want := filepath.Join(home, "ryoku-arch"); source.payload != want {
		t.Fatalf("source payload = %q, want durable checkout %q", source.payload, want)
	}
}

func TestSourcePayloadCloneIsCompleteAndRefreshable(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	logPath := filepath.Join(root, "git.log")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	git := filepath.Join(bin, "git")
	if err := os.WriteFile(git, []byte("#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$GIT_LOG\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GIT_LOG", logPath)

	home := filepath.Join(root, "home")
	e := &engine{
		f:   &facts{homeDir: home, distro: debianLinux},
		p:   &plan{},
		ref: "main",
	}
	e.resolvePayload()
	if err := stepPayload(e); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	clone := string(first)
	if !strings.Contains(clone, "clone --filter=blob:none --branch main "+repoURL+" "+e.payload) {
		t.Fatalf("source clone command = %q", clone)
	}
	if strings.Contains(clone, "--depth") || strings.Contains(clone, "--sparse") || strings.Contains(clone, "sparse-checkout") {
		t.Fatalf("source clone must carry complete history and paths: %q", clone)
	}

	if err := os.MkdirAll(filepath.Join(e.payload, ".git", "info"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.payload, ".git", "info", "sparse-checkout"), []byte("/*\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := stepPayload(e); err != nil {
		t.Fatal(err)
	}
	refresh, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	refreshed := string(refresh)
	for _, command := range []string{
		"remote set-url origin " + repoURL,
		"fetch --prune origin main",
		"sparse-checkout disable",
		"checkout -f FETCH_HEAD",
	} {
		if !strings.Contains(refreshed, command) {
			t.Errorf("source refresh missing %q in %q", command, refreshed)
		}
	}
	if strings.Contains(refreshed, "--depth") || strings.Contains(refreshed, "sparse-checkout set") {
		t.Fatalf("source refresh must retain complete history and paths: %q", refreshed)
	}
}

func TestRunitSessionEnsureAndWrapperRepairOrder(t *testing.T) {
	t.Setenv("RYOKU_HOST_INIT", "runit")
	root := t.TempDir()
	oldSystemBin := ryokuSystemBin
	ryokuSystemBin = filepath.Join(root, "usr-bin")
	t.Cleanup(func() { ryokuSystemBin = oldSystemBin })
	home := filepath.Join(root, "home")
	binDir := filepath.Join(home, ".local", "bin")
	backupDir := filepath.Join(root, "backup")
	for _, dir := range []string{binDir, backupDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	logPath := filepath.Join(root, "host.log")
	host := filepath.Join(binDir, "ryoku-host")
	hostScript := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$HOST_LOG\"\n"
	if err := os.WriteFile(host, []byte(hostScript), 0o755); err != nil {
		t.Fatal(err)
	}
	sudo := filepath.Join(binDir, "sudo")
	sudoScript := "#!/bin/sh\n[ \"$1\" != -n ] || shift\nexec \"$@\"\n"
	if err := os.WriteFile(sudo, []byte(sudoScript), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("HOST_LOG", logPath)
	restorePath := filepath.Join(backupDir, "restore.sh")
	if err := os.WriteFile(restorePath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	e := &engine{
		f:           &facts{homeDir: home, distro: voidLinux},
		p:           &plan{},
		backupDir:   backupDir,
		restorePath: restorePath,
	}
	if err := ensureRunitSession(e); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	wantBackup := filepath.Join(backupDir, "session-wrappers")
	want := "session ensure --system\nsession ensure --user\nsession fix-wrappers --backup-dir " + wantBackup
	if strings.TrimSpace(string(got)) != want {
		t.Fatalf("ryoku-host calls = %q, want %q", strings.TrimSpace(string(got)), want)
	}
	restore, err := os.ReadFile(restorePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(restore), `cp -a "$DIR/session-wrappers/." /`) {
		t.Fatalf("restore.sh does not restore session wrappers:\n%s", restore)
	}
}

func TestRunitSessionCheckExitContract(t *testing.T) {
	root := t.TempDir()
	host := filepath.Join(root, "ryoku-host")
	script := `#!/bin/sh
case "$CHECK_MODE" in
  ok) exit 0 ;;
  findings)
    printf '%s\n' 'turnstile: missing -> enable turnstiled' 'dbus: wrapped -> remove dbus-run-session'
    exit 3
    ;;
  *) printf '%s\n' 'unexpected failure' >&2; exit 4 ;;
esac
`
	if err := os.WriteFile(host, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CHECK_MODE", "ok")
	if findings, err := runRunitSessionCheck(host); err != nil || len(findings) != 0 {
		t.Fatalf("healthy session check = %v, %v", findings, err)
	}
	t.Setenv("CHECK_MODE", "findings")
	findings, err := runRunitSessionCheck(host)
	if err != nil {
		t.Fatal(err)
	}
	want := "turnstile: missing -> enable turnstiled\ndbus: wrapped -> remove dbus-run-session"
	if strings.Join(findings, "\n") != want {
		t.Fatalf("findings = %q, want %q", strings.Join(findings, "\n"), want)
	}
	t.Setenv("CHECK_MODE", "error")
	if _, err := runRunitSessionCheck(host); err == nil {
		t.Fatal("unexpected session-check failure must not look healthy")
	}
}

func TestVoidDefaultsToFirstPackagedProvider(t *testing.T) {
	data, err := os.ReadFile("../void/packages/translations.tsv")
	if err != nil {
		t.Fatal(err)
	}
	edition, err := parseEditionCompositors(data)
	if err != nil {
		t.Fatal(err)
	}
	available := edition.choices.AvailableIDs(installerplan.Compositor)
	p := defaultPlan(&facts{distro: voidLinux}, available)
	if p.compositor != available[0] || p.compositor != "niri" {
		t.Fatalf("Void provider = %q, available = %v", p.compositor, available)
	}
}

func TestAvailableKernelHeaderPackagesMatchInstalledKernels(t *testing.T) {
	root := t.TempDir()
	for version, pkgbase := range map[string]string{
		"7.2.11-arch1-1":         "linux",
		"7.3.0-cachyos-bore-lto": "linux-cachyos-bore-lto",
		"7.3.0-custom":           "linux-custom",
		"stale":                  "linux-zen",
	} {
		dir := filepath.Join(root, version)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "pkgbase"), []byte(pkgbase+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	installed := map[string]bool{
		"linux": true, "linux-cachyos-bore-lto": true, "linux-custom": true,
	}
	available := map[string]bool{
		"linux-headers": true, "linux-cachyos-bore-lto-headers": true,
	}
	got, err := availableKernelHeaderPackages(root,
		func(name string) bool { return installed[name] },
		func(name string) bool { return available[name] })
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"linux-cachyos-bore-lto-headers", "linux-headers"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("headers = %v, want %v", got, want)
	}
}

func TestEditionPackagePlansIncludeSelectedFileManagerAppsAndRadio(t *testing.T) {
	root := filepath.Clean("..")
	for _, editionName := range []string{"void", "fedora"} {
		t.Run(editionName, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, editionName, "packages", "translations.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			edition, err := parseEditionCompositors(data)
			if err != nil {
				t.Fatal(err)
			}
			distro := voidLinux
			if editionName == "fedora" {
				distro = newFedoraLinux()
			}
			p := defaultPlanForEdition(&facts{distro: distro}, edition)
			p.fileManager = "dolphin"
			e := &engine{f: &facts{distro: distro}, p: p, payload: root}
			packages, err := e.readEditionPackages(editionName)
			if err != nil {
				t.Fatal(err)
			}
			set := packageSet(packages)
			for _, pkg := range []string{"dolphin", "ryotunes", "ryomanager", "ryoku-rashin", "mpv", "mpv-mpris"} {
				if !set[pkg] {
					t.Errorf("selected package %s is missing from %v", pkg, packages)
				}
			}
			for _, pkg := range []string{"nautilus", "nautilus-python", "thunar"} {
				if set[pkg] {
					t.Errorf("losing file manager package %s is present", pkg)
				}
			}
		})
	}
}
