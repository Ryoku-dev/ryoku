package main

import (
	"reflect"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestDreamStartCarriesTheResearchPresetAndStatement(t *testing.T) {
	params, err := parseDreamStart([]string{"Compare", "battery", "health", "sources", "--preset", "research", "--name", "Battery report"})
	if err != nil {
		t.Fatal(err)
	}
	if params.Statement != "Compare battery health sources" || params.Preset != "research" || params.Name != "Battery report" {
		t.Fatalf("the research flag lost the question or preset: %+v", params)
	}
}

func TestDreamStartRejectsAnUnknownOrMissingPreset(t *testing.T) {
	for _, args := range [][]string{{"Compare sources", "--preset", "unknown"}, {"Compare sources", "--preset"}} {
		if _, err := parseDreamStart(args); err == nil {
			t.Fatalf("an invalid preset was accepted: %v", args)
		}
	}
}

func TestResearchDraftRouteRoundTripsWithoutStartingTheDream(t *testing.T) {
	draft := &protocol.DreamCreateParams{Statement: "Compare /proc values? Include charge & health.", Preset: "research", Name: "Battery #1", Folder: "/tmp/battery report", Harness: "fake", Model: "research/model", MonthTokens: 100000}
	route := teamsRoute(protocol.UIOpenParams{Tab: protocol.TabDreams, Team: "new", DreamDraft: draft})
	var opened protocol.UIOpenParams
	if err := parseTeamsRoute(route, &opened); err != nil {
		t.Fatal(err)
	}
	if opened.Tab != protocol.TabDreams || opened.Team != "new" || !reflect.DeepEqual(opened.DreamDraft, draft) {
		t.Fatalf("the app did not receive the complete dream draft: %+v", opened)
	}
	if err := parseTeamsRoute("dreams/new?draft="+strings.Repeat("x", protocol.DreamDraftMaxBytes+1), &opened); err == nil {
		t.Fatal("an oversized draft route was accepted")
	}
}
