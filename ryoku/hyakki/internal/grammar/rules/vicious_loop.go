package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
	"strings"
)

type viciousLoopPrefer uint8

const (
	viciousLoopCircle viciousLoopPrefer = iota
	viciousLoopCycle
	viciousLoopDontCare
)

func viciousLoopBuildExpr(pref viciousLoopPrefer) expr.Expr {
	seq := expr.NewSequenceExprWordSet([]string{"vicious", "virtuous", "viscous"}).TWs().ThenWordSet([]string{"circle", "circles", "cycle", "cycles"})
	exception := expr.NewSequenceExpr().ThenWordExcept([]string{"viscous"})
	switch pref {
	case viciousLoopCircle:
		exception.TWs().ThenWordSet([]string{"circle", "circles"})
	case viciousLoopCycle:
		exception.TWs().ThenWordSet([]string{"cycle", "cycles"})
	}
	return expr.ButNot(seq, exception)
}
func viciousLoopToLint(toks []doc.Token, src []rune, pref viciousLoopPrefer) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	adj, noun := toks[0], toks[len(toks)-1]
	badAdj := text.EqCh(adj.GetCh(src), []rune("viscous"))
	badNoun := false
	nounLower := strings.ToLower(noun.GetStr(src))
	switch pref {
	case viciousLoopCircle:
		badNoun = strings.HasPrefix(nounLower, "cycle")
	case viciousLoopCycle:
		badNoun = strings.HasPrefix(nounLower, "circle")
	}
	plural := strings.HasSuffix(nounLower, "s")
	preferred, other := "circle", "cycle"
	if pref == viciousLoopCycle {
		preferred, other = other, preferred
	}
	replacement := ""
	message := ""
	if badNoun && !badAdj {
		span = noun.Span
		replacement = preferred
		if plural {
			replacement += "s"
		}
		if pref == viciousLoopCircle {
			message = "This idiom originally used `circle`, not `cycle`"
		} else {
			message = "Though this idiom originally used `circle`, `cycle` is preferred."
		}
	} else if badNoun && badAdj {
		if pref == viciousLoopDontCare {
			return nil
		}
		replacement = "vicious " + preferred
		if plural {
			replacement += "s"
		}
		message = "The idiom uses the word `vicious`, not `viscous`, which describes thick liquids. And we prefer `" + preferred + "` over `" + other + "`."
	} else if badAdj {
		span = adj.Span
		replacement = "vicious"
		message = "The idiom uses the word `vicious`, not `viscous`, which describes thick liquids."
	} else {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage
	result.Message = message
	result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, src[span.Start:span.End])}
	return &result
}

type ViciousCircle struct{ expression expr.Expr }

func NewViciousCircle() *ViciousCircle {
	return &ViciousCircle{viciousLoopBuildExpr(viciousLoopCircle)}
}
func (r *ViciousCircle) Expr() expr.Expr { return r.expression }
func (r *ViciousCircle) Description() string {
	return "Corrects and standardizes common errors and variants of `vicious/virtuous circle`."
}
func (r *ViciousCircle) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return viciousLoopToLint(t, s, viciousLoopCircle)
}
func (r *ViciousCircle) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }

type ViciousCycle struct{ expression expr.Expr }

func NewViciousCycle() *ViciousCycle    { return &ViciousCycle{viciousLoopBuildExpr(viciousLoopCycle)} }
func (r *ViciousCycle) Expr() expr.Expr { return r.expression }
func (r *ViciousCycle) Description() string {
	return "Corrects and standardizes common errors and variants of `vicious/virtuous cycle`."
}
func (r *ViciousCycle) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return viciousLoopToLint(t, s, viciousLoopCycle)
}
func (r *ViciousCycle) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }

type ViciousCircleOrCycle struct{ expression expr.Expr }

func NewViciousCircleOrCycle() *ViciousCircleOrCycle {
	return &ViciousCircleOrCycle{viciousLoopBuildExpr(viciousLoopDontCare)}
}
func (r *ViciousCircleOrCycle) Expr() expr.Expr { return r.expression }
func (r *ViciousCircleOrCycle) Description() string {
	return "Corrects common errors in `vicious/virtuous circle/cycle`."
}
func (r *ViciousCircleOrCycle) MatchToLint(t []doc.Token, s []rune) *lint.Lint {
	return viciousLoopToLint(t, s, viciousLoopDontCare)
}
func (r *ViciousCircleOrCycle) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
