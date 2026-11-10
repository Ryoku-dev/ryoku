package vault

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	generatedBegin = "<!-- ryoku-vault:generated:begin -->"
	generatedEnd   = "<!-- ryoku-vault:generated:end -->"
)

var generatedFiles = []string{
	"system.md",
	"desktop.md",
	"packages.md",
	"ownership.md",
	"logs.md",
	"user.md",
	"ryoku-repo.md",
}

const agentsContract = `# Ryoku system vault

This is the shared machine map for every agent on this Ryoku desktop. Read
` + "`desktop.md`" + ` before searching the filesystem or guessing where a setting lives.
The edition and compositor come from that map and ` + "`ryoku wm status`" + `; detect them,
never assume.

## What is here

- ` + "`system.md`" + ` covers hardware, the kernel, GPU, displays and disks.
- ` + "`desktop.md`" + ` maps subsystems, config paths, owners, reload commands, the GUI,
  bar and dock.
- ` + "`packages.md`" + ` records installed packages and pending updates.
- ` + "`ownership.md`" + ` says where a config edit belongs. Run ` + "`ryoku owner <path>`" + `
  before editing a config file.
- ` + "`logs.md`" + ` gives the exact log command for each subsystem.
- ` + "`user.md`" + ` records where this user's config differs from the shipped baseline.
- ` + "`ryoku-repo.md`" + ` is the source-tree map shipped with Ryoku.
- ` + "`wiki/`" + ` is the shipped read-only guide set.
- ` + "`memory/`" + ` and ` + "`journal/`" + ` belong to users and agents. Keep durable learned facts
  in memory and dated work notes in ` + "`journal/YYYY-MM-DD.md`" + `.

## Rules

Generated text between the ` + "`ryoku-vault:generated`" + ` markers is replaced by
` + "`ryoku vault refresh`" + `. Keep personal prose outside those fences. Never undo a
change listed in ` + "`user.md`" + ` unless the user asks. Use the linked ` + "`ryoku`" + ` skill
for desktop changes, answer GUI-first, and change state through the owning command rather
than editing generated or shipped files.

Rashin is optional. When it is on, ` + "`rashin search`" + `, ` + "`rashin find`" + `,
` + "`rashin def`" + `, ` + "`rashin outline`" + ` and ` + "`rashin references`" + ` provide cited code
search. The vault remains available when Rashin is off or not installed.
`

func buildFence(body string) string {
	return generatedBegin + "\n" + strings.TrimRight(body, "\n") + "\n" + generatedEnd
}

func ReplaceFenced(doc, body string) string {
	fence := buildFence(body)
	begin := strings.Index(doc, generatedBegin)
	end := strings.Index(doc, generatedEnd)
	if begin >= 0 && end > begin {
		return doc[:begin] + fence + doc[end+len(generatedEnd):]
	}
	if strings.TrimSpace(doc) == "" {
		return fence + "\n"
	}
	return fence + "\n\n" + strings.TrimRight(doc, "\n") + "\n"
}

func Ensure() error {
	root := Path()
	for _, dir := range []string{root, filepath.Join(root, "memory"), filepath.Join(root, "journal")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	agents := filepath.Join(root, "AGENTS.md")
	if err := atomicWrite(agents, []byte(ReplaceFenced(readFile(agents), agentsContract)), 0o644); err != nil {
		return err
	}
	claude := filepath.Join(root, "CLAUDE.md")
	if target, err := os.Readlink(claude); err == nil && target == "AGENTS.md" {
		return syncWiki()
	}
	if _, err := os.Lstat(claude); err == nil {
		saved := conflictPath(claude + ".saved")
		if err := os.Rename(claude, saved); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Symlink("AGENTS.md", claude); err != nil {
		return err
	}
	return syncWiki()
}

func writeGenerated(name, header, body string) error {
	path := filepath.Join(Path(), name)
	doc := readFile(path)
	if doc == "" {
		doc = header + "\n\n" + buildFence(body) + "\n"
	} else {
		doc = ReplaceFenced(doc, body)
	}
	return atomicWrite(path, []byte(doc), 0o644)
}

func syncWiki() error {
	source := sourceDir("wiki")
	if source == "" {
		return nil
	}
	destination := filepath.Join(Path(), "wiki")
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	keep := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(source, entry.Name()))
		if err != nil {
			return err
		}
		keep[entry.Name()] = true
		if err := atomicWrite(filepath.Join(destination, entry.Name()), data, 0o644); err != nil {
			return err
		}
	}
	current, err := os.ReadDir(destination)
	if err != nil {
		return err
	}
	for _, entry := range current {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") && !keep[entry.Name()] {
			if err := os.Remove(filepath.Join(destination, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}

func atomicWrite(path string, data []byte, mode os.FileMode) error {
	writePath := path
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return err
		}
		writePath = resolved
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(writePath), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(writePath), ".vault-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, writePath)
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
