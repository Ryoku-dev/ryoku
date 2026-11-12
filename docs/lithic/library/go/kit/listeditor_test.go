package kit

import "testing"

func TestListKeyMove(t *testing.T) {
	cases := []struct {
		index, count int
		up           bool
		to           int
		ok           bool
	}{
		{0, 3, true, 0, false},
		{1, 3, true, 0, true},
		{1, 3, false, 2, true},
		{2, 3, false, 2, false},
		{0, 1, false, 0, false},
	}
	for _, test := range cases {
		to, ok := listKeyMove(test.index, test.count, test.up)
		if to != test.to || ok != test.ok {
			t.Errorf("listKeyMove(%d, %d, %v) = %d %v, want %d %v", test.index, test.count, test.up, to, ok, test.to, test.ok)
		}
	}
}

func TestListMoveTarget(t *testing.T) {
	// three 44 rows 13 apart: tops 0, 57, 114
	tops, heights := []int{0, 57, 114}, []int{44, 44, 44}
	cases := []struct {
		from, offset, want int
	}{
		{0, 0, 0},
		{0, 30, 0},
		{0, 60, 1},
		{0, 200, 2},
		{2, -60, 1},
		{2, -120, 0},
		{1, -500, 0},
		{1, 10, 1},
	}
	for _, test := range cases {
		if got := listMoveTarget(tops, heights, test.from, test.offset); got != test.want {
			t.Errorf("listMoveTarget(from %d, offset %d) = %d, want %d", test.from, test.offset, got, test.want)
		}
	}
}

func TestListEditorEvents(t *testing.T) {
	var list ListEditor
	list.ensure(3)

	if _, open := list.Editing(); open {
		t.Fatal("zero list editor has an open editor")
	}
	list.Add()
	if index, ok := list.Opened(); !ok || index != ListEditorNew {
		t.Fatalf("Add opened %d %v", index, ok)
	}
	if _, ok := list.Opened(); ok {
		t.Fatal("Opened is not clear-on-read")
	}
	if list.Submit() {
		t.Fatal("Submit accepted an invalid editor")
	}
	list.EditorValid = true
	if !list.Submit() || !list.Added() || list.Added() {
		t.Fatal("a valid new item should report Added once")
	}
	if _, open := list.Editing(); open {
		t.Fatal("Submit left the editor open")
	}

	list.Edit(1)
	list.Edit(2)
	if index, open := list.Editing(); !open || index != 2 {
		t.Fatalf("opening another editor should replace the first, got %d %v", index, open)
	}
	if !list.Submit() {
		t.Fatal("Submit refused a valid edit")
	}
	if index, ok := list.Edited(); !ok || index != 2 {
		t.Fatalf("Edited = %d %v", index, ok)
	}
	if list.Added() {
		t.Fatal("an edit reported an addition")
	}

	list.Edit(0)
	if !list.Cancel() || !list.Cancelled() || list.Cancelled() {
		t.Fatal("Cancel should close and report once")
	}
	if list.Cancel() {
		t.Fatal("Cancel with no editor reported closing one")
	}
	if _, ok := list.Edited(); ok {
		t.Fatal("Cancel reported an edit")
	}

	list.Edit(2)
	list.removeAt(0, 3)
	if index, ok := list.Removed(); !ok || index != 0 {
		t.Fatalf("Removed = %d %v", index, ok)
	}
	if index, open := list.Editing(); !open || index != 1 {
		t.Fatalf("removing an earlier item should keep the editor on its item, got %d %v", index, open)
	}
	list.removeAt(1, 2)
	if _, open := list.Editing(); open {
		t.Fatal("removing the edited item should close its editor")
	}

	list.moveItem(2, 0)
	if from, to, ok := list.Moved(); !ok || from != 2 || to != 0 {
		t.Fatalf("Moved = %d %d %v", from, to, ok)
	}
	if _, _, ok := list.Moved(); ok {
		t.Fatal("Moved is not clear-on-read")
	}
}

func TestPathField(t *testing.T) {
	if got := pathPlaceholder(PathFolder, ""); got != "Choose a folder" {
		t.Errorf("folder placeholder = %q", got)
	}
	if got := pathPlaceholder(PathFile, ""); got != "Choose a file" {
		t.Errorf("file placeholder = %q", got)
	}
	if got := pathPlaceholder(PathFile, "Choose a wallpaper"); got != "Choose a wallpaper" {
		t.Errorf("host placeholder = %q", got)
	}
	field := PathField{Path: "/home/ops/Pictures"}
	field.commit("/home/ops/Pictures")
	if _, ok := field.Edited(); ok {
		t.Error("committing the unchanged path reported an edit")
	}
	field.commit("/home/ops/Wallpapers")
	if path, ok := field.Edited(); !ok || path != "/home/ops/Wallpapers" {
		t.Errorf("Edited = %q %v", path, ok)
	}
	if _, ok := field.Edited(); ok {
		t.Error("Edited is not clear-on-read")
	}
}
