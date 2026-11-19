// Package expr ports Harper's declarative token expressions and patterns.
package expr

import (
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/text"
)

// Expr returns the token window matched at a cursor, including backward windows.
type Expr interface {
	Run(int, []doc.Token, []rune) (text.Span, bool)
}

// Step moves a cursor by a signed token offset when its condition matches.
type Step interface {
	Step([]doc.Token, int, []rune) (int, bool)
}

// Pattern matches at the beginning of a token slice and returns the match length.
type Pattern interface {
	Matches([]doc.Token, []rune) (int, bool)
}

// SingleTokenPattern is the single-token specialization of Pattern.
type SingleTokenPattern interface{ MatchesToken(doc.Token, []rune) bool }
type ExprFunc func(int, []doc.Token, []rune) (text.Span, bool)

func (f ExprFunc) Run(c int, t []doc.Token, s []rune) (text.Span, bool) { return f(c, t, s) }

type Predicate func(doc.Token, []rune) bool

func (p Predicate) MatchesToken(t doc.Token, s []rune) bool { return p(t, s) }
func (p Predicate) Matches(t []doc.Token, s []rune) (int, bool) {
	if len(t) > 0 && p(t[0], s) {
		return 1, true
	}
	return 0, false
}
func (p Predicate) Step(t []doc.Token, c int, s []rune) (int, bool) {
	if c < 0 || c >= len(t) {
		return 0, false
	}
	return p.Matches(t[c:], s)
}
func (p Predicate) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	if c >= 0 && c < len(t) && p(t[c], s) {
		return text.Span{Start: c, End: c + 1}, true
	}
	return text.Span{}, false
}
func singleMatches(p SingleTokenPattern, t []doc.Token, s []rune) (int, bool) {
	if len(t) > 0 && p.MatchesToken(t[0], s) {
		return 1, true
	}
	return 0, false
}
func patternStep(p Pattern, t []doc.Token, c int, s []rune) (int, bool) {
	if c < 0 || c > len(t) {
		return 0, false
	}
	return p.Matches(t[c:], s)
}
func stepRun(p Step, c int, t []doc.Token, s []rune) (text.Span, bool) {
	n, ok := p.Step(t, c, s)
	if !ok || c+n < 0 {
		return text.Span{}, false
	}
	if n < 0 {
		return text.Span{Start: c + n, End: c}, true
	}
	return text.Span{Start: c, End: c + n}, true
}

// AsExpr implements Rust's blanket pattern-to-step-to-expression conversions.
func AsExpr(value any) Expr {
	switch v := value.(type) {
	case Expr:
		return v
	case Step:
		return ExprFunc(func(c int, t []doc.Token, s []rune) (text.Span, bool) { return stepRun(v, c, t, s) })
	case Pattern:
		return ExprFunc(func(c int, t []doc.Token, s []rune) (text.Span, bool) {
			n, ok := patternStep(v, t, c, s)
			return text.Span{Start: c, End: c + n}, ok
		})
	case SingleTokenPattern:
		return AsExpr(Predicate(v.MatchesToken))
	case func(doc.Token, []rune) bool:
		return Predicate(v)
	case func(*doc.Token, []rune) bool:
		return Predicate(func(t doc.Token, s []rune) bool { return v(&t, s) })
	}
	panic("unsupported expression")
}
func IntoBoxed(e any) Expr     { return AsExpr(e) }
func IntoBoxedExpr(e any) Expr { return AsExpr(e) }

// IterMatches filters overlapping windows, preferring the first match.
func IterMatches(e Expr, t []doc.Token, s []rune) []text.Span {
	var out []text.Span
	lastEnd := 0
	for c := range t {
		if span, ok := e.Run(c, t, s); ok && span.Start >= lastEnd {
			out = append(out, span)
			lastEnd = span.End
		}
	}
	return out
}
func IterMatchesInDoc(e Expr, d *doc.Document) []text.Span {
	return IterMatches(e, d.GetTokens(), d.GetSource())
}

// FindAllMatches locates non-overlapping pattern matches, advancing even for empty matches.
func FindAllMatches(p Pattern, t []doc.Token, s []rune) []text.Span {
	var out []text.Span
	for c := 0; c < len(t); {
		n, ok := p.Matches(t[c:], s)
		if ok {
			out = append(out, text.Span{Start: c, End: c + n})
			c += max(n, 1)
		} else {
			c++
		}
	}
	return out
}
func FindAllMatchesInDoc(p Pattern, d *doc.Document) []text.Span {
	return FindAllMatches(p, d.GetTokens(), d.GetSource())
}
func Or(a, b any) *FirstMatchOf          { return NewFirstMatchOf([]Expr{AsExpr(a), AsExpr(b)}) }
func And(a, b any) *All                  { return NewAll([]Expr{AsExpr(a), AsExpr(b)}) }
func ButNot(a, b any) *All               { return And(a, NewUnlessStep(AsExpr(b), NewAnyPattern())) }
func OrLongest(a, b any) *LongestMatchOf { return NewLongestMatchOf([]Expr{AsExpr(a), AsExpr(b)}) }

// FirstMatchOf returns the first successful child, without comparing match lengths.
type FirstMatchOf struct{ exprs []Expr }

func NewFirstMatchOf(es ...[]Expr) *FirstMatchOf {
	e := &FirstMatchOf{}
	for _, x := range es {
		e.exprs = append(e.exprs, x...)
	}
	return e
}
func (e *FirstMatchOf) Add(x any)       { e.exprs = append(e.exprs, AsExpr(x)) }
func (e *FirstMatchOf) AddBoxed(x Expr) { e.Add(x) }
func (e *FirstMatchOf) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	for _, x := range e.exprs {
		if sp, ok := x.Run(c, t, s); ok {
			return sp, true
		}
	}
	return text.Span{}, false
}

// LongestMatchOf returns the longest successful child; equal lengths prefer the last.
type LongestMatchOf struct{ exprs []Expr }

func NewLongestMatchOf(es ...[]Expr) *LongestMatchOf {
	e := &LongestMatchOf{}
	for _, x := range es {
		e.exprs = append(e.exprs, x...)
	}
	return e
}
func (e *LongestMatchOf) Add(x any)       { e.exprs = append(e.exprs, AsExpr(x)) }
func (e *LongestMatchOf) AddBoxed(x Expr) { e.Add(x) }
func (e *LongestMatchOf) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	var best text.Span
	found := false
	for _, x := range e.exprs {
		if sp, ok := x.Run(c, t, s); ok && (!found || sp.End-sp.Start >= best.End-best.Start) {
			best = sp
			found = true
		}
	}
	return best, found
}

// All requires every child to match and returns the longest child's window.
type All struct{ children []Expr }

func NewAll(es ...[]Expr) *All {
	e := &All{}
	for _, x := range es {
		e.children = append(e.children, x...)
	}
	return e
}
func (e *All) Add(x any) { e.children = append(e.children, AsExpr(x)) }
func (e *All) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	var best text.Span
	found := false
	for _, x := range e.children {
		sp, ok := x.Run(c, t, s)
		if !ok {
			return text.Span{}, false
		}
		if !found || sp.End-sp.Start > best.End-best.Start {
			best = sp
			found = true
		}
	}
	return best, found
}

// Optional turns failure into a successful zero-width match.
type Optional struct{ inner Expr }

func NewOptional(e any) *Optional { return &Optional{AsExpr(e)} }
func (e *Optional) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	if sp, ok := e.inner.Run(c, t, s); ok {
		return sp, true
	}
	return text.Span{Start: c, End: c}, true
}

// Not is a zero-width assertion that succeeds only when its child fails.
type Not struct{ inner Expr }

func NewNot(e any) *Not { return &Not{AsExpr(e)} }
func (e *Not) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	_, ok := e.inner.Run(c, t, s)
	return text.Span{Start: c, End: c}, !ok
}

// Repeating greedily consumes a child until failure, requiring a minimum count.
type Repeating struct {
	inner    Expr
	required int
}

func NewRepeating(e any, n int) *Repeating { return &Repeating{AsExpr(e), n} }
func (e *Repeating) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	window := text.Span{Start: c, End: c}
	n := 0
	for {
		sp, ok := e.inner.Run(c, t, s)
		if !ok {
			return window, n >= e.required
		}
		window.ExpandToInclude(sp.Start)
		if sp.End == 0 {
			// Harper's release build wraps usize::MAX + 1 to zero here.
			window.End = 0
		} else {
			window.ExpandToInclude(sp.End - 1)
		}
		if sp.Start == sp.End {
			return window, true
		}
		if sp.Start < c {
			c = sp.Start
		} else {
			c = sp.End
		}
		n++
	}
}

// Filter narrows each preceding result to the first match of the next expression.
type Filter struct{ steps []Expr }

func NewFilter(es []Expr) *Filter { return &Filter{es} }
func (e *Filter) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	if len(e.steps) == 0 {
		return text.Span{}, false
	}
	result, ok := e.steps[0].Run(c, t, s)
	if !ok {
		return result, false
	}
	for _, step := range e.steps[1:] {
		found := false
		for i := range result.End - result.Start {
			sp, ok := step.Run(i, t[result.Start:result.End], s)
			if ok {
				result = text.Span{Start: sp.Start + result.Start, End: sp.End + result.Start}
				found = true
				break
			}
		}
		if !found {
			return text.Span{}, false
		}
	}
	return result, true
}

// UnlessStep runs a step only when the condition expression does not match.
type UnlessStep struct {
	condition Expr
	step      Expr
}

func NewUnlessStep(condition Expr, step any) *UnlessStep { return &UnlessStep{condition, AsExpr(step)} }
func (e *UnlessStep) Step(t []doc.Token, c int, s []rune) (int, bool) {
	if _, ok := e.condition.Run(c, t, s); ok {
		return 0, false
	}
	sp, ok := e.step.Run(c, t, s)
	if sp.Start < c {
		return sp.Start - c, ok
	}
	return sp.End - c, ok
}
func (e *UnlessStep) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	return stepRun(e, c, t, s)
}

// AnchorStart asserts the cursor is on the first word-like token without consuming it.
type AnchorStart struct{}

func NewAnchorStart() *AnchorStart { return &AnchorStart{} }
func (e AnchorStart) Step(t []doc.Token, c int, s []rune) (int, bool) {
	for i, x := range t {
		if x.Kind.IsWordLike() {
			return 0, i == c
		}
	}
	return 0, false
}
func (e AnchorStart) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	return stepRun(e, c, t, s)
}

// AnchorEnd asserts the cursor is at or beyond the last non-whitespace token.
type AnchorEnd struct{}

func NewAnchorEnd() *AnchorEnd { return &AnchorEnd{} }
func (e AnchorEnd) Step(t []doc.Token, c int, s []rune) (int, bool) {
	for i := len(t) - 1; i >= 0; i-- {
		if !t[i].Kind.IsWhitespace() {
			return 0, c >= i
		}
	}
	return 0, false
}
func (e AnchorEnd) Run(c int, t []doc.Token, s []rune) (text.Span, bool) { return stepRun(e, c, t, s) }

type exprRow[T any] struct {
	key   Expr
	value T
}

// ExprMap associates values with expressions, selecting the first matching row.
type ExprMap[T any] struct{ rows []exprRow[T] }

func NewExprMap[T any]() *ExprMap[T]    { return &ExprMap[T]{} }
func (e *ExprMap[T]) Insert(x any, v T) { e.rows = append(e.rows, exprRow[T]{AsExpr(x), v}) }
func (e *ExprMap[T]) Lookup(c int, t []doc.Token, s []rune) (T, bool) {
	for i := range e.rows {
		row := &e.rows[i]
		if _, ok := row.key.Run(c, t, s); ok {
			return row.value, true
		}
	}
	var zero T
	return zero, false
}
func (e *ExprMap[T]) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	for i := range e.rows {
		row := &e.rows[i]
		if sp, ok := row.key.Run(c, t, s); ok {
			return sp, true
		}
	}
	return text.Span{}, false
}

// WordExprGroup dispatches expressions by the exact, case-sensitive first word.
type WordExprGroup struct{ exprs map[string]*FirstMatchOf }

func NewWordExprGroup() *WordExprGroup { return &WordExprGroup{} }
func (e *WordExprGroup) Add(word string, x any) {
	if e.exprs == nil {
		e.exprs = make(map[string]*FirstMatchOf)
	}
	g := e.exprs[word]
	if g == nil {
		g = NewFirstMatchOf()
		e.exprs[word] = g
	}
	g.Add(x)
}
func (e *WordExprGroup) AddWord(word string) { e.Add(word, NewWordExact(word)) }
func (e *WordExprGroup) Run(c int, t []doc.Token, s []rune) (text.Span, bool) {
	if c < 0 || c >= len(t) || !t[c].Kind.IsWord() {
		return text.Span{}, false
	}
	g := e.exprs[string(t[c].GetCh(s))]
	if g == nil {
		return text.Span{}, false
	}
	return g.Run(c, t, s)
}
