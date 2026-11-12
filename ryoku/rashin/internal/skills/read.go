package skills

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

// Get reads a skill's SKILL.md, or one of its files named relative to the
// skill's folder, with the skill's other files a reader may open next. It
// reads only inside that folder: a path that climbs out of it or passes
// through a link is refused, and so is a skill of a set that is off.
func (l *Library) Get(params protocol.SkillParams, off []string) (protocol.SkillContent, error) {
	setID, name, _ := strings.Cut(params.ID, "/")
	found := false
	for _, e := range l.entries() {
		if e.set == setID && e.name == name {
			found = true
			break
		}
	}
	if !found || !on(setID, off) {
		return protocol.SkillContent{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("No skill %s.", params.ID))
	}
	file := params.File
	if file == "" {
		file = "SKILL.md"
	}
	file = filepath.ToSlash(file)
	if !filepath.IsLocal(filepath.FromSlash(file)) || path.Clean(file) != file {
		return protocol.SkillContent{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%s is not a file of the skill %s.", params.File, params.ID))
	}
	root, err := l.Root()
	if err != nil {
		return protocol.SkillContent{}, err
	}
	folder := skillFile(root, setID, name, "")
	target := filepath.Join(folder, filepath.FromSlash(file))
	if err := noLinks(folder, file); err != nil {
		return protocol.SkillContent{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("%s is not a file of the skill %s.", params.File, params.ID))
	}
	body, err := os.ReadFile(target)
	if errors.Is(err, fs.ErrNotExist) {
		return protocol.SkillContent{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.Tf("The skill %s has no file %s.", params.ID, file))
	}
	if err != nil {
		return protocol.SkillContent{}, err
	}
	files, err := skillFiles(folder, file)
	if err != nil {
		return protocol.SkillContent{}, err
	}
	return protocol.SkillContent{ID: params.ID, File: file, Path: target, Text: string(body), Files: files}, nil
}

// noLinks refuses a file whose path below folder passes through a link or
// is no regular file.
func noLinks(folder, file string) error {
	current := folder
	parts := strings.Split(file, "/")
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		last := i == len(parts)-1
		if info.Mode()&fs.ModeSymlink != 0 || (last && !info.Mode().IsRegular()) || (!last && !info.IsDir()) {
			return errors.New("not a plain file")
		}
	}
	return nil
}

// skillFiles are the regular files in a skill's folder other than the one
// read, relative to the folder.
func skillFiles(folder, read string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(folder, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.Type().IsRegular() {
			return err
		}
		rel, err := filepath.Rel(folder, name)
		if err != nil {
			return err
		}
		if rel = filepath.ToSlash(rel); rel != read {
			files = append(files, rel)
		}
		return nil
	})
	return files, err
}
