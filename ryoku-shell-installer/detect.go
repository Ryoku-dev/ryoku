package main

// detect.go is the only place that inspects the machine. Everything the plan
// and the engine decide on comes from one facts struct filled in here, so a
// dry run and a real run see the same world.

import (
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"ryoku-i18n"
)

type facts struct {
	distroID   string
	distroLike string
	distroName string
	archX86    bool
	pacman     bool
	archLike   bool
	distro     *distro
	online     bool
	btrfsRoot  bool

	secureBoot  bool // SecureBoot=1 and SetupMode=0 in efivars
	sbctlSigned bool // sbctl on PATH with a key store, signing hook likely works

	gpus        []string
	hasNvidia   bool
	nouveauLive bool
	ucodePkg    string

	currentDM string   // enabled display-manager unit ("" = none)
	otherNet  []string // enabled network stacks other than NetworkManager
	nmEnabled bool

	aurHelper        string
	rivalPkgs        []string // conflicting shell packages installed
	blockerPkgs      []string // packages that abort the pacman transaction if left
	zshFrameworkPkgs []string // upstream oh-my-zsh installs ryoku-oh-my-zsh replaces
	omarchyRepo      bool     // [omarchy] stanza in pacman.conf
	omarchyMirror    bool     // mirrorlist pinned to Omarchy's package mirror
	omarchyGuards    []string // Omarchy alpm hooks that abort any direct -Syu
	softUnits        []string // enabled user units that fight the shell
	niriFound        bool
	swayFound        bool         // sway config present; sway stays as a fallback session
	desktops         []string     // installed desktop environments (kept, never removed)
	kdeSddmConf      bool         // KDE's sddm-kcm drop-in owns /etc/sddm.conf.d
	monOutputs       []niriOutput // monitor intent salvaged from the old setup
	monSource        string       // config the outputs came from (niri, GNOME, KDE, ...)
	kbLayout         string
	kbVariant        string
	kbOptions        string
	kbSource         string // config the keyboard setup came from
	userShell        string
	ryokuOnBox       bool // ryoku-desktop already installed
	hostname         string
	username         string
	homeDir          string
	hyprCfgDirs      []string  // pre-existing ~/.config/{hypr,quickshell,niri}
	riceFound        []string  // known hyprland rices detected by marker paths
	prevRun          *runState // interrupted earlier run, offered as a resume
}

// rival quickshell stacks; ordered meta -> shell -> runtime so pacman removal
// never trips its own dependency checks (iNiR's conflicts model).
var rivalShellPkgs = []string{
	"cachyos-niri-noctalia", "noctalia-shell", "noctalia-shell-git", "noctalia-qs",
	"dms-shell", "dms-shell-git", "dms-shell-greeter",
	"caelestia-shell", "caelestia-shell-git",
	"inir-shell", "inir-shell-git", "bms-shell-bin",
}

// installed packages that make `pacman -S --noconfirm` of the desktop set
// abort with an unresolved conflict (pacman answers conflict prompts with No).
var conflictBlockerPkgs = []string{
	"pulseaudio", "pulseaudio-alsa", "pulseaudio-bluetooth",
	"quickshell-git", "quickshell-bin",
	// ship the same files as ryoku-cursors; left in place they file-conflict
	// the whole desktop transaction to death
	"bibata-cursor-theme", "bibata-cursor-theme-bin",
}

// user daemons the Ryoku shell replaces (notifications, bar, wallpaper, idle,
// rival shell services). disabled at install, never uninstalled.
var softConflictUnits = []string{
	"dunst.service", "mako.service", "swaync.service", "fnott.service",
	"waybar.service", "polybar.service", "eww.service", "ironbar.service",
	"swww.service", "swww-daemon.service", "hyprpaper.service", "wpaperd.service",
	"swayidle.service", "swayosd.service",
	"noctalia.service", "dms.service", "inir.service", "caelestia.service",
	// HyDE ships real user units in ~/.config/systemd/user
	"hyde-config.service", "hyde-ipc.service",
	// graphical-session.target units follow the user into the new session:
	// clipboard watchers fight the stash, gamma daemons fight hyprsunset
	"cliphist.service", "wl-clip-persist.service", "clipse.service",
	"wlsunset.service", "gammastep.service",
}

// known hyprland rices, one marker path under $HOME suffices. the backup of
// ~/.config/{hypr,quickshell} already kills their exec-once autostart chain;
// HyDE's user units and end-4's meta packages are handled separately.
var riceMarkers = []struct {
	name    string
	markers []string
}{
	{"ML4W", []string{".config/ml4w", ".config/ml4w-dotfiles-settings", ".mydotfiles"}},
	{"HyDE", []string{".config/hyde", ".local/lib/hyde", ".local/share/hyde"}},
	{"JaKooLit", []string{".config/hypr/UserConfigs", ".config/hypr/.initial_startup_done", ".config/hypr/UserScripts"}},
	{"end-4 illogical-impulse", []string{".config/quickshell/ii", ".config/hypr/hyprland"}},
	{"Caelestia", []string{".config/caelestia"}},
}

var otherDMUnits = []string{
	"gdm.service", "gdm3.service", "lightdm.service", "lxdm.service",
	"greetd.service", "ly.service", "cosmic-greeter.service", "xdm.service",
}

var otherNetUnits = []string{
	"systemd-networkd.service", "dhcpcd.service", "connman.service", "netctl.service",
}

func out(name string, args ...string) string {
	b, err := exec.Command(name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func has(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func pacmanHas(pkg string) bool { return installed(pkg) }

func unitEnabled(scope, unit string) bool {
	args := []string{"is-enabled", unit}
	if scope == "user" {
		args = append([]string{"--user"}, args...)
	}
	return out("systemctl", args...) == "enabled"
}

func parseOSRelease(text string) (id, like, name string) {
	for _, ln := range strings.Split(text, "\n") {
		k, v, ok := strings.Cut(ln, "=")
		if !ok {
			continue
		}
		v = strings.Trim(v, `"`)
		switch k {
		case "ID":
			id = v
		case "ID_LIKE":
			like = v
		case "PRETTY_NAME":
			name = v
		}
	}
	return
}

var omarchyStanzaRe = regexp.MustCompile(`(?m)^\[omarchy\]`)

// true when an active Server line routes packages through Omarchy's mirror.
func mirrorlistHasOmarchy(list string) bool {
	for _, ln := range strings.Split(list, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "Server") && strings.Contains(t, "omarchy") {
			return true
		}
	}
	return false
}

// Omarchy 4 ships an alpm hook that aborts every `pacman -Syu` unless
// OMARCHY_ALLOW_DIRECT_PACMAN is set, which stops this installer dead and would
// keep blocking `ryoku update` after the conversion. Both hook directories are
// searched: one left behind is one that still blocks.
func findOmarchyGuards() []string {
	var hooks []string
	for _, dir := range []string{"/usr/share/libalpm/hooks", "/etc/pacman.d/hooks"} {
		hits, _ := filepath.Glob(filepath.Join(dir, "*omarchy*guard*.hook"))
		hooks = append(hooks, hits...)
	}
	return hooks
}

var x11LayoutRe = regexp.MustCompile(`X11 Layout:\s*(\S+)`)

func detect() *facts {
	f := &facts{}

	if b, err := os.ReadFile("/etc/os-release"); err == nil {
		f.distroID, f.distroLike, f.distroName = parseOSRelease(string(b))
	}
	f.archX86 = out("uname", "-m") == "x86_64"
	f.pacman = has("pacman")
	f.archLike = f.distroID == "arch" || strings.Contains(f.distroLike, "arch")
	if d := detectDistro(f.distroID, f.distroLike); d != nil {
		activeDistro = d
		f.distro = d
	}

	u, err := user.Current()
	if err == nil {
		f.username = u.Username
		f.homeDir = u.HomeDir
	}
	if f.homeDir == "" {
		f.homeDir, _ = os.UserHomeDir()
	}
	f.hostname, _ = os.Hostname()
	f.userShell = os.Getenv("SHELL")

	// repo reachability probe; the install needs the network anyway.
	client := &http.Client{Timeout: 6 * time.Second}
	if resp, err := client.Head("https://repo.ryoku.dev/stable/x86_64/ryoku.db"); err == nil {
		resp.Body.Close()
		f.online = resp.StatusCode < 500
	}

	var st syscall.Statfs_t
	if syscall.Statfs("/", &st) == nil {
		f.btrfsRoot = st.Type == 0x9123683e
	}

	f.detectGPUs()
	f.detectUcode()
	f.detectSecureBoot()

	// enabled display manager: the display-manager.service alias symlink is
	// authoritative; fall back to probing the known units.
	if tgt, err := os.Readlink("/etc/systemd/system/display-manager.service"); err == nil {
		f.currentDM = filepath.Base(tgt)
	} else {
		for _, dm := range append([]string{"sddm.service"}, otherDMUnits...) {
			if unitEnabled("system", dm) {
				f.currentDM = dm
				break
			}
		}
	}

	f.nmEnabled = unitEnabled("system", "NetworkManager.service")
	for _, n := range otherNetUnits {
		if unitEnabled("system", n) {
			f.otherNet = append(f.otherNet, n)
		}
	}

	for _, h := range []string{"yay", "paru"} {
		if has(h) {
			f.aurHelper = h
			break
		}
	}

	if f.pacman {
		for _, p := range rivalShellPkgs {
			if pacmanHas(p) {
				f.rivalPkgs = append(f.rivalPkgs, p)
			}
		}
		// end-4's meta packages pull the whole illogical-impulse daemon zoo;
		// plain -R removal, same reasoning as the noctalia metas.
		for _, p := range strings.Fields(out("pacman", "-Qq")) {
			if strings.HasPrefix(p, "illogical-impulse-") {
				f.rivalPkgs = append(f.rivalPkgs, p)
			}
		}
		for _, p := range conflictBlockerPkgs {
			if pacmanHas(p) {
				f.blockerPkgs = append(f.blockerPkgs, p)
			}
		}
		for _, p := range []string{"oh-my-zsh", "oh-my-zsh-git"} {
			if pacmanHas(p) {
				f.zshFrameworkPkgs = append(f.zshFrameworkPkgs, p)
			}
		}
		f.ryokuOnBox = pacmanHas("ryoku-desktop")
		f.niriFound = pacmanHas("niri")
		f.desktops = detectDesktops()
	}
	if _, err := os.Stat("/etc/sddm.conf.d/kde_settings.conf"); err == nil {
		f.kdeSddmConf = true
	}

	// an ex-Omarchy box keeps the [omarchy] repo and, worse, Omarchy's own
	// mirror for core/extra; a converted machine must depend on neither.
	if b, err := os.ReadFile("/etc/pacman.conf"); err == nil {
		f.omarchyRepo = omarchyStanzaRe.Match(b)
	}
	if b, err := os.ReadFile("/etc/pacman.d/mirrorlist"); err == nil {
		f.omarchyMirror = mirrorlistHasOmarchy(string(b))
	}
	f.omarchyGuards = findOmarchyGuards()

	for _, unit := range softConflictUnits {
		if unitEnabled("user", unit) {
			f.softUnits = append(f.softUnits, unit)
		}
	}

	cfg := filepath.Join(f.homeDir, ".config")
	for _, d := range []string{"hypr", "quickshell", "niri"} {
		if _, err := os.Stat(filepath.Join(cfg, d)); err == nil {
			f.hyprCfgDirs = append(f.hyprCfgDirs, d)
			if d == "niri" {
				f.niriFound = true
			}
		}
	}

	for _, r := range riceMarkers {
		for _, m := range r.markers {
			if _, err := os.Stat(filepath.Join(f.homeDir, m)); err == nil {
				f.riceFound = append(f.riceFound, r.name)
				break
			}
		}
	}

	// a pre-existing hyprland setup (plain or rice): its config tree gets
	// moved aside by the backup, so its monitor and keyboard intent is
	// salvaged here first. skipped on a ryoku box, that tree is our own.
	if !f.ryokuOnBox {
		if text := loadHyprConfig(f.homeDir); text != "" {
			layout, variant, options, hasFile := parseHyprInput(text)
			if !hasFile && (layout != "" || variant != "" || options != "") {
				f.kbLayout, f.kbVariant, f.kbOptions, f.kbSource = layout, variant, options, "hyprland"
			}
			if outs := parseHyprMonitors(text); len(outs) > 0 {
				f.monOutputs, f.monSource = outs, "hyprland"
			}
		}
	}

	// keyboard + monitor intent, best source first: compositor configs beat
	// DE stores beat localectl. an xkb keymap file overrides fields, nothing
	// to salvage then.
	if niriText := loadNiriConfig(f.homeDir); niriText != "" {
		layout, variant, options, hasFile := parseNiriXkb(niriText)
		if f.kbLayout == "" && !hasFile && (layout != "" || variant != "" || options != "") {
			f.kbLayout, f.kbVariant, f.kbOptions, f.kbSource = layout, variant, options, "niri"
		}
		if outs := parseNiriOutputs(niriText); len(outs) > 0 && len(f.monOutputs) == 0 {
			f.monOutputs, f.monSource = outs, "niri"
		}
	}
	if swayText := loadSwayConfig(f.homeDir); swayText != "" {
		f.swayFound = true
		layout, variant, options, hasFile := parseSwayInput(swayText)
		if f.kbLayout == "" && !hasFile && (layout != "" || variant != "" || options != "") {
			f.kbLayout, f.kbVariant, f.kbOptions, f.kbSource = layout, variant, options, "sway"
		}
		if outs := parseSwayOutputs(swayText); len(outs) > 0 && len(f.monOutputs) == 0 {
			f.monOutputs, f.monSource = outs, "sway"
		}
	}
	f.deSalvage()
	if f.kbLayout == "" {
		if m := x11LayoutRe.FindStringSubmatch(out("localectl", "status")); m != nil {
			f.kbLayout = m[1]
		}
	}
	// localectl reports placeholders when nothing is configured.
	if f.kbLayout == "(unset)" || f.kbLayout == "n/a" {
		f.kbLayout = ""
	}

	f.prevRun = loadState(f.homeDir)

	return f
}

func (f *facts) detectGPUs() {
	cards, _ := filepath.Glob("/sys/class/drm/card*/device/uevent")
	seen := map[string]bool{}
	for _, c := range cards {
		b, err := os.ReadFile(c)
		if err != nil {
			continue
		}
		for _, ln := range strings.Split(string(b), "\n") {
			if drv, ok := strings.CutPrefix(ln, "DRIVER="); ok && !seen[drv] {
				seen[drv] = true
				f.gpus = append(f.gpus, drv)
			}
		}
	}
	if seen["nvidia"] || seen["nouveau"] {
		f.hasNvidia = true
	}
	// lspci catches NVIDIA cards with no driver bound at all.
	if !f.hasNvidia && has("lspci") {
		if strings.Contains(strings.ToLower(out("lspci")), "nvidia") {
			f.hasNvidia = true
		}
	}
	f.nouveauLive = seen["nouveau"]
	if b, err := os.ReadFile("/proc/modules"); err == nil {
		if strings.Contains(string(b), "nouveau ") {
			f.nouveauLive = true
		}
	}
}

// EFI_GLOBAL_VARIABLE, fixed by the UEFI spec.
const efiGlobalGUID = "8be4df61-93ca-11d2-aa0d-00e098032b8c"

// efivarOn reads an efivarfs boolean: 4-byte attribute header, then the data
// byte. mokutil and bootctl read the same variable the same way.
func efivarOn(b []byte) bool {
	return len(b) >= 5 && b[4] == 1
}

// secureBootEnforcing: SecureBoot=1 alone is not enough, SetupMode=1 means
// the firmware accepts any key and nothing is actually enforced.
func secureBootEnforcing(secureBoot, setupMode []byte) bool {
	return efivarOn(secureBoot) && !efivarOn(setupMode)
}

func (f *facts) detectSecureBoot() {
	sb, err := os.ReadFile("/sys/firmware/efi/efivars/SecureBoot-" + efiGlobalGUID)
	if err != nil {
		return // BIOS boot or no efivars: secure boot impossible
	}
	setup, _ := os.ReadFile("/sys/firmware/efi/efivars/SetupMode-" + efiGlobalGUID)
	f.secureBoot = secureBootEnforcing(sb, setup)
	if f.secureBoot && has("sbctl") {
		if fi, err := os.Stat("/var/lib/sbctl"); err == nil && fi.IsDir() {
			f.sbctlSigned = true
		}
	}
}

// systemdBooted is sd_booted(3)'s canonical test: the directory only exists
// when systemd is pid 1, even if libsystemd is installed under another init.
func systemdBooted() bool {
	fi, err := os.Stat("/run/systemd/system")
	return err == nil && fi.IsDir()
}

func (f *facts) detectUcode() {
	b, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return
	}
	s := string(b)
	switch {
	case strings.Contains(s, "AuthenticAMD"):
		f.ucodePkg = "amd-ucode"
	case strings.Contains(s, "GenuineIntel"):
		f.ucodePkg = "intel-ucode"
	}
	if f.ucodePkg != "" && pacmanHas(f.ucodePkg) {
		f.ucodePkg = "" // already there, nothing to add
	}
}

func (f *facts) gpuSummary() string {
	if len(f.gpus) == 0 {
		return i18n.T("none detected")
	}
	return strings.Join(f.gpus, ", ")
}

func (f *facts) otherDM() string {
	if f.currentDM != "" && f.currentDM != "sddm.service" {
		return f.currentDM
	}
	return ""
}
