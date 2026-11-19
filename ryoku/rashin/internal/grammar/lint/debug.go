package lint

import (
	"ryoku-rashin/internal/grammar/doc"
	"strings"
)

func FormatLintMatch(matched []doc.Token, context *MatchContext, source []rune) string {
	format := func(tokens []doc.Token) string {
		var out strings.Builder
		for _, tok := range tokens {
			if !tok.Kind.IsUnlintable() {
				out.WriteString(tok.GetStr(source))
			}
		}
		return out.String()
	}
	if context == nil {
		return format(matched)
	}
	return "\x1b[2m" + format(context.Before) + "\x1b[0m" + format(matched) + "\x1b[2m" + format(context.After) + "\x1b[0m"
}
