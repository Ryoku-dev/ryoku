package embed

import (
	"context"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestTokenizerDeterministic(t *testing.T) {
	model, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	first := model.tokenizeIDs("RefreshHTTPServer_value42")
	second := model.tokenizeIDs("RefreshHTTPServer_value42")
	if len(first) == 0 || !reflect.DeepEqual(first, second) {
		t.Fatalf("token IDs are not deterministic: %v and %v", first, second)
	}
}
func TestEmbeddingSimilarityAndUnitLength(t *testing.T) {
	model, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	vectors, err := model.Embed(context.Background(), []string{"refresh the source index", "update source search index", "banana smoothie recipe"})
	if err != nil {
		t.Fatal(err)
	}
	for index, vector := range vectors {
		var norm float64
		for _, value := range vector {
			norm += float64(value * value)
		}
		if math.Abs(math.Sqrt(norm)-1) > 0.001 {
			t.Fatalf("vector %d length = %f", index, math.Sqrt(norm))
		}
	}
	if cosine(vectors[0], vectors[1]) <= cosine(vectors[0], vectors[2]) {
		t.Fatalf("related text was not closer: %f <= %f", cosine(vectors[0], vectors[1]), cosine(vectors[0], vectors[2]))
	}
}
func TestEmbeddingDropsUnknownAndTruncatesAt512Tokens(t *testing.T) {
	model := &Model{vocab: map[string]int{"[UNK]": 0, "a": 1, "b": 2}, matrix: []float32{1, 1, 1, 0, 0, 1}, rows: 3, dim: 2, unkID: 0, maxChars: 100}
	unknown := model.embedOne("notinvocab b")
	if math.Abs(float64(unknown[0])) > 0.001 || math.Abs(float64(unknown[1]-1)) > 0.001 {
		t.Fatalf("unknown token affected vector: %v", unknown)
	}
	long := model.embedOne(strings.Repeat("a ", 512) + "b")
	if math.Abs(float64(long[0]-1)) > 0.001 || math.Abs(float64(long[1])) > 0.001 {
		t.Fatalf("token 513 was not truncated: %v", long)
	}
}
func cosine(left, right []float32) float64 {
	var dot float64
	for index := range left {
		dot += float64(left[index] * right[index])
	}
	return dot
}
