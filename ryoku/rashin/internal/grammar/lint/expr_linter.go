package lint

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"strings"
)

type DocumentIterator interface {
	IterUnits(*doc.Document) [][]doc.Token
}
type Chunk struct{}

func (Chunk) IterUnits(d *doc.Document) [][]doc.Token { return d.IterChunks() }

type Sentence struct{}

func (Sentence) IterUnits(d *doc.Document) [][]doc.Token { return d.IterSentences() }

type MatchContext struct{ Before, After []doc.Token }
type ExprLinter interface {
	Expr() expr.Expr
	MatchToLint([]doc.Token, []rune) *Lint
	Description() string
}
type ContextExprLinter interface {
	MatchToLintWithContext([]doc.Token, []rune, *MatchContext) *Lint
}
type UnitExprLinter interface{ Unit() DocumentIterator }

func RunOnChunk(l ExprLinter, unit []doc.Token, source []rune) []Lint {
	return appendChunkLints(nil, l, unit, source)
}

func appendChunkLints(out []Lint, l ExprLinter, unit []doc.Token, source []rune) []Lint {
	for _, span := range expr.IterMatches(l.Expr(), unit, source) {
		matched := unit[span.Start:span.End]
		var result *Lint
		if c, ok := l.(ContextExprLinter); ok {
			result = c.MatchToLintWithContext(matched, source, &MatchContext{Before: unit[:span.Start], After: unit[span.End:]})
		} else {
			result = l.MatchToLint(matched, source)
		}
		if result != nil {
			out = append(out, *result)
		}
	}
	return out
}
func RunExprLinter(l ExprLinter, d *doc.Document) []Lint {
	var unit DocumentIterator = Chunk{}
	if u, ok := l.(UnitExprLinter); ok {
		unit = u.Unit()
	}
	// Partition anew for each linter: document token accessors expose mutable
	// slices. The local buffer only avoids the usual partition-slice allocation.
	var buffer [128][]doc.Token
	var units [][]doc.Token
	switch unit.(type) {
	case Chunk:
		units = doc.AppendChunks(buffer[:0], d.GetTokens())
	case Sentence:
		units = doc.AppendSentences(buffer[:0], d.GetTokens())
	default:
		units = unit.IterUnits(d)
	}
	var out []Lint
	for _, tokens := range units {
		out = appendChunkLints(out, l, tokens, d.GetSource())
	}
	return out
}

type exprAdapter struct{ ExprLinter }

func (l exprAdapter) Lint(d *doc.Document) []Lint { return RunExprLinter(l.ExprLinter, d) }
func AsLinter(l ExprLinter) Linter                { return exprAdapter{l} }
func FindTheOnlyTokenIndexMatching(tokens []doc.Token, source []rune, predicate func(*doc.Token, []rune) bool) (int, bool) {
	found := -1
	for i := range tokens {
		if predicate(&tokens[i], source) {
			if found != -1 {
				return 0, false
			}
			found = i
		}
	}
	return found, found != -1
}
func FindTheOnlyTokenMatching(tokens []doc.Token, source []rune, predicate func(*doc.Token, []rune) bool) *doc.Token {
	i, ok := FindTheOnlyTokenIndexMatching(tokens, source, predicate)
	if !ok {
		return nil
	}
	return &tokens[i]
}
func FollowedByWord(c *MatchContext, p func(*doc.Token) bool) bool {
	return c != nil && len(c.After) >= 2 && c.After[0].Kind.IsWhitespace() && p(&c.After[1])
}
func FollowedByToken(c *MatchContext, p func(*doc.Token) bool) bool {
	return c != nil && len(c.After) > 0 && p(&c.After[0])
}
func FollowedByHyphen(c *MatchContext) bool {
	return FollowedByToken(c, func(t *doc.Token) bool { return t.Kind.IsHyphen() })
}
func AtStartOfSentence(c *MatchContext) bool {
	return c != nil && (len(c.Before) == 0 || len(c.Before) == 1 && c.Before[0].Kind.IsWhitespace())
}
func PrecededByWord(c *MatchContext, p func(*doc.Token) bool) bool {
	return c != nil && len(c.Before) >= 2 && c.Before[len(c.Before)-1].Kind.IsWhitespace() && p(&c.Before[len(c.Before)-2])
}
func SurroundedByWords(c *MatchContext, p func(*doc.Token, *doc.Token) bool) bool {
	return c != nil && len(c.Before) >= 2 && len(c.After) >= 2 && c.Before[len(c.Before)-1].Kind.IsWhitespace() && c.After[0].Kind.IsWhitespace() && p(&c.Before[len(c.Before)-2], &c.After[1])
}

type InitialismLinter struct {
	expression expr.Expr
	expansions []string
}

func NewInitialismLinter(initialism string, expansions []string) *InitialismLinter {
	out := &InitialismLinter{expression: expr.NewWordFromCharString([]rune(initialism))}
	for _, s := range expansions {
		out.expansions = append(out.expansions, asciiLower(s))
	}
	return out
}
func asciiLower(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + 32
		}
		return r
	}, s)
}
func (l *InitialismLinter) Expr() expr.Expr { return l.expression }
func (l *InitialismLinter) MatchToLint(tokens []doc.Token, source []rune) *Lint {
	if len(tokens) == 0 {
		return nil
	}
	tok := tokens[0]
	in := source[tok.Span.Start:tok.Span.End]
	out := &Lint{Span: tok.Span, LintKind: Miscellaneous, Message: "Try expanding this initialism.", Priority: 127}
	for _, s := range l.expansions {
		r := []rune(s)
		if len(r) > 0 && len(in) > 0 && in[0] >= 'A' && in[0] <= 'Z' && r[0] >= 'a' && r[0] <= 'z' {
			r[0] -= 32
		}
		out.Suggestions = append(out.Suggestions, ReplaceWith(r))
	}
	return out
}
func (l *InitialismLinter) Description() string         { return "Expands an initialism." }
func (l *InitialismLinter) Lint(d *doc.Document) []Lint { return RunExprLinter(l, d) }
