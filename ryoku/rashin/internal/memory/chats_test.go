package memory

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"ryoku-rashin/internal/protocol"
)

// Past-conversation search keeps a kept project's conversations to it, and
// finds nothing while the search is off.
func TestSessionSearchRespectsKeptAndSetting(t *testing.T) {
	te := newTestEngine(t, protocol.ProjectMemoryKept)
	te.sessions.add(protocol.SessionInfo{ID: "agent-g", Kind: protocol.KindAgent, Project: "garden", Title: "Kyoto garden"}, turn{user: "Plan the Kyoto temple garden beds", reply: "Done."})
	te.sessions.add(protocol.SessionInfo{ID: "ask-o", Title: "Kyoto trip"}, turn{user: "Plan my Kyoto temple visit", reply: "Done."})
	te.sessions.add(protocol.SessionInfo{ID: "agent-g2", Kind: protocol.KindAgent, Project: "garden"}, turn{user: "More beds", reply: "Ok."})
	outside, _ := te.sessionsSearch(context.Background(), protocol.SessionInfo{ID: "ask-q"}, protocol.MemorySessionsParams{Query: "Kyoto temple"})
	for _, hit := range outside.Sessions {
		if hit.Session == "agent-g" {
			t.Fatal("a conversation outside the kept project found it")
		}
	}
	inside, _ := te.sessionsSearch(context.Background(), protocol.SessionInfo{ID: "agent-g2", Kind: protocol.KindAgent, Project: "garden"}, protocol.MemorySessionsParams{Query: "Kyoto temple"})
	if len(inside.Sessions) != 1 || inside.Sessions[0].Session != "agent-g" {
		t.Fatalf("inside = %+v", inside.Sessions)
	}
	te.settings.Memory.Sessions = false
	if off, _ := te.sessionsSearch(context.Background(), protocol.SessionInfo{}, protocol.MemorySessionsParams{Query: "Kyoto"}); len(off.Sessions) != 0 {
		t.Fatalf("found %+v with the search off", off.Sessions)
	}
}

// A long reply is cut around the sentence the question asks about, not at
// its head.
func TestExcerptKeepsMatchingSentence(t *testing.T) {
	text := strings.Repeat("Cut the newspaper into strips and roll each one tightly around a skewer. ", 8) +
		"Seal the vase with Mod Podge so it resists moisture. " +
		strings.Repeat("Paint the outside in any colour you like once it is dry. ", 8)
	got := excerpt("what sealant for the newspaper vase", text, chatLineChars)
	if !strings.Contains(got, "Seal the vase with Mod Podge") || utf8.RuneCountInString(got) > chatLineChars+2 {
		t.Fatalf("excerpt = %q (%d)", got, utf8.RuneCountInString(got))
	}
	if !strings.HasPrefix(got, "…") || !strings.HasSuffix(got, "…") {
		t.Fatalf("excerpt does not mark both cuts: %q", got)
	}
	if short := "One short line."; excerpt("line", short, chatLineChars) != short {
		t.Fatal("a line that fits was changed")
	}
}

// A question about what the assistant said reads the replies first.
func TestPastLinesFavourRepliesForYouSaid(t *testing.T) {
	te := newTestEngine(t, "")
	te.sessions.add(protocol.SessionInfo{ID: "ask-pow", Title: "Powwow"},
		turn{user: "What dances are performed at powwows by skilled dancers?", reply: "Skilled dancers perform the Hoop Dance at powwows, a game of agility with many hoops."},
		turn{user: "Are skilled dancers at powwows judged in a game or contest?", reply: "Yes, there are dance contests."},
		turn{user: "Which game do skilled dancers at powwows like best?", reply: "It varies."},
	)
	recall := te.Recall(context.Background(), protocol.SessionInfo{ID: "ask-now"}, "Which game did you say skilled dancers performed at powwows?")
	if !strings.Contains(recall.Context, "Reply: Skilled dancers perform the Hoop Dance") {
		t.Fatalf("context = %q", recall.Context)
	}
	if !asksWhatWasSaid("what sealant did you recommend") || asksWhatWasSaid("what sealant should I use") {
		t.Fatal("asksWhatWasSaid is wrong")
	}
}
