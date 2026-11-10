// Package embed provides Rashin's bundled pure-Go code embedding model.
package embed

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
)

const ModelName = "static:potion-code-16M"

type Model struct {
	vocab    map[string]int
	matrix   []float32
	quant    []byte
	scales   []float32
	weights  []float64
	mapping  []int
	rows     int
	dim      int
	unkID    int
	maxChars int
}

func (m *Model) Dim() int             { return m.dim }
func (m *Model) EmbedModelID() string { return ModelName }

func (m *Model) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		out[i] = m.embedOne(text)
	}
	return out, nil
}

func (m *Model) embedOne(text string) []float32 {
	ids := m.tokenizeIDs(text)
	out := make([]float32, m.dim)
	if len(ids) == 0 {
		return out
	}
	acc := make([]float64, m.dim)
	var total float64
	kept := 0
	for _, id := range ids {
		if id == m.unkID || id < 0 || id >= m.rows {
			continue
		}
		if kept >= 512 {
			break
		}
		kept++
		row := id
		if m.mapping != nil {
			row = m.mapping[id]
		}
		weight := 1.0
		if m.weights != nil {
			weight = m.weights[id]
		}
		off := row * m.dim
		if m.quant != nil {
			quantizedWeight := weight * float64(m.scales[row])
			for j := range m.dim {
				acc[j] += quantizedWeight * float64(int8(m.quant[off+j]))
			}
		} else {
			for j := range m.dim {
				acc[j] += weight * float64(m.matrix[off+j])
			}
		}
		total += weight
	}
	if total == 0 {
		return out
	}
	var norm float64
	for i := range acc {
		acc[i] /= total
		norm += acc[i] * acc[i]
	}
	if norm == 0 {
		return out
	}
	inv := 1 / math.Sqrt(norm)
	for i := range acc {
		out[i] = float32(acc[i] * inv)
	}
	return out
}

func loadModel(modelBytes, tokenizerBytes []byte) (*Model, error) {
	tensors, err := loadSafetensors(modelBytes)
	if err != nil {
		return nil, err
	}
	vocab, unknown, maxChars, err := loadTokenizer(tokenizerBytes)
	if err != nil {
		return nil, err
	}
	if len(vocab) != tensors.rows {
		return nil, fmt.Errorf("embed: vocabulary has %d rows, model has %d", len(vocab), tensors.rows)
	}
	if tensors.rows != 61826 || tensors.dim != 256 {
		return nil, fmt.Errorf("embed: bundled model shape is %dx%d, want 61826x256", tensors.rows, tensors.dim)
	}
	return &Model{vocab: vocab, matrix: tensors.matrix, quant: tensors.quant, scales: tensors.scales, weights: tensors.weights, mapping: tensors.mapping, rows: tensors.rows, dim: tensors.dim, unkID: unknown, maxChars: maxChars}, nil
}

func loadTokenizer(raw []byte) (map[string]int, int, int, error) {
	var document struct {
		Model struct {
			Vocab             map[string]int `json:"vocab"`
			UnkToken          string         `json:"unk_token"`
			MaxInputCharsWord *int           `json:"max_input_chars_per_word"`
		} `json:"model"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		return nil, 0, 0, fmt.Errorf("parse bundled tokenizer: %w", err)
	}
	if len(document.Model.Vocab) == 0 {
		return nil, 0, 0, fmt.Errorf("bundled tokenizer has no vocabulary")
	}
	unknown := document.Model.UnkToken
	if unknown == "" {
		unknown = "[UNK]"
	}
	unknownID, ok := document.Model.Vocab[unknown]
	if !ok {
		return nil, 0, 0, fmt.Errorf("bundled tokenizer has no unknown token %q", unknown)
	}
	maxChars := 100
	if document.Model.MaxInputCharsWord != nil && *document.Model.MaxInputCharsWord > 0 {
		maxChars = *document.Model.MaxInputCharsWord
	}
	return document.Model.Vocab, unknownID, maxChars, nil
}
