package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
)

type FriendOfMe struct{ expression expr.Expr }

func NewFriendOfMe() *FriendOfMe {
	return &FriendOfMe{expression: expr.NewSequenceExprWordSet([]string{"friend", "friends", "enemy", "enemies"}).TWs().TAco("of").TWs().ThenObjectPronoun()}
}
func (r *FriendOfMe) Expr() expr.Expr                  { return r.expression }
func (r *FriendOfMe) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *FriendOfMe) Description() string {
	return "Corrects wrong pronoun usage in constructions like `a friend of me`."
}
func (r *FriendOfMe) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) == 0 {
		return nil
	}
	tok := toks[len(toks)-1]
	orig := tok.GetStr(src)
	var replacement string
	switch orig {
	case "me":
		replacement = "mine"
	case "you":
		replacement = "yours"
	case "him":
		replacement = "his"
	case "us":
		replacement = "ours"
	case "them":
		replacement = "theirs"
	default:
		return nil
	}
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.Grammar
	l.Priority = 31
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, tok.GetCh(src))}
	l.Message = fmt.Sprintf("Use `%s` instead of `%s`.", replacement, orig)
	return &l
}
