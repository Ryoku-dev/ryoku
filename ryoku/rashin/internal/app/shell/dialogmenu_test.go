package shell

import (
	"image"
	"testing"
)

// Dialogs draw last, above the shell's own layers. A select in one still opens
// its menu at the select, not where the press before the dialog landed.
func TestASelectInADialogOpensItsMenuAtTheSelect(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("vault")
	stage.click("Add credential")
	stage.click("How it is sent: As a bearer token")
	field, _ := stage.named("How it is sent: As a bearer token")
	option, ok := stage.named("As the mail account password")
	if !ok {
		t.Fatalf("the select opened no menu; on screen: %v", stage.names())
	}
	beside := option.Desc.Bounds.Min.X < field.Desc.Bounds.Max.X && option.Desc.Bounds.Max.X > field.Desc.Bounds.Min.X
	near := option.Desc.Bounds.Min.Y-field.Desc.Bounds.Max.Y < 200 && field.Desc.Bounds.Min.Y-option.Desc.Bounds.Max.Y < 200
	if !beside || !near {
		t.Fatalf("the menu opened at %v, away from its select at %v", option.Desc.Bounds, field.Desc.Bounds)
	}
}
