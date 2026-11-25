package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
	"ryoku-rashin/internal/grammar/text"
)

type WereWhere struct{ expression expr.Expr }

func NewWereWhere() *WereWhere {
	return &WereWhere{expression: expr.NewSequenceExprAnyOf([]expr.Expr{expr.NewSequenceExprWordSet([]string{"they", "we"}).TWs().TAco("where"), expr.NewSequenceExprWordSeq([]string{"you", "where"}).TWs().Then(expr.NewUPOSSet([]pos.UPOS{pos.VERB, pos.AUX, pos.ADJ})), expr.NewSequenceExprWordSeq([]string{"where", "you"}).TWs().Then(expr.NewUPOSSet([]pos.UPOS{pos.VERB, pos.AUX, pos.ADJ})), expr.NewSequenceExprAco("if").TWs().ThenSubjectPronoun().TWs().ThenWordSeq([]string{"where", "to"}), expr.NewSequenceExprWith(func(t doc.Token, s []rune) bool { return t.Kind.IsUpos(pos.VERB) }).TWs().TAco("were").TWs().Then(expr.NewUPOSSet([]pos.UPOS{pos.PRON, pos.DET, pos.PROPN}))})}
}
func (r *WereWhere) Expr() expr.Expr                  { return r.expression }
func (r *WereWhere) Description() string              { return "Detects mixing up `were` and `where`." }
func (r *WereWhere) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *WereWhere) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return r.MatchToLintWithContext(t, s, nil)
}
func (r *WereWhere) MatchToLintWithContext(t []doc.Token, s []rune, c *lint.MatchContext) *lint.Lint {
	var where, were *doc.Token
	for i := range t {
		if !t[i].Kind.IsWord() {
			continue
		}
		if where == nil && text.EqStr(t[i].GetCh(s), "where") {
			where = &t[i]
		}
		if were == nil && text.EqStr(t[i].GetCh(s), "were") {
			were = &t[i]
		}
	}
	tok := were
	replacement := "where"
	message := "It looks like this is a typo, did you mean `where`?"
	if where != nil {
		if !lint.AtStartOfSentence(c) {
			return nil
		}
		tok = where
		replacement = "were"
		message = "It looks like this is a typo, did you mean `were`?"
	}
	if tok == nil {
		return nil
	}
	result := lint.NewLint()
	result.Span = tok.Span
	result.LintKind = lint.Typo
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, tok.GetCh(s))}
	result.Message = message
	return &result
}

func (*WereWhere) Unit() lint.DocumentIterator { return lint.Sentence{} }
