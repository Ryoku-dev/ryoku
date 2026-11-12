package main

import (
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestTheDreamsRouteOpensTheNewDreamDialogAndADream(t *testing.T) {
	for _, route := range []string{"dreams", "dreams/new", "dreams/team-1"} {
		params, err := parseOpen([]string{"--route", route})
		if err != nil || params.Tab != protocol.TabDreams || teamsRoute(params) != route {
			t.Fatalf("%s opens %+v, %v", route, params, err)
		}
	}
	if _, err := parseOpen([]string{"--route", "dreams/team-1/extra"}); err == nil {
		t.Fatal("a dreams route with a view was accepted")
	}
}

func TestDreamsStartReadsTheStatementAndItsFlags(t *testing.T) {
	params, err := parseDreamStart([]string{"Make", "an", "ESP32", "page", "--name", "ESP32 setup", "--harness", "omp", "--tokens", "500000"})
	if err != nil || params.Statement != "Make an ESP32 page" || params.Name != "ESP32 setup" || params.Harness != "omp" || params.MonthTokens != 500000 {
		t.Fatalf("parsed %+v, %v", params, err)
	}
	for _, bad := range [][]string{{}, {"--name", "x"}, {"Make it", "--tokens", "-1"}, {"Make it", "--colour", "red"}, {"Make it", "--name"}} {
		if _, err := parseDreamStart(bad); err == nil {
			t.Fatalf("%v was accepted", bad)
		}
	}
}
