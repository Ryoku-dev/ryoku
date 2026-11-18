package expr

import (
	"reflect"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/text"
	"testing"
)

func matchStrings(e Expr, d *doc.Document) []string {
	out := []string{}
	for _, sp := range IterMatchesInDoc(e, d) {
		var chars []rune
		for _, tok := range d.GetTokens()[sp.Start:sp.End] {
			chars = append(chars, tok.GetCh(d.GetSource())...)
		}
		out = append(out, string(chars))
	}
	return out
}
func assertStrings(t *testing.T, e Expr, input string, want []string) {
	t.Helper()
	got := matchStrings(e, doc.NewPlainEnglishCurated(input))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%q: got %#v, want %#v", input, got, want)
	}
}
func assertNoMatches(t *testing.T, e Expr, inputs ...string) {
	t.Helper()
	for _, input := range inputs {
		assertStrings(t, e, input, []string{})
	}
}
func assertSpans(t *testing.T, got, want []text.Span) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
func TestKindBoth(t *testing.T) {
	assertStrings(t, NewSequenceExpr().ThenKindBoth(doc.TokenKind.IsNoun, doc.TokenKind.IsVerb), "Use a good example.", []string{"Use", "good", "example"})
}
func TestAdjectiveOrDeterminer(t *testing.T) {
	assertStrings(t, NewSequenceExpr().ThenKindEither(doc.TokenKind.IsAdjective, doc.TokenKind.IsDeterminer), "Use a good example.", []string{"a", "good"})
}
func TestNounButNotAdjective(t *testing.T) {
	assertStrings(t, NewSequenceExpr().ThenKindIsButIsNot(doc.TokenKind.IsNoun, doc.TokenKind.IsAdjective), "Use a good example.", []string{"Use", "example"})
}
func TestStartCapture(t *testing.T) {
	e := NewSequenceExpr().ThenExactWord("hello").TWs().StartCapture().ThenExactWord("world")
	assertStrings(t, e, "hello world", []string{"world"})
	assertNoMatches(t, e, "world", " world")
}
func TestEndCapture(t *testing.T) {
	e := NewSequenceExpr().ThenExactWord("hello").EndCapture().TWs().ThenExactWord("world")
	assertStrings(t, e, "hello world", []string{"hello"})
	assertNoMatches(t, e, "hello", "hello ")
}
func TestStartEndCapture(t *testing.T) {
	e := NewSequenceExpr().ThenExactWord("hello").TWs().StartCapture().ThenExactWord("world").EndCapture().TWs().ThenExactWord("testing")
	assertStrings(t, e, "hello world testing", []string{"world"})
	assertNoMatches(t, e, "hello world", "world testing", "world")
}
func TestStartEndCaptureMultiToken(t *testing.T) {
	e := NewSequenceExpr().ThenExactWord("hello").TWs().StartCapture().ThenExactWord("world").TWs().ThenExactWord("testing").EndCapture().TWs().ThenAnyWord()
	assertStrings(t, e, "hello world testing something else", []string{"world testing"})
}
func TestStartEndCaptureMultiMatch(t *testing.T) {
	e := NewSequenceExpr().ThenExactWord("hello").TWs().StartCapture().ThenAnyWord().TWs().ThenExactWord("world").EndCapture().TWs().ThenExactWord("testing")
	assertStrings(t, e, "hello one world testing hello two world testing hello three world something else", []string{"one world", "two world"})
}
func TestStartEndCaptureMultiTokenExpr(t *testing.T) {
	e := NewSequenceExpr().ThenExactWord("hello").TWs().StartCapture().Then(NewRepeating(NewSequenceExpr().ThenExactWord("world").TOws(), 3)).EndCapture().ThenExactWord("testing")
	assertStrings(t, e, "hello world world world testing", []string{"world world world "})
	assertNoMatches(t, e, "hello world world world", "world world world testing", "world world world")
}
func fooAtEnd() Expr {
	return NewSequenceExprAco("foo").ThenAnyOf([]Expr{NewSequenceExprWhitespace().TAco("bar").Then(NewAnchorEnd()), NewAnchorEnd()})
}
func TestFlagFooFollowedByBarOrAtEnd1(t *testing.T) {
	d := doc.NewPlainEnglishCurated("foo bar")
	sp := IterMatchesInDoc(fooAtEnd(), d)
	assertSpans(t, sp, []text.Span{{Start: 0, End: 3}})
	assertStrings(t, fooAtEnd(), "foo bar", []string{"foo bar"})
}
func TestFlagFooFollowedByBarOrAtEnd2(t *testing.T) {
	d := doc.NewPlainEnglishCurated("foo")
	assertSpans(t, IterMatchesInDoc(fooAtEnd(), d), []text.Span{{Start: 0, End: 1}})
	assertStrings(t, fooAtEnd(), "foo", []string{"foo"})
}
func TestFlagFooFollowedByBarOrAtEnd3(t *testing.T) {
	assertStrings(t, fooAtEnd(), "foo bar baz", []string{})
}
func TestAnchorEndMatchesPeriod(t *testing.T) {
	assertSpans(t, IterMatchesInDoc(NewAnchorEnd(), doc.NewPlainEnglishCurated("This is a test.")), []text.Span{{Start: 7, End: 7}})
}
func TestAnchorEndDoesNotMatchEmpty(t *testing.T) {
	assertSpans(t, IterMatchesInDoc(NewAnchorEnd(), doc.NewPlainEnglishCurated("")), nil)
}
func TestWordAtEndOfDocument(t *testing.T) {
	d := doc.NewPlainEnglishCurated("This is the end")
	m := IterMatchesInDoc(NewSequenceExpr().ThenAnyCapitalizationOf("end").Then(NewAnchorEnd()), d)
	if len(m) != 1 || m[0].Start != 6 {
		t.Fatal(m)
	}
}
func TestWordNotAtEndOfDoc(t *testing.T) {
	assertNoMatches(t, NewSequenceExpr().ThenAnyCapitalizationOf("end").Then(NewAnchorEnd()), "This is the end, really")
}
func TestWordAtEndOfChunk(t *testing.T) {
	d := doc.NewPlainEnglishCurated("hello, world")
	chunks := d.IterChunks()
	m := IterMatches(NewSequenceExpr().ThenAnyCapitalizationOf("hello").Then(NewAnchorEnd()), chunks[0], d.GetSource())
	if len(m) != 1 {
		t.Fatal(m)
	}
}
func TestCompareWithAnchorStart(t *testing.T) {
	d := doc.NewPlainEnglishCurated("Start here")
	m := IterMatchesInDoc(NewSequenceExpr().Then(NewAnchorStart()).ThenAnyCapitalizationOf("start"), d)
	if len(m) != 1 || m[0].Start != 0 {
		t.Fatal(m)
	}
}
func TestWordWithTrailingWhitespaceAtEndOfDoc(t *testing.T) {
	d := doc.NewPlainEnglishCurated("foo ")
	m := IterMatchesInDoc(NewSequenceExpr().ThenAnyCapitalizationOf("foo").ThenWhitespace().Then(NewAnchorEnd()), d)
	if len(m) != 1 || m[0].End != 2 {
		t.Fatal(m)
	}
}
func TestAnchorStartMatchesFirstWord(t *testing.T) {
	assertSpans(t, IterMatchesInDoc(NewAnchorStart(), doc.NewPlainEnglishCurated("This is a test.")), []text.Span{{Start: 0, End: 0}})
}
func TestAnchorStartDoesNotMatchEmpty(t *testing.T) {
	assertSpans(t, IterMatchesInDoc(NewAnchorStart(), doc.NewPlainEnglishCurated("")), nil)
}
func riseExpr() Expr {
	return NewSequenceExprWith(NewNot(NewAnchorStart())).ThenAnyCapitalizationOf("give").ThenWhitespace().ThenAnyCapitalizationOf("the").ThenWhitespace().ThenAnyCapitalizationOf("rise").ThenWhitespace().ThenAnyCapitalizationOf("to")
}
func TestRejectsExpressionAtStart(t *testing.T) {
	assertNoMatches(t, riseExpr(), "Give the rise to power.")
}
func TestMatchesExpressionAfterStart(t *testing.T) {
	assertStrings(t, riseExpr(), "They give the rise to power.", []string{"give the rise to"})
}
func TestRepeatingMatchesAnything(t *testing.T) {
	d := doc.NewPlainEnglishCurated("This matcher will match the entirety of any document!")
	m := IterMatchesInDoc(NewRepeating(NewSequenceExprFrom(NewAnyPattern()), 0), d)
	if len(m) == 0 || m[0] != (text.Span{Start: 0, End: len(d.GetTokens())}) {
		t.Fatal(m)
	}
}
func TestRepeatingDoesNotMatchShort(t *testing.T) {
	assertNoMatches(t, NewRepeating(NewSequenceExprFrom(NewAnyPattern()), 4), "No match")
}
func TestNotCaseSensitive(t *testing.T) {
	lower := doc.NewPlainEnglishCurated("hello world")
	upper := doc.NewPlainEnglishCurated("HELLO WORLD")
	title := doc.NewPlainEnglishCurated("Hello World")
	phrase := NewFixedPhraseFromDocument(lower)
	for _, pair := range [][2]*doc.Document{{lower, title}, {lower, upper}, {title, lower}, {title, upper}, {upper, lower}, {upper, title}} {
		sp, ok := phrase.Run(0, pair[0].GetTokens(), pair[1].GetSource())
		if !ok || sp != (text.Span{Start: 0, End: 3}) {
			t.Fatalf("%v %v", sp, ok)
		}
	}
}
