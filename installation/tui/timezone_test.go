package main

import (
	"strings"
	"testing"
)

func TestNormalizeTimezone(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{"America/Sao_Paulo", "America/Sao_Paulo"},
		{"UTC-3", "Etc/GMT+3"},
		{"GMT+5", "Etc/GMT-5"},
		{"UTC+0", "UTC"},
		{"auto", "auto"},
	} {
		got, err := normalizeTimezone(tc.input)
		if err != nil {
			t.Errorf("normalizeTimezone(%q): %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("normalizeTimezone(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestNormalizeTimezoneRejectsUnknownValues(t *testing.T) {
	for _, input := range []string{"UTC-15", "UTC-3:30", "Not/A_Zone", "../UTC", "/usr/share/zoneinfo/UTC"} {
		if got, err := normalizeTimezone(input); err == nil {
			t.Errorf("normalizeTimezone(%q) = %q, want an error", input, got)
		}
	}
}

func TestTimezoneOffsetAliasesAreSearchableAndExplainEtcSigns(t *testing.T) {
	aliases := timezoneOffsetAliases(map[string]bool{
		"Etc/GMT+3":  true,
		"Etc/GMT-14": true,
	})
	if len(aliases) != 2 {
		t.Fatalf("timezoneOffsetAliases returned %d items, want 2", len(aliases))
	}

	p := newPicker(aliases, false)
	p.searching = true
	p.query = "UTC-3"
	p.refilter()
	done, selected := p.update("enter")
	if !done {
		t.Fatal("UTC-3 did not match a fixed-offset picker item")
	}
	if got := p.items[selected].key; got != "Etc/GMT+3" {
		t.Errorf("UTC-3 selected %q, want Etc/GMT+3", got)
	}
	if hint := p.items[selected].hint; !strings.Contains(hint, "signs are reversed") {
		t.Errorf("UTC-3 hint %q does not explain the Etc/GMT sign", hint)
	}
}
