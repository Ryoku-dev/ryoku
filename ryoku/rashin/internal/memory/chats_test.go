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

// A long reply is cut around the sentence that answers the question, not
// around the one echoing what the user asked, and keeps the sentence after
// it. The reply and question are the recorded miss: the question's setup
// words matched the reply's opening, and "sealant" did not match "Seal".
func TestExcerptKeepsMatchingSentence(t *testing.T) {
	reply := "Yes, here are a few DIY home decor projects using recycled materials:\n\n" +
		"1. Wine Cork Bulletin Board - Collect wine corks and arrange them in a square or rectangular shape. Glue them together and add a fabric backing to create a unique and functional bulletin board.\n\n" +
		"2. Newspaper Flower Vase - Roll up strips of newspaper to create a sturdy base, then weave them together to make a vase. Seal the vase with Mod Podge or another varnish to make it water-resistant.\n\n" +
		"3. Bottle Cap Coasters - Collect bottle caps and arrange them in a pattern on a cork base. Use glue to secure them in place.\n\n" +
		"4. Mason Jar Wall Planter - Attach metal hose clamps to a wooden board and insert Mason jars into the clamps. Fill the jars with soil and plant herbs or small flowers.\n\n" +
		"5. Tin Can Lanterns - Punch holes in clean tin cans in a pattern, then place a tea light inside for a warm glow."
	doc := &chatDoc{lines: []chatLine{
		{text: "Can you suggest some DIY home decor projects using recycled materials?"},
		{reply: true, text: reply},
	}}
	question := "I'm going back to our previous conversation about DIY home decor projects using recycled materials. Can you remind me what sealant you recommended for the newspaper flower vase?"
	got := excerpt(doc.weigh(question), reply, chatLineChars)
	if !strings.Contains(got, "Seal the vase with Mod Podge") || utf8.RuneCountInString(got) > chatLineChars+2 {
		t.Fatalf("excerpt = %q (%d)", got, utf8.RuneCountInString(got))
	}
	if !strings.HasPrefix(got, "…") || !strings.HasSuffix(got, "…") {
		t.Fatalf("excerpt does not mark both cuts: %q", got)
	}
	if short := "One short line."; excerpt(nil, short, chatLineChars) != short {
		t.Fatal("a line that fits was changed")
	}
	if !related("sealant", "seal") || related("seal", "sea") || related("vase", "vast") {
		t.Fatal("stems are matched wrongly")
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
