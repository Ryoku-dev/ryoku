package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// distro is the only place the installer knows a package manager. Every step
// asks it for argv and for the local name of a package; nothing else branches on
// the distribution.
//
// fromSource distros have no [ryoku] repository, so the desktop is built from the
// cloned payload with ryoku/shell/deploy.sh instead of installed with pacman.
type distro struct {
	id         string
	name       string
	fromSource bool

	// rename maps a base.packages (Arch) name to the local one. A missing key
	// means the name is identical; an empty value means the package does not
	// exist here and is skipped.
	rename map[string]string

	// build are the extra packages a fromSource install needs to compile the
	// Go programs, the Ryoku.Blobs QML plugin, and the Hyprland plugins.
	build []string

	installCmd []string
	removeCmd  []string
	updateCmd  []string
	refreshCmd []string
	queryCmd   []string
}

var archLinux = &distro{
	id:         "arch",
	name:       "Arch",
	installCmd: []string{"pacman", "-Syu", "--needed", "--noconfirm"},
	removeCmd:  []string{"pacman", "-R", "--noconfirm"},
	updateCmd:  []string{"pacman", "-Syu", "--noconfirm"},
	refreshCmd: []string{"pacman", "-Sy"},
	queryCmd:   []string{"pacman", "-Qq"},
}

// Package names verified against api.ftp-master.debian.org (testing/unstable).
var debianLinux = &distro{
	id:         "debian",
	name:       "Debian",
	fromSource: true,
	installCmd: []string{"apt-get", "-y", "install"},
	removeCmd:  []string{"apt-get", "-y", "remove"},
	updateCmd:  []string{"apt-get", "-y", "dist-upgrade"},
	refreshCmd: []string{"apt-get", "update"},
	queryCmd:   []string{"dpkg-query", "-W", "-f=${Status}"},
	build: []string{
		"build-essential", "cmake", "ninja-build", "pkgconf", "golang-go",
		"qt6-base-dev", "qt6-declarative-dev", "qt6-multimedia-dev",
		"qt6-shadertools-dev", "qt6-svg-dev", "qt6-5compat-dev", "qt6-wayland-dev",
	},
	rename: map[string]string{
		"base":                    "",
		"base-devel":              "build-essential",
		"bluez-utils":             "bluez",
		"docker":                  "docker.io",
		"edk2-ovmf":               "ovmf",
		"fd":                      "fd-find",
		"github-cli":              "gh",
		"go":                      "golang-go",
		"gst-libav":               "gstreamer1.0-libav",
		"gst-plugins-bad":         "gstreamer1.0-plugins-bad",
		"gst-plugins-base":        "gstreamer1.0-plugins-base",
		"gst-plugins-good":        "gstreamer1.0-plugins-good",
		"gst-plugins-ugly":        "gstreamer1.0-plugins-ugly",
		"inter-font":              "fonts-inter",
		"libnotify":               "libnotify-bin",
		"libqalculate":            "libqalculate22t64",
		"linux-firmware":          "linux-firmware",
		"linux-headers":           "linux-headers-generic",
		"intel-ucode":             "intel-microcode",
		"amd-ucode":               "amd64-microcode",
		"mesa":                    "mesa-vulkan-drivers",
		"networkmanager":          "network-manager",
		"noto-fonts":              "fonts-noto-core",
		"noto-fonts-cjk":          "fonts-noto-cjk",
		"noto-fonts-emoji":        "fonts-noto-color-emoji",
		"polkit":                  "polkitd",
		"python":                  "python3",
		"python-pip":              "python3-pip",
		"python-pipx":             "pipx",
		"mise":                    "",
		"qemu-desktop":            "qemu-system-x86",
		"qt5-wayland":             "qtwayland5",
		"qt6-declarative":         "qml6-module-qtquick",
		"qt6-5compat":             "libqt6core5compat6",
		"qt6-svg":                 "libqt6svg6",
		"qt6-multimedia":          "libqt6multimedia6",
		"qt6-multimedia-ffmpeg":   "libqt6multimedia6",
		"rust":                    "rustc",
		"tesseract":               "tesseract-ocr",
		"tesseract-data-eng":      "tesseract-ocr-eng",
		"ttf-firacode-nerd":       "fonts-firacode",
		"ttf-hack-nerd":           "fonts-hack",
		"ttf-jetbrains-mono-nerd": "fonts-jetbrains-mono",
		"virglrenderer":           "libvirglrenderer1",
		"vulkan-icd-loader":       "libvulkan1",
		"wpa_supplicant":          "wpasupplicant",
		"xorg-xwayland":           "xwayland",
		"zbar":                    "libzbar0t64",

		// Absent from Debian/Ubuntu: skipped. matugen means no wallpaper palette,
		// the rest are optional tools and cosmetic extras.
		"limine":                        "",
		"limine-mkinitcpio-hook":        "",
		"limine-snapper-sync":           "",
		"mkinitcpio":                    "",
		"snap-pac":                      "",
		"matugen":                       "",
		"otf-space-grotesk":             "",
		"songrec":                       "",
		"ttf-material-symbols-variable": "",
		"vimix-cursors":                 "",
		"waifu2x-ncnn-vulkan":           "",
		"yazi":                          "",
		// Hyprland-ecosystem sources: Hyprland itself is already on this box
		// (~/.local/bin), quickshell/hyprpicker/hypridle/gpu-screen-recorder are
		// built from source by deploy.sh or skipped there.
		"hypridle":                     "",
		"hyprpicker":                   "",
		"quickshell":                   "",
		"wl-clip-persist":              "",
		"gpu-screen-recorder":          "",
		"ttf-maple-mono-nf":            "",
		"gamescope":                    "",
		"xpadneo-dkms":                 "",
		"broadcom-bt-firmware":         "",
		"blesh":                        "",
		"fastfetch":                    "",
		"zsh-history-substring-search": "",
		"lazygit":                      "",
		"starship":                     "",
		"openrgb":                      "",
		"nautilus-python":              "",
		"chromium":                     "",
	},
}

// activeDistro is set once by detectFacts; installed() reads it from the
// detection paths that have no engine to hand.
var activeDistro = archLinux

func detectDistro(id, like string) *distro {
	switch {
	case id == "arch" || strings.Contains(like, "arch"):
		return archLinux
	case id == "debian" || strings.Contains(like, "debian"):
		return debianLinux
	}
	return nil
}

// local returns the package's name on this distro, or "" when it does not exist.
func (d *distro) local(pkg string) string {
	if to, ok := d.rename[pkg]; ok {
		return to
	}
	return pkg
}

// localAll maps a base.packages list, dropping what this distro does not carry.
func (d *distro) localAll(pkgs []string) []string {
	out := make([]string, 0, len(pkgs))
	for _, p := range pkgs {
		if l := d.local(p); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func (d *distro) installArgs(pkgs []string) []string {
	return append(append([]string{}, d.installCmd...), pkgs...)
}

func (d *distro) removeArgs(pkgs []string) []string {
	return append(append([]string{}, d.removeCmd...), pkgs...)
}

func (d *distro) installedPkg(pkg string) bool {
	args := append(append([]string{}, d.queryCmd[1:]...), pkg)
	out, err := exec.Command(d.queryCmd[0], args...).Output()
	if err != nil {
		return false
	}
	if d.id == "debian" {
		return strings.Contains(string(out), "install ok installed")
	}
	return true
}

// installed queries the detected distro. Replaces the old pacman-only helper.
func installed(pkg string) bool { return activeDistro.installedPkg(pkg) }

// d is the engine's detected distro; archLinux until detection says otherwise.
func (e *engine) d() *distro {
	if e.f != nil && e.f.distro != nil {
		return e.f.distro
	}
	return activeDistro
}

// ryokuBin finds the ryoku CLI: /usr/bin from a package, ~/.local/bin from a
// fromSource build. Empty when it is not installed yet.
func (e *engine) ryokuBin() string {
	cands := []string{"/usr/bin/ryoku"}
	if e.f != nil && e.f.homeDir != "" {
		cands = append(cands, filepath.Join(e.f.homeDir, ".local", "bin", "ryoku"))
	}
	for _, c := range cands {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}

// detectHostDistro resolves the distro from /etc/os-release and latches it, so
// the preflight gate and the later detection pass agree.
func detectHostDistro() *distro {
	b, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return nil
	}
	id, like, _ := parseOSRelease(string(b))
	d := detectDistro(id, like)
	if d != nil {
		activeDistro = d
	}
	return d
}
