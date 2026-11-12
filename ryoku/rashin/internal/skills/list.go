package skills

import (
	"os/exec"
	"path/filepath"
	"strings"

	"ryoku-rashin/internal/protocol"
)

// List is every built-in set with its skills, where they lie, what each set
// relies on and whether this computer has it. A set named in off is listed
// as off.
func (l *Library) List(off []string) (protocol.SkillsResult, error) {
	root, err := l.Root()
	if err != nil {
		return protocol.SkillsResult{}, err
	}
	result := protocol.SkillsResult{Root: root, Sets: make([]protocol.SkillSet, 0, len(l.catalog))}
	entries := l.entries()
	for _, s := range l.catalog {
		info := s.info()
		info.On = on(s.id, off)
		for _, n := range s.needs {
			_, missing := exec.LookPath(n.command)
			info.Needs = append(info.Needs, protocol.SkillNeed{Command: n.command, Present: missing == nil, Install: n.install})
		}
		info.Skills = []protocol.Skill{}
		for _, e := range entries {
			if e.set == s.id {
				info.Skills = append(info.Skills, e.skill(root))
			}
		}
		result.Sets = append(result.Sets, info)
	}
	return result, nil
}

// skillFile is where file of a skill lies under root.
func skillFile(root, set, name, file string) string {
	return filepath.Join(root, set, "skills", name, filepath.FromSlash(file))
}

// oneLine is text with its line breaks and runs of blanks made single
// spaces, as a description is shown on one line.
func oneLine(text string) string { return strings.Join(strings.Fields(text), " ") }
