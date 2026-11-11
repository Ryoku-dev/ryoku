package teams

import (
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestARequestedCredentialNamesExactHostsAndTrustsNone(t *testing.T) {
	h := newHarness(t, 0)
	api := newAPIServer(t)
	team := h.newTeam(t, "Ops", nil)
	ref := h.agent(t, h.newTask(t, team, "Check the API.").ID)

	wide := protocol.AgentCredentialRequestParams{AgentRef: ref, Name: "API_TOKEN", Reason: "To read the API.", Hosts: []string{api.host(), "*.example.org"}}
	refused(t, h, protocol.MTeamsAgentRequest, wide, protocol.CodeBadRequest)
	if waiting := h.s.listApprovals(protocol.ApprovalsParams{Pending: true}).Approvals; len(waiting) != 0 {
		t.Fatalf("a request for every name under a domain asked the user: %+v", waiting)
	}

	asked := do[protocol.AgentAnswer](t, h, protocol.MTeamsAgentRequest, protocol.AgentCredentialRequestParams{AgentRef: ref, Name: "API_TOKEN", Reason: "To read the API.", Hosts: []string{api.host()}})
	decided := do[protocol.Approval](t, h, protocol.MTeamsApprovalDecide, protocol.ApprovalDecideParams{ID: h.pending(t).ID, Decision: protocol.DecideProvide, Value: apiToken})
	waitFor(t, "the agent to hear the answer", func() bool { return len(h.sessions.notes(ref.Session)) == 1 })
	credential, ok := h.s.vault.Find("API_TOKEN")
	if !ok || len(credential.Trusted) != 0 {
		t.Fatalf("providing the value trusted the team for %+v, want no trust", credential.Trusted)
	}

	first := do[protocol.AgentHTTPResult](t, h, protocol.MTeamsAgentHTTP, httpCall(ref, api.URL+"/data"))
	if first.Waiting == "" || api.total() != 0 {
		t.Fatalf("the first use of a provided credential went out without asking: %+v", first)
	}
	h.noSecret(t, apiToken, asked, decided, first)
}
