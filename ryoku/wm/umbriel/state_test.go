package main

import "testing"

func TestActiveWorkspaceName(t *testing.T) {
	s := uState{Workspaces: []uWorkspace{
		{ID: "eDP-1:1", Name: "1"},
		{ID: "Virtual-1:1", Name: "1"},
	}}
	if got := activeWorkspaceName(s, "eDP-1:1"); got != "1" {
		t.Fatalf("got %q, want 1", got)
	}
	if got := activeWorkspaceName(s, "missing"); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestNormalizeMode(t *testing.T) {
	cases := map[string]string{
		"1920x1080@60010": "1920x1080@60.01",
		"1024x768@60004":  "1024x768@60.004",
		"2560x1440@144":   "2560x1440@144",
		"1920x1080":       "1920x1080",
		"1920x1080@bad":   "",
	}
	for in, want := range cases {
		if got := normalizeMode(in); got != want {
			t.Fatalf("normalizeMode(%q) = %q, want %q", in, got, want)
		}
	}
}
