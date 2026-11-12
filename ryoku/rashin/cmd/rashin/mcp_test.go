package main

import (
	"strings"
	"testing"
	"unicode/utf8"

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

func TestTheMemoryToolsComeOnlyWithMemory(t *testing.T) {
	for _, test := range []struct {
		env  map[string]string
		want []string
	}{
		{map[string]string{"RASHIN_SESSION": "session-1"}, nil},
		{map[string]string{"RASHIN_SESSION": "session-1", "RASHIN_MEMORY": "1"}, []string{brief.ToolMemorySearch, brief.ToolRemember, brief.ToolForget}},
		{map[string]string{"RASHIN_SESSION": "session-1", "RASHIN_MEMORY": "1", "RASHIN_MEMORY_SESSIONS": "1"}, []string{brief.ToolMemorySearch, brief.ToolRemember, brief.ToolForget, brief.ToolSessionSearch}},
		// a team keeps its own memory and never searches the user's conversations
		{map[string]string{"RASHIN_SESSION": "session-2", "RASHIN_TEAM": "team-1", "RASHIN_TEAM_TOKEN": "token", "RASHIN_MEMORY": "1", "RASHIN_MEMORY_SESSIONS": "1"}, []string{brief.ToolMemorySearch, brief.ToolRemember, brief.ToolForget}},
	} {
		instructions, tools := serverTools(func(key string) string { return test.env[key] })
		var got []string
		for _, tool := range tools {
			if brief.IsMemoryTool(tool.Name) {
				got = append(got, tool.Name)
			}
		}
		if strings.Join(got, ",") != strings.Join(test.want, ",") {
			t.Fatalf("with %v the memory tools are %v, want %v", test.env, got, test.want)
		}
		if strings.Contains(instructions, "memory_search") != (len(test.want) > 0) {
			t.Fatalf("with %v the instructions say %q", test.env, instructions)
		}
	}
}

func TestOnlyADreamsManagerIsHandedTheDreamTools(t *testing.T) {
	dream := []string{"hire", "release", "dream_status", "present"}
	for _, test := range []struct {
		env     map[string]string
		dream   bool
		reports bool
	}{
		{map[string]string{"RASHIN_SESSION": "s", "RASHIN_TEAM": "team-1", "RASHIN_TEAM_TOKEN": "token", "RASHIN_TEAM_LEAD": "1"}, false, true},
		{map[string]string{"RASHIN_SESSION": "s", "RASHIN_TEAM": "team-1", "RASHIN_TEAM_TOKEN": "token"}, false, true},
		// a dream's manager has no report: only the user's accepting ends a dream
		{map[string]string{"RASHIN_SESSION": "s", "RASHIN_TEAM": "team-1", "RASHIN_TEAM_TOKEN": "token", "RASHIN_TEAM_LEAD": "1", "RASHIN_TEAM_DREAM": "1"}, true, false},
	} {
		_, tools := serverTools(func(key string) string { return test.env[key] })
		names := map[string]bool{}
		for _, tool := range tools {
			names[tool.Name] = true
		}
		for _, name := range dream {
			if names[name] != test.dream {
				t.Fatalf("with %v the tool %s is offered: %v", test.env, name, names[name])
			}
		}
		if names["report"] != test.reports || test.dream && !names["delegate"] {
			t.Fatalf("with %v the tools are %v", test.env, names)
		}
	}
}

// A model that puts its words in an argument the tool does not take hears
// which ones it does, rather than losing them; ask_user takes question.
func TestAToolRefusesAnArgumentItDoesNotTake(t *testing.T) {
	var args struct {
		Title    string `json:"title"`
		Question string `json:"question"`
	}
	err := toolArguments([]byte(`{"title":"Look","body":"Light or dark?"}`), &args, "title")
	if err == nil || !strings.Contains(err.Error(), "body") || !strings.Contains(err.Error(), "title, question") {
		t.Fatalf("an unknown argument: %v", err)
	}
	if err := toolArguments([]byte(`{"title":"Look","question":"Light or dark?"}`), &args, "title"); err != nil || args.Question != "Light or dark?" {
		t.Fatalf("a known argument: %v, %+v", err, args)
	}
	_, tools := serverTools(func(key string) string {
		return map[string]string{"RASHIN_SESSION": "s", "RASHIN_TEAM": "team-1", "RASHIN_TEAM_TOKEN": "token", "RASHIN_TEAM_DREAM": "1"}[key]
	})
	for _, tool := range tools {
		if tool.Name != "ask_user" {
			continue
		}
		if properties := tool.Input["properties"].(map[string]any); properties["question"] == nil {
			t.Fatalf("ask_user takes %v", properties)
		}
		return
	}
	t.Fatal("no ask_user tool")
}

// ask_user needs a question or a title; a question alone gives the card its
// first sentence as the title, at most 60 runes.
func TestAskUserTakesAQuestionWithoutATitle(t *testing.T) {
	if _, _, err := (askArguments{Detail: "Only detail."}).card(); err == nil {
		t.Fatal("a call with neither a question nor a title was accepted")
	}
	title, detail, err := (askArguments{Question: "Light or dark? And should it list nearby networks.", Detail: "Plan: Ada builds it."}).card()
	if err != nil || title != "Light or dark?" || detail != "Light or dark? And should it list nearby networks.\n\nPlan: Ada builds it." {
		t.Fatalf("got %q, %q, %v", title, detail, err)
	}
	long := strings.Repeat("word ", 30) + "end?"
	if title, _, _ := (askArguments{Question: long}).card(); utf8.RuneCountInString(title) > askTitleRunes || title == "" {
		t.Fatalf("a long question gave the title %q", title)
	}
	if title, _, _ := (askArguments{Title: "Look", Question: "Light or dark?"}).card(); title != "Look" {
		t.Fatalf("a given title became %q", title)
	}
	_, tools := serverTools(func(key string) string {
		return map[string]string{"RASHIN_SESSION": "s", "RASHIN_TEAM": "team-1", "RASHIN_TEAM_TOKEN": "token"}[key]
	})
	for _, tool := range tools {
		if tool.Name == "ask_user" && len(tool.Input["required"].([]string)) != 0 {
			t.Fatalf("ask_user still requires %v", tool.Input["required"])
		}
	}
}
