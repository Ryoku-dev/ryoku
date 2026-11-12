package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ids returns the tunable ids for a gpu, so a test asserts the shape without
// caring about order or the other fields.
func ids(ts []Tunable, gpu string) []string {
	var out []string
	for _, t := range ts {
		if t.GPU == gpu {
			out = append(out, t.ID)
		}
	}
	return out
}

func has(ts []Tunable, gpu, id string) *Tunable { return findTunable(ts, gpu, id) }

func TestBuildTunablesNvidia(t *testing.T) {
	in := tuneInputs{gpus: []gpuProbe{{
		slot: "0000:01:00.0", driver: "nvidia",
		nv: &nvInfo{powerSupported: true, powerMinW: 5, powerMaxW: 125, powerCurW: 80,
			persistence: false, clockMaxGr: 3105, clockCurGr: 675},
	}}}
	ts := buildTunables(in)
	if pl := has(ts, "0000:01:00.0", "power_limit"); pl == nil || pl.Max != 125 || pl.Kind != "slider" {
		t.Fatalf("power_limit missing or wrong: %+v", pl)
	}
	if has(ts, "0000:01:00.0", "persistence") == nil {
		t.Error("persistence toggle missing")
	}
	if cl := has(ts, "0000:01:00.0", "clock_lock"); cl == nil || cl.Risk != "advanced" {
		t.Error("clock_lock should be present and advanced")
	}
}

// A laptop GPU that reports power management unsupported must not show a slider.
func TestBuildTunablesNvidiaNoPower(t *testing.T) {
	in := tuneInputs{gpus: []gpuProbe{{
		slot: "0000:01:00.0", driver: "nvidia",
		nv: &nvInfo{powerSupported: false, clockMaxGr: 0},
	}}}
	ts := buildTunables(in)
	if has(ts, "0000:01:00.0", "power_limit") != nil {
		t.Error("power_limit must be hidden when unsupported")
	}
	if has(ts, "0000:01:00.0", "clock_lock") != nil {
		t.Error("clock_lock must be hidden when max clock unknown")
	}
}

func TestBuildTunablesAMD(t *testing.T) {
	in := tuneInputs{gpus: []gpuProbe{{
		slot: "0000:65:00.0", driver: "amdgpu",
		amdPerfLevel: "auto",
		amdOD:        &odRange{minMHz: 800, maxMHz: 2799, curMin: 800, curMax: 2799},
	}}}
	ts := buildTunables(in)
	if pl := has(ts, "0000:65:00.0", "perf_level"); pl == nil || pl.Kind != "segment" {
		t.Fatal("perf_level segment missing")
	}
	if od := has(ts, "0000:65:00.0", "sclk_od"); od == nil || od.Min != 800 || od.Max != 2799 || od.Risk != "advanced" {
		t.Fatalf("sclk_od wrong: %+v", od)
	}
	// No power1_cap and no pwm1 on this APU: neither knob may appear.
	if has(ts, "0000:65:00.0", "power_cap") != nil || has(ts, "0000:65:00.0", "fan_pct") != nil {
		t.Error("APU without hwmon cap/fan must not expose power_cap or fan_pct")
	}
}

// The ACPI platform profile must NOT appear among the GPU session knobs. ppd
// rewrites that file on every profile change, so a live knob here silently lost;
// it now belongs to a CPU power-profile definition, which is re-applied after ppd
// settles. This pins the absence so the clobbering knob cannot come back.
func TestBuildTunablesPlatformNotOffered(t *testing.T) {
	in := tuneInputs{platform: &platformProfile{choices: []string{"quiet", "balanced", "performance"}, current: "balanced"}}
	ts := buildTunables(in)
	if th := has(ts, "platform", "thermal"); th != nil {
		t.Fatalf("platform thermal profile must not be a GPU session knob, got %+v", th)
	}
	if len(ts) != 0 {
		t.Errorf("a platform profile alone must yield no knobs, got %d", len(ts))
	}
}

// A machine with nothing writable returns an empty list, which the page renders
// as a clean read-only state.
func TestBuildTunablesEmpty(t *testing.T) {
	if ts := buildTunables(tuneInputs{gpus: []gpuProbe{{slot: "x", driver: "nouveau"}}}); len(ts) != 0 {
		t.Errorf("unsupported driver must yield no knobs, got %v", ids(ts, "x"))
	}
}

func TestParseOD(t *testing.T) {
	// The real pp_od_clk_voltage layout from an amdgpu APU.
	s := "OD_SCLK:\n0:        800Mhz\n1:       2799Mhz\nOD_RANGE:\nSCLK:     800Mhz       2799Mhz\n"
	od := parseOD(s)
	if od == nil {
		t.Fatal("parseOD returned nil for valid input")
	}
	if od.minMHz != 800 || od.maxMHz != 2799 || od.curMax != 2799 {
		t.Errorf("parseOD = %+v", od)
	}
	if parseOD("") != nil {
		t.Error("empty pp_od_clk_voltage must parse to nil (overdrive locked)")
	}
	if parseOD("OD_SCLK:\n0: 800Mhz\n") != nil {
		t.Error("missing OD_RANGE must yield nil, not a zero range")
	}
}

func TestClampAndValidate(t *testing.T) {
	if clampf(200, 5, 125) != 125 {
		t.Error("over-max must clamp to max")
	}
	if clampf(1, 5, 125) != 5 {
		t.Error("under-min must clamp to min")
	}
	if !inList([]string{"auto", "low", "high"}, "low") {
		t.Error("valid segment option rejected")
	}
	if inList([]string{"auto", "low", "high"}, "turbo") {
		t.Error("invalid segment option accepted")
	}
}

// TestApplyKnobFakeSysfs exercises the real write dispatch against a temp sysfs
// tree, so the privileged half is proven without root or touching a GPU. The
// nvidia knobs go through nvidia-smi (a real exec) and are not covered here.
func TestApplyKnobFakeSysfs(t *testing.T) {
	root := t.TempDir()
	t.Setenv("RYOKU_SYSFS_ROOT", root)
	dev := filepath.Join(root, "sys/class/drm/card0/device")
	hw := filepath.Join(dev, "hwmon", "hwmon3")
	mustMkdir(t, hw)
	// seed the files a real amdgpu card exposes
	seed(t, filepath.Join(dev, "power_dpm_force_performance_level"), "auto")
	seed(t, filepath.Join(dev, "pp_od_clk_voltage"), "OD_SCLK:\n0: 800Mhz\n1: 2799Mhz\nOD_RANGE:\nSCLK: 800Mhz 2799Mhz\n")
	seed(t, filepath.Join(hw, "power1_cap"), "15000000")
	seed(t, filepath.Join(hw, "pwm1_enable"), "2")
	seed(t, filepath.Join(hw, "pwm1"), "0")
	seed(t, filepath.Join(root, "sys/firmware/acpi/platform_profile"), "balanced")

	in := tuneInputs{gpus: []gpuProbe{{slot: "0000:65:00.0", card: "card0", driver: "amdgpu"}}}

	if err := applyKnob(in, "0000:65:00.0", "perf_level", "high"); err != nil {
		t.Fatal(err)
	}
	if got := readTrim(filepath.Join(dev, "power_dpm_force_performance_level")); got != "high" {
		t.Errorf("perf_level = %q, want high", got)
	}

	// power_cap takes watts and must land as microwatts.
	if err := applyKnob(in, "0000:65:00.0", "power_cap", "12"); err != nil {
		t.Fatal(err)
	}
	if got := readTrim(filepath.Join(hw, "power1_cap")); got != "12000000" {
		t.Errorf("power1_cap = %q, want 12000000", got)
	}

	// fan_pct switches to manual (1) and writes the scaled duty.
	if err := applyKnob(in, "0000:65:00.0", "fan_pct", "50"); err != nil {
		t.Fatal(err)
	}
	if got := readTrim(filepath.Join(hw, "pwm1_enable")); got != "1" {
		t.Errorf("pwm1_enable = %q, want 1", got)
	}
	if got := readTrim(filepath.Join(hw, "pwm1")); got != "127" {
		t.Errorf("pwm1 = %q, want 127 (50%% of 255)", got)
	}
	// and "auto" hands the fan back to the card (2), which is what Undo sends
	if err := applyKnob(in, "0000:65:00.0", "fan_pct", "auto"); err != nil {
		t.Fatal(err)
	}
	if got := readTrim(filepath.Join(hw, "pwm1_enable")); got != "2" {
		t.Errorf("pwm1_enable after auto = %q, want 2", got)
	}

	// sclk_od ends the commit protocol with "c" left in the file.
	if err := applyKnob(in, "0000:65:00.0", "sclk_od", "2600"); err != nil {
		t.Fatal(err)
	}
	if got := readTrim(filepath.Join(dev, "pp_od_clk_voltage")); got != "c" {
		t.Errorf("pp_od_clk_voltage last write = %q, want c", got)
	}
	if got := readTrim(filepath.Join(dev, "power_dpm_force_performance_level")); got != "manual" {
		t.Errorf("sclk_od must switch perf to manual, got %q", got)
	}

	// platform thermal writes the profile file directly.
	if err := applyKnob(in, "platform", "thermal", "quiet"); err != nil {
		t.Fatal(err)
	}
	if got := readTrim(filepath.Join(root, "sys/firmware/acpi/platform_profile")); got != "quiet" {
		t.Errorf("platform_profile = %q, want quiet", got)
	}
}

// a reset that cannot put one knob back still resets every other knob and card,
// then names what failed instead of reporting done
func TestResetTuneReportsEveryFailure(t *testing.T) {
	root := t.TempDir()
	t.Setenv("RYOKU_SYSFS_ROOT", root)
	calls := filepath.Join(t.TempDir(), "calls")
	t.Setenv("CALLS", calls)
	fakeTools(t, map[string]string{
		"nvidia-smi": `echo "$*" >> "$CALLS"; case "$*" in *-rgc*) echo "not supported"; exit 3 ;; esac`,
	})
	var gpus []gpuProbe
	for i, card := range []string{"card0", "card1"} {
		dev := amdDevDir(root, card)
		hw := filepath.Join(dev, "hwmon", "hwmon0")
		seed(t, filepath.Join(dev, "power_dpm_force_performance_level"), "high")
		seed(t, filepath.Join(dev, "pp_od_clk_voltage"), "")
		seed(t, filepath.Join(hw, "pwm1_enable"), "1")
		seed(t, filepath.Join(hw, "power1_cap"), "12000000")
		gpus = append(gpus, gpuProbe{
			slot: fmt.Sprintf("0000:0%d:00.0", i+3), card: card, driver: "amdgpu",
			amdOD: &odRange{}, amdFan: true, amdCap: &amdCap{defUW: 15000000},
		})
	}
	// card0's overdrive table cannot be written (a directory fails even as root)
	stuck := filepath.Join(amdDevDir(root, "card0"), "pp_od_clk_voltage")
	if err := os.Remove(stuck); err != nil {
		t.Fatal(err)
	}
	mustMkdir(t, stuck)
	gpus = append(gpus, gpuProbe{slot: "0000:01:00.0", driver: "nvidia", nv: &nvInfo{powerDefW: 120, clockMaxGr: 3105}})

	err := resetTune(tuneInputs{gpus: gpus}, "")
	if err == nil || !strings.Contains(err.Error(), stuck) || !strings.Contains(err.Error(), "0000:01:00.0 clock_lock") {
		t.Fatalf("reset must name every failure, got %v", err)
	}
	for _, card := range []string{"card0", "card1"} {
		dev := amdDevDir(root, card)
		hw := filepath.Join(dev, "hwmon", "hwmon0")
		if got := readTrim(filepath.Join(hw, "pwm1_enable")); got != "2" {
			t.Errorf("%s fan = %q, want automatic (2)", card, got)
		}
		if got := readTrim(filepath.Join(hw, "power1_cap")); got != "15000000" {
			t.Errorf("%s power cap = %q, want the default", card, got)
		}
		if got := readTrim(filepath.Join(dev, "power_dpm_force_performance_level")); got != "auto" {
			t.Errorf("%s perf level = %q, want auto", card, got)
		}
	}
	for _, want := range []string{"-i 0000:01:00.0 -pl 120", "-pm 0"} {
		if !strings.Contains(readFile(calls), want) {
			t.Errorf("the failed clock reset stopped %q; calls: %q", want, readFile(calls))
		}
	}
}

func mustMkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
}

func seed(t *testing.T, p, content string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(p))
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizePerf(t *testing.T) {
	for in, want := range map[string]string{"auto": "auto", "low": "low", "high": "high", "manual": "auto", "profile_peak": "auto"} {
		if got := normalizePerf(in); got != want {
			t.Errorf("normalizePerf(%q) = %q, want %q", in, got, want)
		}
	}
}

// a knob the page still shows but the driver stopped offering is named the way
// the page names it, never by its id and PCI slot
func TestMissingTunableErrorUsesThePagesWords(t *testing.T) {
	t.Setenv("RYOKU_SYSFS_ROOT", t.TempDir())
	t.Setenv("RYOKU_GPU_BIN", "")
	fakeTools(t, map[string]string{
		"ryoku-gpu":  `echo '[{"slot":"0000:01:00.0","card":"card1","driver":"nvidia","class":"discrete","model":"GeForce RTX 4060"}]'`,
		"nvidia-smi": `echo 'Supported, 100, 50, 150, 120, Disabled, 525, 3105'`,
	})
	err := tuneSet("0000:01:00.0", "fan_pct", "40")
	if err == nil {
		t.Fatal("a knob the card does not offer was accepted")
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "Fan speed is no longer offered for GeForce RTX 4060") ||
		strings.Contains(msg, "fan_pct") || strings.Contains(msg, "0000:01:00.0") {
		t.Fatalf("error = %q, want the knob and card in the page's words", msg)
	}
}

// "Keep the driver loaded" is a toggle: on/off (and 1/0/true/false, spelled
// on/off) reach the escalated write, and anything else is refused in the
// page's words before any password prompt
func TestPersistenceToggleTakesOnAndOff(t *testing.T) {
	calls := filepath.Join(t.TempDir(), "calls")
	t.Setenv("CALLS", calls)
	t.Setenv("RYOKU_SYSFS_ROOT", t.TempDir())
	t.Setenv("RYOKU_GPU_BIN", "")
	fakeTools(t, map[string]string{
		"ryoku-gpu":  `echo '[{"slot":"0000:01:00.0","card":"card1","driver":"nvidia","class":"discrete"}]'`,
		"nvidia-smi": `echo 'Supported, 100, 50, 150, 120, Disabled, 525, 3105'`,
		"pkexec":     `echo "$*" >> "$CALLS"`,
	})
	for value, want := range map[string]string{"on": "on", "off": "off", "1": "on", "0": "off", "true": "on", "FALSE": "off"} {
		os.Remove(calls)
		if err := tuneSet("0000:01:00.0", "persistence", value); err != nil {
			t.Errorf("persistence %s: %v", value, err)
			continue
		}
		if got := strings.TrimSpace(readFile(calls)); !strings.HasSuffix(got, "gpu tune set 0000:01:00.0 persistence "+want) {
			t.Errorf("persistence %s escalated %q, want it sent as %s", value, got, want)
		}
	}
	os.Remove(calls)
	err := tuneSet("0000:01:00.0", "persistence", "maybe")
	if err == nil || err.Error() != "Keep the driver loaded must be on or off" {
		t.Errorf("persistence maybe = %v, want it refused in the page's words", err)
	}
	if fileExists(calls) {
		t.Errorf("a refused value still escalated: %q", readFile(calls))
	}
}
