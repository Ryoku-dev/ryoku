package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"strings"
)

type NounCountability struct{ expression expr.Expr }

func NewNounCountability() *NounCountability {
	return &NounCountability{expression: expr.NewSequenceExprAnyOf([]expr.Expr{expr.AsExpr(expr.NewIndefiniteArticle()), expr.AsExpr(expr.NewWordSet([]string{"another", "both", "each", "every", "few", "fewer", "many", "multiple", "one", "several"}))}).TWs().ThenMassNounOnly()}
}
func (r *NounCountability) Expr() expr.Expr                  { return r.expression }
func (r *NounCountability) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *NounCountability) Description() string {
	return "Correct mass nouns that are preceded by the wrong determiners or quantifiers."
}
func (r *NounCountability) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *NounCountability) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	if len(toks) != 3 || lint.FollowedByHyphen(ctx) || lint.FollowedByWord(ctx, func(t *doc.Token) bool { return t.Kind.IsNoun() || t.Kind.IsOov() || text.EqStr(t.GetCh(src), "and") }) || (massNounsNounCountabilityIsIngWord(toks[2], src) && massNounsNounCountabilityFollowedByNominalHead(ctx, src)) {
		return nil
	}
	span, _ := doc.Span(toks)
	template := span.GetContent(src)
	dq := strings.ToLower(toks[0].GetStr(src))
	noun := strings.ToLower(toks[2].GetStr(src))
	if noun == "software" && lint.FollowedByWord(ctx, func(t *doc.Token) bool { return text.EqStr(t.GetCh(src), "rendered") }) {
		return nil
	}
	singular := false
	plural := false
	switch dq {
	case "a", "an", "another", "each", "every", "one":
		singular = true
	case "both", "many", "multiple", "several":
		plural = true
	}
	var synonyms []string
	switch noun {
	case "advice":
		if singular {
			synonyms = []string{"tip", "suggestion", "recommendation"}
		} else if plural {
			synonyms = []string{"tips", "suggestions", "recommendations"}
		}
	case "clothing":
		if singular {
			synonyms = []string{"garment"}
		} else if plural {
			synonyms = []string{"garments"}
		}
	case "luggage":
		if singular {
			synonyms = []string{"suitcase", "bag"}
		} else if plural {
			synonyms = []string{"suitcases", "bags"}
		}
	case "punctuation":
		if singular {
			synonyms = []string{"punctuation mark"}
		} else if plural {
			synonyms = []string{"punctuation marks"}
		}
	case "software":
		if dq == "a" {
			synonyms = []string{"program", "software package", "software tool"}
		} else if singular {
			synonyms = []string{"app", "application", "program", "software package", "software tool"}
		} else if plural {
			synonyms = []string{"apps", "applications", "programs", "software packages", "software tools"}
		}
	}
	var suggestions []lint.Suggestion
	add := func(value string) { suggestions = append(suggestions, lint.ReplaceWithMatchCaseStr(value, template)) }
	for _, syn := range synonyms {
		add(dq + " " + syn)
	}
	noPiece := noun == "punctuation" || noun == "traffic"
	switch dq {
	case "a", "an":
		add(noun)
		add("some " + noun)
		if !noPiece {
			add("a piece of " + noun)
		}
	case "another", "each", "every", "one":
		if !noPiece {
			add(dq + " piece of " + noun)
		}
	case "both", "multiple", "several":
		if !noPiece {
			add(dq + " pieces of " + noun)
		}
	case "few":
		add("little " + noun)
		if !noPiece {
			add(dq + " pieces of " + noun)
		}
	case "fewer":
		add("less " + noun)
		if !noPiece {
			add(dq + " pieces of " + noun)
		}
	case "many":
		add("much " + noun)
		add("a lot of " + noun)
		if !noPiece {
			add(dq + " pieces of " + noun)
		}
	}
	return &lint.Lint{Span: span, LintKind: lint.Agreement, Suggestions: suggestions, Message: fmt.Sprintf("`%s` is a mass noun.", noun), Priority: 31}
}
func massNounsNounCountabilityIsIngWord(t doc.Token, src []rune) bool {
	return text.EndsWithIgnoreASCIICaseStr(t.GetCh(src), "ing")
}
func massNounsNounCountabilityFollowedByNominalHead(ctx *lint.MatchContext, src []rune) bool {
	if ctx == nil {
		return false
	}
	after := ctx.After
	if massNounsNounCountabilityFollowsModifiersThenNoun(after, src) {
		return true
	}
	cursor := 0
	sawSlash := false
	for cursor+1 < len(after) {
		if !after[cursor].Kind.IsSlash() || !massNounsNounCountabilityIsIngWord(after[cursor+1], src) {
			break
		}
		sawSlash = true
		cursor += 2
	}
	return sawSlash && massNounsNounCountabilityFollowsModifiersThenNoun(after[cursor:], src)
}
func massNounsNounCountabilityFollowsModifiersThenNoun(toks []doc.Token, src []rune) bool {
	for cursor := 0; cursor+1 < len(toks); cursor += 2 {
		if !toks[cursor].Kind.IsWhitespace() {
			return false
		}
		word := toks[cursor+1]
		if massNounsNounCountabilityIsNominalChainBoundary(word, src) {
			return false
		}
		if word.Kind.IsNoun() || word.Kind.IsOov() {
			return true
		}
		if !(word.Kind.IsAdjective() || word.Kind.IsAdverb() || word.Kind.IsVerbProgressiveForm() || word.Kind.IsVerbPastParticipleForm()) {
			return false
		}
	}
	return false
}
func massNounsNounCountabilityIsNominalChainBoundary(t doc.Token, src []rune) bool {
	return t.Kind.IsPreposition() || t.Kind.IsDeterminer() || t.Kind.IsConjunction() || text.EqAnyIgnoreASCIICaseStr(t.GetCh(src), []string{"about", "after", "as", "at", "before", "by", "for", "from", "in", "into", "of", "on", "through", "to", "under", "with"})
}
