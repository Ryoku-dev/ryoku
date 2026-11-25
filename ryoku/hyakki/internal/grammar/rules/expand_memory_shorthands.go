package rules

import (
	"fmt"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"strings"
)

type ExpandMemoryShorthands struct{ expression expr.Expr }

func NewExpandMemoryShorthands() *ExpandMemoryShorthands {
	hotwords := expr.NewWordSet([]string{"B", "kB", "MB", "GB", "TB", "PB", "EB", "ZB", "YB", "RB", "QB", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB", "ZiB", "YiB", "RiB", "QiB"})
	return &ExpandMemoryShorthands{expression: expr.NewSequenceExprWith(expr.NewImpliesQuantity()).ThenLongestOf([]expr.Expr{expr.NewSequenceExprWith(hotwords), expr.NewSequenceExpr().TWsH().Then(hotwords)})}
}
func (r *ExpandMemoryShorthands) Expr() expr.Expr { return r.expression }
func (r *ExpandMemoryShorthands) Description() string {
	return "Expands memory-related abbreviations (`B`, `kB`, `MB`, `GB`, `TB`, `PB`, `KiB`, `MiB`, `GiB`, `TiB`, `PiB`, etc.) to their full forms (`byte`, `kilobyte`, `megabyte`, `gigabyte`, `terabyte`, `petabyte`, `kibibyte`, `mebibyte`, `gibibyte`, `tebibyte`, `pebibyte`, etc.)."
}
func (r *ExpandMemoryShorthands) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *ExpandMemoryShorthands) MatchToLint(toks []doc.Token, src []rune) *lint.Lint {
	return r.MatchToLintWithContext(toks, src, nil)
}
func (r *ExpandMemoryShorthands) MatchToLintWithContext(toks []doc.Token, src []rune, ctx *lint.MatchContext) *lint.Lint {
	span, ok := doc.Span(toks)
	if !ok {
		return nil
	}
	result := lint.NewLint()
	result.Span = span
	result.LintKind = lint.WordChoice
	result.Priority = 31
	last := toks[len(toks)-1]
	if strings.ToLower(last.GetStr(src)) == "b" && ctx != nil && len(ctx.After) > 0 && (ctx.After[0].Kind.IsPlus() || ctx.After[0].Kind.IsHyphen()) {
		return nil
	}
	word := last.GetStr(src)
	forms, ok := expandMemoryShorthandsReplacements[word]
	if !ok {
		return nil
	}
	plural, known := expr.ImpliesPlurality(toks[0], src)
	if !known {
		plural = strings.HasSuffix(word, "s")
	}
	replacement := forms[0]
	if plural {
		replacement = forms[1]
	}
	chars := replacement
	if len(toks) == 2 {
		chars = " " + chars
	}
	if chars == word {
		return nil
	}
	result.Span = last.Span
	result.Suggestions = []lint.Suggestion{lint.ReplaceWith([]rune(chars))}
	result.Message = fmt.Sprintf("Did you mean `%s`?", replacement)
	return &result
}

var expandMemoryShorthandsReplacements = map[string][2]string{
	"B":   {"byte", "bytes"},
	"kB":  {"kilobyte", "kilobytes"},
	"MB":  {"megabyte", "megabytes"},
	"GB":  {"gigabyte", "gigabytes"},
	"TB":  {"terabyte", "terabytes"},
	"PB":  {"petabyte", "petabytes"},
	"EB":  {"exabyte", "exabytes"},
	"ZB":  {"zettabyte", "zettabytes"},
	"YB":  {"yottabyte", "yottabytes"},
	"RB":  {"ronnabyte", "ronnabytes"},
	"QB":  {"quettabyte", "quettabytes"},
	"KiB": {"kibibyte", "kibibytes"},
	"MiB": {"mebibyte", "mebibytes"},
	"GiB": {"gibibyte", "gibibytes"},
	"TiB": {"tebibyte", "tebibytes"},
	"PiB": {"pebibyte", "pebibytes"},
	"EiB": {"exbibyte", "exbibytes"},
	"ZiB": {"zebibyte", "zebibytes"},
	"YiB": {"yobibyte", "yobibytes"},
	"RiB": {"robibyte", "robibytes"},
	"QiB": {"quebibyte", "quebibytes"},
}
