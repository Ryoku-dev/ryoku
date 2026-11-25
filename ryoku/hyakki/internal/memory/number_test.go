package memory

import (
	"context"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/core"
	"ryoku-rashin/internal/protocol"
)

const (
	cardFact   = "Pays for groceries with the card 4111 1111 1111 1111"
	maskedCard = "Pays for groceries with the card •••• •••• •••• 1111"
)

// Memory refuses a card, social security, identity or bank account number
// at the door, from every caller: the app, an agent session, the command
// line and a team. Before the door an agent's or the command line's
// memory.add of a Luhn-valid card number was kept.
func TestTheDoorRefusesNumbersFromEveryCaller(t *testing.T) {
	names := stubNames{teams: map[string]protocol.Team{"alpha": {ID: "alpha", Name: "Alpha"}}}
	e, sessions, _ := shellEngine(t, true, names)
	callers := []struct {
		name  string
		from  core.MemoryCaller
		scope string
	}{
		{"command line", core.MemoryCaller{}, ""},
		{"agent session", core.MemoryCaller{Session: "s1"}, ""},
		{"app", core.MemoryCaller{App: true}, ""},
		{"team", core.MemoryCaller{Team: "alpha", Task: "t1"}, "team"},
	}
	texts := []string{
		cardFact,
		"My social security number is 123-45-6789",
		"National insurance AB123456C",
		"Salary goes to GB82 WEST 1234 5698 7654 32",
		"My passport number is 533380006",
	}
	for _, c := range callers {
		for _, text := range texts {
			err := call(t, e, c.from, protocol.MMemoryAdd, protocol.MemoryAddParams{Scope: c.scope, Text: text}, nil)
			if err == nil {
				t.Fatalf("%s: %q was kept", c.name, text)
			}
			if perr, ok := err.(*protocol.Error); !ok || perr.Code != protocol.CodeBadRequest || strings.Contains(perr.Message, "1111") {
				t.Fatalf("%s: refusal = %v", c.name, err)
			}
		}
	}
	lines := sessions.lines["s1"]
	if len(lines) != len(texts) {
		t.Fatalf("agent session lines = %+v", lines)
	}
	for _, line := range lines {
		if line.Kind != protocol.MemoryRefused || line.Reason != protocol.MemoryRefusedNumber || line.Text != "" {
			t.Fatalf("refusal line = %+v", line)
		}
	}
	var list protocol.MemoryListResult
	if err := call(t, e, core.MemoryCaller{App: true}, protocol.MMemoryList, protocol.MemoryListParams{}, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Memories) != 0 {
		t.Fatalf("kept %+v", list.Memories)
	}
	// a last four alone, a date or an order number is not a card or an ID
	for _, text := range []string{"Their card number ends in 4242", "Passport renewal is due 2027-03-14", "Order 482913 ships Monday"} {
		if err := call(t, e, core.MemoryCaller{}, protocol.MMemoryAdd, protocol.MemoryAddParams{Text: text}, nil); err != nil {
			t.Fatalf("%q refused: %v", text, err)
		}
	}
}

// The store's own door holds for writers that never pass Handle: learning,
// an import and a move from another engine add through Store.Apply.
func TestTheStoreRefusesNumbersFromLearningAndImport(t *testing.T) {
	e, _, _ := shellEngine(t, true, nil)
	st, err := e.store()
	if err != nil {
		t.Fatal(err)
	}
	for _, by := range []string{protocol.MemoryByLearning, protocol.MemoryByImport, protocol.MemoryByYou} {
		_, err := st.Apply(Origin{By: by}, []Op{{Kind: OpAdd, Scope: protocol.MemoryYou, Text: cardFact}})
		refused, ok := errRefused(err)
		if !ok || refused.Reason != protocol.MemoryRefusedNumber || refused.Text != "" {
			t.Fatalf("%s: err = %v", by, err)
		}
	}
	if problem := factProblem(cardFact); problem == "" {
		t.Fatal("a learned fact with a card number passed")
	}
}

// legacy plants a memory kept before the door refused numbers.
func legacy(t *testing.T, e *Engine, text string) string {
	t.Helper()
	st, err := e.store()
	if err != nil {
		t.Fatal(err)
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	id := st.newID()
	now := time.Now()
	st.facts[id] = &fact{ID: id, Scope: protocol.MemoryYou, Text: text, Status: protocol.MemoryActive, Kind: protocol.MemoryDurable,
		Source: protocol.MemorySource{Kind: protocol.MemoryFromYou}, Explicit: true, Created: now, Updated: now}
	return id
}

// A memory kept before the door is masked wherever it is shown or recalled,
// while the stored record keeps its text until the user edits or forgets it.
func TestLegacyNumbersAreMaskedOnRead(t *testing.T) {
	e, sessions, base := shellEngine(t, true, nil)
	ctx := context.Background()
	id := legacy(t, e, cardFact)
	app := core.MemoryCaller{App: true}

	var list protocol.MemoryListResult
	if err := call(t, e, core.MemoryCaller{}, protocol.MMemoryList, protocol.MemoryListParams{}, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Memories) != 1 || list.Memories[0].Text != maskedCard {
		t.Fatalf("list = %+v", list.Memories)
	}
	var got protocol.Memory
	if err := call(t, e, app, protocol.MMemoryGet, protocol.MemoryRefParams{ID: id}, &got); err != nil || got.Text != maskedCard {
		t.Fatalf("get = %+v %v", got, err)
	}
	var card protocol.MemoryCard
	if err := call(t, e, app, protocol.MMemoryCard, protocol.MemoryCardParams{Rebuild: true}, &card); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(card.Text, "4111 1111") || !strings.Contains(card.Text, "•••• 1111") {
		t.Fatalf("card = %q", card.Text)
	}
	if status := e.Status(ctx); strings.Contains(status.Card.Text, "4111 1111") {
		t.Fatalf("status card = %q", status.Card.Text)
	}
	recall := e.Recall(ctx, protocol.SessionInfo{ID: "s1", Kind: protocol.KindAsk, Cwd: base}, "which card do I use for groceries?")
	if strings.Contains(recall.System+recall.Context, "4111 1111") || !strings.Contains(recall.Context, maskedCard) {
		t.Fatalf("recall = %+v", recall)
	}
	if recall.Used != nil {
		for _, item := range recall.Used.Used {
			if strings.Contains(item.Text, "4111 1111") {
				t.Fatalf("used line = %+v", item)
			}
		}
	}

	// masking is presentation: the record is as it was kept
	if st, _ := e.store(); st.facts[id].Text != cardFact {
		t.Fatalf("stored text = %q", st.facts[id].Text)
	}

	// editing it keeps the number out: the same text is refused, a text
	// without the number replaces it
	if err := call(t, e, app, protocol.MMemoryEdit, protocol.MemoryEditParams{ID: id, Text: new(cardFact)}, nil); err == nil {
		t.Fatal("an edit kept the number")
	}
	var change protocol.MemoryChange
	if err := call(t, e, app, protocol.MMemoryEdit, protocol.MemoryEditParams{ID: id, Text: new("Pays for groceries with the blue card")}, &change); err != nil {
		t.Fatal(err)
	}
	var edited protocol.Memory
	if err := call(t, e, app, protocol.MMemoryGet, protocol.MemoryRefParams{ID: id}, &edited); err != nil {
		t.Fatal(err)
	}
	for _, v := range edited.Versions {
		if strings.Contains(v.Text, "4111 1111") {
			t.Fatalf("an earlier text shows the number: %+v", edited.Versions)
		}
	}
	// and the earlier text holding it is never brought back
	if err := call(t, e, app, protocol.MMemoryRestore, protocol.MemoryRestoreParams{ID: id, Version: 1}, nil); err == nil {
		t.Fatal("a restore brought the number back")
	}

	// forgetting a legacy record shows the masked text in the change
	other := legacy(t, e, "Old SSN 123-45-6789 on file")
	if err := call(t, e, core.MemoryCaller{Session: "s1"}, protocol.MMemoryForget, protocol.MemoryForgetParams{IDs: []string{other}}, &change); err != nil {
		t.Fatal(err)
	}
	if len(change.Items) != 1 || strings.Contains(change.Items[0].Text, "6789") {
		t.Fatalf("forget change = %+v", change)
	}
	for _, line := range sessions.lines["s1"] {
		if line.Change != nil {
			for _, item := range line.Change.Items {
				if strings.Contains(item.Text, "6789") {
					t.Fatalf("change line = %+v", line)
				}
			}
		}
	}
}
