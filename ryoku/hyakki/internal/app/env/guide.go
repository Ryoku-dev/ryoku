package env

import "ryoku-rashin/internal/oni/keep"

// Guide is what Oni's guide settings read on this computer: the
// walkthroughs the user kept, in the file the daemon writes, and whether
// Oni can read text off a picture of a window. A page without it leaves
// both nil.
type Guide struct {
	// Keeps holds the kept walkthroughs: goal, app and steps, never a
	// picture or a window title.
	Keeps *keep.Store
	// OCR says why Oni cannot read the text in a window, "" when it can. It
	// looks once per call; nothing polls it.
	OCR func() string
}
