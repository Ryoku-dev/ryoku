package localmodels

import (
	"bytes"
	encodingbin "encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/protocol"
)

func syntheticGGUF(context uint32, template string, parameterCount uint64, tensor bool) []byte {
	var b bytes.Buffer
	b.WriteString("GGUF")
	put32 := func(n uint32) { _ = encodingbin.Write(&b, encodingbin.LittleEndian, n) }
	put64 := func(n uint64) { _ = encodingbin.Write(&b, encodingbin.LittleEndian, n) }
	text := func(s string) { put64(uint64(len(s))); b.WriteString(s) }
	put32(3)
	if tensor {
		put64(1)
	} else {
		put64(0)
	}
	count := uint64(3)
	if parameterCount > 0 {
		count++
	}
	put64(count)
	text("general.architecture")
	put32(8)
	text("llama")
	text("llama.context_length")
	put32(4)
	put32(context)
	text("tokenizer.chat_template")
	put32(8)
	text(template)
	if parameterCount > 0 {
		text("general.parameter_count")
		put32(10)
		put64(parameterCount)
	}
	if tensor {
		text("output.weight")
		put32(2)
		put64(13)
		put64(5)
		put32(0)
		put64(0)
	}
	return b.Bytes()
}

func TestGGUFHeaderReadsCapabilitiesAndTensorDimensions(t *testing.T) {
	without, err := parseGGUF(syntheticGGUF(512, "{% for message in messages %}{{ message.content }}{% endfor %}", 65_000_000, false))
	if err != nil || without.Context != 512 || without.Params != 65_000_000 || without.Tools {
		t.Fatalf("small model metadata = %+v, %v", without, err)
	}
	if text := modelWarning(without); !strings.Contains(text, "512") || !strings.Contains(text, "no tool calling") || !strings.Contains(text, "0.5B") {
		t.Fatalf("missing small-model warning: %q", text)
	}
	with, err := parseGGUF(syntheticGGUF(8192, "{% if tools %}{% for call in message.tool_calls %}{{ call.function.name }}{% endfor %}{% endif %}", 0, true))
	if err != nil || with.Context != 8192 || with.Params != 65 || !with.Tools || modelWarning(with) == "" {
		t.Fatalf("tensor parameter count = %+v, %v", with, err)
	}
	if _, err := parseGGUF(syntheticGGUF(512, "", 0, true)[:35]); err == nil {
		t.Fatal("a truncated GGUF header was accepted")
	}
}

func TestGGUFArchitectureShapeIndependentOfMetadataOrder(t *testing.T) {
	var b bytes.Buffer
	b.WriteString("GGUF")
	put32 := func(n uint32) { _ = encodingbin.Write(&b, encodingbin.LittleEndian, n) }
	put64 := func(n uint64) { _ = encodingbin.Write(&b, encodingbin.LittleEndian, n) }
	text := func(s string) { put64(uint64(len(s))); b.WriteString(s) }
	put32(3)
	put64(0)
	put64(10)
	for _, item := range []struct {
		key string
		n   uint32
	}{
		{"llama.context_length", 32768}, {"llama.block_count", 32},
		{"llama.embedding_length", 4096}, {"llama.attention.head_count", 32},
		{"llama.attention.head_count_kv", 8}, {"llama.attention.key_length", 128},
		{"llama.attention.value_length", 128}, {"llama.attention.sliding_window", 4096},
	} {
		text(item.key)
		put32(4)
		put32(item.n)
	}
	text("general.architecture")
	put32(8)
	text("llama")
	text("general.parameter_count")
	put32(10)
	put64(7_000_000_000)
	meta, err := parseGGUF(b.Bytes())
	if err != nil || meta.Architecture != "llama" || meta.Context != 32768 ||
		meta.Blocks != 32 || meta.Embedding != 4096 || meta.Heads != 32 ||
		meta.KVHeads != 8 || meta.KeyLength != 128 || meta.ValueLength != 128 || meta.SlidingWindow != 4096 {
		t.Fatalf("architecture shape: %+v, %v", meta, err)
	}
}

func TestCustomPresetCapsRequestedContextAtTrainedContext(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	data := syntheticGGUF(512, "", 65_000_000, false)
	e := Entry{ID: "hf-test", Repo: "owner/model", Revision: strings.Repeat("a", 40), Context: 32768, Tools: true,
		Files: []File{{Name: "model.gguf", Role: protocol.LocalFileModel, Size: int64(len(data)), SHA256: strings.Repeat("a", 64)}}}
	if err := os.MkdirAll(repoDir(e.Repo), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoDir(e.Repo), "model.gguf"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	meta, err := localGGUF(e)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeManifest(e, manifest{ID: e.ID, Repo: e.Repo, Revision: e.Revision, Files: e.Files, Verified: time.Now(), Header: &meta}); err != nil {
		t.Fatal(err)
	}
	manager := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	ini := renderPresets(manager.makePresets([]Entry{e}, nil))
	if !strings.Contains(ini, "c = 512") || strings.Contains(ini, "c = 32768") {
		t.Fatalf("preset ignores trained context:\n%s", ini)
	}
}
