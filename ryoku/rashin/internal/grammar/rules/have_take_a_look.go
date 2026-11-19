package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type HaveTakeALook struct {
	expression expr.Expr
	dialect    dict.Dialect
}

func NewHaveTakeALook(dialect dict.Dialect) *HaveTakeALook {
	words := []string{"have", "had", "had", "has", "having"}
	if dialect == dict.British || dialect == dict.Australian {
		words = []string{"take", "took", "taken", "takes", "taking"}
	}
	return &HaveTakeALook{expression: expr.NewSequenceExprWordSet(words).TWs().ThenFixedPhrase("a look"), dialect: dialect}
}
func (r *HaveTakeALook) Expr() expr.Expr { return r.expression }
func (r *HaveTakeALook) Description() string {
	return "Corrects either `have a look` or `take a look` to the other, depending on the dialect."
}
func (r *HaveTakeALook) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *HaveTakeALook) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Regionalism
	result.Priority = 63
	tok := toks[0]
	word := strings.ToLower(tok.GetStr(src))
	translated, ok := haveTakeALookVariants[word]
	if !ok {
		return nil
	}
	labels := make([]string, 0, len(translated))
	for _, s := range translated {
		result.Suggestions = append(result.Suggestions, lint.ReplaceWithMatchCaseStr(s, tok.GetCh(src)))
		labels = append(labels, "`"+s+" a look`")
	}
	result.Span = tok.Span
	result.Message = fmt.Sprintf("%s English prefers %s over `%s a look`.", r.dialect, strings.Join(labels, " or "), word)
	return &result
}

var haveTakeALookVariants = map[string][]string{
	"have": {"take"}, "had": {"took", "taken"}, "has": {"takes"}, "having": {"taking"},
	"take": {"have"}, "took": {"had"}, "taken": {"had"}, "takes": {"has"}, "taking": {"having"},
}
