package diffview

import (
	"image"
	"testing"
	"time"

	"gioui.org/f32"
	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/theme"

	"ryoku-rashin/internal/protocol"
)

func TestFromTextsLineNumbers(t *testing.T) {
	tests := []struct {
		name      string
		oldText   string
		newText   string
		wantKinds []string
		wantOld   []int
		wantNew   []int
	}{
		{
			name: "insert", oldText: "a\nc\n", newText: "a\nb\nc\n",
			wantKinds: []string{" ", "+", " "},
			wantOld:   []int{1, 0, 2},
			wantNew:   []int{1, 2, 3},
		},
		{
			name: "delete", oldText: "a\nb\nc\n", newText: "a\nc\n",
			wantKinds: []string{" ", "-", " "},
			wantOld:   []int{1, 2, 3},
			wantNew:   []int{1, 0, 2},
		},
		{
			name: "replace", oldText: "a\nb\nc\n", newText: "a\nx\nc\n",
			wantKinds: []string{" ", "-", "+", " "},
			wantOld:   []int{1, 2, 0, 3},
			wantNew:   []int{1, 0, 2, 3},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file := FromTexts("file.go", test.oldText, test.newText)
			if len(file.Hunks) != 1 {
				t.Fatalf("hunks = %d, want 1", len(file.Hunks))
			}
			lines := file.Hunks[0].Lines
			if len(lines) != len(test.wantKinds) {
				t.Fatalf("lines = %#v", lines)
			}
			for index, line := range lines {
				if line.Kind != test.wantKinds[index] || line.Old != test.wantOld[index] || line.New != test.wantNew[index] {
					t.Fatalf("line %d = %#v, want kind %q old %d new %d", index, line, test.wantKinds[index], test.wantOld[index], test.wantNew[index])
				}
			}
		})
	}
}

type diffFrame struct {
	ops    op.Ops
	router input.Router
	now    time.Time
	theme  *theme.Theme
}

func newDiffFrame() *diffFrame {
	return &diffFrame{now: time.Unix(100, 0), theme: theme.New(palette.Signature(true))}
}

func (f *diffFrame) context() layout.Context {
	f.ops.Reset()
	return layout.Context{
		Ops:    &f.ops,
		Now:    f.now,
		Source: f.router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(
			image.Pt(760, 420),
		),
	}
}

func (f *diffFrame) finish() {
	f.router.Frame(&f.ops)
	f.now = f.now.Add(time.Millisecond)
}

func TestCommentGutterProducesActionThroughRouter(t *testing.T) {
	frame := newDiffFrame()
	var view View
	result := protocol.DiffResult{Files: []protocol.FileDiff{FromTexts("file.go", "a\n", "a\nb\n")}}
	render := func() {
		view.Layout(frame.context(), frame.theme, result, Options{Commentable: true, Wrap: true})
		frame.finish()
	}
	render()
	point := f32.Pt(28, 66)
	frame.router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, PointerID: 1, Buttons: pointer.ButtonPrimary, Position: point})
	render()
	frame.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, PointerID: 1, Position: point})
	render()
	if view.commentLine == 0 {
		t.Fatal("gutter click did not open the comment field")
	}
	view.commentField.SetText("Please keep this name")
	frame.router.Queue(key.Event{Name: key.NameReturn, State: key.Press})
	render()
	actions := view.Actions()
	if len(actions) != 1 || actions[0].Kind != ActComment || actions[0].Path != "file.go" || actions[0].Text != "Please keep this name" {
		t.Fatalf("actions = %#v", actions)
	}
}
