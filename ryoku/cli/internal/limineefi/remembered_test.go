package limineefi

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	oldGUIDEntry = "LimineLastBootedEntry-513ee0d0-6e43-cb05-b272-f146a2fcb88a"
	newGUIDEntry = "LimineLastBootedEntry-03884334-1756-4978-9c8c-86fb8998b4ea"
	unrelatedVar = "SecureBoot-8be4df61-93ca-11d2-aa0d-00e098032b8c"
)

var rememberedLayouts = []struct {
	name    string
	present []string
}{
	{name: "old guid only", present: []string{oldGUIDEntry}},
	{name: "new guid only", present: []string{newGUIDEntry}},
	{name: "both guids", present: []string{oldGUIDEntry, newGUIDEntry}},
	{name: "neither", present: nil},
}

func fakeEFIVars(t *testing.T, present []string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range append([]string{unrelatedVar}, present...) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("Ryoku Linux"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func assertOnlyUnrelatedLeft(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var left []string
	for _, entry := range entries {
		left = append(left, entry.Name())
	}
	if !slices.Equal(left, []string{unrelatedVar}) {
		t.Fatalf("efivars after clear = %v, want only %s", left, unrelatedVar)
	}
}

func TestClearLastBootedEntryRemovesEveryGUID(t *testing.T) {
	for _, layout := range rememberedLayouts {
		t.Run(layout.name, func(t *testing.T) {
			dir := fakeEFIVars(t, layout.present)
			if err := ClearLastBootedEntry(dir); err != nil {
				t.Fatal(err)
			}
			assertOnlyUnrelatedLeft(t, dir)
		})
	}
}

func TestClearLastBootedEntryWithSudoRemovesEveryGUID(t *testing.T) {
	for _, layout := range rememberedLayouts {
		t.Run(layout.name, func(t *testing.T) {
			dir := fakeEFIVars(t, layout.present)
			var commands []string
			sudo := func(args ...string) error {
				commands = append(commands, strings.Join(args, " "))
				if args[0] == "rm" {
					return os.Remove(args[len(args)-1])
				}
				return nil
			}
			if err := ClearLastBootedEntryWithSudo(dir, sudo); err != nil {
				t.Fatal(err)
			}
			assertOnlyUnrelatedLeft(t, dir)
			var want []string
			for _, name := range layout.present {
				path := filepath.Join(dir, name)
				want = append(want, "chattr -i "+path, "rm -f "+path)
			}
			slices.Sort(want)
			got := slices.Clone(commands)
			slices.Sort(got)
			if !slices.Equal(got, want) {
				t.Fatalf("privileged commands = %q, want %q", commands, want)
			}
			for i := 0; i+1 < len(commands); i += 2 {
				if !strings.HasPrefix(commands[i], "chattr -i ") || commands[i+1] != "rm -f "+strings.TrimPrefix(commands[i], "chattr -i ") {
					t.Fatalf("privileged commands = %q, want immutable flag cleared before each removal", commands)
				}
			}
		})
	}
}
