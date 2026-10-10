package main

import (
	"testing"

	wm "ryoku-wm"
)

func TestParsePerfFlag(t *testing.T) {
	cases := []struct {
		name string
		body string
		key  string
		want bool
	}{
		{"true", `{"unloadWidgetsWhenCovered":true}`, "unloadWidgetsWhenCovered", true},
		{"false", `{"unloadWidgetsWhenCovered":false}`, "unloadWidgetsWhenCovered", false},
		{"other key", `{"freezeVisualizerWhenIdle":true}`, "unloadWidgetsWhenCovered", false},
		{"absent", `{}`, "unloadOverviewWhenIdle", false},
		{"malformed", `not json`, "x", false},
		{"empty", ``, "x", false},
		{"wrong type", `{"x":"yes"}`, "x", false},
		{"palette key", `{"unloadOverviewWhenIdle":true}`, "unloadOverviewWhenIdle", true},
	}
	for _, c := range cases {
		if got := parsePerfFlag([]byte(c.body), c.key); got != c.want {
			t.Errorf("%s: parsePerfFlag(%q,%q) = %v, want %v", c.name, c.body, c.key, got, c.want)
		}
	}
}

// An output whose active workspace holds no windows means the desktop is
// showing there; only when every output is covered are the widgets parked. A
// cold or unknown reading stays visible so a bare desktop is never left.
func TestDesktopVisibleFrom(t *testing.T) {
	cases := []struct {
		name       string
		outputs    []wm.Output
		workspaces []wm.Workspace
		want       bool
	}{
		{
			"single empty workspace is visible",
			[]wm.Output{{Name: "eDP-1", ActiveWorkspace: "1"}},
			[]wm.Workspace{{ID: "1", Windows: 0}},
			true,
		},
		{
			"single covered workspace is hidden",
			[]wm.Output{{Name: "eDP-1", ActiveWorkspace: "1"}},
			[]wm.Workspace{{ID: "1", Windows: 2}},
			false,
		},
		{
			"one of two outputs empty is visible",
			[]wm.Output{{ActiveWorkspace: "1"}, {ActiveWorkspace: "2"}},
			[]wm.Workspace{{ID: "1", Windows: 3}, {ID: "2", Windows: 0}},
			true,
		},
		{
			"all outputs covered is hidden",
			[]wm.Output{{ActiveWorkspace: "1"}, {ActiveWorkspace: "2"}},
			[]wm.Workspace{{ID: "1", Windows: 3}, {ID: "2", Windows: 1}},
			false,
		},
		{"no outputs stays visible", nil, []wm.Workspace{{ID: "1", Windows: 2}}, true},
		{"unknown active workspace stays visible", []wm.Output{{ActiveWorkspace: "9"}}, []wm.Workspace{{ID: "1", Windows: 2}}, true},
	}
	for _, c := range cases {
		if got := desktopVisibleFrom(c.outputs, c.workspaces); got != c.want {
			t.Errorf("%s: desktopVisibleFrom = %v, want %v", c.name, got, c.want)
		}
	}
}
