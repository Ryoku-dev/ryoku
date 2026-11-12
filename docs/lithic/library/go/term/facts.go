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

// Facts describes a fact table. Width bounds the whole row, so values wrap
// in what is left after the label column and the gap; 0 leaves them unbounded.
type Facts struct {
	Rows       []Fact
	LabelWidth int
	Width      int
}

// RenderFacts renders aligned facts separated by vein rules. Labels keep their
// column (140 over s2 cells) and values start s4 over s2 cells after it. A
// longer label wraps inside its column, never eliding, and a row grows to its
// taller side with both sides centred in it.
func RenderFacts(th Theme, facts Facts) string {
	labelWidth := max(facts.LabelWidth, tokens.SizeFactsLabelWidth/tokens.SpaceS2)
	gap := tokens.SpaceS4 / tokens.SpaceS2
	valueWidth := 0
	if facts.Width > 0 {
		labelWidth = min(labelWidth, max(1, facts.Width-gap-1))
		valueWidth = max(1, facts.Width-labelWidth-gap)
	}
	blocks := make([][]string, 0, len(facts.Rows))
	width := facts.Width
	for _, row := range facts.Rows {
		value := row.Value
		if value == "" {
			value = "-"
		}
		labels := wrapLines(row.Label, labelWidth)
		values := []string{value}
		if valueWidth > 0 {
			values = wrapLines(value, valueWidth)
		}
		height := max(len(labels), len(values))
		labelTop, valueTop := (height-len(labels))/2, (height-len(values))/2
		lines := make([]string, height)
		for i := range lines {
			label, value := "", ""
			if i >= labelTop && i-labelTop < len(labels) {
				label = labels[i-labelTop]
			}
			if i >= valueTop && i-valueTop < len(values) {
				value = values[i-valueTop]
			}
			lines[i] = ink(th.Roles.Fg3).Render(fit(label, labelWidth)) + strings.Repeat(" ", gap)
			if value != "" {
				lines[i] += ink(th.Roles.Fg).Render(value)
			}
			if facts.Width == 0 {
				width = max(width, lipgloss.Width(lines[i]))
			}
		}
		blocks = append(blocks, lines)
	}
	rule := ink(th.Roles.Vein).Render(strings.Repeat(glyphs().Rule, width))
	rows := make([]string, 0, len(blocks)*3)
	for i, lines := range blocks {
		if i != 0 {
			rows = append(rows, rule)
		}
		rows = append(rows, lines...)
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// wrapLines breaks text at words, and inside a word only when it cannot fit.
func wrapLines(text string, width int) []string {
	lines := strings.Split(lipgloss.Wrap(text, width, ""), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return lines
}
