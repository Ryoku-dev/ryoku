package localmodels

import (
	encodingbin "encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"

	i18n "ryoku-i18n"
)

// ggufMetadata reads the model's header without reading its weights. A
// truncated header is unknown, never evidence that the model supports tools.
type ggufMetadata struct {
	Params        int64  `json:"params,omitempty"`
	Context       int    `json:"context,omitempty"`
	Tools         bool   `json:"tools"`
	Architecture  string `json:"architecture,omitempty"`
	Blocks        int    `json:"blocks,omitempty"`
	Embedding     int    `json:"embedding,omitempty"`
	Heads         int    `json:"heads,omitempty"`
	KVHeads       int    `json:"kvHeads,omitempty"`
	KeyLength     int    `json:"keyLength,omitempty"`
	ValueLength   int    `json:"valueLength,omitempty"`
	SlidingWindow int    `json:"slidingWindow,omitempty"`
	ModelBytes    int64  `json:"modelBytes,omitempty"`
	MmprojBytes   int64  `json:"mmprojBytes,omitempty"`
}

const ggufHeaderLimit = 8 << 20

var errGGUFHeader = errors.New("incomplete or invalid GGUF header")

type ggufCursor struct {
	data []byte
	at   uint64
}

func (c *ggufCursor) take(n uint64) ([]byte, error) {
	if n > uint64(len(c.data))-c.at {
		return nil, errGGUFHeader
	}
	b := c.data[c.at : c.at+n]
	c.at += n
	return b, nil
}

func (c *ggufCursor) u32() (uint32, error) {
	b, err := c.take(4)
	if err != nil {
		return 0, err
	}
	return encodingbin.LittleEndian.Uint32(b), nil
}

func (c *ggufCursor) u64() (uint64, error) {
	b, err := c.take(8)
	if err != nil {
		return 0, err
	}
	return encodingbin.LittleEndian.Uint64(b), nil
}

func (c *ggufCursor) text() (string, error) {
	n, err := c.u64()
	if err != nil {
		return "", err
	}
	b, err := c.take(n)
	return string(b), err
}

// value consumes one GGUF metadata value. An array can itself hold strings;
// never allocate it, since only the few scalar keys matter here.
func (c *ggufCursor) value(kind uint32, depth int) (uint64, string, error) {
	if depth > 2 {
		return 0, "", errGGUFHeader
	}
	switch kind {
	case 0, 1, 7: // u8, i8, bool
		b, err := c.take(1)
		if err != nil {
			return 0, "", err
		}
		return uint64(b[0]), "", nil
	case 2, 3: // u16, i16
		b, err := c.take(2)
		if err != nil {
			return 0, "", err
		}
		return uint64(encodingbin.LittleEndian.Uint16(b)), "", nil
	case 4, 5, 6: // u32, i32, f32
		n, err := c.u32()
		return uint64(n), "", err
	case 8:
		text, err := c.text()
		return 0, text, err
	case 9:
		item, err := c.u32()
		if err != nil {
			return 0, "", err
		}
		count, err := c.u64()
		if err != nil || count > uint64(len(c.data)) {
			return 0, "", errGGUFHeader
		}
		for i := uint64(0); i < count; i++ {
			if _, _, err = c.value(item, depth+1); err != nil {
				return 0, "", err
			}
		}
		return 0, "", nil
	case 10, 11, 12: // u64, i64, f64
		n, err := c.u64()
		return n, "", err
	default:
		return 0, "", errGGUFHeader
	}
}

func parseGGUF(data []byte) (ggufMetadata, error) {
	var out ggufMetadata
	if len(data) < 24 || string(data[:4]) != "GGUF" {
		return out, errGGUFHeader
	}
	c := ggufCursor{data: data, at: 4}
	version, err := c.u32()
	if err != nil || version < 2 || version > 3 {
		return out, errGGUFHeader
	}
	tensors, err := c.u64()
	if err != nil {
		return out, err
	}
	count, err := c.u64()
	if err != nil || count > 100000 || tensors > 1000000 {
		return out, errGGUFHeader
	}
	var arch, template string
	values := make(map[string]int, 10)
	for i := uint64(0); i < count; i++ {
		key, err := c.text()
		if err != nil {
			return out, err
		}
		kind, err := c.u32()
		if err != nil {
			return out, err
		}
		n, text, err := c.value(kind, 0)
		if err != nil {
			return out, err
		}
		switch {
		case key == "general.architecture":
			arch = text
		case key == "general.parameter_count" && n > 0 && n <= math.MaxInt64:
			out.Params = int64(n)
		case key == "tokenizer.chat_template":
			template = text
		case n <= math.MaxInt32 && (strings.HasSuffix(key, ".context_length") ||
			strings.HasSuffix(key, ".block_count") || strings.HasSuffix(key, ".embedding_length") ||
			strings.HasSuffix(key, ".attention.head_count") || strings.HasSuffix(key, ".attention.head_count_kv") ||
			strings.HasSuffix(key, ".attention.key_length") || strings.HasSuffix(key, ".attention.value_length") ||
			strings.HasSuffix(key, ".attention.sliding_window")):
			values[key] = int(n)
		}
	}
	out.Architecture = arch
	out.Context = values[arch+".context_length"]
	out.Blocks = values[arch+".block_count"]
	out.Embedding = values[arch+".embedding_length"]
	out.Heads = values[arch+".attention.head_count"]
	out.KVHeads = values[arch+".attention.head_count_kv"]
	out.KeyLength = values[arch+".attention.key_length"]
	out.ValueLength = values[arch+".attention.value_length"]
	out.SlidingWindow = values[arch+".attention.sliding_window"]
	out.Tools = strings.Contains(template, "tools") && (strings.Contains(template, "tool_call") || strings.Contains(template, "function"))
	if out.Params != 0 {
		return out, nil
	}
	// Tensor extents count parameters even when weights use a quantized type.
	// GGUF tensor records follow the key/value directory; no weight bytes needed.
	var total uint64
	for i := uint64(0); i < tensors; i++ {
		if _, err := c.text(); err != nil {
			return out, err
		}
		dims, err := c.u32()
		if err != nil || dims > 4 {
			return out, errGGUFHeader
		}
		product := uint64(1)
		for d := uint32(0); d < dims; d++ {
			n, err := c.u64()
			if err != nil || n > math.MaxInt64 || product > math.MaxInt64/max(n, 1) {
				return out, errGGUFHeader
			}
			product *= n
		}
		if _, err := c.take(12); err != nil || total > math.MaxInt64-product {
			return out, errGGUFHeader
		}
		total += product
	}
	out.Params = int64(total)
	return out, nil
}

func localGGUF(e Entry) (ggufMetadata, error) {
	model, ok := e.model()
	if !ok {
		return ggufMetadata{}, errGGUFHeader
	}
	f, err := os.Open(filePath(e, model.Name))
	if err != nil {
		return ggufMetadata{}, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, ggufHeaderLimit))
	if err != nil {
		return ggufMetadata{}, err
	}
	return parseGGUF(data)
}

func modelWarning(meta ggufMetadata) string {
	var reasons []string
	if meta.Context == 0 {
		reasons = append(reasons, i18n.T("its trained context could not be read"))
	} else if meta.Context < 8192 {
		reasons = append(reasons, i18n.Tf("was trained on %d tokens of context", meta.Context))
	}
	if !meta.Tools {
		reasons = append(reasons, i18n.T("has no tool calling"))
	}
	if meta.Params == 0 {
		reasons = append(reasons, i18n.T("its parameter count could not be read"))
	} else if meta.Params < 500_000_000 {
		reasons = append(reasons, i18n.Tf("has only %s parameters", formatParams(meta.Params)))
	}
	if len(reasons) == 0 {
		return ""
	}
	return i18n.Tf("This model %s. It may answer ordinary questions, but a model with tool calling, at least 8K context and at least 0.5B parameters is recommended for agent tasks.", strings.Join(reasons, i18n.T(" and ")))
}

func formatParams(n int64) string { return fmt.Sprintf("%.2fB", float64(n)/1e9) }
