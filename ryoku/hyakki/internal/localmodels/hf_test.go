package localmodels

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestCustomSearchPinsRevisionAndLFSHash(t *testing.T) {
	const revision = "abcdef123456789012345678901234567890abcd"
	data := []byte("gguf")
	hash := sha256.Sum256(data)
	sum := hex.EncodeToString(hash[:])
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/models/Owner/Repo":
			fmt.Fprintf(w, `{"sha":%q,"gated":false,"cardData":{"license":"apache-2.0"}}`, revision)
		case "/api/models/Owner/Repo/tree/" + revision:
			fmt.Fprintf(w, `[{"path":"weights/model.gguf","type":"file","size":4,"lfs":{"oid":%q,"size":4}},{"path":"README.md","type":"file","size":10}]`, sum)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	manager := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	manager.client = server.Client()
	manager.hfBase = server.URL
	result, err := manager.Search(context.Background(), "Owner/Repo")
	if err != nil {
		t.Fatal(err)
	}
	if result.Revision != revision || result.Gated || result.License != "apache-2.0" || len(result.Files) != 1 || result.Files[0].SHA256 != sum {
		t.Fatalf("unexpected pinned result: %+v", result)
	}
	if !strings.Contains(Entry{Repo: result.Repo, Revision: result.Revision}.url(server.URL, result.Files[0].Name), "/resolve/"+revision+"/") {
		t.Fatal("file URL was not pinned")
	}
}

func TestCustomSearchReadsRangeMetadataBeforeDownload(t *testing.T) {
	const revision = "abcdef123456789012345678901234567890abcd"
	data := syntheticGGUF(512, "{{ message.content }}", 65_000_000, false)
	hash := sha256.Sum256(data)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/models/SupraLabs/Supra2.5-Pro-Instruct":
			fmt.Fprintf(w, `{"sha":%q,"gated":false}`, revision)
		case "/api/models/SupraLabs/Supra2.5-Pro-Instruct/tree/" + revision:
			fmt.Fprintf(w, `[{"path":"Supra2.5-65M.F16.gguf","type":"file","size":%d,"lfs":{"oid":%q,"size":%d}}]`, len(data), hex.EncodeToString(hash[:]), len(data))
		case "/SupraLabs/Supra2.5-Pro-Instruct/resolve/" + revision + "/Supra2.5-65M.F16.gguf":
			if got := r.Header.Get("Range"); got != "bytes=0-8388607" {
				t.Errorf("missing bounded Range: %q", got)
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", len(data)-1, len(data)))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(data)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	manager := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	manager.client, manager.hfBase = server.Client(), server.URL
	result, err := manager.Search(context.Background(), "SupraLabs/Supra2.5-Pro-Instruct")
	if err != nil || len(result.Files) != 1 {
		t.Fatalf("search = %+v, %v", result, err)
	}
	model := result.Files[0]
	if model.Params != 65_000_000 || model.Context != 512 || model.Tools || !strings.Contains(model.Warning, "no tool calling") {
		t.Fatalf("custom metadata not offered before download: %+v", model)
	}
}

func TestParseHFInputAndRejectOtherHosts(t *testing.T) {
	cases := []struct{ input, repo, file string }{
		{"owner/model", "owner/model", ""},
		{"https://huggingface.co/owner/model", "owner/model", ""},
		{"https://huggingface.co/owner/model/blob/main/weights/model-Q4_K_M.gguf", "owner/model", "weights/model-Q4_K_M.gguf"},
		{"https://huggingface.co/owner/model/resolve/main/model.gguf", "owner/model", "model.gguf"},
	}
	for _, tc := range cases {
		repo, file, err := ParseHFInput(tc.input)
		if err != nil || repo != tc.repo || file != tc.file {
			t.Errorf("ParseHFInput(%q) = %q, %q, %v", tc.input, repo, file, err)
		}
	}
	for _, input := range []string{
		"https://huggingface.co.attacker.test/owner/model",
		"https://huggingface.co/owner/model/blob/main/../bad.gguf",
		"https://huggingface.co/owner/model/resolve/main/README.md",
		"https://huggingface.co/owner/model?token=private",
		"owner/model/../../bad",
	} {
		if _, _, err := ParseHFInput(input); err == nil {
			t.Errorf("accepted unsafe input %q", input)
		}
	}
}

func TestHFSplitSetAndMatchingProjector(t *testing.T) {
	var files []protocol.LocalFile
	for _, name := range []string{
		"models/alpha-Q4_K_M-00001-of-00003.gguf", "models/alpha-Q4_K_M-00002-of-00003.gguf",
		"models/alpha-Q4_K_M-00003-of-00003.gguf", "models/mmproj-alpha-Q4_K_M.gguf",
		"models/mmproj-alpha-F16.gguf",
	} {
		role := protocol.LocalFileModel
		if strings.Contains(name, "mmproj") {
			role = protocol.LocalFileMmproj
		}
		files = append(files, protocol.LocalFile{Name: name, Role: role, Size: 1024, SHA256: strings.Repeat("a", 64)})
	}
	set := matchGGUFFiles(files, files[1].Name)
	if len(set) != 3 || set[0].Name != files[0].Name || matchingMmproj(files, files[0].Name) != files[3].Name {
		t.Fatalf("split set / projector = %+v / %q", set, matchingMmproj(files, files[0].Name))
	}
	if !completeSplitNames([]string{files[0].Name, files[1].Name, files[2].Name}) {
		t.Fatal("complete split rejected")
	}
	if completeSplitNames([]string{files[0].Name, files[2].Name}) {
		t.Fatal("incomplete split accepted")
	}
	if parts := matchGGUFFiles(files[:2], files[0].Name); len(parts) != 0 {
		t.Fatalf("incomplete split offered: %+v", parts)
	}
	ambiguous := []protocol.LocalFile{
		{Name: "weights/mmproj-alpha.gguf", Role: protocol.LocalFileMmproj},
		{Name: "weights/mmproj-beta.gguf", Role: protocol.LocalFileMmproj},
	}
	if got := matchingMmproj(ambiguous, "weights/model-Q4_K_M.gguf"); got != "" {
		t.Fatalf("unrelated projectors were paired: %q", got)
	}
}

func TestHFRankingChoosesBestQuantThatFitsSimulatedGPU(t *testing.T) {
	result := protocol.LocalSearchResult{Files: []protocol.LocalFile{
		{Name: "model-F16.gguf", Role: protocol.LocalFileModel, Size: 12 << 30, Params: 7_000_000_000, Context: 32768, Tools: true},
		{Name: "model-Q4_K_M.gguf", Role: protocol.LocalFileModel, Size: 4 << 30, Params: 7_000_000_000, Context: 32768, Tools: true},
		{Name: "model-Q3_K_M.gguf", Role: protocol.LocalFileModel, Size: 2 << 30, Params: 7_000_000_000, Context: 32768, Tools: true},
	}}
	h := hardwareProfile{RAMMiB: 24576, AvailableMiB: 16384, Cores: 8,
		Devices: []hardwareDevice{{LocalDevice: protocol.LocalDevice{ID: "gpu0", Name: "Simulated GPU", FreeMiB: 7168}, Kind: "discrete"}}}
	rankHFFiles(&result, h, nil)
	if result.Recommended != "model-Q4_K_M.gguf" || result.Path != result.Recommended ||
		result.Files[0].Name != result.Recommended || !result.Files[0].Recommended ||
		result.Files[0].Parts != 1 || result.Files[0].Quant != "Q4_K_M" ||
		result.Files[0].Plan.Device != "gpu0" {
		t.Fatalf("wrong quant ranking / preview: %+v", result)
	}
}

func TestQuantFloorRejectsSubThreeBitVariants(t *testing.T) {
	for _, name := range []string{"model-UD-IQ1_M.gguf", "model-IQ3_XXS.gguf", "model-Q3_K_S.gguf"} {
		if _, quality := quantAndQuality(name); quality >= 40 {
			t.Errorf("%s crosses the minimum quality floor", name)
		}
	}
	for _, name := range []string{"model-IQ3_XS.gguf", "model-Q3_K_M.gguf", "model-Q4_K_M.gguf"} {
		if _, quality := quantAndQuality(name); quality < 40 {
			t.Errorf("%s was rejected at or above the minimum quality floor", name)
		}
	}
}

func TestHFURLSelectionUsesAPIMetadataAndCompleteSplitSet(t *testing.T) {
	const revision = "abcdef123456789012345678901234567890abcd"
	var headerRequests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/models/Owner/Repo":
			fmt.Fprintf(w, `{"sha":%q,"gated":false}`, revision)
		case "/api/models/Owner/Repo/tree/" + revision:
			fmt.Fprint(w, `[{"path":"weights/model-Q4_K_M-00001-of-00002.gguf","type":"file","size":1000000000,"lfs":{"oid":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","size":1000000000},"gguf":{"metadata":{"general.architecture":"llama","general.parameter_count":2000000000,"llama.context_length":32768,"tokenizer.chat_template":"tools tool_call"}}},{"path":"weights/model-Q4_K_M-00002-of-00002.gguf","type":"file","size":1000000000,"lfs":{"oid":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","size":1000000000}},{"path":"weights/mmproj-model-Q4_K_M.gguf","type":"file","size":300000000,"lfs":{"oid":"cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc","size":300000000}}]`)
		default:
			headerRequests++
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	manager := New(func() protocol.Settings { return protocol.Settings{} }, nil)
	manager.client, manager.hfBase = server.Client(), server.URL
	result, err := manager.Search(context.Background(), "https://huggingface.co/Owner/Repo/blob/main/weights/model-Q4_K_M-00002-of-00002.gguf")
	if err != nil {
		t.Fatal(err)
	}
	var model protocol.LocalFile
	for _, file := range result.Files {
		if file.Name == result.Path {
			model = file
		}
	}
	if result.Repo != "Owner/Repo" || result.Path != "weights/model-Q4_K_M-00001-of-00002.gguf" ||
		result.Mmproj != "weights/mmproj-model-Q4_K_M.gguf" || model.Params != 2_000_000_000 ||
		model.Context != 32768 || !model.Tools || model.Parts != 2 || headerRequests != 0 {
		t.Fatalf("split URL/API metadata = %+v, header requests=%d", result, headerRequests)
	}
}
