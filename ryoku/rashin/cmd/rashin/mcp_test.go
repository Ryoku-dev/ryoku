package main

import (
	"testing"

	"ryoku-rashin/internal/brief"
)

func TestEveryAgentSessionIsHandedTheTeamTools(t *testing.T) {
	for _, env := range []map[string]string{
		{"RASHIN_SESSION": "session-1", "RASHIN_CWD": "/work/app"},
		{"RASHIN_SESSION": "session-2", "RASHIN_CWD": "/work/app", "RASHIN_TEAM": "team-1", "RASHIN_TEAM_TOKEN": "token"},
	} {
		instructions, tools := serverTools(func(key string) string { return env[key] })
		names := make(map[string]bool, len(tools))
		for _, tool := range tools {
			if names[tool.Name] {
				t.Fatalf("%s is offered twice", tool.Name)
			}
			names[tool.Name] = true
		}
		for _, want := range []string{brief.ToolTeamsTemplates, brief.ToolTeamsList, brief.ToolProposeTeam} {
			if !names[want] {
				t.Fatalf("a session with %v is not handed %s", env, want)
			}
		}
		if names["report"] != (env["RASHIN_TEAM"] != "") {
			t.Fatalf("a session with %v has the team task tools: %v", env, names["report"])
		}
		if instructions == "" {
			t.Fatal("the server says nothing about its tools")
		}
	}
}
