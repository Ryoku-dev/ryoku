package dict

import (
	"encoding/json"
	"math/big"
	"testing"
)

// Independent arbitrary-precision reference to the 128-bit numeric sponge.
func referenceWordID(chars []rune) CanonicalWordId {
	accumulator := uint64(0x082efa98ec4e6c89)
	seed := uint64(0xc0ac29b7c97c50dd)
	sponge := new(big.Int).SetUint64(uint64(len(chars)))
	occupied := uint(64)
	mask := new(big.Int).SetUint64(^uint64(0))
	fold := func(a, b uint64) uint64 {
		product := new(big.Int).Mul(new(big.Int).SetUint64(a), new(big.Int).SetUint64(b))
		lo := new(big.Int).And(new(big.Int).Set(product), mask).Uint64()
		hi := new(big.Int).Rsh(product, 64).Uint64()
		return lo ^ hi
	}
	mix := func() {
		lo := new(big.Int).And(new(big.Int).Set(sponge), mask).Uint64()
		hi := new(big.Int).Rsh(new(big.Int).Set(sponge), 64).Uint64()
		accumulator = fold(lo^accumulator, hi^seed)
	}
	for _, c := range chars {
		if occupied+32 > 128 {
			mix()
			sponge.SetUint64(uint64(c))
			occupied = 32
		} else {
			value := new(big.Int).Lsh(new(big.Int).SetUint64(uint64(c)), occupied)
			sponge.Or(sponge, value)
			occupied += 32
		}
	}
	mix()
	return CanonicalWordId(accumulator)
}
func TestWordIDMatchesRustNumericSponge(t *testing.T) {
	for _, word := range []string{"", "a", "ab", "abc", "abcd", "abcde", "abcdef", "abcdefghijklmno", "can't", "Straße", "İ", "😀über"} {
		want := referenceWordID([]rune(word))
		if got := NewCanonicalWordIdFromWordStr(word); got != want {
			t.Errorf("%q: got %x, want %x", word, got, want)
		}
	}
}
func TestWordIDCaseFolding(t *testing.T) {
	for _, pair := range [][2]string{{"Hello", "hello"}, {"can’t", "can't"}, {"İ", "i\u0307"}, {"ÜBER", "über"}, {"Straße", "straße"}} {
		got, _ := NewCaseFoldedWordIdFromWordStr(pair[0])
		want := NewCanonicalWordIdFromWordStr(pair[1]).AsCaseFolded()
		if got != want {
			t.Errorf("%q folded differs from %q", pair[0], pair[1])
		}
	}
	for _, word := range []string{"hello", "über", ""} {
		_, canonical := NewCaseFoldedWordIdFromWordStr(word)
		if canonical == nil || *canonical != NewCanonicalWordIdFromWordStr(word) {
			t.Errorf("%q should provide canonical ID", word)
		}
	}
	for _, word := range []string{"Hello", "can't", "abc1", "can’t"} {
		_, canonical := NewCaseFoldedWordIdFromWordStr(word)
		if canonical != nil {
			t.Errorf("%q should not provide borrowed canonical ID", word)
		}
	}
}
func TestWordIDPair(t *testing.T) {
	for _, word := range []string{"hello", "Hello", "can't", "über", "İ"} {
		pair := NewWordIdPairFromWordStr(word)
		folded, _ := NewCaseFoldedWordIdFromWordStr(word)
		if pair.Canonical != NewCanonicalWordIdFromWordStr(word) || pair.CaseFolded != folded {
			t.Errorf("invalid pair for %q", word)
		}
	}
}
func TestWordIDRustJSON(t *testing.T) {
	id := CanonicalWordId(123)
	encoded, err := json.Marshal(id)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"hash":123}` {
		t.Fatalf("got %s", encoded)
	}
	var restored CanonicalWordId
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if restored != id {
		t.Fatalf("got %v, want %v", restored, id)
	}
}
