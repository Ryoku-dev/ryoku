package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBatteryAlertsDefaultsAndPersistence(t *testing.T) {
	s := newTestStore(t)
	want := batteryAlertsSettings{WarningEnabled: true, CriticalEnabled: true, Enabled: true, WarningPercent: 25, CriticalPercent: 10}
	if s.cur.BatteryAlerts != want {
		t.Fatalf("defaults = %+v, want %+v", s.cur.BatteryAlerts, want)
	}
	for path, value := range map[string]string{"batteryAlerts.enabled": "false", "batteryAlerts.warningPercent": "30", "batteryAlerts.criticalPercent": "12"} {
		if err := s.patch(path, rm(value)); err != nil {
			t.Fatal(err)
		}
	}
	want = batteryAlertsSettings{WarningEnabled: true, CriticalEnabled: true, Enabled: false, WarningPercent: 30, CriticalPercent: 12}
	if got := newSettingsStore(s.path).cur.BatteryAlerts; got != want {
		t.Fatalf("reloaded = %+v, want %+v", got, want)
	}
	if err := s.reset("batteryAlerts"); err != nil {
		t.Fatal(err)
	}
	if s.cur.BatteryAlerts != defaultSettings().BatteryAlerts {
		t.Fatalf("reset = %+v", s.cur.BatteryAlerts)
	}
}

func TestBatteryAlertsRejectsInvalidPatchWithoutMutation(t *testing.T) {
	for _, tc := range []struct{ path, value string }{
		{"warningEnabled", "1"}, {"criticalEnabled", "1"}, {"enabled", "1"}, {"warningPercent", "4"}, {"warningPercent", "51"},
		{"criticalPercent", "0"}, {"criticalPercent", "50"},
		{"criticalPercent", "25"}, {"warningPercent", "10"}, {"unknown", "1"},
	} {
		t.Run(tc.path+"="+tc.value, func(t *testing.T) {
			s := newTestStore(t)
			before := s.frameLocked()
			if err := s.patch("batteryAlerts."+tc.path, rm(tc.value)); err == nil {
				t.Fatal("invalid patch accepted")
			}
			if !reflect.DeepEqual(before, s.frameLocked()) {
				t.Fatal("rejected patch mutated settings")
			}
			if _, err := os.Stat(s.path); !os.IsNotExist(err) {
				t.Fatalf("rejected patch persisted: %v", err)
			}
		})
	}
}

func TestBatteryAlertsLenientLoadNormalizesThresholds(t *testing.T) {
	for _, tc := range []struct{ warning, critical, wantWarning, wantCritical int }{
		{0, 90, 5, 4}, {90, -1, 50, 1}, {15, 20, 15, 14}, {5, 1, 5, 1}, {50, 49, 50, 49},
	} {
		s, err := buildSettings(map[string]any{"batteryAlerts": map[string]any{"warningPercent": tc.warning, "criticalPercent": tc.critical}}, false)
		if err != nil {
			t.Fatal(err)
		}
		if s.BatteryAlerts.WarningPercent != tc.wantWarning || s.BatteryAlerts.CriticalPercent != tc.wantCritical || !s.BatteryAlerts.Enabled {
			t.Fatalf("load (%d,%d) = %+v", tc.warning, tc.critical, s.BatteryAlerts)
		}
	}
}

func TestBatteryAlertsTestRPC(t *testing.T) {
	previous := batteryAlertsTestIPC
	t.Cleanup(func() { batteryAlertsTestIPC = previous })
	called := 0
	batteryAlertsTestIPC = func() string { called++; return "ok" }
	d := &daemon{}
	d.registerBatteryAlertsCalls()
	fn := d.callHandler("battery-alerts.test")
	if fn == nil {
		t.Fatal("test call not registered")
	}
	if _, err := fn(rm(`{}`)); err != nil || called != 1 {
		t.Fatalf("forwarding: calls=%d, err=%v", called, err)
	}
	batteryAlertsTestIPC = func() string { return "err qs ipc shell/test: unavailable" }
	if _, err := fn(rm(`{}`)); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("forward failure = %v", err)
	}
}

func TestBatteryAlertsTestIPCUsesNativeTarget(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "argv")
	if err := os.WriteFile(filepath.Join(dir, "qs"), []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$BATTERY_ALERTS_ARGV\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("BATTERY_ALERTS_ARGV", logPath)
	previous := shellDir
	shellDir = ""
	t.Cleanup(func() { shellDir = previous })
	if got := batteryAlertsTestIPC(); got != "ok" {
		t.Fatal(got)
	}
	argv, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(argv) != "-c\nshell\nipc\ncall\nbattery-alerts-native\ntest\n" {
		t.Fatalf("unexpected IPC argv: %q", argv)
	}
}

func TestBatteryAlertsIndependentLevelsPersist(t *testing.T) {
	for _, mode := range []struct{ warning, critical string }{{"true", "false"}, {"false", "true"}, {"false", "false"}, {"true", "true"}} {
		s := newTestStore(t)
		if err := s.patch("batteryAlerts.warningEnabled", rm(mode.warning)); err != nil {
			t.Fatal(err)
		}
		if err := s.patch("batteryAlerts.criticalEnabled", rm(mode.critical)); err != nil {
			t.Fatal(err)
		}
		got := newSettingsStore(s.path).cur.BatteryAlerts
		if got.WarningEnabled != (mode.warning == "true") || got.CriticalEnabled != (mode.critical == "true") {
			t.Fatalf("mode did not persist: %+v", got)
		}
	}
}
