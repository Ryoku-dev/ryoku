package chat

import (
	"image"
	"strconv"
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

// While a command runs, what it printed last is what is worth seeing: a long
// output shows its end. Once it has finished it reads from the top again.
func TestARunningCommandShowsTheEndOfItsOutput(t *testing.T) {
	c := newConversation(image.Pt(900, 900))
	lines := make([]string, 0, 30)
	for line := 1; line <= 30; line++ {
		lines = append(lines, "line "+strconv.Itoa(line))
	}
	c.model.Apply(testEvent(t, 1, protocol.EvUser, protocol.UserData{Run: "seq 30"}))
	c.model.Apply(testEvent(t, 2, protocol.EvTurnStart, protocol.TurnStartData{Turn: 1}))
	c.model.Apply(testEvent(t, 3, protocol.EvTool, protocol.ToolData{Call: "run", Title: "Running", Kind: protocol.ToolExecute, Status: protocol.ToolRunning, Input: "seq 30", Terminal: "run"}))
	c.model.Apply(testEvent(t, 4, protocol.EvTerminal, protocol.TerminalData{Terminal: "run", Delta: strings.Join(lines, "\n") + "\n"}))
	shown := func() string {
		c.layout()
		c.layout()
		for _, text := range c.view.state.selectables {
			if strings.Contains(text.Text(), "line ") {
				return text.Text()
			}
		}
		return ""
	}
	if text := shown(); !strings.HasSuffix(text, "line 30") || strings.Contains(text, "line 5\n") {
		t.Fatalf("while it runs the output shows %q", text)
	}
	// the running line carries the flakes, so the foot of the column does not say Thinking under it
	if marked, _ := tailMark(c.model.state); marked {
		t.Fatal("the foot of the column shows a second mark under a running command")
	}
	c.model.Apply(testEvent(t, 5, protocol.EvTool, protocol.ToolData{Call: "run", Status: protocol.ToolDone}))
	c.model.Apply(testEvent(t, 6, protocol.EvTurnEnd, protocol.TurnEndData{Turn: 1, Stop: "end_turn"}))
	// it folded when it finished; the reader opens it again
	c.layout()
	for _, visual := range c.view.state.work {
		visual.open = true
	}
	if text := shown(); !strings.HasPrefix(text, "line 1\n") || strings.Contains(text, "line 30") {
		t.Fatalf("once it has finished the output shows %q", text)
	}
}
