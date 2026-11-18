package chat

import (
	"errors"
	"image"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func comparisonTranscript(t *testing.T, id, answer string, finished bool) *Model {
	t.Helper()
	model := NewModel(protocol.SessionInfo{ID: id, Kind: protocol.KindAsk, Model: id})
	model.Apply(testEvent(t, 1, protocol.EvUser, protocol.UserData{Text: "Which answer should I keep?"}))
	model.Apply(testEvent(t, 2, protocol.EvTurnStart, protocol.TurnStartData{Turn: 1}))
	model.Apply(testEvent(t, 3, protocol.EvText, protocol.TextData{Message: "answer", Delta: answer}))
	if finished {
		model.Apply(testEvent(t, 4, protocol.EvAnswer, protocol.AnswerData{Via: id}))
		model.Apply(testEvent(t, 5, protocol.EvTurnEnd, protocol.TurnEndData{Turn: 1, Stop: protocol.StopEnd}))
	}
	return model
}

func comparisonLabelBounds(c *conversation, label string) (image.Rectangle, bool) {
	for _, node := range c.frame.router.AppendSemantics(nil) {
		if node.Desc.Label == label {
			return node.Desc.Bounds, true
		}
	}
	return image.Rectangle{}, false
}

func TestDisabledComparisonKeepsTheExistingAnswerSurface(t *testing.T) {
	c := newConversation(image.Pt(1200, 800))
	c.model = comparisonTranscript(t, "source", "The original answer stays here", true)
	c.model.Apply(testEvent(t, 6, protocol.EvCompare, protocol.CompareStartResult{ID: "group", Source: "source", Seq: 5, Lanes: []protocol.CompareLane{{Session: "lane", Model: "Another model"}}}))
	attached := 0
	c.view.SetFeatures(func(string) bool { return false })
	c.view.SetAnswers(AnswerOptions{SessionID: "source", Attach: func(string, func(error)) { attached++ }})
	c.layout()
	c.layout()
	if _, exists := comparisonLabelBounds(c, "The original answer stays here"); !exists {
		t.Fatal("turning the feature off hid the original answer")
	}
	for _, label := range []string{"Compare", "Share as a picture", "Keep this one", "Another model"} {
		if _, exists := comparisonLabelBounds(c, label); exists {
			t.Fatalf("disabled feature still showed %q", label)
		}
	}
	if attached != 0 {
		t.Fatal("disabled comparison started loading lanes")
	}
}

func TestWideComparisonShowsIsolatedStreamingAnswers(t *testing.T) {
	c := newConversation(image.Pt(1400, 900))
	c.model = comparisonTranscript(t, "source", "The replaced answer", true)
	first := comparisonTranscript(t, "first", "First model words", false)
	second := comparisonTranscript(t, "second", "Second model words", true)
	lanes := map[string]*Model{"first": first, "second": second}
	c.model.Apply(testEvent(t, 6, protocol.EvCompare, protocol.CompareStartResult{ID: "group", Source: "source", Seq: 5, Lanes: []protocol.CompareLane{{Session: "first", Model: "First model"}, {Session: "second", Model: "Second model"}}}))
	c.view.SetFeatures(func(id string) bool { return id == "compare" })
	c.view.SetAnswers(AnswerOptions{SessionID: "source", Session: func(id string) *Model { return lanes[id] }})
	for range 3 {
		c.layout()
	}
	left, firstShown := comparisonLabelBounds(c, "First model words")
	right, secondShown := comparisonLabelBounds(c, "Second model words")
	if !firstShown || !secondShown || left.Min.X >= right.Min.X {
		t.Fatalf("wide answers were not shown side by side: %v %v", left, right)
	}
	if _, exists := comparisonLabelBounds(c, "The replaced answer"); exists {
		t.Fatal("the original answer was duplicated above comparison lanes")
	}
	first.Apply(testEvent(t, 4, protocol.EvText, protocol.TextData{Message: "answer", Delta: " continue arriving"}))
	c.layout()
	if _, exists := comparisonLabelBounds(c, "First model words continue arriving"); !exists {
		t.Fatal("new words did not reach the first lane")
	}
	if _, exists := comparisonLabelBounds(c, "Second model words"); !exists {
		t.Fatal("streaming one lane replaced the other answer")
	}
	c.view.SetAnswers(AnswerOptions{SessionID: "another-conversation"})
	if c.view.ensureState().hasComparison() {
		t.Fatal("comparison lanes followed the reader into another conversation")
	}
}

func TestComparisonRechecksTemporaryLanesAfterReconnect(t *testing.T) {
	c := newConversation(image.Pt(1400, 900))
	c.model = comparisonTranscript(t, "source", "The source answer is still available", true)
	first := comparisonTranscript(t, "first", "First live answer", true)
	second := comparisonTranscript(t, "second", "Second live answer", true)
	lanes := map[string]*Model{"first": first, "second": second}
	c.model.Apply(testEvent(t, 6, protocol.EvCompare, protocol.CompareStartResult{
		ID: "group", Source: "source", Seq: 5,
		Lanes: []protocol.CompareLane{{Session: "first", Model: "First model"}, {Session: "second", Model: "Second model"}},
	}))
	c.view.SetFeatures(func(id string) bool { return id == "compare" })
	c.view.SetAnswers(AnswerOptions{
		SessionID: "source", Epoch: 1,
		Attach:  func(_ string, done func(error)) { done(nil) },
		Session: func(id string) *Model { return lanes[id] },
	})
	for range 3 {
		c.layout()
	}
	if _, exists := comparisonLabelBounds(c, "First live answer"); !exists {
		t.Fatal("a live comparison was not restored")
	}
	var replies []func(error)
	c.view.SetAnswers(AnswerOptions{
		SessionID: "source", Epoch: 2,
		Attach: func(_ string, done func(error)) { replies = append(replies, done) },
	})
	c.layout()
	c.layout()
	if _, exists := comparisonLabelBounds(c, "The source answer is still available"); !exists {
		t.Fatal("reconnecting hid the source before validating temporary lanes")
	}
	if len(replies) != 2 {
		t.Fatalf("reconnect did not recheck both temporary lanes: %d", len(replies))
	}
	replies[0](nil)
	c.layout()
	if _, exists := comparisonLabelBounds(c, "The source answer is still available"); !exists {
		t.Fatal("one successful attachment hid the source before the other lane was checked")
	}
	replies[1](errors.New("session no longer exists"))
	for range 3 {
		c.layout()
	}
	if _, exists := comparisonLabelBounds(c, "The source answer is still available"); !exists {
		t.Fatal("a comparison lost at restart hid the original answer")
	}
	if c.view.ensureState().hasComparison() {
		t.Fatal("a comparison with a missing temporary lane remained active")
	}
	if _, exists := comparisonLabelBounds(c, "Keep this one"); exists {
		t.Fatal("a missing temporary lane still offered Keep this one")
	}
	if len(replies) != 2 {
		t.Fatal("a dead comparison retried its attachments every frame")
	}
}
