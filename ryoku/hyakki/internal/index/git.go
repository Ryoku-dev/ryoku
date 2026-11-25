package index

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"ryoku-rashin/internal/protocol"
)

func (e *Engine) Wip(ctx context.Context, p protocol.WipParams) (protocol.WipResult, error) {
	entry, store, err := e.queryStore(ctx, p.IndexRef)
	if err != nil {
		return protocol.WipResult{}, err
	}
	root, err := projectRoot(entry)
	if err != nil {
		return protocol.WipResult{}, err
	}
	output, err := exec.CommandContext(ctx, "git", "-C", root, "status", "--porcelain=v1", "-z").Output()
	if err != nil {
		return protocol.WipResult{}, err
	}
	changes := parsePorcelain(output)
	return e.decorateChanges(ctx, entry, store, root, "", changes)
}
func (e *Engine) Changed(ctx context.Context, p protocol.WipParams) (protocol.WipResult, error) {
	entry, store, err := e.queryStore(ctx, p.IndexRef)
	if err != nil {
		return protocol.WipResult{}, err
	}
	root, err := projectRoot(entry)
	if err != nil {
		return protocol.WipResult{}, err
	}
	base := p.Base
	if base == "" {
		base = "main"
	}
	output, err := exec.CommandContext(ctx, "git", "-C", root, "diff", "--name-status", "-z", base+"...HEAD").Output()
	if err != nil {
		return protocol.WipResult{}, err
	}
	changes := parseNameStatus(output)
	working, workErr := exec.CommandContext(ctx, "git", "-C", root, "status", "--porcelain=v1", "-z").Output()
	if workErr == nil {
		changes = mergeChanges(changes, parsePorcelain(working))
	}
	return e.decorateChanges(ctx, entry, store, root, base, changes)
}
func projectRoot(entry registryEntry) (string, error) {
	if entry.Kind != "project" || len(entry.Roots) == 0 {
		return "", os.ErrInvalid
	}
	return entry.Roots[0], nil
}
func parsePorcelain(output []byte) []protocol.GitChange {
	parts := strings.Split(string(output), "\x00")
	var out []protocol.GitChange
	for i := 0; i < len(parts); i++ {
		part := parts[i]
		if len(part) < 4 {
			continue
		}
		code, path := part[:2], filepath.ToSlash(part[3:])
		status := "modified"
		switch {
		case code == "??":
			status = "untracked"
		case strings.Contains(code, "R"):
			status = "renamed"
			if i+1 < len(parts) {
				i++
			}
		case strings.Contains(code, "D"):
			status = "deleted"
		case strings.Contains(code, "A"):
			status = "added"
		case code[0] != ' ' && code[0] != '?':
			status = "staged"
		}
		out = append(out, protocol.GitChange{Path: path, Status: status})
	}
	return out
}
func parseNameStatus(output []byte) []protocol.GitChange {
	parts := strings.Split(string(output), "\x00")
	var out []protocol.GitChange
	for index := 0; index < len(parts); {
		code := parts[index]
		index++
		if code == "" || index >= len(parts) {
			continue
		}
		changedPath := parts[index]
		index++
		status := "modified"
		switch code[0] {
		case 'A':
			status = "added"
		case 'D':
			status = "deleted"
		case 'R':
			status = "renamed"
			if index < len(parts) && parts[index] != "" {
				changedPath = parts[index]
				index++
			}
		}
		out = append(out, protocol.GitChange{Path: filepath.ToSlash(changedPath), Status: status})
	}
	return out
}
func mergeChanges(groups ...[]protocol.GitChange) []protocol.GitChange {
	byPath := map[string]protocol.GitChange{}
	for _, group := range groups {
		for _, change := range group {
			byPath[change.Path] = change
		}
	}
	out := make([]protocol.GitChange, 0, len(byPath))
	for _, change := range byPath {
		out = append(out, change)
	}
	return out
}
func (e *Engine) decorateChanges(ctx context.Context, entry registryEntry, store *indexStore, root, base string, changes []protocol.GitChange) (protocol.WipResult, error) {
	files, err := store.files()
	if err != nil {
		return protocol.WipResult{}, err
	}
	imports, err := store.imports()
	if err != nil {
		return protocol.WipResult{}, err
	}
	known := map[string]bool{}
	for _, file := range files {
		known[file.Path] = true
	}
	graph := buildImportGraph(entry, files, imports)
	for i := range changes {
		if err := ctx.Err(); err != nil {
			return protocol.WipResult{}, err
		}
		changes[i].Indexed = known[changes[i].Path]
		changes[i].Dependents = transitiveCount(graph, changes[i].Path)
		if changes[i].Status != "deleted" {
			changes[i].Markers = changeMarkers(filepath.Join(root, filepath.FromSlash(changes[i].Path)))
		}
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return protocol.WipResult{Index: entry.ID, Root: root, Base: base, Changes: changes}, nil
}
func transitiveCount(graph importGraph, target string) int {
	visited := map[string]bool{target: true}
	queue := setKeys(graph.reverse[target])
	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]
		if visited[item] {
			continue
		}
		visited[item] = true
		queue = append(queue, setKeys(graph.reverse[item])...)
	}
	delete(visited, target)
	return len(visited)
}
func changeMarkers(path string) []string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	var out []string
	line := 0
	for scanner.Scan() {
		line++
		text := scanner.Text()
		upper := strings.ToUpper(text)
		if strings.Contains(upper, "TODO") || strings.Contains(upper, "FIXME") || strings.Contains(upper, "XXX") {
			out = append(out, fmt.Sprintf("%d: %s", line, strings.TrimSpace(text)))
		}
	}
	return out
}
