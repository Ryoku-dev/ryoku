package doctor

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// ---- reconciler: stale dev/recovery residue ------------------------------------

// reconcileDevResidue clears home-installed Ryoku artifacts off a packaged box.
// deploy.sh (the dev loop, and `ryoku recovery`) installs binaries into
// ~/.local/bin and QML modules into ~/.local/lib/qt6/qml; both outrank the
// packaged copies on PATH and the QML import path, so once the box is back on
// the pacman channel the leftovers pin it to whatever vintage last deployed
// them and every later package update is silently shadowed. a checkout box
// (git channel) IS the dev loop: left alone.
//
// The binaries are only half of it. Every user unit the deploy pointed at
// ~/.local/bin (a drop-in today, a rewritten copy or a user_edits fork of one
// from older deploys, plus rashin and Prowl's whole home units) still shadows
// the packaged service. Clearing the binaries alone leaves the shell unit
// failing at its first ExecStartPre and the desktop comes up with no shell
// after the next login. Those units go back to the packaged ones in the same
// pass.

var (
	devPackageInstalled = func(name string) bool {
		return host.Default().Package([]string{"installed", name}) == host.ExitOK
	}
	devPackageOwner = func(path string) (string, error) {
		return host.Default().PackageOwner(path)
	}
	devPathExists  = sys.Exists
	devService     = func(args []string) int { return host.Default().Service(args) }
	reloadDevUnits = func(units []string) {
		_ = devService([]string{"--user", "daemon-reload"})
		_ = devService(append([]string{"--user", "reset-failed"}, units...))
	}
)

func reconcileDevResidue(checkOnly bool) recResult {
	if sys.ResolveRepo() != "" {
		return okRes(i18n.T("checkout box; home-deployed artifacts are the live desktop"))
	}
	if !devPackageInstalled("ryoku-desktop") {
		return okRes(i18n.T("not a packaged install"))
	}
	l := residueLayout{
		localBin:   filepath.Join(sys.Home(), ".local", "bin"),
		configHome: sys.ConfigHome(),
		editsDir:   sys.UserEditsDir(),
		baseDir:    sys.BaseConfigDir(),
		usrUnits:   "/usr/lib/systemd/user",
	}
	seen := map[string]bool{}
	twin := func(name string) bool {
		v, ok := seen[name]
		if !ok {
			v = packagedTwin(name)
			seen[name] = v
		}
		return v
	}

	var residue []string
	if qml := filepath.Join(sys.Home(), ".local", "lib", "qt6", "qml", "Ryoku"); sys.Exists(qml) {
		residue = append(residue, qml)
	}
	// every ~/.local/bin entry that shadows a Ryoku-packaged /usr/bin binary.
	// deploy.sh installs a wide, release-dependent set (shell, livewall, CLI,
	// hub, rashin, hardware helpers, app bins), so pacman is the manifest: a
	// home copy of anything a ryoku package ships is residue. A fixed name
	// list here once missed ryoku-livewall and the helpers, leaving a stale
	// player and tools shadowing every later update. Anything the packages
	// never shipped (a user's own script) has no ryoku-owned /usr/bin twin
	// and is left alone; a wrapper deliberately named after a packaged ryoku
	// tool is treated as residue too -- doctor owns converging the packaged
	// toolchain, and an intercepted tool is exactly the drift it heals.
	entries, _ := os.ReadDir(l.localBin)
	for _, e := range entries {
		if twin(e.Name()) {
			residue = append(residue, filepath.Join(l.localBin, e.Name()))
		}
	}
	if w := homeRyokuWrapper(l.configHome); w != "" {
		residue = append(residue, w)
	}
	heals := l.planDevUnits(twin)
	if len(residue) == 0 && len(heals) == 0 {
		return okRes(i18n.T("no home-deployed artifacts shadowing the packages"))
	}
	if checkOnly {
		what := residue
		for _, h := range heals {
			what = append(what, h.path)
		}
		return wouldRes(i18n.T("stale home-deployed artifacts shadow the packaged install: %s"), strings.Join(what, ", ")).
			withFix(i18n.T("ryoku doctor removes them; the packaged copies take over on the next reload"))
	}
	// report what could not be removed: a survivor keeps shadowing the packaged
	// install (Hyprland's autostart relaunches it by PATH at next login), so
	// claiming "removed" while it lives would hide the very drift this heals.
	var kept []string
	for _, p := range residue {
		if err := os.RemoveAll(p); err != nil {
			kept = append(kept, p)
		}
	}
	units, failed := l.applyDevUnits(heals)
	kept = append(kept, failed...)
	if len(units) > 0 {
		reloadDevUnits(units)
	}
	if len(kept) > 0 {
		return failRes(i18n.T("could not remove home-deployed artifact(s) still shadowing the packaged install: %s"), strings.Join(kept, ", ")).
			withFix(i18n.T("remove them by hand (check ownership/permissions), then `ryoku reload`"))
	}
	return fixedRes(i18n.T("removed %d home-deployed artifact(s) shadowing the packaged install and pointed %d user unit file(s) back at the packaged binaries; `ryoku reload` switches to the packaged shell"), len(residue), len(heals))
}

// packagedTwin reports whether a ~/.local/bin entry of this name shadows a
// /usr/bin binary a Ryoku package ships. ryogami is the one package outside the
// ryoku-* names that the dev deploy also builds into ~/.local/bin.
var packagedTwin = func(name string) bool {
	usr := "/usr/bin/" + name
	if !devPathExists(usr) {
		return false
	}
	owner, err := devPackageOwner(usr)
	if err != nil {
		return false
	}
	owner = strings.TrimSpace(owner)
	return strings.HasPrefix(owner, "ryoku") || owner == "ryogami"
}

// homeRyokuCall matches a shell function that runs the CLI from ~/.local/bin,
// not a ryoku-shell or ryoku-hub path that merely shares the prefix.
var homeRyokuCall = regexp.MustCompile(`\.local/bin/ryoku(\s|$)`)

// homeRyokuWrapper names the fish function the retired dev-switch installed to
// route `ryoku` to the home build. Once that build is cleared it turns every
// `ryoku` command into "Unknown command", update included, so it goes with it.
func homeRyokuWrapper(configHome string) string {
	p := filepath.Join(configHome, "fish", "functions", "ryoku.fish")
	if b, err := os.ReadFile(p); err == nil && homeRyokuCall.Match(b) {
		return p
	}
	return ""
}

// devBinDropIn is the drop-in deploy.sh writes to point a shipped unit's Exec
// lines at ~/.local/bin.
const devBinDropIn = "ryoku-dev-bin.conf"

// residueLayout is where a box keeps what the dev deploy laid, lifted out so
// the unit plan runs against a temp tree.
type residueLayout struct {
	localBin   string // ~/.local/bin
	configHome string // ~/.config
	editsDir   string // ~/.config/ryoku/user_edits
	baseDir    string // the packaged base config materialize lays
	usrUnits   string // /usr/lib/systemd/user
}

// unitHeal is one file to put back: relay set lays the packaged base copy over
// path; relay empty removes path. wants set repoints the .wants links that
// named the removed home unit at the packaged one, so the unit stays enabled.
type unitHeal struct {
	path, relay, wants string
}

func (l residueLayout) userUnits() string { return filepath.Join(l.configHome, "systemd", "user") }

// planDevUnits lists every user unit file that still runs a home build of a
// packaged binary: the deploy's drop-ins, user_edits forks of a deploy-rewritten
// unit (an older materialize read the rewrite as a hand edit) with the live
// copy they keep re-laying, and live units the deploy rewrote or wrote whole.
// A live unit with neither a base copy nor a packaged twin is the user's own
// and is left alone.
func (l residueLayout) planDevUnits(twin func(string) bool) []unitHeal {
	var heals []unitHeal
	planned := map[string]bool{}
	add := func(h unitHeal) {
		if !planned[h.path] {
			planned[h.path] = true
			heals = append(heals, h)
		}
	}
	dropIns, _ := filepath.Glob(filepath.Join(l.userUnits(), "*.d", devBinDropIn))
	for _, p := range dropIns {
		add(unitHeal{path: p})
	}
	forkRoot := filepath.Join(l.editsDir, "systemd", "user")
	_ = filepath.WalkDir(forkRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() || !l.execsDevBin(p, twin) {
			return nil
		}
		rel, _ := filepath.Rel(forkRoot, p)
		add(unitHeal{path: p})
		if base := filepath.Join(l.baseDir, "systemd", "user", rel); sys.Exists(base) {
			add(unitHeal{path: filepath.Join(l.userUnits(), rel), relay: base})
		}
		return nil
	})
	live, _ := os.ReadDir(l.userUnits())
	for _, e := range live {
		p := filepath.Join(l.userUnits(), e.Name())
		wholeHomeUnit := e.Name() == "ryoku-rashin.service" || e.Name() == "ryoku-prowl.service"
		if planned[p] || !e.Type().IsRegular() || (!wholeHomeUnit && !l.execsDevBin(p, twin)) {
			continue
		}
		if base := filepath.Join(l.baseDir, "systemd", "user", e.Name()); sys.Exists(base) {
			add(unitHeal{path: p, relay: base})
		} else if pkg := filepath.Join(l.usrUnits, e.Name()); sys.Exists(pkg) {
			add(unitHeal{path: p, wants: pkg})
		}
	}
	return heals
}

// execsDevBin reports whether a unit file's Exec lines run a ~/.local/bin build
// of a packaged binary, present or already cleared.
func (l residueLayout) execsDevBin(path string, twin func(string) bool) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(b), "\n") {
		key, val, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || !strings.HasPrefix(key, "Exec") {
			continue
		}
		fields := strings.Fields(strings.TrimLeft(val, "-@:+!"))
		if len(fields) > 0 && filepath.Dir(fields[0]) == l.localBin && twin(filepath.Base(fields[0])) {
			return true
		}
	}
	return false
}

// applyDevUnits carries out the plan and returns the live unit names it touched
// (for the reload) and the paths it could not fix.
func (l residueLayout) applyDevUnits(heals []unitHeal) (units, failed []string) {
	seen := map[string]bool{}
	for _, h := range heals {
		var err error
		if h.relay != "" {
			err = sys.CopyFile(h.relay, h.path)
		} else {
			err = os.Remove(h.path)
			if err == nil {
				_ = os.Remove(filepath.Dir(h.path)) // only when that left it empty
			}
		}
		if err == nil && h.wants != "" {
			err = l.repointWants(h.path, h.wants)
		}
		if err != nil {
			failed = append(failed, h.path)
			continue
		}
		rel, rerr := filepath.Rel(l.userUnits(), h.path)
		if rerr != nil || strings.HasPrefix(rel, "..") {
			continue // a user_edits fork, not a live unit
		}
		unit := strings.TrimSuffix(strings.Split(rel, string(filepath.Separator))[0], ".d")
		if !seen[unit] {
			seen[unit] = true
			units = append(units, unit)
		}
	}
	return units, failed
}

// repointWants moves every .wants link that named the removed home unit onto
// the packaged unit, keeping it enabled without asking systemctl to re-enable.
func (l residueLayout) repointWants(removed, pkg string) error {
	links, _ := filepath.Glob(filepath.Join(l.userUnits(), "*.wants", filepath.Base(removed)))
	for _, link := range links {
		if target, err := os.Readlink(link); err != nil || target != removed {
			continue
		}
		if err := os.Remove(link); err != nil {
			return err
		}
		if err := os.Symlink(pkg, link); err != nil {
			return err
		}
	}
	return nil
}
