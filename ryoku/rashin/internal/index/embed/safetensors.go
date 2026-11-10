package embed

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
)

type staticTensors struct {
	matrix  []float32
	quant   []byte
	scales  []float32
	rows    int
	dim     int
	weights []float64
	mapping []int
}

type tensorMeta struct {
	DType   string `json:"dtype"`
	Shape   []int  `json:"shape"`
	Offsets [2]int `json:"data_offsets"`
}

func loadSafetensors(raw []byte) (*staticTensors, error) {
	if len(raw) < 8 {
		return nil, fmt.Errorf("bundled safetensors file is too short")
	}
	headerLength := binary.LittleEndian.Uint64(raw[:8])
	if headerLength > uint64(len(raw)-8) {
		return nil, fmt.Errorf("bundled safetensors header exceeds the file")
	}
	var header map[string]json.RawMessage
	if err := json.Unmarshal(raw[8:8+headerLength], &header); err != nil {
		return nil, fmt.Errorf("parse bundled safetensors header: %w", err)
	}
	base := 8 + int(headerLength)
	tensor := func(name string) ([]byte, tensorMeta, bool, error) {
		encoded, ok := header[name]
		if !ok {
			return nil, tensorMeta{}, false, nil
		}
		var meta tensorMeta
		if err := json.Unmarshal(encoded, &meta); err != nil {
			return nil, meta, true, fmt.Errorf("parse tensor %s: %w", name, err)
		}
		start, end := base+meta.Offsets[0], base+meta.Offsets[1]
		if start < base || end < start || end > len(raw) {
			return nil, meta, true, fmt.Errorf("tensor %s exceeds bundled model", name)
		}
		return raw[start:end], meta, true, nil
	}

	embeddings, meta, ok, err := tensor("embeddings")
	if err != nil {
		return nil, err
	}
	if !ok || len(meta.Shape) != 2 {
		return nil, fmt.Errorf("bundled model has no embedding matrix")
	}
	rows, dim := meta.Shape[0], meta.Shape[1]
	result := &staticTensors{rows: rows, dim: dim}
	switch meta.DType {
	case "F32":
		if len(embeddings) != rows*dim*4 {
			return nil, fmt.Errorf("bundled embedding matrix size is invalid")
		}
		result.matrix = make([]float32, rows*dim)
		for i := range result.matrix {
			result.matrix[i] = math.Float32frombits(binary.LittleEndian.Uint32(embeddings[i*4:]))
		}
	case "I8":
		if len(embeddings) != rows*dim {
			return nil, fmt.Errorf("bundled embedding matrix size is invalid")
		}
		scaleBytes, scaleMeta, found, err := tensor("scales")
		if err != nil {
			return nil, err
		}
		if !found || scaleMeta.DType != "F32" || len(scaleMeta.Shape) != 1 || scaleMeta.Shape[0] != rows || len(scaleBytes) != rows*4 {
			return nil, fmt.Errorf("bundled model has invalid int8 scales")
		}
		result.quant = embeddings
		result.scales = make([]float32, rows)
		for i := range result.scales {
			result.scales[i] = math.Float32frombits(binary.LittleEndian.Uint32(scaleBytes[i*4:]))
		}
	default:
		return nil, fmt.Errorf("bundled embedding dtype %q is not supported", meta.DType)
	}
	if weightBytes, weightMeta, found, err := tensor("weights"); err != nil {
		return nil, err
	} else if found {
		if weightMeta.DType != "F64" || len(weightMeta.Shape) != 1 || weightMeta.Shape[0] != rows || len(weightBytes) != rows*8 {
			return nil, fmt.Errorf("bundled token weights are invalid")
		}
		result.weights = make([]float64, rows)
		for i := range result.weights {
			result.weights[i] = math.Float64frombits(binary.LittleEndian.Uint64(weightBytes[i*8:]))
		}
	}
	if mappingBytes, mappingMeta, found, err := tensor("mapping"); err != nil {
		return nil, err
	} else if found {
		if mappingMeta.DType != "I64" || len(mappingMeta.Shape) != 1 || mappingMeta.Shape[0] != rows || len(mappingBytes) != rows*8 {
			return nil, fmt.Errorf("bundled token mapping is invalid")
		}
		mapping := make([]int, rows)
		identity := true
		for i := range mapping {
			value := int(int64(binary.LittleEndian.Uint64(mappingBytes[i*8:])))
			if value < 0 || value >= rows {
				return nil, fmt.Errorf("bundled token mapping row %d is invalid", i)
			}
			mapping[i] = value
			identity = identity && value == i
		}
		if !identity {
			result.mapping = mapping
		}
	}
	return result, nil
}
