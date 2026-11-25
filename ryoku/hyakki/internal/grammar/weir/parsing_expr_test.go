package weir

import (
	"reflect"
	"ryoku-rashin/internal/grammar/doc"
	"ryoku-rashin/internal/grammar/pos"
	"ryoku-rashin/internal/grammar/text"
	"testing"
)

func TestCompactParserTokensPreserveLexicalData(t *testing.T) {
	source := "word\t 12.5\n\"quoted\\\"text\" “smart” £€₹ .,:;[]{}()@# /"
	runes := []rune(source)
	want := doc.LexWith(runes, doc.LexWeirToken)
	got := newParser(source).tokens
	if len(got) != len(want) {
		t.Fatalf("got %d tokens, want %d", len(got), len(want))
	}
	for i, token := range got {
		expected := want[i]
		if token.Span != expected.Span || token.kind != expected.Kind.Kind || token.quote != expected.Kind.IsQuote() ||
			token.punctuation != expected.Kind.Punctuation.Kind || token.currency != expected.Kind.Punctuation.Currency {
			t.Fatalf("token %d loses lexical data: %#v versus %#v", i, token, expected)
		}
		if expected.Kind.Punctuation.Quote.TwinLoc != nil {
			t.Fatalf("lexer unexpectedly produced paired quote data at token %d", i)
		}
	}
}

func TestParsesWhitespace(t *testing.T) {
	got, err := ParseExprStr(" ", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Whitespace}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesWord(t *testing.T) {
	got, err := ParseExprStr("word", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Word, Text: "word"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesWordSpace(t *testing.T) {
	got, err := ParseExprStr("word ", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Seq, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "word"}, AstExprNode{Kind: Whitespace}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesWordSpaceWord(t *testing.T) {
	got, err := ParseExprStr("word word", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Seq, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "word"}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: Word, Text: "word"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesSimpleSeq(t *testing.T) {
	got, err := ParseExprStr("a (b c) d", false)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Seq, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "a"}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: Seq, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "b"}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: Word, Text: "c"}}}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: Word, Text: "d"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesNestedSeqs(t *testing.T) {
	got, err := ParseExprStr("a (b (c)) d", false)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Seq, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "a"}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: Seq, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "b"}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: Seq, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "c"}}}}}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: Word, Text: "d"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesPairedSeqs(t *testing.T) {
	got, err := ParseExprStr("a (b) (c) d", false)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Seq, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "a"}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: Seq, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "b"}}}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: Seq, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "c"}}}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: Word, Text: "d"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesNot(t *testing.T) {
	got, err := ParseExprStr("a !b c", false)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Seq, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "a"}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: Not, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "b"}}}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: Word, Text: "c"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesNotSeq(t *testing.T) {
	got, err := ParseExprStr("a !(b c) d", false)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Seq, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "a"}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: Not, Children: []AstExprNode{AstExprNode{Kind: Seq, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "b"}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: Word, Text: "c"}}}}}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: Word, Text: "d"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesEmptyArray(t *testing.T) {
	got, err := ParseExprStr("[]", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Arr, Children: []AstExprNode{}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesSingleElementArray(t *testing.T) {
	got, err := ParseExprStr("[a]", false)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Seq, Children: []AstExprNode{AstExprNode{Kind: Arr, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "a"}}}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestOptimizerDeconstructsSingleElementArray(t *testing.T) {
	got, err := ParseExprStr("[a]", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Word, Text: "a"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestOptimizerDeconstructsSingleElementSeq(t *testing.T) {
	got, err := ParseExprStr("(a)", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Word, Text: "a"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesDoubleElementArray(t *testing.T) {
	got, err := ParseExprStr("[a, b]", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Arr, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "a"}, AstExprNode{Kind: Word, Text: "b"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesTripleElementArray(t *testing.T) {
	got, err := ParseExprStr("[a, b, c]", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Arr, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "a"}, AstExprNode{Kind: Word, Text: "b"}, AstExprNode{Kind: Word, Text: "c"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesNotTripleElementArray(t *testing.T) {
	got, err := ParseExprStr("![a, b, c]", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Not, Children: []AstExprNode{AstExprNode{Kind: Arr, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "a"}, AstExprNode{Kind: Word, Text: "b"}, AstExprNode{Kind: Word, Text: "c"}}}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesTripleDot(t *testing.T) {
	got, err := ParseExprStr("...", false)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Seq, Children: []AstExprNode{AstExprNode{Kind: Punctuation, Punctuation: text.Punctuation{Kind: text.Period}}, AstExprNode{Kind: Punctuation, Punctuation: text.Punctuation{Kind: text.Period}}, AstExprNode{Kind: Punctuation, Punctuation: text.Punctuation{Kind: text.Period}}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesSpaceComma(t *testing.T) {
	got, err := ParseExprStr("[( ), (,)]", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Arr, Children: []AstExprNode{AstExprNode{Kind: Whitespace}, AstExprNode{Kind: Punctuation, Punctuation: text.Punctuation{Kind: text.Comma}}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesSpaceDash(t *testing.T) {
	got, err := ParseExprStr("[( ), (-)]", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Arr, Children: []AstExprNode{AstExprNode{Kind: Whitespace}, AstExprNode{Kind: Punctuation, Punctuation: text.Punctuation{Kind: text.Hyphen}}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesFilter(t *testing.T) {
	got, err := ParseExprStr("<a, b, c>", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Filter, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "a"}, AstExprNode{Kind: Word, Text: "b"}, AstExprNode{Kind: Word, Text: "c"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesFilterWithSpacePrefixingElement(t *testing.T) {
	got, err := ParseExprStr("< a, b, c>", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Filter, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "a"}, AstExprNode{Kind: Word, Text: "b"}, AstExprNode{Kind: Word, Text: "c"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesFilterWithSpacePostfixingElement(t *testing.T) {
	got, err := ParseExprStr("<a, b, c >", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Filter, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "a"}, AstExprNode{Kind: Word, Text: "b"}, AstExprNode{Kind: Word, Text: "c"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesDerivative(t *testing.T) {
	got, err := ParseExprStr("$word", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: DerivativeOf, Text: "word"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesDerivativeSeq(t *testing.T) {
	got, err := ParseExprStr("$a $b $c", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Seq, Children: []AstExprNode{AstExprNode{Kind: DerivativeOf, Text: "a"}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: DerivativeOf, Text: "b"}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: DerivativeOf, Text: "c"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesContraction(t *testing.T) {
	got, err := ParseExprStr("don't do this", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Seq, Children: []AstExprNode{AstExprNode{Kind: Word, Text: "don't"}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: Word, Text: "do"}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: Word, Text: "this"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesUpos(t *testing.T) {
	got, err := ParseExprStr("PROPN NOUN VERB", false)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Seq, Children: []AstExprNode{AstExprNode{Kind: UPOSSet, UPOS: []pos.UPOS{pos.PROPN}}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: UPOSSet, UPOS: []pos.UPOS{pos.NOUN}}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: UPOSSet, UPOS: []pos.UPOS{pos.VERB}}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestOptimizesUposSet(t *testing.T) {
	got, err := ParseExprStr("[PROPN, NOUN, VERB]", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: UPOSSet, UPOS: []pos.UPOS{pos.PROPN, pos.NOUN, pos.VERB}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesProg(t *testing.T) {
	got, err := ParseExprStr("PROG", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Progressive}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesExprRef(t *testing.T) {
	got, err := ParseExprStr("@test", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: ExprRef, Text: "test"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesExprRefArray(t *testing.T) {
	got, err := ParseExprStr("[@a, @b, @c]", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Arr, Children: []AstExprNode{AstExprNode{Kind: ExprRef, Text: "a"}, AstExprNode{Kind: ExprRef, Text: "b"}, AstExprNode{Kind: ExprRef, Text: "c"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesAnything(t *testing.T) {
	got, err := ParseExprStr("*", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Anything}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
func TestParsesAnythingSeq(t *testing.T) {
	got, err := ParseExprStr("* * *", true)
	if err != nil {
		t.Fatal(err)
	}
	want := AstExprNode{Kind: Seq, Children: []AstExprNode{AstExprNode{Kind: Anything}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: Anything}, AstExprNode{Kind: Whitespace}, AstExprNode{Kind: Anything}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
