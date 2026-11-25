package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func TestPromptLibraryLifecycle(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	items, err := List()
	if err != nil || len(items) != 14 {
		t.Fatalf("built-ins = %d, %v", len(items), err)
	}
	if _, err := os.Stat(filepath.Join(paths.DataDir(), "prompts.json")); !os.IsNotExist(err) {
		t.Fatalf("listing created a store: %v", err)
	}
	item, err := Save(protocol.PromptSaveParams{Name: "Weekly update", Kind: "template", Text: "Hello {{recipient}}, {{content}}"})
	if err != nil || item.ID == "" || item.BuiltIn || item.Updated == "" {
		t.Fatalf("save = %#v, %v", item, err)
	}
	stat, err := os.Stat(filepath.Join(paths.DataDir(), "prompts.json"))
	if err != nil {
		t.Fatal(err)
	}
	if stat.Mode().Perm() != 0600 {
		t.Fatalf("store permissions = %o", stat.Mode().Perm())
	}
	text, err := ExpandID(protocol.PromptExpandParams{ID: item.ID, Vars: map[string]string{"recipient": "Mina", "content": "the update"}})
	if err != nil || text != "Hello Mina, the update" {
		t.Fatalf("expand = %q, %v", text, err)
	}
	updated, err := Save(protocol.PromptSaveParams{ID: item.ID, Name: "Status", Kind: "snippet", Text: "Ready"})
	if err != nil || updated.ID != item.ID || updated.Name != "Status" {
		t.Fatalf("update = %#v, %v", updated, err)
	}
	items, err = List()
	if err != nil || len(items) != 15 || items[14].Name != "Status" {
		t.Fatalf("list after update = %#v, %v", items, err)
	}
	if err := Delete(item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := ExpandID(protocol.PromptExpandParams{ID: item.ID}); err == nil {
		t.Fatal("deleted prompt still expands")
	}
}

func TestBuiltInsStayReadOnly(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	for _, item := range BuiltIns() {
		if _, err := Save(protocol.PromptSaveParams{ID: item.ID, Name: item.Name, Kind: item.Kind, Text: item.Text}); err == nil {
			t.Fatalf("saved built-in %s", item.ID)
		}
		if err := Delete(item.ID); err == nil {
			t.Fatalf("deleted built-in %s", item.ID)
		}
		vars := map[string]string{}
		for _, name := range Variables(item.Text) {
			vars[name] = "sample"
		}
		if _, err := Expand(item, vars); err != nil {
			t.Fatalf("expand built-in %s: %v", item.ID, err)
		}
	}
	copy := BuiltIns()[0]
	if _, err := Save(protocol.PromptSaveParams{Name: copy.Name, Kind: copy.Kind, Text: copy.Text}); err != nil {
		t.Fatalf("copy built-in: %v", err)
	}
}

func TestVariableExpansionIsLiteral(t *testing.T) {
	item := protocol.Prompt{Text: "{{ content }} | {{content}} | {{lang}} | {{recipient}} | {{date}}"}
	if got := strings.Join(Variables(item.Text), ","); got != "content,lang,recipient,date" {
		t.Fatalf("variables = %q", got)
	}
	if _, err := Expand(item, map[string]string{"content": "one"}); err == nil {
		t.Fatal("missing variable accepted")
	}
	text, err := Expand(item, map[string]string{"content": "{{recipient}} $1", "lang": "English", "recipient": "Ren", "date": "2026-11-18"})
	if err != nil || text != "{{recipient}} $1 | {{recipient}} $1 | English | Ren | 2026-11-18" {
		t.Fatalf("literal expansion = %q, %v", text, err)
	}
}

func TestBrokenLibraryIsNotOverwritten(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if err := os.MkdirAll(paths.DataDir(), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(paths.DataDir(), "prompts.json")
	if err := os.WriteFile(path, []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Save(protocol.PromptSaveParams{Name: "Note", Kind: "snippet", Text: "Keep this"}); err == nil {
		t.Fatal("saved over broken library")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "broken" {
		t.Fatalf("broken store changed: %q, %v", data, err)
	}
}

func TestExpansionLimitsOutput(t *testing.T) {
	item := protocol.Prompt{Text: strings.Repeat("{{content}}", 10)}
	if _, err := Expand(item, map[string]string{"content": strings.Repeat("x", 64<<10)}); err == nil {
		t.Fatal("accepted an oversized expansion")
	}
}
