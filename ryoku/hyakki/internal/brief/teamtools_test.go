package brief

import (
	"encoding/json"
	"testing"
)

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
