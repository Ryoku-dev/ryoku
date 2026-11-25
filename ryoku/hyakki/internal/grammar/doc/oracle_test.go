package doc

import (
	"encoding/json"
	"os"
	"reflect"
	"ryoku-rashin/internal/grammar/pos"
	"ryoku-rashin/internal/grammar/text"
	"testing"
)

type oracleKind struct {
	Kind  string          `json:"kind"`
	Value json.RawMessage `json:"value"`
}
type oracleToken struct {
	Span struct{ Start, End int }
	Kind oracleKind
}
type oracleCase struct {
	Source string
	Tokens []oracleToken
}

func TestOracleTokenization(t *testing.T) {
	data, err := os.ReadFile("testdata/oracle_tokens.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []oracleCase
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	passed := 0
	for _, c := range cases {
		t.Run(c.Source, func(t *testing.T) {
			d := NewBasicTokenize(c.Source, PlainEnglish{})
			if len(d.tokens) != len(c.Tokens) {
				t.Fatalf("got %d tokens, want %d; actual %#v", len(d.tokens), len(c.Tokens), d.FatStringTokens())
			}
			names := []string{"Unlintable", "Word", "Punctuation", "Decade", "Number", "Space", "Newline", "EmailAddress", "Url", "Hostname", "ParagraphBreak", "Regexish", "HeadingStart"}
			for i, expected := range c.Tokens {
				actual := d.tokens[i]
				if actual.Span.Start != expected.Span.Start || actual.Span.End != expected.Span.End || names[actual.Kind.Kind] != expected.Kind.Kind {
					t.Errorf("token %d got %s %v want %s %v", i, names[actual.Kind.Kind], actual.Span, expected.Kind.Kind, expected.Span)
					continue
				}
				switch actual.Kind.Kind {
				case Space, Newline:
					var count int
					json.Unmarshal(expected.Kind.Value, &count)
					if actual.Kind.Count != count {
						t.Errorf("token %d count %d != %d", i, actual.Kind.Count, count)
					}
				case Number:
					var n struct {
						Value     float64
						Suffix    *string
						Radix     uint32
						Precision int
					}
					json.Unmarshal(expected.Kind.Value, &n)
					var suffix *text.OrdinalSuffix
					if n.Suffix != nil {
						x := map[string]text.OrdinalSuffix{"St": text.St, "Nd": text.Nd, "Rd": text.Rd, "Th": text.Th}[*n.Suffix]
						suffix = &x
					}
					want := text.Number{Value: n.Value, Suffix: suffix, Radix: n.Radix, Precision: n.Precision}
					if !reflect.DeepEqual(actual.Kind.Number, want) {
						t.Errorf("token %d number %#v != %#v", i, actual.Kind.Number, want)
					}
				case Punctuation:
					var p struct {
						Kind  string
						Value json.RawMessage
					}
					json.Unmarshal(expected.Kind.Value, &p)
					if p.Kind == "Quote" {
						var q struct {
							TwinLoc *int `json:"twin_loc"`
						}
						json.Unmarshal(expected.Kind.Value, &q)
						if !actual.Kind.IsQuote() || !reflect.DeepEqual(actual.Kind.Punctuation.Quote.TwinLoc, q.TwinLoc) {
							t.Errorf("token %d quote twins %#v != %#v", i, actual.Kind.Punctuation.Quote.TwinLoc, q.TwinLoc)
						}
					} else {
						puncts := map[string]text.PunctuationKind{"Degree": text.Degree, "Ellipsis": text.Ellipsis, "EnDash": text.EnDash, "EmDash": text.EmDash, "Ampersand": text.Ampersand, "Period": text.Period, "Bang": text.Bang, "Question": text.Question, "Colon": text.Colon, "Semicolon": text.Semicolon, "Comma": text.Comma, "Hyphen": text.Hyphen, "OpenSquare": text.OpenSquare, "CloseSquare": text.CloseSquare, "OpenRound": text.OpenRound, "CloseRound": text.CloseRound, "OpenCurly": text.OpenCurly, "CloseCurly": text.CloseCurly, "Hash": text.Hash, "Apostrophe": text.Apostrophe, "Percent": text.Percent, "ForwardSlash": text.ForwardSlash, "Backslash": text.Backslash, "LessThan": text.LessThan, "GreaterThan": text.GreaterThan, "Equal": text.Equal, "Star": text.Star, "Tilde": text.Tilde, "At": text.At, "Caret": text.Caret, "Plus": text.Plus, "Currency": text.CurrencyPunctuation, "Pipe": text.Pipe, "Underscore": text.Underscore, "Acute": text.Acute, "OpenSingle": text.OpenSingle, "SinglePrime": text.SinglePrime, "DoublePrime": text.DoublePrime, "Backtick": text.Backtick}
						kind, ok := puncts[p.Kind]
						if !ok || actual.Kind.Punctuation.Kind != kind {
							t.Errorf("token %d punct %v != %s", i, actual.Kind.Punctuation.Kind, p.Kind)
						}
						if p.Kind == "Currency" {
							var fields map[string]json.RawMessage
							if err := json.Unmarshal(expected.Kind.Value, &fields); err != nil {
								t.Fatal(err)
							}
							names := map[text.Currency]string{text.Dollar: "Dollar", text.Cent: "Cent", text.Euro: "Euro", text.Ruble: "Ruble", text.Lira: "Lira", text.Pound: "Pound", text.Yen: "Yen", text.Baht: "Baht", text.Won: "Won", text.Kip: "Kip", text.Rupee: "Rupee"}
							if _, ok := fields[names[actual.Kind.Punctuation.Currency]]; !ok {
								t.Errorf("token %d currency differs", i)
							}
						}
					}
				}
			}
			if !t.Failed() {
				passed++
			}
		})
	}
	t.Logf("tokenization parity: %d/%d inputs (%.2f%%)", passed, len(cases), 100*float64(passed)/float64(len(cases)))
}
func TestOracleAnnotations(t *testing.T) {
	data, err := os.ReadFile("testdata/oracle_tokens.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []oracleCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	passed := 0
	for _, c := range cases {
		t.Run(c.Source, func(t *testing.T) {
			d := NewPlainEnglishCurated(c.Source)
			if len(d.tokens) != len(c.Tokens) {
				t.Fatal("token count")
			}
			for i, expected := range c.Tokens {
				if expected.Kind.Kind != "Word" {
					continue
				}
				var metadata *struct {
					PosTag   *string `json:"pos_tag"`
					NpMember *bool   `json:"np_member"`
				}
				if err := json.Unmarshal(expected.Kind.Value, &metadata); err != nil {
					t.Fatal(err)
				}
				actual := d.tokens[i].Kind.Metadata
				if (metadata == nil) != (actual == nil) {
					t.Errorf("token %d %q vocabulary metadata differs", i, d.tokens[i].GetStr(d.source))
					continue
				}
				if metadata == nil {
					continue
				}
				var wantedTag *pos.UPOS
				if metadata.PosTag != nil {
					tag, ok := pos.ParseUPOS(*metadata.PosTag)
					if !ok {
						t.Fatal(*metadata.PosTag)
					}
					wantedTag = &tag
				}
				if !reflect.DeepEqual(actual.PosTag, wantedTag) || !reflect.DeepEqual(actual.NpMember, metadata.NpMember) {
					observed, _ := json.Marshal(struct {
						Tag *pos.UPOS
						NP  *bool
					}{actual.PosTag, actual.NpMember})
					wanted, _ := json.Marshal(struct {
						Tag *pos.UPOS
						NP  *bool
					}{wantedTag, metadata.NpMember})
					t.Errorf("token %d %q annotation got %s, want %s", i, d.tokens[i].GetStr(d.source), observed, wanted)
				}
			}
			if !t.Failed() {
				passed++
			}
		})
	}
	t.Logf("annotation parity: %d/%d inputs (%.2f%%)", passed, len(cases), 100*float64(passed)/float64(len(cases)))
}
