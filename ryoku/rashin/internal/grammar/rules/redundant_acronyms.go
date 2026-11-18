package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"strings"
)

type RedundantAcronyms struct{ expression expr.Expr }

func NewRedundantAcronyms() *RedundantAcronyms {
	var es []expr.Expr
	for _, a := range redundantAcronymsMappings {
		lastPlural := a.last + "s"
		es = append(es, expr.NewSequenceExprAco(a.acronym).TWs().ThenAnyOf([]expr.Expr{expr.AsExpr(expr.NewWord(a.last)), expr.AsExpr(func(t doc.Token, s []rune) bool { return text.EqStr(t.GetCh(s), lastPlural) })}))
	}
	return &RedundantAcronyms{expr.NewFirstMatchOf(es)}
}
func (r *RedundantAcronyms) Expr() expr.Expr                  { return r.expression }
func (r *RedundantAcronyms) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (*RedundantAcronyms) Description() string {
	return "Identifies redundant acronyms where the last word repeats the last letter's meaning (e.g., `ATM machine` → `ATM` or `automated teller machine`)."
}
func (r *RedundantAcronyms) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	if len(t) == 0 {
		return nil
	}
	last := t[len(t)-1].GetCh(s)
	acronym := t[0].GetStr(s)
	var mapping *redundantAcronymsMapping
	for i := range redundantAcronymsMappings {
		a := &redundantAcronymsMappings[i]
		if strings.EqualFold(a.acronym, acronym) {
			mapping = a
			break
		}
	}
	if mapping == nil {
		return nil
	}
	asciiUpper := func(s string) bool {
		for _, c := range s {
			if c < 'A' || c > 'Z' {
				return false
			}
		}
		return true
	}
	if mapping.flag == 1 && !asciiUpper(acronym) {
		return nil
	}
	allCaps := asciiUpper(string(last))
	ending := ""
	if mapping.flag != 2 && len(last) > 0 && (last[len(last)-1] == 's' || last[len(last)-1] == 'S') {
		ending = string(last[len(last)-1])
	}
	suggestions := []lint.Suggestion{lint.ReplaceWith([]rune(acronym + ending))}
	for _, middle := range mapping.middle {
		last := mapping.last
		if allCaps {
			middle = strings.ToUpper(middle)
			last = strings.ToUpper(last)
		}
		suggestions = append(suggestions, lint.ReplaceWith([]rune(middle+" "+last+ending)))
	}
	span, _ := doc.Span(t)
	return &lint.Lint{Span: span, LintKind: lint.Redundancy, Suggestions: suggestions, Message: "The acronym's last letter already stands for the last word. Use just the acronym or the full phrase.", Priority: 127}
}

type redundantAcronymsMapping struct {
	acronym string
	middle  []string
	last    string
	flag    uint8
}

var redundantAcronymsMappings = []redundantAcronymsMapping{{"ATM", []string{"automated teller", "automatic teller"}, "machine", 0}, {"BWT", []string{"Burrows-Wheeler"}, "transform", 0}, {"FFT", []string{"Fast Fourier"}, "transform", 0}, {"GOP", []string{"Grand Old"}, "Party", 0}, {"GUI", []string{"graphical user"}, "interface", 0}, {"LCD", []string{"liquid crystal"}, "display", 0}, {"LLM", []string{"large language"}, "model", 0}, {"PCB", []string{"printed circuit"}, "board", 0}, {"PIN", []string{"personal identification"}, "number", 1}, {"TUI", []string{"text-based user", "terminal user"}, "interface", 0}, {"UI", []string{"user"}, "interface", 0}, {"USD", []string{"US"}, "dollars", 2}, {"VIN", []string{"vehicle identification"}, "number", 0}}
