package localmodels

import (
	"fmt"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

const mib = int64(1 << 20)

// planModel makes a conservative prediction; llama.cpp fit remains the final allocator.
func planModel(e Entry, meta ggufMetadata, h hardwareProfile, choice protocol.LocalModelOverride) protocol.LocalPlan {
	p := protocol.LocalPlan{Kind: "cpu", Device: deviceNone, DeviceName: i18n.T("Processor"), Layers: meta.Blocks, KVType: "f16"}
	if p.Layers == 0 {
		p.Layers = estimatedBlocks(meta.Params)
	}
	weights := meta.ModelBytes
	if weights <= 0 {
		for _, file := range e.Files {
			if file.Role == protocol.LocalFileModel {
				weights += file.Size
			}
		}
	}
	if weights <= 0 {
		weights = e.size()
	}
	projector := meta.MmprojBytes
	if projector == 0 {
		for _, file := range e.Files {
			if file.Role == protocol.LocalFileMmproj {
				projector += file.Size
			}
		}
	}
	weights += projector
	kvAt := func(n int) int64 { return kvBytesForContext(meta, n) }
	trained := meta.Context
	if trained == 0 {
		trained = e.Context
	}
	if trained <= 0 {
		trained = 8192
	}
	minimum := min(8192, trained)
	target := min(32768, trained)
	headroom := func(free int64) int64 { return max(768*mib, free/10) }
	reserve := int64(max(1024, min(4096, h.AvailableMiB/8))) * mib
	hostRoom := int64(h.AvailableMiB)*mib - reserve
	if h.RAMMiB == 0 || h.AvailableMiB == 0 {
		hostRoom = 0
	}
	cpuRoom := max(0, hostRoom)
	deviceRoom := func(d hardwareDevice) int64 {
		room := int64(d.FreeMiB)*mib - headroom(int64(d.FreeMiB)*mib)
		if d.Shared {
			room = min(room, cpuRoom)
		}
		return max(0, room)
	}
	// Budget the target context before selecting a device or an offload ratio.
	overhead := int64(256) * mib
	cacheAt := func(n int, quant string) int64 {
		size := kvAt(n)
		if quant == protocol.LocalKVQ8 {
			size = (size*34 + 63) / 64
		}
		return size
	}
	var gpuKVRoom int64
	for i := range h.Devices {
		d := &h.Devices[i]
		if d.Kind != protocol.LocalDeviceDiscrete || choice.Device == "cpu" || choice.Device != "" && choice.Device != d.ID {
			continue
		}
		room := deviceRoom(*d)
		quant := choice.KVType
		if quant == "" {
			quant = protocol.LocalKVF16
		}
		if room < weights+cacheAt(target, quant)+overhead && choice.KVType == "" {
			quant = protocol.LocalKVQ8
		}
		if room >= weights+cacheAt(target, quant)+overhead {
			p.Kind, p.Device, p.DeviceName, p.KVType = d.Kind, d.ID, d.Name, quant
			cpuRoom = room
			break
		}
	}
	if p.Kind == protocol.LocalDeviceCPU && choice.Device != "cpu" {
		var best *hardwareDevice
		var layers int
		var cacheRoom int64
		var kvType string
		for i := range h.Devices {
			d := &h.Devices[i]
			if d.Kind != protocol.LocalDeviceDiscrete || choice.Device != "" && choice.Device != d.ID {
				continue
			}
			room := deviceRoom(*d)
			quant := choice.KVType
			if quant == "" {
				quant = protocol.LocalKVF16
			}
			if room < cacheAt(target, quant)+overhead+1024*mib && choice.KVType == "" {
				quant = protocol.LocalKVQ8
			}
			free := room - cacheAt(target, quant) - overhead
			if free < 1024*mib || weights <= 0 {
				continue
			}
			n := max(1, min(p.Layers-1, int(int64(p.Layers)*free/weights)))
			gpuWeights := weights * int64(n) / int64(max(1, p.Layers))
			if hostRoom < weights-gpuWeights+overhead || best != nil && n <= layers {
				continue
			}
			best, layers, cacheRoom, kvType = d, n, room-gpuWeights-overhead, quant
		}
		if best != nil {
			p.Kind, p.Device, p.DeviceName, p.GPULayers, p.KVType = best.Kind, best.ID, best.Name, layers, kvType
			gpuKVRoom = cacheRoom
			cpuRoom = hostRoom + weights*int64(layers)/int64(max(1, p.Layers))
			p.Reason = i18n.T("The graphics card holds part of the model and its context; the remaining layers run from system memory, which is slower.")
		}
	}
	if p.Kind == protocol.LocalDeviceCPU && choice.Device != "cpu" {
		for i := range h.Devices {
			d := &h.Devices[i]
			if d.Kind != protocol.LocalDeviceIntegrated || choice.Device != "" && choice.Device != d.ID {
				continue
			}
			quant := choice.KVType
			if quant == "" {
				quant = protocol.LocalKVF16
			}
			if deviceRoom(*d) < weights+cacheAt(target, quant)+overhead && choice.KVType == "" {
				quant = protocol.LocalKVQ8
			}
			if deviceRoom(*d) < weights+cacheAt(target, quant)+overhead {
				continue
			}
			p.Kind, p.Device, p.DeviceName, p.KVType = d.Kind, d.ID, d.Name, quant
			cpuRoom = deviceRoom(*d)
			break
		}
	}
	if p.Reason == "" {
		if p.Kind == "cpu" {
			if choice.Device != "" && choice.Device != "cpu" {
				p.Reason = i18n.T("The selected graphics card has too little free memory; using the processor.")
			} else {
				p.Reason = i18n.T("The processor has more usable memory than the graphics cards.")
			}
		} else if p.Kind == "discrete" {
			p.Reason = i18n.T("The discrete graphics card holds the model and its context.")
		} else {
			p.Reason = i18n.T("Shared graphics memory leaves enough memory for the system.")
		}
	}
	if choice.KVType == "q8_0" {
		p.KVType = "q8_0"
	}
	room := cpuRoom - weights - 256*mib
	if gpuKVRoom > 0 {
		room = min(room, gpuKVRoom)
	}
	fits := func(n int, quant string) bool {
		if n < 1 || room <= 0 {
			return false
		}
		bytes := kvAt(n)
		if quant == "q8_0" {
			bytes = (bytes*34 + 63) / 64
		}
		return bytes <= room
	}
	options := []int{32768, 16384, 8192}
	for _, n := range options {
		if n > target {
			continue
		}
		if fits(n, p.KVType) {
			p.ContextOptions = append(p.ContextOptions, n)
		}
	}
	if trained < 8192 && fits(trained, p.KVType) {
		p.ContextOptions = append(p.ContextOptions, trained)
	}
	if len(p.ContextOptions) > 0 {
		p.Context = p.ContextOptions[0]
	}
	if choice.KVType == "" && p.Context < target {
		for _, n := range options {
			if n <= target && n > p.Context && fits(n, "q8_0") {
				p.KVType = "q8_0"
				p.Context = n
				if p.GPULayers == 0 {
					p.Reason = i18n.T("An 8-bit cache makes a longer context fit without reducing model weights.")
				}
				p.ContextOptions = p.ContextOptions[:0]
				for _, option := range options {
					if option <= target && fits(option, p.KVType) {
						p.ContextOptions = append(p.ContextOptions, option)
					}
				}
				break
			}
		}
	}
	if choice.Context > 0 && choice.Context <= trained && fits(choice.Context, p.KVType) {
		p.Context = choice.Context
		p.Reason = i18n.T("Using the model's chosen settings within the available memory.")
	}
	if p.Context == 0 {
		p.Verdict = protocol.LocalVerdictTooBig
		p.Context = minimum
		need := (weights + kvAt(minimum) + overhead + mib - 1) / mib
		available := max(0, hostRoom/mib)
		p.Reason = i18n.Tf("Needs about %.1f GB of usable memory; this computer has %.1f GB available.", float64(need)/1024, float64(available)/1024)
		p.Kind, p.Device, p.DeviceName, p.GPULayers = protocol.LocalDeviceCPU, deviceNone, i18n.T("Processor"), 0
	} else if p.GPULayers > 0 && p.GPULayers < p.Layers {
		p.Verdict = protocol.LocalVerdictSplit
	} else if p.Kind == "cpu" {
		p.Verdict = protocol.LocalVerdictCPU
	} else if p.Kind == "integrated" {
		p.Verdict = protocol.LocalVerdictShared
	} else {
		p.Verdict = protocol.LocalVerdictCard
	}
	if p.Verdict != protocol.LocalVerdictTooBig && (trained < protocol.LocalMinContext || meta.Params > 0 && meta.Params < 500_000_000) {
		p.Verdict = protocol.LocalVerdictUnsuitable
		p.Reason = i18n.T("This model has too little capacity or context for Hyakki's everyday tasks; plain chat is still possible.")
	}
	if p.Kind != "cpu" && p.GPULayers == 0 {
		p.GPULayers = p.Layers
	}
	used := weights + kvAt(p.Context) + 256*mib
	if p.KVType == "q8_0" {
		used = weights + (kvAt(p.Context)*34+63)/64 + 256*mib
	}
	p.EstimatedMiB = int(min(int64(1<<31-1), (used+mib-1)/mib))
	return p
}

func kvBytesPerToken(m ggufMetadata) int64 {
	blocks, heads, kvHeads := m.Blocks, m.Heads, m.KVHeads
	if blocks <= 0 {
		blocks = estimatedBlocks(m.Params)
	}
	if heads <= 0 {
		heads = max(1, m.Embedding/128)
	}
	if kvHeads <= 0 {
		kvHeads = heads
	}
	width := 128
	if m.Embedding > 0 {
		width = max(1, m.Embedding/heads)
	}
	key, value := m.KeyLength, m.ValueLength
	if key <= 0 {
		key = width
	}
	if value <= 0 {
		value = width
	}
	per := int64(blocks) * int64(kvHeads) * int64(key+value) * 2
	if per <= 0 || per > 1<<30 {
		return 256 << 10
	}
	return per
}

func estimatedBlocks(params int64) int {
	switch {
	case params >= 30e9:
		return 64
	case params >= 12e9:
		return 40
	case params >= 5e9:
		return 32
	default:
		return 24
	}
}

func kvBytesForContext(m ggufMetadata, context int) int64 {
	full := kvBytesPerToken(m) * int64(context)
	if m.SlidingWindow <= 0 || m.SlidingWindow >= context {
		return full
	}
	// Gemma 3 uses five sliding layers followed by one full-attention layer.
	// For unknown patterns keep the full estimate instead of under-budgeting VRAM.
	if m.Architecture == "gemma3" && m.Blocks > 0 {
		fullLayers := max(1, m.Blocks/6)
		sliding := m.Blocks - fullLayers
		return (kvBytesPerToken(m) / int64(m.Blocks)) *
			(int64(fullLayers)*int64(context) + int64(sliding)*int64(m.SlidingWindow))
	}
	return full
}

func planPresetArgs(p protocol.LocalPlan, cores int) map[string]string {
	if cores < 1 {
		cores = 1
	}
	gpu := fmt.Sprint(p.GPULayers)
	if p.Kind == "cpu" {
		gpu = "0"
	} else if p.GPULayers >= p.Layers {
		gpu = "all"
	}
	return map[string]string{
		"n-gpu-layers": gpu, "fit": "on", "fit-ctx": fmt.Sprint(p.Context),
		"flash-attn": "auto", "cache-type-k": p.KVType, "cache-type-v": p.KVType,
		"parallel": "1", "threads": fmt.Sprint(cores),
	}
}
