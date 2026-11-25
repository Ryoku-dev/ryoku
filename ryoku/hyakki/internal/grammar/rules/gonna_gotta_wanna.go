package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type GonnaGottaWanna struct {
	expression       expr.Expr
	dialect          dict.Dialect
	informal, formal string
}

func NewGonnaGottaWanna(dialect dict.Dialect, informal, formal string) *GonnaGottaWanna {
	return &GonnaGottaWanna{expression: expr.NewSequenceExprAco(informal).ThenOptional(expr.NewSequenceExprWhitespace().TSet([]string{"to", "a"})), dialect: dialect, informal: informal, formal: formal}
}
func (r *GonnaGottaWanna) Expr() expr.Expr { return r.expression }
func (r *GonnaGottaWanna) Description() string {
	return "Corrects the informal contractions `gonna`, `gotta`, and `wanna` to their full forms."
}
func (r *GonnaGottaWanna) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *GonnaGottaWanna) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *GonnaGottaWanna) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	verbChars := toks[0].GetCh(src)
	if len(verbChars) < 5 {
		return nil
	}
	verbChars = verbChars[:5]
	verb := strings.ToLower(string(verbChars))
	if verb != "gonna" && verb != "gotta" && verb != "wanna" {
		return nil
	}
	rest := ""
	if len(toks) > 2 {
		candidate := strings.ToLower(toks[2].GetStr(src))
		if candidate == "to" || candidate == "a" {
			rest = candidate
		}
	}
	full := src[span.Start:span.End]
	suggestions := []lint.Suggestion{}
	add := func(s string) { suggestions = append(suggestions, lint.ReplaceWithMatchCaseStr(s, full)) }
	if rest != "a" {
		add(r.formal + " to")
	}
	switch verb {
	case "gonna":
		if rest == "to" {
			suggestions = append(suggestions, lint.ReplaceWith(verbChars))
		}
	case "gotta":
		switch rest {
		case "":
			add("have to")
		case "to":
			suggestions = append(suggestions, lint.ReplaceWith(verbChars))
			add("have to")
		case "a":
			vowel := lint.FollowedByWord(ctx, func(t *doc.Token) bool {
				sound, ok := dict.StartsWithVowel(t.GetCh(src), r.dialect)
				return ok && sound == dict.Vowel
			})
			if vowel {
				add("got an")
			} else {
				add("got a")
			}
		}
	case "wanna":
		if rest == "to" {
			suggestions = append(suggestions, lint.ReplaceWith(verbChars))
		} else if rest == "a" {
			add("want a")
		}
	}
	message := fmt.Sprintf("`%s` is very informal.", r.informal)
	if rest != "" {
		message = fmt.Sprintf("`%s` is very informal and the final `a` means `to`.", r.informal)
	}
	result := lint.NewLint()
	result.Span = span
	result.Suggestions = suggestions
	result.Message = message
	return &result
}
func NewGonnaGottaWannaLintGroup(dialect dict.Dialect) *lint.LintGroup {
	group := lint.NewLintGroup()
	for _, entry := range [][3]string{{"Gonna", "gonna", "going"}, {"Gotta", "gotta", "got"}, {"Wanna", "wanna", "want"}} {
		group.Add(entry[0], NewGonnaGottaWanna(dialect, entry[1], entry[2]))
	}
	enabled := true
	group.SetAllRulesTo(&enabled)
	return group
}
