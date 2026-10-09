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
type initSystem uint8

const (
	initSystemd initSystem = iota
	initRunit
)

type distro struct {
	id         string
	name       string
	fromSource bool

	// rename maps a base.packages (Arch) name to the local one. A missing key
	// means the name is identical; an empty value means the package does not
	// exist here and is skipped.
	rename map[string]string

	// build and runtime extend base.packages for source builds. build carries
	// the compiler toolchain; runtime carries package dependencies that are not
	// already in the Arch machine manifest.
	build   []string
	runtime []string

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
		"build-essential", "cmake", "ninja-build", "pkgconf", "golang",
		"qt6-base-dev", "qt6-base-private-dev", "qt6-declarative-dev", "qt6-multimedia-dev",
		"qt6-shadertools-dev", "qt6-svg-dev", "qt6-5compat-dev", "qt6-wayland-dev",
		"hyprland-dev", "libhyprutils-dev", "libgtk-3-dev", "libwebkit2gtk-4.1-dev",
	},
	rename: map[string]string{
		"base":                    "",
		"base-devel":              "build-essential",
		"bluez-utils":             "bluez",
		"edk2-ovmf":               "ovmf",
		"fd":                      "fd-find",
		"github-cli":              "gh",
		"gst-libav":               "gstreamer1.0-libav",
		"gst-plugins-bad":         "gstreamer1.0-plugins-bad",
		"gst-plugins-base":        "gstreamer1.0-plugins-base",
		"gst-plugins-good":        "gstreamer1.0-plugins-good",
		"gst-plugins-ugly":        "gstreamer1.0-plugins-ugly",
		"inter-font":              "fonts-inter",
		"linux-firmware":          "firmware-linux-free",
		"linux-headers":           "linux-headers-amd64",
		"networkmanager":          "network-manager",
		"noto-fonts":              "fonts-noto-core",
		"noto-fonts-cjk":          "fonts-noto-cjk",
		"noto-fonts-emoji":        "fonts-noto-color-emoji",
		"polkit":                  "polkitd",
		"python":                  "python3",
		"qemu-desktop":            "qemu-system-x86",
		"qt6-multimedia-ffmpeg":   "qt6-multimedia-dev",
		"rust":                    "rustc",
		"tesseract-data-eng":      "tesseract-ocr-eng",
		"ttf-firacode-nerd":       "fonts-firacode",
		"ttf-hack-nerd":           "fonts-hack",
		"ttf-jetbrains-mono-nerd": "fonts-jetbrains-mono",
		"vulkan-icd-loader":       "libvulkan1",
		"wpa_supplicant":          "wpasupplicant",
		"xorg-xwayland":           "xwayland",

		// Absent from Debian: skipped. matugen means no wallpaper palette,
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
	},
}

// Package names verified against the Void glibc repository. Ryoku is built
// from the payload because there is no signed XBPS repository yet.
var voidLinux = &distro{
	id:         "void",
	name:       "Void",
	fromSource: true,
	installCmd: []string{"xbps-install", "-Sy"},
	removeCmd:  []string{"xbps-remove", "-y"},
	updateCmd:  []string{"xbps-install", "-Syu"},
	refreshCmd: []string{"xbps-install", "-S"},
	queryCmd:   []string{"xbps-query"},
	build: []string{
		"base-devel", "go", "cmake", "ninja", "pkg-config",
		"qt6-base-devel", "qt6-base-private-devel", "qt6-declarative-devel", "qt6-multimedia-devel",
		"qt6-shadertools-devel", "qt6-svg-devel", "qt6-qt5compat-devel",
		"qt6-wayland-devel", "wayland-devel", "wayland-protocols", "ffmpeg6-devel",
		"gtk+3-devel", "libwebkit2gtk41-devel",
	},
	runtime: []string{
		"dbus", "elogind", "turnstile", "polkit",
		"xdg-desktop-portal", "xdg-desktop-portal-gnome", "xdg-desktop-portal-gtk",
		"niri", "xwayland-satellite", "sddm",
		"gtk+3", "libwebkit2gtk41", "qt5-wayland", "qt6-imageformats",
		"syntax-highlighting", "pam-u2f", "wlsunset", "wayland",
		"uv", "socat", "gum", "qrencode",
	},
	rename: map[string]string{
		"base":                          "",
		"blesh":                         "",
		"bluez-utils":                   "",
		// AMD microcode ships in Void's AMD firmware package; Intel's lives
		// in the nonfree repository a stock install does not enable.
		"amd-ucode":                     "linux-firmware-amd",
		"ffmpeg":                        "ffmpeg6",
		"fish":                          "fish-shell",
		"game-devices-udev":             "",
		"gst-plugins-bad":               "gst-plugins-bad1",
		"gst-plugins-base":              "gst-plugins-base1",
		"gst-plugins-good":              "gst-plugins-good1",
		"gst-plugins-ugly":              "gst-plugins-ugly1",
		"hypridle":                      "",
		"imagemagick":                   "ImageMagick",
		"inter-font":                    "font-inter",
		"limine":                        "",
		"limine-mkinitcpio-hook":        "",
		"limine-snapper-sync":           "",
		"mangohud":                      "MangoHud",
		"mesa":                          "mesa-dri",
		"mkinitcpio":                    "",
		"intel-ucode":                   "",
		"networkmanager":                "NetworkManager",
		"noto-fonts":                    "noto-fonts-ttf",
		"otf-space-grotesk":             "",
		"pipewire-alsa":                 "alsa-pipewire",
		"pipewire-audio":                "",
		"pipewire-pulse":                "",
		"python":                        "python3",
		"qemu-desktop":                  "qemu",
		"qt6-5compat":                   "qt6-qt5compat",
		"qt6-multimedia-ffmpeg":         "",
		"ryoku-oh-my-zsh":               "",
		"snap-pac":                      "",
		"tesseract":                     "tesseract-ocr",
		"tesseract-data-eng":            "tesseract-ocr-eng",
		"ttf-firacode-nerd":             "nerd-fonts-ttf",
		"ttf-hack-nerd":                 "nerd-fonts-ttf",
		"ttf-jetbrains-mono-nerd":       "nerd-fonts-ttf",
		"ttf-maple-mono-nf":             "",
		"ttf-material-symbols-variable": "",
		"ttf-readex-pro":                "",
		"ttf-rubik-vf":                  "",
		"vulkan-icd-loader":             "vulkan-loader",
		"vimix-cursors":                 "",
		"waifu2x-ncnn-vulkan":           "",
		"xorg-xwayland":                 "xorg-server-xwayland",
		"xpadneo-dkms":                  "xpadneo",
	},
}

// activeDistro is set once by detectFacts; installed() reads it from the
// detection paths that have no engine to hand.
var activeDistro = archLinux

func detectDistro(id, like string) *distro {
	switch {
	case id == "void":
		return voidLinux
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
	seen := make(map[string]bool, len(pkgs))
	for _, p := range pkgs {
		if l := d.local(p); l != "" && !seen[l] {
			seen[l] = true
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
	cmd := exec.Command(d.queryCmd[0], args...)
	if d.id != "debian" {
		return cmd.Run() == nil
	}
	out, err := cmd.Output()
	return err == nil && strings.Contains(string(out), "install ok installed")
}

func hostInit() initSystem {
	fi, err := os.Stat("/run/systemd/system")
	if err == nil && fi.IsDir() {
		return initSystemd
	}
	return initRunit
}

func supportsInit(d *distro, init initSystem) bool {
	return init == initSystemd || d != nil && d.id == "void" && init == initRunit
}

func supportedInitBooted(d *distro) bool {
	return supportsInit(d, hostInit())
}

func (e *engine) usesRunit() bool {
	return hostInit() == initRunit
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

// ryokuTool finds a Ryoku-built program: /usr/bin from a package, ~/.local/bin
// from a fromSource build (which a fresh login's PATH does not carry). Empty
// when it is not installed yet.
func (e *engine) ryokuTool(name string) string {
	cands := []string{filepath.Join("/usr/bin", name)}
	if e.f != nil && e.f.homeDir != "" {
		cands = append(cands, filepath.Join(e.f.homeDir, ".local", "bin", name))
	}
	for _, c := range cands {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	return ""
}

func (e *engine) ryokuBin() string { return e.ryokuTool("ryoku") }

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
