package doc

import (
	"math/rand/v2"
	"ryoku-rashin/internal/grammar/dict"
	"testing"
)

func TestPreservesIcloudCamelCaseAsFirstWord(t *testing.T) {
	got := MakeTitleCaseStr("icloud syncs your files", PlainEnglish{}, dict.Curated())
	if got != "iCloud Syncs Your Files" {
		t.Fatal(got)
	}
}
func TestFirstWordIsUpcase(t *testing.T) {
	r := rand.New(rand.NewPCG(12, 34))
	dictionary := dict.Curated()
	for range 1000 {
		input := make([]rune, r.IntN(40))
		for i := range input {
			input[i] = rune('a' + r.IntN(26))
		}
		out := MakeTitleCaseChars(input, PlainEnglish{}, dictionary)
		if len(out) > 0 && asciiAlpha(out[0]) && !(out[0] >= 'A' && out[0] <= 'Z') {
			t.Fatal(string(input), string(out))
		}
	}
}
