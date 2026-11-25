package index

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"ryoku-rashin/internal/index/extract"
)

type walkedFile struct {
	Path     string
	Full     string
	Size     int64
	MTime    int64
	Language string
}

type walkPolicy struct {
	System  bool
	MaxSize int64
}

var skippedDirectories = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "third_party": true, "build": true, ".idea": true, ".vscode": true, ".cursor": true, ".zed": true, ".helix": true, ".omp": true, ".factory": true,
	"__pycache__": true, ".pytest_cache": true, ".mypy_cache": true, ".ruff_cache": true, ".tox": true, ".venv": true, "venv": true, ".next": true, ".nuxt": true, "coverage": true,
	".cache": true, ".gradle": true, ".dart_tool": true, ".terraform": true, ".direnv": true, "target": true, "dist": true, "out": true,
}
var systemSkippedRoots = map[string]bool{"mozilla": true, "bravesoftware": true, "vesktop": true, "code": true, "vscodium": true, "code - oss": true, "cursor": true, "slack": true, "signal": true, "element": true, "teams": true, "microsoft teams": true, "zoom": true, "skype": true, "keyrings": true, "electron": true, ".aws": true, ".gnupg": true, ".ssh": true, "1password": true, "bitwarden": true, "bitwarden cli": true, "keepassxc": true, "vivaldi": true, "legcord": true, "armcord": true, "webcord": true, "gcloud": true, "github-copilot": true, "hub": true, "op": true}
var systemSkippedPrefixes = []string{"google-chrome", "chromium", "microsoft-edge", "discord", "opera"}

func walkRoot(ctx context.Context, root, prefix string, policy walkPolicy) ([]walkedFile, error) {
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	root = canonicalRoot
	gitignores := map[string][]string{"": loadGitignore(root)}
	var files []walkedFile
	err = filepath.WalkDir(root, func(full string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsPermission(walkErr) {
				if entry != nil && entry.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(root, full)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			if skippedDirectories[entry.Name()] || (policy.System && backupIgnored(entry.Name(), true)) || ignoredBy(gitignores, nil, rel, true) || systemIgnored(rel, true, policy.System) {
				return filepath.SkipDir
			}
			if patterns := loadGitignore(full); len(patterns) > 0 {
				gitignores[rel] = patterns
			}
			return nil
		}
		if ignoredBy(gitignores, nil, rel, false) || systemIgnored(rel, false, policy.System) {
			return nil
		}
		if policy.System && backupIgnored(entry.Name(), false) {
			return nil
		}
		if !policy.System && projectFileIgnored(rel, 0) {
			return nil
		}
		candidate := full
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			candidate, err = filepath.EvalSymlinks(full)
			if err != nil {
				return nil
			}
			if !within(canonicalRoot, candidate) {
				return nil
			}
			info, err = os.Stat(candidate)
			if err != nil || !info.Mode().IsRegular() {
				return nil
			}
		}
		if !info.Mode().IsRegular() || (policy.MaxSize > 0 && info.Size() > policy.MaxSize) {
			return nil
		}
		if !policy.System && projectFileIgnored(rel, info.Size()) {
			return nil
		}
		file, err := os.Open(candidate)
		if err != nil {
			return nil
		}
		head := make([]byte, 8192)
		n, _ := file.Read(head)
		file.Close()
		head = head[:n]
		if binaryContent(head) {
			return nil
		}
		language := extract.Detect(rel, head)
		if language == "" {
			return nil
		}
		display := rel
		if prefix != "" {
			display = filepath.ToSlash(filepath.Join(prefix, rel))
		}
		files = append(files, walkedFile{Path: display, Full: candidate, Size: info.Size(), MTime: info.ModTime().UnixNano(), Language: language})
		return nil
	})
	return files, err
}

const maxProjectDataBytes = 256 << 10

var projectDataExtensions = map[string]bool{
	".json": true, ".jsonc": true, ".yaml": true, ".yml": true, ".toml": true, ".csv": true, ".tsv": true,
}

func projectFileIgnored(path string, size int64) bool {
	base := strings.ToLower(filepath.Base(path))
	if base == "package-lock.json" || base == "go.sum" || base == "cargo.lock" || strings.HasSuffix(base, ".lock") || strings.HasSuffix(base, ".min.js") || strings.HasSuffix(base, ".map") {
		return true
	}
	return size > maxProjectDataBytes && projectDataExtensions[strings.ToLower(filepath.Ext(base))]
}

func backupIgnored(name string, directory bool) bool {
	lower := strings.ToLower(name)
	if directory && (lower == "backup" || lower == "backups") {
		return true
	}
	return strings.Contains(lower, ".bak") ||
		strings.Contains(lower, ".backup") ||
		strings.HasSuffix(lower, ".old") ||
		strings.HasSuffix(lower, "~") ||
		strings.HasSuffix(lower, ".orig") ||
		strings.HasSuffix(lower, ".rej")
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && (rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."))
}
func binaryContent(head []byte) bool {
	for _, b := range head {
		if b == 0 {
			return true
		}
	}
	return false
}

func systemIgnored(rel string, isDir, system bool) bool {
	if !system {
		return false
	}
	clean := strings.ToLower(filepath.ToSlash(rel))
	first := clean
	if i := strings.IndexByte(first, '/'); i >= 0 {
		first = first[:i]
	}
	for _, part := range strings.Split(clean, "/") {
		if isDir && (part == "cache" || part == "caches" || part == "gpucache" || part == "code cache" || part == "service worker" || part == "extensions") {
			return true
		}
	}
	if systemSkippedRoots[first] {
		return true
	}
	for _, prefix := range systemSkippedPrefixes {
		if strings.HasPrefix(first, prefix) {
			return true
		}
	}
	base := filepath.Base(clean)
	if clean == "gh/hosts.yml" || clean == "rclone/rclone.conf" || base == ".netrc" || base == "auth.json" || strings.Contains(base, "credential") || strings.Contains(base, "token") || strings.Contains(base, "secret") || strings.Contains(base, "adc.json") || strings.Contains(clean, "/keyring") {
		return true
	}
	if !isDir {
		ext := strings.ToLower(filepath.Ext(clean))
		switch ext {
		case ".db", ".sqlite", ".sqlite3", ".mdb", ".ldb":
			return true
		}
	}
	return false
}

func loadGitignore(dir string) []string {
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		return nil
	}
	return parseGitignore(data)
}
func parseGitignore(data []byte) []string {
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			out = append(out, line)
		}
	}
	return out
}
func ancestorDirs(rel string) []string {
	dirs := []string{""}
	parts := strings.Split(rel, "/")
	for i := 1; i < len(parts); i++ {
		dirs = append(dirs, strings.Join(parts[:i], "/"))
	}
	return dirs
}
func ignoredBy(gitignores map[string][]string, extra []string, rel string, isDir bool) bool {
	ignored := false
	for _, dir := range ancestorDirs(rel) {
		patterns, ok := gitignores[dir]
		if !ok {
			continue
		}
		sub := rel
		if dir != "" {
			sub = strings.TrimPrefix(rel, dir+"/")
		}
		if value, matched := matchAny(patterns, sub, isDir); matched {
			ignored = value
		}
	}
	if value, matched := matchAny(extra, rel, isDir); matched {
		ignored = value
	}
	return ignored
}
func matchAny(patterns []string, rel string, isDir bool) (ignored, matched bool) {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" || strings.HasPrefix(pattern, "#") {
			continue
		}
		negated := strings.HasPrefix(pattern, "!")
		if negated {
			pattern = strings.TrimPrefix(pattern, "!")
		}
		anchored := strings.HasPrefix(pattern, "/")
		dirOnly := strings.HasSuffix(pattern, "/")
		pattern = strings.Trim(strings.TrimPrefix(pattern, "/"), "/")
		if pattern == "" || (dirOnly && !isDir) {
			continue
		}
		matches := gitMatch(pattern, rel)
		if anchored {
			matches = matchSegments(strings.Split(filepath.ToSlash(pattern), "/"), strings.Split(filepath.ToSlash(rel), "/"))
		}
		if matches {
			ignored, matched = !negated, true
		}
	}
	return
}
func gitMatch(pattern, rel string) bool {
	pattern = filepath.ToSlash(pattern)
	rel = filepath.ToSlash(rel)
	if !strings.Contains(pattern, "/") {
		ok, _ := filepath.Match(pattern, pathBase(rel))
		return ok
	}
	return matchSegments(strings.Split(pattern, "/"), strings.Split(rel, "/"))
}
func pathBase(rel string) string {
	if index := strings.LastIndexByte(rel, '/'); index >= 0 {
		return rel[index+1:]
	}
	return rel
}
func matchSegments(pattern, path []string) bool {
	if len(pattern) == 0 {
		return len(path) == 0
	}
	if pattern[0] == "**" {
		if matchSegments(pattern[1:], path) {
			return true
		}
		return len(path) > 0 && matchSegments(pattern, path[1:])
	}
	if len(path) == 0 {
		return false
	}
	ok, _ := filepath.Match(pattern[0], path[0])
	return ok && matchSegments(pattern[1:], path[1:])
}
