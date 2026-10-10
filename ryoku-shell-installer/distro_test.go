package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectDistro(t *testing.T) {
	for _, c := range []struct {
		id, like, want string
	}{
		{"arch", "", "arch"},
		{"cachyos", "arch", "arch"},
		{"endeavouros", "arch", "arch"},
		{"debian", "", "debian"},
		{"ubuntu", "debian", "debian"},
		{"linuxmint", "ubuntu debian", "debian"},
		{"void", "", "void"},
		{"fedora", "", ""},
	} {
		d := detectDistro(c.id, c.like)
		got := ""
		if d != nil {
			got = d.id
		}
		if got != c.want {
			t.Errorf("detectDistro(%q,%q) = %q, want %q", c.id, c.like, got, c.want)
		}
	}
}

func TestLocalAllRenamesAndDrops(t *testing.T) {
	in := []string{"git", "networkmanager", "fd", "matugen", "limine", "kitty"}
	got := debianLinux.localAll(in)
	want := []string{"git", "network-manager", "fd-find", "kitty"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("localAll = %v, want %v", got, want)
	}
	if archLinux.local("networkmanager") != "networkmanager" {
		t.Error("arch must pass base.packages names through unchanged")
	}
}

func TestSupportedInit(t *testing.T) {
	if !supportsInit(archLinux, initSystemd) || supportsInit(archLinux, initRunit) {
		t.Fatal("Arch must require systemd")
	}
	if !supportsInit(voidLinux, initSystemd) || !supportsInit(voidLinux, initRunit) {
		t.Fatal("Void must accept the systemd predicate's runit alternative")
	}
}

// The Arch step list is the contract that must not drift. Debian retains its
// source-only font and build steps; packaged Void follows the repository path
// and configures drivers after its package transaction.
func TestStepsPerDistro(t *testing.T) {
	ids := func(f *facts) []string {
		e := newEngine(f, &plan{}, true, "", "")
		var out []string
		for _, s := range e.steps {
			out = append(out, s.id)
		}
		return out
	}

	t.Setenv("RYOKU_HOST_INIT", "systemd")
	arch := strings.Join(ids(&facts{distro: archLinux}), " ")
	wantArch := "legacy sysupgrade tools payload backup repo conflicts packages drivers session configs aur shell doctor verify"
	if arch != wantArch {
		t.Errorf("arch steps = %q, want %q", arch, wantArch)
	}

	debian := strings.Join(ids(&facts{distro: debianLinux}), " ")
	wantDebian := "sysupgrade tools payload backup conflicts packages fonts build session configs shell doctor verify"
	if debian != wantDebian {
		t.Errorf("debian steps = %q, want %q", debian, wantDebian)
	}

	t.Setenv("RYOKU_HOST_INIT", "runit")
	void := strings.Join(ids(&facts{distro: voidLinux}), " ")
	wantVoid := "sysupgrade tools payload backup repo conflicts packages drivers session configs shell doctor verify"
	if void != wantVoid {
		t.Errorf("void steps = %q, want %q", void, wantVoid)
	}
}

func TestInstallArgs(t *testing.T) {
	got := strings.Join(archLinux.installArgs([]string{"git"}), " ")
	if got != "pacman -Syu --needed --noconfirm git" {
		t.Errorf("arch installArgs = %q", got)
	}
	got = strings.Join(debianLinux.installArgs([]string{"git"}), " ")
	if got != "apt-get -y install git" {
		t.Errorf("debian installArgs = %q", got)
	}
	got = strings.Join(debianLinux.removeArgs([]string{"dunst"}), " ")
	if got != "apt-get -y remove dunst" {
		t.Errorf("debian removeArgs = %q", got)
	}
}

func TestVoidInstallArgs(t *testing.T) {
	if got := strings.Join(voidLinux.installArgs([]string{"git"}), " "); got != "xbps-install -Sy git" {
		t.Errorf("Void installArgs = %q", got)
	}
	if got := strings.Join(voidLinux.removeArgs([]string{"dunst"}), " "); got != "xbps-remove -y dunst" {
		t.Errorf("Void removeArgs = %q", got)
	}
}

func TestVoidRepositoryPackagesInstallFirst(t *testing.T) {
	pkgs := []string{
		"mesa", "void-repo-multilib", "libdrm-32bit",
		"void-repo-nonfree", "nvidia-utils",
	}
	phases := voidLinux.installPhases(pkgs)
	if len(phases) != 2 {
		t.Fatalf("install phases = %v, want repository and package transactions", phases)
	}
	if got, want := strings.Join(phases[0], " "), "void-repo-multilib void-repo-nonfree"; got != want {
		t.Fatalf("repository phase = %q, want %q", got, want)
	}
	if got, want := strings.Join(phases[1], " "), "mesa libdrm-32bit nvidia-utils"; got != want {
		t.Fatalf("package phase = %q, want %q", got, want)
	}

	e := &engine{f: &facts{distro: voidLinux}, dry: true, events: make(chan any, 4)}
	if err := installPackagePlan(e, voidLinux, pkgs); err != nil {
		t.Fatal(err)
	}
	close(e.events)
	var commands []string
	for event := range e.events {
		if line, ok := event.(evLine); ok {
			commands = append(commands, line.line)
		}
	}
	wantCommands := []string{
		"DRYRUN: sudo -n xbps-install --repository " + voidStableRepo + " -Sy void-repo-multilib void-repo-nonfree",
		"DRYRUN: sudo -n xbps-install --repository " + voidStableRepo + " -Sy mesa libdrm-32bit nvidia-utils",
	}
	if strings.Join(commands, "\n") != strings.Join(wantCommands, "\n") {
		t.Fatalf("install commands = %v, want %v", commands, wantCommands)
	}
}

func TestVoidInstalledPackageUsesExitStatus(t *testing.T) {
	query := filepath.Join(t.TempDir(), "xbps-query")
	if err := os.WriteFile(query, []byte("#!/bin/sh\n[ \"$1\" = installed ]\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	d := &distro{id: "void", queryCmd: []string{query}}
	if !d.installedPkg("installed") {
		t.Fatal("successful xbps-query with no output must mean installed")
	}
	if d.installedPkg("missing") {
		t.Fatal("failed xbps-query must mean missing")
	}
}

// desktopPacmanArgs must --overwrite the ryoku-desktop-owned paths a prior partial
// install, a dev deploy, or the ISO installer can leave unowned (the bin helpers,
// their polkit rules, and the Plymouth splash theme) so a resume or conversion
// never aborts on "exists in filesystem". Dropping any path silently reintroduces
// that outage, so pin coverage here. fromSource distros build from the payload and
// must never carry --overwrite.
func TestDesktopPacmanArgsAdoptsRyokuPaths(t *testing.T) {
	args := desktopPacmanArgs(archLinux, []string{"ryoku-desktop"})
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--overwrite") {
		t.Fatalf("arch desktop install missing --overwrite: %v", args)
	}
	var glob string
	for i, a := range args {
		if a == "--overwrite" && i+1 < len(args) {
			glob = args[i+1]
		}
	}
	for _, p := range []string{
		"/usr/bin/ryoku-dns",
		"/usr/share/polkit-1/rules.d/50-ryoku-dns.rules",
		"/usr/share/plymouth/themes/ryoku/bullet.png",
	} {
		covered := false
		for _, g := range strings.Split(glob, ",") {
			if ok, _ := filepath.Match(g, p); ok {
				covered = true
				break
			}
		}
		if !covered {
			t.Errorf("--overwrite %q does not cover seeded path %q", glob, p)
		}
	}
	if strings.Contains(strings.Join(desktopPacmanArgs(debianLinux, []string{"foo"}), " "), "--overwrite") {
		t.Error("fromSource distro must not carry --overwrite")
	}
}
