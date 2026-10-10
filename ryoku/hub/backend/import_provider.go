package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	wm "ryoku-wm"
)

type providerImportMetadata struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Supported        bool   `json:"supported"`
	DefaultSourceDir string `json:"defaultSourceDir"`
	Entry            string `json:"entry"`
	Active           bool   `json:"active"`
	Found            bool   `json:"found"`
	Path             string `json:"path,omitempty"`
}

type providerImportLoss struct {
	Raw    string `json:"raw"`
	Reason string `json:"reason"`
}

type providerMappedBind struct {
	Norm           string         `json:"norm"`
	Combo          string         `json:"combo"`
	Store          map[string]any `json:"store,omitempty"`
	RebindFrom     string         `json:"rebindFrom,omitempty"`
	Conflict       bool           `json:"conflict,omitempty"`
	ShadowsShipped bool           `json:"shadowsShipped,omitempty"`
}

type providerImportScan struct {
	Provider         string               `json:"provider"`
	Name             string               `json:"name"`
	Path             string               `json:"path"`
	Error            string               `json:"error,omitempty"`
	DefaultSourceDir string               `json:"defaultSourceDir"`
	Items            []scanItem           `json:"items"`
	Conflicts        []scanConflict       `json:"conflicts"`
	Losses           []providerImportLoss `json:"losses"`
	Patch            map[string]any       `json:"patch"`
	Binds            []providerMappedBind `json:"binds"`
	Preserved        string               `json:"preserved,omitempty"`
	PreservePath     string               `json:"preservePath,omitempty"`
	SourceDirs       []string             `json:"sourceDirs,omitempty"`
}

var scanProviderImports = readProviderImports

func providerImportMetadataList() []providerImportMetadata {
	detected := wm.Detect().Name
	var out []providerImportMetadata
	for _, id := range wm.Providers() {
		client := wm.OpenNamed(id)
		if !client.Available() {
			continue
		}
		raw, err := client.Import("metadata")
		if err != nil {
			continue
		}
		var meta providerImportMetadata
		if json.Unmarshal(raw, &meta) != nil || !meta.Supported || meta.ID == "" {
			continue
		}
		meta.Active = meta.ID == detected
		meta.Path, meta.Found = findProviderDefault(meta)
		out = append(out, meta)
	}
	hasActive := false
	for _, meta := range out {
		hasActive = hasActive || meta.Active
	}
	if detected != "" && !hasActive {
		dir := wm.ConfigDir(detected)
		legacy := providerImportMetadata{
			ID: detected, Name: detected, Supported: true,
			DefaultSourceDir: dir, Active: true,
		}
		if app, ok := scanHyprland(filepath.Join(configHome(), dir)); ok {
			legacy.Name, legacy.Found, legacy.Path = app.Name, true, app.Path
		}
		out = append(out, legacy)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Active != out[j].Active {
			return out[i].Active
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func findProviderDefault(meta providerImportMetadata) (string, bool) {
	if meta.DefaultSourceDir == "" || meta.Entry == "" {
		return "", false
	}
	path := filepath.Join(configHome(), meta.DefaultSourceDir)
	st, err := os.Stat(filepath.Join(path, meta.Entry))
	return path, err == nil && !st.IsDir()
}

func detectImportSource() providerImportMetadata {
	list := providerImportMetadataList()
	for _, meta := range list {
		if meta.Active && meta.Found {
			return meta
		}
	}
	for _, meta := range list {
		if meta.Found {
			return meta
		}
	}
	for _, meta := range list {
		if meta.Active {
			return meta
		}
	}
	if len(list) > 0 {
		return list[0]
	}
	return providerImportMetadata{}
}

func readProviderImports(source string) []providerImportScan {
	var out []providerImportScan
	for _, id := range wm.Providers() {
		client := wm.OpenNamed(id)
		if !client.Available() {
			continue
		}
		metaRaw, err := client.Import("metadata")
		if err != nil {
			continue
		}
		var meta providerImportMetadata
		if json.Unmarshal(metaRaw, &meta) != nil || !meta.Supported || meta.ID == "" {
			continue
		}
		entry, found := findProviderSourceEntry(meta, source)
		raw, err := client.Import("scan", source)
		if err != nil {
			if found {
				reason := fmt.Sprintf("Could not parse this provider config: %v", err)
				out = append(out, providerImportScan{
					Provider: meta.ID, Name: meta.Name, Path: entry, Error: reason,
					Items: []scanItem{}, Conflicts: []scanConflict{},
					Losses: []providerImportLoss{},
					Patch:  map[string]any{}, Binds: []providerMappedBind{},
				})
			}
			continue
		}
		var scan providerImportScan
		if json.Unmarshal(raw, &scan) != nil || scan.Provider == "" || scan.Path == "" {
			continue
		}
		if scan.Items == nil {
			scan.Items = []scanItem{}
		}
		if scan.Conflicts == nil {
			scan.Conflicts = []scanConflict{}
		}
		if scan.Losses == nil {
			scan.Losses = []providerImportLoss{}
		}
		out = append(out, scan)
	}
	return out
}

func findProviderSourceEntry(meta providerImportMetadata, source string) (string, bool) {
	if meta.Entry == "" {
		return "", false
	}
	if st, err := os.Stat(source); err == nil && !st.IsDir() && filepath.Base(source) == meta.Entry {
		return source, true
	}
	for _, path := range []string{
		filepath.Join(source, meta.Entry),
		filepath.Join(source, meta.DefaultSourceDir, meta.Entry),
		filepath.Join(source, ".config", meta.DefaultSourceDir, meta.Entry),
	} {
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return path, true
		}
	}
	return "", false
}

func providerScanApp(scan providerImportScan) scanApp {
	summary := fmt.Sprintf("%d mapped items", countIngestable(scan.Items))
	if scan.Error != "" {
		summary = scan.Error
	}
	if len(scan.Losses) > 0 {
		summary += fmt.Sprintf(", %d preserved", len(scan.Losses))
	}
	return scanApp{
		ID: "provider:" + scan.Provider, Name: scan.Name, Present: true,
		Path: filepath.Dir(scan.Path), Tier: "deep", Summary: summary, Error: scan.Error,
		Items: scan.Items, Conflicts: scan.Conflicts, Losses: scan.Losses,
	}
}

func countIngestable(items []scanItem) int {
	n := 0
	for _, item := range items {
		if item.Ingestable {
			n++
		}
	}
	return n
}

func providerExcludedDirs(scans []providerImportScan) map[string]bool {
	excluded := map[string]bool{}
	for _, scan := range scans {
		for _, dir := range scan.SourceDirs {
			if dir = strings.TrimSpace(dir); dir != "" && dir != ".config" {
				excluded[dir] = true
			}
		}
	}
	return excluded
}

func providerAppID(provider string) string { return "provider:" + provider }

func providerPatchFor(scan providerImportScan, choices map[string]conflictChoice) (map[string]any, int, int) {
	patch := cloneJSONMap(scan.Patch)
	type plannedBind struct {
		bind   providerMappedBind
		choice conflictChoice
		combo  string
	}
	var planned []plannedBind
	byNorm := map[string]int{}
	for _, bind := range scan.Binds {
		choice := conflictChoice{mode: "mine"}
		if bind.Conflict {
			choice = choices[bind.Norm]
			if choice.mode == "" {
				choice.mode = "ryoku"
			}
		}
		if choice.mode == "ryoku" {
			continue
		}
		combo := bind.Combo
		if choice.mode == "remap" {
			if strings.TrimSpace(choice.remap) == "" {
				continue
			}
			combo = choice.remap
		}
		row := plannedBind{bind: bind, choice: choice, combo: combo}
		if bind.Conflict && !bind.ShadowsShipped && choice.mode == "mine" {
			if i, ok := byNorm[bind.Norm]; ok {
				planned[i] = row
				continue
			}
		}
		byNorm[bind.Norm] = len(planned)
		planned = append(planned, row)
	}

	binds, unbinds := 0, 0
	for _, row := range planned {
		if row.bind.ShadowsShipped && row.choice.mode == "mine" {
			appendImportPatch(patch, row.combo, "desktop", "unbinds")
			unbinds++
		}
		if row.bind.RebindFrom != "" {
			setImportPatchMap(patch, row.bind.RebindFrom, row.combo, "desktop", "keybindRebinds")
			binds++
			continue
		}
		if len(row.bind.Store) > 0 {
			store := cloneJSONMap(row.bind.Store)
			store["keys"] = row.combo
			appendImportPatch(patch, store, "desktop", "keybinds")
			binds++
		}
	}
	return patch, binds, unbinds
}

func cloneJSONMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	b, _ := json.Marshal(in)
	out := map[string]any{}
	_ = json.Unmarshal(b, &out)
	return out
}

func setImportPatchMap(root map[string]any, key string, value any, path ...string) {
	cur := root
	for _, part := range path {
		next, ok := cur[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[part] = next
		}
		cur = next
	}
	cur[key] = value
}

func appendImportPatch(root map[string]any, value any, path ...string) {
	if len(path) == 0 {
		return
	}
	cur := root
	for _, part := range path[:len(path)-1] {
		next, ok := cur[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[part] = next
		}
		cur = next
	}
	leaf := path[len(path)-1]
	list, _ := cur[leaf].([]any)
	cur[leaf] = append(list, value)
}

func mergeImportPatch(dst, patch map[string]any) {
	for key, value := range patch {
		if incoming, ok := value.(map[string]any); ok {
			current, _ := dst[key].(map[string]any)
			if current == nil {
				current = map[string]any{}
				dst[key] = current
			}
			mergeImportPatch(current, incoming)
			continue
		}
		if incoming, ok := value.([]any); ok {
			current, _ := dst[key].([]any)
			dst[key] = append(current, incoming...)
			continue
		}
		dst[key] = value
	}
}

func importPatchNonEmpty(value any) bool {
	switch v := value.(type) {
	case map[string]any:
		for _, child := range v {
			if importPatchNonEmpty(child) {
				return true
			}
		}
	case []any:
		return len(v) > 0
	case nil:
		return false
	default:
		return true
	}
	return false
}

func providerPreserveTarget(rel string) (string, error) {
	rel = filepath.Clean(strings.TrimSpace(rel))
	if rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("provider import returned unsafe preserve path %q", rel)
	}
	return filepath.Join(configHome(), rel), nil
}

func wmImportProviderLive(provider string) bool {
	detection := wm.Detect()
	return detection.Live && detection.Name == provider
}

func providerGeneratedConfigWrites(provider string) []pendingWrite {
	client := wm.OpenNamed(provider)
	caps, err := client.Caps()
	if err != nil {
		return nil
	}
	base := configHome()
	out := make([]pendingWrite, 0, len(caps.GeneratedFiles))
	for _, rel := range caps.GeneratedFiles {
		path := filepath.Join(base, rel)
		data, _ := os.ReadFile(path)
		out = append(out, pendingWrite{path: path, content: data})
	}
	return out
}

func applyImportedProvider(provider string) error {
	detection := wm.Detect()
	if detection.Name != provider || !detection.Live {
		return nil
	}
	_, err := wm.OpenNamed(provider).Apply(desktopStorePath())
	return err
}
