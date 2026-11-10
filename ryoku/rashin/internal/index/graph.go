package index

import (
	"bufio"
	"context"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"ryoku-rashin/internal/protocol"
)

func (e *Engine) Overview(ctx context.Context, ref protocol.IndexRef) (protocol.Overview, error) {
	entry, store, err := e.queryStore(ctx, ref)
	if err != nil {
		return protocol.Overview{}, err
	}
	files, err := store.files()
	if err != nil {
		return protocol.Overview{}, err
	}
	symbols, err := store.symbols()
	if err != nil {
		return protocol.Overview{}, err
	}
	chunks, err := store.chunks()
	if err != nil {
		return protocol.Overview{}, err
	}
	imports, err := store.imports()
	if err != nil {
		return protocol.Overview{}, err
	}
	languages := map[string]int{}
	var largest []protocol.FileStat
	var docs []string
	var entrypoints []string
	for _, file := range files {
		languages[file.Language]++
		largest = append(largest, protocol.FileStat{Path: file.Path, Value: int(file.Size)})
		if file.Language == "markdown" && isGuide(file.Path) {
			docs = append(docs, file.Path)
		}
		if isEntrypoint(file) {
			entrypoints = append(entrypoints, file.Path)
		}
	}
	languageCounts := make([]protocol.Count, 0, len(languages))
	for name, count := range languages {
		languageCounts = append(languageCounts, protocol.Count{Name: name, Count: count})
	}
	sortCounts(languageCounts)
	sort.Slice(largest, func(i, j int) bool {
		if largest[i].Value != largest[j].Value {
			return largest[i].Value > largest[j].Value
		}
		return largest[i].Path < largest[j].Path
	})
	if len(largest) > 10 {
		largest = largest[:10]
	}
	graph := buildImportGraph(entry, files, imports)
	imported := map[string]int{}
	for target, sources := range graph.reverse {
		imported[target] = len(sources)
	}
	var most []protocol.FileStat
	for file, count := range imported {
		most = append(most, protocol.FileStat{Path: file, Value: count})
	}
	sort.Slice(most, func(i, j int) bool {
		if most[i].Value != most[j].Value {
			return most[i].Value > most[j].Value
		}
		return most[i].Path < most[j].Path
	})
	if len(most) > 10 {
		most = most[:10]
	}
	sort.Strings(entrypoints)
	sort.Strings(docs)
	if len(docs) > 8 {
		docs = docs[:8]
	}
	root := ""
	if len(entry.Roots) > 0 {
		root = entry.Roots[0]
	}
	return protocol.Overview{Index: entry.ID, Root: root, Files: len(files), Symbols: len(symbols), Chunks: len(chunks), Languages: languageCounts, Entrypoints: entrypoints, Largest: largest, MostImported: most, Docs: docs}, nil
}
func sortCounts(values []protocol.Count) {
	sort.Slice(values, func(i, j int) bool {
		if values[i].Count != values[j].Count {
			return values[i].Count > values[j].Count
		}
		return values[i].Name < values[j].Name
	})
}
func isGuide(file string) bool {
	base := strings.ToLower(path.Base(file))
	return base == "readme.md" || base == "architecture.md" || base == "contributing.md" || strings.HasPrefix(strings.ToLower(file), "docs/") && strings.Contains(base, "guide")
}
func isEntrypoint(file fileRecord) bool {
	clean := filepath.ToSlash(file.Path)
	base := path.Base(clean)
	if base == "main.go" || base == "main.py" || base == "index.js" || base == "index.ts" || base == "__main__.py" || strings.HasPrefix(clean, "cmd/") {
		return true
	}
	source, err := os.Open(file.Source)
	if err != nil {
		return false
	}
	defer source.Close()
	scanner := bufio.NewScanner(source)
	lineNumber := 0
	for scanner.Scan() && lineNumber < 30 {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if lineNumber == 1 && strings.HasPrefix(line, "#!") {
			return true
		}
		if file.Language == "go" && line == "package main" {
			return true
		}
	}
	return false
}

type importGraph struct {
	forward map[string]map[string]bool
	reverse map[string]map[string]bool
}
type goModule struct{ path, dir string }

func indexedGoModules(files []fileRecord) []goModule {
	var modules []goModule
	for _, file := range files {
		if path.Base(file.Path) != "go.mod" {
			continue
		}
		data, err := os.ReadFile(file.Source)
		if err != nil {
			continue
		}
		fields := strings.Fields(string(data))
		for index := 0; index+1 < len(fields); index++ {
			if fields[index] == "module" {
				modules = append(modules, goModule{path: fields[index+1], dir: path.Dir(file.Path)})
				break
			}
		}
	}
	sort.Slice(modules, func(i, j int) bool { return len(modules[i].path) > len(modules[j].path) })
	return modules
}
func addImportEdge(graph *importGraph, source, target string) {
	if target == source {
		return
	}
	if graph.forward[source] == nil {
		graph.forward[source] = map[string]bool{}
	}
	if graph.reverse[target] == nil {
		graph.reverse[target] = map[string]bool{}
	}
	graph.forward[source][target] = true
	graph.reverse[target][source] = true
}
func buildImportGraph(_ registryEntry, files []fileRecord, imports []importRecord) importGraph {
	graph := importGraph{forward: map[string]map[string]bool{}, reverse: map[string]map[string]bool{}}
	known := map[string]fileRecord{}
	for _, file := range files {
		known[file.Path] = file
	}
	modules := indexedGoModules(files)
	for _, edge := range imports {
		source, ok := known[edge.Source]
		if !ok {
			continue
		}
		for _, target := range resolveImport(source, edge.Target, modules, known) {
			addImportEdge(&graph, source.Path, target)
		}
	}
	for _, source := range files {
		if source.Language != "qml" {
			continue
		}
		dir := path.Dir(source.Path)
		for candidate, file := range known {
			if file.Language == "qml" && path.Dir(candidate) == dir {
				addImportEdge(&graph, source.Path, candidate)
			}
		}
	}
	return graph
}
func resolveImport(source fileRecord, target string, modules []goModule, known map[string]fileRecord) []string {
	target = strings.TrimSpace(strings.Trim(target, "\"'"))
	if target == "" {
		return nil
	}
	base := path.Dir(source.Path)
	var stems []string
	switch source.Language {
	case "go":
		for _, module := range modules {
			if target == module.path || strings.HasPrefix(target, module.path+"/") {
				relative := strings.TrimPrefix(strings.TrimPrefix(target, module.path), "/")
				stems = append(stems, path.Clean(path.Join(module.dir, relative)))
				break
			}
		}
		if len(stems) == 0 && strings.HasPrefix(target, ".") {
			stems = append(stems, path.Clean(path.Join(base, target)))
		}
	case "python":
		dots := 0
		for dots < len(target) && target[dots] == '.' {
			dots++
		}
		clean := strings.ReplaceAll(target[dots:], ".", "/")
		origin := base
		for i := 1; i < dots; i++ {
			origin = path.Dir(origin)
		}
		if dots > 0 {
			stems = append(stems, path.Join(origin, clean))
		} else {
			stems = append(stems, clean)
		}
	case "javascript", "typescript":
		if strings.HasPrefix(target, ".") {
			stems = append(stems, path.Clean(path.Join(base, target)))
		} else {
			stems = append(stems, target)
		}
	case "c", "cpp":
		stems = append(stems, path.Clean(path.Join(base, target)), target)
	case "qml":
		stems = append(stems, path.Clean(path.Join(base, target)))
	case "bash", "fish":
		stems = append(stems, path.Clean(path.Join(base, target)), path.Clean(target))
	case "lua":
		stems = append(stems, path.Clean(path.Join(base, strings.ReplaceAll(target, ".", "/"))), strings.ReplaceAll(target, ".", "/"))
	}
	var out []string
	seen := map[string]bool{}
	extensions := []string{"", ".go", ".py", ".js", ".jsx", ".ts", ".tsx", ".qml", ".lua", ".sh", ".fish", ".c", ".h", ".cpp", ".hpp"}
	for _, stem := range stems {
		for _, ext := range extensions {
			candidate := stem + ext
			if _, ok := known[candidate]; ok && !seen[candidate] {
				out = append(out, candidate)
				seen[candidate] = true
			}
		}
		for _, index := range []string{"index.js", "index.ts", "__init__.py"} {
			candidate := path.Join(stem, index)
			if _, ok := known[candidate]; ok && !seen[candidate] {
				out = append(out, candidate)
				seen[candidate] = true
			}
		}
		if source.Language == "go" || source.Language == "qml" {
			for candidate, file := range known {
				if path.Dir(candidate) == strings.TrimSuffix(stem, "/") && !seen[candidate] && (source.Language != "qml" || file.Language == "qml") {
					out = append(out, candidate)
					seen[candidate] = true
				}
			}
		}
	}
	return out
}

func (e *Engine) Impact(ctx context.Context, p protocol.PathParams) (protocol.Impact, error) {
	entry, store, err := e.queryStore(ctx, p.IndexRef)
	if err != nil {
		return protocol.Impact{}, err
	}
	files, err := store.files()
	if err != nil {
		return protocol.Impact{}, err
	}
	imports, err := store.imports()
	if err != nil {
		return protocol.Impact{}, err
	}
	target := filepath.ToSlash(filepath.Clean(p.Path))
	graph := buildImportGraph(entry, files, imports)
	direct := setKeys(graph.reverse[target])
	sort.Strings(direct)
	visited := map[string]bool{target: true}
	queue := append([]string(nil), direct...)
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		if visited[item] {
			continue
		}
		visited[item] = true
		for dependent := range graph.reverse[item] {
			if !visited[dependent] {
				queue = append(queue, dependent)
			}
		}
	}
	delete(visited, target)
	all := setKeys(visited)
	sort.Strings(all)
	result := protocol.Impact{Index: entry.ID, Path: target, Direct: direct, Total: len(all)}
	if p.All {
		result.All = all
	}
	return result, nil
}
func setKeys[V ~bool](values map[string]V) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	return out
}
