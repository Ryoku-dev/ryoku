package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type InOnesDefense struct{ expression expr.Expr }

func NewInOnesDefense() *InOnesDefense {
	return &InOnesDefense{expression: expr.NewSequenceExprAco("to").TWs().ThenWordSet([]string{"my", "your", "his", "her", "its", "our", "their", "one's"}).TWs().ThenWordSet([]string{"defense", "defence"})}
}
func (r *InOnesDefense) Expr() expr.Expr                  { return r.expression }
func (r *InOnesDefense) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *InOnesDefense) Description() string {
	return "Corrects `to my defense` to `in my defense` when it introduces a justification, but not after verbs of motion such as `came to my defense`."
}
func (r *InOnesDefense) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}

// Verbs of motion after which "to my defense" is correct ("rushed to my defense").
var inOnesDefenseMotionVerbs = []string{"come", "comes", "came", "coming", "jump", "jumps", "jumped", "jumping", "leap", "leaps", "leapt", "leaped", "leaping", "rise", "rises", "rose", "risen", "rising", "rush", "rushes", "rushed", "rushing", "spring", "springs", "sprang", "sprung", "springing"}

func (r *InOnesDefense) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if lint.PrecededByWord(ctx, func(t *doc.Token) bool { return text.EqAnyIgnoreASCIICaseStr(t.GetCh(src), inOnesDefenseMotionVerbs) }) {
		return nil
	}
	if len(toks) == 0 {
		return nil
	}
	span := toks[0].Span
	l := lint.NewLint()
	l.Span = span
	l.LintKind = lint.Usage
	l.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("in", span.GetContent(src))}
	l.Message = "When giving a justification, the idiom is `in my defense`, not `to my defense`."
	return &l
}
