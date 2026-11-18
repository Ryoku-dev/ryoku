package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"strings"
)

type QuiteQuiet struct{ expression expr.Expr }

func NewQuiteQuiet() *QuiteQuiet                       { return &QuiteQuiet{expression: quiteQuietExpr()} }
func (r *QuiteQuiet) Expr() expr.Expr                  { return r.expression }
func (r *QuiteQuiet) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*QuiteQuiet) Description() string {
	return "Helps distinguish between ‘quiet’ (making ‘little noise’) and ‘quite’ (meaning ‘rather’)."
}
func (r *QuiteQuiet) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	span, _ := doc.Span(toks)
	s := strings.ToLower(span.GetContentString(src))
	var tok doc.Token
	var target, message string
	if strings.HasSuffix(s, "quite") {
		tok = toks[len(toks)-1]
		target = "quiet"
		message = "‘Quite’ might be a typo here. It means ‘rather’ but you might be trying to say ‘quiet’ (not noisy)."
	} else if strings.HasPrefix(s, "quiet") || strings.HasSuffix(s, "quiet") {
		tok = toks[0]
		if !strings.HasPrefix(s, "quiet") {
			tok = toks[len(toks)-1]
		}
		target = "quite"
		message = "‘Quiet’ might be a typo here. It means ‘not noisy’ but you might be trying to say ‘quite’ (rather)."
	} else {
		return nil
	}
	return &lint.Lint{Span: tok.Span, LintKind: lint.Typo, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr(target, tok.GetCh(src))}, Message: message, Priority: 63}
}

func quiteQuietExpr() expr.Expr {
	quiet := expr.NewSequenceExprAco("quiet").TWs().Then(func(t doc.Token, src []rune) bool {
		k := t.Kind
		return (k.IsAdjective() || k.IsAdverb() || k.IsVerb()) && !k.IsNoun() && !k.IsPreposition() && !k.IsConjunction() && !text.EqAnyIgnoreASCIICaseStr(t.GetCh(src), []string{"here", "there", "too", "already", "lately"})
	})
	negative := expr.NewSequenceExprWith(func(t doc.Token, src []rune) bool {
		return t.Kind.IsVerb() && t.Kind.IsApostrophized() && text.EndsWithAnyIgnoreASCIICaseChars(t.GetCh(src), [][]rune{[]rune("n't"), []rune("n’t")})
	}).TWs().TAco("quiet")
	adverb := expr.NewSequenceExpr().ThenKindExcept(doc.TokenKind.IsAdverb, []string{"actually", "never", "not", "probably", "really", "generally"}).TWs().TAco("quite")
	return expr.NewFirstMatchOf([]expr.Expr{quiet, negative, adverb})
}
