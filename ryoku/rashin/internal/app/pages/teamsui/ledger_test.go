package teamsui

import (
	"testing"

	"ryoku-rashin/internal/protocol"
)

// a choice is not an approval: the ledger says which option was picked
func TestTheLedgerSaysWhatWasChosen(t *testing.T) {
	entry := protocol.Activity{
		Actor: protocol.InitiatorYou, Action: "approval.choose", Target: "Which angle should the piece take?", Choice: "beta", OK: true,
	}
	if got, want := ActivitySentence(entry), "You chose “beta” for Which angle should the piece take?"; got != want {
		t.Errorf("the choice reads %q, want %q", got, want)
	}
}
