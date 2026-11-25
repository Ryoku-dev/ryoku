package main

import (
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestSettingsViewsOpenByFlagsAndRoute(t *testing.T) {
	for _, view := range []string{"features", "providers", "oni", "oni-voice", "oni-screen", "oni-commands", "oni-writing", "hands", "oni-privacy", "parts"} {
		for _, args := range [][]string{{"--tab", protocol.TabSettings, "--view", view}, {"--route", "settings/" + view}} {
			params, err := parseOpen(args)
			if err != nil || params.Tab != protocol.TabSettings || params.View != view {
				t.Fatalf("%v opened %+v: %v", args, params, err)
			}
		}
		for _, args := range [][]string{{"--view", view}, {"--tab", protocol.TabProjects, "--view", view}, {"--tab", protocol.TabSettings, "--project", "work", "--view", view}} {
			if _, err := parseOpen(args); err == nil {
				t.Fatalf("Settings-only route accepted outside Settings: %v", args)
			}
		}
	}
}
