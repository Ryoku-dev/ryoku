package ask

import "testing"

func TestANewAskIsWaitedForUntilTheSessionListHasIt(t *testing.T) {
	page := New()
	if page.lost(false) {
		t.Fatal("the new conversation was treated as a lost ask")
	}
	// the first question was sent and its session made, but the session list hasn't arrived yet
	page.starting, page.selected = true, "ask-new"
	if page.lost(false) {
		t.Fatal("an ask that was just made was dropped before it was listed")
	}
	if page.lost(true) || page.starting {
		t.Fatal("a listed ask must end the wait")
	}
	if !page.lost(false) {
		t.Fatal("an ask that left the list after being shown was kept open")
	}
}
