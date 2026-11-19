package rules

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type FleshOutVsFullFledged struct{ expression expr.Expr }

func NewFleshOutVsFullFledged() *FleshOutVsFullFledged {
	return &FleshOutVsFullFledged{expression: expr.NewSequenceExprOptional(expr.NewSequenceExprWordSet([]string{"full", "fully"}).TWsH()).ThenWordSet([]string{"fledge", "fledged", "fledged", "fledges", "fledging", "flesh", "fleshed", "fleshed", "fleshes", "fleshing", "pledge", "pledged", "pledged", "pledges", "pledging"}).ThenOptional(expr.NewSequenceExpr().TWsH().TAco("out"))}
}
func (r *FleshOutVsFullFledged) Expr() expr.Expr { return r.expression }
func (r *FleshOutVsFullFledged) Description() string {
	return "Corrects mixing up `flesh out` and `full fledged`."
}
func (r *FleshOutVsFullFledged) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *FleshOutVsFullFledged) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *FleshOutVsFullFledged) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.Usage

	first := strings.ToLower(toks[0].GetStr(src))
	full := first == "full" || first == "fully"
	hasOut := strings.ToLower(toks[len(toks)-1].GetStr(src)) == "out"
	if hasOut && ctx != nil && len(ctx.After) > 0 && ctx.After[0].Kind.IsHyphen() {
		hasOut = false
		toks = toks[:len(toks)-2]
	}
	index := 0
	if full {
		index = 2
	}
	tok := toks[index]
	chars := tok.GetCh(src)
	if len(chars) == 0 {
		return nil
	}
	form := "lemma"
	switch chars[len(chars)-1] {
	case 'd':
		form = "past"
	case 's':
		form = "third"
	case 'g':
		form = "ing"
	}
	word := strings.ToLower(string(chars))
	verb := "pledge"
	if strings.HasPrefix(word, "fledg") {
		verb = "fledge"
	} else if strings.HasPrefix(word, "flesh") {
		verb = "flesh"
	}
	flags := 0
	for i := 1; i < len(toks); i += 2 {
		if toks[i].Kind.IsHyphen() {
			flags |= 1
		} else if toks[i].Kind.IsWhitespace() {
			flags |= 2
		} else {
			flags |= 4
		}
	}
	if flags != 1 && flags != 2 {
		return nil
	}
	switch {
	case full && !hasOut && ((verb == "pledge" && (form == "lemma" || form == "past")) || (verb == "fledge" && form == "lemma")):
		prefix, _ := doc.GetStr(toks[:2], src)
		template, _ := doc.GetCh(toks[:2], src)
		result.Span, _ = doc.Span(toks)
		result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(prefix+"fledged", template)}
		result.Message = "This idiom uses the word `fledged`."
	case !full && hasOut && (verb == "fledge" || verb == "pledge"):
		replacement := "flesh"
		switch form {
		case "past":
			replacement = "fleshed"
		case "ing":
			replacement = "fleshing"
		case "third":
			replacement = "fleshes"
		}
		result.Span = tok.Span
		result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr(replacement, chars)}
		result.Message = "This idiom uses the word `flesh`."
	case full && form == "past" && (((verb == "fledge" || verb == "pledge") && hasOut) || (verb == "flesh" && !hasOut)):
		sep := " "
		if flags == 1 {
			sep = "-"
		}
		result.Span, _ = doc.Span(toks[index:])
		result.Suggestions = []lint.Suggestion{lint.ReplaceWithMatchCaseStr("fledged", chars), lint.ReplaceWithMatchCaseStr("fleshed"+sep+"out", chars)}
		result.Message = "Perhaps you're confusing `fully fledged` and `fleshed out`?"
	default:
		return nil
	}
	return &result
}
