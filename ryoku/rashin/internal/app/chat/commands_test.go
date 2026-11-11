package chat

import (
	"image"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func offering(t *testing.T, text string, commands ...string) *conversation {
	t.Helper()
	plan := &protocol.CommandPlan{Summary: "Add it up"}
	for _, command := range commands {
		plan.Commands = append(plan.Commands, protocol.PlannedCommand{Command: command, Tier: protocol.TierRead})
	}
	c := newConversation(image.Pt(900, 900))
	c.model.Apply(testEvent(t, 1, protocol.EvUser, protocol.UserData{Text: "How big is it?"}))
	c.model.Apply(testEvent(t, 2, protocol.EvTurnStart, protocol.TurnStartData{Turn: 1}))
	c.model.Apply(testEvent(t, 3, protocol.EvText, protocol.TextData{Message: "m1", Delta: text}))
	c.model.Apply(testEvent(t, 4, protocol.EvAnswer, protocol.AnswerData{Plan: plan}))
	c.model.Apply(testEvent(t, 5, protocol.EvTurnEnd, protocol.TurnEndData{Turn: 1, Stop: "end_turn"}))
	c.layout()
	return c
}

func (c *conversation) runPlaques() (inText, underAnswer int) {
	for key := range c.view.state.plaques {
		switch {
		case !strings.HasSuffix(key, ":run"):
		case strings.HasPrefix(key, "text:"):
			inText++
		default:
			underAnswer++
		}
	}
	return inText, underAnswer
}

// An answer that shows a command and also offers to run it used to show it
// twice, as a block of code and again in a card under the answer. The slab
// that runs it now stands where the text shows it, and nowhere else.
func TestAnOfferedCommandStandsOnceWhereTheTextShowsIt(t *testing.T) {
	c := offering(t, "Run this:\n\n```bash\n$ du -sh ~\n```\n\nIt adds up your home folder. The kernel is another matter:\n\n```\nuname -r\n```", "du -sh ~")
	if inText, underAnswer := c.runPlaques(); inText != 1 || underAnswer != 0 {
		t.Fatalf("the command can be run from %d places in the text and %d under the answer, want one in the text", inText, underAnswer)
	}
	if blocks := len(c.view.state.codes); blocks != 1 {
		t.Fatalf("%d blocks of code are shown, want only the one the answer does not offer to run", blocks)
	}
}

// Two commands shown in one block are still one offer.
func TestCommandsShownTogetherAreOneOffer(t *testing.T) {
	c := offering(t, "Run both:\n\n```\ndu -sh ~\ndf -h\n```", "du -sh ~", "df -h")
	if inText, underAnswer := c.runPlaques(); inText != 2 || underAnswer != 0 {
		t.Fatalf("%d and %d run buttons, want the two commands in the text", inText, underAnswer)
	}
	if blocks := len(c.view.state.codes); blocks != 0 {
		t.Fatalf("%d blocks of code are still shown", blocks)
	}
}

// A command the text never shows is offered under the answer, as before.
func TestACommandTheTextDoesNotShowIsOfferedUnderTheAnswer(t *testing.T) {
	c := offering(t, "Your home folder can be added up.", "du -sh ~")
	if inText, underAnswer := c.runPlaques(); inText != 0 || underAnswer != 1 {
		t.Fatalf("%d and %d run buttons, want one under the answer", inText, underAnswer)
	}
}
