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

	"ryoku-rashin/internal/app/look"
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
	theme  *look.Theme
	view   View
	result protocol.DiffResult
}

func newDiffFrame(files ...protocol.FileDiff) *diffFrame {
	return &diffFrame{now: time.Unix(100, 0), theme: look.New(0, palette.Look{}), result: protocol.DiffResult{Files: files}}
}

func (f *diffFrame) layout(options Options) {
	f.ops.Reset()
	gtx := layout.Context{
		Ops: &f.ops, Now: f.now, Source: f.router.Source(),
		Metric: unit.Metric{PxPerDp: 1, PxPerSp: 1}, Constraints: layout.Exact(image.Pt(760, 420)),
	}
	f.view.Layout(gtx, f.theme, f.result, options)
	f.router.Frame(&f.ops)
	f.now = f.now.Add(time.Millisecond)
}

func (f *diffFrame) click(at f32.Point, options Options) {
	f.router.Queue(pointer.Event{Kind: pointer.Press, Source: pointer.Mouse, PointerID: 1, Buttons: pointer.ButtonPrimary, Position: at})
	f.layout(options)
	f.router.Queue(pointer.Event{Kind: pointer.Release, Source: pointer.Mouse, PointerID: 1, Position: at})
	f.layout(options)
}

func TestGutterPressOpensACommentThatEnterSends(t *testing.T) {
	frame := newDiffFrame(FromTexts("file.go", "a\n", "a\nb\n"))
	options := Options{Commentable: true}
	frame.layout(options)
	// walk down the gutter until a line takes the press so this doesn't depend on the rows above
	for y := float32(4); y < 200 && frame.view.commentLine == 0; y += 6 {
		frame.click(f32.Pt(8, y), options)
	}
	if frame.view.commentLine == 0 {
		t.Fatal("no line's gutter opened a comment field")
	}
	line := frame.view.commentLine
	frame.view.commentField.SetText("Please keep this name")
	frame.router.Queue(key.Event{Name: key.NameReturn, State: key.Press})
	frame.layout(options)
	actions := frame.view.Actions()
	if len(actions) != 1 || actions[0].Kind != ActComment || actions[0].Path != "file.go" ||
		actions[0].Line != line || actions[0].Text != "Please keep this name" {
		t.Fatalf("actions = %#v", actions)
	}
	if frame.view.commentLine != 0 {
		t.Fatal("the comment field stayed open after it sent")
	}
}

func TestGutterIsInertWhereCommentsAreNotTaken(t *testing.T) {
	frame := newDiffFrame(FromTexts("file.go", "a\n", "a\nb\n"))
	frame.layout(Options{})
	for y := float32(4); y < 200; y += 6 {
		frame.click(f32.Pt(8, y), Options{})
	}
	if frame.view.commentLine != 0 {
		t.Fatal("a diff that takes no comments opened a comment field")
	}
}

func TestCollapsedFileUnfoldsWhenItsRowIsPressed(t *testing.T) {
	frame := newDiffFrame(FromTexts("file.go", "a\n", "a\nb\n"))
	options := Options{Collapsed: true}
	frame.layout(options)
	if frame.view.unfolded("file.go", options) {
		t.Fatal("a collapsed diff started with its file unfolded")
	}
	frame.click(f32.Pt(200, 12), options)
	if !frame.view.unfolded("file.go", options) {
		t.Fatal("pressing a file's row did not unfold it")
	}
}
