package pos

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestUPOS(t *testing.T) {
	predicates := []func(UPOS) bool{UPOS.IsAdj, UPOS.IsAdp, UPOS.IsAdv, UPOS.IsAux, UPOS.IsCconj, UPOS.IsDet, UPOS.IsIntj, UPOS.IsNoun, UPOS.IsNum, UPOS.IsPart, UPOS.IsPron, UPOS.IsPropn, UPOS.IsPunct, UPOS.IsSconj, UPOS.IsSym, UPOS.IsVerb}
	for i, name := range uposNames {
		u, ok := ParseUPOS(name)
		if !ok || int(u) != i {
			t.Fatalf("parse %s: %v %v", name, u, ok)
		}
		if u.AsRefStr() != name || u.String() != uposDescriptions[i] {
			t.Fatalf("display %s", name)
		}
		if !predicates[i](u) {
			t.Fatalf("predicate %s", name)
		}
		b, e := json.Marshal(u)
		if e != nil || string(b) != "\""+name+"\"" {
			t.Fatalf("marshal %s: %s %v", name, b, e)
		}
		var got UPOS
		if e = json.Unmarshal(b, &got); e != nil || got != u {
			t.Fatalf("round trip %s", name)
		}
		text, _ := u.MarshalText()
		if e = got.UnmarshalText(text); e != nil || got != u {
			t.Fatalf("text round trip %s", name)
		}
	}
	for _, u := range []UPOS{NOUN, PROPN, PRON} {
		if !u.IsNominal() {
			t.Fatalf("nominal %v", u)
		}
	}
	if VERB.IsNominal() {
		t.Fatal("verb nominal")
	}
	if _, ok := ParseUPOS("X"); ok {
		t.Fatal("X must not parse")
	}
	var u UPOS
	if json.Unmarshal([]byte(`"invalid"`), &u) == nil {
		t.Fatal("accepted invalid UPOS")
	}
}
func TestFreqDict(t *testing.T) {
	f := NewFreqDict()
	f.Mapping["hello"] = INTJ
	tags := f.TagSentence([]string{"HELLO", "Hello", "unknown"})
	if tags[0] == nil || *tags[0] != INTJ || tags[1] == nil || *tags[1] != INTJ || tags[2] != nil {
		t.Fatalf("tags %v", tags)
	}
	*tags[0] = NOUN
	if *tags[1] != INTJ {
		t.Fatal("tag values alias")
	}
	u, ok := f.Get("hello")
	if !ok || u != INTJ {
		t.Fatal("returned tags mutate model")
	}
}
func TestPatchCriteria(t *testing.T) {
	tokens := []string{"The", "small", "cat", "sleeps"}
	tags := []*UPOS{new(DET), new(ADJ), new(NOUN), new(VERB)}
	flags := []bool{true, true, true, false}
	cases := []struct {
		name string
		p    PatchCriteria
		i    int
		want bool
	}{
		{"relative", PatchCriteria{Kind: WordIsTaggedWith, Relative: -1, IsTagged: ADJ}, 2, true},
		{"underflow", PatchCriteria{Kind: WordIsTaggedWith, Relative: -1, IsTagged: DET}, 0, false},
		{"overflow", PatchCriteria{Kind: WordIsTaggedWith, Relative: 2, IsTagged: VERB}, 3, false},
		{"any forward includes current", PatchCriteria{Kind: AnyWordIsTaggedWith, MaxRelative: 2, IsTagged: DET}, 0, true},
		{"any forward excludes endpoint", PatchCriteria{Kind: AnyWordIsTaggedWith, MaxRelative: 2, IsTagged: NOUN}, 0, false},
		{"any backward excludes current", PatchCriteria{Kind: AnyWordIsTaggedWith, MaxRelative: -2, IsTagged: NOUN}, 2, false},
		{"any backward includes endpoint", PatchCriteria{Kind: AnyWordIsTaggedWith, MaxRelative: -2, IsTagged: DET}, 2, true},
		{"any underflow rejects entire range", PatchCriteria{Kind: AnyWordIsTaggedWith, MaxRelative: -3, IsTagged: DET}, 1, false},
		{"any zero empty", PatchCriteria{Kind: AnyWordIsTaggedWith, IsTagged: DET}, 0, false},
		{"sandwich", PatchCriteria{Kind: SandwichTaggedWith, PrevWordTagged: DET, PostWordTagged: NOUN}, 1, true},
		{"sandwich edge", PatchCriteria{Kind: SandwichTaggedWith, PrevWordTagged: DET, PostWordTagged: NOUN}, 0, false},
		{"word ASCII insensitive", PatchCriteria{Kind: WordIs, Relative: -2, Word: "the"}, 2, true},
		{"word zipped prefix", PatchCriteria{Kind: WordIs, Word: "sm"}, 1, true},
		{"word zipped empty", PatchCriteria{Kind: WordIs, Word: ""}, 1, true},
		{"word differs", PatchCriteria{Kind: WordIs, Word: "big"}, 1, false},
		{"np true", PatchCriteria{Kind: NounPhraseAt, Relative: -1, IsNP: true}, 2, true},
		{"np false", PatchCriteria{Kind: NounPhraseAt, Relative: 1, IsNP: false}, 2, true},
		{"np out of range", PatchCriteria{Kind: NounPhraseAt, Relative: 9, IsNP: false}, 2, false},
		{"combined", PatchCriteria{Kind: Combined, A: &PatchCriteria{Kind: WordIs, Word: "cat"}, B: &PatchCriteria{Kind: WordIsTaggedWith, IsTagged: NOUN}}, 2, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.p.Fulfils(tokens, tags, flags, c.i); got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}
func TestPatchInPlaceOrdering(t *testing.T) {
	f := NewFreqDict()
	f.Mapping["a"] = NOUN
	f.Mapping["b"] = NOUN
	b := NewBrillTagger(f)
	b.Patches = []TaggerPatch{{NOUN, VERB, PatchCriteria{Kind: WordIs, Word: "a"}}, {NOUN, ADJ, PatchCriteria{Kind: WordIsTaggedWith, Relative: -1, IsTagged: VERB}}}
	tags := b.TagSentence([]string{"a", "b"})
	if *tags[0] != VERB || *tags[1] != ADJ {
		t.Fatal("patch ordering")
	}
	base := NewUPOSFreqDict()
	base.Counts[NOUN] = 1
	c := NewBrillChunker(base)
	c.Patches = []ChunkerPatch{{true, PatchCriteria{Kind: WordIs, Word: "a"}}, {true, PatchCriteria{Kind: NounPhraseAt, Relative: -1, IsNP: false}}}
	if got := c.ChunkSentence([]string{"a", "b"}, []*UPOS{new(NOUN), new(NOUN)}); !reflect.DeepEqual(got, []bool{false, false}) {
		t.Fatalf("chunk patch ordering %v", got)
	}
}
func TestModelLoading(t *testing.T) {
	start := time.Now()
	b, e := LoadBurnChunkerCPU(burnSource, vocabSource, 6, 0.3)
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("Burn model load %s; vocab %d; embedding %dx%d; hidden %d", time.Since(start), len(b.vocab), len(b.vocab), b.embedDim, b.hiddenDim)
	if len(b.vocab) != 32960 || len(b.words) != 197760 || len(b.upos) != 160 || len(b.linear) != 28 {
		t.Fatal("model dimensions")
	}
	if len(b.ChunkSentence(nil, nil)) != 0 {
		t.Fatal("empty sentence")
	}
	tokens := []string{"The", "quick", "brown", "fox", "jumps", "over", "the", "lazy", "dog", "."}
	tags := SharedBrillTagger().TagSentence(tokens)
	want := []bool{true, true, true, true, false, false, true, true, true, false}
	if got := b.ChunkSentence(tokens, tags); !reflect.DeepEqual(got, want) {
		t.Fatalf("chunk %v want %v", got, want)
	}
	if SharedBrillTagger() != SharedBrillTagger() || SharedBrillChunker() != SharedBrillChunker() || SharedBurnChunker() != SharedBurnChunker() {
		t.Fatal("shared model identities")
	}
	unknown := SharedBrillTagger().TagSentence([]string{"librarian", "watchman", "clever", "apples", "soundly"})
	for i, tag := range unknown {
		if tag != nil {
			t.Fatalf("raw tagger must preserve None for %d: %v", i, tag)
		}
	}
}
func TestRecordRejectsTruncation(t *testing.T) {
	for _, data := range [][]byte{nil, {0x81}, {0xd9, 20}, {0xc4, 3, 1}, {0xdc, 0xff, 0xff}, {0xde, 0xff, 0xff}} {
		r := recordReader{data: data}
		if _, e := r.value(); e == nil {
			t.Fatalf("accepted truncated %v", data)
		}
	}
	if _, e := LoadBurnChunkerCPU(burnSource[:100], vocabSource, 6, 0.3); e == nil {
		t.Fatal("accepted truncated model")
	}
}

type countedChunker struct{ calls int }

func (c *countedChunker) ChunkSentence(s []string, _ []*UPOS) []bool {
	c.calls++
	out := make([]bool, len(s))
	if len(out) > 0 {
		out[0] = true
	}
	return out
}
func TestCachedChunker(t *testing.T) {
	inner := &countedChunker{}
	c := NewCachedChunker(inner, 2)
	a, b, d := []string{"a"}, []string{"b"}, []string{"d"}
	tags := []*UPOS{nil}
	first := c.ChunkSentence(a, tags)
	first[0] = false
	if got := c.ChunkSentence(a, tags); !got[0] || inner.calls != 1 {
		t.Fatal("cache copy or hit")
	}
	c.ChunkSentence(b, tags)
	c.ChunkSentence(a, tags)
	c.ChunkSentence(d, tags)
	c.ChunkSentence(b, tags)
	if inner.calls != 4 {
		t.Fatalf("LRU calls %d", inner.calls)
	}
	c.ChunkSentence(b, []*UPOS{new(NOUN)})
	if inner.calls != 5 {
		t.Fatal("tags absent from cache key")
	}
	if chunkKey([]string{"a", "bc"}, nil) == chunkKey([]string{"ab", "c"}, nil) {
		t.Fatal("ambiguous cache key")
	}
}
func BenchmarkBurnLoad(b *testing.B) {
	for range b.N {
		if _, e := LoadBurnChunkerCPU(burnSource, vocabSource, 6, 0.3); e != nil {
			b.Fatal(e)
		}
	}
}
func BenchmarkBurnSentence(b *testing.B) {
	model, e := LoadBurnChunkerCPU(burnSource, vocabSource, 6, 0.3)
	if e != nil {
		b.Fatal(e)
	}
	sentence := []string{"The", "quick", "brown", "fox", "jumps", "over", "the", "lazy", "dog", "."}
	tags := SharedBrillTagger().TagSentence(sentence)
	b.ResetTimer()
	for range b.N {
		model.ChunkSentence(sentence, tags)
	}
}
