package term

// SplitPlaque describes the two independently focused parts of a split action.
type SplitPlaque struct {
	Main      Plaque
	Expanded  bool
	FocusMore bool
	MoreLabel string
}

// RenderSplitPlaque renders the main action and menu segment.
func RenderSplitPlaque(th Theme, p SplitPlaque) string {
	main := p.Main
	main.Focused = main.Focused && !p.FocusMore
	more := p.MoreLabel
	if more == "" {
		if p.Expanded {
			more = "Less"
		} else {
			more = "More"
		}
	}
	return RenderPlaque(th, main) + " " + RenderPlaque(th, Plaque{Label: more, Kind: PlaqueQuiet, Focused: p.FocusMore, Disabled: main.Disabled})
}
