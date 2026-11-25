package index

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/paths"
)

type registryEntry struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Kind    string    `json:"kind"`
	Roots   []string  `json:"roots"`
	Created time.Time `json:"created"`
}
type registryFile struct {
	Indexes []registryEntry `json:"indexes"`
}

func loadRegistry() (map[string]registryEntry, error) {
	entries := map[string]registryEntry{}
	data, err := os.ReadFile(paths.IndexRegistry())
	if os.IsNotExist(err) {
		return entries, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read index registry: %w", err)
	}
	var document registryFile
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse index registry: %w", err)
	}
	for _, entry := range document.Indexes {
		entries[entry.ID] = entry
	}
	return entries, nil
}
func saveRegistry(entries map[string]registryEntry) error {
	if err := paths.EnsureDir(filepath.Dir(paths.IndexRegistry())); err != nil {
		return err
	}
	document := registryFile{Indexes: make([]registryEntry, 0, len(entries))}
	for _, entry := range entries {
		document.Indexes = append(document.Indexes, entry)
	}
	sort.Slice(document.Indexes, func(i, j int) bool { return document.Indexes[i].ID < document.Indexes[j].ID })
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	temporary := paths.IndexRegistry() + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, paths.IndexRegistry())
}
func projectID(root string) string {
	sum := sha256.Sum256([]byte(root))
	return hex.EncodeToString(sum[:6])
}
func gitRoot(cwd string) (string, error) {
	absolute, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		absolute = filepath.Dir(absolute)
	}
	for {
		if info, err := os.Stat(filepath.Join(absolute, ".git")); err == nil && (info.IsDir() || info.Mode().IsRegular()) {
			return absolute, nil
		}
		parent := filepath.Dir(absolute)
		if parent == absolute {
			break
		}
		absolute = parent
	}
	return "", fmt.Errorf("%s", i18n.Tf("%s is not inside a Git project.", cwd))
}
func entryContains(entry registryEntry, cwd string) bool {
	absolute, err := filepath.Abs(cwd)
	if err != nil {
		return false
	}
	if canonical, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
		absolute = canonical
	}
	for _, root := range entry.Roots {
		relative, err := filepath.Rel(root, absolute)
		if err == nil && (relative == "." || (!strings.HasPrefix(relative, ".."+string(filepath.Separator)) && relative != "..")) {
			return true
		}
	}
	return false
}
