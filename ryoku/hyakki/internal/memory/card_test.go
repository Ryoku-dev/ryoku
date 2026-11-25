package memory

import (
	"context"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// Forgetting a fact takes it out of the card at once.
func TestRefreshCardDropsForgotten(t *testing.T) {
	te := newTestEngine(t, "")
	ids := te.remember(t, protocol.MemoryYou, "The user plays the cello on weekends", "The user is called Sam")
	if card, err := te.cardGet(context.Background(), true); err != nil || !strings.Contains(card.Text, "cello") {
		t.Fatalf("card = %q, %v", card.Text, err)
	}
	store, _ := te.store()
	if _, err := store.Apply(Origin{By: protocol.MemoryByYou}, []Op{{Kind: OpForget, ID: ids[0]}}); err != nil {
		t.Fatal(err)
	}
	if err := te.refreshCard(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	if recall := te.Recall(context.Background(), protocol.SessionInfo{ID: "ask-c"}, "hi"); strings.Contains(recall.System, "cello") || !strings.Contains(recall.System, "Sam") {
		t.Fatalf("system = %q", recall.System)
	}
}
