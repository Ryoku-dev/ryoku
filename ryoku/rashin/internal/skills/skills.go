// Package skills serves the skill sets Rashin ships built in: Superpowers,
// OpenSpec, Spec Kit and Elephant and Goldfish, vendored unchanged under
// sets/. The first time they are used they are written out under Rashin's
// data dir, so Ask, agents and team sandboxes read them as files.
package skills

import (
	"embed"
	"io/fs"
	"path"
	"slices"
	"sort"
	"sync"

	"ryoku-rashin/internal/protocol"
)

//go:embed all:sets
var tree embed.FS

// Library is the built-in sets: their index, read once from the program,
// and the folder they are written out to.
type Library struct {
	files   fs.FS
	catalog []set
	dir     string

	indexOnce sync.Once
	index     []entry

	mu   sync.Mutex
	root string
}

// entry is one skill as the index holds it, without where it lies on disk.
type entry struct {
	set, name, description string
	manual                 bool
}

// New is the built-in sets, written out under dir when first used.
func New(dir string) *Library {
	files, err := fs.Sub(tree, "sets")
	if err != nil {
		panic(err)
	}
	return &Library{files: files, catalog: catalog, dir: dir}
}

// entries is the index: every skill of every set, sets in catalog order and
// skills by name. A folder whose SKILL.md cannot be read is no skill.
func (l *Library) entries() []entry {
	l.indexOnce.Do(func() {
		for _, s := range l.catalog {
			folders, _ := fs.ReadDir(l.files, path.Join(s.id, "skills"))
			var skills []entry
			for _, folder := range folders {
				if !folder.IsDir() {
					continue
				}
				body, err := fs.ReadFile(l.files, path.Join(s.id, "skills", folder.Name(), "SKILL.md"))
				if err != nil {
					continue
				}
				keys := frontmatter(string(body))
				skills = append(skills, entry{set: s.id, name: folder.Name(), description: oneLine(keys["description"]), manual: truthy(keys["disable-model-invocation"])})
			}
			sort.Slice(skills, func(i, j int) bool { return skills[i].name < skills[j].name })
			l.index = append(l.index, skills...)
		}
	})
	return l.index
}

// on reports whether the set id is on, given the sets turned off.
func on(id string, off []string) bool { return !slices.Contains(off, id) }

// skill is an entry as the protocol shows it, lying under root.
func (e entry) skill(root string) protocol.Skill {
	return protocol.Skill{
		ID: e.set + "/" + e.name, Name: e.name, Set: e.set, Description: e.description, Manual: e.manual,
		Path: skillFile(root, e.set, e.name, "SKILL.md"),
	}
}
