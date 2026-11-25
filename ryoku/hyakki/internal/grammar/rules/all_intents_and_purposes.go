package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"strings"
	"unicode"
)

type AllIntentsAndPurposes struct{ expression expr.Expr }

func NewAllIntentsAndPurposes() *AllIntentsAndPurposes {
	return &AllIntentsAndPurposes{expression: expr.ButNot(expr.NewSequenceExpr().ThenPreposition().TWs().TAco("all").TWs().ThenAnyOf([]expr.Expr{expr.NewSequenceExprWordSet([]string{"intents", "extents", "intense"}).TWs().TAco("and"), expr.NewSequenceExprWordSet([]string{"intended", "intense", "intensive", "intrinsic"})}).TWs().TSet([]string{"purposes", "purpose"}), expr.NewSequenceExprWordSet([]string{"for", "to"}).TWs().ThenWordSeq([]string{"all", "intents", "and", "purposes"}))}
}
func (r *AllIntentsAndPurposes) Expr() expr.Expr { return r.expression }
func (r *AllIntentsAndPurposes) Description() string {
	return "Finds and corrects common wrong forms of the phrase 'for all intents and purposes' / 'to all intents and purposes'."
}
func (r *AllIntentsAndPurposes) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *AllIntentsAndPurposes) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	span, ok := doc.Span(t)
	if !ok {
		return nil
	}
	prep := t[0].GetCh(s)
	f, to, rest := "for", "to", " all intents and purposes"
	if len(prep) > 1 && unicode.IsUpper(prep[0]) && unicode.IsUpper(prep[1]) {
		f, to, rest = "FOR", "TO", " ALL INTENTS AND PURPOSES"
	} else if len(prep) > 0 && unicode.IsUpper(prep[0]) {
		f, to = "For", "To"
	}
	if text.StartsWithIgnoreASCIICaseStr(prep, "to") {
		f, to = to, f
	}
	return &lint.Lint{Span: span, LintKind: lint.Nonstandard, Suggestions: []lint.Suggestion{lint.ReplaceWith([]rune(f + rest)), lint.ReplaceWith([]rune(to + rest))}, Message: fmt.Sprintf("The correct form is '%s all intents and purposes'.", strings.ToLower(string(prep))), Priority: 50}
}
