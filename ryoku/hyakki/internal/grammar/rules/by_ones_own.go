package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type ByOnesOwn struct{ expression expr.Expr }

func NewByOnesOwn() *ByOnesOwn {
	return &ByOnesOwn{expression: expr.NewSequenceExprAco("by").TWs().ThenPossessiveDeterminer().TWs().TAco("own")}
}
func (r *ByOnesOwn) Expr() expr.Expr                  { return r.expression }
func (r *ByOnesOwn) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ByOnesOwn) Description() string {
	return "Fixes incorrect phrases like `by my own` by suggesting `on my own` or `by myself`."
}
func (r *ByOnesOwn) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if lint.FollowedByWord(ctx, func(*doc.Token) bool { return true }) {
		return nil
	}
	span, ok := doc.Span(toks)
	if !ok || len(toks) < 3 {
		return nil
	}
	var reflexives []string
	switch strings.ToLower(toks[2].GetStr(src)) {
	case "my":
		reflexives = []string{"myself"}
	case "our":
		reflexives = []string{"ourselves"}
	case "your":
		reflexives = []string{"yourself", "yourselves"}
	case "his":
		reflexives = []string{"himself"}
	case "her":
		reflexives = []string{"herself"}
	case "its":
		reflexives = []string{"itself"}
	case "their":
		reflexives = []string{"themselves", "themself"}
	case "one's":
		reflexives = []string{"oneself"}
	default:
		return nil
	}
	byws, _ := doc.GetStr(toks[:2], src)
	whoseOwn, _ := doc.GetStr(toks[2:], src)
	suggestions := []lint.Suggestion{lint.ReplaceWithMatchCaseStr("on"+toks[1].GetStr(src)+whoseOwn, span.GetContent(src))}
	for _, reflexive := range reflexives {
		suggestions = append(suggestions, lint.ReplaceWithMatchCaseStr(byws+reflexive, span.GetContent(src)))
	}
	return &lint.Lint{Span: span, LintKind: lint.Usage, Suggestions: suggestions, Message: "`By one's own` is not idiomatic. Consider either `on one's own` or `by oneself`.", Priority: 127}
}
func (r *ByOnesOwn) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
