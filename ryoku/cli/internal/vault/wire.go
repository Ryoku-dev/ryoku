package vault

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	pointerBegin    = "<!-- ryoku-vault:begin -->"
	pointerEnd      = "<!-- ryoku-vault:end -->"
	oldPointerBegin = "<!-- ryoku-rashin:begin -->"
	oldPointerEnd   = "<!-- ryoku-rashin:end -->"
)

func pointerBlock() string {
	return pointerBegin + `
## Ryoku vault

This machine's maintained map lives at ` + "`" + Path() + "/`" + `.
Read ` + "`AGENTS.md`" + ` there before searching for config paths. Keep durable notes in
` + "`memory/`" + ` and dated work in ` + "`journal/YYYY-MM-DD.md`" + `. The linked ` + "`ryoku`" + ` skill
is the desktop change contract. Rashin is optional and the vault does not depend on it.
` + pointerEnd
}

type harnessTarget struct {
	parent string
	file   string
	skill  string
}

func harnessTargets() []harnessTarget {
	h := home()
	return []harnessTarget{
		{filepath.Join(h, ".claude"), filepath.Join(h, ".claude", "CLAUDE.md"), filepath.Join(h, ".claude", "skills", "ryoku")},
		{filepath.Join(h, ".codex"), filepath.Join(h, ".codex", "AGENTS.md"), filepath.Join(h, ".codex", "skills", "ryoku")},
		{filepath.Join(sysConfigHome(), "opencode"), filepath.Join(sysConfigHome(), "opencode", "AGENTS.md"), ""},
		{filepath.Join(h, ".omp", "agent"), filepath.Join(h, ".omp", "agent", "AGENTS.md"), filepath.Join(h, ".omp", "agent", "skills", "ryoku")},
	}
}

func skillTargets() []struct{ parent, link string } {
	h := home()
	out := []struct{ parent, link string }{
		{filepath.Join(h, ".agents"), filepath.Join(h, ".agents", "skills", "ryoku")},
		{filepath.Join(h, ".hermes"), filepath.Join(h, ".hermes", "skills", "ryoku")},
	}
	for _, target := range harnessTargets() {
		if target.skill != "" {
			out = append(out, struct{ parent, link string }{target.parent, target.skill})
		}
	}
	return out
}

func sysConfigHome() string {
	if value := os.Getenv("XDG_CONFIG_HOME"); value != "" {
		return value
	}
	return filepath.Join(home(), ".config")
}

func Wire() error {
	if err := Ensure(); err != nil {
		return err
	}
	var first error
	for _, target := range harnessTargets() {
		if !isDir(target.parent) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target.file), 0o755); err != nil {
			if first == nil {
				first = err
			}
			continue
		}
		doc := removeBlock(readFile(target.file), oldPointerBegin, oldPointerEnd)
		doc = upsertPointer(doc)
		if err := atomicWrite(target.file, []byte(doc), 0o644); err != nil && first == nil {
			first = err
		}
	}
	source := sourceDir(filepath.Join("skills", "ryoku"))
	if source != "" {
		for _, target := range skillTargets() {
			if !isDir(target.parent) {
				continue
			}
			if err := linkSkill(target.link, source); err != nil && first == nil {
				first = err
			}
		}
	}
	if first != nil {
		return first
	}
	if err := os.Remove(OptOutPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func Unwire() error {
	var first error
	for _, target := range harnessTargets() {
		doc := readFile(target.file)
		if doc != "" {
			doc = removeBlock(doc, pointerBegin, pointerEnd)
			doc = removeBlock(doc, oldPointerBegin, oldPointerEnd)
			if err := atomicWrite(target.file, []byte(doc), 0o644); err != nil && first == nil {
				first = err
			}
		}
	}
	for _, target := range skillTargets() {
		if ownsSkillLink(target.link) {
			if err := os.Remove(target.link); err != nil && !errors.Is(err, os.ErrNotExist) && first == nil {
				first = err
			}
		}
	}
	if err := atomicWrite(OptOutPath(), []byte("unwired\n"), 0o644); err != nil && first == nil {
		first = err
	}
	return first
}

func IsOptedOut() bool {
	return fileExists(OptOutPath())
}

func Wired() bool {
	for _, target := range harnessTargets() {
		if !isDir(target.parent) {
			continue
		}
		doc := readFile(target.file)
		if !strings.Contains(doc, pointerBlock()) || strings.Contains(doc, oldPointerBegin) {
			return false
		}
	}
	source := sourceDir(filepath.Join("skills", "ryoku"))
	if source == "" {
		return true
	}
	for _, target := range skillTargets() {
		if isDir(target.parent) && !skillLinkPointsTo(target.link, source) {
			return false
		}
	}
	return true
}

func skillLinkPointsTo(link, source string) bool {
	target, err := os.Readlink(link)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(link), target)
	}
	return filepath.Clean(target) == filepath.Clean(source)
}

func upsertPointer(doc string) string {
	doc = removeBlock(doc, oldPointerBegin, oldPointerEnd)
	begin := strings.Index(doc, pointerBegin)
	end := strings.Index(doc, pointerEnd)
	if begin >= 0 && end > begin {
		return doc[:begin] + pointerBlock() + doc[end+len(pointerEnd):]
	}
	if strings.TrimSpace(doc) == "" {
		return pointerBlock() + "\n"
	}
	return strings.TrimRight(doc, "\n") + "\n\n" + pointerBlock() + "\n"
}

func removeBlock(doc, beginMark, endMark string) string {
	for {
		begin := strings.Index(doc, beginMark)
		end := strings.Index(doc, endMark)
		if begin < 0 || end <= begin {
			return doc
		}
		before := strings.TrimRight(doc[:begin], "\n")
		after := strings.TrimLeft(doc[end+len(endMark):], "\n")
		switch {
		case before == "":
			doc = after
		case after == "":
			doc = before + "\n"
		default:
			doc = before + "\n\n" + after
		}
	}
}

func linkSkill(link, source string) error {
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	if info, err := os.Lstat(link); err == nil {
		if info.Mode()&os.ModeSymlink == 0 || !ownsSkillLink(link) {
			return nil
		}
		current, _ := os.Readlink(link)
		if filepath.Clean(current) == filepath.Clean(source) {
			return nil
		}
		if err := os.Remove(link); err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Symlink(source, link)
}

func ownsSkillLink(link string) bool {
	info, err := os.Lstat(link)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return false
	}
	target, err := os.Readlink(link)
	if err != nil {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(target))
	if strings.HasSuffix(clean, "/ryoku/vault/skills/ryoku") ||
		strings.HasSuffix(clean, "/share/ryoku/vault/skills/ryoku") ||
		strings.HasSuffix(clean, "/ryoku/rashin/skills/ryoku") ||
		strings.HasSuffix(clean, "/share/ryoku/skills/ryoku") ||
		strings.HasSuffix(clean, "/share/ryoku/rashin/skills/ryoku") {
		return true
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(link), target)
	}
	return strings.Contains(readFile(filepath.Join(target, "SKILL.md")), "name: ryoku")
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
