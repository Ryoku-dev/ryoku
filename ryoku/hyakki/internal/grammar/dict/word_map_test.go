package dict

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestWordMapCanonicalAndFolded(t *testing.T) {
	words := NewWordMap()
	words.Insert(NewWordMapEntryStr("hello"))
	words.Insert(NewWordMapEntryStr("Hello"))
	words.Insert(NewWordMapEntryStr("HELLO"))
	if words.Len() != 3 || words.IsEmpty() {
		t.Fatalf("unexpected size %d", words.Len())
	}
	folded, _ := NewCaseFoldedWordIdFromWordStr("hELLo")
	if !words.ContainsCaseFolded(folded) {
		t.Fatal("folded spelling missing")
	}
	entries := words.GetCaseFolded(folded)
	if len(entries) != 3 {
		t.Fatalf("got %d folded entries", len(entries))
	}
	for i, want := range []string{"hello", "Hello", "HELLO"} {
		entry := words.GetCanonical(NewCanonicalWordIdFromWordStr(want))
		if entry == nil || string(entry.CanonicalSpelling) != want {
			t.Fatalf("missing %s", want)
		}
		if string(entries[i].CanonicalSpelling) != want || words.At(i) != entry {
			t.Fatalf("insertion order differs at %d", i)
		}
	}
	if len(words.GetCaseFoldedChars([]rune("hello"))) != 3 {
		t.Fatal("folded chars lookup differs")
	}
}

func TestWordMapIndexCollisionsAndGrowth(t *testing.T) {
	for _, capacity := range []int{0, 1, 256} {
		words := NewWordMapWithCapacity(capacity)
		for i := range 256 {
			words.Insert(WordMapEntry{
				CanonicalSpelling: []rune{'x', rune(i)},
				WordIds: WordIdPair{
					Canonical:  CanonicalWordId(i * 1024),
					CaseFolded: CaseFoldedWordId((i / 2) * 1024),
				},
			})
		}
		for i := range 256 {
			entry := words.GetCanonical(CanonicalWordId(i * 1024))
			if entry == nil || entry != words.At(i) || entry.CanonicalSpelling[1] != rune(i) {
				t.Fatalf("capacity %d lost canonical entry %d", capacity, i)
			}
			variants := words.GetCaseFolded(CaseFoldedWordId((i / 2) * 1024))
			if len(variants) != 2 || variants[0] != words.At(i-i%2) || variants[1] != words.At(i-i%2+1) {
				t.Fatalf("capacity %d reordered folded entry %d", capacity, i)
			}
		}
		if words.GetCanonical(256*1024) != nil || words.ContainsCaseFolded(128*1024) {
			t.Fatalf("capacity %d matched a colliding missing ID", capacity)
		}
	}
}
func TestWordMapMergesMetadataWithoutReordering(t *testing.T) {
	words := NewWordMapWithCapacity(1)
	words.Insert(NewWordMapEntryStr("word").WithMD(DictWordMetadata{Noun: &NounData{}}))
	words.AppendWordStr("second", DictWordMetadata{})
	words.Insert(NewWordMapEntryStr("word").WithMD(DictWordMetadata{Adjective: &AdjectiveData{}}))
	if words.Len() != 2 {
		t.Fatalf("got %d words", words.Len())
	}
	entry := words.At(0)
	if string(entry.CanonicalSpelling) != "word" || !entry.Metadata.IsNoun() || !entry.Metadata.IsAdjective() {
		t.Fatal("duplicate metadata was not merged")
	}
	folded, _ := NewCaseFoldedWordIdFromWordStr("word")
	if len(words.GetCaseFolded(folded)) != 1 {
		t.Fatal("duplicate folded entry")
	}
	metadata := words.GetMetadataMutCanonical(entry.WordIds.Canonical)
	metadata.Common = true
	if !words.GetCanonical(entry.WordIds.Canonical).Metadata.Common {
		t.Fatal("metadata mutation was lost")
	}
}
func TestWordMapZeroValueAndExtension(t *testing.T) {
	var words WordMap
	if !words.IsEmpty() || words.GetCanonical(1) != nil || words.ContainsCaseFolded(1) {
		t.Fatal("zero word map is not empty")
	}
	words.AppendWord([]rune("first"), DictWordMetadata{})
	words.Extend([]WordMapEntry{NewWordMapEntryStr("second"), NewWordMapEntryStr("first")})
	words.Reserve(20)
	if words.Len() != 2 || len(words.Iter()) != 2 || !words.ContainsCanonical(NewCanonicalWordIdFromWordStr("second")) {
		t.Fatal("extension or reservation lost entries")
	}
}
func TestDerivedFromSortedUniqueSet(t *testing.T) {
	var derived DerivedFrom
	if !derived.IsEmpty() || !derived.IsNone() {
		t.Fatal("zero derivation not empty")
	}
	derived.Extend([]CanonicalWordId{3, 1, 2, 1, 3})
	if !reflect.DeepEqual(derived, DerivedFrom{1, 2, 3}) {
		t.Fatalf("got %v", derived)
	}
	for _, id := range []CanonicalWordId{1, 2, 3} {
		if !derived.Contains(id) {
			t.Errorf("missing %d", id)
		}
	}
	if derived.Contains(0) || derived.Contains(4) {
		t.Fatal("contains unknown ID")
	}
	if !reflect.DeepEqual(derived.Iter(), []CanonicalWordId{1, 2, 3}) {
		t.Fatal("iteration not ordered")
	}
	if !NewDerivedFromFromCanonicalWordId(8).Contains(8) {
		t.Fatal("single-ID construction failed")
	}
}

func TestDerivedFromUnion(t *testing.T) {
	for _, test := range []struct {
		left, right, want DerivedFrom
	}{
		{nil, nil, nil},
		{nil, DerivedFrom{2}, DerivedFrom{2}},
		{DerivedFrom{1, 3}, nil, DerivedFrom{1, 3}},
		{DerivedFrom{1, 3}, DerivedFrom{1, 3}, DerivedFrom{1, 3}},
		{DerivedFrom{1, 3}, DerivedFrom{2, 4}, DerivedFrom{1, 2, 3, 4}},
		{DerivedFrom{2, 4}, DerivedFrom{1, 3}, DerivedFrom{1, 2, 3, 4}},
		{DerivedFrom{1, 3, 5}, DerivedFrom{3, 4, 5, 6}, DerivedFrom{1, 3, 4, 5, 6}},
	} {
		got := test.left.union(test.right)
		if !reflect.DeepEqual(got, test.want) {
			t.Fatalf("union(%v, %v) = %v; want %v", test.left, test.right, got, test.want)
		}
	}
	left := make(DerivedFrom, 2, 8)
	left[0], left[1] = 1, 3
	right := DerivedFrom{2, 3}
	got := left.union(right)
	got[0] = 9
	if !reflect.DeepEqual(left, DerivedFrom{1, 3}) || !reflect.DeepEqual(right, DerivedFrom{2, 3}) {
		t.Fatal("union with additional IDs modified input storage")
	}
}
func TestDerivedFromRustJSON(t *testing.T) {
	var derived DerivedFrom
	encoded, err := json.Marshal(derived)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"inner":[]}` {
		t.Fatalf("got %s", encoded)
	}
	if err := json.Unmarshal([]byte(`{"inner":[{"hash":3},{"hash":1},{"hash":3}]}`), &derived); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(derived, DerivedFrom{1, 3}) {
		t.Fatalf("got %v", derived)
	}
}
