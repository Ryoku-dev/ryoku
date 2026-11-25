package ask

import (
	"reflect"
	"testing"
	"time"

	"ryoku-rashin/internal/app/chat"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

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

func TestFreshAskSuppressesResumeAndPreservesRepeatedDraftsWithAttachments(t *testing.T) {
	page := New()
	store := state.New(nil)
	now := time.Now()
	store.ApplySnapshot(protocol.Snapshot{
		Settings: protocol.Settings{
			Features: map[string]bool{"history": true},
			Ask:      protocol.AskSettings{ResumeMinutes: 60},
		},
		Sessions: []protocol.SessionInfo{{
			ID: "ask-recent", Kind: protocol.KindAsk, State: protocol.StateIdle,
			Created: now.Add(-time.Minute), Updated: now,
		}},
	})
	e := &env.Env{Store: store}
	e.Navigate = func(tab, id string) {
		if tab != protocol.TabAsk || id != "" {
			t.Fatalf("explicit Fresh resumed an old conversation: %s %s", tab, id)
		}
		page.Select(e, id)
	}
	drafts := []chat.Draft{
		{Text: "First unsent question", Attachments: []string{"/tmp/first.png"}},
		{Text: "Second unsent question", Attachments: []string{"/tmp/second.png", "/tmp/third.png"}},
	}
	for _, draft := range drafts {
		page.composer.SetDraft(draft)
		page.Fresh(e)
		if got := page.composer.Draft(); got.Text != "" || len(got.Attachments) != 0 || page.selected != "" {
			t.Fatalf("Fresh restored prior context: %+v %s", got, page.selected)
		}
	}
	if len(page.freshDrafts) != len(drafts) {
		t.Fatalf("repeated Fresh overwrote retained work: %d drafts", len(page.freshDrafts))
	}
	for index := len(drafts) - 1; index >= 0; index-- {
		page.restorePreviousDraft()
		if got := page.composer.Draft(); !reflect.DeepEqual(got, drafts[index]) {
			t.Fatalf("restored draft %d lost text or attachments: %+v", index, got)
		}
		// Simulate finishing this draft before asking to recover the older one.
		page.composer.SetDraft(chat.Draft{})
	}
}

func TestRestoringFreshAskDraftDoesNotOverwriteNewUnsentWork(t *testing.T) {
	page := New()
	old := chat.Draft{Text: "Old question", Attachments: []string{"/tmp/old.png"}}
	newer := chat.Draft{Text: "New question", Attachments: []string{"/tmp/new.png"}}
	page.keepFreshDraft(old)
	page.composer.SetDraft(newer)
	page.restorePreviousDraft()
	if got := page.composer.Draft(); !reflect.DeepEqual(got, old) {
		t.Fatalf("previous draft was not restored: %+v", got)
	}
	page.restorePreviousDraft()
	if got := page.composer.Draft(); !reflect.DeepEqual(got, newer) {
		t.Fatalf("restore discarded new unsent text or attachments: %+v", got)
	}
}
