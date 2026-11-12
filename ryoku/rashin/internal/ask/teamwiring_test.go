package ask

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"ryoku-rashin/internal/brief"
	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/llm"
	"ryoku-rashin/internal/protocol"
)

// proposingTeams records what Ask asked of Teams; every other method is left unimplemented
type proposingTeams struct {
	core.Teams
	method string
	params protocol.TeamProposeParams
}

func (t *proposingTeams) Handle(_ context.Context, method string, params json.RawMessage) (any, error) {
	t.method = method
	return protocol.Approval{}, json.Unmarshal(params, &t.params)
}

func TestAReadOnlyAskCanProposeATeam(t *testing.T) {
	call := llm.ToolCall{ID: "call-1", Name: brief.ToolProposeTeam, Arguments: `{"team":{},"reason":"a weekly digest needs its own desk"}`}
	client := &scriptedClient{streams: []*scriptedStream{
		finalStream(llm.Delta{ToolCalls: []llm.ToolCall{call}}),
		finalStream(llm.Delta{Text: "It waits for you in the Inbox."}),
	}}
	teams := &proposingTeams{}
	sink := &recordingSink{}
	if _, err := testRunnerWith(t, client, teams).Prompt(context.Background(), core.PromptInput{Turn: 1, Text: "make me a news team"}, sink); err != nil {
		t.Fatal(err)
	}
	offered := slices.ContainsFunc(client.calls[0].Tools, func(tool llm.Tool) bool { return tool.Name == brief.ToolProposeTeam })
	if !offered {
		t.Fatal("a read-only Ask was not handed propose_team")
	}
	if teams.method != protocol.MTeamsPropose || teams.params.Session != "ask-test" || teams.params.Reason == "" {
		t.Fatalf("Teams heard %q with %+v, want a proposal from the session", teams.method, teams.params)
	}
	told := client.calls[1].Messages[len(client.calls[1].Messages)-1]
	if told.Role != llm.RoleTool || told.Content != brief.Proposed {
		t.Fatalf("the model was told %q", told.Content)
	}
	// the row reads as a change the user approves, never as the model thinking
	rows := 0
	for _, event := range sink.events {
		tool, ok := event.data.(protocol.ToolData)
		if !ok || event.kind != protocol.EvTool || tool.Call != call.ID {
			continue
		}
		rows++
		if tool.Kind != protocol.ToolEdit {
			t.Fatalf("the proposal's row is a %q tool", tool.Kind)
		}
	}
	if rows == 0 {
		t.Fatalf("the proposal showed no row: %+v", sink.events)
	}
}

// reading the teams and templates is a lookup, shown with the search icon
func TestTeamReadsShowAsLookups(t *testing.T) {
	for _, name := range []string{brief.ToolTeamsTemplates, brief.ToolTeamsList} {
		if _, kind, _ := toolPresentation(llm.ToolCall{Name: name, Arguments: `{}`}); kind != protocol.ToolSearch {
			t.Errorf("%s reads as a %q tool", name, kind)
		}
	}
}
