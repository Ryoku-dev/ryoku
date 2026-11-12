package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

const skillsUsage = "skills list | search <words> [--limit n] | read <set/name> [file] | setup <set> [dir], with [--format human|json|toon] for list and search"

func init() {
	register(&Command{Name: "skills", Group: "skills", Usage: "list|search|read|setup", Summary: i18n.T("List, search and read the skills Rashin ships, and set one up in a project."), Run: runSkills})
}

// setupPayloads are the folders a set puts in a project before its skills
// work there, by set: what Spec Kit's own `specify init` would put there
var setupPayloads = map[string]string{"speckit": ".specify"}

func runSkills(args []string) int {
	if len(args) == 0 {
		return usageError(skillsUsage)
	}
	verb, rest := args[0], args[1:]
	format, limit := defaultOutputFormat(), 0
	var words []string
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--format", "--limit":
			if i+1 == len(rest) {
				return usageError(skillsUsage)
			}
			if rest[i] == "--format" {
				format = rest[i+1]
			} else if n, err := strconv.Atoi(rest[i+1]); err == nil && n > 0 {
				limit = n
			} else {
				return usageError(skillsUsage)
			}
			i++
		default:
			if strings.HasPrefix(rest[i], "--") {
				return usageError(skillsUsage)
			}
			words = append(words, rest[i])
		}
	}
	switch {
	case verb == "list" && len(words) == 0:
		var result protocol.SkillsResult
		return skillsOutput(protocol.MSkillsList, protocol.Empty{}, &result, format, func(w io.Writer) { printSkillSets(w, result) })
	case verb == "search" && len(words) > 0:
		var result protocol.SkillsSearchResult
		params := protocol.SkillsSearchParams{Query: strings.Join(words, " "), Limit: limit}
		return skillsOutput(protocol.MSkillsSearch, params, &result, format, func(w io.Writer) { printSkills(w, result.Skills) })
	case verb == "read" && (len(words) == 1 || len(words) == 2):
		params := protocol.SkillParams{ID: words[0]}
		if len(words) == 2 {
			params.File = words[1]
		}
		var result protocol.SkillContent
		if err := daemonCall(context.Background(), protocol.MSkillsGet, params, &result); err != nil {
			return report(err)
		}
		fmt.Fprint(os.Stdout, result.Text)
		return 0
	case verb == "setup" && (len(words) == 1 || len(words) == 2):
		dir := "."
		if len(words) == 2 {
			dir = words[1]
		}
		return setupSkills(words[0], dir)
	}
	return usageError(skillsUsage)
}

// skillsOutput calls a skills method and prints its answer, human through
// human
func skillsOutput(method string, params, result any, format string, human func(io.Writer)) int {
	if err := daemonCall(context.Background(), method, params, result); err != nil {
		return report(err)
	}
	if format == "human" {
		human(os.Stdout)
		return 0
	}
	return printFormatted(result, format)
}

func printSkillSets(w io.Writer, result protocol.SkillsResult) {
	for _, set := range result.Sets {
		state := i18n.T("on")
		if !set.On {
			state = i18n.T("off")
		}
		fmt.Fprintf(w, "%s (%s)  %s  %s  %s\n", set.Name, set.ID, i18n.Tf("by %s", set.By), set.License, state)
		fmt.Fprintf(w, "  %s\n", set.Summary)
		for _, need := range set.Needs {
			if !need.Present {
				fmt.Fprintf(w, "  %s\n", i18n.Tf("Needs %s, which is missing: %s", need.Command, need.Install))
			}
		}
		printSkills(w, set.Skills)
	}
	fmt.Fprintln(w, i18n.Tf("Skills lie in %s", result.Root))
}

func printSkills(w io.Writer, skills []protocol.Skill) {
	for _, skill := range skills {
		fmt.Fprintf(w, "  %s  %s", skill.ID, skill.Description)
		if skill.Manual {
			fmt.Fprintf(w, "  (%s)", i18n.T("when you ask"))
		}
		fmt.Fprintln(w)
	}
}

// setupSkills puts a set's payload into the project at dir, as the set's own
// installer would, and refuses when the project already has one
func setupSkills(id, dir string) int {
	payload, ok := setupPayloads[id]
	if !ok {
		return report(errors.New(i18n.Tf("The skill set %s needs no setup.", id)))
	}
	var result protocol.SkillsResult
	if err := daemonCall(context.Background(), protocol.MSkillsList, protocol.Empty{}, &result); err != nil {
		return report(err)
	}
	var name string
	for _, set := range result.Sets {
		if set.ID == id {
			if !set.On {
				return report(errors.New(i18n.Tf("%s is turned off in Rashin > Settings > Skills.", set.Name)))
			}
			name = set.Name
		}
	}
	if name == "" {
		return report(errors.New(i18n.Tf("No skill set %s.", id)))
	}
	project, err := filepath.Abs(dir)
	if err != nil {
		return report(err)
	}
	if info, err := os.Stat(project); err != nil || !info.IsDir() {
		return report(errors.New(i18n.Tf("%s is not a folder.", project)))
	}
	target := filepath.Join(project, payload)
	if err := copyTree(filepath.Join(result.Root, id, payload), target); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return report(errors.New(i18n.Tf("%s already exists; nothing was changed.", target)))
		}
		return report(err)
	}
	fmt.Fprintln(os.Stdout, i18n.Tf("%s is set up in %s.", name, target))
	return 0
}

// copyTree copies the folder from to to, which must not exist yet, keeping
// each file's mode; a copy that fails part way is removed
func copyTree(from, to string) error {
	if err := os.Mkdir(to, 0o755); err != nil {
		return err
	}
	err := filepath.WalkDir(from, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, name)
		if err != nil || rel == "." {
			return err
		}
		target := filepath.Join(to, rel)
		if entry.IsDir() {
			return os.Mkdir(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		body, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, info.Mode().Perm())
	})
	if err != nil {
		_ = os.RemoveAll(to)
	}
	return err
}
