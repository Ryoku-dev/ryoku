package skills

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// The built-in tree, as vendored: how many skills each set brings and
// whether its authors mean them to be started only by name.
func TestRealTreeIndexesEverySet(t *testing.T) {
	result, err := New(filepath.Join(t.TempDir(), "skills")).List(nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct {
		count  int
		manual bool
	}{"superpowers": {14, false}, "openspec": {12, false}, "speckit": {9, false}, "egm": {7, true}}
	if len(result.Sets) != len(want) {
		t.Fatalf("sets = %d", len(result.Sets))
	}
	for _, s := range result.Sets {
		w := want[s.ID]
		if len(s.Skills) != w.count {
			t.Errorf("%s has %d skills, want %d", s.ID, len(s.Skills), w.count)
		}
		if s.Name == "" || s.By == "" || s.License == "" || s.Commit == "" || s.Summary == "" || s.URL == "" {
			t.Errorf("%s lacks its metadata: %+v", s.ID, s)
		}
		for _, skill := range s.Skills {
			body, err := os.ReadFile(skill.Path)
			if err != nil {
				t.Fatalf("%s: %v", skill.ID, err)
			}
			if name := frontmatter(string(body))["name"]; name != skill.Name {
				t.Errorf("%s says its name is %q", skill.ID, name)
			}
			if skill.Description == "" || skill.Manual != w.manual {
				t.Errorf("%s = %+v", skill.ID, skill)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(result.Root, "superpowers/skills/diagnosing-superpowers")); err == nil {
		t.Error("diagnosing-superpowers is vendored")
	}
}

func TestRealTreeHasNoLinks(t *testing.T) {
	err := fs.WalkDir(tree, ".", func(name string, entry fs.DirEntry, err error) error {
		if err == nil && entry.Type()&fs.ModeSymlink != 0 {
			t.Errorf("%s is a link", name)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(tree, path.Join("sets", "speckit", ".specify", "templates")); err != nil {
		t.Errorf("Spec Kit's payload is missing: %v", err)
	}
}

func TestSearchFindsTestDrivenDevelopment(t *testing.T) {
	found, err := New(filepath.Join(t.TempDir(), "skills")).Search(protocol.SkillsSearchParams{Query: "write a failing test first"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(found.Skills) == 0 || found.Skills[0].ID != "superpowers/test-driven-development" {
		t.Fatalf("search = %+v", found.Skills)
	}
}

func TestOpenSpecMissingShowsInNeeds(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	result, err := New(filepath.Join(t.TempDir(), "skills")).List(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range result.Sets {
		if s.ID != "openspec" {
			continue
		}
		if len(s.Needs) != 1 || s.Needs[0] != (protocol.SkillNeed{Command: "openspec", Install: "npm install -g @fission-ai/openspec"}) {
			t.Fatalf("needs = %+v", s.Needs)
		}
		return
	}
	t.Fatal("no openspec set")
}

func TestOpenSpecPresentShowsInNeeds(t *testing.T) {
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "openspec"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	result, err := New(filepath.Join(t.TempDir(), "skills")).List(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range result.Sets {
		if s.ID == "openspec" && (len(s.Needs) != 1 || !s.Needs[0].Present) {
			t.Fatalf("needs = %+v", s.Needs)
		}
	}
}
