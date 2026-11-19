package localmodels

import (
	"sort"
	"strconv"
	"strings"

	"ryoku-rashin/internal/protocol"
)

// presetModel is one section of the router's presets: the routed id, its
// files and what it runs with.
type presetModel struct {
	ID       string
	Model    string
	Mmproj   string
	Context  int
	Device   string
	Args     map[string]string
	Shards   []string
	PlanArgs map[string]string
	Plan     protocol.LocalPlan
}

// presetReserved are keys Rashin sets itself (or the router overrides); an
// entry's Args cannot change them.
var presetReserved = map[string]bool{
	"model": true, "m": true, "mmproj": true, "mm": true, "c": true, "ctx-size": true,
	"device": true, "dev": true, "host": true, "port": true, "api-key": true, "api-key-file": true,
	"hf-repo": true, "hf": true, "alias": true, "a": true, "models-preset": true, "models-dir": true,
	"log-file": true, "load-on-startup": true, "webui": true, "no-webui": true, "shard": true,
	"n-gpu-layers": true, "fit": true, "fit-ctx": true, "flash-attn": true,
	"parallel": true, "threads": true, "cache-type-k": true, "cache-type-v": true,
}

// renderPresets writes one deterministic router preset per planned model.
func renderPresets(models []presetModel) string {
	var b strings.Builder
	b.WriteString("version = 1\n\n[*]\njinja = true\nfit = on\nflash-attn = auto\nparallel = 1\nstop-timeout = 10\n")
	for _, m := range models {
		if !presetValue(m.ID) || !presetValue(m.Model) || m.Mmproj != "" && !presetValue(m.Mmproj) {
			continue
		}
		b.WriteString("\n[" + m.ID + "]\n")
		b.WriteString("model = " + m.Model + "\n")
		if m.Mmproj != "" {
			b.WriteString("mmproj = " + m.Mmproj + "\n")
		}
		if len(m.Shards) > 0 {
			safe := true
			for _, shard := range m.Shards {
				safe = safe && presetValue(shard) && !strings.Contains(shard, ",")
			}
			if safe {
				b.WriteString("shard = " + strings.Join(m.Shards, ",") + "\n")
			}
		}
		if m.Context > 0 {
			b.WriteString("c = " + strconv.Itoa(m.Context) + "\n")
		}
		if m.Device != "" && presetValue(m.Device) {
			b.WriteString("device = " + m.Device + "\n")
		}
		planKeys := make([]string, 0, len(m.PlanArgs))
		for k := range m.PlanArgs {
			planKeys = append(planKeys, k)
		}
		sort.Strings(planKeys)
		for _, k := range planKeys {
			if presetKey(k) && presetValue(m.PlanArgs[k]) {
				b.WriteString(k + " = " + m.PlanArgs[k] + "\n")
			}
		}
		keys := make([]string, 0, len(m.Args))
		for k := range m.Args {
			if !presetReserved[k] && presetKey(k) && presetValue(m.Args[k]) {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(k + " = " + m.Args[k] + "\n")
		}
	}
	return b.String()
}

// presetValue refuses what would break the INI: line breaks, a leading
// comment character, brackets in a section name.
func presetValue(v string) bool {
	return v != "" && !strings.ContainsAny(v, "\r\n\x00[]") && !strings.HasPrefix(v, ";") && !strings.HasPrefix(v, "#") && strings.TrimSpace(v) == v
}

func presetKey(k string) bool {
	if k == "" {
		return false
	}
	for _, r := range k {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r >= 'A' && r <= 'Z') {
			return false
		}
	}
	return true
}

// presetIDs lists the sections a rendered presets file holds, [*] aside.
func presetIDs(ini string) map[string]bool {
	ids := map[string]bool{}
	for _, line := range strings.Split(ini, "\n") {
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") && line != "[*]" {
			ids[line[1:len(line)-1]] = true
		}
	}
	return ids
}
