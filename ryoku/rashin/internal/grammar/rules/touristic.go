package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"slices"
	"strings"
)

type Touristic struct{ expression expr.Expr }

func NewTouristic() *Touristic {
	return &Touristic{expression: expr.NewLongestMatchOf([]expr.Expr{expr.NewSequenceExprAnyWord().TWs().TAco("touristic").TWs().ThenAnyWord(), expr.NewSequenceExprAnyWord().TWs().TAco("touristic"), expr.NewSequenceExpr().TAco("touristic").TWs().ThenAnyWord(), expr.NewSequenceExpr().TAco("touristic")})}
}
func (r *Touristic) Expr() expr.Expr                  { return r.expression }
func (r *Touristic) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *Touristic) Description() string {
	return "Suggests replacing the uncommon word `touristic` with `tourist`, `tourism`, and/or `touristy`."
}
func (r *Touristic) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	content := strings.ToLower(span.GetContentString(src))
	touristyPref, nounFormsPref := Neutral, Neutral
	idx := -1
	switch {
	case len(toks) == 1:
		nounFormsPref = Allow
		touristyPref = Allow
		idx = 0
	case len(toks) == 3 && strings.HasPrefix(content, "touristic ") && !strings.HasSuffix(content, " touristic"):
		nextWord := toks[2].GetStr(src)
		if toks[2].Kind.IsNoun() {
			if slices.Contains(touristicWhitelist, nextWord) {
				touristyPref = Allow
			}
			if slices.Contains(touristicBlacklist, nextWord) {
				touristyPref = Deny
			}
		}
		idx = 0
	case len(toks) == 3 && !strings.HasPrefix(content, "touristic ") && strings.HasSuffix(content, " touristic"):
		if toks[0].Kind.IsAdjective() || toks[0].Kind.IsLinkingVerb() {
			nounFormsPref = Deny
		} else {
			nounFormsPref = Allow
		}
		idx = 2
	case len(toks) == 5:
		if toks[0].Kind.IsAdverb() {
			nounFormsPref = Deny
		}
		nextWord := strings.ToLower(toks[4].GetStr(src))
		if toks[4].Kind.IsNoun() {
			if slices.Contains(touristicWhitelist, nextWord) {
				touristyPref = Allow
			}
			if slices.Contains(touristicBlacklist, nextWord) {
				touristyPref = Deny
			}
		}
		if toks[4].Kind.IsAdjective() && !toks[4].Kind.IsNoun() {
			nounFormsPref = Deny
			touristyPref = Allow
		}
		idx = 2
	default:
		return nil
	}
	var suggestions []lint.Suggestion
	template := toks[idx].GetCh(src)
	if nounFormsPref != Deny {
		suggestions = append(suggestions, lint.ReplaceWithMatchCaseStr("tourist", template), lint.ReplaceWithMatchCaseStr("tourism", template))
	}
	if touristyPref != Deny {
		suggestions = append(suggestions, lint.ReplaceWithMatchCaseStr("touristy", template))
	}
	result := lint.NewLint()
	result.Span = toks[idx].Span
	result.LintKind = lint.WordChoice
	result.Suggestions = suggestions
	result.Message = "The word `touristic` is rarely used by native speakers."
	result.Priority = 31
	return &result
}

type SuggestionPreference uint8

const (
	// Neutral is not explicitly allowed or denied.
	Neutral SuggestionPreference = iota
	// Allow explicitly allows this suggestion.
	Allow
	// Deny explicitly denies this suggestion.
	Deny
)

var touristicBlacklist = []string{"app", "apps", "data", "content", "establishment", "establishments", "info", "information", "interest", "platform", "platforms", "service", "services"}
var touristicWhitelist = []string{"activity", "activities", "area", "areas", "destination", "destinations", "location", "locations", "place", "places", "route", "routes", "spot", "spots"}
