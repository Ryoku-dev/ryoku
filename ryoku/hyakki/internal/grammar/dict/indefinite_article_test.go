package dict

import "testing"

func TestNpm4139(t *testing.T) {
	got, ok := StartsWithVowel([]rune{'n', 'p', 'm'}, American)
	if !ok || got != Vowel {
		t.Fatalf("got (%v, %v), want Some(Vowel)", got, ok)
	}
}
