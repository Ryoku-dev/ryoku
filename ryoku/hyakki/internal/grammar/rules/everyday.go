package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"strings"
)

type Everyday struct{ expression expr.Expr }

func NewEveryday() *Everyday                         { return &Everyday{expression: everydayExpression()} }
func (r *Everyday) Expr() expr.Expr                  { return r.expression }
func (r *Everyday) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *Everyday) Description() string {
	return "This rule tries to sort out confusing the adjective `everyday` and the adverb `every day`."
}
func (r *Everyday) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) < 2 {
		return nil
	}
	norm := func(i int) string { return strings.ToLower(toks[i].GetStr(src)) }
	isws := func(i int) bool { return toks[i].Kind.IsWhitespace() }
	var span text.Span
	replacement, pos := "", ""
	sliceSpan := func(i int) text.Span { s, _ := doc.Span(toks[i : i+3]); return s }
	switch len(toks) {
	case 2:
		if norm(0) == "everyday" {
			span = toks[0].Span
			replacement = "every day"
			pos = "adverb"
		}
	case 3:
		if norm(0) == "everyday" && isws(1) {
			span = toks[0].Span
			replacement = "every day"
			pos = "adverb"
		} else if norm(2) == "everyday" && isws(1) {
			span = toks[2].Span
			replacement = "every day"
			pos = "adverb"
		}
	case 5:
		if isws(1) && isws(3) {
			if norm(0) == "every" && norm(2) == "day" {
				span = sliceSpan(0)
				replacement = "everyday"
				pos = "adjective"
			} else if norm(2) == "every" && norm(4) == "day" {
				span = sliceSpan(2)
				replacement = "everyday"
				pos = "adjective"
			} else if norm(0) == "everyday" {
				span = toks[0].Span
				replacement = "every day"
				pos = "adverb"
			}
		}
	case 6:
		if norm(0) == "every" && norm(2) == "day" && isws(1) && isws(3) {
			span = sliceSpan(0)
			replacement = "everyday"
			pos = "adjective"
		}
	}
	if replacement == "" {
		return nil
	}
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.WordChoice
	l.Priority = 31
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, span.GetContent(src))}
	l.Message = fmt.Sprintf("You probably mean the %s `%s` here.", pos, replacement)
	return &l
}
func everydayExpression() expr.Expr {
	everyday := expr.NewWord("everyday")
	everyDay := expr.NewSequenceExprWordSeq([]string{"every", "day"})
	punct := func(k doc.TokenKind) bool {
		p, ok := k.AsPunctuation()
		return ok && (p.IsQuestion() || p.IsComma() || p.IsPeriod())
	}
	badAfter := expr.NewAll([]expr.Expr{expr.NewSequenceExprWith(everyday).TWs().ThenAnyWord(), expr.NewSequenceExprAnything().TAny().ThenKindWhere(func(k doc.TokenKind) bool { return !k.IsNoun() && !k.IsOov() && !k.IsVerbProgressiveForm() })})
	badBefore := expr.NewAll([]expr.Expr{expr.NewSequenceExprAnyWord().TWs().Then(everyDay), expr.AsExpr(func(t doc.Token, _ []rune) bool { return t.Kind.IsDeterminer() && !t.Kind.IsPronoun() })})
	ambiverb := expr.NewAll([]expr.Expr{expr.NewSequenceExprWith(everyday).TWs().ThenAnyWord().TWs().ThenAnyWord(), expr.NewSequenceExprAnything().TAny().ThenKindBoth(doc.TokenKind.IsNoun, doc.TokenKind.IsVerb).TAny().ThenDeterminer()})
	punctuationAfter := expr.NewAll([]expr.Expr{expr.NewSequenceExprWith(everyday).ThenPunctuation(), expr.NewSequenceExprAnything().ThenKindWhere(punct)})
	nounPunctuation := expr.NewAll([]expr.Expr{expr.NewSequenceExprWith(everyDay).TWs().ThenPluralNoun().ThenPunctuation(), expr.NewSequenceExprAnything().TAny().TAny().TAny().TAny().ThenKindWhere(punct)})
	return expr.NewLongestMatchOf([]expr.Expr{badAfter, badBefore, ambiverb, punctuationAfter, nounPunctuation})
}
