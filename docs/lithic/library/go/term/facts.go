package term

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/tokens"
)

// Fact is one label and value row.
type Fact struct {
	Label   string
	Value   string
	Literal bool
}

// Facts describes a fact table.
type Facts struct {
	Rows       []Fact
	LabelWidth int
}

// RenderFacts renders aligned facts separated by vein rules.
func RenderFacts(th Theme, facts Facts) string {
	labelWidth := max(facts.LabelWidth, tokens.SizeFactsLabelWidth/tokens.SpaceS2)
	rows := make([]string, 0, len(facts.Rows)*2)
	for i, row := range facts.Rows {
		value := row.Value
		if value == "" {
			value = "-"
		}
		label := ink(th.Roles.Fg3).Render(fit(row.Label, labelWidth))
		line := label + "  " + ink(th.Roles.Fg).Render(value)
		if i != 0 {
			rows = append(rows, ink(th.Roles.Vein).Render(strings.Repeat(glyphs().FrameTop, lipgloss.Width(line))))
		}
		rows = append(rows, line)
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}
