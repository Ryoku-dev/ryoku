package lint_test

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"

	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/expr"
	"ryoku-rashin/internal/grammar/lint"
	"ryoku-rashin/internal/grammar/linttest"
	"ryoku-rashin/internal/grammar/text"
)

func TestInsertCommaAfter(t *testing.T) {
	chars := []rune("This is a test")
	lint.InsertAfter([]rune(",")).Apply(text.Span{Start: 0, End: 4}, &chars)
	if string(chars) != "This, is a test" {
		t.Fatal(string(chars))
	}
}
func TestSuggestionYourMatchCase(t *testing.T) {
	actual := lint.ReplaceWithMatchCase([]rune("you are"), []rune("You're"))
	if !reflect.DeepEqual(actual, lint.ReplaceWith([]rune("You are"))) {
		t.Fatal(actual)
	}
}
func TestIssue1065(t *testing.T) {
	actual := lint.ReplaceWithMatchCase([]rune("stackoverflow"), []rune("Stack Overflow"))
	if !reflect.DeepEqual(actual, lint.ReplaceWith([]rune("StackOverflow"))) {
		t.Fatal(actual)
	}
}

type testExprLinter struct {
	expression        expr.Expr
	context, sentence bool
	calls             int
}

func newTestExpr(phrase string) *testExprLinter {
	return &testExprLinter{expression: expr.NewFixedPhraseFromPhrase(phrase)}
}
func (l *testExprLinter) Expr() expr.Expr   { return l.expression }
func (*testExprLinter) Description() string { return "test linter" }
func (l *testExprLinter) Unit() lint.DocumentIterator {
	if l.sentence {
		return lint.Sentence{}
	}
	return lint.Chunk{}
}
func tokenString(tokens []doc.Token, source []rune) string {
	if len(tokens) == 0 {
		return ""
	}
	return string(source[tokens[0].Span.Start:tokens[len(tokens)-1].Span.End])
}
func (l *testExprLinter) MatchToLint(tokens []doc.Token, source []rune) *lint.Lint {
	return l.MatchToLintWithContext(tokens, source, nil)
}
func (l *testExprLinter) MatchToLintWithContext(tokens []doc.Token, source []rune, c *lint.MatchContext) *lint.Lint {
	l.calls++
	if len(tokens) == 0 {
		return nil
	}
	result := lint.NewLint()
	result.Span = text.Span{Start: tokens[0].Span.Start, End: tokens[len(tokens)-1].Span.End}
	result.Message = "simple"
	replacement := "2"
	if l.sentence {
		replacement = "2&2"
		result.Message = "sentence"
	}
	if l.context {
		if c == nil || len(c.Before) == 0 || len(c.After) == 0 {
			return nil
		}
		before, after := tokenString(c.Before, source), tokenString(c.After, source)
		result.Message = "dunno"
		replacement = "?"
		if strings.EqualFold(before, "one ") && strings.EqualFold(after, " three") {
			result.Message = "ascending"
			replacement = ">"
		} else if strings.EqualFold(before, "three ") && strings.EqualFold(after, " one") {
			result.Message = "descending"
			replacement = "<"
		}
	}
	result.Suggestions = []lint.Suggestion{lint.ReplaceWith([]rune(replacement))}
	return &result
}
func TestSimpleTest123(t *testing.T) {
	linttest.AssertSuggestionResult(t, "one two three", newTestExpr("two"), "one 2 three")
}
func TestContextTest123(t *testing.T) {
	l := newTestExpr("two")
	l.context = true
	linttest.AssertSuggestionResult(t, "one two three", l, "one > three")
}
func TestContextTest321(t *testing.T) {
	l := newTestExpr("two")
	l.context = true
	linttest.AssertSuggestionResult(t, "three two one", l, "three < one")
}
func TestSentenceTest123(t *testing.T) {
	l := newTestExpr("two, two")
	l.sentence = true
	linttest.AssertSuggestionResult(t, "one, two, two, three", l, "one, 2&2, three")
}
func TestSuggestionApplyRuneOffsetsAndCapacity(t *testing.T) {
	for _, tc := range []struct {
		source   string
		span     text.Span
		s        lint.Suggestion
		expected string
	}{{"éabc語", text.Span{Start: 1, End: 4}, lint.ReplaceWith([]rune("Z")), "éZ語"}, {"éabc語", text.Span{Start: 1, End: 4}, lint.ReplaceWith([]rune("more text")), "émore text語"}, {"éabc語", text.Span{Start: 1, End: 4}, lint.Remove(), "é語"}, {"éabc語", text.Span{Start: 1, End: 4}, lint.InsertAfter([]rune("!")), "éabc!語"}} {
		for _, capacity := range []int{len([]rune(tc.source)), 40} {
			chars := make([]rune, len([]rune(tc.source)), capacity)
			copy(chars, []rune(tc.source))
			tc.s.Apply(tc.span, &chars)
			if string(chars) != tc.expected {
				t.Fatalf("%s != %s", string(chars), tc.expected)
			}
		}
	}
}
func TestGroupOrderingCacheAndConfig(t *testing.T) {
	g := lint.NewLintGroup()
	l := newTestExpr("two")
	if !g.AddChunkExprLinter("Z", l) || g.AddChunkExprLinter("Z", l) {
		t.Fatal("duplicate registration")
	}
	g.Config.SetRuleEnabled("Z", true)
	first := g.Lint(doc.NewPlainEnglishCurated("one two three"))
	if len(first) != 1 || l.calls != 1 {
		t.Fatal(first, l.calls)
	}
	first[0].Suggestions[0].Replacement[0] = 'X'
	second := g.Lint(doc.NewPlainEnglishCurated("one two three"))
	if l.calls != 1 || string(second[0].Suggestions[0].Replacement) != "2" {
		t.Fatal("cache mutated", second)
	}
	shifted := g.Lint(doc.NewPlainEnglishCurated("zero, one two three"))
	if len(shifted) != 1 || shifted[0].Span.Start != 10 {
		t.Fatal(shifted)
	}
	g.Config.SetRuleEnabled("Z", false)
	if len(g.Lint(doc.NewPlainEnglishCurated("one two three"))) != 0 {
		t.Fatal("disabled cached result")
	}
	g.Config.SetRuleEnabled("Z", true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := g.OrganizedLintsContext(ctx, doc.NewPlainEnglishCurated("one two three")); err != context.Canceled {
		t.Fatal(err)
	}
}
func TestGroupPreservesEmptyEnabledResultsAcrossCacheHits(t *testing.T) {
	g := lint.NewLintGroup()
	g.AddChunkExprLinter("Empty", newTestExpr("two"))
	g.Config.SetRuleEnabled("Empty", true)
	document := doc.NewPlainEnglishCurated("one three")
	for range 2 {
		results := g.OrganizedLints(document)
		if values, present := results["Empty"]; !present || len(values) != 0 {
			t.Fatalf("missing enabled empty result: %#v", results)
		}
	}
	g.Config.SetRuleEnabled("Empty", false)
	if results := g.OrganizedLints(document); len(results) != 0 {
		t.Fatalf("disabled rule survived cached empty result: %#v", results)
	}
}
func TestRemoveOverlapsMatchesHarperOrdering(t *testing.T) {
	input := []lint.Lint{{Span: text.Span{Start: 2, End: 4}, Priority: 0}, {Span: text.Span{Start: 1, End: 5}, Priority: 100}, {Span: text.Span{Start: 1, End: 5}, Priority: 2}, {Span: text.Span{Start: 5, End: 6}, Priority: 1}}
	actual := lint.RemoveOverlaps(input)
	if len(actual) != 2 || actual[0].Priority != 2 || actual[1].Span.Start != 5 {
		t.Fatal(actual)
	}
	m := map[string][]lint.Lint{"B": {{Span: text.Span{Start: 1, End: 5}, Priority: 2}}, "A": {{Span: text.Span{Start: 2, End: 4}}, {Span: text.Span{Start: 5, End: 6}}}}
	lint.RemoveOverlapsMap(m)
	if len(m["B"]) != 1 || len(m["A"]) != 1 || m["A"][0].Span.Start != 5 {
		t.Fatal(m)
	}
}
func TestInitialismExpansionCase(t *testing.T) {
	l := lint.NewInitialismLinter("idk", []string{"I Do Not Know"})
	linttest.AssertSuggestionResult(t, "idk", l, "i do not know")
	linttest.AssertSuggestionResult(t, "IDK", l, "I do not know")
}
func TestPooledLinterConcurrent(t *testing.T) {
	p := lint.NewPooledLinter(func() *lint.InitialismLinter { return lint.NewInitialismLinter("idk", []string{"I do not know"}) })
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() { linttest.AssertLintCount(t, "idk", p.Clone(), 1) })
	}
	wg.Wait()
}
func TestContextHelpers(t *testing.T) {
	d := doc.NewPlainEnglishCurated("one two three")
	tokens := d.GetTokens()
	c := &lint.MatchContext{Before: tokens[:2], After: tokens[3:]}
	word := func(tok *doc.Token) bool { return tok.Kind.IsWord() }
	if !lint.FollowedByWord(c, word) || !lint.PrecededByWord(c, word) || !lint.SurroundedByWords(c, func(a, b *doc.Token) bool { return word(a) && word(b) }) {
		t.Fatal("word context")
	}
	if lint.AtStartOfSentence(c) || !lint.AtStartOfSentence(&lint.MatchContext{}) || lint.AtStartOfSentence(nil) {
		t.Fatal("sentence start")
	}
	if lint.FollowedByWord(nil, word) || lint.PrecededByWord(nil, word) || lint.FollowedByToken(nil, word) {
		t.Fatal("nil context")
	}
	predicate := func(tok *doc.Token, src []rune) bool { return tok.GetStr(src) == "two" }
	i, ok := lint.FindTheOnlyTokenIndexMatching(tokens, d.GetSource(), predicate)
	if !ok || i != 2 || lint.FindTheOnlyTokenMatching(tokens, d.GetSource(), predicate) == nil {
		t.Fatal(i, ok)
	}
	if _, ok := lint.FindTheOnlyTokenIndexMatching(tokens, d.GetSource(), func(tok *doc.Token, _ []rune) bool { return tok.Kind.IsWord() }); ok {
		t.Fatal("multiple matches")
	}
}

type cancelRule struct {
	cancel context.CancelFunc
	calls  int
}

func (*cancelRule) Description() string { return "Cancellation fixture." }
func (r *cancelRule) Lint(*doc.Document) []lint.Lint {
	r.calls++
	if r.cancel != nil {
		r.cancel()
	}
	return nil
}
func TestContextCancellationBetweenRules(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	first, second := &cancelRule{cancel: cancel}, &cancelRule{}
	g := lint.NewLintGroup()
	g.Add("First", first)
	g.Add("Second", second)
	g.Config.SetRuleEnabled("First", true)
	g.Config.SetRuleEnabled("Second", true)
	_, err := g.OrganizedLintsContext(ctx, doc.NewPlainEnglishCurated("English"))
	if err != context.Canceled || first.calls != 1 || second.calls != 0 {
		t.Fatal(err, first.calls, second.calls)
	}
}
func TestGroupMergePreservesCategoryMaps(t *testing.T) {
	g, incoming := lint.NewLintGroup(), lint.NewLintGroup()
	g.Add("Same", lint.AsLinter(newTestExpr("two")))
	incoming.AddChunkExprLinter("Same", newTestExpr("two"))
	incoming.Config.SetRuleEnabled("Same", true)
	g.MergeFrom(incoming)
	if !reflect.DeepEqual(g.ClashingLinterNames, []string{"Same"}) {
		t.Fatal(g.ClashingLinterNames)
	}
	if !reflect.DeepEqual(g.IterKeys(), []string{"Same", "Same"}) {
		t.Fatal(g.IterKeys())
	}
	if result := g.Lint(doc.NewPlainEnglishCurated("two")); len(result) != 2 {
		t.Fatal(result)
	}
}
