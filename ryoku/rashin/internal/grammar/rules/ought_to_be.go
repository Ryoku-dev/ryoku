package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

// OughtToBe detects the eggcorn `out to be` when the intended phrase is `ought to be`.
// Legitimate phrasal-verb uses like `turn out to be`, `work out to be`, or
// `make it out to be` are ignored.
type OughtToBe struct{ expression *expr.ExprMap[int] }

func NewOughtToBe() *OughtToBe {
	e := expr.NewExprMap[int]()
	e.Insert(expr.NewSequenceExpr().ThenKindIsButIsNot(doc.TokenKind.IsWord, doc.TokenKind.IsVerb).ThenWhitespace().ThenPronoun().ThenWhitespace().ThenFixedPhrase("out to be"), 4)
	e.Insert(expr.NewSequenceExprWith(expr.NewAnchorStart()).ThenPronoun().ThenWhitespace().ThenFixedPhrase("out to be"), 2)
	e.Insert(expr.NewSequenceExpr().ThenPunctuation().ThenWhitespace().ThenPronoun().ThenWhitespace().ThenFixedPhrase("out to be"), 4)
	return &OughtToBe{e}
}
func (r *OughtToBe) Expr() expr.Expr                  { return r.expression }
func (r *OughtToBe) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *OughtToBe) Description() string {
	return "Detects the mistaken `out to be` and suggests `ought to be`, while ignoring legitimate phrasal-verb uses such as `turn out to be` and `make it out to be`."
}
func (r *OughtToBe) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	i, ok := r.expression.Lookup(0, toks, src)
	if !ok || i >= len(toks) {
		return nil
	}
	result := lint.NewLint()
	result.Span = toks[i].Span
	result.LintKind = lint.Eggcorn
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("ought", toks[i].GetCh(src))}
	result.Message = "Did you mean `ought to be` (expressing expectation or obligation)?"
	result.Priority = 31
	return &result
}
