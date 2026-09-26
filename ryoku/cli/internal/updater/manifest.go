package updater

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ryoku-cli/internal/ryokumanifest"
	"ryoku-cli/internal/sys"
)

// The box side of the control manifest. The channel serves manifest.json beside
// release.json; a box caches it, keeps the last one it reconciled against, and
// moves itself toward the new one. Additions land on the next update; removals
// from the release never uninstall anything, and a name the user deleted is
// never put back -- the provisioned ledger is what tells those apart.

// manifestFetchTTL matches release.json's: status polls both, and a publish
// within ten minutes of the last read is not worth a second request.
const manifestFetchTTL = releaseFetchTTL

// ManifestPath is where the box keeps the manifest it last reconciled against.
// A var so tests point it at a fixture.
var ManifestPath = func() string {
	return filepath.Join(sys.StateDir(), "manifest.json")
}

// FetchManifest reads what the box's channel serves, or an error when the box
// is not a packaged install on a published channel, the fetch failed, and
// nothing is cached.
func FetchManifest() (ryokumanifest.Manifest, error) {
	channel := sys.PackagedChannel()
	if channel == "" {
		return ryokumanifest.Manifest{}, fmt.Errorf("not a packaged channel")
	}
	url := sys.ChannelURL(channel) + "/manifest.json"
	b := fetchCached("manifest-"+sanitize(channel)+".json", url, manifestFetchTTL)
	if b == nil {
		return ryokumanifest.Manifest{}, fmt.Errorf("manifest for channel %s unreachable", channel)
	}
	var m ryokumanifest.Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return ryokumanifest.Manifest{}, fmt.Errorf("manifest for channel %s: %w", channel, err)
	}
	if m.Schema > ryokumanifest.Schema {
		return m, fmt.Errorf("manifest schema %d is newer than this ryoku understands (%d); run a full update first", m.Schema, ryokumanifest.Schema)
	}
	return m, nil
}

// Applied is the box's saved baseline: the manifest it last converged to, and
// which of that manifest's names were installed when it was saved. The
// presence set is what makes a later removal legible -- a name present at the
// baseline and gone now was deleted by the user -- without a ledger that has to
// record every package on first run.
type Applied struct {
	Manifest ryokumanifest.Manifest `json:"manifest"`
	Present  []string               `json:"present"`
}

// LoadApplied returns the box's baseline, or nil on first use.
func LoadApplied() *Applied {
	b, err := os.ReadFile(ManifestPath())
	if err != nil {
		return nil
	}
	var a Applied
	if json.Unmarshal(b, &a) != nil || a.Manifest.Schema == 0 {
		return nil
	}
	return &a
}

// PresentSet returns the baseline's installed names as a set.
func (a *Applied) PresentSet() map[string]bool {
	set := map[string]bool{}
	if a == nil {
		return set
	}
	for _, n := range a.Present {
		set[n] = true
	}
	return set
}

// SaveApplied records the manifest and the box's present names as the new
// baseline the next update diffs against.
func SaveApplied(m ryokumanifest.Manifest, present []string) error {
	sort.Strings(present)
	b, err := json.MarshalIndent(Applied{Manifest: m, Present: present}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(ManifestPath()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(ManifestPath(), append(b, '\n'), 0o644)
}

// StickyPresent is the presence set a new baseline records: every parity name
// installed now, plus every name an earlier baseline already recorded as
// delivered. A user removal is therefore permanent in the baseline's eyes: the
// next reconcile reads the gone name as UserGone (stays deleted), not as
// never-delivered (re-install). Re-saving the snapshot from the live box
// alone forgets each removal exactly one run after it happens, and the run
// after that reinstalls the package -- the loop #268 reported.
func StickyPresent(prev *Applied, m ryokumanifest.Manifest, installed map[string]bool) []string {
	set := map[string]bool{}
	if prev != nil {
		for _, n := range prev.Present {
			set[n] = true
		}
	}
	for _, n := range PresentNames(m, installed) {
		set[n] = true
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Plan is one reconciliation of the box's package set against a manifest.
type Plan struct {
	Install   []string // a newer manifest adds the name, absent here, not a user removal
	AUR       []string // same, but arriving best-effort through the AUR helper
	UserGone  []string // present at the baseline and gone now: the user removed it, it stays gone
	Retired   []string // the release dropped the name; it stays installed, reported only
	Untracked []string // first run only: manifest names the box lacks, recorded not judged
}

// ownLanes is the manifest's parity set: what a box is expected to have
// without the user choosing it. Hardware sections are profile-scoped (an intel
// box has no reason to carry amd-ucode) and compositor packages belong to the
// switch, so neither lane appears here.
func ownLanes(m ryokumanifest.Manifest) [][]string {
	return [][]string{m.Base, m.Dev, m.FirstParty, m.AUR, m.Provisioned}
}

// convergeLanes is the subset this package's reconciler moves. The provisioned
// lane is reconcileShippedApps' deliver-once business (it also re-marks apps
// that arrived as dependencies), so installing it here would be a second
// pacman transaction over the same names.
func convergeLanes(m ryokumanifest.Manifest) [][]string {
	return [][]string{m.Base, m.Dev, m.FirstParty, m.AUR}
}

func nameSet(lanes [][]string) map[string]bool {
	set := map[string]bool{}
	for _, lane := range lanes {
		for _, n := range lane {
			if ryokumanifest.IsOptIn(n) {
				continue // a gated reconciler or the user owns it, not the manifest
			}
			set[n] = true
		}
	}
	return set
}

func manifestNames(m ryokumanifest.Manifest) map[string]bool {
	return nameSet(ownLanes(m))
}

// convergeNames is the set this reconciler moves: the converge lanes minus the
// deliver-once apps. Many Apps() names are ALSO named by base.packages (the
// installer's pacstrap list), and the lane table would otherwise let the
// converge pass re-install what reconcileShippedApps just honoured as a user
// deletion. The provisioned lane owns those names; the manifest never does.
func convergeNames(m ryokumanifest.Manifest) map[string]bool {
	names := nameSet(convergeLanes(m))
	for _, n := range m.Provisioned {
		delete(names, n)
	}
	return names
}

// PresentNames is the baseline snapshot for a box: the manifest's parity names
// it has installed right now. A later run diffs against it to tell a user
// removal from a name that was never delivered.
func PresentNames(m ryokumanifest.Manifest, installed map[string]bool) []string {
	var out []string
	for n := range manifestNames(m) {
		if installed[n] {
			out = append(out, n)
		}
	}
	return out
}

// PlanManifest decides how a packaged box converges to the served manifest
// (next). It acts only on the converge lanes -- base, dev, first-party, AUR;
// the provisioned apps are reconcileShippedApps' deliver-once business.
//
// The baseline (prev, plus prevPresent: which parity names were installed when
// prev was recorded) is what makes a user's removal legible without a ledger
// that floods on first run:
//   - a name the manifest wants, absent now, that WAS present at the baseline
//     was deleted by the user, so it stays gone;
//   - a name absent now that was NOT present at the baseline was simply never
//     delivered to this box -- the ISO/manifest divergence -- so it is delivered
//     now. This self-heals a box that has been missing a package the release
//     always named, not only one this very release added.
//
// On the first run there is no baseline to read a removal from, so nothing is
// installed or judged: the run records one and reports the gap it now sees.
func PlanManifest(prev *ryokumanifest.Manifest, prevPresent map[string]bool, next *ryokumanifest.Manifest, installed map[string]bool) Plan {
	var p Plan
	if next == nil {
		return p
	}
	if prev == nil {
		for n := range manifestNames(*next) {
			if !installed[n] {
				p.Untracked = append(p.Untracked, n)
			}
		}
		sort.Strings(p.Untracked)
		return p
	}
	names := convergeNames(*next)
	aur := map[string]bool{}
	for _, n := range next.AUR {
		aur[n] = true
	}
	for n := range names {
		switch {
		case installed[n]:
		case prevPresent[n]:
			p.UserGone = append(p.UserGone, n)
		default:
			if aur[n] {
				p.AUR = append(p.AUR, n)
			} else {
				p.Install = append(p.Install, n)
			}
		}
	}
	// Names the release retired: still installed, reported, never removed.
	old := convergeNames(*prev)
	for n := range old {
		if !names[n] && installed[n] {
			p.Retired = append(p.Retired, n)
		}
	}
	for _, s := range []*[]string{&p.Install, &p.AUR, &p.UserGone, &p.Retired} {
		sort.Strings(*s)
	}
	return p
}

// VerifyReport answers `ryoku verify`: is this box the machine the release
// describes? It reads the same presence baseline the reconciler converges
// against, so the two never disagree about what counts as a user removal.
type VerifyReport struct {
	Release  string   `json:"release"`
	Version  string   `json:"version"`
	Commit   string   `json:"commit"`
	Missing  []string `json:"missing"`  // manifest-named, absent, never present at the baseline: not delivered
	UserGone []string `json:"userGone"` // manifest-named, absent, present at the baseline: the user removed it, respected
	Extra    []string `json:"extra"`    // explicitly installed, named by no manifest lane: the user added it
	NotSaved bool     `json:"notSaved"` // the box has never reconciled against a manifest
}

// Verify computes the box-vs-manifest diff. installed is `pacman -Qq`, explicit
// is `pacman -Qqe`, present is the baseline's installed set (nil before the
// first reconcile, when every absent name reads as not-yet-delivered). A box's
// honest additions (extra) are listed so two machines can be compared, never
// removed.
func Verify(m ryokumanifest.Manifest, installed, explicit, present map[string]bool) VerifyReport {
	r := VerifyReport{Release: m.Release, Version: m.Version, Commit: m.Commit}
	own := manifestNames(m)
	for n := range own {
		switch {
		case installed[n]:
		case present[n]:
			r.UserGone = append(r.UserGone, n)
		default:
			r.Missing = append(r.Missing, n)
		}
	}
	for n := range explicit {
		if !own[n] {
			r.Extra = append(r.Extra, n)
		}
	}
	for _, s := range []*[]string{&r.Missing, &r.UserGone, &r.Extra} {
		sort.Strings(*s)
	}
	return r
}

// PacmanInstalled / PacmanExplicit read the box. They are vars so the
// reconciler and `ryoku verify` are testable without pacman.
var (
	PacmanInstalled = func() (map[string]bool, error) { return pacmanSet("-Qq") }
	PacmanExplicit  = func() (map[string]bool, error) { return pacmanSet("-Qqe") }
)

func pacmanSet(flag string) (map[string]bool, error) {
	out, err := sys.RunOut("pacman", flag)
	if err != nil {
		return nil, fmt.Errorf("pacman %s: %w", flag, err)
	}
	set := map[string]bool{}
	for _, ln := range strings.Split(out, "\n") {
		if n := strings.TrimSpace(ln); n != "" {
			set[n] = true
		}
	}
	return set, nil
}
