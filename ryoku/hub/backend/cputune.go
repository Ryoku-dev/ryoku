package main

// cputune.go: `ryoku-hub cpu ...`, the Machine page's CPU power-profile and
// battery surface. Unlike gputune.go it probes no hardware itself: ryoku-power
// is the single source of truth for the knob set, so this only shells out to
// `ryoku-power capabilities --json` / `profile get` and reshapes the result
// into the []Tunable the page already renders. cpuTunables is the pure reshape
// and is unit-tested without executing ryoku-power.

import (
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strconv"
)

const ryokuPowerBin = "ryoku-power"

// cpuCaps mirrors `ryoku-power capabilities --json`. Every knob is a pointer so
// an absent capability key drops its Tunable rather than rendering an empty row.
type cpuCaps struct {
	ActiveProfile string      `json:"activeProfile"`
	Profiles      []string    `json:"profiles"`
	CPU           cpuKnobCaps `json:"cpu"`
	Battery       *struct {
		ChargeLimit *sliderCap `json:"chargeLimit"`
	} `json:"battery"`
	ASPM *segmentCap `json:"aspm"`
	Idle *idleCaps   `json:"idle"`
}

type cpuKnobCaps struct {
	Governor        *segmentCap `json:"governor"`
	EPP             *segmentCap `json:"epp"`
	MaxFreqPct      *sliderCap  `json:"maxFreqPct"`
	PlatformProfile *segmentCap `json:"platformProfile"`
}

type segmentCap struct {
	Options []string `json:"options"`
	Current string   `json:"current"`
}

type sliderCap struct {
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Current float64 `json:"current"`
}

// idleCaps mirrors `ryoku-power capabilities --json` .idle: the merged idle
// policy (defaults overlaid with power.json). Seconds are the stored unit; the
// page shows minutes.
type idleCaps struct {
	Enabled    bool       `json:"enabled"`
	OnDesktops bool       `json:"onDesktops"`
	Battery    idleStages `json:"battery"`
	AC         idleStages `json:"ac"`
}

type idleStages struct {
	DimSec       int `json:"dimSec"`
	LockSec      int `json:"lockSec"`
	ScreenOffSec int `json:"screenOffSec"`
	SuspendSec   int `json:"suspendSec"`
}

// profileDef is one profile's stored definition from `ryoku-power profile get`.
// A missing key means "unset": the reshape falls back to the live capability
// value, never a forced default.
type profileDef struct {
	Governor        string `json:"governor"`
	EPP             string `json:"epp"`
	MaxFreqPct      *int   `json:"maxFreqPct"`
	PlatformProfile string `json:"platformProfile"`
}

func runCpu(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("cpu needs caps|active|switch|set")
	}
	switch args[0] {
	case "caps":
		profile := ""
		if len(args) > 1 {
			profile = args[1]
		}
		return cpuCapsReport(profile)
	case "active":
		return cpuActiveReport()
	case "switch":
		if len(args) < 2 {
			return fmt.Errorf("cpu switch needs <profile>")
		}
		return cpuSwitch(args[1])
	case "set":
		if len(args) < 4 {
			return fmt.Errorf("cpu set needs <scope> <id> <value>")
		}
		return cpuSet(args[1], args[2], args[3])
	default:
		return fmt.Errorf("cpu needs caps|active|switch|set")
	}
}

// cpuSwitch changes the LIVE power profile. The write goes through the shell
// daemon, not straight at ppd: the daemon is the single owner of the user's
// pick (it banks it for the reboot restore, keeps game mode's stash intact,
// and re-applies the Ryoku CPU definition after ppd settles). A second, raw
// writer would silently undo all three.
func cpuSwitch(profile string) error {
	return daemonCall("powerprofiles.setProfile", map[string]string{"profile": profile}, nil)
}

func cpuCapsReport(profile string) error {
	caps, err := readCapabilities()
	if err != nil {
		return err
	}
	if profile == "" {
		profile = caps.ActiveProfile
	}
	return printJSON(cpuTunables(caps, readProfileDef(profile)))
}

// cpuActiveReport gives the page the live profile name and the list to edit;
// caps stays a bare []Tunable, which has no room for these.
func cpuActiveReport() error {
	caps, err := readCapabilities()
	if err != nil {
		return err
	}
	return printJSON(struct {
		ActiveProfile string   `json:"activeProfile"`
		Profiles      []string `json:"profiles"`
	}{caps.ActiveProfile, caps.Profiles})
}

func readCapabilities() (cpuCaps, error) {
	var caps cpuCaps
	out, err := exec.Command(ryokuPowerBin, "capabilities", "--json").Output()
	if err != nil {
		return caps, fmt.Errorf("ryoku-power capabilities: %w", err)
	}
	if err := json.Unmarshal(out, &caps); err != nil {
		return caps, fmt.Errorf("parse capabilities: %w", err)
	}
	return caps, nil
}

// readProfileDef returns the stored definition, or an empty def when the profile
// has none yet: an undefined profile is not an error, its knobs show live values.
func readProfileDef(profile string) profileDef {
	var def profileDef
	if profile == "" {
		return def
	}
	out, err := exec.Command(ryokuPowerBin, "profile", "get", profile).Output()
	if err != nil {
		return def
	}
	_ = json.Unmarshal(out, &def)
	return def
}

// cpuTunables is the pure reshape: capabilities plus one profile's definition
// into the knob list the page renders. Stored definition wins over the live
// current; an absent capability key omits its knob entirely.
func cpuTunables(caps cpuCaps, def profileDef) []Tunable {
	const cpuDesc = "Persists · re-applies on profile switch"
	const batDesc = "Persists · re-applies at login"
	var out []Tunable

	if c := caps.CPU.Governor; c != nil {
		out = append(out, Tunable{
			GPU: "cpu", ID: "governor", Label: "Governor",
			Kind: "segment", Options: c.Options,
			Value: firstNonEmpty(def.Governor, c.Current),
			Risk:  "safe", Src: "scaling_governor", Desc: cpuDesc,
		})
	}
	if c := caps.CPU.EPP; c != nil {
		out = append(out, Tunable{
			GPU: "cpu", ID: "epp", Label: "Energy preference",
			Kind: "segment", Options: c.Options,
			Value: firstNonEmpty(def.EPP, c.Current),
			Risk:  "safe", Src: "energy_performance_preference", Desc: cpuDesc,
		})
	}
	if c := caps.CPU.MaxFreqPct; c != nil {
		cur := c.Current
		if def.MaxFreqPct != nil {
			cur = float64(*def.MaxFreqPct)
		}
		out = append(out, Tunable{
			GPU: "cpu", ID: "maxFreqPct", Label: "Max frequency",
			Kind: "slider", Unit: "%",
			Min: c.Min, Max: c.Max, Current: cur,
			Risk: "safe", Src: "scaling_max_freq", Desc: cpuDesc,
		})
	}
	if c := caps.CPU.PlatformProfile; c != nil {
		out = append(out, Tunable{
			GPU: "cpu", ID: "platformProfile", Label: "Thermal profile",
			Kind: "segment", Options: c.Options,
			Value: firstNonEmpty(def.PlatformProfile, c.Current),
			Risk:  "safe", Src: "platform_profile", Desc: cpuDesc,
		})
	}
	if caps.Battery != nil && caps.Battery.ChargeLimit != nil {
		cl := caps.Battery.ChargeLimit
		out = append(out, Tunable{
			GPU: "battery", ID: "chargeLimit", Label: "Charge limit",
			Kind: "slider", Unit: "%",
			Min: cl.Min, Max: cl.Max, Current: cl.Current,
			Risk: "safe", Src: "charge_control_end_threshold", Desc: batDesc,
		})
	}
	if caps.ASPM != nil {
		out = append(out, Tunable{
			GPU: "battery", ID: "aspm", Label: "PCIe ASPM",
			Kind: "segment", Options: caps.ASPM.Options,
			Value: caps.ASPM.Current,
			Risk:  "safe", Src: "pcie_aspm/parameters/policy", Desc: batDesc,
		})
	}
	if idle := caps.Idle; idle != nil {
		out = append(out,
			Tunable{GPU: "idle", ID: "enabled", Label: "Idle timeouts", Kind: "toggle",
				Value: onOff(idle.Enabled), Risk: "safe", Src: "power.json idle.enabled",
				Desc: "Dim, lock, blank and suspend the machine when it sits idle"},
			Tunable{GPU: "idle", ID: "onDesktops", Label: "Also on desktops", Kind: "toggle",
				Value: onOff(idle.OnDesktops), Risk: "safe", Src: "power.json idle.onDesktops",
				Desc: "Run these timeouts on this desktop too, not only on laptops"},
			idleStepper("battery.dimSec", "Dim", idle.Battery.DimSec, 60),
			idleStepper("battery.lockSec", "Lock", idle.Battery.LockSec, 120),
			idleStepper("battery.screenOffSec", "Screen off", idle.Battery.ScreenOffSec, 120),
			idleStepper("battery.suspendSec", "Suspend", idle.Battery.SuspendSec, 240),
			idleStepper("ac.dimSec", "Dim", idle.AC.DimSec, 60),
			idleStepper("ac.lockSec", "Lock", idle.AC.LockSec, 120),
			idleStepper("ac.screenOffSec", "Screen off", idle.AC.ScreenOffSec, 120),
			idleStepper("ac.suspendSec", "Suspend", idle.AC.SuspendSec, 240),
		)
	}
	return out
}

func cpuSet(scope, id, value string) error {
	switch scope {
	case "battery":
		switch id {
		case "chargeLimit":
			return ttyRun(ryokuPowerBin, "charge-limit", "set", value)
		case "aspm":
			return ttyRun(ryokuPowerBin, "aspm", "set", value)
		default:
			return fmt.Errorf("unknown battery knob: %s", id)
		}
	case "idle":
		return idleSet(id, value)
	}
	return ttyRun(ryokuPowerBin, "profile", "set", scope, id, value)
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// idleStepper renders one idle timeout as a minute stepper. Seconds are the
// stored unit, so the current value is rounded to whole minutes; 0 stays 0 and
// means the stage is off.
func idleStepper(id, label string, sec, maxMin int) Tunable {
	return Tunable{
		GPU: "idle", ID: id, Label: label,
		Kind: "stepper", Unit: "min",
		Min: 0, Max: float64(maxMin), StepBy: 1,
		Current: math.Round(float64(sec) / 60),
		Risk:    "safe", Src: "power.json idle." + id, Desc: "0 disables this stage",
	}
}

// idleStoredValue maps a page value to what power.json stores: a toggle becomes
// true/false, and a minute stepper is multiplied back into seconds (its stored
// unit). Pure, so the conversion is unit-tested without shelling out.
func idleStoredValue(id, value string) (string, error) {
	switch id {
	case "enabled", "onDesktops":
		if value == "on" || value == "true" {
			return "true", nil
		}
		return "false", nil
	default: // a {battery,ac}.<stage>Sec path, in minutes from the page
		mins, err := strconv.Atoi(value)
		if err != nil {
			return "", fmt.Errorf("idle %s needs a whole minute value: %w", id, err)
		}
		if mins < 0 {
			mins = 0
		}
		return strconv.Itoa(mins * 60), nil
	}
}

// idleSet persists one idle key through ryoku-power and re-renders hypridle so
// the change takes effect this session, not only at the next login.
func idleSet(id, value string) error {
	stored, err := idleStoredValue(id, value)
	if err != nil {
		return err
	}
	if err := ttyRun(ryokuPowerBin, "idle", "set", id, stored); err != nil {
		return err
	}
	return exec.Command("ryoku-idle", "apply").Run()
}
