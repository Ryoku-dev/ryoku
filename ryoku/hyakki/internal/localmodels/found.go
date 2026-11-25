package localmodels

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

const (
	foundMaxEntries  = 20000
	foundMaxDepth    = 8
	foundMaxResults  = 512
	foundMaxManifest = 1 << 20
)

type foundRoot struct {
	path   string
	source string
	ollama bool
	cache  bool
}

func foundRoots(home string, folders []string) []foundRoot {
	cache := os.Getenv("XDG_CACHE_HOME")
	if !filepath.IsAbs(cache) {
		cache = filepath.Join(home, ".cache")
	}
	roots := []foundRoot{
		{path: filepath.Join(cache, "huggingface", "hub"), source: protocol.LocalFoundHuggingFace, cache: true},
		{path: filepath.Join(cache, "lm-studio", "models"), source: protocol.LocalFoundLMStudio, cache: true},
		{path: filepath.Join(cache, "llama.cpp"), source: protocol.LocalFoundLlamaCpp, cache: true},
		{path: filepath.Join(home, ".local", "share", "llama.cpp", "models"), source: protocol.LocalFoundLlamaCpp},
		{path: filepath.Join(home, "llama.cpp", "models"), source: protocol.LocalFoundLlamaCpp},
		{path: filepath.Join(home, ".lmstudio", "models"), source: protocol.LocalFoundLMStudio},
		{path: filepath.Join(home, ".ollama", "models", "manifests"), source: protocol.LocalFoundOllama, ollama: true},
		{path: filepath.Join(home, ".local", "share", "ollama", "models", "manifests"), source: protocol.LocalFoundOllama, ollama: true},
		{path: filepath.Join(home, ".local", "share", "Jan", "data", "models"), source: protocol.LocalFoundFolder},
		{path: filepath.Join(home, ".config", "Jan", "data", "models"), source: protocol.LocalFoundFolder},
		{path: filepath.Join(home, "jan", "models"), source: protocol.LocalFoundFolder},
		{path: filepath.Join(cache, "gpt4all"), source: protocol.LocalFoundFolder, cache: true},
		{path: filepath.Join(home, ".local", "share", "nomic.ai", "GPT4All"), source: protocol.LocalFoundFolder},
		{path: filepath.Join(home, "GPT4All"), source: protocol.LocalFoundFolder},
		{path: filepath.Join(home, "koboldcpp"), source: protocol.LocalFoundFolder},
		{path: filepath.Join(home, "koboldcpp", "models"), source: protocol.LocalFoundFolder},
		{path: filepath.Join(home, "text-generation-webui", "models"), source: protocol.LocalFoundFolder},
		{path: filepath.Join(home, "text-generation-webui", "user_data", "models"), source: protocol.LocalFoundFolder},
		{path: filepath.Join(home, "models"), source: protocol.LocalFoundFolder},
		{path: filepath.Join(home, "Models"), source: protocol.LocalFoundFolder},
		{path: filepath.Join(home, "AI"), source: protocol.LocalFoundFolder},
		{path: filepath.Join(home, "Downloads"), source: protocol.LocalFoundFolder},
	}
	if hf := os.Getenv("HF_HOME"); filepath.IsAbs(hf) {
		roots = append(roots, foundRoot{path: filepath.Join(hf, "hub"), source: protocol.LocalFoundHuggingFace, cache: true})
	}
	if hf := os.Getenv("HF_HUB_CACHE"); filepath.IsAbs(hf) {
		roots = append(roots, foundRoot{path: hf, source: protocol.LocalFoundHuggingFace, cache: true})
	}
	if llama := os.Getenv("LLAMA_CACHE"); filepath.IsAbs(llama) {
		roots = append(roots, foundRoot{path: llama, source: protocol.LocalFoundLlamaCpp, cache: true})
	}
	// Keep wildcard expansion bounded even if home contains many files.
	if f, err := os.Open(home); err == nil {
		entries, _ := f.ReadDir(2048)
		_ = f.Close()
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(strings.ToLower(entry.Name()), "llm") {
				roots = append(roots, foundRoot{path: filepath.Join(home, entry.Name()), source: protocol.LocalFoundFolder})
			}
		}
	}
	if ollama := os.Getenv("OLLAMA_MODELS"); filepath.IsAbs(ollama) {
		roots = append(roots, foundRoot{path: filepath.Join(ollama, "manifests"), source: protocol.LocalFoundOllama, ollama: true})
	}
	for _, folder := range folders {
		if filepath.IsAbs(folder) {
			roots = append(roots, foundRoot{path: filepath.Clean(folder), source: protocol.LocalFoundFolder})
		}
	}
	roots = append(roots, foundRoot{path: "/usr/share/ollama/.ollama/models/manifests", source: protocol.LocalFoundOllama, ollama: true})
	return roots
}

// Found examines user-owned locations only when explicitly requested by the local models UI.
func (m *Manager) Found(ctx context.Context, folders []string) ([]protocol.LocalFoundModel, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	roots := foundRoots(home, folders)
	seen := make(map[string]bool)
	used := make(map[string]string)
	for _, e := range readCustom() {
		if e.External != "" {
			if model, ok := e.model(); ok {
				used[filepath.Join(e.External, model.Name)] = e.ID
			}
		}
	}
	devices := m.Devices(ctx)
	hardware := hostHardware(devices)
	var found []protocol.LocalFoundModel
	remaining := foundMaxEntries
	add := func(path, source, label string) {
		if len(found) >= foundMaxResults || seen[path] {
			return
		}
		selected := path
		seen[path] = true
		e, meta, err := inspectFound(path)
		if err != nil {
			return
		}
		model, _ := e.model()
		path = filepath.Join(e.External, model.Name)
		if path != selected && seen[path] {
			return
		}
		seen[path] = true
		plan := planModel(e, meta, hardware, protocol.LocalModelOverride{})
		quant, _ := quantAndQuality(model.Name)
		if label == "" {
			label = model.Name
		}
		found = append(found, protocol.LocalFoundModel{Path: path, Name: label, Source: source,
			Size: e.size(), Quant: quant, Params: meta.Params, Context: meta.Context, Tools: meta.Tools,
			Vision: e.Vision, Plan: plan, Warning: modelWarning(meta), Model: used[path]})
	}
	for _, root := range roots {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if remaining <= 0 || len(found) >= foundMaxResults {
			break
		}
		base, err := filepath.Abs(root.path)
		if err != nil {
			continue
		}
		if root.ollama {
			walkFound(ctx, base, &remaining, !root.cache, func(path string, entry fs.DirEntry) {
				if !entry.Type().IsRegular() {
					return
				}
				info, err := entry.Info()
				if err != nil || info.Size() > foundMaxManifest {
					return
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return
				}
				var manifest struct {
					Layers []struct {
						MediaType string `json:"mediaType"`
						Digest    string `json:"digest"`
					} `json:"layers"`
				}
				if json.Unmarshal(data, &manifest) != nil || len(manifest.Layers) > 64 {
					return
				}
				for _, layer := range manifest.Layers {
					if !strings.Contains(layer.MediaType, "image.model") || !strings.HasPrefix(layer.Digest, "sha256:") {
						continue
					}
					digest := strings.TrimPrefix(layer.Digest, "sha256:")
					if len(digest) != 64 || strings.Trim(digest, "0123456789abcdef") != "" {
						continue
					}
					blob := filepath.Join(filepath.Dir(base), "blobs", "sha256-"+digest)
					add(blob, protocol.LocalFoundOllama, filepath.Base(filepath.Dir(path))+":"+filepath.Base(path))
				}
			})
			continue
		}
		walkFound(ctx, base, &remaining, !root.cache, func(path string, entry fs.DirEntry) {
			if !strings.EqualFold(filepath.Ext(path), ".gguf") || strings.HasPrefix(strings.ToLower(entry.Name()), "mmproj") {
				return
			}
			add(path, root.source, "")
		})
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Path < found[j].Path })
	return found, nil
}

func walkFound(ctx context.Context, root string, remaining *int, skipHidden bool, visit func(string, fs.DirEntry)) {
	type dir struct {
		path  string
		depth int
	}
	stack := []dir{{root, 0}}
	for len(stack) != 0 && *remaining > 0 && ctx.Err() == nil {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		f, err := os.Open(current.path)
		if err != nil {
			continue
		}
		for *remaining > 0 && ctx.Err() == nil {
			items, err := f.ReadDir(128)
			for _, entry := range items {
				if *remaining == 0 || ctx.Err() != nil {
					break
				}
				*remaining--
				path := filepath.Join(current.path, entry.Name())
				if entry.IsDir() {
					if skipHidden && strings.HasPrefix(entry.Name(), ".") {
						continue
					}
					if current.depth < foundMaxDepth {
						stack = append(stack, dir{path, current.depth + 1})
					}
				} else if entry.Type().IsRegular() {
					visit(path, entry)
				} else if entry.Type()&os.ModeSymlink != 0 {
					if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
						visit(path, entry)
					}
				}
			}
			if err != nil {
				break
			}
		}
		_ = f.Close()
	}
}

// inspectFound checks a complete split group and its adjacent projector without hashing any weights.
func readFoundDir(path string, maxEntries int) ([]fs.DirEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	items, err := f.ReadDir(maxEntries + 1)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if len(items) > maxEntries {
		return nil, errGGUFHeader
	}
	return items, nil
}

func inspectFound(path string) (Entry, ggufMetadata, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Entry{}, ggufMetadata{}, err
	}
	abs = filepath.Clean(abs)
	base, dir := filepath.Base(abs), filepath.Dir(abs)
	if strings.HasPrefix(strings.ToLower(base), "mmproj") {
		return Entry{}, ggufMetadata{}, errGGUFHeader
	}
	items, err := readFoundDir(dir, 4096)
	if err != nil {
		return Entry{}, ggufMetadata{}, err
	}
	names := []string{base}
	if part := splitGGUF.FindStringSubmatch(base); part != nil {
		names = nil
		for _, item := range items {
			match := splitGGUF.FindStringSubmatch(item.Name())
			if match != nil && match[1] == part[1] && match[3] == part[3] {
				names = append(names, item.Name())
			}
		}
		if !completeSplitNames(names) {
			return Entry{}, ggufMetadata{}, errGGUFHeader
		}
		sort.Strings(names)
	}
	var candidates []protocol.LocalFile
	for _, item := range items {
		if strings.HasPrefix(strings.ToLower(item.Name()), "mmproj") && strings.EqualFold(filepath.Ext(item.Name()), ".gguf") {
			candidates = append(candidates, protocol.LocalFile{Name: item.Name(), Role: protocol.LocalFileMmproj})
		}
	}
	projector := matchingMmproj(candidates, names[0])
	e := Entry{External: dir, Name: names[0], Class: protocol.LocalClassLight}
	for _, name := range names {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || !validName(name) {
			return Entry{}, ggufMetadata{}, errGGUFHeader
		}
		e.Files = append(e.Files, File{Name: name, Size: info.Size(), ModTime: info.ModTime().UnixNano(), Role: protocol.LocalFileModel})
	}
	if projector != "" {
		if info, err := os.Stat(filepath.Join(dir, projector)); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
			e.Files = append(e.Files, File{Name: projector, Size: info.Size(), ModTime: info.ModTime().UnixNano(), Role: protocol.LocalFileMmproj})
			e.Vision, e.Audio, e.Video = true, true, true
		}
	}
	f, err := os.Open(filepath.Join(dir, names[0]))
	if err != nil {
		return Entry{}, ggufMetadata{}, err
	}
	meta, err := readFoundHeader(f)
	after, statErr := f.Stat()
	_ = f.Close()
	if err != nil {
		return Entry{}, ggufMetadata{}, err
	}
	if statErr != nil || after.Size() != e.Files[0].Size || after.ModTime().UnixNano() != e.Files[0].ModTime {
		return Entry{}, ggufMetadata{}, errGGUFHeader
	}
	for _, file := range e.Files {
		if file.Role == protocol.LocalFileModel {
			meta.ModelBytes += file.Size
		} else {
			meta.MmprojBytes += file.Size
		}
	}
	e.Params, e.Context, e.Tools = meta.Params, meta.Context, meta.Tools
	if meta.Params > protocol.LightParams {
		e.Class = protocol.LocalClassMain
	}
	sum := sha256.Sum256([]byte(filepath.Join(dir, names[0])))
	tag := hex.EncodeToString(sum[:12])
	e.ID, e.Repo, e.Revision = "found-"+tag, "found/"+tag, tag
	e.License.Name = i18n.T("Local file; check its licence before sharing")
	return e, meta, nil
}

func readFoundHeader(f *os.File) (ggufMetadata, error) {
	data := make([]byte, 64<<10)
	n, err := io.ReadFull(f, data)
	for {
		if n < 4 || string(data[:4]) != "GGUF" {
			return ggufMetadata{}, errGGUFHeader
		}
		if meta, parseErr := parseGGUF(data[:n]); parseErr == nil {
			return meta, nil
		}
		if err != nil || len(data) >= ggufHeaderLimit {
			return ggufMetadata{}, errGGUFHeader
		}
		grown := make([]byte, min(len(data)*2, ggufHeaderLimit))
		copy(grown, data)
		more, readErr := io.ReadFull(f, grown[n:])
		data, n, err = grown, n+more, readErr
	}
}

func foundOllamaLabel(blob string) string {
	if filepath.Base(filepath.Dir(blob)) != "blobs" {
		return ""
	}
	digest := strings.TrimPrefix(filepath.Base(blob), "sha256-")
	if len(digest) != 64 || strings.Trim(digest, "0123456789abcdef") != "" {
		return ""
	}
	root := filepath.Join(filepath.Dir(filepath.Dir(blob)), "manifests")
	remaining := 2048
	var label string
	walkFound(context.Background(), root, &remaining, true, func(path string, entry fs.DirEntry) {
		if label != "" || !entry.Type().IsRegular() {
			return
		}
		info, err := entry.Info()
		if err != nil || info.Size() > foundMaxManifest {
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return
		}
		var manifest struct {
			Layers []struct {
				MediaType string `json:"mediaType"`
				Digest    string `json:"digest"`
			} `json:"layers"`
		}
		if json.Unmarshal(data, &manifest) != nil || len(manifest.Layers) > 64 {
			return
		}
		for _, layer := range manifest.Layers {
			if strings.Contains(layer.MediaType, "image.model") && layer.Digest == "sha256:"+digest {
				label = filepath.Base(filepath.Dir(path)) + ":" + filepath.Base(path)
				return
			}
		}
	})
	return label
}

// UseFound links existing files in place; only Rashin's own manifest and catalogue are written.
func (m *Manager) UseFound(path string) (protocol.LocalModel, error) {
	e, meta, err := inspectFound(path)
	if err != nil {
		return protocol.LocalModel{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Choose a complete, readable GGUF model."))
	}
	if filepath.Base(e.External) == "blobs" && strings.HasPrefix(e.Name, "sha256-") {
		if label := foundOllamaLabel(path); label != "" {
			e.Name = label
		}
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return protocol.LocalModel{}, protocol.Errorf(protocol.CodeUnavailable, "%s", i18n.T("Hyakki is shutting down."))
	}
	old := readCustom()
	for _, existing := range old {
		if existing.ID == e.ID {
			if current, _ := onDisk(existing); current.State == protocol.LocalInstalled {
				m.mu.Unlock()
				return current, nil
			}
		}
	}
	updated := make([]Entry, 0, len(old)+1)
	for _, existing := range old {
		if existing.ID != e.ID {
			updated = append(updated, existing)
		}
	}
	updated = append(updated, e)
	if err := writeCustom(updated); err != nil {
		m.mu.Unlock()
		return protocol.LocalModel{}, err
	}
	if err := writeManifest(e, manifest{ID: e.ID, Repo: e.Repo, Revision: e.Revision, Files: e.Files, Header: &meta}); err != nil {
		_ = writeCustom(old)
		m.mu.Unlock()
		return protocol.LocalModel{}, err
	}
	model, _ := onDisk(e)
	m.mu.Unlock()
	m.refreshRunningPresets(context.Background())
	m.notify()
	return model, nil
}
