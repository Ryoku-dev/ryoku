package rank

import (
	"encoding/binary"
	"math"
)

// Quantize stores a vector as a little-endian float32 scale followed by one
// int8 per dimension.
func Quantize(vector []float32) []byte {
	var maxValue float32
	for _, value := range vector {
		if value < 0 {
			value = -value
		}
		if value > maxValue {
			maxValue = value
		}
	}
	scale := maxValue / 127
	if scale == 0 {
		scale = 1
	}
	out := make([]byte, 4+len(vector))
	binary.LittleEndian.PutUint32(out, math.Float32bits(scale))
	for i, value := range vector {
		q := int(math.Round(float64(value / scale)))
		if q > 127 {
			q = 127
		}
		if q < -127 {
			q = -127
		}
		out[4+i] = byte(int8(q))
	}
	return out
}

// Cosine compares a unit query vector with a quantized one; a vector of
// another size scores 0.
func Cosine(query []float32, raw []byte) float64 {
	if len(raw) != len(query)+4 {
		return 0
	}
	scale := math.Float32frombits(binary.LittleEndian.Uint32(raw))
	var dot, norm float64
	for i, value := range query {
		v := float64(int8(raw[4+i])) * float64(scale)
		dot += float64(value) * v
		norm += v * v
	}
	if norm == 0 {
		return 0
	}
	return dot / math.Sqrt(norm)
}

// Dequantize gives a quantized vector back as floats, scaled to unit length
// so Cosine against another quantized vector is a true cosine.
func Dequantize(raw []byte) []float32 {
	if len(raw) < 4 {
		return nil
	}
	scale := math.Float32frombits(binary.LittleEndian.Uint32(raw))
	out := make([]float32, len(raw)-4)
	var norm float64
	for i := range out {
		v := float64(int8(raw[4+i])) * float64(scale)
		out[i] = float32(v)
		norm += v * v
	}
	if norm == 0 {
		return out
	}
	norm = math.Sqrt(norm)
	for i := range out {
		out[i] = float32(float64(out[i]) / norm)
	}
	return out
}
