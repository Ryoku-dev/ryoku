package weir

import (
	"ryoku-rashin/internal/grammar/doc"
	"testing"
)

func TestMetadataValidation(t *testing.T) {
	base := "expr main old\nlet description \"description\"\nlet message \"message\"\nlet becomes \"new\"\n"
	for _, tc := range []struct {
		source string
		kind   ErrorKind
	}{{`let strategy ""`, InvalidReplacementStrategy}, {`let strategy "wrong"`, InvalidReplacementStrategy}, {`let scope ""`, InvalidScope}, {`let kind ""`, InvalidLintKind}, {`let kind "wrong"`, InvalidLintKind}, {`let strategy []`, ExpectedDifferentVariableType}, {`let description []`, ExpectedDifferentVariableType}, {`let message []`, ExpectedDifferentVariableType}, {`let becomes [["new"]]`, UnexpectedToken}} {
		t.Run(tc.source, func(t *testing.T) {
			_, err := NewWeirLinter(base + tc.source)
			e, ok := err.(Error)
			if !ok || e.Kind != tc.kind {
				t.Fatalf("got %v; want error kind %v", err, tc.kind)
			}
		})
	}
}
func TestLastVariableAndExpressionDefinitionsWin(t *testing.T) {
	source := "expr main old\nexpr main newer\nlet description \"first\"\nlet description \"last\"\nlet message \"message\"\nlet becomes \"new\"\n"
	r, err := NewWeirLinter(source)
	if err != nil {
		t.Fatal(err)
	}
	if r.Description() != "last" {
		t.Fatal(r.Description())
	}
	if len(r.Lint(doc.NewPlainEnglishCurated("old"))) != 0 {
		t.Fatal("old expression retained")
	}
	if fail := r.RunTest(TestCase{Input: "newer", Expected: "new"}); fail != nil {
		t.Fatal(fail)
	}
}
func TestUnderscoreDeclarationDelimiterMatchesHarper(t *testing.T) {
	a, err := Parse("expr clause_start [when, if]")
	if err != nil {
		t.Fatal(err)
	}
	n, ok := a.GetExpr("clause")
	if !ok || n.Kind != Seq || len(n.Children) != 3 || n.Children[0].Text != "start" {
		t.Fatalf("unexpected declaration: %#v", a)
	}
}
