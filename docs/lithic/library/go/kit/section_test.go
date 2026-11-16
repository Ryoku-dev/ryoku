package kit

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/input"
	"gioui.org/op"
)

// TestSectionRuleUnderStretchedWidth: a ruled head draws its hairline from s3 after the title
// to s3 before the mark, at the title's middle, whether its parent leaves Min.X at 0 or
// stretches it to the full width, as a vertical Flex does under Min.X = Max.X. Stretched, the
// title used to take the whole width and leave the rule no room (Section.qml:47-54).
func TestSectionRuleUnderStretchedWidth(t *testing.T) {
	th := partsTh()
	const width = 600
	for _, stretched := range []bool{false, true} {
		var router input.Router
		var ops op.Ops
		gtx := partsFrame(&router, &ops, time.Unix(1000, 0), image.Pt(width, 400))
		if stretched {
			gtx.Constraints.Min.X = width
		}
		section := SectionStyle{Title: "Corners", Ruled: true, Mark: Ordinal(1, 8)}
		head, rule := section.head(gtx, th)
		title := trackedLine(gtx, th, th.T.Section, 0, section.Title, false)
		mark := trackedLine(gtx, th, th.T.MonoSm, 0, section.Mark, false)
		s3 := gtx.Dp(th.Sp.S3)
		want := image.Rect(title.width+s3, (head.Size.Y-1)/2, width-mark.width-s3, (head.Size.Y-1)/2+1)
		if rule != want {
			t.Errorf("stretched=%v: rule %v, want %v (head %v)", stretched, rule, want, head.Size)
		}
		if head.Size.X != width {
			t.Errorf("stretched=%v: head %d wide, want the full %d", stretched, head.Size.X, width)
		}
		// and through Layout, in a stretching vertical Flex
		dims := section.Layout(gtx, th, nil)
		if dims.Size.X != width {
			t.Errorf("stretched=%v: section %d wide, want %d", stretched, dims.Size.X, width)
		}
	}
}
