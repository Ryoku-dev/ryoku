package collections

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestSavedCollectionAndRestart(t *testing.T) {
	s := &Store{Dir: filepath.Join(t.TempDir(), "collections")}
	list, err := s.List()
	if err != nil || len(list) != 1 || list[0].ID != Default {
		t.Fatalf("default: %v %v", list, err)
	}
	item, err := s.Add(protocol.CollectionAddParams{Kind: "answer", Text: "Keep this answer", Source: "theia"})
	if err != nil {
		t.Fatal(err)
	}
	reopened := &Store{Dir: s.Dir}
	items, err := reopened.Items("")
	if err != nil || len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("reload: %v %v", items, err)
	}
	if _, err := reopened.Save(protocol.CollectionSaveParams{ID: Default, Name: "Reading"}); err != nil {
		t.Fatal(err)
	}
	if err := reopened.Remove(Default, item.ID); err != nil {
		t.Fatal(err)
	}
	items, err = reopened.Items(Default)
	if err != nil || len(items) != 0 {
		t.Fatalf("remove: %v %v", items, err)
	}
}

func TestCopiedFileAndChangedCheck(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(source, []byte("the saved passage"), 0600); err != nil {
		t.Fatal(err)
	}
	s := &Store{Dir: filepath.Join(dir, "collections")}
	item, err := s.Add(protocol.CollectionAddParams{Kind: "file", Path: source, Text: "saved passage"})
	if err != nil {
		t.Fatal(err)
	}
	if item.Path == source {
		t.Fatal("source was not copied")
	}
	if _, err = os.Stat(item.Path); err != nil {
		t.Fatal(err)
	}
	checked, err := s.Check(context.Background(), Default, item.ID)
	if err != nil || checked.Changed {
		t.Fatalf("unchanged: %v %v", checked, err)
	}
	if err = os.WriteFile(source, []byte("a different passage"), 0600); err != nil {
		t.Fatal(err)
	}
	checked, err = s.Check(context.Background(), Default, item.ID)
	if err != nil || !checked.Changed || checked.Checked == "" {
		t.Fatalf("changed: %v %v", checked, err)
	}
	if err = s.Remove(Default, item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(item.Path); !os.IsNotExist(err) {
		t.Fatalf("copied file remains: %v", err)
	}
	if _, err = os.Stat(source); err != nil {
		t.Fatal("source was removed")
	}
}

func TestCollectionBounds(t *testing.T) {
	s := &Store{Dir: filepath.Join(t.TempDir(), "collections")}
	if _, err := s.Items("../escape"); err == nil {
		t.Fatal("accepted traversal")
	}
	if _, err := s.Save(protocol.CollectionSaveParams{Name: " "}); err == nil {
		t.Fatal("accepted blank name")
	}
	if _, err := s.Add(protocol.CollectionAddParams{Kind: "answer", Text: strings.Repeat("x", (1<<20)+1)}); err == nil {
		t.Fatal("accepted oversized text")
	}
	file := filepath.Join(t.TempDir(), "large")
	f, err := os.Create(file)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Truncate(MaxFile + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err = s.Add(protocol.CollectionAddParams{Kind: "file", Path: file}); err == nil {
		t.Fatal("accepted oversized file")
	}
}

func TestChatBudgetAndAttachments(t *testing.T) {
	text, files := Chat([]protocol.CollectionItem{{Title: "first", Text: strings.Repeat("a", 48<<10)}, {Title: "second", Text: "not inlined", Path: "/tmp/note", Image: "/tmp/note"}}, "What connects them?")
	if strings.Contains(text, "\nnot inlined") || !strings.Contains(text, "## second") || !strings.HasSuffix(text, "What connects them?") {
		t.Fatal("chat lost its budget, item name or question")
	}
	if len(files) != 1 || files[0].Path != "/tmp/note" {
		t.Fatalf("attachments: %v", files)
	}
}
