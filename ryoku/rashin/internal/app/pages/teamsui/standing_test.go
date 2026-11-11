package teamsui

import (
	"testing"

	"ryoku-rashin/internal/protocol"
)

// a choice question is answered with an option, so the record says which, not that it was approved
func TestAnAnsweredChoiceSaysWhatWasChosen(t *testing.T) {
	approval := protocol.Approval{
		Kind: protocol.ApprovalAction, Status: protocol.ApprovalApproved, Decision: protocol.DecideChoose,
		Options: []string{"alpha", "beta", "gamma"}, Choice: "beta",
	}
	if word, _ := ApprovalOutcome(approval); word != "Chose B · beta" {
		t.Errorf("the answered choice reads %q, want %q", word, "Chose B · beta")
	}
	approval.Decision, approval.Choice = protocol.DecideOnce, ""
	if word, _ := ApprovalOutcome(approval); word != "Approved once" {
		t.Errorf("an approval reads %q, want %q", word, "Approved once")
	}
}
