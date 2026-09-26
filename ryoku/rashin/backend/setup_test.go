package main

import (
	"reflect"
	"testing"
)

// TestHermesInstallerFlags pins the #279 fix: the official installer exits 1 on
// any option it does not know, so an optional flag is passed only when the
// downloaded script advertises it, while --non-interactive (the guard against a
// hidden tty prompt) is always passed.
func TestHermesInstallerFlags(t *testing.T) {
	cases := []struct {
		name   string
		script string
		want   []string
	}{
		{
			name:   "old script supports both optional flags",
			script: "usage: install.sh [--non-interactive] [--skip-browser] [--skip-computer-use]",
			want:   []string{"--non-interactive", "--skip-browser", "--skip-computer-use"},
		},
		{
			name:   "upstream dropped computer use",
			script: "usage: install.sh [--non-interactive] [--skip-browser]",
			want:   []string{"--non-interactive", "--skip-browser"},
		},
		{
			name:   "script knows neither optional flag",
			script: "usage: install.sh [--non-interactive]",
			want:   []string{"--non-interactive"},
		},
		{
			name:   "empty download still keeps the tty guard",
			script: "",
			want:   []string{"--non-interactive"},
		},
	}
	for _, tc := range cases {
		if got := hermesInstallerFlags(tc.script); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: hermesInstallerFlags = %v, want %v", tc.name, got, tc.want)
		}
	}
}
