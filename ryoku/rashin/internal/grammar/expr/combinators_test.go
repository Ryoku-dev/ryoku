package expr

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/pos"
	"ryoku-rashin/internal/grammar/text"
	"testing"
)

func TestOraclePhraseParity(t *testing.T) {
	cases := []struct{ input, output string }{{"foo bar", "matched"}, {"FOO BAR", "MATCHED"}, {"foo  bar", "matched"}, {"baz foo bar quux", "baz matched quux"}, {"foo baz bar", "foo baz bar"}}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			d := doc.NewPlainEnglishCurated(tc.input)
			matches := IterMatchesInDoc(NewSequenceExprWordSeq([]string{"foo", "bar"}), d)
			source := d.GetSource()
			for i := len(matches) - 1; i >= 0; i-- {
				span := matches[i]
				tokens := d.GetTokens()[span.Start:span.End]
				start, end := tokens[0].Span.Start, tokens[len(tokens)-1].Span.End
				replacement := text.CopyCasing(source[start:end], []rune("matched"))
				result := append([]rune(nil), source[:start]...)
				result = append(result, replacement...)
				source = append(result, source[end:]...)
			}
			if string(source) != tc.output {
				t.Fatalf("got %q, want oracle output %q", string(source), tc.output)
			}
		})
	}
}
func TestCollectionsAndFiltering(t *testing.T) {
	short, long := NewWord("foo"), NewSequenceExprWordSeq([]string{"foo", "bar"})
	assertStrings(t, NewFirstMatchOf([]Expr{short, long}), "foo bar", []string{"foo"})
	assertStrings(t, NewLongestMatchOf([]Expr{short, long}), "foo bar", []string{"foo bar"})
	assertStrings(t, NewAll([]Expr{short, long}), "foo bar", []string{"foo bar"})
	assertNoMatches(t, NewAll([]Expr{short, NewWord("baz")}), "foo bar")
	assertNoMatches(t, NewAll(), "foo")
	assertStrings(t, NewFilter([]Expr{NewSequenceExprWordSeq([]string{"chock", "full"}), NewWhitespacePattern()}), "This test is chock full of insights.", []string{" "})
	assertNoMatches(t, NewFilter(nil), "foo")
	assertNoMatches(t, NewFilter([]Expr{short, NewWord("bar")}), "foo")
	assertStrings(t, Or(short, NewWord("bar")), "foo bar", []string{"foo", "bar"})
	assertStrings(t, And(short, long), "foo bar", []string{"foo bar"})
	assertNoMatches(t, ButNot(short, long), "foo bar")
	assertStrings(t, OrLongest(short, long), "foo bar", []string{"foo bar"})
}
func TestMapAndWordGroup(t *testing.T) {
	d := doc.NewPlainEnglishCurated("foo bar Foo")
	m := NewExprMap[string]()
	m.Insert(NewWord("foo"), "first")
	m.Insert(NewWord("bar"), "second")
	if v, ok := m.Lookup(0, d.GetTokens(), d.GetSource()); !ok || v != "first" {
		t.Fatal(v, ok)
	}
	if _, ok := m.Lookup(1, d.GetTokens(), d.GetSource()); ok {
		t.Fatal("unexpected whitespace lookup")
	}
	assertStrings(t, m, "foo bar Foo", []string{"foo", "bar", "Foo"})
	g := NewWordExprGroup()
	g.Add("foo", NewSequenceExprWordSeq([]string{"foo", "bar"}))
	g.AddWord("bar")
	assertStrings(t, g, "foo bar Foo", []string{"foo bar"})
	assertStrings(t, g, "bar Foo", []string{"bar"})
}
func TestNegativeStepAndZeroWidth(t *testing.T) {
	backward := ExprFunc(func(c int, t []doc.Token, s []rune) (text.Span, bool) {
		if c < 1 {
			return text.Span{}, false
		}
		return text.Span{Start: c - 1, End: c}, true
	})
	d := doc.NewPlainEnglishCurated("a b")
	e := NewUnlessStep(NewWord("absent"), backward)
	step, ok := e.Step(d.GetTokens(), 2, d.GetSource())
	if !ok || step != -1 {
		t.Fatal(step, ok)
	}
	span, ok := e.Run(2, d.GetTokens(), d.GetSource())
	if !ok || span != (text.Span{Start: 1, End: 2}) {
		t.Fatal(span, ok)
	}
	assertStrings(t, NewSequenceExprAco("a").ThenOptional(NewWord("missing")), "a", []string{"a"})
	assertNoMatches(t, NewSequenceExprUnless(NewWord("a")), "a")
	assertStrings(t, NewSequenceExprUnless(NewWord("b")), "a", []string{"a"})
}
func TestExceptionPredicatesIgnoreASCIIcase(t *testing.T) {
	assertNoMatches(t, NewSequenceExpr().ThenWordExcept([]string{"foo"}), "FOO")
	assertNoMatches(t, NewSequenceExpr().ThenKindExcept(doc.TokenKind.IsWord, []string{"foo"}), "FOO")
	assertStrings(t, NewSequenceExpr().ThenKindAnyOrWords(nil, []string{"foo"}), "FOO", []string{"FOO"})
}
func TestWordSetContainsDoesNotNormalize(t *testing.T) {
	set := NewWordSet([]string{"They're"})
	if !set.Contains("THEY'RE") || set.Contains("They’re") {
		t.Fatal("Contains should only ASCII-fold, unlike MatchesToken")
	}
	assertStrings(t, set, "They’re", []string{"They’re"})
}
func TestSpecialPatternVariants(t *testing.T) {
	assertStrings(t, NewInflectionOfBe(), "be am is are was were been being", []string{"be", "am", "is", "are", "was", "were", "been", "being"})
	assertStrings(t, NewModalVerbWithoutCommonErrors(), "can can't cannot cannt", []string{"can", "can't", "cannot"})
	assertStrings(t, NewModalVerbWithCommonErrors(), "can can't cannot cannt", []string{"can", "can't", "cannot", "cannt"})
	assertStrings(t, PrepositionalPreceder(), "prone to used attached", []string{"prone", "used", "attached"})
	assertStrings(t, NewRelativePronoun(), "that which who whom whose", []string{"that", "which", "who", "whom", "whose"})
	assertStrings(t, NewTimeUnitExpr(), "ms day day's days weekend weekends weekend's", []string{"ms", "day", "day's", "days", "weekend", "weekends", "weekend's"})
	assertStrings(t, NewTimeUnitExprPluralsOnly(), "ms day day's days weekend weekends weekend's", []string{"days", "weekends"})
	assertStrings(t, NewWithinEditDistanceFromStr("hello", 1), "hello HELLO hallo hellos kitten", []string{"hello", "HELLO", "hallo", "hellos"})
	assertStrings(t, NewSimilarToPhraseFromPhrase("hello world", 1), "hello world hallo world", []string{"hallo world"})
	assertStrings(t, NewFixedPhraseFromPhrase("hello, 10"), "HELLO, 20", []string{"HELLO, 20"})
}
func TestDerivedAndUPOS(t *testing.T) {
	assertStrings(t, NewDerivedFromStr("call"), "call Call calling recall", []string{"call", "Call", "calling", "recall"})
	source := []rune("a")
	tag := pos.NOUN
	tokens := []doc.Token{{Span: text.Span{Start: 0, End: 1}, Kind: doc.TokenKind{Kind: doc.Word, Metadata: &dict.DictWordMetadata{PosTag: &tag}}}}
	if _, ok := NewUPOSSet([]pos.UPOS{pos.NOUN}).Run(0, tokens, source); !ok {
		t.Fatal("NOUN tag not matched")
	}
	if _, ok := NewUPOSSet([]pos.UPOS{pos.VERB}).Run(0, tokens, source); ok {
		t.Fatal("wrong tag matched")
	}
}
func TestExactKindUsesMetadataValues(t *testing.T) {
	first, second := dict.DictWordMetadata{Common: true}, dict.DictWordMetadata{Common: true}
	tokens := []doc.Token{{Kind: doc.TokenKind{Kind: doc.Word, Metadata: &first}}}
	if _, ok := NewSequenceExpr().ThenExactKind(doc.TokenKind{Kind: doc.Word, Metadata: &second}).Run(0, tokens, nil); !ok {
		t.Fatal("equivalent metadata must compare by value")
	}
}

func TestRepeatingZeroWidthReleaseSemantics(t *testing.T) {
	empty := ExprFunc(func(c int, _ []doc.Token, _ []rune) (text.Span, bool) {
		return text.Span{Start: c, End: c}, true
	})
	for _, tc := range []struct {
		cursor int
		want   text.Span
	}{{0, text.Span{Start: 0, End: 0}}, {1, text.Span{Start: 0, End: 2}}} {
		got, ok := NewRepeating(empty, 3).Run(tc.cursor, nil, nil)
		if !ok || got != tc.want {
			t.Fatalf("cursor %d: got %v, %v; want %v", tc.cursor, got, ok, tc.want)
		}
	}
}
