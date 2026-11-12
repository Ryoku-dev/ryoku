package main

// gpupreset.go: named bundles of tuning knobs for the Hub GPU page. Built-ins
// are symbolic (min/max/low/high/quiet/...) and resolve against whatever the
// current machine actually exposes, so they are hardware-generic and silently
// skip a knob a box lacks. Custom presets store concrete values the user saved.
//
// A preset applies as one privileged batch: `preset apply` escalates once and
// writes every knob in a single root process, so the user sees one pkexec
// prompt, not one per knob. Only the definition is persisted (to
// gpu-presets.json); the GPU state it produces is still per session.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

type presetEntry struct {
	GPU   string `json:"gpu"` // "" means every GPU that has the knob; "platform" for ACPI
	ID    string `json:"id"`
	Value string `json:"value"` // symbolic (built-in) or concrete (custom)
}

type preset struct {
	Name    string        `json:"name"`
	Builtin bool          `json:"builtin"`
	Entries []presetEntry `json:"entries"`
}

// The three shipped intents. Order matters: perf_level before sclk_od, so the
// overdrive edit (which forces manual mode) is the last word on an OD-capable
// card while perf_level still applies on cards without overdrive.
var builtinPresets = []preset{
	{Name: "Quiet", Builtin: true, Entries: []presetEntry{
		{GPU: "platform", ID: "thermal", Value: "quiet"},
		{ID: "perf_level", Value: "low"},
	}},
	{Name: "Balanced", Builtin: true, Entries: []presetEntry{
		{GPU: "platform", ID: "thermal", Value: "balanced"},
		{ID: "perf_level", Value: "auto"},
	}},
	{Name: "Performance", Builtin: true, Entries: []presetEntry{
		{GPU: "platform", ID: "thermal", Value: "performance"},
		{ID: "perf_level", Value: "high"},
		{ID: "power_limit", Value: "max"},
		{ID: "sclk_od", Value: "max"},
	}},
}

func runGpuPreset(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("gpu tune preset needs list|apply|save|rename|delete")
	}
	switch args[0] {
	case "list":
		return printJSON(allPresets())
	case "apply":
		if len(args) < 2 {
			return fmt.Errorf("preset apply needs a name")
		}
		return presetApply(args[1])
	case "save":
		if len(args) < 2 {
			return fmt.Errorf("preset save needs a name")
		}
		return presetSave(args[1])
	case "delete":
		if len(args) < 2 {
			return fmt.Errorf("preset delete needs a name")
		}
		return presetDelete(args[1])
	case "rename":
		if len(args) < 3 {
			return fmt.Errorf("preset rename needs <old> <new>")
		}
		return presetRename(args[1], args[2])
	default:
		return fmt.Errorf("gpu tune preset needs list|apply|save|rename|delete")
	}
}

func allPresets() []preset {
	out := append([]preset{}, builtinPresets...)
	return append(out, loadCustomPresets()...)
}

func findPreset(name string) *preset {
	for i, p := range allPresets() {
		if p.Name == name {
			all := allPresets()
			return &all[i]
		}
	}
	return nil
}

// resolveJobs turns a preset's (possibly symbolic) entries into concrete,
// clamped {gpu,id,value} jobs against the machine's live knobs. Anything a knob
// cannot satisfy is skipped, never guessed.
func resolveJobs(p *preset, tunables []Tunable) []presetEntry {
	var jobs []presetEntry
	for _, e := range p.Entries {
		for i := range tunables {
			t := &tunables[i]
			if t.ID != e.ID {
				continue
			}
			if e.GPU != "" && t.GPU != e.GPU {
				continue
			}
			v := resolvePresetValue(t, e.Value)
			if v == "" {
				continue
			}
			jobs = append(jobs, presetEntry{GPU: t.GPU, ID: t.ID, Value: v})
		}
	}
	return jobs
}

// resolvePresetValue maps a symbolic or concrete value onto one knob, or ""
// (skip) when it does not apply to this knob's kind.
func resolvePresetValue(t *Tunable, sym string) string {
	switch t.Kind {
	case "segment", "toggle":
		if inList(t.Options, sym) {
			return sym
		}
		if t.Kind == "toggle" && (sym == "on" || sym == "off") {
			return sym
		}
		return ""
	case "slider":
		switch sym {
		case "min":
			return trimTrailingZero(t.Min)
		case "max":
			return trimTrailingZero(t.Max)
		default:
			if _, err := strconv.ParseFloat(sym, 64); err == nil {
				return sym // a concrete number from a custom preset
			}
			return ""
		}
	}
	return ""
}

func presetApply(name string) error {
	p := findPreset(name)
	if p == nil {
		return fmt.Errorf("no preset %q", name)
	}
	in := detectTune()
	jobs := resolveJobs(p, buildTunables(in))
	if len(jobs) == 0 {
		return fmt.Errorf("preset %q touches nothing this hardware exposes", name)
	}
	if os.Geteuid() != 0 {
		next := func(locks map[string]int) map[string]int {
			for _, j := range jobs {
				if j.ID == "clock_lock" {
					locks = nextClockLocks(locks, j.GPU, int(atofTrim(j.Value)))
				}
			}
			return locks
		}
		return withClockLocks(next, func() error { return escalateSelf("gpu", "tune", "preset", "apply", name) }, nil)
	}
	for _, j := range jobs {
		if err := applyKnob(in, j.GPU, j.ID, j.Value); err != nil {
			return fmt.Errorf("preset %q: %s %s: %w", name, j.GPU, j.ID, err)
		}
	}
	return printJSON(map[string]any{"applied": name, "jobs": jobs})
}

// presetSave snapshots the current live knob values into a named custom preset.
// No privilege needed: it only reads knobs and writes the user's config.
func presetSave(name string) error {
	if isBuiltin(name) {
		return fmt.Errorf("%q is a built-in preset name; pick another", name)
	}
	tunables := buildTunables(detectTune())
	var entries []presetEntry
	for _, t := range tunables {
		// no clock lock, a fan on automatic: there is no chosen value to keep
		if t.Auto {
			continue
		}
		entries = append(entries, presetEntry{GPU: t.GPU, ID: t.ID, Value: currentValue(t)})
	}
	customs := loadCustomPresets()
	replaced := false
	for i := range customs {
		if customs[i].Name == name {
			customs[i].Entries = entries
			replaced = true
		}
	}
	if !replaced {
		customs = append(customs, preset{Name: name, Entries: entries})
	}
	if err := saveCustomPresets(customs); err != nil {
		return err
	}
	return printJSON(map[string]string{"saved": name})
}

func presetDelete(name string) error {
	customs := loadCustomPresets()
	out := customs[:0]
	for _, p := range customs {
		if p.Name != name {
			out = append(out, p)
		}
	}
	if err := saveCustomPresets(out); err != nil {
		return err
	}
	return printJSON(map[string]string{"deleted": name})
}

// presetRename gives a saved preset another name and keeps its values
func presetRename(from, to string) error {
	if isBuiltin(to) {
		return fmt.Errorf("%q is a built-in preset name; pick another", to)
	}
	if from == to {
		return printJSON(map[string]string{"renamed": to})
	}
	customs := loadCustomPresets()
	found := false
	for _, p := range customs {
		if p.Name == to {
			return fmt.Errorf("a preset called %q already exists", to)
		}
	}
	for i := range customs {
		if customs[i].Name == from {
			customs[i].Name = to
			found = true
		}
	}
	if !found {
		return fmt.Errorf("no saved preset %q", from)
	}
	if err := saveCustomPresets(customs); err != nil {
		return err
	}
	return printJSON(map[string]string{"renamed": to})
}

func currentValue(t Tunable) string {
	if t.Kind == "slider" {
		return trimTrailingZero(t.Current)
	}
	return t.Value
}

func isBuiltin(name string) bool {
	for _, p := range builtinPresets {
		if p.Name == name {
			return true
		}
	}
	return false
}

// ── storage: ~/.config/ryoku/gpu-presets.json ───────────────────────────────

type presetFile struct {
	Presets []preset `json:"presets"`
}

func presetsPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "ryoku", "gpu-presets.json")
}

func loadCustomPresets() []preset {
	b, err := os.ReadFile(presetsPath())
	if err != nil {
		return nil
	}
	var f presetFile
	if json.Unmarshal(b, &f) != nil {
		return nil
	}
	for i := range f.Presets {
		f.Presets[i].Builtin = false
	}
	return f.Presets
}

func saveCustomPresets(ps []preset) error {
	p := presetsPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(presetFile{Presets: ps}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}

func trimTrailingZero(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
