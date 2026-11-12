package term

import "strings"

// Marks describes ordered resource traits.
type Marks struct {
	Disposable bool
	Instant    bool
	Sealed     bool
	Additional []string
}

// RenderMarks renders resource traits with disposable first.
func RenderMarks(th Theme, marks Marks) string {
	parts := make([]string, 0, 3+len(marks.Additional))
	if marks.Disposable {
		parts = append(parts, ink(th.Roles.Amber).Render("Disposable"))
	}
	if marks.Instant {
		parts = append(parts, "Instant")
	}
	if marks.Sealed {
		parts = append(parts, "Sealed")
	}
	parts = append(parts, marks.Additional...)
	return ink(th.Roles.Fg2).Render(strings.Join(parts, "   "))
}
