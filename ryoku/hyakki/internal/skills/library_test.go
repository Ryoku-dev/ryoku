package skills

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// fixture is a library over testdata/sets, written out under a folder of
// the test's own.
func fixture(t *testing.T) *Library {
	t.Helper()
	return &Library{
		files: os.DirFS("testdata/sets"),
		catalog: []set{
			{id: "alpha", name: "Alpha", summary: "The first fixture set."},
			{id: "beta", name: "Beta", summary: "The second fixture set.", needs: []need{{command: "rashin-fixture-missing", install: "get it"}}},
		},
		dir: filepath.Join(t.TempDir(), "skills"),
	}
}

func TestIndexReadsNamesDescriptionsAndManual(t *testing.T) {
	result, err := fixture(t).List(nil)
	if err != nil {
		t.Fatal(err)
	}
	var got []protocol.Skill
	for _, s := range result.Sets {
		if !s.On {
			t.Errorf("%s is off with nothing turned off", s.ID)
		}
		got = append(got, s.Skills...)
	}
	want := []protocol.Skill{
		{ID: "alpha/plan", Name: "plan", Set: "alpha", Description: "Use when a task has many steps. Plans them first."},
		{ID: "alpha/tdd", Name: "tdd", Set: "alpha", Description: "Use when implementing any feature, before writing the code."},
		{ID: "beta/review", Name: "review", Set: "beta", Description: `Reviews a design document with fresh eyes, "goldfish" style.`, Manual: true},
	}
	if len(got) != len(want) {
		t.Fatalf("skills = %+v", got)
	}
	for i := range want {
		want[i].Path = filepath.Join(result.Root, want[i].Set, "skills", want[i].Name, "SKILL.md")
		if got[i] != want[i] {
			t.Errorf("skill %d = %+v, want %+v", i, got[i], want[i])
		}
		if _, err := os.Stat(got[i].Path); err != nil {
			t.Errorf("%s is not on disk: %v", got[i].Path, err)
		}
	}
}

func TestTreeIsWrittenOncePerHashAndOldOnesGo(t *testing.T) {
	library := fixture(t)
	old := filepath.Join(library.dir, "0123456789abcdef")
	unfinished := filepath.Join(library.dir, ".writing-123")
	for _, dir := range []string{old, unfinished} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	root, err := library.Root()
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(library.dir)
	if len(entries) != 1 || filepath.Join(library.dir, entries[0].Name()) != root {
		t.Fatalf("skills folder holds %v, want only %s", entries, root)
	}
	script, err := os.Stat(filepath.Join(root, "alpha/skills/plan/notes/run.sh"))
	if err != nil || script.Mode().Perm() != 0o755 {
		t.Errorf("script mode = %v, %v", script, err)
	}
	notes, _ := os.Stat(filepath.Join(root, "alpha/skills/plan/notes/more.md"))
	if notes.Mode().Perm() != 0o644 {
		t.Errorf("notes mode = %v", notes.Mode())
	}

	// a second library over the same tree, as after a restart, finds it
	// written and writes nothing
	marker := filepath.Join(root, "marker")
	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	again := fixture(t)
	again.dir = library.dir
	if second, err := again.Root(); err != nil || second != root {
		t.Fatalf("second root = %s, %v; want %s", second, err, root)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Error("the tree was written again over the one already there")
	}

	// removed behind its back, it is written again
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if third, err := again.Root(); err != nil || third != root {
		t.Fatalf("third root = %s, %v", third, err)
	}
	if _, err := os.Stat(filepath.Join(root, "alpha/skills/tdd/SKILL.md")); err != nil {
		t.Error("the removed tree was not written again")
	}
}

func TestChangedTreeGetsANewFolderAndTheOldOneGoes(t *testing.T) {
	source := t.TempDir()
	write := func(text string) {
		dir := filepath.Join(source, "alpha", "skills", "one")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dir := filepath.Join(t.TempDir(), "skills")
	open := func() string {
		library := &Library{files: os.DirFS(source), catalog: []set{{id: "alpha"}}, dir: dir}
		root, err := library.Root()
		if err != nil {
			t.Fatal(err)
		}
		return root
	}
	write("---\nname: one\ndescription: first\n---\n")
	first := open()
	write("---\nname: one\ndescription: second\n---\n")
	second := open()
	if first == second {
		t.Fatal("a changed tree kept its folder")
	}
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Errorf("the old tree is still there: %v", err)
	}
}

func TestGetReadsOnlyInsideTheSkill(t *testing.T) {
	library := fixture(t)
	content, err := library.Get(protocol.SkillParams{ID: "alpha/plan"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if content.File != "SKILL.md" || !strings.HasPrefix(content.Text, "---\nname: plan\n") || !slices.Equal(content.Files, []string{"notes/more.md", "notes/run.sh"}) {
		t.Errorf("content = %+v", content)
	}
	if more, err := library.Get(protocol.SkillParams{ID: "alpha/plan", File: "notes/more.md"}, nil); err != nil || more.Text != "More about plans.\n" {
		t.Errorf("notes/more.md = %+v, %v", more, err)
	}
	root, _ := library.Root()
	folder := filepath.Join(root, "alpha/skills/plan")
	if err := os.Symlink(filepath.Join(root, "beta/shared.md"), filepath.Join(folder, "link.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "beta"), filepath.Join(folder, "linked")); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"../../../beta/shared.md", "../tdd/SKILL.md", "notes/../SKILL.md", "/etc/passwd", "link.md", "linked/shared.md", "notes"} {
		if got, err := library.Get(protocol.SkillParams{ID: "alpha/plan", File: file}, nil); err == nil {
			t.Errorf("%s was read: %q", file, got.Text)
		}
	}
	for _, id := range []string{"alpha/missing", "../alpha/plan", "alpha", "gamma/plan"} {
		if _, err := library.Get(protocol.SkillParams{ID: id}, nil); err == nil {
			t.Errorf("%s was read", id)
		}
	}
}

func TestATurnedOffSetIsAbsent(t *testing.T) {
	library := fixture(t)
	off := []string{"beta"}
	result, err := library.List(off)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range result.Sets {
		if s.On == (s.ID == "beta") {
			t.Errorf("%s on = %v", s.ID, s.On)
		}
	}
	found, err := library.Search(protocol.SkillsSearchParams{Query: "review a design document"}, off)
	if err != nil {
		t.Fatal(err)
	}
	listed, _ := library.Search(protocol.SkillsSearchParams{}, off)
	for _, s := range append(found.Skills, listed.Skills...) {
		if s.Set == "beta" {
			t.Errorf("search found %s of a set that is off", s.ID)
		}
	}
	if len(listed.Skills) != 2 {
		t.Errorf("an empty search lists %d skills, want the 2 of alpha", len(listed.Skills))
	}
	if _, err := library.Get(protocol.SkillParams{ID: "beta/review"}, off); err == nil {
		t.Error("a skill of a set that is off was read")
	}
	if on, _ := library.Search(protocol.SkillsSearchParams{Query: "review a design document"}, nil); len(on.Skills) == 0 || on.Skills[0].ID != "beta/review" {
		t.Errorf("with beta on, search = %+v", on.Skills)
	}
}

func TestNeedsSayWhatIsMissing(t *testing.T) {
	result, err := fixture(t).List(nil)
	if err != nil {
		t.Fatal(err)
	}
	needs := result.Sets[1].Needs
	if len(needs) != 1 || needs[0] != (protocol.SkillNeed{Command: "rashin-fixture-missing", Present: false, Install: "get it"}) {
		t.Errorf("needs = %+v", needs)
	}
}
