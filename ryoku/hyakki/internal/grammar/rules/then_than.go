package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

// ThenThan corrects the misuse of `then` to `than`.
type ThenThan struct{ expression expr.Expr }

func NewThenThan() *ThenThan {
	comparison := expr.NewAll([]expr.Expr{expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExprWith(thenThanIsComparative).TWs().TAco("then").TWs().ThenUnless(expr.NewWord("that")), expr.NewSequenceExprWordSet([]string{"more", "less"}).TWs().ThenKindEither(doc.TokenKind.IsAdjective, doc.TokenKind.IsAdverb).TWs().TAco("then").TWs().ThenUnless(expr.NewWord("that"))}), expr.AsExpr(expr.NewInvert(expr.NewWordSet([]string{"back", "this", "so", "but"})))})
	e := expr.NewFirstMatchOf([]expr.Expr{comparison, expr.NewFixedPhraseFromPhrase("easier said then done"), expr.NewFixedPhraseFromPhrase("now and than"), expr.NewFixedPhraseFromPhrase("other then"), expr.NewFixedPhraseFromPhrase("rather then"), expr.NewFixedPhraseFromPhrase("than again"), expr.NewFixedPhraseFromPhrase("until than")})
	return &ThenThan{expression: e}
}
func (r *ThenThan) Expr() expr.Expr                  { return r.expression }
func (r *ThenThan) Description() string              { return "Corrects mixing up `then` and `than`." }
func (r *ThenThan) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ThenThan) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	tok := lint.FindTheOnlyTokenMatching(t, s, func(t *doc.Token, s []rune) bool {
		return text.EqAnyIgnoreASCIICaseStr(t.GetCh(s), []string{"then", "than"})
	})
	if tok == nil {
		return nil
	}
	word := "then"
	if text.EqStr(tok.GetCh(s), "then") {
		word = "than"
	}
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.Miscellaneous
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(word, tok.GetCh(s))}
	l.Message = "Did you mean `" + word + "`?"
	l.Priority = 31
	return &l
}
func thenThanIsComparative(t doc.Token, s []rune) bool {
	return t.Kind.IsComparativeAdjective() || text.EqAnyIgnoreASCIICaseStr(t.GetCh(s), []string{"less", "more"})
}
