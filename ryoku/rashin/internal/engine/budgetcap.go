package engine

// Derived from oh-my-pi (MIT), packages/agent/src/output-budget.ts@c7fff67

import (
	"strconv"
	"strings"
	"time"

	"ryoku-rashin/internal/llm"
)

// Session ceiling option keys. They ride in Options.Options beside
// protocol.OptionPlan and protocol.OptionEffort, but the user never sets
// them in a picker: goal and dream runs, and team tasks with a budget, pass
// theirs when they create the session. Values are decimal integers; 0 or
// absent is no ceiling.
const (
	// OptionMaxMinutes caps the wall-clock time the session's turns may
	// spend, all turns together.
	OptionMaxMinutes = "maxMinutes"
	// OptionMaxTokens caps the tokens (input and output) the session's
	// model calls may spend, subagents included.
	OptionMaxTokens = "maxTokens"
)

// ceilings is one session's running totals against its caps. The runner
// keeps it across turns and in State, so a resumed session does not start
// over.
type ceilings struct {
	Wall   time.Duration `json:"wall,omitempty"`
	Tokens int           `json:"tokens,omitempty"`

	SpentWall   time.Duration `json:"spentWall,omitempty"`
	SpentTokens int           `json:"spentTokens,omitempty"`
}

// ceilingsFrom reads the caps out of session options.
func ceilingsFrom(options map[string]string) ceilings {
	var c ceilings
	if m := positive(options[OptionMaxMinutes]); m > 0 {
		c.Wall = time.Duration(m) * time.Minute
	}
	c.Tokens = positive(options[OptionMaxTokens])
	return c
}

func positive(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// spend adds a model call's tokens.
func (c *ceilings) spend(u llm.Usage) { c.SpentTokens += u.Input + u.Output }

// exceeded reports a cap reached, with started the running turn's start:
// "time", "tokens" or "".
func (c ceilings) exceeded(started time.Time) string {
	switch {
	case c.Wall > 0 && c.SpentWall+time.Since(started) >= c.Wall:
		return "time"
	case c.Tokens > 0 && c.SpentTokens >= c.Tokens:
		return "tokens"
	}
	return ""
}

// remaining is the wall-clock time left for a turn started at started;
// zero means no cap.
func (c ceilings) remaining(started time.Time) time.Duration {
	if c.Wall <= 0 {
		return 0
	}
	left := c.Wall - c.SpentWall - time.Since(started)
	if left <= 0 {
		return time.Nanosecond
	}
	return left
}

// endTurn books a finished turn's time.
func (c *ceilings) endTurn(started time.Time) { c.SpentWall += time.Since(started) }
