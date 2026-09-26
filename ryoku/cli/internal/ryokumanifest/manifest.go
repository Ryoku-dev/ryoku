// Package ryokumanifest is the control manifest: what a Ryoku release is made
// of, as data. build-repo.sh generates it at publish time and serves it beside
// the channel's release.json, so a box can ask "what should I look like" and
// compare. It is never hand-authored and never hand-edited: the generator is
// the only writer, and the repo files it reads are the source of truth.
//
// The manifest states intent, not versions. First-party packages are pinned to
// the release that built them (one shared RYOKU_PKGVER, as ryoku-desktop's
// depends already do); distribution and AUR packages are named only, because
// the kernel and the AUR are not Ryoku's to move. That is the same two-lane
// rule docs/updates.md states for `ryoku update`.
package ryokumanifest

import "sort"

// Schema is the manifest format version. A box that reads a newer schema than
// it understands says so rather than reconciling against a partial contract.
const Schema = 1

// Manifest is one release's desired state.
type Manifest struct {
	Schema      int                 `json:"schema"`
	Release     string              `json:"release"` // release string, as release.json names it
	Version     string              `json:"version"` // shared RYOKU_PKGVER the first-party set is pinned to
	Commit      string              `json:"commit"`  // full SHA this manifest was generated from
	Channel     string              `json:"channel"` // stable | testing | v<tag>
	Date        string              `json:"date"`
	Base        []string            `json:"base"`                 // system/packages/base.packages
	Dev         []string            `json:"dev"`                  // system/packages/dev.packages
	Hardware    map[string][]string `json:"hardware,omitempty"`   // hardware.packages, per profile section
	AUR         []string            `json:"aur"`                  // system/packages/aur.packages
	FirstParty  []string            `json:"firstParty"`           // release/packages/*, pinned to Version
	Compositor  map[string][]string `json:"compositor,omitempty"` // provider -> its packages, from its caps
	Provisioned []string            `json:"provisioned"`          // apps doctor delivers once and never re-adds
}

// Names returns the manifest's full package-name set: every lane's names plus
// the provisioned apps, deduplicated and sorted. Reconcilers compare against
// this; the lane a name came from decides how a miss is treated.
func (m Manifest) Names() []string {
	set := map[string]bool{}
	add := func(names ...string) {
		for _, n := range names {
			if n != "" {
				set[n] = true
			}
		}
	}
	add(m.Base...)
	add(m.Dev...)
	for _, section := range m.Hardware {
		add(section...)
	}
	add(m.AUR...)
	add(m.FirstParty...)
	for _, pkgs := range m.Compositor {
		add(pkgs...)
	}
	add(m.Provisioned...)
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// provisionedByLane answers where a name belongs, and therefore what a missing
// one means: base/dev/first-party/compositor names reach a packaged box through
// `ryoku update`'s set or a hard depend (a miss there is a delivery bug), AUR
// names arrive best-effort post-install, and provisioned names are the doctor's
// deliver-once list, where a removal is the user's and stands.
const (
	LaneBase        = "base"
	LaneDev         = "dev"
	LaneHardware    = "hardware"
	LaneAUR         = "aur"
	LaneFirstParty  = "first-party"
	LaneCompositor  = "compositor"
	LaneProvisioned = "provisioned"
)

// Lane reports which lane a package name belongs to, and "" when the manifest
// does not name it at all. First-party and provisioned win over the wider
// sets: a name in both is delivered by the narrower, version-pinned route.
func (m Manifest) Lane(name string) string {
	switch {
	case contains(m.FirstParty, name):
		return LaneFirstParty
	case contains(m.Provisioned, name):
		return LaneProvisioned
	case contains(m.Base, name):
		return LaneBase
	case contains(m.Dev, name):
		return LaneDev
	case contains(m.AUR, name):
		return LaneAUR
	}
	for _, pkgs := range m.Compositor {
		if contains(pkgs, name) {
			return LaneCompositor
		}
	}
	for _, section := range m.Hardware {
		if contains(section, name) {
			return LaneHardware
		}
	}
	return ""
}

// DeliverableOnce names the provisioned apps: the set the doctor installs once
// and never puts back. This is the shipped-apps rule, widened from a hardcoded
// Go list to whatever the release's manifest says.
func (m Manifest) DeliverableOnce() []string { return m.Provisioned }

// OptIn names packages the release publishes but no box must carry by
// default: a hardware-gated reconciler or an explicit user choice owns them,
// so the manifest's converge lanes never install them. asusctl and
// dualsensectl belong to an ASUS laptop or a DualSense pad (reconcileAsusAura
// and the controller docs deliver them); pam-fprint-grosshack belongs to a
// machine with a fingerprint reader (reconcileFingerprintModule);
// limine-snapper-sync belongs to a btrfs+Limine box (the snapshot offer and
// the doctor); ryoku-palette-bridge is a Ryogami setting the user turns on.
// Without this list the control manifest reads every [ryoku] PKGBUILD as a
// requirement and offers to install all of them on every machine.
var OptIn = []string{
	"asusctl",
	"dualsensectl",
	"pam-fprint-grosshack",
	"limine-snapper-sync",
	"ryoku-palette-bridge",
}

// IsOptIn reports whether a package name belongs to no box by default.
func IsOptIn(name string) bool { return contains(OptIn, name) }

func contains(set []string, name string) bool {
	for _, n := range set {
		if n == name {
			return true
		}
	}
	return false
}

// App is a package the release delivers once and never puts back: the user's
// removal stands. This is the doctor's shipped-apps table, kept here so the
// manifest generator and the reconciler read the same list rather than one
// being a copy of the other.
type App struct {
	Pkg  string `json:"pkg"`
	What string `json:"what"`
}

// Membership rule: a standalone application whose absence costs only itself.
// Tools the shell calls by name (grim, playerctl, matugen, cava, mpv for the
// launcher's radio, the pill's OCR/capture backends) stay hard depends, because
// losing them breaks a Ryoku surface the user never touched. Ryotunes has its
// own official-release install/reconciliation path.
func Apps() []App {
	return []App{
		{"kitty", "the default terminal (Settings > App Overrides repoints the terminal role)"},
		{"fish", "the optional interactive shell"},
		{"blesh", "Bash line editor"},
		{"starship", "the shell prompt"},
		{"fastfetch", "system summary (launcher, RyoStore covers)"},
		{"yazi", "terminal file manager"},
		{"neovim", "the shipped editor"},
		{"nautilus", "the graphical file manager"},
		{"nautilus-python", "the Ryoku stash actions in Nautilus' right-click menu"},
		{"ryomotion", "the screen-demo recorder and editor"},
		{"waifu2x-ncnn-vulkan", "AI upscale behind ryoshot Beautify HD and ryowalls Enhance"},
		{"pavucontrol", "the GUI mixer the bar's Open audio button launches"},
		{"songrec", "Recognize Music in the launcher"},
		{"openrgb", "keyboard and mouse lighting (Settings > Appearance > Lighting)"},
		{"gamescope", "the nested gaming micro-compositor"},
		{"gamemode", "the gaming performance governor Steam invokes"},
		{"mangohud", "the in-game FPS and frametime overlay"},
	}
}
