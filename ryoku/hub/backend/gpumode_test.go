package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestClassifyMode(t *testing.T) {
	const dgpu = "0000:01:00.0" // token 0000-01-00-0
	const igpu = "0000:65:00.0" // token 0000-65-00-0
	cases := []struct {
		name, value, want string
	}{
		{"empty is hybrid", "", "hybrid"},
		{"dgpu primary is performance", "/dev/dri/ryoku-gpu-0000-01-00-0:/dev/dri/ryoku-gpu-0000-65-00-0", "performance"},
		{"igpu solo is passthrough", "/dev/dri/ryoku-gpu-0000-65-00-0", "passthrough"},
		{"igpu primary but not solo is hybrid", "/dev/dri/ryoku-gpu-0000-65-00-0:/dev/dri/ryoku-gpu-0000-01-00-0", "hybrid"},
		{"cardN form is unclassified (hybrid)", "/dev/dri/card1:/dev/dri/card0", "hybrid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyMode(tc.value, dgpu, igpu); got != tc.want {
				t.Errorf("classifyMode(%q) = %q, want %q", tc.value, got, tc.want)
			}
		})
	}
}

func TestSlotToken(t *testing.T) {
	if got := slotToken("0000:01:00.0"); got != "0000-01-00-0" {
		t.Errorf("slotToken = %q, want 0000-01-00-0", got)
	}
}

func TestGpuPerformanceAvailability(t *testing.T) {
	old := gpuPolicyStatusOutput
	t.Cleanup(func() { gpuPolicyStatusOutput = old })

	gpuPolicyStatusOutput = func() ([]byte, error) {
		return []byte(`{"performance_available":false,"performance_reason":"Switch the laptop's graphics MUX to discrete first."}`), nil
	}
	available, reason := gpuPerformanceAvailability()
	if available || reason != "Switch the laptop's graphics MUX to discrete first." {
		t.Fatalf("availability = %v, %q", available, reason)
	}

	gpuPolicyStatusOutput = func() ([]byte, error) {
		return nil, errors.New("older ryoku-gpu")
	}
	available, reason = gpuPerformanceAvailability()
	if !available || reason != "" {
		t.Fatalf("failed status = %v, %q; want compatible available default", available, reason)
	}
}

func TestGpuModeCost(t *testing.T) {
	mux := Capability{Strategy: "mux-reboot"}
	live := Capability{Strategy: "live-bind"}
	if c := gpuModeCost("passthrough", mux); c != "reboot" {
		t.Errorf("passthrough on mux = %q, want reboot", c)
	}
	if c := gpuModeCost("passthrough", live); c != "relogin" {
		t.Errorf("passthrough on live = %q, want relogin", c)
	}
	if c := gpuModeCost("performance", mux); c != "relogin" {
		t.Errorf("performance = %q, want relogin", c)
	}
}

func TestReadAQValue(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "gpu.lua")
	lua := "-- managed\nhl.env(\"AQ_DRM_DEVICES\", \"/dev/dri/ryoku-gpu-0000-01-00-0\")\nhl.config({})\n"
	if err := os.WriteFile(p, []byte(lua), 0o644); err != nil {
		t.Fatal(err)
	}
	if v := readAQValue(p); v != "/dev/dri/ryoku-gpu-0000-01-00-0" {
		t.Errorf("readAQValue = %q", v)
	}
	if v := readAQValue(filepath.Join(dir, "missing.lua")); v != "" {
		t.Errorf("missing file = %q, want empty", v)
	}
}

func TestWouldStrandDisplay(t *testing.T) {
	dgpuDrives := Capability{
		Passthrough: &GPU{Slot: "0000:01:00.0", DrivesDisplay: true},
		Host:        &GPU{Slot: "0000:65:00.0", DrivesDisplay: false},
	}
	if !wouldStrandDisplay(dgpuDrives) {
		t.Error("dGPU drives the only display: passthrough must be refused")
	}
	dgpuFree := Capability{
		Passthrough: &GPU{Slot: "0000:01:00.0", DrivesDisplay: false},
		Host:        &GPU{Slot: "0000:65:00.0", DrivesDisplay: true},
	}
	if wouldStrandDisplay(dgpuFree) {
		t.Error("dGPU is free: passthrough is safe")
	}
	bothDrive := Capability{
		Passthrough: &GPU{Slot: "0000:01:00.0", DrivesDisplay: true},
		Host:        &GPU{Slot: "0000:65:00.0", DrivesDisplay: true},
	}
	if wouldStrandDisplay(bothDrive) {
		t.Error("host has its own display: passthrough is safe")
	}
}
