package weir

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/text"
)

type ReplacementStrategy uint8

const (
	MatchCase ReplacementStrategy = iota
	Exact
)

type WeirScope uint8

const (
	Chunk WeirScope = iota
	Sentence
)

type TestResult struct{ Expected, Got string }
type WeirLinter struct {
	expression           expr.Expr
	description, message string
	strategy             ReplacementStrategy
	replacements         []string
	kind                 lint.LintKind
	scope                WeirScope
	ast                  *Ast
}

func NewWeirLinter(code string) (*WeirLinter, error) {
	a, err := Parse(code)
	if err != nil {
		return nil, err
	}
	return Compile(a)
}
func Compile(a *Ast) (*WeirLinter, error) {
	ctx := make(map[string]expr.Expr)
	for _, s := range a.Stmts {
		if s.Kind == SetExpr {
			e, err := s.Expr.ToExpr(ctx)
			if err != nil {
				return nil, err
			}
			ctx[s.Name] = e
		}
	}
	e, ok := ctx["main"]
	if !ok {
		return nil, Error{Kind: ExpectedVariableUndefined}
	}
	str := func(name string, required bool) (string, error) {
		v, ok := a.GetVariableValue(name)
		if !ok {
			if required {
				return "", Error{Kind: ExpectedVariableUndefined}
			}
			return "", nil
		}
		if v.IsArray {
			return "", Error{Kind: ExpectedDifferentVariableType}
		}
		return v.String, nil
	}
	d, err := str("description", true)
	if err != nil {
		return nil, err
	}
	m, err := str("message", true)
	if err != nil {
		return nil, err
	}
	v, ok := a.GetVariableValue("becomes")
	if !ok {
		return nil, Error{Kind: ExpectedVariableUndefined}
	}
	var replacements []string
	if v.IsArray {
		for _, r := range v.Array {
			if r.IsArray {
				return nil, Error{Kind: ExpectedDifferentVariableType}
			}
			replacements = append(replacements, r.String)
		}
	} else {
		replacements = []string{v.String}
	}
	r := &WeirLinter{expression: e, description: d, message: m, replacements: replacements, kind: lint.Miscellaneous, ast: a}
	s, err := str("strategy", false)
	if err != nil {
		return nil, err
	}
	if _, present := a.GetVariableValue("strategy"); present {
		switch s {
		case "MatchCase":
		case "Exact":
			r.strategy = Exact
		default:
			return nil, Error{Kind: InvalidReplacementStrategy}
		}
	}
	s, err = str("scope", false)
	if err != nil {
		return nil, err
	}
	if _, present := a.GetVariableValue("scope"); present {
		switch s {
		case "Chunk":
		case "Sentence":
			r.scope = Sentence
		default:
			return nil, Error{Kind: InvalidScope}
		}
	}
	s, err = str("kind", false)
	if err != nil {
		return nil, err
	}
	if _, present := a.GetVariableValue("kind"); present {
		k, ok := lint.LintKindFromStringKey(s)
		if !ok {
			return nil, Error{Kind: InvalidLintKind}
		}
		r.kind = k
	}
	return r, nil
}
func (r *WeirLinter) Description() string { return r.description }
func (r *WeirLinter) Expr() expr.Expr     { return r.expression }
func (r *WeirLinter) Unit() lint.DocumentIterator {
	if r.scope == Sentence {
		return lint.Sentence{}
	}
	return lint.Chunk{}
}
func (r *WeirLinter) IntoChunkLinter() (*WeirLinter, bool)    { return r, r.scope == Chunk }
func (r *WeirLinter) IntoSentenceLinter() (*WeirLinter, bool) { return r, r.scope == Sentence }
func (r *WeirLinter) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
	if len(tokens) == 0 {
		return nil
	}
	span := text.Span{Start: tokens[0].Span.Start, End: tokens[len(tokens)-1].Span.End}
	original := span.GetContent(source)
	suggestions := make([]lint.Suggestion, 0, len(r.replacements))
	for _, s := range r.replacements {
		replacement := []rune(s)
		if r.strategy == MatchCase {
			replacement = text.CopyCasing(original, replacement)
		}
		suggestions = append(suggestions, lint.ReplaceWith(replacement))
	}
	return &lint.Lint{Span: span, LintKind: r.kind, Message: r.message, Priority: 31, Suggestions: suggestions}
}
func (r *WeirLinter) Lint(d *doc.Document) []lint.Lint { return lint.RunExprLinter(r, d) }
func (r *WeirLinter) Tests() []TestCase                { return r.ast.Tests() }
func (r *WeirLinter) CountTests() int {
	count := 0
	for _, s := range r.ast.Stmts {
		if s.Kind == Test || s.Kind == Allows {
			count++
		}
	}
	return count
}
func applySuggestion(input string, l lint.Lint, s lint.Suggestion) string {
	chars := []rune(input)
	s.Apply(l.Span, &chars)
	return string(chars)
}
func (r *WeirLinter) RunTest(tc TestCase) *TestResult {
	type state struct {
		s     string
		depth int
	}
	queue := []state{{tc.Input, 0}}
	seen := map[string]bool{tc.Input: true}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.s == tc.Expected {
			if len(r.Lint(doc.NewPlainEnglishCurated(cur.s))) == 0 {
				return nil
			}
			return &TestResult{tc.Expected, cur.s}
		}
		if cur.depth >= lint.MaxSuggestionTransformationDepth {
			continue
		}
		ls := r.Lint(doc.NewPlainEnglishCurated(cur.s))
		if len(ls) > 0 {
			for _, s := range ls[0].Suggestions {
				next := applySuggestion(cur.s, ls[0], s)
				if !seen[next] {
					seen[next] = true
					queue = append(queue, state{next, cur.depth + 1})
				}
			}
		}
	}
	got := tc.Input
	for range lint.MaxSuggestionTransformationDepth {
		ls := r.Lint(doc.NewPlainEnglishCurated(got))
		if len(ls) == 0 || len(ls[0].Suggestions) == 0 {
			break
		}
		got = applySuggestion(got, ls[0], ls[0].Suggestions[0])
	}
	return &TestResult{tc.Expected, got}
}
func (r *WeirLinter) RunTests() []TestResult {
	var out []TestResult
	for _, tc := range r.Tests() {
		if f := r.RunTest(tc); f != nil {
			out = append(out, *f)
		}
	}
	return out
}
