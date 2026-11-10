package vault

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var legacyGenerated = map[string]bool{
	"AGENTS.md":     true,
	"CLAUDE.md":     true,
	"system.md":     true,
	"desktop.md":    true,
	"packages.md":   true,
	"ryoku-repo.md": true,
	"user.md":       true,
	"habits.md":     true,
	"ownership.md":  true,
	"logs.md":       true,
	"wiki":          true,
	"source":        true,
}

func LegacyPresent() bool {
	agents := readFile(filepath.Join(legacyPath(), "AGENTS.md"))
	return strings.Contains(agents, "<!-- rashin:generated:begin -->")
}

func Migrate() error {
	old := legacyPath()
	if !LegacyPresent() {
		return nil
	}
	if err := Ensure(); err != nil {
		return err
	}
	entries, err := os.ReadDir(old)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if legacyGenerated[name] {
			if err := preserveLegacyGenerated(filepath.Join(old, name), name); err != nil {
				return err
			}
			continue
		}
		if err := mergeMove(filepath.Join(old, name), filepath.Join(Path(), name)); err != nil {
			return err
		}
	}
	return os.RemoveAll(old)
}

func preserveLegacyGenerated(source, name string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.IsDir() || info.Mode()&os.ModeSymlink != 0 || name == "CLAUDE.md" {
		return os.RemoveAll(source)
	}
	prose := legacyProse(readFile(source))
	if prose != "" {
		destination := filepath.Join(Path(), name)
		existing := readFile(destination)
		if !strings.Contains(existing, prose) {
			doc := strings.TrimRight(existing, "\n")
			if doc != "" {
				doc += "\n\n"
			}
			doc += prose + "\n"
			if err := atomicWrite(destination, []byte(doc), 0o644); err != nil {
				return err
			}
		}
	}
	return os.Remove(source)
}

func legacyProse(doc string) string {
	doc = removeBlock(doc, "<!-- rashin:generated:begin -->", "<!-- rashin:generated:end -->")
	doc = removeBlock(doc, "<!-- rashin:generated:begin bar -->", "<!-- rashin:generated:end bar -->")
	return strings.TrimSpace(doc)
}

func mergeMove(source, destination string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		target := destination
		if _, err := os.Lstat(target); err == nil {
			target = conflictPath(destination)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.Rename(source, target)
	}
	if err := os.MkdirAll(destination, info.Mode().Perm()); err != nil {
		return err
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := mergeMove(filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name())); err != nil {
			return err
		}
	}
	return os.Remove(source)
}

func conflictPath(path string) string {
	base := path + ".rashin-1"
	if _, err := os.Lstat(base); errors.Is(err, os.ErrNotExist) {
		return base
	}
	for number := 2; ; number++ {
		candidate := fmt.Sprintf("%s.rashin-1-%d", path, number)
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
}
