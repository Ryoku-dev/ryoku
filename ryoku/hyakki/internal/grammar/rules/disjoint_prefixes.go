package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type DisjointPrefixes struct {
	expression expr.Expr
	dictionary dict.Dictionary
}

func NewDisjointPrefixes(dictionary dict.Dictionary) *DisjointPrefixes {
	return &DisjointPrefixes{expression: expr.ButNot(expr.NewSequenceExprWordSet([]string{"anti", "auto", "bi", "counter", "de", "dis", "extra", "fore", "hyper", "il", "im", "inter", "ir", "macro", "mal", "micro", "mid", "mini", "mis", "mono", "multi", "non", "omni", "post", "pre", "pro", "re", "semi", "sub", "super", "trans", "tri", "ultra", "un", "uni", "co", "out", "over", "under", "up"}).TWsH().ThenKindEither(doc.TokenKind.IsVerb, doc.TokenKind.IsNoun).ThenOptionalHyphen(), expr.NewSequenceExprAnyOf([]expr.Expr{expr.NewSequenceExprAnything().TAny().TAny().ThenHyphen(), expr.NewSequenceExprAco("co").TAny().TSet([]string{"op", "ops"}), expr.NewSequenceExprAco("out").TWs().TSet([]string{"boxes", "facing", "guesses", "live", "numbers", "playing"}), expr.NewSequenceExprAco("over").TWs().TSet([]string{"all", "joy", "long", "night", "reading", "steps", "time"}), expr.NewSequenceExprAco("under").TWs().TSet([]string{"development", "mine"}), expr.NewSequenceExprAco("up").TWs().TSet([]string{"loading", "right", "state", "time", "trend"})})), dictionary: dictionary}
}
func (r *DisjointPrefixes) Expr() expr.Expr { return r.expression }
func (r *DisjointPrefixes) Description() string {
	return "Looks for words with their prefixes written with a space or hyphen between instead of joined."
}
func (r *DisjointPrefixes) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *DisjointPrefixes) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}
func (r *DisjointPrefixes) MatchToLintWithContext(t []doc.Token, s []rune, c *lint.MatchContext) *lint.Lint {
	if len(t) < 3 || c == nil {
		return nil
	}
	if n := len(c.Before); n > 0 && (c.Before[n-1].Kind.IsHyphen() || c.Before[n-1].Kind.IsApostrophe()) {
		return nil
	}
	sep := " "
	if t[1].Kind.IsHyphen() {
		sep = "-"
	}
	a, b := t[0].GetStr(s), t[2].GetStr(s)
	if r.dictionary.ContainsWordStr(a + sep + b) {
		return nil
	}
	var suggestions []lint.Suggestion
	if sep != "-" && r.dictionary.ContainsWordStr(a+"-"+b) {
		suggestions = append(suggestions, lint.ReplaceWithMatchCaseStr(a+"-"+b, t[0].GetCh(s)))
	}
	if r.dictionary.ContainsWordStr(a + b) {
		suggestions = append(suggestions, lint.ReplaceWithMatchCaseStr(a+b, t[0].GetCh(s)))
	}
	if len(suggestions) == 0 {
		return nil
	}
	result := lint.NewLint()
	result.Span = text.Span{Start: t[0].Span.Start, End: t[len(t)-1].Span.End}
	result.LintKind = lint.Spelling
	result.Suggestions = suggestions
	result.Message = "This looks like a prefix that can be joined with the rest of the word."
	return &result
}
