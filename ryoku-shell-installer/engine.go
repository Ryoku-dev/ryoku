package main

// engine.go runs the install: ordered steps, each an idempotent shell-out
// sequence streamed line by line to the UI and the log file. The recipe is
// installation/backend/lib/deploy.sh translated from chroot to live system,
// plus the migration work (backup, rival shells, DM/network switch) that an
// existing machine needs and a blank ISO target never did.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"ryoku-i18n"
	wm "ryoku-wm"
)

// line-anchored so a commented-out "#[ryoku]" stanza does not count.
var ryokuStanzaRe = regexp.MustCompile(`(?m)^\[ryoku\]`)

const repoURL = "https://github.com/ryoku-dev/ryoku-arch.git"

const pacmanStanza = `
[ryoku]
SigLevel = Required
Server = https://repo.ryoku.dev/stable/$arch
`

// ryokuPkgs mirrors the ISO's deploy.sh: the keyring plus the ryoku-desktop
// umbrella. The umbrella version-pins and pulls every monorepo component and
// the desktop's runtime tools (recorder, night light, dictation, OCR/QR, LED,
// external-monitor brightness, ...) as hard depends, so installing just these
// two -- exactly what the ISO installs -- is the single source of truth for the
// Ryoku desktop set. Everything else (session, base OS, fonts) comes from
// base.packages via readBasePackages: the same manifest the ISO pacstraps.
var ryokuPkgs = []string{"ryoku-keyring", "ryoku-desktop"}

// ryokuOverwriteGlob names the ryoku-desktop-owned paths that a prior partial
// install, a dev deploy, or the ISO installer can leave unowned on disk: the
// privileged helpers, their polkit rules, and the Plymouth splash theme. The
// desktop transaction --overwrites them so a resume or a conversion never aborts
// on "exists in filesystem". Mirrors the ISO installer and `ryoku update`
// (updater.ryokuOverwriteGlob); keep the three in sync.
const ryokuOverwriteGlob = "/usr/bin/ryoku-*," +
	"/usr/share/polkit-1/rules.d/*ryoku*.rules," +
	"/usr/share/plymouth/themes/ryoku/*"

// bootChainSkip: base.packages entries a shell-converted box must NOT get -- it
// already owns its bootloader, initramfs, encryption and snapshot stack, and
// (re)installing these would rewrite its boot. Everything else in base.packages
// installs, so a shell box matches the ISO's set with no second hand-kept list
// to drift out of sync.
var bootChainSkip = map[string]bool{
	"base": true, "base-devel": true, "linux": true, "linux-firmware": true,
	"mkinitcpio": true, "sudo": true, "btrfs-progs": true, "cryptsetup": true,
	"dosfstools": true, "efibootmgr": true, "limine": true, "plymouth": true,
	"snapper": true, "snap-pac": true,
	// the limine hook packages pull limine + mkinitcpio back as depends and ship
	// /etc/pacman.d/hooks entries that collide with a host keeping its own boot
	// stack (Garuda's garuda-hooks owns 90-mkinitcpio-install.hook, #58).
	"limine-mkinitcpio-hook": true, "limine-snapper-sync": true,
}

// the standard Ryoku extras, all best-effort here. ryogami (the wallpaper
// daemon, from the [ryoku] repo) and matugen (the palette generator, from the
// official repo) are hard ryoku-desktop depends the packages step already
// pulled; no AUR build is needed.
var aurPkgs = []string{"bibata-cursor-theme-bin", "localsend-bin", "voxtype-bin"}

// system/packages/dev.packages; ryoku recovery builds from source and needs go.
var devPkgs = []string{"go", "nodejs", "npm", "python", "python-pip", "python-pipx", "mise"}

var sparsePaths = []string{
	"ryoku/lockscreen", "ryoku/assets", "ryoku/apps",
	"system/hardware/drivers", "system/hardware/input",
	"system/packages", "release/packages/ryoku-keyring",
}

type plan struct {
	nvidia     bool   // proprietary NVIDIA driver setup
	switchDM   bool   // disable current DM, enable SDDM
	switchNet  bool   // disable other network stacks, enable NetworkManager
	rivals     bool   // remove rival shell packages
	softOff    bool   // disable conflicting user daemons
	aur        bool   // AUR extras
	fish       bool   // fish as login shell
	devtools   bool   // dev.packages toolchains (go/rust/node/python; recovery needs go)
	omarchy    bool   // retire the [omarchy] repo and mirror pin
	monPins    bool   // pin the salvaged monitor layout in monitors_user.lua
	greeter    bool   // point SDDM at the Ryoku greeter theme
	resume     bool   // skip steps a previous interrupted run already finished
	azertyFR   bool   // force the French AZERTY layout (fr) on desktop, console, greeter
	azertyBE   bool   // force the Belgian AZERTY layout (be) on desktop, console, greeter
	compositor string // window manager to install (selects ryoku-desktop-<name>)
}

func defaultPlan(f *facts) *plan {
	return &plan{
		// secure boot rejects unsigned dkms modules and the nvidia script also
		// blacklists nouveau: proceeding would boot into a black screen. only
		// an sbctl-managed box gets to keep the default.
		nvidia:    f.hasNvidia && !f.nouveauLive && !(f.secureBoot && !f.sbctlSigned),
		switchDM:  true,
		switchNet: true,
		rivals:    true,
		softOff:   true,
		aur:       true,
		fish:      !strings.HasSuffix(f.userShell, "/fish"),
		omarchy:   f.omarchyRepo || f.omarchyMirror || len(f.omarchyGuards) > 0,
		monPins:   len(f.monOutputs) > 0,
		// when KDE's sddm-kcm owns sddm.conf.d the user chose that greeter
		// look; keep it unless they opt in.
		greeter:    !f.kdeSddmConf,
		resume:     f.prevRun != nil,
		compositor: compositors()[0],
		// the AZERTY overrides are opt-in only; a salvaged layout already
		// covers anyone who had one configured.
	}
}

// compositors lists the window managers with a shipped variant package, in
// wm.Providers order. --compositor picks one; the default is the first. The
// verify and package steps read plan.compositor, so a pick flows end to end.
func compositors() []string {
	return wm.Providers()
}

// monitorPinFile is the hand-pinned display file for a compositor, relative to
// ~/.config, taken from the seam's own user-owned list so the installer never
// spells a compositor's file name or its config directory.
func monitorPinFile(provider string) string {
	for _, rel := range wm.ConfigUserOwned(provider) {
		if strings.Contains(filepath.Base(rel), "monitors_user") {
			return rel
		}
	}
	return ""
}

// azertyExclusive keeps the two AZERTY toggles mutually exclusive: switching
// one on switches the other off. just is the toggle that was just flipped.
func (p *plan) azertyExclusive(just *bool) {
	if !*just {
		return
	}
	switch just {
	case &p.azertyFR:
		p.azertyBE = false
	case &p.azertyBE:
		p.azertyFR = false
	}
}

type evStep struct {
	idx   int
	title string
}
type evLine struct {
	line string
	// a live progress repaint (\r-terminated): the UI replaces the previous
	// transient line instead of appending, and the log file skips it
	transient bool
}
type evDone struct {
	err error
	idx int
}

type estep struct {
	id    string
	title string
	fn    func(*engine) error
}

type engine struct {
	f   *facts
	p   *plan
	dry bool
	ref string

	events  chan any
	logf    *os.File
	logPath string
	logMu   sync.Mutex

	payloadOverride string
	payload         string
	backupDir       string
	restorePath     string
	prevBackups     int
	pendingRestore  []string // undo lines queued before restore.sh exists
	state           *runState

	steps []estep
}

func newEngine(f *facts, p *plan, dry bool, ref, payloadOverride string) *engine {
	e := &engine{f: f, p: p, dry: dry, ref: ref, payloadOverride: payloadOverride}
	e.openLog()
	// resuming continues the previous run's backup dir so restore.sh stays
	// one script; declining starts a fresh state (the file is rewritten at
	// the first completed step).
	if p.resume && f.prevRun != nil {
		e.state = f.prevRun
		if f.prevRun.BackupDir != "" {
			if fi, err := os.Stat(f.prevRun.BackupDir); err == nil && fi.IsDir() {
				e.backupDir = f.prevRun.BackupDir
			}
		}
	}
	// repo trust comes before conflict removal on purpose: nothing gets
	// uninstalled until the [ryoku] db has actually been fetched. legacy
	// sources go first so the full upgrade already runs on clean mirrors.
	pacmanOnly := map[string]bool{"legacy": true, "repo": true, "aur": true, "drivers": true}
	all := []estep{
		{"legacy", i18n.T("Retiring the previous distro's package sources"), stepLegacy},
		{"sysupgrade", i18n.T("Updating the system"), stepSysupgrade},
		{"tools", i18n.T("Installing installer tools (git, build tools)"), stepTools},
		{"payload", i18n.T("Fetching the Ryoku payload"), stepPayload},
		{"backup", i18n.T("Backing up your configs"), stepBackup},
		{"repo", i18n.T("Trusting the [ryoku] package repository"), stepRepo},
		{"conflicts", i18n.T("Clearing conflicting shells and daemons"), stepConflicts},
		{"packages", i18n.T("Installing the Ryoku desktop"), stepPackages},
		{"drivers", i18n.T("Setting up GPU drivers"), stepDrivers},
		{"build", i18n.T("Building the Ryoku desktop from source"), stepBuild},
		{"session", i18n.T("Wiring the login session (SDDM, network)"), stepSession},
		{"configs", i18n.T("Laying down your Ryoku configs"), stepConfigs},
		{"aur", i18n.T("Building the AUR extras"), stepAUR},
		{"shell", i18n.T("Switching your login shell to fish"), stepFish},
		{"doctor", i18n.T("Converging the system (ryoku doctor)"), stepDoctor},
		{"verify", i18n.T("Verifying the install"), stepVerify},
	}
	src := f.distro != nil && f.distro.fromSource
	for _, s := range all {
		if src && pacmanOnly[s.id] {
			continue
		}
		if !src && s.id == "build" {
			continue
		}
		e.steps = append(e.steps, s)
	}
	return e
}

func (e *engine) openLog() {
	// dry runs must not touch the filesystem beyond a throwaway log.
	dir := os.TempDir()
	if !e.dry {
		dir = filepath.Join(e.f.homeDir, ".local/state/ryoku")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			dir = os.TempDir()
		}
	}
	e.logPath = filepath.Join(dir, "shell-install.log")
	e.logf, _ = os.OpenFile(e.logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	e.log(fmt.Sprintf("---- ryoku-shell-install run %s (dry=%v ref=%s) ----", time.Now().Format(time.RFC3339), e.dry, e.ref))
}

func (e *engine) log(s string) {
	e.logMu.Lock()
	defer e.logMu.Unlock()
	if e.logf != nil {
		fmt.Fprintln(e.logf, s)
	}
}

func (e *engine) say(s string) {
	e.log(s)
	if e.events != nil {
		e.events <- evLine{line: s}
	}
}

func (e *engine) sayf(format string, a ...any) { e.say(fmt.Sprintf(format, a...)) }

func (e *engine) sayTransient(s string) {
	if e.events != nil {
		e.events <- evLine{line: s, transient: true}
	}
}

// runFrom executes steps starting at idx (retry re-enters at the failed one).
func (e *engine) runFrom(idx int) chan any {
	e.events = make(chan any, 256)
	go func() {
		for i := idx; i < len(e.steps); i++ {
			s := e.steps[i]
			e.events <- evStep{idx: i, title: s.title}
			e.log("==== step " + s.id + " ====")
			if e.p.resume && e.state != nil && e.state.has(s.id) {
				e.say(i18n.T("finished in the previous run, resuming past it"))
				continue
			}
			if err := s.fn(e); err != nil {
				e.sayf(i18n.T("step %s failed: %v"), s.id, err)
				e.events <- evDone{err: err, idx: i}
				return
			}
			e.markStepDone(s.id)
		}
		e.clearState()
		e.events <- evDone{idx: len(e.steps)}
	}()
	return e.events
}

// cleanTermLine reduces raw child output to what a terminal would leave
// visible: text after the last \r, tabs spaced, control bytes dropped. A bare
// \r (curl's progress meter) rendered inside the TUI panel jumps the cursor
// to column 0 and tears the frame.
func cleanTermLine(s string) string {
	s = strings.TrimRight(s, "\r\n")
	if i := strings.LastIndexByte(s, '\r'); i >= 0 {
		s = s[i+1:]
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteString("  ")
		case r < 0x20 || r == 0x7f:
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func shellJoin(name string, args []string) string {
	parts := []string{name}
	for _, a := range args {
		if strings.ContainsAny(a, " \t\"'$") {
			a = fmt.Sprintf("%q", a)
		}
		parts = append(parts, a)
	}
	return strings.Join(parts, " ")
}

// cmd runs a command streaming merged stdout/stderr into the UI and the log.
func (e *engine) cmd(dir string, env []string, name string, args ...string) error {
	line := shellJoin(name, args)
	if e.dry {
		e.say("DRYRUN: " + line)
		return nil
	}
	e.say("$ " + line)
	c := exec.Command(name, args...)
	c.Dir = dir
	if len(env) > 0 {
		c.Env = append(os.Environ(), env...)
	}
	pr, pw := io.Pipe()
	c.Stdout, c.Stderr = pw, pw
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// byte loop, not a Scanner: a Scanner stops at its buffer cap and the
		// write side of the pipe then blocks forever. \r ends a token too, so
		// pacman's download progress surfaces live instead of buffering until
		// each file's final newline; a panel that ticks during long downloads
		// is the difference between "working" and "waiting for my password".
		rd := bufio.NewReader(pr)
		var buf []byte
		var lastProgress time.Time
		for {
			b, err := rd.ReadByte()
			if err != nil {
				if ln := cleanTermLine(string(buf)); ln != "" {
					e.say("  " + ln)
				}
				return
			}
			switch b {
			case '\n':
				if ln := cleanTermLine(string(buf)); ln != "" {
					e.say("  " + ln)
				}
				buf = buf[:0]
			case '\r':
				if nxt, perr := rd.Peek(1); perr == nil && nxt[0] == '\n' {
					continue // \r\n: the \n case finishes the line
				}
				// a progress repaint: replaced in place in the UI, rate-capped,
				// never written to the log file
				if ln := cleanTermLine(string(buf)); ln != "" && time.Since(lastProgress) > 80*time.Millisecond {
					lastProgress = time.Now()
					e.sayTransient("  " + ln)
				}
				buf = buf[:0]
			default:
				buf = append(buf, b)
			}
		}
	}()
	err := c.Run()
	pw.Close()
	wg.Wait()
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// sudoArgv: what `sudo -n` gets. Omarchy's guard hook aborts a pacman -Syu it did
// not start, and both updateCmd and installCmd are -Syu, so until stepLegacy
// retires the hook every package step needs the escape the guard documents.
func sudoArgv(args []string, guarded bool) []string {
	if !guarded || len(args) == 0 || args[0] != "pacman" {
		return args
	}
	return append([]string{"env", "OMARCHY_ALLOW_DIRECT_PACMAN=1"}, args...)
}

func (e *engine) sudo(args ...string) error {
	guarded := e.f != nil && len(e.f.omarchyGuards) > 0
	return e.cmd("", nil, "sudo", append([]string{"-n"}, sudoArgv(args, guarded)...)...)
}

// sudoSh runs a fixed shell snippet as root; only static strings go in here.
func (e *engine) sudoSh(script string) error {
	return e.cmd("", nil, "sudo", "-n", "sh", "-c", script)
}

// sudoWrite replaces a root-owned file with the given content via tee.
func (e *engine) sudoWrite(path, content string) error {
	if e.dry {
		e.say(i18n.Tf("DRYRUN: write %s", path))
		return nil
	}
	e.say("$ sudo tee " + path)
	c := exec.Command("sudo", "-n", "tee", path)
	c.Stdin = strings.NewReader(content)
	c.Stdout, c.Stderr = io.Discard, io.Discard
	if err := c.Run(); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// stripPacmanSection drops one [section] and its body from pacman.conf text.
func stripPacmanSection(conf, section string) string {
	var out []string
	skip := false
	for _, ln := range strings.Split(conf, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			skip = t == "["+section+"]"
		}
		if !skip {
			out = append(out, ln)
		}
	}
	return strings.Join(out, "\n")
}

// ---- steps ----

// an ex-Omarchy machine still trusts the [omarchy] repo and routes core/extra
// through Omarchy's own mirror; a Ryoku box must depend on neither. originals
// are kept as *.pre-ryoku and the undo lands in restore.sh.
func stepLegacy(e *engine) error {
	if !e.p.omarchy || (!e.f.omarchyRepo && !e.f.omarchyMirror && len(e.f.omarchyGuards) == 0) {
		e.say(i18n.T("no previous distro package sources to retire"))
		return nil
	}
	if e.f.omarchyRepo {
		conf, err := os.ReadFile("/etc/pacman.conf")
		if err != nil {
			return err
		}
		stripped := stripPacmanSection(string(conf), "omarchy")
		if stripped != string(conf) {
			if err := e.sudo("cp", "/etc/pacman.conf", "/etc/pacman.conf.pre-ryoku"); err != nil {
				return err
			}
			if err := e.sudoWrite("/etc/pacman.conf", stripped); err != nil {
				return err
			}
			e.say(i18n.T("dropped the [omarchy] repository (original at /etc/pacman.conf.pre-ryoku)"))
			e.pendingRestore = append(e.pendingRestore,
				"sudo cp /etc/pacman.conf.pre-ryoku /etc/pacman.conf")
		}
	}
	if e.f.omarchyMirror {
		if err := e.sudo("cp", "/etc/pacman.d/mirrorlist", "/etc/pacman.d/mirrorlist.pre-ryoku"); err != nil {
			return err
		}
		ml := "# restored by ryoku-shell-install: the previous Omarchy install pinned its\n" +
			"# own package mirror here. original at mirrorlist.pre-ryoku; rank your own\n" +
			"# mirrors with reflector if you want more than the worldwide CDN.\n" +
			"Server = https://geo.mirror.pkgbuild.com/$repo/os/$arch\n"
		if err := e.sudoWrite("/etc/pacman.d/mirrorlist", ml); err != nil {
			return err
		}
		e.say(i18n.T("restored a standard Arch mirrorlist (original at /etc/pacman.d/mirrorlist.pre-ryoku)"))
		e.pendingRestore = append(e.pendingRestore,
			"sudo cp /etc/pacman.d/mirrorlist.pre-ryoku /etc/pacman.d/mirrorlist")
	}
	for _, hook := range e.f.omarchyGuards {
		aside := hook + ".pre-ryoku"
		if err := e.sudo("mv", hook, aside); err != nil {
			return err
		}
		e.say(i18n.Tf("retired Omarchy's pacman guard hook, which aborts every -Syu (moved to %s)", aside))
		e.pendingRestore = append(e.pendingRestore, "sudo mv "+aside+" "+hook)
	}
	if !e.dry && pacmanHas("omarchy-keyring") {
		if err := e.sudo("pacman", "-R", "--noconfirm", "omarchy-keyring"); err != nil {
			e.say(i18n.T("warning: could not remove omarchy-keyring (continuing)"))
		}
	}
	return nil
}

func stepSysupgrade(e *engine) error {
	d := e.d()
	if d.id != "arch" {
		if err := e.sudo(d.refreshCmd...); err != nil {
			return err
		}
	}
	return e.sudo(d.updateCmd...)
}

func stepTools(e *engine) error {
	d := e.d()
	return e.sudo(d.installArgs([]string{"git", d.local("base-devel")})...)
}

func stepPayload(e *engine) error {
	if e.payloadOverride != "" {
		e.payload = e.payloadOverride
		if _, err := os.Stat(filepath.Join(e.payload, "ryoku/lockscreen/install-qylock")); err != nil && !e.dry {
			return errors.New(i18n.Tf("payload override %s does not look like a ryoku-arch checkout", e.payload))
		}
		e.say(i18n.Tf("using payload checkout %s", e.payload))
		return nil
	}
	cache := os.Getenv("XDG_CACHE_HOME")
	if cache == "" {
		cache = filepath.Join(e.f.homeDir, ".cache")
	}
	e.payload = filepath.Join(cache, "ryoku-shell-install/repo")

	if _, err := os.Stat(filepath.Join(e.payload, ".git")); err == nil {
		if err := e.cmd(e.payload, nil, "git", "fetch", "--depth=1", "origin", e.ref); err != nil {
			return err
		}
		if err := e.cmd(e.payload, nil, "git", "checkout", "-f", "FETCH_HEAD"); err != nil {
			return err
		}
	} else {
		if !e.dry {
			if err := os.MkdirAll(filepath.Dir(e.payload), 0o755); err != nil {
				return err
			}
		}
		if err := e.cmd("", nil, "git", "clone", "--depth=1", "--filter=blob:none", "--sparse",
			"--branch", e.ref, repoURL, e.payload); err != nil {
			return err
		}
	}
	if err := e.cmd(e.payload, nil, "git", append([]string{"sparse-checkout", "set"}, sparsePaths...)...); err != nil {
		return err
	}
	// a cache from an older installer can come out of the update missing paths
	// the current engine needs; a broken cache is worth less than a fresh clone.
	if _, err := os.Stat(filepath.Join(e.payload, "system/packages/base.packages")); err != nil && !e.dry {
		e.say(i18n.T("payload cache is incomplete; recloning it fresh"))
		if err := os.RemoveAll(e.payload); err != nil {
			return err
		}
		if err := e.cmd("", nil, "git", "clone", "--depth=1", "--filter=blob:none", "--sparse",
			"--branch", e.ref, repoURL, e.payload); err != nil {
			return err
		}
		return e.cmd(e.payload, nil, "git", append([]string{"sparse-checkout", "set"}, sparsePaths...)...)
	}
	return nil
}

// paths under $HOME that materialize or the seeds will touch. hypr, quickshell
// and the lockscreen dirs are moved aside wholesale: a stale hyprland.conf or a
// foreign quickshell tree next to the Ryoku one is exactly the breakage a
// migration must avoid. the rest are copied, then clobbered in place.
// xdg-desktop-portal moves aside too: a user-level portals.conf outranks the
// packaged hyprland one and breaks screenshare under the new session.
var backupMove = []string{
	".config/hypr", ".config/quickshell", ".config/xdg-desktop-portal",
	".local/share/quickshell-lockscreen", ".local/share/qylock",
}
var backupCopy = []string{
	".config/niri", ".config/sway", ".config/kitty", ".config/fish", ".config/nvim",
	".config/fastfetch", ".config/yazi", ".config/matugen", ".config/qt6ct",
	".config/starship.toml", ".config/mimeapps.list",
	".config/systemd/user", // raw symlink tree, restore.sh puts wants wiring back as-was
}

func stepBackup(e *engine) error {
	root := filepath.Join(e.f.homeDir, ".local/state/ryoku/shell-install")
	if prev, _ := filepath.Glob(filepath.Join(root, "backup-*")); len(prev) > 0 {
		e.prevBackups = len(prev)
		e.say(i18n.Tf("note: %d earlier backup(s) exist under %s; the oldest holds your pre-Ryoku configs", len(prev), root))
	}
	if e.dry {
		e.say(i18n.Tf("DRYRUN: back up %s", strings.Join(append(append([]string{}, backupMove...), backupCopy...), ", ")))
		return nil
	}
	// a retry in the same run reuses the dir: items already saved are skipped
	// instead of being re-captured into a second, half-empty backup.
	if e.backupDir == "" {
		e.backupDir = filepath.Join(root, "backup-"+time.Now().Format("20060102-150405"))
	}
	e.restorePath = filepath.Join(e.backupDir, "restore.sh")
	if err := os.MkdirAll(e.backupDir, 0o755); err != nil {
		return err
	}
	// restore.sh exists before the first item moves, and grows a line per
	// saved item, so a kill at any point leaves a script that undoes exactly
	// what happened so far.
	if _, err := os.Stat(e.restorePath); err != nil {
		hdr := "#!/usr/bin/env bash\n" +
			"# restore configs saved by ryoku-shell-install. undo lines are appended as\n" +
			"# the installer changes things; run the whole script to roll back.\n" +
			"set -euo pipefail\nDIR=\"$(cd \"$(dirname \"$0\")\" && pwd)\"\n"
		if err := os.WriteFile(e.restorePath, []byte(hdr), 0o755); err != nil {
			return err
		}
	}
	// undo lines queued by steps that ran before this file existed (legacy
	// package-source retirement runs first).
	for _, ln := range e.pendingRestore {
		e.recordRestore(ln)
	}
	e.pendingRestore = nil
	saveOne := func(rel string, move bool) error {
		src := filepath.Join(e.f.homeDir, rel)
		dst := filepath.Join(e.backupDir, rel)
		if _, err := os.Lstat(dst); err == nil {
			return nil // already saved by a previous attempt of this run
		}
		if _, err := os.Lstat(src); err != nil {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if move {
			if err := os.Rename(src, dst); err != nil {
				// cross-device home layouts (bind mounts, separate subvolume
				// mounts): fall back to copy + delete.
				if err := copyTree(src, dst); err != nil {
					return err
				}
				if err := os.RemoveAll(src); err != nil {
					return err
				}
			}
			e.say(i18n.Tf("moved aside %s", rel))
		} else {
			if err := copyTree(src, dst); err != nil {
				return err
			}
			e.say(i18n.Tf("backed up %s", rel))
		}
		e.recordRestore(fmt.Sprintf("rm -rf %q && mkdir -p %q && cp -a %q %q",
			"$HOME/"+rel, filepath.Dir("$HOME/"+rel), "$DIR/"+rel, "$HOME/"+rel))
		return nil
	}
	for _, rel := range backupMove {
		if err := saveOne(rel, true); err != nil {
			return err
		}
	}
	// moving the hypr/quickshell trees is what disarms a rice's exec-once
	// autostart chain; name it so the log explains the disappearing bar.
	for _, r := range e.f.riceFound {
		e.say(i18n.Tf("rice %s: its autostart lives in the trees moved above; extra configs (waybar, swaync, ...) stay untouched in place", r))
	}
	for _, rel := range backupCopy {
		if err := saveOne(rel, false); err != nil {
			return err
		}
	}
	e.say(i18n.Tf("backup at %s (restore.sh inside)", e.backupDir))
	return nil
}

// recordRestore appends an undo line to restore.sh.
func (e *engine) recordRestore(line string) {
	if e.dry || e.restorePath == "" {
		return
	}
	fh, err := os.OpenFile(e.restorePath, os.O_APPEND|os.O_WRONLY, 0o755)
	if err != nil {
		return
	}
	defer fh.Close()
	fmt.Fprintln(fh, line)
}

func stepConflicts(e *engine) error {
	// units first, while their unit files still exist. disable only, never
	// stop: running daemons die with the old session.
	if e.p.softOff {
		for _, u := range e.f.softUnits {
			if err := e.cmd("", nil, "systemctl", "--user", "disable", u); err != nil {
				e.say(i18n.Tf("warning: could not disable %s", u))
				continue
			}
			// || true: add-wants units have no [Install] and refuse a bare enable
			e.recordRestore("systemctl --user enable " + u + " || true")
		}
	}
	if e.p.rivals && len(e.f.rivalPkgs) > 0 {
		// plain -R, no -ns cascade: -Rns on a meta like cachyos-niri-noctalia
		// would drag niri itself out, and the plan promises niri survives as a
		// fallback session. leftover deps become orphans doctor can report.
		e.say(i18n.Tf("removing rival shell packages: %s", strings.Join(e.f.rivalPkgs, " ")))
		if err := e.sudo(e.d().removeArgs(e.f.rivalPkgs)...); err != nil {
			e.say(i18n.T("bulk removal failed, retrying one by one"))
			for _, p := range e.f.rivalPkgs {
				if err := e.sudo(e.d().removeArgs([]string{p})...); err != nil {
					e.say(i18n.Tf("warning: could not remove %s (continuing)", p))
				}
			}
		}
	}
	if len(e.f.zshFrameworkPkgs) > 0 {
		// ryoku-oh-my-zsh provides and replaces both upstream frameworks, but a
		// plain -R under --noconfirm refuses while an installed plugin package
		// depends on oh-my-zsh-git, and the dependency conflict then aborts the
		// whole desktop transaction. -Rdd drops the framework alone; its plugins
		// re-resolve against ryoku-oh-my-zsh's provides in the install step.
		e.say(i18n.Tf("replacing %s with the Ryoku zsh framework", strings.Join(e.f.zshFrameworkPkgs, " ")))
		if err := e.sudo(append([]string{"pacman", "-Rdd", "--noconfirm"}, e.f.zshFrameworkPkgs...)...); err != nil {
			e.say(i18n.Tf("warning: could not remove %s; the package step may abort on a dependency conflict", strings.Join(e.f.zshFrameworkPkgs, " ")))
		} else {
			for _, p := range e.f.zshFrameworkPkgs {
				e.recordRestore("sudo pacman -S --asdeps " + p)
			}
		}
	}
	if len(e.f.blockerPkgs) > 0 {
		// pacman --noconfirm answers conflict prompts with No and aborts, so
		// packages that conflict with the desktop set (pulseaudio vs
		// pipewire-pulse, quickshell-git vs quickshell) must go first.
		e.say(i18n.Tf("removing packages that block the desktop install: %s", strings.Join(e.f.blockerPkgs, " ")))
		if err := e.sudo(e.d().removeArgs(e.f.blockerPkgs)...); err != nil {
			for _, p := range e.f.blockerPkgs {
				if err := e.sudo(e.d().removeArgs([]string{p})...); err != nil {
					e.say(i18n.Tf("warning: could not remove %s; the package step may abort on a conflict", p))
				}
			}
		}
	}
	return nil
}

func stepRepo(e *engine) error {
	// on a box that already has ryoku-keyring, the keyring files under
	// /usr/share/pacman/keyrings are package-owned: seeding and deleting them
	// again would strip files out of the installed package. the trustdb is
	// already populated, so the whole dance is unnecessary.
	if !e.dry && pacmanHas("ryoku-keyring") {
		e.say(i18n.T("ryoku-keyring already installed, key trust in place"))
	} else {
		kdir := filepath.Join(e.payload, "release/packages/ryoku-keyring")
		kd := "/usr/share/pacman/keyrings"
		for _, f := range []string{"ryoku.gpg", "ryoku-trusted", "ryoku-revoked"} {
			if err := e.sudo("install", "-Dm644", filepath.Join(kdir, f), filepath.Join(kd, f)); err != nil {
				return err
			}
		}
		if err := e.sudo("pacman-key", "--populate", "ryoku"); err != nil {
			return err
		}
		// drop the seeds so the ryoku-keyring package installs without a file
		// conflict; the trustdb keeps the key (same dance as deploy.sh).
		if err := e.sudo("rm", "-f", kd+"/ryoku.gpg", kd+"/ryoku-trusted", kd+"/ryoku-revoked"); err != nil {
			return err
		}
	}

	conf, err := os.ReadFile("/etc/pacman.conf")
	if err != nil {
		return err
	}
	if !ryokuStanzaRe.Match(conf) {
		// whole-file swap, not `>>`: a crash mid-append could leave a truncated
		// stanza that pacman rejects while a resume's regex check still sees
		// `[ryoku]` and skips the repair. mv on the same fs commits atomically.
		// chmod pins 0644: the new inode's mode would otherwise follow the
		// invoking user's umask (sudo propagates it), and a umask-077 box would
		// flip pacman.conf to 0600, breaking every non-root pacman reader,
		// including this installer's own resume read above.
		if err := e.sudoSh(`{ cat /etc/pacman.conf && printf '%s' '` + pacmanStanza + `'; } > /etc/pacman.conf.ryoku-new && ` +
			`chmod 644 /etc/pacman.conf.ryoku-new && ` +
			`mv -f /etc/pacman.conf.ryoku-new /etc/pacman.conf`); err != nil {
			return err
		}
		e.say(i18n.T("added the [ryoku] repository to /etc/pacman.conf"))
	} else {
		e.say(i18n.T("[ryoku] repository already present in /etc/pacman.conf"))
	}
	// refresh right after the -Syu step, so this cannot strand a partial
	// upgrade; it only pulls the fresh [ryoku] db.
	return e.sudo("pacman", "-Sy")
}

// readBasePackages parses system/packages/base.packages from the payload (the
// same manifest the ISO pacstraps): one package per line, '#' comments and
// blank lines dropped, boot-chain entries skipped. This is the single source of
// truth for the machine package set, shared verbatim with the ISO.
func (e *engine) readBasePackages() ([]string, error) {
	path := filepath.Join(e.payload, "system/packages/base.packages")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var pkgs []string
	for _, ln := range strings.Split(string(data), "\n") {
		if i := strings.IndexByte(ln, '#'); i >= 0 {
			ln = ln[:i]
		}
		ln = strings.TrimSpace(ln)
		if ln == "" || bootChainSkip[ln] {
			continue
		}
		pkgs = append(pkgs, ln)
	}
	return pkgs, nil
}

func (e *engine) asusAura() bool {
	detector := filepath.Join(e.payload, "system/hardware/input/ryoku-hw-asus-aura")
	return exec.Command(detector).Run() == nil
}

// providerAnswers probes the installed window-manager provider. caps, not state:
// verification runs before the new session exists, and caps answers without a
// live compositor.
func (e *engine) providerAnswers() bool {
	return exec.Command("ryoku-wm-"+e.p.compositor, "caps").Run() == nil
}

func stepPackages(e *engine) error {
	d := e.d()
	base, err := e.readBasePackages()
	if err != nil {
		if !e.dry {
			return err
		}
		e.say(i18n.T("DRYRUN: payload not cloned; would read system/packages/base.packages"))
	}
	var pkgs []string
	if d.fromSource {
		// no [ryoku] repository here: the desktop is built from the payload, so
		// install its dependencies plus the toolchain that builds it.
		pkgs = append(d.localAll(base), d.build...)
	} else {
		pkgs = append(append([]string{}, ryokuPkgs...), base...)
		// name the chosen variant so pacman installs it directly instead of
		// prompting when more than one provides ryoku-desktop-compositor.
		pkgs = append(pkgs, "ryoku-desktop-"+e.p.compositor)
		if e.asusAura() {
			if d.installedPkg("tlp") {
				e.say(i18n.T("ASUS Aura lighting skipped because TLP is installed"))
			} else {
				pkgs = append(pkgs, "asusctl")
			}
		}
	}
	if e.f.ucodePkg != "" {
		pkgs = append(pkgs, d.local(e.f.ucodePkg))
	}
	if e.p.devtools {
		pkgs = append(pkgs, d.localAll(devPkgs)...)
	}
	pkgs = e.dropSatisfied(pkgs)
	if d.id == "arch" {
		// a .part resumed against a mirror whose bytes moved on trips pacman's
		// size cap on every retry; dropping resume state just costs a re-download.
		if err := e.sudoSh(`rm -f /var/cache/pacman/pkg/*.part`); err != nil {
			e.say(i18n.T("warning: could not clear partial downloads (continuing)"))
		}
	}
	// -Syu, not -S: a resumed run holds the db its first attempt synced, and
	// a publish in between replaces or prunes the files that db points at.
	return e.sudo(desktopPacmanArgs(d, pkgs)...)
}

// desktopPacmanArgs builds the package transaction for stepPackages. On Arch it
// --overwrites the ryoku-desktop-owned paths a prior partial install, a dev
// deploy, or the ISO installer can leave unowned (ryokuOverwriteGlob), so a
// resume or a conversion adopts them instead of aborting the whole transaction on
// "exists in filesystem". fromSource distros build from the payload and never take
// this path, so they keep the plain install command.
func desktopPacmanArgs(d *distro, pkgs []string) []string {
	args := append([]string{}, d.installCmd...)
	if d.id == "arch" {
		args = append(args, "--overwrite", ryokuOverwriteGlob)
	}
	return append(args, pkgs...)
}

// dropSatisfied keeps only what no installed provider satisfies (pacman -T
// prints the unmet names), so a provides-conflict like qt6ct-kde cannot
// abort the install under --noconfirm.
func (e *engine) dropSatisfied(pkgs []string) []string {
	if e.dry || len(pkgs) == 0 {
		return pkgs
	}
	if e.d().id != "arch" {
		return pkgs
	}
	out, err := exec.Command("pacman", append([]string{"-T"}, pkgs...)...).Output()
	// exit 127 aside, pacman -T exits 1 when any target is unmet; the list on
	// stdout is the answer either way, so only a total failure falls back.
	if len(out) == 0 && err != nil {
		return pkgs
	}
	keep := filterByUnmet(pkgs, string(out))
	if len(keep) < len(pkgs) {
		e.say(i18n.Tf("%d of %d packages already satisfied by installed providers", len(pkgs)-len(keep), len(pkgs)))
	}
	return keep
}

// stepBuild is the fromSource replacement for installing ryoku-desktop: the
// payload's deploy.sh already builds the Go programs, the QML modules and the
// Ryoku.Blobs plugin, then materializes the config. It skips the Hyprland
// compositor plugins when makepkg is absent, which is the case off Arch.
func stepBuild(e *engine) error {
	script := filepath.Join(e.payload, "ryoku", "shell", "deploy.sh")
	if e.dry {
		e.say(i18n.Tf("DRYRUN: would run %s", script))
		return nil
	}
	if _, err := os.Stat(script); err != nil {
		return errors.New(i18n.T("payload is missing ryoku/shell/deploy.sh"))
	}
	e.say(i18n.T("building the desktop from the payload (this takes a few minutes)"))
	return e.cmd(filepath.Join(e.payload, "ryoku", "shell"), nil, "bash", script)
}

// filterByUnmet keeps only the names pacman -T reported as unmet.
func filterByUnmet(pkgs []string, unmetOut string) []string {
	unmet := map[string]bool{}
	for _, ln := range strings.Split(strings.TrimSpace(unmetOut), "\n") {
		if ln != "" {
			unmet[ln] = true
		}
	}
	var keep []string
	for _, p := range pkgs {
		if unmet[p] {
			keep = append(keep, p)
		}
	}
	return keep
}

func stepDrivers(e *engine) error {
	drv := filepath.Join(e.payload, "system/hardware/drivers")
	scripts := []string{"amd.sh", "intel.sh", "vulkan.sh"}
	if e.p.nvidia {
		scripts = append(scripts, "nvidia.sh")
	} else if e.f.hasNvidia {
		if e.f.secureBoot && !e.f.sbctlSigned {
			e.say(i18n.T("skipping the NVIDIA driver setup (Secure Boot is on and would reject the unsigned modules)"))
		} else {
			e.say(i18n.T("skipping the NVIDIA driver setup (kept nouveau; re-run with the toggle on to switch)"))
		}
	}
	// the vendor scripts each do a bare `pacman -S`, so -- exactly like
	// stepPackages -- they must transact against a current db. this step is
	// re-entered on a resume with the sysupgrade/packages steps already
	// skipped, and a repo publish can land between those steps and this one,
	// pruning the files a stale db still points at (pacman's "failed
	// retrieving file" abort that reads as a driver that would not install).
	// clear resumed .part downloads and bring the system current first.
	if err := e.sudoSh(`rm -f /var/cache/pacman/pkg/*.part`); err != nil {
		e.say(i18n.T("warning: could not clear partial downloads (continuing)"))
	}
	if err := e.sudo("pacman", "-Syu", "--noconfirm"); err != nil {
		e.say(i18n.T("warning: could not refresh the package db before the driver install; a stale mirror may still fail a download (continuing)"))
	}
	// a single vendor script failing must NOT sink the whole desktop install,
	// matching installation/backend/lib/drivers.sh: the box still boots on the
	// iGPU or software renderer, and stepDoctor plus first boot heal the driver.
	for _, s := range scripts {
		if err := e.cmd("", nil, "bash", filepath.Join(drv, s)); err != nil {
			e.sayf(i18n.T("warning: %s did not finish; leaving the GPU driver for `ryoku doctor` after first boot (continuing)"), s)
		}
	}
	if e.p.nvidia && e.f.hasNvidia {
		// the scripts leave the initramfs to the caller. probe for whichever
		// generator the box uses; a missed rebuild is a warning, not an abort,
		// the next kernel update rebuilds anyway.
		var err error
		switch {
		case has("limine-mkinitcpio"):
			err = e.sudo("limine-mkinitcpio")
		case has("mkinitcpio"):
			err = e.sudo("mkinitcpio", "-P")
		case has("dracut-rebuild"):
			err = e.sudo("dracut-rebuild")
		case has("dracut"):
			err = e.sudo("dracut", "--regenerate-all", "--force")
		default:
			e.say(i18n.T("warning: no known initramfs generator found, skipping the rebuild"))
		}
		if err != nil {
			e.say(i18n.T("warning: initramfs rebuild failed; run it by hand before rebooting (see log)"))
		}
	}
	return nil
}

func stepSession(e *engine) error {
	if e.p.switchDM {
		if dm := e.f.otherDM(); dm != "" {
			// disable, never mask or uninstall: reversible, and the running
			// greeter session is untouched until reboot.
			if err := e.sudo("systemctl", "disable", dm); err != nil {
				return err
			}
			e.recordRestore("sudo systemctl disable sddm.service && sudo systemctl enable " + dm)
		} else if e.f.currentDM == "" {
			e.recordRestore("sudo systemctl disable sddm.service && sudo systemctl set-default multi-user.target")
		}
		if err := e.cmd("", nil, "bash", filepath.Join(e.payload, "ryoku/lockscreen/sddm/setup")); err != nil {
			return err
		}
	} else {
		e.say(i18n.T("keeping your current display manager; select the Hyprland session at login"))
	}

	// qylock bundle lives at the same system path the ISO uses, then its own
	// installer wires greeter theme + user lockscreen.
	if err := e.sudo("mkdir", "-p", "/usr/share/ryoku"); err != nil {
		return err
	}
	if err := e.sudoSh(`rm -rf /usr/share/ryoku/qylock`); err != nil {
		return err
	}
	if err := e.sudo("cp", "-r", filepath.Join(e.payload, "ryoku/lockscreen/qylock"), "/usr/share/ryoku/qylock"); err != nil {
		return err
	}
	if err := e.cmd("", []string{
		"RYOKU_QYLOCK_BUNDLE=/usr/share/ryoku/qylock",
		"RYOKU_QYLOCK_MODE=live",
	}, "bash", filepath.Join(e.payload, "ryoku/lockscreen/install-qylock")); err != nil {
		return err
	}

	// greeter policy: install-qylock wrote 99-ryoku.conf and sddm/setup wrote
	// 10-ryoku-wayland.conf (DisplayServer=wayland). SDDM reads conf.d lexically
	// and later files win per key, so KDE's kde_settings.conf outranks the theme.
	// The greeter toggle decides who ends up on top; opting out drops both Ryoku
	// files so the user's greeter theme AND display server stand.
	switch {
	case !e.p.greeter:
		if err := e.sudo("rm", "-f", "/etc/sddm.conf.d/99-ryoku.conf", "/etc/sddm.conf.d/zz-ryoku.conf", "/etc/sddm.conf.d/10-ryoku-wayland.conf"); err != nil {
			return err
		}
		e.say(i18n.T("kept your current SDDM greeter (Ryoku's theme and Wayland greeter are installed, not selected)"))
	case e.f.kdeSddmConf:
		zz := "# written by ryoku-shell-install: sorts after kde_settings.conf so the\n" +
			"# ryoku greeter theme wins. delete this file to get the KDE greeter back.\n" +
			"[Theme]\nCurrent=ryoku\n"
		if err := e.sudoWrite("/etc/sddm.conf.d/zz-ryoku.conf", zz); err != nil {
			return err
		}
		e.recordRestore("sudo rm -f /etc/sddm.conf.d/zz-ryoku.conf /etc/sddm.conf.d/99-ryoku.conf /etc/sddm.conf.d/10-ryoku-wayland.conf")
		e.say(i18n.T("Ryoku greeter theme selected past KDE's kde_settings.conf drop-in"))
	default:
		e.recordRestore("sudo rm -f /etc/sddm.conf.d/99-ryoku.conf /etc/sddm.conf.d/10-ryoku-wayland.conf")
	}

	// sddm/setup strips pam_gnome_keyring on purpose: a fresh Ryoku box uses a
	// passwordless default keyring. an ex-GNOME user instead has a password
	// protected login keyring from GDM, and without these lines every login
	// prompts for it (stock Arch /etc/pam.d/sddm only carries kwallet).
	if e.p.switchDM && hasDesktop(e.f.desktops, "GNOME") {
		if err := e.sudoSh(`f=/etc/pam.d/sddm
if [ -f "$f" ] && ! grep -q pam_gnome_keyring "$f"; then
  printf '%s\n' 'auth        optional    pam_gnome_keyring.so' 'session     optional    pam_gnome_keyring.so    auto_start' >> "$f"
fi`); err != nil {
			return err
		}
		e.recordRestore(`sudo sed -i '/pam_gnome_keyring\.so/d' /etc/pam.d/sddm`)
		e.say(i18n.T("kept gnome-keyring auto-unlock working under SDDM (your GNOME login keyring)"))
	}
	if len(e.f.desktops) > 0 {
		e.sayf(i18n.T("%s stays installed; pick it from the session menu at the login screen anytime"),
			strings.Join(e.f.desktops, ", "))
	}

	if e.p.switchNet {
		for _, n := range e.f.otherNet {
			if err := e.sudo("systemctl", "disable", n); err != nil {
				e.say(i18n.Tf("warning: could not disable %s", n))
				continue
			}
			e.recordRestore("sudo systemctl enable " + n)
		}
		if !e.f.nmEnabled {
			if err := e.sudo("systemctl", "enable", "NetworkManager.service"); err != nil {
				return err
			}
			e.recordRestore("sudo systemctl disable NetworkManager.service")
		}
		// iwd backend pin, Ryoku network policy. takes effect at the next NM
		// restart (reboot), so the live wifi connection is never dropped.
		if err := e.sudoSh(`install -Dm644 /dev/stdin /etc/NetworkManager/conf.d/wifi-backend.conf <<'EOF'
[device]
wifi.backend=iwd
EOF`); err != nil {
			return err
		}
	} else {
		e.say(i18n.T("keeping your current network stack"))
	}
	return nil
}

func stepConfigs(e *engine) error {
	ryoku := e.ryokuBin()
	if !e.dry && ryoku == "" {
		return errors.New(i18n.T("the ryoku CLI is missing; the package step did not finish"))
	}
	if ryoku == "" {
		ryoku = "ryoku"
	}
	if err := e.cmd("", nil, ryoku, "materialize"); err != nil {
		return err
	}

	// salvaged monitor pins go in before the stub pass, real pins beat a
	// comment stub. only the hyprland dialect supports desc: names.
	//
	// The file and the dialect both follow the compositor being installed: the
	// seam names the hand-pin file, and its extension picks the emitter. A
	// hardcoded Lua path would drop a file niri never reads, and the salvaged
	// layout would vanish with no error.
	if rel := monitorPinFile(e.p.compositor); e.p.monPins && len(e.f.monOutputs) > 0 && rel != "" {
		render := renderPins
		if filepath.Ext(rel) == ".kdl" {
			render = renderKdlPins
		}
		pins, skipped := render(e.f.monOutputs, e.f.monSource == "hyprland", e.f.monSource)
		leaf := filepath.Base(rel)
		for _, name := range skipped {
			e.sayf(i18n.T("note: %s output %q is matched by description; pin it by connector in %s"), e.f.monSource, name, leaf)
		}
		if pins != "" {
			mu := filepath.Join(e.f.homeDir, ".config", rel)
			if e.dry {
				e.sayf(i18n.T("DRYRUN: write %s monitor pins to ~/.config/%s"), e.f.monSource, rel)
			} else if _, err := os.Lstat(mu); err != nil {
				if err := os.MkdirAll(filepath.Dir(mu), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(mu, []byte(pins), 0o644); err != nil {
					return err
				}
				e.sayf(i18n.T("carried the %s monitor layout into %s"), e.f.monSource, rel)
			}
		}
	}

	// an explicit AZERTY choice in the plan beats any salvaged layout.
	azerty := e.p.azertyFR || e.p.azertyBE
	if azerty {
		layout := "fr"
		if e.p.azertyBE {
			layout = "be"
		}
		e.f.kbLayout, e.f.kbVariant, e.f.kbOptions, e.f.kbSource = layout, "", "", "plan"
	}

	// keyboard.lua is user-owned: seeded once, never touched by updates.
	lua := func(s string) string { return strings.NewReplacer(`"`, ``, `\`, ``).Replace(s) }
	if e.f.kbLayout != "" && (e.f.kbLayout != "us" || e.f.kbVariant != "" || e.f.kbOptions != "") {
		src := e.f.kbSource
		if src == "" {
			src = "localectl"
		}
		kb := filepath.Join(e.f.homeDir, ".config/hypr/keyboard.lua")
		// a salvaged layout never clobbers an existing file (a repair run
		// keeps hand edits); an explicit AZERTY choice always writes.
		if _, err := os.Lstat(kb); err == nil && !azerty {
			e.say(i18n.T("hypr/keyboard.lua already exists, keeping it"))
		} else {
			e.sayf(i18n.T("seeding keyboard layout %q variant %q options %q (from %s) into hypr/keyboard.lua"),
				e.f.kbLayout, e.f.kbVariant, e.f.kbOptions, src)
			if !e.dry {
				content := "-- keyboard layout, carried over by the installer. edits here stick.\n" +
					"hl.config({\n    input = {\n        kb_layout = \"" + lua(e.f.kbLayout) + "\",\n" +
					"        kb_variant = \"" + lua(e.f.kbVariant) + "\",\n" +
					"        kb_options = \"" + lua(e.f.kbOptions) + "\",\n    },\n})\n"
				if err := os.WriteFile(kb, []byte(content), 0o644); err != nil {
					return err
				}
			}
		}
	}

	// console + login-screen parity for an explicit AZERTY choice: the vt
	// keymap and SDDM's X11 greeter follow the desktop layout.
	if azerty {
		layout, keymap := "fr", "fr"
		if e.p.azertyBE {
			layout, keymap = "be", "be-latin1"
		}
		e.say(i18n.Tf("setting the console keymap to %s in /etc/vconsole.conf", keymap))
		if err := e.sudoSh(`install -Dm644 /dev/stdin /etc/vconsole.conf <<'EOF'
KEYMAP=` + keymap + `
EOF`); err != nil {
			return err
		}
		e.say(i18n.Tf("pointing the SDDM login screen at the %s layout via xorg.conf.d", layout))
		if err := e.sudoSh(`install -Dm644 /dev/stdin /etc/X11/xorg.conf.d/00-keyboard.conf <<'EOF'
Section "InputClass"
        Identifier "system-keyboard"
        MatchIsKeyboard "on"
        Option "XkbLayout" "` + layout + `"
EndSection
EOF`); err != nil {
			return err
		}
	}

	// the published loader still flags missing optional drop-ins in the
	// config-error overlay, so stub them until the searchpath fix ships. These
	// are that loader's own drop-ins, so they are skipped entirely for a
	// compositor whose config tree is not theirs: a niri install would
	// otherwise grow a stray hypr tree nothing ever reads.
	stubs := []struct{ rel, content string }{
		{".config/hypr/monitors_user.lua", "-- hand-pinned displays, see monitors_user.lua.example. pins here win.\n"},
		{".config/hypr/user.lua", "-- your hyprland overrides. loaded last, never touched by updates.\n"},
		{".config/hypr/theme.lua", "-- owned by ryoku settings: applying a theme replaces this file.\n"},
		{".config/hypr/settings.lua", "-- owned by ryoku settings, regenerated by the hub.\n"},
		{".config/hypr/modules/private.lua", "-- optional private module, yours to fill in.\n"},
		{".config/hypr/ghosttype.lua", "-- owned by ghosttype when installed.\n"},
	}
	own := ".config/" + wm.ConfigDir(e.p.compositor) + "/"
	for _, s := range stubs {
		if !strings.HasPrefix(s.rel, own) {
			continue
		}
		if e.dry {
			e.say(i18n.Tf("DRYRUN: stub ~/%s if absent", s.rel))
			continue
		}
		p := filepath.Join(e.f.homeDir, s.rel)
		if _, err := os.Lstat(p); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(s.content), 0o644); err != nil {
			return err
		}
		e.say(i18n.Tf("stubbed ~/%s", s.rel))
	}

	seeds := []struct {
		src, dst string
		dir      bool
		ifAbsent bool
	}{
		{"ryoku/assets/brand", ".local/share/ryoku/assets/brand", true, false},
		{"ryoku/assets/wallpapers", "Pictures/Wallpapers", true, true},
		{"ryoku/apps/npm/npmrc", ".npmrc", false, true},
		{"ryoku/apps/nvim/ryoku-nvim.desktop", ".local/share/applications/ryoku-nvim.desktop", false, false},
		// no mimeapps.list seed: the default-app map belongs to the vendor layer
		// (/usr/share/applications/mimeapps.list, laid by ryoku-desktop or by
		// deploy.sh), never to ~/.config/mimeapps.list, which is where the user's
		// own "Set as default" picks live.
	}
	for _, s := range seeds {
		src := filepath.Join(e.payload, s.src)
		dst := filepath.Join(e.f.homeDir, s.dst)
		if e.dry {
			e.say(i18n.Tf("DRYRUN: seed %s -> ~/%s", s.src, s.dst))
			continue
		}
		if err := seedPath(src, dst, s.dir, s.ifAbsent); err != nil {
			return fmt.Errorf("seed %s: %w", s.dst, err)
		}
		e.say(i18n.Tf("seeded ~/%s", s.dst))
	}
	return e.cmd("", nil, "systemctl", "--user", "daemon-reload")
}

func stepAUR(e *engine) error {
	if !e.p.aur {
		e.say(i18n.T("AUR extras skipped by choice; the wallpaper daemon is a package depend and is unaffected"))
		return nil
	}
	helper := e.f.aurHelper
	if helper == "" {
		e.say(i18n.T("no AUR helper found, bootstrapping yay-bin"))
		tmp, err := os.MkdirTemp("", "ryoku-yay-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		if err := e.cmd(tmp, nil, "git", "clone", "https://aur.archlinux.org/yay-bin.git"); err != nil {
			return err
		}
		// build and install separately: makepkg -i runs a plain interactive
		// sudo, which on a lapsed credential prompts on /dev/tty over the TUI
		// and hangs there; the engine's sudo -n fails loudly instead.
		if err := e.cmd(filepath.Join(tmp, "yay-bin"), nil, "makepkg", "--noconfirm"); err != nil {
			return err
		}
		built, _ := filepath.Glob(filepath.Join(tmp, "yay-bin", "*.pkg.tar.zst"))
		if len(built) == 0 {
			return errors.New(i18n.T("yay-bin build produced no package"))
		}
		if err := e.sudo(append([]string{"pacman", "-U", "--noconfirm"}, built...)...); err != nil {
			return err
		}
		helper = "yay"
	}
	var failed []string
	for _, p := range aurPkgs {
		// --sudoflags=-n, same reason: the helper's own sudo must fail into
		// the log, never prompt over the TUI. yay and paru both take it.
		if err := e.cmd("", nil, helper, "-S", "--needed", "--noconfirm", "--sudoflags=-n", p); err != nil {
			failed = append(failed, p)
			e.say(i18n.Tf("warning: AUR build failed for %s (continuing)", p))
		}
	}
	if len(failed) > 0 {
		e.say(i18n.Tf("AUR packages that did not install: %s", strings.Join(failed, " ")))
		e.say(i18n.Tf("re-run later with: %s -S %s", helper, strings.Join(failed, " ")))
	}
	return nil
}

func stepFish(e *engine) error {
	if !e.p.fish {
		e.say(i18n.T("keeping your current login shell"))
		return nil
	}
	if err := e.sudo("usermod", "-s", "/usr/bin/fish", e.f.username); err != nil {
		return err
	}
	e.recordRestore("sudo usermod -s " + e.f.userShell + " " + e.f.username)
	return nil
}

func stepDoctor(e *engine) error {
	// doctor converges snapper (btrfs only), NVIDIA modeset, greeter perms,
	// session components. findings are advice, not failure.
	ryoku := e.ryokuBin()
	if ryoku == "" {
		ryoku = "ryoku"
	}
	if err := e.cmd("", nil, ryoku, "doctor"); err != nil {
		e.say(i18n.T("note: ryoku doctor reported findings (see above); the install itself is done"))
	}
	return nil
}

func stepVerify(e *engine) error {
	if e.dry {
		e.say(i18n.T("DRYRUN: verify [ryoku] repo, packages, session files"))
		return nil
	}
	var bad []string
	check := func(ok bool, what string) {
		if ok {
			e.say(gCheck + " " + what)
		} else {
			bad = append(bad, what)
			e.say(gBad + " " + what)
		}
	}
	if e.d().fromSource {
		check(e.ryokuBin() != "", i18n.T("ryoku CLI built and installed"))
		_, err := os.Stat(filepath.Join(e.f.homeDir, ".local/bin/ryoku-shell"))
		check(err == nil, i18n.T("ryoku-shell daemon built"))
	} else {
		conf, _ := os.ReadFile("/etc/pacman.conf")
		check(strings.Contains(string(conf), "[ryoku]"), i18n.T("[ryoku] repository in /etc/pacman.conf"))
		check(pacmanHas("ryoku-keyring"), i18n.T("ryoku-keyring package installed"))
		check(pacmanHas("ryoku-desktop"), i18n.T("ryoku-desktop package installed"))
		check(has("ryoku"), i18n.T("ryoku CLI on PATH"))
		st, err := os.Stat("/usr/share/ryoku/config")
		check(err == nil && st.IsDir(), i18n.T("base config tree at /usr/share/ryoku/config"))
	}
	var err error
	cdir := wm.ConfigDir(e.p.compositor)
	_, err = os.Stat(filepath.Join(e.f.homeDir, ".config", cdir))
	check(err == nil, i18n.Tf("%s config materialized in ~/.config/%s", e.p.compositor, cdir))
	_, err = os.Stat(filepath.Join("/usr/share/wayland-sessions", e.p.compositor+".desktop"))
	check(err == nil, i18n.Tf("%s wayland session registered", e.p.compositor))
	// caps, not state: this runs before the new session is up, and caps answers
	// without a live compositor (state would falsely fail here).
	check(e.providerAnswers(), i18n.T("window-manager provider responds"))
	if e.p.switchDM {
		check(unitEnabled("system", "sddm.service"), i18n.T("sddm.service enabled"))
	}
	if e.p.switchDM && e.p.greeter {
		if theme := effectiveSDDMTheme(); theme != "" && theme != "ryoku" {
			e.say(gWarn + " " + i18n.Tf("an SDDM drop-in still selects greeter theme %s; check /etc/sddm.conf.d", theme))
		}
	}
	if e.f.hasNvidia && e.f.secureBoot && !e.p.nvidia {
		e.say(gWarn + " " + i18n.T("Secure Boot is on, so the proprietary NVIDIA driver was skipped: unsigned DKMS modules are rejected at boot."))
		e.say(i18n.T("To switch later, disable Secure Boot in firmware or sign the kernel and modules (sbctl), then re-run this installer."))
	}
	// matugen is a hard ryoku-desktop depend on Arch, so a miss means the desktop
	// set install is broken. Debian does not package it: warn instead of failing.
	if e.d().local("matugen") == "" {
		if !has("matugen") {
			e.say(gWarn + " " + i18n.T("matugen is not packaged here: wallpaper palettes stay at their defaults"))
		}
	} else {
		check(has("matugen"), i18n.T("matugen palette generator (colors follow the wallpaper)"))
	}
	if !has("ryogami") {
		e.say(gWarn + " " + i18n.T("ryogami missing: the wallpaper will not paint until it installs (ryoku doctor retries it)"))
	}
	if e.p.devtools {
		check(has("go"), i18n.T("go toolchain on PATH (ryoku recovery rebuilds from source)"))
	} else {
		e.say(gWarn + " " + i18n.T("developer toolchain skipped: ryoku recovery needs go; install with: sudo pacman -S go"))
	}
	if e.p.omarchy {
		conf2, _ := os.ReadFile("/etc/pacman.conf")
		if omarchyStanzaRe.Match(conf2) {
			e.say(gWarn + " " + i18n.T("the [omarchy] repository is still in /etc/pacman.conf"))
		}
	}
	if len(bad) > 0 {
		return errors.New(i18n.Tf("%d check(s) failed: %s", len(bad), strings.Join(bad, "; ")))
	}
	e.say(i18n.T("all checks passed"))
	return nil
}

// ---- fs helpers ----

func copyTree(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		tgt, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(tgt, dst)
	case info.IsDir():
		if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
			return err
		}
		ents, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, ent := range ents {
			if err := copyTree(filepath.Join(src, ent.Name()), filepath.Join(dst, ent.Name())); err != nil {
				return err
			}
		}
		return nil
	default:
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	}
}

// seedPath copies src to dst. dirs merge without overwriting when ifAbsent
// (user wallpapers win); files honor ifAbsent per file.
func seedPath(src, dst string, dir, ifAbsent bool) error {
	if !dir {
		if ifAbsent {
			if _, err := os.Lstat(dst); err == nil {
				return nil
			}
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return copyTree(src, dst)
	}
	ents, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, ent := range ents {
		s, d := filepath.Join(src, ent.Name()), filepath.Join(dst, ent.Name())
		if ifAbsent {
			if _, err := os.Lstat(d); err == nil {
				continue
			}
		} else if !ent.IsDir() {
			os.Remove(d)
		}
		if err := copyTree(s, d); err != nil {
			return err
		}
	}
	return nil
}
