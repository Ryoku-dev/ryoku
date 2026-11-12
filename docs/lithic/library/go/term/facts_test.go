package term

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
)

var sgr = regexp.MustCompile("\x1b\\[[0-9;:]*m")

func TestRenderFactsWrapsLongLabels(t *testing.T) {
	th := Theme{Roles: palette.Signature(true)}
	out := sgr.ReplaceAllString(RenderFacts(th, Facts{Width: 40, Rows: []Fact{
		{Label: "Server", Value: "ally"},
		{Label: "Snapshot kept before the last update", Value: "Yesterday"},
	}}), "")
	lines := strings.Split(out, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	// The 17 cell column, two cells of gap, the value centred beside a
	// three-line label, and one rule across the table between the rows
	want := []string{
		"Server             ally",
		strings.Repeat("─", 40),
		"Snapshot kept",
		"before the last    Yesterday",
		"update",
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("RenderFacts =\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}
