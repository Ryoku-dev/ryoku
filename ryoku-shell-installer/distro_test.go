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
	} {
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

func TestVoidPackageRenamesAndChoiceFiltering(t *testing.T) {
	base := []string{
		"firefox", "chromium", "fish", "zsh", "blesh",
		"networkmanager", "imagemagick", "ffmpeg", "ttf-hack-nerd",
		"ttf-jetbrains-mono-nerd", "vimix-cursors",
	}
	filtered := filterChoicePackages(base, &plan{browser: "firefox", shell: "fish"})
	got := voidLinux.localAll(filtered)
	want := []string{"firefox", "fish-shell", "NetworkManager", "ImageMagick", "ffmpeg6", "nerd-fonts-ttf"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("Void filtered package set = %v, want %v", got, want)
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

// The Arch step list is the contract that must not drift; source builds replace
// repository-only steps and add their pinned font installation.
func TestStepsPerDistro(t *testing.T) {
	ids := func(f *facts) []string {
		e := newEngine(f, &plan{}, true, "", "")
		var out []string
		for _, s := range e.steps {
			out = append(out, s.id)
		}
		return out
	}

	arch := strings.Join(ids(&facts{distro: archLinux}), " ")
	wantArch := "legacy sysupgrade tools payload backup repo conflicts packages drivers session configs aur shell doctor verify"
	if arch != wantArch {
		t.Errorf("arch steps = %q, want %q", arch, wantArch)
	}

	wantSource := "sysupgrade tools payload backup conflicts packages fonts build session configs shell doctor verify"
	for _, d := range []*distro{debianLinux, voidLinux} {
		got := strings.Join(ids(&facts{distro: d}), " ")
		if got != wantSource {
			t.Errorf("%s steps = %q, want %q", d.id, got, wantSource)
		}
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
