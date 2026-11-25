package pos

import (
	"encoding/json"
	"fmt"
	"math"
)

type gate struct{ input, hidden []float32 }
type recurrent struct{ gates [4]gate }

// BurnChunker runs Harper's bidirectional LSTM using float32 arithmetic.
type BurnChunker struct {
	vocab               map[string]int
	words, upos         []float32
	forward, reverse    recurrent
	linear              []float32
	bias                float32
	embedDim, hiddenDim int
}
type BurnChunkerCPU = BurnChunker

func LoadBurnChunkerCPU(modelBytes, vocabBytes []byte, embedDim int, dropout float32) (*BurnChunker, error) {
	if embedDim <= 0 {
		return nil, fmt.Errorf("embedding dimension must be positive")
	}
	b := &BurnChunker{embedDim: embedDim, hiddenDim: embedDim + 8}
	if e := json.Unmarshal(vocabBytes, &b.vocab); e != nil {
		return nil, e
	}
	r := recordReader{data: modelBytes}
	record, e := r.value()
	if e != nil {
		return nil, e
	}
	if r.at != len(modelBytes) {
		return nil, fmt.Errorf("trailing Burn record bytes")
	}
	item := recordAt(record, "item")
	if item == nil {
		return nil, fmt.Errorf("missing Burn model item")
	}
	if b.words, e = recordTensor(recordAt(item, "embedding_words", "weight"), len(b.vocab), embedDim); e != nil {
		return nil, e
	}
	for _, i := range b.vocab {
		if i < 0 || i >= len(b.vocab) {
			return nil, fmt.Errorf("invalid vocabulary index %d", i)
		}
	}
	if b.upos, e = recordTensor(recordAt(item, "embedding_upos", "weight"), 20, 8); e != nil {
		return nil, e
	}
	for dir, target := range map[string]*recurrent{"forward": &b.forward, "reverse": &b.reverse} {
		for i, name := range []string{"input_gate", "forget_gate", "output_gate", "cell_gate"} {
			g := &target.gates[i]
			if g.input, e = recordTensor(recordAt(item, "lstm", dir, name, "input_transform", "weight"), b.hiddenDim, b.hiddenDim); e != nil {
				return nil, e
			}
			if g.hidden, e = recordTensor(recordAt(item, "lstm", dir, name, "hidden_transform", "weight"), b.hiddenDim, b.hiddenDim); e != nil {
				return nil, e
			}
		}
	}
	if b.linear, e = recordTensor(recordAt(item, "linear_out", "weight"), b.hiddenDim*2, 1); e != nil {
		return nil, e
	}
	bias, e := recordTensor(recordAt(item, "linear_out", "bias"), 1)
	if e != nil {
		return nil, e
	}
	b.bias = bias[0]
	return b, nil
}
func sigmoid(x float32) float32 { return 1 / (1 + float32(math.Exp(float64(-x)))) }
func tanh(x float32) float32    { return float32(math.Tanh(float64(x))) }
func (r *recurrent) run(inputs, outputs []float32, n, dim int, reverse bool) {
	// Fixed-size state covers Harper's 14 hidden units; dynamic models use one
	// contiguous allocation rather than allocating per gate or per timestep.
	var local [14 * 7]float32
	state := local[:]
	if dim > 14 {
		state = make([]float32, dim*7)
	}
	hidden, cell, next := state[:dim], state[dim:2*dim], state[2*dim:3*dim]
	gates := state[3*dim : 7*dim]
	for step := range n {
		t := step
		if reverse {
			t = n - 1 - step
		}
		x := inputs[t*dim : (t+1)*dim]
		for g := range r.gates {
			p := &r.gates[g]
			for j := range dim {
				var a, b float32
				for k := range dim {
					a += float32(x[k] * p.input[k*dim+j])
					b += float32(hidden[k] * p.hidden[k*dim+j])
				}
				v := a + b
				if g == 3 {
					v = tanh(v)
				} else {
					v = sigmoid(v)
				}
				gates[g*dim+j] = v
			}
		}
		for j := range dim {
			cell[j] = float32(gates[dim+j]*cell[j]) + float32(gates[j]*gates[3*dim+j])
			next[j] = gates[2*dim+j] * tanh(cell[j])
		}
		copy(hidden, next)
		copy(outputs[t*dim:(t+1)*dim], hidden)
	}
}
func (b *BurnChunker) ChunkSentence(sentence []string, tags []*UPOS) []bool {
	n := len(sentence)
	if len(tags) != n {
		panic("pos: sentence and tag lengths differ")
	}
	out := make([]bool, n)
	if n == 0 {
		return out
	}
	d := b.hiddenDim
	work := make([]float32, n*d*3)
	inputs, fwd, rev := work[:n*d], work[n*d:2*n*d], work[2*n*d:]
	for i, w := range sentence {
		idx, ok := b.vocab[w]
		if !ok {
			idx = 1
		}
		copy(inputs[i*d:i*d+b.embedDim], b.words[idx*b.embedDim:(idx+1)*b.embedDim])
		u := 1
		if tags[i] != nil {
			u = int(*tags[i]) + 2
		}
		copy(inputs[i*d+b.embedDim:(i+1)*d], b.upos[u*8:(u+1)*8])
	}
	b.forward.run(inputs, fwd, n, d, false)
	b.reverse.run(inputs, rev, n, d, true)
	for i := range n {
		var s float32
		for j := range d {
			s += float32(fwd[i*d+j] * b.linear[j])
		}
		for j := range d {
			s += float32(rev[i*d+j] * b.linear[d+j])
		}
		out[i] = s+b.bias > 0.5
	}
	return out
}
