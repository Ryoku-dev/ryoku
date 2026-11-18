package pos

import (
	"encoding/binary"
	"fmt"
	"math"
)

// Burn's NamedMpkBytesRecorder uses named MessagePack maps. TensorData stores
// row-major F32 values as little-endian raw bytes, not MessagePack floats.
type recordReader struct {
	data []byte
	at   int
}

func (r *recordReader) take(n int) ([]byte, error) {
	if n < 0 || n > len(r.data)-r.at {
		return nil, fmt.Errorf("truncated Burn record at %d", r.at)
	}
	b := r.data[r.at : r.at+n]
	r.at += n
	return b, nil
}
func (r *recordReader) size(n int) (int, error) {
	b, e := r.take(n)
	if e != nil {
		return 0, e
	}
	switch n {
	case 1:
		return int(b[0]), nil
	case 2:
		return int(binary.BigEndian.Uint16(b)), nil
	case 4:
		return int(binary.BigEndian.Uint32(b)), nil
	}
	return 0, fmt.Errorf("invalid integer width")
}
func (r *recordReader) value() (any, error) {
	b, e := r.take(1)
	if e != nil {
		return nil, e
	}
	c := b[0]
	if c <= 0x7f {
		return int(c), nil
	}
	if c >= 0xa0 && c <= 0xbf {
		return r.str(int(c & 31))
	}
	if c >= 0x80 && c <= 0x8f {
		return r.mapping(int(c & 15))
	}
	if c >= 0x90 && c <= 0x9f {
		return r.array(int(c & 15))
	}
	switch c {
	case 0xc0:
		return nil, nil
	case 0xc2:
		return false, nil
	case 0xc3:
		return true, nil
	case 0xcc, 0xcd, 0xce:
		n, e := r.size(1 << uint(c-0xcc))
		return n, e
	case 0xc4, 0xc5, 0xc6:
		n, e := r.size(1 << uint(c-0xc4))
		if e != nil {
			return nil, e
		}
		return r.take(n)
	case 0xd9, 0xda, 0xdb:
		n, e := r.size(1 << uint(c-0xd9))
		if e != nil {
			return nil, e
		}
		return r.str(n)
	case 0xdc, 0xdd:
		n, e := r.size(2 << uint(c-0xdc))
		if e != nil {
			return nil, e
		}
		return r.array(n)
	case 0xde, 0xdf:
		n, e := r.size(2 << uint(c-0xde))
		if e != nil {
			return nil, e
		}
		return r.mapping(n)
	}
	return nil, fmt.Errorf("unsupported Burn record marker %#x", c)
}
func (r *recordReader) str(n int) (any, error) { b, e := r.take(n); return string(b), e }
func (r *recordReader) array(n int) (any, error) {
	if n > len(r.data)-r.at {
		return nil, fmt.Errorf("oversized Burn array")
	}
	a := make([]any, n)
	for i := range a {
		v, e := r.value()
		if e != nil {
			return nil, e
		}
		a[i] = v
	}
	return a, nil
}
func (r *recordReader) mapping(n int) (any, error) {
	if n > (len(r.data)-r.at)/2 {
		return nil, fmt.Errorf("oversized Burn map")
	}
	m := make(map[string]any, n)
	for range n {
		k, e := r.value()
		if e != nil {
			return nil, e
		}
		s, ok := k.(string)
		if !ok {
			return nil, fmt.Errorf("non-string Burn map key")
		}
		v, e := r.value()
		if e != nil {
			return nil, e
		}
		m[s] = v
	}
	return m, nil
}
func recordAt(v any, path ...string) any {
	for _, p := range path {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[p]
	}
	return v
}
func recordTensor(v any, shape ...int) ([]float32, error) {
	if recordAt(v, "param", "dtype") != "F32" {
		return nil, fmt.Errorf("Burn tensor is not F32")
	}
	dims, ok := recordAt(v, "param", "shape").([]any)
	if !ok || len(dims) != len(shape) {
		return nil, fmt.Errorf("unexpected Burn tensor rank")
	}
	n := 1
	for i, d := range shape {
		if dims[i] != d {
			return nil, fmt.Errorf("unexpected Burn tensor shape %v, expected %v", dims, shape)
		}
		n *= d
	}
	b, ok := recordAt(v, "param", "bytes").([]byte)
	if !ok || len(b) != n*4 {
		return nil, fmt.Errorf("invalid Burn tensor byte length")
	}
	out := make([]float32, n)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return out, nil
}
