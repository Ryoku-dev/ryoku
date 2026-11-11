package main

import (
	"encoding/json"
	"testing"
)

const fullCaps = `{
  "activeProfile": "power-saver",
  "profiles": ["power-saver","balanced","performance"],
  "cpu": {
    "governor":        {"options":["performance","powersave"],"current":"powersave"},
    "epp":             {"options":["default","performance","balance_performance","balance_power","power"],"current":"power"},
    "maxFreqPct":      {"min":20,"max":100,"current":76},
    "platformProfile": {"options":["quiet","balanced","performance"],"current":"quiet"}
  },
  "battery": {"chargeLimit":{"min":50,"max":100,"current":80}},
  "aspm":    {"options":["default","performance","powersave","powersupersave"],"current":"default"}
}`

const noPlatformCaps = `{
  "activeProfile": "balanced",
  "profiles": ["power-saver","balanced","performance"],
  "cpu": {
    "governor":   {"options":["performance","powersave"],"current":"powersave"},
    "epp":        {"options":["default","power"],"current":"power"},
    "maxFreqPct": {"min":20,"max":100,"current":100}
  },
  "battery": {"chargeLimit":{"min":50,"max":100,"current":80}},
  "aspm":    {"options":["default","powersave"],"current":"default"}
}`

const noBatteryCaps = `{
  "activeProfile": "performance",
  "cpu": {
    "governor":   {"options":["performance","powersave"],"current":"performance"},
    "epp":        {"options":["performance","power"],"current":"performance"},
    "maxFreqPct": {"min":20,"max":100,"current":100}
  },
  "aspm": {"options":["default","powersave"],"current":"default"}
}`

const noAspmCaps = `{
  "activeProfile": "balanced",
  "cpu": {
    "governor":   {"options":["performance","powersave"],"current":"powersave"},
    "epp":        {"options":["default","power"],"current":"power"},
    "maxFreqPct": {"min":20,"max":100,"current":85}
  },
  "battery": {"chargeLimit":{"min":50,"max":100,"current":60}}
}`

const fixedChargeCaps = `{
  "cpu": {},
  "battery": {"chargeLimit":{"min":50,"max":100,"current":80,"options":["80","100"]}}
}`

func mustCaps(t *testing.T, s string) cpuCaps {
	t.Helper()
	var c cpuCaps
	if err := json.Unmarshal([]byte(s), &c); err != nil {
		t.Fatalf("unmarshal caps: %v", err)
	}
	return c
}

func byID(ts []Tunable) map[string]Tunable {
	m := make(map[string]Tunable, len(ts))
	for _, t := range ts {
		m[t.ID] = t
	}
	return m
}

func TestCpuTunablesShape(t *testing.T) {
	cases := []struct {
		name    string
		caps    string
		def     profileDef
		wantIDs []string
	}{
		{"full", fullCaps, profileDef{}, []string{"governor", "epp", "maxFreqPct", "platformProfile", "chargeLimit", "aspm"}},
		{"noPlatform", noPlatformCaps, profileDef{}, []string{"governor", "epp", "maxFreqPct", "chargeLimit", "aspm"}},
		{"noBattery", noBatteryCaps, profileDef{}, []string{"governor", "epp", "maxFreqPct", "aspm"}},
		{"noAspm", noAspmCaps, profileDef{}, []string{"governor", "epp", "maxFreqPct", "chargeLimit"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cpuTunables(mustCaps(t, tc.caps), tc.def)
			if len(got) != len(tc.wantIDs) {
				t.Fatalf("got %d tunables, want %d: %+v", len(got), len(tc.wantIDs), got)
			}
			for i, id := range tc.wantIDs {
				if got[i].ID != id {
					t.Errorf("tunable %d: got id %q, want %q", i, got[i].ID, id)
				}
			}
		})
	}
}

func TestCpuTunablesKindsUnitsSlots(t *testing.T) {
	m := byID(cpuTunables(mustCaps(t, fullCaps), profileDef{}))

	want := map[string]struct {
		gpu, kind, unit string
	}{
		"governor":        {"cpu", "segment", ""},
		"epp":             {"cpu", "segment", ""},
		"maxFreqPct":      {"cpu", "slider", "%"},
		"platformProfile": {"cpu", "segment", ""},
		"chargeLimit":     {"battery", "slider", "%"},
		"aspm":            {"battery", "segment", ""},
	}
	for id, w := range want {
		tu, ok := m[id]
		if !ok {
			t.Fatalf("missing tunable %q", id)
		}
		if tu.GPU != w.gpu || tu.Kind != w.kind || tu.Unit != w.unit {
			t.Errorf("%s: gpu=%q kind=%q unit=%q, want gpu=%q kind=%q unit=%q",
				id, tu.GPU, tu.Kind, tu.Unit, w.gpu, w.kind, w.unit)
		}
		if tu.Risk != "safe" {
			t.Errorf("%s: risk=%q, want safe", id, tu.Risk)
		}
		if tu.Desc == "" || tu.Src == "" {
			t.Errorf("%s: desc=%q src=%q, both must be set", id, tu.Desc, tu.Src)
		}
	}

	if got := m["governor"].Options; len(got) != 2 || got[0] != "performance" || got[1] != "powersave" {
		t.Errorf("governor options = %v", got)
	}
	if got := m["aspm"].Options; len(got) != 4 || got[0] != "default" {
		t.Errorf("aspm options = %v", got)
	}
	if s := m["maxFreqPct"]; s.Min != 20 || s.Max != 100 {
		t.Errorf("maxFreqPct range = %v..%v, want 20..100", s.Min, s.Max)
	}
	if s := m["chargeLimit"]; s.Min != 50 || s.Max != 100 {
		t.Errorf("chargeLimit range = %v..%v, want 50..100", s.Min, s.Max)
	}
}

func TestFixedChargeLimitUsesAvailableChoices(t *testing.T) {
	tunable := byID(cpuTunables(mustCaps(t, fixedChargeCaps), profileDef{}))["chargeLimit"]
	if tunable.Kind != "segment" || tunable.Value != "80" {
		t.Fatalf("fixed charge limit = kind %q value %q, want segment 80", tunable.Kind, tunable.Value)
	}
	if len(tunable.Options) != 2 || tunable.Options[0] != "80" || tunable.Options[1] != "100" {
		t.Errorf("fixed charge options = %v, want [80 100]", tunable.Options)
	}
}

func TestStoredValueOverridesLive(t *testing.T) {
	pct := 60
	def := profileDef{Governor: "performance", MaxFreqPct: &pct}
	m := byID(cpuTunables(mustCaps(t, fullCaps), def))

	if got := m["governor"].Value; got != "performance" {
		t.Errorf("governor value = %q, want stored %q", got, "performance")
	}
	if got := m["maxFreqPct"].Current; got != 60 {
		t.Errorf("maxFreqPct current = %v, want stored 60", got)
	}
	// epp and platformProfile were not in the definition: they hold the live value.
	if got := m["epp"].Value; got != "power" {
		t.Errorf("epp value = %q, want live %q", got, "power")
	}
	if got := m["platformProfile"].Value; got != "quiet" {
		t.Errorf("platformProfile value = %q, want live %q", got, "quiet")
	}
}

func TestBatteryKnobsIgnoreProfileDef(t *testing.T) {
	// A profile definition must never change the global battery knobs.
	pct := 55
	def := profileDef{Governor: "performance", MaxFreqPct: &pct}
	m := byID(cpuTunables(mustCaps(t, fullCaps), def))
	if got := m["chargeLimit"].Current; got != 80 {
		t.Errorf("chargeLimit current = %v, want live 80", got)
	}
	if got := m["aspm"].Value; got != "default" {
		t.Errorf("aspm value = %q, want live %q", got, "default")
	}
}

const idleCapsJSON = `{
  "profiles": ["power-saver","balanced","performance"],
  "cpu": {},
  "idle": {
    "enabled": true,
    "onDesktops": false,
    "battery": {"dimSec": 120, "lockSec": 300, "screenOffSec": 330, "suspendSec": 900},
    "ac": {"dimSec": 300, "lockSec": 600, "screenOffSec": 660, "suspendSec": 1800}
  }
}`

func TestCpuTunablesIdle(t *testing.T) {
	m := byID(cpuTunables(mustCaps(t, idleCapsJSON), profileDef{}))

	if en := m["enabled"]; en.GPU != "idle" || en.Kind != "toggle" || en.Value != "on" {
		t.Errorf("enabled = %+v, want idle/toggle/on", en)
	}
	if od := m["onDesktops"]; od.Kind != "toggle" || od.Value != "off" {
		t.Errorf("onDesktops = %+v, want toggle/off", od)
	}

	// Stored seconds render as whole minutes; 330s (5.5m) rounds up to 6.
	stages := map[string]struct{ cur, max float64 }{
		"battery.dimSec":       {2, 60},
		"battery.screenOffSec": {6, 120},
		"battery.suspendSec":   {15, 240},
		"ac.lockSec":           {10, 120},
	}
	for id, w := range stages {
		tu, ok := m[id]
		if !ok {
			t.Errorf("%s missing", id)
			continue
		}
		if tu.GPU != "idle" || tu.Kind != "stepper" || tu.Unit != "min" || tu.StepBy != 1 || tu.Min != 0 {
			t.Errorf("%s = %+v, want idle/stepper/min/stepBy1/min0", id, tu)
		}
		if tu.Current != w.cur || tu.Max != w.max {
			t.Errorf("%s current=%v max=%v, want current=%v max=%v", id, tu.Current, tu.Max, w.cur, w.max)
		}
	}
}

func TestIdleStoredValue(t *testing.T) {
	cases := []struct {
		id, in, want string
		wantErr      bool
	}{
		{"enabled", "on", "true", false},
		{"enabled", "off", "false", false},
		{"onDesktops", "on", "true", false},
		{"battery.dimSec", "5", "300", false}, // minutes -> seconds
		{"ac.suspendSec", "0", "0", false},    // 0 stays 0 (stage off)
		{"battery.lockSec", "-3", "0", false}, // clamped to 0
		{"battery.dimSec", "notanumber", "", true},
	}
	for _, c := range cases {
		got, err := idleStoredValue(c.id, c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("idleStoredValue(%q,%q) = %q, want error", c.id, c.in, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("idleStoredValue(%q,%q) = %q,%v, want %q,nil", c.id, c.in, got, err, c.want)
		}
	}
}
