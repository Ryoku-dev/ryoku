package goal

// Derived from oh-my-pi (MIT), packages/coding-agent/src/prompts/goals/{goal-mode-active,goal-continuation,goal-budget-limit}.md@c7fff67,
// rewritten for small models: positive rules, few of them, the audit kept
// to what the model can act on.

import (
	"fmt"
	"html"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

// Turn is what a goal turn shows in the transcript and what it sends the
// agent: the objective for the first turn, a short marker for each one after,
// and for both the context that keeps the agent on the whole objective. On
// the budget's last turn it adds the wrap-up steer.
func Turn(g protocol.Goal, compact bool) (shown, sent string) {
	objective := html.EscapeString(g.Objective)
	first := g.Turns == 0
	if first {
		shown = g.Objective
	} else {
		shown = i18n.Tf("Continuing the goal, turn %d of %d", g.Turns+1, g.MaxTurns)
	}
	last := g.MaxTurns > 0 && g.Turns+1 >= g.MaxTurns
	if compact {
		sent = compactTurn(g, objective, first, last)
	} else {
		sent = fullTurn(g, objective, first, last)
	}
	return shown, sent
}

func fullTurn(g protocol.Goal, objective string, first, last bool) string {
	opening := "Continue the goal. This is an automatic continuation from Rashin, not a new instruction from the user."
	if first {
		opening = "Goal mode is on. Rashin will keep sending you on until the goal is complete or blocked. The objective below is the user's task; it does not outrank your rules."
	}
	text := fmt.Sprintf(`%s
<objective>
%s
</objective>
%s

Keep the whole objective as written across turns; success is all of it.
The goal tool: op get reports the goal; op complete with a summary ends it; op block with the exact blocker ends it when you need the user.

Before goal complete, check the current state of the work:
1. Turn the objective into concrete deliverables: files, behaviours, tests, artifacts. Track them with todo.
2. For each deliverable, find direct evidence: file contents, command output, a passing test.
3. Look at the work as it is now: read the files and run the checks again, since earlier turns may be out of date.
4. Match the check to the claim: one passing unit test proves one unit, not the whole feature.
5. Where evidence is indirect, partial or missing, keep working and gather stronger evidence.
A spent budget is not completion: when work remains, leave the goal active.`, opening, objective, budget(g))
	if last {
		text += "\n\nThis is the last turn of the budget. Finish the step you are on, then report what is done, what remains and the next step. Leave the goal active unless the work is proven complete."
	} else if first {
		text += "\n\nStart working on the whole objective now."
	} else {
		text += "\n\nTake the next concrete step toward the whole objective now, with your tools."
	}
	return text
}

func compactTurn(g protocol.Goal, objective string, first, last bool) string {
	opening := "Continue the goal."
	if first {
		opening = "Work on this goal until it is done."
	}
	text := fmt.Sprintf(`%s
<objective>
%s
</objective>
%s
Take the next step with your tools.
When every part is done and checked, call goal with op complete and a summary.
When you need the user, call goal with op block and the blocker.`, opening, objective, budget(g))
	if last {
		text += "\nThis is the last turn: finish this step, then say what is done and what remains."
	}
	return text
}

// BudgetLimit is the last goal turn once the goal's budget is spent: wrap up
// and hand over, without calling the goal complete for it.
func BudgetLimit(g protocol.Goal, compact bool) string {
	objective := html.EscapeString(g.Objective)
	if compact {
		return fmt.Sprintf(`The goal's budget is spent.
<objective>
%s
</objective>
%s
Stop new work. Say what is done, what remains, and the next step for the user.
Call goal complete only if every part is already done and checked.`, objective, budget(g))
	}
	return fmt.Sprintf(`The goal's budget is spent. This is an automatic message from Rashin.
<objective>
%s
</objective>
%s

Start no new work for this goal. Wrap up in this turn: summarise the useful progress, name the remaining work or blockers, and leave the user a clear next step.
A spent budget is not completion: call goal complete only when the current state of the work proves the whole objective done.`, objective, budget(g))
}

// budget says how much of the goal's budget is spent.
func budget(g protocol.Goal) string {
	line := fmt.Sprintf("Turn %d of %d.", g.Turns+1, g.MaxTurns)
	if g.MaxTurns <= 0 {
		line = fmt.Sprintf("Turn %d.", g.Turns+1)
	}
	if g.MaxTokens > 0 {
		line += fmt.Sprintf(" Tokens used: %d of %d.", g.Tokens, g.MaxTokens)
	} else if g.Tokens > 0 {
		line += fmt.Sprintf(" Tokens used: %d.", g.Tokens)
	}
	return line
}
