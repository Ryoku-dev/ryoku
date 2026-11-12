package doctor

import (
	"encoding/json"
	"math"
	"path/filepath"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
	wm "ryoku-wm"
)

// performance.json's look switches each moved to the one place it is set now:
// reduce motion and the motion speed to shell.json theme.motion (Animations).
// disableBlur and disableShadows turned off both the window manager's blur or
// shadows and the shell's own glass or shadows, so a true one turns off both
// homes in Window style: desktop.json for windows, shell.json theme.effects for
// panels and the dock. liveWallpaper60 and pauseLiveWallpaperWhenFullscreen
// never had a reader. A value the user set is carried to its new home first,
// then every retired key is dropped. Idempotent.

var retiredPerfKeys = []string{"reduceMotion", "motionSpeed", "disableBlur", "disableShadows", "liveWallpaper60", "pauseLiveWallpaperWhenFullscreen"}

// applyWindowStyle re-renders the compositor from desktop.json so a carried
// blur or shadows switch reaches the running session.
var applyWindowStyle = func(store string) { _, _ = wm.Open().Apply(store) }

func reconcileRetiredPerfKeys(checkOnly bool) recResult {
	dir := filepath.Join(sys.ConfigHome(), "ryoku")
	perfPath := filepath.Join(dir, "performance.json")
	perf, exists, err := readRawStore(perfPath)
	if err != nil {
		return warnRes(i18n.T("performance.json does not parse (%v); the shell falls back to defaults"), err).
			withFix(i18n.T("delete %s to start fresh"), perfPath)
	}
	if !exists {
		return okRes(i18n.T("no performance.json yet"))
	}
	present := false
	for _, k := range retiredPerfKeys {
		if _, ok := perf[k]; ok {
			present = true
		}
	}
	if !present {
		return okRes(i18n.T("performance.json carries no retired look switches"))
	}
	if checkOnly {
		return wouldRes(i18n.T("performance.json still carries retired look switches")).
			withFix(i18n.T("ryoku doctor carries them into Animations and Window style, then drops them"))
	}
	shellPath := filepath.Join(dir, "shell.json")
	if err := carryPerfMotion(shellPath, perf); err != nil {
		return failRes(i18n.T("could not carry the motion settings into shell.json: %v"), err)
	}
	if err := carryPerfShellEffects(shellPath, perf); err != nil {
		return failRes(i18n.T("could not carry the panels' glass and shadows into shell.json: %v"), err)
	}
	desktopPath := filepath.Join(dir, "desktop.json")
	styled, err := carryPerfWindowStyle(desktopPath, perf)
	if err != nil {
		return failRes(i18n.T("could not carry blur and shadows into desktop.json: %v"), err)
	}
	for _, k := range retiredPerfKeys {
		delete(perf, k)
	}
	if err := writeRawStore(perfPath, perf); err != nil {
		return failRes(i18n.T("could not write %s: %v"), perfPath, err)
	}
	if styled {
		applyWindowStyle(desktopPath)
	}
	return fixedRes(i18n.T("moved the retired performance.json look switches into Animations and Window style"))
}

// carryPerfMotion folds reduceMotion=true into theme.motion.reduce, and a
// non-default motionSpeed into theme.motion.scale while that is still the
// shipped 1. A box with no shell.json has never run the shell, so nothing is
// carried there.
func carryPerfMotion(path string, perf map[string]json.RawMessage) error {
	reduce := string(perf["reduceMotion"]) == "true"
	scale, hasScale := perfMotionScale(perf["motionSpeed"])
	if !reduce && !hasScale {
		return nil
	}
	shell, exists, err := readRawStore(path)
	if err != nil || !exists {
		return err
	}
	changed := false
	if reduce {
		cur, err := rawLeaf(shell, "theme", "motion", "reduce")
		if err != nil {
			return err
		}
		if string(cur) != "true" {
			if err := setRawLeaf(shell, json.RawMessage("true"), "theme", "motion", "reduce"); err != nil {
				return err
			}
			changed = true
		}
	}
	if hasScale {
		cur, err := rawLeaf(shell, "theme", "motion", "scale")
		if err != nil {
			return err
		}
		var f float64
		if cur == nil || (json.Unmarshal(cur, &f) == nil && f == 1) {
			b, _ := json.Marshal(scale)
			if err := setRawLeaf(shell, b, "theme", "motion", "scale"); err != nil {
				return err
			}
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return writeRawStore(path, shell)
}

// perfMotionScale reads a set motionSpeed, clamped into theme.motion.scale's
// range; the default 1 and anything unusable count as unset.
func perfMotionScale(v json.RawMessage) (float64, bool) {
	var f float64
	if v == nil || json.Unmarshal(v, &f) != nil || f <= 0 || math.IsNaN(f) || math.IsInf(f, 0) || f == 1 {
		return 0, false
	}
	return math.Min(3, math.Max(0.25, f)), true
}

// carryPerfWindowStyle turns Window style's blur or shadows behind windows off
// for a stored disableBlur or disableShadows of true: those flags won over the
// Window style value, so off is what the user saw.
func carryPerfWindowStyle(path string, perf map[string]json.RawMessage) (bool, error) {
	var leaves [][]string
	if string(perf["disableBlur"]) == "true" {
		leaves = append(leaves, []string{"desktop", "appearance", "blurEnabled"})
	}
	if string(perf["disableShadows"]) == "true" {
		leaves = append(leaves, []string{"desktop", "appearance", "shadowEnabled"})
	}
	return turnOffLeaves(path, leaves, true)
}

// carryPerfShellEffects turns the shell's glass or shadows on its panels and the
// dock off for a stored disableBlur or disableShadows of true. False or absent
// carries nothing, so they stay at their default, on, whatever Window style says
// about windows. A box with no shell.json has never run the shell, so nothing is
// carried there.
func carryPerfShellEffects(path string, perf map[string]json.RawMessage) error {
	var leaves [][]string
	if string(perf["disableBlur"]) == "true" {
		leaves = append(leaves, []string{"theme", "effects", "glass"})
	}
	if string(perf["disableShadows"]) == "true" {
		leaves = append(leaves, []string{"theme", "effects", "shadows"})
	}
	_, err := turnOffLeaves(path, leaves, false)
	return err
}

// turnOffLeaves sets each leaf of the JSON store at path to false and reports
// whether it wrote. A missing store is created only when create is set.
func turnOffLeaves(path string, leaves [][]string, create bool) (bool, error) {
	if len(leaves) == 0 {
		return false, nil
	}
	store, exists, err := readRawStore(path)
	if err != nil || (!exists && !create) {
		return false, err
	}
	changed := false
	for _, leaf := range leaves {
		cur, err := rawLeaf(store, leaf...)
		if err != nil {
			return false, err
		}
		if string(cur) == "false" {
			continue
		}
		if err := setRawLeaf(store, json.RawMessage("false"), leaf...); err != nil {
			return false, err
		}
		changed = true
	}
	if !changed {
		return false, nil
	}
	return true, writeRawStore(path, store)
}
