package main

// gputune.go: `ryoku-hub gpu tune ...`, the per-session GPU tuning surface the
// Hub GPU page drives. Everything here is runtime state (sysfs / nvidia-smi):
// it applies live and is gone on the next reboot, which is the whole safety
// model. A snapper snapshot would not help (it captures the filesystem, not GPU
// runtime state), so the backup is `reset` or a reboot, nothing persistent.
//
// The design mirrors hwcaps.go: buildTunables() is a pure function of gathered
// inputs, unit-tested across hardware shapes; detectTune() does the messy live
// probing. Nothing is assumed about the hardware. A knob appears only if the
// machine actually exposes it, so the same binary is correct on an Intel laptop,
// a single-GPU desktop, or this AMD+NVIDIA box.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Tunable = one knob the page can render. kind picks the control:
// slider (min/max/current + unit), stepper (min/max/current/stepBy), segment
// (options + value) or toggle.
type Tunable struct {
	GPU     string   `json:"gpu"`   // pci slot, or "platform"
	ID      string   `json:"id"`    // power_limit | perf_level | ...
	Label   string   `json:"label"` // human label
	Kind    string   `json:"kind"`  // slider | stepper | segment | toggle
	Unit    string   `json:"unit,omitempty"`
	Min     float64  `json:"min,omitempty"`
	Max     float64  `json:"max,omitempty"`
	Current float64  `json:"current,omitempty"`
	StepBy  float64  `json:"stepBy,omitempty"` // stepper nudge granularity
	Options []string `json:"options,omitempty"`
	Value   string   `json:"value,omitempty"` // segment/toggle current
	Risk    string   `json:"risk"`            // safe | advanced
	Src     string   `json:"src"`             // source tag (sysfs path or cmd)
	Desc    string   `json:"desc,omitempty"`  // persistence note the page renders
	// a limit knob with nothing set (no clock lock, the fan on automatic); current
	// is then not a value anyone chose, so a preset never saves it
	Auto bool `json:"auto,omitempty"`
}

// odRange = the pp_od_clk_voltage sclk overdrive window and its current top.
type odRange struct {
	minMHz, maxMHz int // OD_RANGE SCLK bounds
	curMin, curMax int // OD_SCLK level 0 / 1
}

// amdCap = the amdgpu hwmon power cap, microwatts as sysfs reports it.
type amdCap struct {
	curUW, maxUW, defUW int
}

// nvInfo = what nvidia-smi reports for one NVIDIA GPU.
type nvInfo struct {
	powerSupported                             bool
	powerMinW, powerMaxW, powerCurW, powerDefW float64
	persistence                                bool
	clockMaxGr                                 int
	clockCurGr                                 int
	clockLock                                  int // the lock this Hub set this boot; 0 = none
}

// gpuProbe = one GPU as the tuner sees it: identity from `ryoku-gpu detect`
// plus whatever knobs its driver exposes on this box (nil/zero when absent).
type gpuProbe struct {
	slot, card, driver, model, class   string
	amdPerfLevel                       string   // "" if absent
	amdCap                             *amdCap  // nil if no power1_cap
	amdOD                              *odRange // nil if overdrive locked
	amdFan                             bool     // pwm1_enable present
	amdFanManual                       bool     // pwm1_enable is 1: a fixed duty
	amdFanPct                          int      // that duty, 0..100
	intelMinMHz, intelMaxMHz, intelRP0 int      // 0 if not Intel/absent
	nv                                 *nvInfo  // nil for non-nvidia
}

type platformProfile struct {
	choices []string
	current string
}

type tuneInputs struct {
	gpus     []gpuProbe
	platform *platformProfile
}

func runGpuTune(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("gpu tune needs caps|get|set|reset")
	}
	switch args[0] {
	case "caps", "get":
		return printJSON(buildTunables(detectTune()))
	case "set":
		if len(args) < 4 {
			return fmt.Errorf("gpu tune set needs <gpu> <id> <value>")
		}
		return tuneSet(args[1], args[2], args[3])
	case "reset":
		gpu := ""
		if len(args) > 1 {
			gpu = args[1]
		}
		return tuneReset(gpu)
	case "preset":
		return runGpuPreset(args[1:])
	default:
		return fmt.Errorf("gpu tune needs caps|get|set|reset|preset")
	}
}

// ── the pure builder: gathered inputs -> the knob list ──────────────────────

func buildTunables(in tuneInputs) []Tunable {
	var out []Tunable
	for _, g := range in.gpus {
		if g.nv != nil {
			out = append(out, nvidiaTunables(g)...)
		}
		if g.driver == "amdgpu" {
			out = append(out, amdTunables(g)...)
		}
		if g.driver == "i915" || g.driver == "xe" {
			out = append(out, intelTunables(g)...)
		}
	}
	// The ACPI platform profile is deliberately NOT offered here. ppd rewrites it
	// on every profile change, so a live session knob silently loses; it belongs to
	// a CPU power-profile definition, which is re-applied after ppd settles. Two
	// controls over one file with opposite persistence would only mislead.
	// desc was hardcoded in GpuPage.qml; carry it in the payload so the page
	// renders these rows unchanged while CPU rows state their own persistence.
	for i := range out {
		if out[i].Risk == "advanced" {
			out[i].Desc = "Advanced · per session, can misbehave"
		} else {
			out[i].Desc = "Applies now, resets on reboot"
		}
	}
	return out
}

func nvidiaTunables(g gpuProbe) []Tunable {
	var out []Tunable
	nv := g.nv
	if nv.powerSupported && nv.powerMaxW > nv.powerMinW {
		out = append(out, Tunable{
			GPU: g.slot, ID: "power_limit", Label: tunableLabels["power_limit"],
			Kind: "slider", Unit: "W",
			Min: nv.powerMinW, Max: nv.powerMaxW, Current: nv.powerCurW,
			Risk: "safe", Src: "nvidia-smi -pl",
		})
	}
	out = append(out, Tunable{
		GPU: g.slot, ID: "persistence", Label: tunableLabels["persistence"],
		Kind: "toggle", Value: boolStr(nv.persistence),
		Risk: "safe", Src: "nvidia-smi -pm",
	})
	// nvidia-smi reports the clock running now, not a lock, so the knob shows
	// the lock this Hub set (gpulock.go), or no limit
	if nv.clockMaxGr > 0 {
		lock := Tunable{
			GPU: g.slot, ID: "clock_lock", Label: tunableLabels["clock_lock"],
			Kind: "slider", Unit: "MHz",
			Min: 300, Max: float64(nv.clockMaxGr), Current: float64(nv.clockMaxGr),
			Risk: "advanced", Src: "nvidia-smi -lgc", Auto: true,
		}
		if nv.clockLock > 0 {
			lock.Current, lock.Auto = clampf(float64(nv.clockLock), lock.Min, lock.Max), false
		}
		out = append(out, lock)
	}
	return out
}

func amdTunables(g gpuProbe) []Tunable {
	var out []Tunable
	if g.amdPerfLevel != "" {
		out = append(out, Tunable{
			GPU: g.slot, ID: "perf_level", Label: tunableLabels["perf_level"],
			Kind: "segment", Options: []string{"auto", "low", "high"},
			Value: normalizePerf(g.amdPerfLevel), Risk: "safe",
			Src: "power_dpm_force_performance_level",
		})
	}
	if g.amdCap != nil && g.amdCap.maxUW > 0 {
		out = append(out, Tunable{
			GPU: g.slot, ID: "power_cap", Label: tunableLabels["power_cap"],
			Kind: "slider", Unit: "W",
			Min: 1, Max: float64(g.amdCap.maxUW) / 1e6,
			Current: float64(g.amdCap.curUW) / 1e6,
			Risk:    "safe", Src: "hwmon/power1_cap",
		})
	}
	if g.amdOD != nil && g.amdOD.maxMHz > g.amdOD.minMHz {
		out = append(out, Tunable{
			GPU: g.slot, ID: "sclk_od", Label: tunableLabels["sclk_od"],
			Kind: "slider", Unit: "MHz",
			Min: float64(g.amdOD.minMHz), Max: float64(g.amdOD.maxMHz),
			Current: float64(g.amdOD.curMax),
			Risk:    "advanced", Src: "pp_od_clk_voltage",
		})
	}
	if g.amdFan {
		fan := Tunable{
			GPU: g.slot, ID: "fan_pct", Label: tunableLabels["fan_pct"],
			Kind: "slider", Unit: "%", Min: 0, Max: 100, Current: 0,
			Risk: "advanced", Src: "hwmon/pwm1", Auto: true,
		}
		if g.amdFanManual {
			fan.Current, fan.Auto = float64(g.amdFanPct), false
		}
		out = append(out, fan)
	}
	return out
}

func intelTunables(g gpuProbe) []Tunable {
	if g.intelMaxMHz <= 0 {
		return nil
	}
	top := g.intelRP0
	if top <= 0 {
		top = g.intelMaxMHz
	}
	return []Tunable{{
		GPU: g.slot, ID: "gt_freq", Label: tunableLabels["gt_freq"],
		Kind: "slider", Unit: "MHz",
		Min: float64(g.intelMinMHz), Max: float64(top), Current: float64(g.intelMaxMHz),
		Risk: "advanced", Src: "gt_max_freq_mhz",
	}}
}

// tunableLabels names every knob as the Graphics page does, so a message about
// one (refused, or gone because the driver stopped offering it) uses the name
// the user sees
var tunableLabels = map[string]string{
	"power_limit": "Power limit",
	"persistence": "Keep the driver loaded",
	"clock_lock":  "Highest graphics clock",
	"perf_level":  "Performance level",
	"power_cap":   "Power limit",
	"sclk_od":     "Highest graphics clock",
	"fan_pct":     "Fan speed",
	"gt_freq":     "Highest graphics clock",
}

// missingTunableError explains, in the page's words, a change to a knob this
// machine no longer offers
func missingTunableError(in tuneInputs, gpu, id string) error {
	knob := tunableLabels[id]
	if knob == "" {
		knob = "This setting"
	}
	card := "this graphics card"
	if g := findGPU(in.gpus, gpu); g != nil && g.model != "" {
		card = g.model
	}
	return fmt.Errorf("%s is no longer offered for %s, so nothing was changed. Reopen the page to see what it offers now", knob, card)
}

// ── live probing ────────────────────────────────────────────────────────────

func detectTune() tuneInputs {
	root := sysfsRoot()
	in := tuneInputs{}
	recs, err := gpuRecordsFromTool()
	if err == nil {
		for _, r := range recs {
			g := gpuProbe{
				slot: r.Slot, card: r.Card, driver: r.Driver,
				model: prettyModel(r.Model), class: r.Class,
			}
			switch r.Driver {
			case "amdgpu":
				readAMD(root, &g)
			case "i915", "xe":
				readIntel(root, &g)
			case "nvidia":
				g.nv = readNvidia(r.Slot)
				g.nv.clockLock = clockLockFor(r.Slot)
			}
			in.gpus = append(in.gpus, g)
		}
	}
	in.platform = readPlatform(root)
	return in
}

func amdDevDir(root, card string) string {
	return filepath.Join(root, "sys/class/drm", card, "device")
}

func readAMD(root string, g *gpuProbe) {
	dev := amdDevDir(root, g.card)
	g.amdPerfLevel = readTrim(filepath.Join(dev, "power_dpm_force_performance_level"))
	if od := parseOD(readFile(filepath.Join(dev, "pp_od_clk_voltage"))); od != nil {
		g.amdOD = od
	}
	// hwmon knobs live one level down under an opaque hwmonN dir.
	if hw := firstHwmon(dev); hw != "" {
		cur := atoiTrim(readTrim(filepath.Join(hw, "power1_cap")))
		max := atoiTrim(readTrim(filepath.Join(hw, "power1_cap_max")))
		def := atoiTrim(readTrim(filepath.Join(hw, "power1_cap_default")))
		if max > 0 {
			g.amdCap = &amdCap{curUW: cur, maxUW: max, defUW: def}
		}
		if fileExists(filepath.Join(hw, "pwm1_enable")) {
			g.amdFan = true
			g.amdFanManual = readTrim(filepath.Join(hw, "pwm1_enable")) == "1"
			g.amdFanPct = clampInt((atoiTrim(readTrim(filepath.Join(hw, "pwm1")))*100+127)/255, 0, 100)
		}
	}
}

func readIntel(root string, g *gpuProbe) {
	dev := amdDevDir(root, g.card) // same /sys/class/drm/<card>/device layout
	g.intelMinMHz = atoiTrim(readTrim(filepath.Join(dev, "gt_min_freq_mhz")))
	g.intelMaxMHz = atoiTrim(readTrim(filepath.Join(dev, "gt_max_freq_mhz")))
	g.intelRP0 = atoiTrim(readTrim(filepath.Join(dev, "gt_RP0_freq_mhz")))
}

// readNvidia asks nvidia-smi one CSV line for this GPU. Any failure returns a
// zero nvInfo with powerSupported false, so the knob simply does not appear.
func readNvidia(slot string) *nvInfo {
	nv := &nvInfo{}
	out, err := exec.Command("nvidia-smi", "-i", slot,
		"--query-gpu=power.management,power.limit,power.min_limit,power.max_limit,power.default_limit,persistence_mode,clocks.current.graphics,clocks.max.graphics",
		"--format=csv,noheader,nounits").Output()
	if err != nil {
		return nv
	}
	f := strings.Split(strings.TrimSpace(string(out)), ",")
	for i := range f {
		f[i] = strings.TrimSpace(f[i])
	}
	if len(f) < 8 {
		return nv
	}
	nv.powerSupported = strings.EqualFold(f[0], "Supported")
	nv.powerCurW = atofTrim(f[1])
	nv.powerMinW = atofTrim(f[2])
	nv.powerMaxW = atofTrim(f[3])
	nv.powerDefW = atofTrim(f[4])
	nv.persistence = strings.EqualFold(f[5], "Enabled")
	nv.clockCurGr = atoiTrim(f[6])
	nv.clockMaxGr = atoiTrim(f[7])
	return nv
}

func readPlatform(root string) *platformProfile {
	choices := strings.Fields(readTrim(filepath.Join(root, "sys/firmware/acpi/platform_profile_choices")))
	cur := readTrim(filepath.Join(root, "sys/firmware/acpi/platform_profile"))
	if len(choices) == 0 {
		return nil
	}
	return &platformProfile{choices: choices, current: cur}
}

// ── apply: set one knob live, escalating to root for the write ──────────────

func tuneSet(gpu, id, value string) error {
	in := detectTune()
	t := findTunable(buildTunables(in), gpu, id)
	if t == nil {
		return missingTunableError(in, gpu, id)
	}
	// Validate + clamp against the probed envelope before any privileged write.
	switch {
	case value == "auto" && autoKnobs[id]:
		// no limit, the fan on automatic: what Undo and a reset put back
	case t.Kind == "toggle":
		switch strings.ToLower(value) {
		case "on", "1", "true":
			value = "on"
		case "off", "0", "false":
			value = "off"
		default:
			return fmt.Errorf("%s must be on or off", tunableLabels[id])
		}
	case t.Kind == "segment":
		if !inList(t.Options, value) {
			return fmt.Errorf("%s must be one of %s", tunableLabels[id], strings.Join(t.Options, ", "))
		}
	default:
		v, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("%s needs a number", tunableLabels[id])
		}
		value = strconv.FormatFloat(clampf(v, t.Min, t.Max), 'f', -1, 64)
	}
	if os.Geteuid() != 0 {
		escalate := func() error { return escalateSelf("gpu", "tune", "set", gpu, id, value) }
		if id == "clock_lock" {
			mhz := int(atofTrim(value))
			return withClockLocks(func(l map[string]int) map[string]int { return nextClockLocks(l, gpu, mhz) }, escalate, nil)
		}
		return escalate()
	}
	if err := applyKnob(in, gpu, id, value); err != nil {
		return err
	}
	return printJSON(map[string]string{"gpu": gpu, "id": id, "value": value})
}

func tuneReset(gpu string) error {
	if os.Geteuid() != 0 {
		args := []string{"gpu", "tune", "reset"}
		if gpu != "" {
			args = append(args, gpu)
		}
		// a reset that ran but failed on some knob still lifted the locks: a card
		// that took a lock takes -rgc, and the error names whatever did fail
		return withClockLocks(func(l map[string]int) map[string]int { return nextClockLocks(l, gpu, 0) },
			func() error { return escalateSelf(args...) }, escalationRan)
	}
	if err := resetTune(detectTune(), gpu); err != nil {
		return err
	}
	return printJSON(map[string]string{"reset": orAll(gpu)})
}

// resetTune resets every knob on every selected card, going on past a failure
// so one stuck knob leaves no other one as it was. The platform profile has no
// exported default; the firmware's stays until a reboot.
func resetTune(in tuneInputs, gpu string) error {
	var errs []error
	for _, g := range in.gpus {
		if gpu == "" || g.slot == gpu {
			errs = append(errs, resetGPU(g))
		}
	}
	return errors.Join(errs...)
}

// applyKnob dispatches one validated write. Runs as root (tuneSet escalated).
func applyKnob(in tuneInputs, gpu, id, value string) error {
	if gpu == "platform" {
		return writeSysfs(filepath.Join(sysfsRoot(), "sys/firmware/acpi/platform_profile"), value)
	}
	g := findGPU(in.gpus, gpu)
	if g == nil {
		return fmt.Errorf("unknown gpu %s", gpu)
	}
	dev := amdDevDir(sysfsRoot(), g.card)
	switch id {
	case "power_limit":
		return runErr("nvidia-smi", "-i", g.slot, "-pl", trimDot(value))
	case "persistence":
		return runErr("nvidia-smi", "-pm", boolNum(value))
	case "clock_lock":
		if value == "auto" {
			return runErr("nvidia-smi", "-i", g.slot, "-rgc")
		}
		return runErr("nvidia-smi", "-i", g.slot, "-lgc", "0,"+trimDot(value))
	case "perf_level":
		return writeSysfs(filepath.Join(dev, "power_dpm_force_performance_level"), value)
	case "power_cap":
		uw := int(atofTrim(value) * 1e6)
		return writeSysfs(firstHwmon(dev)+"/power1_cap", strconv.Itoa(uw))
	case "sclk_od":
		return amdSetMaxSclk(dev, trimDot(value))
	case "fan_pct":
		if value == "auto" {
			hw := firstHwmon(dev)
			if hw == "" {
				return fmt.Errorf("no hwmon fan on this GPU")
			}
			return writeSysfs(hw+"/pwm1_enable", "2") // 2 = auto
		}
		return amdSetFan(dev, atoiTrim(trimDot(value)))
	}
	return fmt.Errorf("cannot apply %s", id)
}

// knobs whose "auto" lifts what was set: the clock lock and a fixed fan duty
var autoKnobs = map[string]bool{"clock_lock": true, "fan_pct": true}

// resetGPU puts back each knob this Hub offers on g, trying all of them
func resetGPU(g gpuProbe) error {
	var errs []error
	dev := amdDevDir(sysfsRoot(), g.card)
	nvidia := func(id string, args ...string) {
		if err := runErr("nvidia-smi", args...); err != nil {
			errs = append(errs, fmt.Errorf("%s %s: %w", g.slot, id, err))
		}
	}
	sysfs := func(path, content string) {
		errs = append(errs, writeSysfs(path, content))
	}
	switch g.driver {
	case "nvidia":
		if g.nv != nil && g.nv.powerDefW > 0 {
			nvidia("power_limit", "-i", g.slot, "-pl", strconv.Itoa(int(g.nv.powerDefW)))
		}
		nvidia("clock_lock", "-i", g.slot, "-rgc")
		nvidia("persistence", "-pm", "0")
	case "amdgpu":
		if g.amdOD != nil {
			od := filepath.Join(dev, "pp_od_clk_voltage")
			if err := writeSysfs(od, "r"); err != nil {
				errs = append(errs, err)
			} else {
				sysfs(od, "c")
			}
		}
		if g.amdFan {
			sysfs(firstHwmon(dev)+"/pwm1_enable", "2") // 2 = auto
		}
		if g.amdCap != nil && g.amdCap.defUW > 0 {
			sysfs(firstHwmon(dev)+"/power1_cap", strconv.Itoa(g.amdCap.defUW))
		}
		sysfs(filepath.Join(dev, "power_dpm_force_performance_level"), "auto")
	}
	return errors.Join(errs...)
}

// amdSetMaxSclk raises the sclk overdrive ceiling: OD needs manual mode, then a
// "s 1 <mhz>" edit committed with "c" (kernel pp_od_clk_voltage protocol).
func amdSetMaxSclk(dev, mhz string) error {
	if err := writeSysfs(filepath.Join(dev, "power_dpm_force_performance_level"), "manual"); err != nil {
		return err
	}
	od := filepath.Join(dev, "pp_od_clk_voltage")
	if err := writeSysfs(od, "s 1 "+mhz); err != nil {
		return err
	}
	return writeSysfs(od, "c")
}

// amdSetFan switches pwm1 to manual and writes the 0..255 duty for a 0..100 pct.
func amdSetFan(dev string, pct int) error {
	hw := firstHwmon(dev)
	if hw == "" {
		return fmt.Errorf("no hwmon fan on this GPU")
	}
	if err := writeSysfs(hw+"/pwm1_enable", "1"); err != nil { // 1 = manual
		return err
	}
	duty := clampInt(pct*255/100, 0, 255)
	return writeSysfs(hw+"/pwm1", strconv.Itoa(duty))
}

// ── small helpers ───────────────────────────────────────────────────────────

func parseOD(s string) *odRange {
	if s == "" {
		return nil
	}
	od := &odRange{}
	seen := false
	section := ""
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "OD_SCLK"):
			section = "sclk"
		case strings.HasPrefix(line, "OD_RANGE"):
			section = "range"
		case section == "sclk" && strings.HasPrefix(line, "0:"):
			od.curMin = mhzOf(line)
		case section == "sclk" && strings.HasPrefix(line, "1:"):
			od.curMax = mhzOf(line)
		case section == "range" && strings.HasPrefix(line, "SCLK:"):
			f := strings.Fields(line)
			if len(f) >= 3 {
				od.minMHz = mhzWord(f[1])
				od.maxMHz = mhzWord(f[2])
				seen = true
			}
		}
	}
	if !seen {
		return nil
	}
	return od
}

// mhzOf pulls the MHz figure from "1:  2799Mhz" style lines.
func mhzOf(line string) int {
	f := strings.Fields(line)
	if len(f) < 2 {
		return 0
	}
	return mhzWord(f[1])
}

func mhzWord(w string) int {
	w = strings.TrimSpace(w)
	w = strings.TrimSuffix(strings.TrimSuffix(w, "Mhz"), "MHz")
	w = strings.TrimSuffix(w, "mhz")
	n, _ := strconv.Atoi(strings.TrimFunc(w, func(r rune) bool { return r < '0' || r > '9' }))
	return n
}

func firstHwmon(dev string) string {
	entries, err := os.ReadDir(filepath.Join(dev, "hwmon"))
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "hwmon") {
			return filepath.Join(dev, "hwmon", e.Name())
		}
	}
	return ""
}

func writeSysfs(path, content string) error {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func runErr(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %v: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func readFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func readTrim(path string) string { return strings.TrimSpace(readFile(path)) }

func atoiTrim(s string) int { n, _ := strconv.Atoi(strings.TrimSpace(s)); return n }

func atofTrim(s string) float64 { f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64); return f }

func clampf(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func normalizePerf(s string) string {
	s = strings.TrimSpace(s)
	switch s {
	case "auto", "low", "high":
		return s
	default:
		return "auto" // manual/profile_* collapse to auto for the segment
	}
}

func boolStr(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

func boolNum(v string) string {
	if v == "on" || v == "1" || v == "true" {
		return "1"
	}
	return "0"
}

// trimDot drops a trailing ".0" so "70.0" writes as "70" to tools that want ints.
func trimDot(v string) string {
	if i := strings.IndexByte(v, '.'); i >= 0 {
		return v[:i]
	}
	return v
}

func findTunable(ts []Tunable, gpu, id string) *Tunable {
	for i := range ts {
		if ts[i].GPU == gpu && ts[i].ID == id {
			return &ts[i]
		}
	}
	return nil
}

func findGPU(gs []gpuProbe, slot string) *gpuProbe {
	for i := range gs {
		if gs[i].slot == slot {
			return &gs[i]
		}
	}
	return nil
}

func inList(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func orAll(s string) string {
	if s == "" {
		return "all"
	}
	return s
}
