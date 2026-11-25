package surface

// Hide reasons in the ledger.
const (
	hideHidden   = "hidden"
	hideCourtesy = "courtesy"
	hideOutput   = "output"
)

// Courtesy mechanisms the start line names. With wlr-foreign-toplevel the
// surface sees fullscreen windows itself and unmaps. Without it, it relies
// on the Top layer: niri and Hyprland draw a fullscreen window over Top
// layer surfaces, and a compositor stops sending frame callbacks to a
// surface it does not show, so the frame-callback clock stops with no hide
// recorded.
const (
	courtesyForeign  = "courtesy:foreign-toplevel"
	courtesyTopLayer = "courtesy:top-layer"
)

// visibility decides whether the character is on screen and, if not, why.
// The daemon's hide wins over courtesy, which wins over a missing output.
func visibility(daemonHidden, courtesy, covered, haveOutput bool) (show bool, reason string) {
	switch {
	case daemonHidden:
		return false, hideHidden
	case courtesy && covered:
		return false, hideCourtesy
	case !haveOutput:
		return false, hideOutput
	}
	return true, ""
}
