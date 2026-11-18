package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type RogueRouge struct{ expression expr.Expr }

func NewRogueRouge() *RogueRouge {
	e := expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExprWordSet(rogueRougeRareBeforeRogue).TWs().ThenWordSeq([]string{"rogue"}), expr.NewSequenceExprWordSet(rogueRougeRareBeforeRouge).TWs().TAco("rouge"), expr.NewSequenceExprWordSeq([]string{"rogue"}).TWs().TSet(rogueRougeRareAfterRogue), expr.NewSequenceExprAco("rouge").TWs().TSet(rogueRougeRareAfterRouge), expr.NewSequenceExprAco("rouge").ThenHyphen().TSet([]string{"like", "likes"})})
	return &RogueRouge{expression: e}
}
func (r *RogueRouge) Expr() expr.Expr                  { return r.expression }
func (r *RogueRouge) Description() string              { return "Detects mixing up `rogue` and `rouge`." }
func (r *RogueRouge) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *RogueRouge) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	tok := lint.FindTheOnlyTokenMatching(t, s, func(t *doc.Token, s []rune) bool {
		return text.EqAnyIgnoreASCIICaseStr(t.GetCh(s), []string{"rogue", "rouge"})
	})
	if tok == nil {
		return nil
	}
	word := append([]rune(nil), tok.GetCh(s)...)
	word[2], word[3] = word[3], word[2]
	l := lint.NewLint()
	l.Span = tok.Span
	l.LintKind = lint.WordChoice
	l.Suggestions = []lint.Suggestion{lint.ReplaceWith(word)}
	l.Message = "Are you confusing `rouge` (color/makeup) with `rogue` (untrustworthy person)?"
	return &l
}

var rogueRougeRareBeforeRogue = []string{"baton", "cheek", "khmer", "lip", "moulin"}
var rogueRougeRareBeforeRouge = []string{"go", "goes", "going", "gone", "went"}
var rogueRougeRareAfterRogue = []string{"lipstick"}
var rogueRougeRareAfterRouge = []string{"like", "likes", "nation", "nations", "one", "squadron", "trader", "traders", "wave", "waves", "who"}
