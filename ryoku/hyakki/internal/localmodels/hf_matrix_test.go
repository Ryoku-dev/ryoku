package localmodels

import (
	"fmt"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestRealRepositoryShapesAcrossHardware(t *testing.T) {
	const gib = int64(1 << 30)
	profiles := []struct {
		name string
		h    hardwareProfile
		want [4]string
	}{
		{"hybrid8", hardwareProfile{RAMMiB: 43000, AvailableMiB: 34000, Devices: []hardwareDevice{
			{LocalDevice: protocol.LocalDevice{ID: "card", FreeMiB: 7600}, Kind: protocol.LocalDeviceDiscrete},
			{LocalDevice: protocol.LocalDevice{ID: "igpu", FreeMiB: 22000}, Kind: protocol.LocalDeviceIntegrated, Shared: true},
		}}, [4]string{"Q5_K_M.gguf", "Q4_K_M.gguf", "Supra.F16.gguf", "coder-Q4_K_M-00001-of-00002.gguf"}},
		{"shared16", hardwareProfile{RAMMiB: 16384, AvailableMiB: 14000, Devices: []hardwareDevice{
			{LocalDevice: protocol.LocalDevice{ID: "igpu", FreeMiB: 11000}, Kind: protocol.LocalDeviceIntegrated, Shared: true},
		}}, [4]string{"Q8_0.gguf", "Q4_K_M.gguf", "Supra.F16.gguf", "coder-Q4_K_M-00001-of-00002.gguf"}},
		{"cpu8", hardwareProfile{RAMMiB: 8192, AvailableMiB: 7000}, [4]string{"Q5_K_M.gguf", "Q4_K_M.gguf", "Supra.F16.gguf", "coder-Q4_K_M-00001-of-00002.gguf"}},
		{"card24", hardwareProfile{RAMMiB: 65536, AvailableMiB: 54000, Devices: []hardwareDevice{
			{LocalDevice: protocol.LocalDevice{ID: "card", FreeMiB: 23500}, Kind: protocol.LocalDeviceDiscrete},
		}}, [4]string{"Q8_0.gguf", "Q8_0.gguf", "Supra.F16.gguf", "coder-Q4_K_M-00001-of-00002.gguf"}},
	}
	qwen4 := modelFiles("", 4e9, 32768, map[string]int64{"Q8_0": 4400, "Q5_K_M": 3100, "Q4_K_M": 2600, "IQ3_XS": 1900})
	qwen14 := modelFiles("", 14e9, 32768, map[string]int64{"Q8_0": 15200, "Q4_K_M": 8900, "IQ3_XS": 6600, "UD-IQ1_M": 3200})
	supra := []protocol.LocalFile{{Name: "Supra.F16.gguf", Role: protocol.LocalFileModel, Size: 130 << 20, Params: 65e6, Context: 512}}
	coder := modelFiles("coder-", 32e9, 32768, map[string]int64{"Q8_0": 34000, "Q4_K_M": 20100, "IQ3_XS": 14700})
	for _, name := range []string{"Q8_0", "Q4_K_M"} {
		base := fmt.Sprintf("coder-%s.gguf", name)
		for i := range coder {
			if coder[i].Name != base {
				continue
			}
			size := coder[i].Size
			coder[i].Name = fmt.Sprintf("coder-%s-00001-of-00002.gguf", name)
			coder[i].Size = size / 2
			coder = append(coder, protocol.LocalFile{Name: fmt.Sprintf("coder-%s-00002-of-00002.gguf", name), Role: protocol.LocalFileModel, Size: size - size/2})
			break
		}
	}
	for i := range 22 {
		coder = append(coder, protocol.LocalFile{Name: fmt.Sprintf("mmproj-%02d.gguf", i), Role: protocol.LocalFileMmproj, Size: 1 << 20})
	}
	coder = append(coder, protocol.LocalFile{Name: "imatrix.dat", Role: protocol.LocalFileModel, Size: 1 << 20})
	if len(coder) != 28 {
		t.Fatalf("coder repository fixture has %d files, want 28", len(coder))
	}
	cases := []struct {
		name  string
		files []protocol.LocalFile
		meta  ggufMetadata
	}{
		{"qwen4", qwen4, ggufMetadata{Params: 4e9, Context: 32768, Blocks: 36, Embedding: 4096, Heads: 32, KVHeads: 8}},
		{"qwen14", qwen14, ggufMetadata{Params: 14e9, Context: 32768, Blocks: 40, Embedding: 5120, Heads: 40, KVHeads: 8}},
		{"supra", supra, ggufMetadata{Params: 65e6, Context: 512, Blocks: 12}},
		{"coder32", coder, ggufMetadata{Params: 32e9, Context: 32768, Blocks: 64, Embedding: 5120, Heads: 40, KVHeads: 8}},
	}
	for _, hw := range profiles {
		for index, tc := range cases {
			t.Run(hw.name+"/"+tc.name, func(t *testing.T) {
				result := protocol.LocalSearchResult{Files: append([]protocol.LocalFile(nil), tc.files...)}
				headers := map[string]ggufMetadata{}
				for _, file := range result.Files {
					if file.Role == protocol.LocalFileModel && strings.HasSuffix(file.Name, ".gguf") {
						headers[file.Name] = tc.meta
					}
				}
				rankHFFiles(&result, hw.h, headers)
				if result.Recommended != hw.want[index] {
					t.Fatalf("recommendation %q, want %q; files=%+v", result.Recommended, hw.want[index], result.Files)
				}
				var chosen protocol.LocalFile
				for _, file := range result.Files {
					if file.Name == result.Recommended {
						chosen = file
					}
				}
				if index == 0 && hw.name == "hybrid8" && (chosen.Plan.Context != 32768 || chosen.Verdict != protocol.LocalVerdictCard) {
					t.Fatalf("context before quant: %+v", chosen)
				}
				if index == 1 && hw.name == "hybrid8" && (chosen.Plan.Context != 32768 || chosen.Verdict != protocol.LocalVerdictSplit || chosen.Plan.GPULayers == 0) {
					t.Fatalf("quality floor / split: %+v", chosen)
				}
				if index == 2 && chosen.Verdict != protocol.LocalVerdictUnsuitable {
					t.Fatalf("tiny chat model marked suitable: %+v", chosen)
				}
				if index == 3 && hw.name == "cpu8" && (chosen.Verdict != protocol.LocalVerdictTooBig || !strings.Contains(chosen.Plan.Reason, "Needs about") || !strings.Contains(chosen.Plan.Reason, "has")) {
					t.Fatalf("too-big verdict lacks budget: %+v", chosen)
				}
			})
		}
	}
}

func modelFiles(prefix string, params int64, context int, sizes map[string]int64) []protocol.LocalFile {
	files := make([]protocol.LocalFile, 0, len(sizes))
	for quant, size := range sizes {
		files = append(files, protocol.LocalFile{Name: prefix + quant + ".gguf", Role: protocol.LocalFileModel, Size: size << 20, Params: params, Context: context, Tools: true})
	}
	return files
}
