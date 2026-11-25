package shell

import (
	"image"
	"slices"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// a team an agent proposed is read and made from its Inbox card, and the credential it needs is asked for there next
func TestAProposedTeamIsMadeFromItsInboxCard(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 3000))
	stage.open("inbox")
	for _, name := range []string{"Translations", "Polish provider cards", "Translator", "Monday catalogue pass", "DEEPL_API_KEY", "Not now"} {
		if _, ok := stage.named(name); !ok {
			t.Fatalf("the proposal card does not show %q; on screen: %v", name, stage.names())
		}
	}
	stage.click("Create team")
	stage.await("the daemon makes the team", func() bool {
		return stage.approval("approval-translations").Status == protocol.ApprovalApproved
	})
	made := stage.approval("approval-translations").Team
	stage.await("the team and its credential request arrive", func() bool {
		snapshot := stage.runtime.store.Snapshot()
		team := slices.ContainsFunc(snapshot.Teams, func(team protocol.Team) bool { return team.ID == made && team.Name == "Translations" })
		request := slices.ContainsFunc(snapshot.Approvals, func(approval protocol.Approval) bool {
			return approval.Team == made && approval.Kind == protocol.ApprovalRequest && approval.Status == protocol.ApprovalPending
		})
		return team && request
	})
	if _, ok := stage.named("Created the team Translations"); !ok {
		t.Fatalf("the card did not fold to its answer; on screen: %v", stage.names())
	}
}
