package main

import (
	"slices"
	"testing"
)

// The qt6ct-kde shape: an installed provider must drop the shipped name from
// the install set instead of the run dying on the conflict prompt.
func TestDropSatisfiedKeepsOnlyUnmet(t *testing.T) {
	e := &engine{}
	unmetOut := "qt6ct\nryoku-desktop\n"
	got := filterByUnmet([]string{"git", "qt6ct", "ryoku-desktop"}, unmetOut)
	want := []string{"qt6ct", "ryoku-desktop"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("filter = %v, want %v", got, want)
	}
	_ = e
}

func TestDropSatisfiedEmptyUnmetDropsAll(t *testing.T) {
	if got := filterByUnmet([]string{"git", "curl"}, ""); len(got) != 0 {
		t.Errorf("everything satisfied should drop all, got %v", got)
	}
}

func TestDropFedoraSatisfiedByAlternativeProvider(t *testing.T) {
	tests := []struct {
		name     string
		provided map[string]bool
		want     []string
	}{
		{
			name: "RPM Fusion ffmpeg and Docker CE",
			provided: map[string]bool{
				"/usr/bin/ffmpeg": true,
				"docker":          true,
			},
			want: []string{"ryoku-desktop"},
		},
		{
			name: "no alternatives installed",
			want: []string{"ffmpeg-free", "moby-engine", "ryoku-desktop"},
		},
	}
	packages := []string{"ffmpeg-free", "moby-engine", "ryoku-desktop"}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := filterFedoraSatisfied(packages, func(capability string) bool {
				return test.provided[capability]
			})
			if !slices.Equal(got, test.want) {
				t.Errorf("filter = %v, want %v", got, test.want)
			}
		})
	}
}
