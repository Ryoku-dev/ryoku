// The numeric hash sponge is a Go adaptation of foldhash 0.2.0
// (Copyright (c) 2024 Orson Peters, Zlib license), not the original Rust implementation.
// See the distributed third-party notices.
package dict

import (
	"encoding/json"
	"fmt"
	"math/bits"
	"unicode"

	"ryoku-rashin/internal/grammar/text"
)

// CanonicalWordId identifies a word with its canonical casing.
type CanonicalWordId uint64

// CaseFoldedWordId identifies words with identical normalized lowercase spellings.
type CaseFoldedWordId uint64

// WordIdPair contains both identifiers of a word.
type WordIdPair struct {
	Canonical  CanonicalWordId  `json:"canonical"`
	CaseFolded CaseFoldedWordId `json:"case_folded"`
}

func (id CanonicalWordId) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Hash uint64 `json:"hash"`
	}{uint64(id)})
}
func (id *CanonicalWordId) UnmarshalJSON(data []byte) error {
	hash, err := unmarshalWordID(data)
	if err == nil {
		*id = CanonicalWordId(hash)
	}
	return err
}
func (id CaseFoldedWordId) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Hash uint64 `json:"hash"`
	}{uint64(id)})
}
func (id *CaseFoldedWordId) UnmarshalJSON(data []byte) error {
	hash, err := unmarshalWordID(data)
	if err == nil {
		*id = CaseFoldedWordId(hash)
	}
	return err
}
func unmarshalWordID(data []byte) (uint64, error) {
	var value struct {
		Hash *uint64 `json:"hash"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return 0, err
	}
	if value.Hash == nil {
		return 0, fmt.Errorf("missing word ID hash")
	}
	return *value.Hash, nil
}

// NewCanonicalWordIdFromWordChars hashes Rust's [char] representation with
// foldhash 0.2.0 fast::FixedState: a 64-bit length followed by 32-bit scalars.
func NewCanonicalWordIdFromWordChars(chars []rune) CanonicalWordId {
	const seed = uint64(0xc0ac29b7c97c50dd)
	accumulator := uint64(0x082efa98ec4e6c89)
	lo, hi, occupied := uint64(len(chars)), uint64(0), uint(64)
	for _, c := range chars {
		if occupied == 128 {
			accumulator = foldWordMultiply(lo^accumulator, hi^seed)
			lo, hi, occupied = uint64(uint32(c)), 0, 32
		} else {
			if occupied < 64 {
				lo |= uint64(uint32(c)) << occupied
			} else {
				hi |= uint64(uint32(c)) << (occupied - 64)
			}
			occupied += 32
		}
	}
	return CanonicalWordId(foldWordMultiply(lo^accumulator, hi^seed))
}

func foldWordMultiply(a, b uint64) uint64 { hi, lo := bits.Mul64(a, b); return hi ^ lo }

func NewCanonicalWordIdFromWordStr(word string) CanonicalWordId {
	return NewCanonicalWordIdFromWordChars([]rune(word))
}

// AsCaseFolded reinterprets an ID; it does not change the word's casing.
func (id CanonicalWordId) AsCaseFolded() CaseFoldedWordId { return CaseFoldedWordId(id) }

// NewCaseFoldedWordIdFromWordChars normalizes and lowercases the word. The
// optional canonical ID is present only when Rust would borrow both transforms.
func NewCaseFoldedWordIdFromWordChars(chars []rune) (CaseFoldedWordId, *CanonicalWordId) {
	folded, canonical, borrowed := caseFoldedWordID(chars)
	if borrowed {
		return folded, &canonical
	}
	return folded, nil
}

// caseFoldedWordID keeps the optional borrowed ID by value for internal callers
// that do not need the public pointer representation.
func caseFoldedWordID(chars []rune) (CaseFoldedWordId, CanonicalWordId, bool) {
	simple := true
	borrowed := true
	for _, c := range chars {
		if c > 127 || c >= 'A' && c <= 'Z' {
			simple = false
			break
		}
		if c < 'a' || c > 'z' {
			borrowed = false
		}
	}
	if simple {
		canonical := NewCanonicalWordIdFromWordChars(chars)
		if borrowed {
			return CaseFoldedWordId(canonical), canonical, true
		}
		return CaseFoldedWordId(canonical), 0, false
	}
	normalized := text.Normalized(chars)
	lower := text.ToLower(normalized)
	canonical := NewCanonicalWordIdFromWordChars(lower)
	alreadyFolded := len(normalized) == len(chars)
	for i, c := range chars {
		if !alreadyFolded || normalized[i] != c || !(unicode.IsLower(c) || unicode.In(c, unicode.Other_Lowercase)) {
			alreadyFolded = false
			break
		}
	}
	if alreadyFolded {
		return CaseFoldedWordId(canonical), canonical, true
	}
	return CaseFoldedWordId(canonical), 0, false
}

func NewCaseFoldedWordIdFromWordStr(word string) (CaseFoldedWordId, *CanonicalWordId) {
	return NewCaseFoldedWordIdFromWordChars([]rune(word))
}

func NewWordIdPairFromWordChars(chars []rune) WordIdPair {
	folded, canonical, borrowed := caseFoldedWordID(chars)
	if borrowed {
		return WordIdPair{Canonical: canonical, CaseFolded: folded}
	}
	return WordIdPair{Canonical: NewCanonicalWordIdFromWordChars(chars), CaseFolded: folded}
}
func NewWordIdPairFromWordStr(word string) WordIdPair {
	return NewWordIdPairFromWordChars([]rune(word))
}
