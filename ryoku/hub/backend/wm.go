package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	wm "ryoku-wm"
)

// ryoku-hub wm is the neutral compositor-switch backend for the Hub. It answers
// two questions and mutates nothing: which providers exist and their state
// (list), and what a switch to one would cost (preview). The switch itself is
// `ryoku wm use <name>`, a reversible package transaction the Hub launches, so
// there is never a second package path with different rollback semantics.
//
//	wm list             every provider: name, package, installed, active, caps
//	wm preview <name>   the honest switch report, degrading when the target's
//	                    provider binary is absent the way `ryoku wm use` does

func runWm(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("wm needs list|preview")
	}
	switch args[0] {
	case "list":
		return wmList()
	case "preview":
		if len(args) < 2 {
			return fmt.Errorf("wm preview needs a name")
		}
		return wmPreview(args[1])
	default:
		return fmt.Errorf("wm needs list|preview")
	}
}

func wmPackage(name string) string { return "ryoku-desktop-" + name }

// wmDeployed reports whether a provider works without its package: its binary
// answers caps, its config tree exists, and its greeter session is real (the
// compositor is installed and a session entry exists). All three, because a
// checkout deploys the first two and the switch would then promise a session
// the greeter cannot boot.
func wmDeployed(name string) bool {
	if _, err := wm.OpenNamed(name).Caps(); err != nil {
		return false
	}
	dir := wm.ConfigDir(name)
	if dir == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(configHome(), dir)); err != nil {
		return false
	}
	return wm.SessionReady(name)
}

// wmProvider is one row of `wm list`. Package and configDir travel with the row
// so the Hub can build the switch and the keep-or-remove cleanup from data,
// never from a compositor name spelled in QML.
type wmProvider struct {
	Name      string `json:"name"`
	Package   string `json:"package"`
	ConfigDir string `json:"configDir"`
	Installed bool   `json:"installed"`
	Available bool   `json:"available"`
	Reason    string `json:"reason"`
	// Deployed reports a provider usable without its package, which is what a
	// checkout leaves behind.
	Deployed bool     `json:"deployed"`
	Active   bool     `json:"active"`
	Caps     *wm.Caps `json:"caps,omitempty"`
}

func wmList() error {
	detection := wm.Detect()
	out := []wmProvider{}
	for _, name := range wm.Providers() {
		pkg := wmPackage(name)
		available := pkgAvailable(pkg)
		p := wmProvider{
			Name:      name,
			Package:   pkg,
			ConfigDir: wm.ConfigDir(name),
			Installed: pkgInstalled(pkg),
			Available: available,
			Deployed:  wmDeployed(name),
			Active:    detection.Live && name == detection.Name,
		}
		if !available {
			p.Reason = pkgUnavailableReason(pkg)
		}
		// The manifest is present only when the provider binary is, so a box
		// that has never installed a compositor still lists it as a target.
		if caps, err := wm.OpenNamed(name).Caps(); err == nil {
			p.Caps = &caps
		}
		out = append(out, p)
	}
	return printJSON(out)
}

// wmPreviewReport is the honest switch report the confirmation sheet renders.
// exact is false when the target's provider binary is absent: the dry run could
// not run, so unhonored is empty and the Hub says "install the package to see
// the exact list", the same degradation `ryoku wm use` prints.
type wmPreviewReport struct {
	Target    string `json:"target"`
	Active    string `json:"active"`
	Package   string `json:"package"`
	ConfigDir string `json:"configDir"`
	Installed bool   `json:"installed"`
	Available bool   `json:"available"`
	// Deployed reports that the provider works without its package.
	Deployed     bool           `json:"deployed"`
	KeybindCount int            `json:"keybindCount"`
	Exact        bool           `json:"exact"`
	Provider     string         `json:"provider,omitempty"`
	Unhonored    []wm.Unhonored `json:"unhonored"`
	ReloadNeeded bool           `json:"reloadNeeded"`
	// Reclaim is what leaving the active compositor for this target would remove:
	// the packages, their count and their total size, or Removable false when
	// nothing is installed to reclaim. It is what turns the sheet's keep-or-
	// remove question from a guess about the meta-package into the truth about
	// the compositor's own packages. Absent when there is no compositor to leave.
	Reclaim *wm.ReclaimSet `json:"reclaim,omitempty"`
}

func wmPreview(name string) error {
	if !wmKnown(name) {
		return fmt.Errorf("unknown compositor %q", name)
	}
	store := desktopStorePath()
	out := wmPreviewReport{
		Target:       name,
		Active:       wm.Detect().Name,
		Package:      wmPackage(name),
		ConfigDir:    wm.ConfigDir(name),
		Installed:    pkgInstalled(wmPackage(name)),
		Available:    pkgAvailable(wmPackage(name)),
		Deployed:     wmDeployed(name),
		KeybindCount: storeKeybindCount(store),
		Unhonored:    []wm.Unhonored{},
	}
	// Best-effort: the target may not be installed yet, and DryRun then errors
	// rather than authoring its config. The report degrades, it does not fail.
	if rep, err := wm.OpenNamed(name).DryRun(store); err == nil {
		out.Exact = true
		out.Provider = rep.Provider
		out.ReloadNeeded = rep.ReloadNeeded
		if len(rep.Unhonored) > 0 {
			out.Unhonored = rep.Unhonored
		}
	}
	// What leaving the active compositor reclaims. Computed for the outgoing
	// compositor, not the target, and best-effort: a box without its package
	// manager (or with no active compositor) simply carries no reclaim block and
	// the sheet reads that as nothing to remove.
	if out.Active != "" && out.Active != name {
		if rs, err := wm.Reclaim(out.Active, name); err == nil {
			out.Reclaim = &rs
		}
	}
	return printJSON(out)
}

func wmKnown(name string) bool {
	for _, p := range wm.Providers() {
		if p == name {
			return true
		}
	}
	return false
}

// pkgAvailable reports whether the host's configured repositories provide a
// package, so an unavailable target is shown honestly instead of offering a
// switch whose transaction would fail.
func pkgAvailable(pkg string) bool {
	return exec.Command("ryoku-host", "pkg", "available", pkg).Run() == nil
}

func pkgUnavailableReason(pkg string) string {
	out, err := exec.Command("ryoku-host", "pkg", "why", pkg).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// storeKeybindCount is the neutral keybind carry-over: the count `ryoku wm use`
// prints, so the sheet says the same number. Zero when the store or field is
// absent.
func storeKeybindCount(store string) int {
	raw, err := os.ReadFile(store)
	if err != nil {
		return 0
	}
	var doc struct {
		Desktop struct {
			Keybinds       json.RawMessage `json:"keybinds"`
			KeybindRebinds json.RawMessage `json:"keybindRebinds"`
		} `json:"desktop"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return 0
	}
	return jsonLen(doc.Desktop.Keybinds) + jsonLen(doc.Desktop.KeybindRebinds)
}

// jsonLen counts entries in a JSON array or object, or 0 for anything else.
func jsonLen(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		return len(arr)
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(raw, &obj) == nil {
		return len(obj)
	}
	return 0
}
