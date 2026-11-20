package main

import (
	"strings"
	"testing"
)

func TestRenderBindsShippedTable(t *testing.T) {
	body, _ := renderBinds(nil, nil)
	if !strings.HasPrefix(body, "[keybinds]\n") {
		t.Fatalf("missing table header:\n%s", body[:80])
	}
	for _, chord := range []string{`"Mod+Q"`, `"Mod+Shift+P"`, `"Mod+P"`, `"Mod+WheelUp"`, `"Alt+Tab"`} {
		if !strings.Contains(body, chord) {
			t.Fatalf("missing shipped chord %s", chord)
		}
	}
	if strings.Contains(body, "does not generate keybinds") {
		t.Fatal("shipped template was not used")
	}
}

func TestRenderBindsRebindMovesChord(t *testing.T) {
	body, unh := renderBinds(map[string]string{"SUPER + Q": "SUPER + X"}, nil)
	if !strings.Contains(body, `"Mod+X" = { action = "window-close"`) {
		t.Fatalf("rebind not rendered:\n%s", body[:400])
	}
	if strings.Contains(body, `"Mod+Q" =`) {
		t.Fatal("old chord still bound")
	}
	if len(unh) != 0 {
		t.Fatalf("unexpected unhonoured: %+v", unh)
	}
}

func TestRenderBindsFamilyRebind(t *testing.T) {
	body, unh := renderBinds(map[string]string{"SUPER + {n}": "CTRL + {n}"}, nil)
	if !strings.Contains(body, `"Ctrl+1" = "workspace-switch:1"`) {
		t.Fatalf("family rebind not rendered:\n%s", body[:400])
	}
	if strings.Contains(body, `"Mod+1" =`) {
		t.Fatal("old family chord still bound")
	}
	if len(unh) != 0 {
		t.Fatalf("unexpected unhonoured: %+v", unh)
	}
}

func TestRenderBindsRejectsDuplicateTargets(t *testing.T) {
	// Two rebinds onto the same target would emit the chord twice, which
	// Umbriel rejects as a redefinition and breaks the whole config.
	body, unh := renderBinds(map[string]string{
		"SUPER + Q": "SUPER + Z",
		"SUPER + W": "SUPER + Z",
	}, nil)
	if n := strings.Count(body, `"Mod+Z"`); n != 1 {
		t.Fatalf("target chord emitted %d times:\n%s", n, body)
	}
	if len(unh) != 1 {
		t.Fatalf("expected one unhonoured collision, got %+v", unh)
	}
}

func TestRenderBindsDisabledTargetNotResurrected(t *testing.T) {
	body, _ := renderBinds(
		map[string]string{"SUPER + Q": "SUPER + Z"},
		[]string{"SUPER + Z"},
	)
	if strings.Contains(body, `"Mod+Z"`) {
		t.Fatalf("disabled rebind target was emitted:\n%s", body)
	}
}

func TestRenderBindsDisabled(t *testing.T) {
	body, _ := renderBinds(nil, []string{"SUPER + Q"})
	if strings.Contains(body, `"Mod+Q" =`) {
		t.Fatal("disabled chord still bound")
	}
}

func TestRenderBindsUnhonoured(t *testing.T) {
	_, unh := renderBinds(map[string]string{
		"SUPER + Q":         "SUPER + mouse:272",
		"SUPER + SHIFT + P": "SUPER + Q",
	}, nil)
	if len(unh) != 2 {
		t.Fatalf("expected two unhonoured rows, got %+v", unh)
	}
	for _, row := range unh {
		if row.Reason == "" || !strings.HasPrefix(row.Key, "desktop.keybindRebinds.") {
			t.Fatalf("bad unhonoured row: %+v", row)
		}
	}
}

func TestUmbrielChord(t *testing.T) {
	cases := map[string]string{
		"SUPER + Q":           "Mod+Q",
		"SUPER + SHIFT + P":   "Mod+Shift+P",
		"SUPER + mouse_up":    "Mod+WheelUp",
		"SUPER + comma":       "Mod+Comma",
		"ALT + Tab":           "Alt+Tab",
		"XF86AudioNext":       "XF86AudioNext",
		"SUPER + bracketleft": "Mod+bracketleft",
	}
	for in, want := range cases {
		got, ok := umbrielChord(in)
		if !ok || got != want {
			t.Fatalf("umbrielChord(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	if _, ok := umbrielChord("SUPER + mouse:272"); ok {
		t.Fatal("mouse drag chord should be unsupported")
	}
}
