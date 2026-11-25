package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
	"unicode"
)

type OpenCompounds struct{ expression expr.Expr }

func NewOpenCompounds() *OpenCompounds                    { return &OpenCompounds{expression: openCompoundsExpr()} }
func (r *OpenCompounds) Expr() expr.Expr                  { return r.expression }
func (r *OpenCompounds) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*OpenCompounds) Description() string {
	return "Corrects compound words that should be written as two words."
}
func (r *OpenCompounds) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	index, ok := 0, false
	for _, phrase := range openCompoundsPhrases {
		compound := strings.ReplaceAll(phrase, " ", "")
		if idx, yes := openCompoundsGetCompoundIdx(toks, src, compound); yes {
			index = idx
			ok = true
			break
		}
	}
	if !ok {
		return nil
	}
	span := toks[index].Span
	compound := span.GetContentString(src)
	for i, t := range toks {
		if i != index && t.Kind.IsHyphen() {
			return nil
		}
	}
	phrase, ok := openCompoundsPhrases[strings.ToLower(compound)]
	if !ok {
		return nil
	}
	at := strings.IndexByte(phrase, ' ')
	runes := []rune(compound)
	if at >= len(runes) || unicode.IsUpper(runes[at]) {
		return nil
	}
	return &lint.Lint{Span: span, LintKind: lint.BoundaryError, Suggestions: []lint.Suggestion{lint.ReplaceWithMatchCaseStr(phrase, span.GetContent(src))}, Message: "`" + phrase + "` should be written as two words.", Priority: 31}
}

var openCompoundsPhrases = map[string]string{"afew": "a few", "alot": "a lot", "aswell": "as well", "atall": "at all", "atleast": "at least", "eachother": "each other", "incase": "in case", "infact": "in fact", "infront": "in front", "upto": "up to"}

func openCompoundsExpr() expr.Expr {
	words := expr.NewWordSet()
	for compound := range openCompoundsPhrases {
		words.Add(compound)
	}
	compound := expr.NewSequenceExprWith(words)
	return expr.NewLongestMatchOf([]expr.Expr{expr.NewSequenceExprAnything().Then(compound).ThenAnything(), expr.NewSequenceExprAnything().Then(compound), expr.NewSequenceExprWith(compound).ThenAnything(), compound})
}
func openCompoundsGetCompoundIdx(toks []doc.Token, src []rune, compound string) (int, bool) {
	switch len(toks) {
	case 1:
		return 0, true
	case 3:
		return 1, true
	case 2:
		a, b := toks[0], toks[1]
		n := len(compound)
		if a.Span.Len() == n && b.Span.Len() != n {
			return 0, true
		}
		if b.Span.Len() == n && a.Span.Len() != n {
			return 1, true
		}
		if a.Kind.IsWord() && !b.Kind.IsWord() {
			return 0, true
		}
		if !a.Kind.IsWord() && b.Kind.IsWord() {
			return 1, true
		}
		if strings.EqualFold(a.GetStr(src), compound) {
			return 0, true
		}
		return 1, true
	}
	return 0, false
}
