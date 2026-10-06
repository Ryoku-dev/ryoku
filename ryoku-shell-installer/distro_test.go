package main

import (
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
		{"fedora", "", ""},
		{"void", "", ""},
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

// Ubuntu/Pop!_OS (noble) names: Arch names must map to installable apt names,
// and Hyprland-ecosystem sources must be skipped (built by deploy.sh).
func TestDebianUbuntuNobleRenames(t *testing.T) {
	renamed := map[string]string{
		"go":              "golang-go",
		"python-pip":      "python3-pip",
		"python-pipx":     "pipx",
		"linux-headers":   "linux-headers-generic",
		"intel-ucode":     "intel-microcode",
		"amd-ucode":       "amd64-microcode",
		"mesa":            "mesa-vulkan-drivers",
		"qt5-wayland":     "qtwayland5",
		"qt6-declarative": "qml6-module-qtquick",
		"qt6-5compat":     "libqt6core5compat6",
		"qt6-svg":         "libqt6svg6",
		"qt6-multimedia":  "libqt6multimedia6",
		"tesseract":       "tesseract-ocr",
		"virglrenderer":   "libvirglrenderer1",
		"zbar":            "libzbar0t64",
		"libnotify":       "libnotify-bin",
		"libqalculate":    "libqalculate22t64",
		"docker":          "docker.io",
		"linux-firmware":  "linux-firmware",
	}
	for arch, want := range renamed {
		if got := debianLinux.local(arch); got != want {
			t.Errorf("debian local(%q) = %q, want %q", arch, got, want)
		}
	}
	dropped := []string{
		"hypridle", "hyprpicker", "quickshell", "wl-clip-persist",
		"gpu-screen-recorder", "ttf-maple-mono-nf", "gamescope",
		"xpadneo-dkms", "broadcom-bt-firmware", "blesh", "fastfetch",
		"zsh-history-substring-search", "lazygit", "starship", "openrgb",
		"nautilus-python", "chromium", "mise",
	}
	for _, arch := range dropped {
		if got := debianLinux.local(arch); got != "" {
			t.Errorf("debian local(%q) = %q, want dropped (\"\")", arch, got)
		}
	}
	// build toolchain must not reference Arch-only dev packages.
	for _, b := range debianLinux.build {
		if b == "golang" || b == "hyprland-dev" || b == "libhyprutils-dev" {
			t.Errorf("debian build must not contain Arch-only %q", b)
		}
	}
}

// The Arch step list is the contract that must not drift; the Debian one swaps
// the pacman-only steps for the source build.
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

	deb := strings.Join(ids(&facts{distro: debianLinux}), " ")
	wantDeb := "sysupgrade tools payload backup conflicts packages build session configs shell doctor verify"
	if deb != wantDeb {
		t.Errorf("debian steps = %q, want %q", deb, wantDeb)
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
