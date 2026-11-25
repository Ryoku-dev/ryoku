package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type ModalOf struct{ expression expr.Expr }

func NewModalOf() *ModalOf {
	modal := expr.ButNot(expr.NewSequenceExprWith(expr.NewModalVerb()).ThenWhitespace().TAco("of"), expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExprWith(expr.NewWord("can")), expr.NewSequenceExprWith(expr.NewWordExact("May"))}))
	noun := expr.NewSequenceExprWordSet([]string{"the", "a"}).ThenWhitespace().TAco("will").ThenWhitespace().TAco("of")
	course := expr.NewSequenceExprWhitespace().TAco("course")
	anymight := expr.NewSequenceExprAnyWord().ThenWhitespace().TAco("might").ThenWhitespace().TAco("of")
	return &ModalOf{expression: expr.NewLongestMatchOf([]expr.Expr{expr.NewSequenceExprWith(anymight).Then(course), expr.NewSequenceExprWith(modal).Then(course), anymight, noun, modal})}
}
func (r *ModalOf) Expr() expr.Expr                  { return r.expression }
func (r *ModalOf) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ModalOf) Description() string {
	return "Detects `of` mistakenly used with `would`, `could`, `should`, etc."
}
func (r *ModalOf) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	idx := 0
	switch len(toks) {
	case 3:
	case 5:
		if toks[len(toks)-1].GetStr(src) != "of" {
			return nil
		}
		if toks[0].Kind.IsAdjective() || toks[0].Kind.IsDeterminer() {
			return nil
		}
		idx = 2
	case 7:
		return nil
	default:
		panic("unexpected modal-of match length")
	}
	span, _ := doc.Span(toks[idx : idx+3])
	return &lint.Lint{Span: span, LintKind: lint.WordChoice, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr(toks[idx].GetStr(src)+" have", span.GetContent(src))}, Message: "Use `have` rather than `of` here.", Priority: 126}
}
