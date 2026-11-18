package pos

import (
	"encoding/json"
	"strings"
	"unicode/utf8"
)

// Tagger assigns optional universal POS tags to a sentence.
type Tagger interface{ TagSentence([]string) []*UPOS }

// Chunker identifies each token belonging to a noun phrase.
type Chunker interface {
	ChunkSentence([]string, []*UPOS) []bool
}

type FreqDict struct {
	Mapping map[string]UPOS `json:"mapping"`
}

func NewFreqDict() *FreqDict { return &FreqDict{Mapping: make(map[string]UPOS)} }
func (f *FreqDict) Get(word string) (UPOS, bool) {
	u, ok := f.Mapping[strings.ToLower(word)]
	return u, ok
}
func (f *FreqDict) TagSentence(sentence []string) []*UPOS {
	out := make([]*UPOS, len(sentence))
	values := make([]UPOS, len(sentence))
	for i, w := range sentence {
		if u, ok := f.Get(w); ok {
			values[i] = u
			out[i] = &values[i]
		}
	}
	return out
}

type PatchCriteriaKind uint8

const (
	WordIsTaggedWith PatchCriteriaKind = iota
	AnyWordIsTaggedWith
	SandwichTaggedWith
	WordIs
	NounPhraseAt
	Combined
)

type PatchCriteria struct {
	Kind           PatchCriteriaKind
	Relative       int    `json:"relative"`
	MaxRelative    int    `json:"max_relative"`
	IsTagged       UPOS   `json:"is_tagged"`
	PrevWordTagged UPOS   `json:"prev_word_tagged"`
	PostWordTagged UPOS   `json:"post_word_tagged"`
	Word           string `json:"word"`
	IsNP           bool   `json:"is_np"`
	A, B           *PatchCriteria
}

func (p *PatchCriteria) UnmarshalJSON(data []byte) error {
	var variants map[string]json.RawMessage
	if err := json.Unmarshal(data, &variants); err != nil {
		return err
	}
	type fields PatchCriteria
	for name, b := range variants {
		switch name {
		case "WordIsTaggedWith":
			p.Kind = WordIsTaggedWith
		case "AnyWordIsTaggedWith":
			p.Kind = AnyWordIsTaggedWith
		case "SandwichTaggedWith":
			p.Kind = SandwichTaggedWith
		case "WordIs":
			p.Kind = WordIs
		case "NounPhraseAt":
			p.Kind = NounPhraseAt
		case "Combined":
			p.Kind = Combined
		default:
			continue
		}
		return json.Unmarshal(b, (*fields)(p))
	}
	return nil
}
func (p PatchCriteria) Fulfils(tokens []string, tags []*UPOS, flags []bool, index int) bool {
	tagged := func(i int, u UPOS) bool { return i >= 0 && i < len(tags) && tags[i] != nil && *tags[i] == u }
	switch p.Kind {
	case WordIsTaggedWith:
		return tagged(index+p.Relative, p.IsTagged)
	case AnyWordIsTaggedWith:
		far := index + p.MaxRelative
		if far < 0 {
			return false
		}
		lo, hi := index, far
		if lo > hi {
			lo, hi = hi, lo
		}
		if hi > len(tags) {
			hi = len(tags)
		}
		for i := lo; i < hi; i++ {
			if tagged(i, p.IsTagged) {
				return true
			}
		}
		return false
	case SandwichTaggedWith:
		return tagged(index-1, p.PrevWordTagged) && tagged(index+1, p.PostWordTagged)
	case WordIs:
		i := index + p.Relative
		if i < 0 || i >= len(tokens) {
			return false
		}
		a, b := tokens[i], p.Word
		// Harper compares zipped characters, deliberately ignoring unequal lengths.
		for len(a) > 0 && len(b) > 0 {
			x, nx := utf8.DecodeRuneInString(a)
			y, ny := utf8.DecodeRuneInString(b)
			a, b = a[nx:], b[ny:]
			if x >= 'A' && x <= 'Z' {
				x += 32
			}
			if y >= 'A' && y <= 'Z' {
				y += 32
			}
			if x != y {
				return false
			}
		}
		return true
	case NounPhraseAt:
		i := index + p.Relative
		return i >= 0 && i < len(flags) && flags[i] == p.IsNP
	case Combined:
		return p.A.Fulfils(tokens, tags, flags, index) && p.B.Fulfils(tokens, tags, flags, index)
	}
	return false
}

type TaggerPatch struct {
	From     UPOS          `json:"from"`
	To       UPOS          `json:"to"`
	Criteria PatchCriteria `json:"criteria"`
}
type BrillTagger struct {
	Base    Tagger
	Patches []TaggerPatch
}

func NewBrillTagger(base Tagger) *BrillTagger { return &BrillTagger{Base: base} }
func (b *BrillTagger) UnmarshalJSON(data []byte) error {
	var v struct {
		Base    FreqDict      `json:"base"`
		Patches []TaggerPatch `json:"patches"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	b.Base = &v.Base
	b.Patches = v.Patches
	return nil
}
func (b *BrillTagger) TagSentence(sentence []string) []*UPOS {
	tags := b.Base.TagSentence(sentence)
	for _, p := range b.Patches {
		for i := range sentence {
			if tags[i] != nil && *tags[i] == p.From && p.Criteria.Fulfils(sentence, tags, nil, i) {
				tags[i] = new(p.To)
			}
		}
	}
	return tags
}

type UPOSFreqDict struct {
	Counts map[UPOS]int `json:"counts"`
}

func NewUPOSFreqDict() *UPOSFreqDict                    { return &UPOSFreqDict{Counts: make(map[UPOS]int)} }
func (f *UPOSFreqDict) IsLikelyNPComponent(u UPOS) bool { return f.Counts[u] > 0 }
func (f *UPOSFreqDict) ChunkSentence(_ []string, tags []*UPOS) []bool {
	out := make([]bool, len(tags))
	for i, u := range tags {
		out[i] = u != nil && f.IsLikelyNPComponent(*u)
	}
	return out
}

type ChunkerPatch struct {
	From     bool          `json:"from"`
	Criteria PatchCriteria `json:"criteria"`
}
type BrillChunker struct {
	Base    *UPOSFreqDict  `json:"base"`
	Patches []ChunkerPatch `json:"patches"`
}

func NewBrillChunker(base *UPOSFreqDict) *BrillChunker { return &BrillChunker{Base: base} }
func (b *BrillChunker) ChunkSentence(sentence []string, tags []*UPOS) []bool {
	out := b.Base.ChunkSentence(sentence, tags)
	for _, p := range b.Patches {
		for i := range sentence {
			if out[i] == p.From && p.Criteria.Fulfils(sentence, tags, out, i) {
				out[i] = !out[i]
			}
		}
	}
	return out
}
