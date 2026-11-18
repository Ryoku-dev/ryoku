package pos_test

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/pos"
)

type oracleSentence struct {
	Text        string      `json:"text"`
	Tokens      []string    `json:"tokens"`
	Spans       [][2]int    `json:"spans"`
	Tags        []*pos.UPOS `json:"tags"`
	NP          []*bool     `json:"np"`
	Annotations []string    `json:"annotations"`
	Phrases     []string    `json:"phrases"`
}

// The fixture records harper-cli 3.2.0 parse, annotate and nominal-phrases on
// 340 independently authored sentences; it contains no training corpus data.
func TestOracleParity(t *testing.T) {
	data, e := os.ReadFile("testdata/oracle.json")
	if e != nil {
		t.Fatal(e)
	}
	var cases []oracleSentence
	if e = json.Unmarshal(data, &cases); e != nil {
		t.Fatal(e)
	}
	if len(cases) < 300 {
		t.Fatal("oracle corpus must cover at least 300 sentences")
	}
	taggerSource, e := os.ReadFile("model/trained_tagger_model.json")
	if e != nil {
		t.Fatal(e)
	}
	modelSource, e := os.ReadFile("model/model.mpk")
	if e != nil {
		t.Fatal(e)
	}
	vocabSource, e := os.ReadFile("model/vocab.json")
	if e != nil {
		t.Fatal(e)
	}
	start := time.Now()
	var tagger pos.BrillTagger
	if e = json.Unmarshal(taggerSource, &tagger); e != nil {
		t.Fatal(e)
	}
	tagLoad := time.Since(start)
	start = time.Now()
	chunker, e := pos.LoadBurnChunkerCPU(modelSource, vocabSource, 6, 0.3)
	if e != nil {
		t.Fatal(e)
	}
	burnLoad := time.Since(start)
	dictionary := dict.Curated()
	tokens, observed, tagMatches, npMatches, phraseMatches, sentenceMatches, fallbacks := 0, 0, 0, 0, 0, 0, 0
	var tagElapsed, chunkElapsed time.Duration
	for _, c := range cases {
		start = time.Now()
		tags := tagger.TagSentence(c.Tokens)
		tagElapsed += time.Since(start)
		start = time.Now()
		flags := chunker.ChunkSentence(c.Tokens, tags)
		chunkElapsed += time.Since(start)
		tokens += len(c.Tokens)
		same := true
		labels := []string{}
		for i, expectedNP := range c.NP {
			if expectedNP == nil {
				continue
			}
			observed++
			// annotate exposes document metadata, not raw tagger results. Harper's
			// document.rs applies infer_pos_tag only after passing raw tags to Burn.
			tag := tags[i]
			if tag == nil {
				fallbacks++
				meta := dictionary.GetWordMetadataStr(c.Tokens[i])
				if meta == nil {
					t.Fatalf("oracle word missing dictionary metadata: %s", c.Tokens[i])
				}
				if inferred, ok := meta.InferPosTag(); ok {
					tag = new(inferred)
				}
			}
			tagOK := tag == nil && c.Tags[i] == nil || tag != nil && c.Tags[i] != nil && *tag == *c.Tags[i]
			if tagOK {
				tagMatches++
			} else {
				t.Errorf("tag %q token %d (%s): got %v want %v", c.Text, i, c.Tokens[i], tag, c.Tags[i])
				same = false
			}
			label := "NONE"
			if tag != nil {
				label = tag.String()
			}
			labels = append(labels, label)
			if flags[i] == *expectedNP {
				npMatches++
			} else {
				t.Errorf("NP %q token %d (%s): got %v want %v", c.Text, i, c.Tokens[i], flags[i], *expectedNP)
				same = false
			}
		}
		if !reflect.DeepEqual(labels, c.Annotations) {
			t.Errorf("annotate %q: got %v want %v", c.Text, labels, c.Annotations)
			same = false
		}
		phrases := []string{}
		chars := []rune(c.Text)
		lo, hi := -1, 0
		flush := func() {
			if lo >= 0 {
				phrases = append(phrases, string(chars[lo:hi]))
				lo = -1
			}
		}
		for i := range c.Tokens {
			if c.NP[i] != nil && flags[i] {
				if lo < 0 {
					lo = c.Spans[i][0]
				}
				hi = c.Spans[i][1]
			} else {
				flush()
			}
		}
		flush()
		if reflect.DeepEqual(phrases, c.Phrases) {
			phraseMatches++
		} else {
			t.Errorf("phrases %q: got %v want %v", c.Text, phrases, c.Phrases)
			same = false
		}
		if same {
			sentenceMatches++
		}
	}
	t.Logf("oracle: %d/%d exact sentences; %d raw tokens; annotate tags %d/%d (%d dictionary fallbacks as in Harper document.rs); parse NP flags %d/%d; nominal phrases %d/%d", sentenceMatches, len(cases), tokens, tagMatches, observed, fallbacks, npMatches, observed, phraseMatches, len(cases))
	t.Logf("load: Brill %s, Burn %s; uncached mean sentence: tag %s, inference %s", tagLoad, burnLoad, tagElapsed/time.Duration(len(cases)), chunkElapsed/time.Duration(len(cases)))
}
