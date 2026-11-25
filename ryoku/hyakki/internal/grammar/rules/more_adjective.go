package rules

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type MoreAdjective struct {
	expression expr.Expr
	dictionary dict.Dictionary
}

func NewMoreAdjective(dictionary dict.Dictionary) *MoreAdjective {
	return &MoreAdjective{dictionary: dictionary, expression: expr.NewSequenceExprWordSet([]string{"more", "most"}).TWs().ThenPositiveAdjective().ThenOptional(expr.NewFirstMatchOf([]expr.Expr{expr.NewSequenceExprWhitespace().TAco("than").TWs().ThenPositiveAdjective(), expr.AsExpr(func(t doc.Token, _ []rune) bool { return t.Kind.IsHyphen() })}))}
}
func (r *MoreAdjective) Expr() expr.Expr                  { return r.expression }
func (r *MoreAdjective) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *MoreAdjective) Description() string {
	return "Looks for comparative adjective constructions with `more` that could use inflected forms."
}
func (r *MoreAdjective) addValidCandidate(candidates *[]string, candidate string) bool {
	if md := r.dictionary.GetWordMetadataStr(candidate); md != nil && (md.IsComparativeAdjective() || md.IsSuperlativeAdjective()) {
		*candidates = append(*candidates, candidate)
		return true
	}
	return false
}
func (r *MoreAdjective) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	if len(toks) != 3 || !toks[1].Kind.IsWhitespace() || !toks[2].Kind.IsPositiveAdjective() {
		return nil
	}
	phrase, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	degree := toks[0].GetCh(src)
	ending := ""
	if text.EqStr(degree, "more") {
		ending = "er"
	} else if text.EqStr(degree, "most") {
		ending = "est"
	} else {
		return nil
	}
	ch := toks[2].GetCh(src)
	adj := string(ch)
	if len(ch) < 2 || adj == "human" {
		return nil
	}
	var candidates []string
	switch adj {
	case "bad":
		if ending == "er" {
			candidates = append(candidates, "worse")
		} else {
			candidates = append(candidates, "worst")
		}
	case "good":
		if ending == "er" {
			candidates = append(candidates, "better")
		} else {
			candidates = append(candidates, "best")
		}
	case "far":
		if ending == "er" {
			candidates = append(candidates, "further", "farther")
		} else {
			candidates = append(candidates, "furthest", "farthest")
		}
	}
	r.addValidCandidate(&candidates, adj+ending)
	penult, last := ch[len(ch)-2], ch[len(ch)-1]
	if text.IsVowel(penult) && !text.IsVowel(last) {
		r.addValidCandidate(&candidates, adj+string(last)+ending)
	}
	if last == 'y' {
		r.addValidCandidate(&candidates, string(ch[:len(ch)-1])+"i"+ending)
	} else if last == 'e' {
		r.addValidCandidate(&candidates, string(ch[:len(ch)-1])+ending)
	}
	if len(candidates) == 0 {
		return nil
	}
	l := lint.NewLint()
	l.Span = phrase
	l.LintKind = lint.Style
	l.Suggestions = lint.ToReplaceSuggestions(candidates, phrase.GetContent(src))
	l.Message = "This is not an error, but an inflected form of this adjective also exists"
	return &l
}
