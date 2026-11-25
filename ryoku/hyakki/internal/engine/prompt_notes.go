package engine

// Derived from oh-my-pi (MIT), packages/coding-agent/src/prompts/system/date-cwd-reminder.md@c7fff67
// Derived from oh-my-pi (MIT), packages/coding-agent/src/prompts/system/plan-mode-approved.md@c7fff67

import (
	"fmt"
	"strings"
	"time"

	"ryoku-rashin/internal/protocol"
)

// Reminder is the note the runner puts ahead of a user turn, not in the
// system prompt, so the prompt and the history before it stay cached: the
// day, the session folder and, for an Ask from a terminal, that terminal.
// The runner attaches it when it differs from the last one it attached.
func Reminder(now time.Time, cwd string, terminal *protocol.TerminalContext) string {
	var b strings.Builder
	b.WriteString("<system-reminder>\n")
	fmt.Fprintf(&b, "Today is %s.", now.Format("Monday 2 January 2006"))
	if cwd != "" {
		fmt.Fprintf(&b, " The session folder is %s.", cwd)
	}
	b.WriteString(" Use this as context and leave it out of your reply.\n")
	if t := terminal; t != nil {
		b.WriteString("Terminal context:\n")
		if t.Cwd != "" {
			fmt.Fprintf(&b, "- cwd: %s\n", t.Cwd)
		}
		if t.Shell != "" {
			fmt.Fprintf(&b, "- shell: %s\n", t.Shell)
		}
		if t.LastCommand != "" {
			fmt.Fprintf(&b, "- last command: %s\n", t.LastCommand)
		}
		if t.LastStatus != nil {
			fmt.Fprintf(&b, "- last exit status: %d\n", *t.LastStatus)
		}
		fmt.Fprintf(&b, "- buffer: %t\n", t.Buffer)
	}
	b.WriteString("</system-reminder>")
	return b.String()
}

// PlanApproved ends Plan first: it tells the model the user approved the plan,
// the plan's text as the todo list holds it, and the tools that change
// files are back.
func PlanApproved(plan string) string {
	var b strings.Builder
	b.WriteString("<system-reminder>\nThe user approved your plan. You now hold the tools that change files: carry the plan out top to bottom, keep the todo list current as each step is done, and finish with the checks the plan names.\n")
	if plan = strings.TrimSpace(plan); plan != "" {
		fmt.Fprintf(&b, "<plan>\n%s\n</plan>\n", plan)
	}
	b.WriteString("</system-reminder>")
	return b.String()
}
