package brief

import (
	"encoding/json"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// Every agent hears what a team is, that teams do not talk to each other,
// and how it proposes one: with its tools when it holds them, else the CLI.
func TestEveryAgentIsToldHowToProposeATeam(t *testing.T) {
	machine(t, false)
	withTools := Agent(Session{Info: protocol.SessionInfo{Cwd: "/work/app"}, Tools: true})
	without := Agent(Session{Info: protocol.SessionInfo{Cwd: "/work/app"}})
	for _, text := range []string{withTools, without} {
		for _, want := range []string{"\nTeams\n", "a lead that hands pieces", "Teams do not talk to each other", "Rashin Inbox", "credential"} {
			if !strings.Contains(text, want) {
				t.Fatalf("the brief does not say %q:\n%s", want, text)
			}
		}
	}
	if !strings.Contains(withTools, ToolProposeTeam) || !strings.Contains(withTools, ToolTeamsTemplates) || strings.Contains(withTools, "rashin teams propose") {
		t.Fatalf("an agent with Rashin's tools is not pointed at them:\n%s", withTools)
	}
	if !strings.Contains(without, "rashin teams propose <file.json>") || strings.Contains(without, ToolProposeTeam) {
		t.Fatalf("an agent without Rashin's tools is not pointed at the CLI:\n%s", without)
	}
}

func TestTeamToolsCallTheirMethodsAndOnlyProposingAsks(t *testing.T) {
	methods := map[string]string{}
	for _, tool := range TeamTools() {
		methods[tool.Name] = tool.Method
		if _, err := json.Marshal(tool.Schema); err != nil {
			t.Fatalf("%s has no JSON schema: %v", tool.Name, err)
		}
		if tool.ReadOnly == (tool.Method == protocol.MTeamsPropose) {
			t.Fatalf("%s says read-only %v", tool.Name, tool.ReadOnly)
		}
	}
	want := map[string]string{ToolTeamsTemplates: protocol.MTeamsTemplates, ToolTeamsList: protocol.MTeamsList, ToolProposeTeam: protocol.MTeamsPropose}
	for name, method := range want {
		if methods[name] != method {
			t.Fatalf("%s calls %q, want %s", name, methods[name], method)
		}
	}
}

func TestProposalReadsTheToolArgumentsForTheSession(t *testing.T) {
	p, err := Proposal(json.RawMessage(`{"team":{"template":"repo","name":"Repo watch","purpose":"Watch my repos."},"credentials":[{"name":"GITHUB_TOKEN","hosts":["api.github.com"],"why":"To read pull requests."}],"reason":"You asked.","session":"forged"}`), "session-1")
	if err != nil || p.Session != "session-1" || p.Team.Template != "repo" || len(p.Credentials) != 1 {
		t.Fatalf("read %+v, %v", p, err)
	}
	if _, err := Proposal(json.RawMessage(`{"team":{"name":"Repo watch"},"reason":"You asked.","extra":1}`), "session-1"); err == nil {
		t.Fatal("a proposal with a field it does not know was read")
	}
	if _, err := Proposal(json.RawMessage(`{"team":{"name":"Repo watch"}}`), "session-1"); err == nil {
		t.Fatal("a proposal without a reason was read")
	}
}

// Haiku called propose_team three times wrong before getting it right: the
// description carries one call that is read as a proposal as it stands.
func TestProposeTeamDescriptionCarriesAValidExample(t *testing.T) {
	var description string
	for _, tool := range TeamTools() {
		if tool.Name == ToolProposeTeam {
			description = tool.Description
		}
	}
	start := strings.Index(description, "{")
	if start < 0 {
		t.Fatalf("the description shows no arguments:\n%s", description)
	}
	example := json.RawMessage(description[start:])
	p, err := Proposal(example, "session-1")
	if err != nil {
		t.Fatalf("the example is not a valid proposal: %v", err)
	}
	if p.Team.Name == "" || p.Team.Purpose == "" || p.Team.Template == "" || len(p.Team.Members) == 0 || len(p.Credentials) == 0 {
		t.Fatalf("the example leaves out a part of the team: %+v", p)
	}
}

// After proposing, agents told the user the routine "starts automatically":
// the tool result and the brief say what really happens next.
func TestAProposalSaysWhatHappensNext(t *testing.T) {
	brief := Teams(true)
	for _, text := range []string{Proposed, brief} {
		for _, want := range []string{"create", "Rashin Inbox", "routines start off", "turn"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%q is not said in:\n%s", want, text)
			}
		}
	}
	if !strings.Contains(Proposed, "credential") {
		t.Fatalf("the tool result does not say credentials are asked for in the Inbox:\n%s", Proposed)
	}
}
