package pos

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"sync"
	"testing"
)

func TestEmbeddedModelIdentity(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		hash string
	}{
		{"trained_tagger_model.json", taggerSource, "21b1d58967aaa40e6ab29444207de8e6f1ce3d2de335796b3ec9b90bafe3f0b4"},
		{"trained_chunker_model.json", chunkerSource, "3668f3a875d853b9a518848a52f3638beeb72d0c0dcf8259865a0aa4e77e43a3"},
		{"model.mpk", burnSource, "4cf349bd091f12e2f8904f6c13036e8e054238854ed5471650e666ae1f3d1cc1"},
		{"vocab.json", vocabSource, "aa91405ad238f0409bf625fb39067406c006cbf92e09cf2cb29f92fce9d00e5e"},
	}
	for _, c := range cases {
		if got := fmt.Sprintf("%x", sha256.Sum256(c.data)); got != c.hash {
			t.Errorf("%s differs from Harper 3.2.0: %s", c.name, got)
		}
	}
}
func TestConcurrentSharedInference(t *testing.T) {
	sentences := [][]string{{"The", "quick", "brown", "fox", "jumps", "over", "the", "lazy", "dog", "."}, {"She", "has", "been", "working", "here", "since", "Monday", "."}, {"A", "quiet", "librarian", "watched", "the", "bright", "stars", "."}}
	type result struct {
		tags  []*UPOS
		flags []bool
	}
	expected := make([]result, len(sentences))
	for i, s := range sentences {
		tags := SharedBrillTagger().TagSentence(s)
		expected[i] = result{tags, SharedBurnChunker().ChunkSentence(s, tags)}
	}
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			for i, s := range sentences {
				tags := SharedBrillTagger().TagSentence(s)
				flags := SharedBurnChunker().ChunkSentence(s, tags)
				if !reflect.DeepEqual(tags, expected[i].tags) || !reflect.DeepEqual(flags, expected[i].flags) {
					t.Errorf("concurrent inference differs for %v", s)
				}
				for j := range flags {
					flags[j] = !flags[j]
				}
			}
		})
	}
	wg.Wait()
	for i, s := range sentences {
		if flags := SharedBurnChunker().ChunkSentence(s, expected[i].tags); !reflect.DeepEqual(flags, expected[i].flags) {
			t.Errorf("returned flag mutation reached shared cache for %v", s)
		}
	}
}
