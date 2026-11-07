.pragma library

// The Kairos island's geometry, shared by the bar island and the launcher it
// grows into. The island has three sizes: the resting pill around the time, the
// hover state (clock over the date wheel), and the launcher -- which grows from
// the RESTING pill, never from the hover size, so Super+Space reads as the
// island opening rather than a bar-sized window appearing over it.

var topGap = 6

var restHeight = 34
var restFontSize = 15
var restPadX = 22

var hoverWidth = 380
var hoverHeight = 150
var hoverRadius = 40
var hoverFontSize = 26
var hoverClockCentreY = 27
var wheelTop = 44
var wheelHeight = 92

var launcherFontSize = 34

// Music: the cover bubble that grows beside the clock into the now-playing
// panel. The panel's cover sits where the bubble was, so the bubble reads as the
// left end of the panel rather than a second shape appearing.
var bubbleSize = 34
var bubbleGap = 8
var musicWidth = 470
var musicHeight = 236
var musicRadius = 40
// Hover peeks the panel just far enough to reach the transport; a click opens it
// fully.
var musicPeekHeight = 100
var musicPeekRadius = 28
var coverPeek = 52
var coverPeekX = 14
var coverPeekY = 24
var coverPeekRadius = 12
var coverOpen = 150
var coverX = 24
var coverY = 43
var coverRadius = 16

// The black frame the ambient backdrop is inset by, so the pill's near-black
// reads as a bezel around the blurred cover.
var framePad = 12

var shadowBleed = 32
