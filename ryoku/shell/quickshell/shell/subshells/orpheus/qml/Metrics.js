.pragma library

var smoothing = 12
var squareSmoothing = 5
var shoulder = 12
var shoulderReach = 48
// The card welds into the drop it grows from: its top edge buries this far up
// into the bar band so the smooth-min fillet fuses card and bar into one liquid
// edge (the frame popout's neck). On close the burial drains the fillet back
// into the band so the card melts shut instead of stranding a nub.
var weld = 10
var cardMaxW = 560
var cardMaxH = 640
