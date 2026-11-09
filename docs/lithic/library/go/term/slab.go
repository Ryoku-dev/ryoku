package term

// Slab describes a quiet grouping container.
type Slab struct {
	Content string
	Width   int
}

// RenderSlab renders a stone1 cut frame.
func RenderSlab(th Theme, slab Slab) string {
	content := slab.Content
	if slab.Width > 0 {
		content = fit(content, slab.Width)
	}
	return framed(th, content, th.Roles.Stone1, false, false)
}
