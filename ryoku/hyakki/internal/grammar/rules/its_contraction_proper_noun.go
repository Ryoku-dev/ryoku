package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/pos"
	"unicode"
)

type ProperNoun struct {
	expression expr.Expr
	mapping    *expr.ExprMap[int]
}

func NewProperNoun() *ProperNoun {
	opinion := expr.Or(expr.Or(expr.Or(expr.Or(expr.NewDerivedFromStr("think"), expr.NewDerivedFromStr("hope")), expr.NewDerivedFromStr("assume")), expr.NewDerivedFromStr("doubt")), expr.NewDerivedFromStr("guess"))
	capitalized := func(tok doc.Token, src []rune) bool {
		chars := tok.GetCh(src)
		return tok.Kind.IsWord() && len(chars) > 0 && unicode.IsUpper(chars[0])
	}
	head := expr.Or(expr.NewUPOSSet([]pos.UPOS{pos.PROPN}), capitalized)
	mapping := expr.NewExprMap[int]()
	mapping.Insert(expr.NewSequenceExprWith(opinion).TWs().TAco("its").TWs().Then(head).ThenOptional(expr.NewSequenceExpr().TWs().ThenAnyWord()), 2)
	return &ProperNoun{expression: mapping, mapping: mapping}
}
func (r *ProperNoun) Expr() expr.Expr { return r.expression }
func (r *ProperNoun) Description() string {
	return "Suggests the contraction `it's` after opinion verbs when it introduces a proper noun."
}
func (r *ProperNoun) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ProperNoun) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
	if len(tokens) >= 7 {
		chars := tokens[6].GetCh(source)
		kind := tokens[6].Kind
		if len(chars) > 0 && unicode.IsLower(chars[0]) && (kind.IsUpos(pos.NOUN) || kind.IsUpos(pos.ADJ)) {
			return nil
		}
	}
	index, ok := r.mapping.Lookup(0, tokens, source)
	if !ok || index >= len(tokens) {
		return nil
	}
	target := tokens[index]
	return &lint.Lint{Span: target.Span, LintKind: lint.Punctuation, Priority: 31, Message: "Use `it's` (short for \"it is\") before a proper noun in this construction.", Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr("it's", target.GetCh(source))}}
}
