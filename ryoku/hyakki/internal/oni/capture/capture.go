// Package capture takes the one picture an Oni screen ask is allowed: the
// perched window, through the narrowest path the desktop offers. Pictures
// live in memory only. grim writes to its standard output, nothing is ever
// written to disk, and a Shot zeroes every buffer it handed out when cleared.
//
// The ladder, in order of preference:
//
//   - toplevel: grim -T captures one toplevel through ext-image-copy-capture.
//     It needs the compositor's ext-foreign-toplevel-list identifier, so it
//     runs only when the caller supplies a Toplevel resolver.
//   - niri-window: never used. niri saves window screenshots to a file, and
//     Oni keeps pictures in memory only.
//   - output-crop: grim -g grabs the screen region under the window. Anything
//     drawn over that region is in the grab, so a window that is not focused
//     or is covered by another window is refused, and Oni's own pixels are
//     painted white.
//   - portal, x11: disclosed as not offered here. They arrive with the generic
//     desktop provider; a whole-desktop grab is never taken in their place.
package capture

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/oni/window"
)

// Path names one way of taking the picture.
type Path string

const (
	PathToplevel   Path = "toplevel"
	PathNiriWindow Path = "niri-window"
	PathOutputCrop Path = "output-crop"
	PathPortal     Path = "portal"
	PathX11        Path = "x11"
)

// Ladder is every path in order of preference.
var Ladder = []Path{PathToplevel, PathNiriWindow, PathOutputCrop, PathPortal, PathX11}

// Capability is what one path can do on this desktop right now.
type Capability struct {
	Path Path
	// Offered is true when Capture may use this path.
	Offered bool
	// ScopeExact is true when only the window's own pixels are read.
	ScopeExact bool
	// WholeOutputInMemory is true when more of the screen than the window
	// passes through memory on the way to the crop.
	WholeOutputInMemory bool
	// MasksNeeded is true when Oni's own pixels can be in the grab.
	MasksNeeded bool
	// Reason says why the path is not offered; empty when it is.
	Reason string
}

// Disclosure names the path that ran, in words a person can check.
type Disclosure struct {
	Path       Path
	Text       string
	ScopeExact bool
}

// Service takes pictures of one window.
type Service interface {
	// Probe lists every path in ladder order with what it can do.
	Probe(ctx context.Context) []Capability
	// Capture takes w through the first offered path that accepts it. masks
	// are the presenter's own rects on w's output, in output-local logical
	// pixels; they are painted white on the crop path.
	Capture(ctx context.Context, w window.Window, masks []window.Box) (*Shot, Disclosure, error)
}

// Option configures a Service.
type Option func(*service)

// Toplevel supplies the compositor's ext-foreign-toplevel-list identifier
// for a window. Without it the toplevel path is not offered.
func Toplevel(resolve func(ctx context.Context, w window.Window) (string, error)) Option {
	return func(s *service) { s.toplevel = resolve }
}

// Layout supplies the current outputs and windows: outputs place the crop in
// layout coordinates, windows show whether anything covers the target.
// Without it the crop path is not offered.
func Layout(read func(ctx context.Context) (window.Snapshot, error)) Option {
	return func(s *service) { s.layout = read }
}

// Refusals. Each is final for the ask: the ladder never falls back to a wider
// grab after one of these.
var (
	ErrNoGeometry = errors.New(i18n.T("This desktop does not say where that window is, so Oni cannot picture it."))
	ErrHidden     = errors.New(i18n.T("That window is not on screen right now."))
	ErrNotFocused = errors.New(i18n.T("That window is behind another one. Bring it forward first, so the picture holds only that window."))
	ErrOccluded   = errors.New(i18n.T("Another window covers part of that one, so the picture would show it too. Move it aside first."))
	ErrOffOutput  = errors.New(i18n.T("That window is not on any screen Oni can see."))
	ErrNoPath     = errors.New(i18n.T("This desktop offers no way to picture just one window."))
	ErrTooLarge   = errors.New(i18n.T("The picture was larger than any screen, so Oni dropped it."))
	ErrBadImage   = errors.New(i18n.T("The screen tool returned a picture Oni could not read."))
	ErrCleared    = errors.New(i18n.T("That picture was already cleared."))
)

// ErrNotAllowed is returned for any argv outside the capture allowlist.
var ErrNotAllowed = errors.New("capture: command not in the capture allowlist")

// New returns a Service that may use only the paths in allowed, so a lab can
// hide a protocol and prove the fallback. run starts grim; its stdout is the
// picture.
func New(run window.Runner, allowed []Path, opts ...Option) Service {
	s := &service{run: run, allowed: map[Path]bool{}}
	for _, p := range allowed {
		s.allowed[p] = true
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

type service struct {
	run      window.Runner
	allowed  map[Path]bool
	toplevel func(ctx context.Context, w window.Window) (string, error)
	layout   func(ctx context.Context) (window.Snapshot, error)
}

func (s *service) Probe(ctx context.Context) []Capability {
	out := make([]Capability, 0, len(Ladder))
	for _, p := range Ladder {
		out = append(out, s.capability(p))
	}
	return out
}

func (s *service) capability(p Path) Capability {
	c := Capability{Path: p}
	switch p {
	case PathToplevel:
		c.ScopeExact = true
		if s.toplevel == nil {
			c.Reason = i18n.T("This desktop gives Oni no toplevel handle to capture one window by.")
		}
	case PathNiriWindow:
		c.ScopeExact = true
		c.Reason = i18n.T("niri saves window pictures to a file, and Oni keeps pictures in memory only.")
	case PathOutputCrop:
		c.WholeOutputInMemory = true
		c.MasksNeeded = true
		if s.layout == nil {
			c.Reason = i18n.T("Oni has no screen layout to place the crop with.")
		}
	case PathPortal:
		c.WholeOutputInMemory = true
		c.MasksNeeded = true
		c.Reason = i18n.T("The desktop portal path arrives with the generic desktop support; it is not offered here.")
	case PathX11:
		c.Reason = i18n.T("X11 capture arrives with the generic desktop support; it is not offered here.")
	}
	if c.Reason == "" && !s.allowed[p] {
		c.Reason = i18n.T("This path is turned off for this run.")
	}
	c.Offered = c.Reason == ""
	return c
}

func (s *service) Capture(ctx context.Context, w window.Window, masks []window.Box) (*Shot, Disclosure, error) {
	if w.Minimized || !w.Visible {
		return nil, Disclosure{}, ErrHidden
	}
	var tried error
	for _, p := range Ladder {
		if !s.capability(p).Offered {
			continue
		}
		switch p {
		case PathToplevel:
			shot, err := s.captureToplevel(ctx, w)
			if err == nil {
				return shot, disclose(p), nil
			}
			if ctx.Err() != nil {
				return nil, Disclosure{}, ctx.Err()
			}
			// The compositor may not offer toplevel capture for this window;
			// the crop path below applies its own stricter checks.
			tried = err
		case PathOutputCrop:
			shot, err := s.captureCrop(ctx, w, masks)
			if err != nil {
				return nil, Disclosure{}, err
			}
			return shot, disclose(p), nil
		}
	}
	if tried != nil {
		return nil, Disclosure{}, fmt.Errorf("%w (%v)", ErrNoPath, tried)
	}
	return nil, Disclosure{}, ErrNoPath
}

func disclose(p Path) Disclosure {
	switch p {
	case PathToplevel:
		return Disclosure{Path: p, ScopeExact: true,
			Text: i18n.T("Oni pictured only this window, through the desktop's window capture.")}
	case PathOutputCrop:
		return Disclosure{Path: p,
			Text: i18n.T("Oni grabbed the screen region under this window and cropped it. Anything drawn over that region was in the grab; Oni's own pixels were painted white.")}
	}
	return Disclosure{Path: p}
}

func (s *service) captureToplevel(ctx context.Context, w window.Window) (*Shot, error) {
	id, err := s.toplevel(ctx, w)
	if err != nil {
		return nil, err
	}
	if !validIdentifier(id) {
		return nil, fmt.Errorf("capture: toplevel identifier %q is not printable", id)
	}
	return s.grab(ctx, "-T", id)
}

func (s *service) captureCrop(ctx context.Context, w window.Window, masks []window.Box) (*Shot, error) {
	if w.Box == nil {
		return nil, ErrNoGeometry
	}
	if !w.Focused {
		return nil, ErrNotFocused
	}
	snap, err := s.layout(ctx)
	if err != nil {
		return nil, err
	}
	var out *window.Output
	for i := range snap.Outputs {
		if snap.Outputs[i].Name == w.Output {
			out = &snap.Outputs[i]
			break
		}
	}
	if out == nil {
		return nil, ErrOffOutput
	}
	if covered(w, snap.Windows) {
		return nil, ErrOccluded
	}
	// The visible part of the window on its output, in output-local px.
	vis, ok := intersect(*w.Box, window.Box{W: float64(out.W), H: float64(out.H)})
	if !ok {
		return nil, ErrOffOutput
	}
	x0 := int(math.Floor(vis.X))
	y0 := int(math.Floor(vis.Y))
	x1 := int(math.Ceil(vis.X + vis.W))
	y1 := int(math.Ceil(vis.Y + vis.H))
	region := window.Box{X: float64(x0), Y: float64(y0), W: float64(x1 - x0), H: float64(y1 - y0)}
	geom := strconv.Itoa(out.X+x0) + "," + strconv.Itoa(out.Y+y0) + " " +
		strconv.Itoa(x1-x0) + "x" + strconv.Itoa(y1-y0)
	shot, err := s.grab(ctx, "-g", geom)
	if err != nil {
		return nil, err
	}
	shot.mask(region, masks)
	return shot, nil
}

// grab runs grim with one selector and reads the PNG from its stdout.
func (s *service) grab(ctx context.Context, flag, value string) (*Shot, error) {
	args := []string{flag, value, "-t", "png", "-l", "1", "-"}
	if !Allowed("grim", args...) {
		return nil, ErrNotAllowed
	}
	raw, err := s.run(ctx, "grim", args...)
	if err != nil {
		zero(raw)
		return nil, err
	}
	return decode(raw)
}

// Allowed reports whether argv is one this package may start: grim with a
// single toplevel or region selector, PNG to stdout. Nothing else.
func Allowed(name string, args ...string) bool {
	if name != "grim" || len(args) != 7 {
		return false
	}
	if args[2] != "-t" || args[3] != "png" || args[4] != "-l" || args[5] != "1" || args[6] != "-" {
		return false
	}
	switch args[0] {
	case "-T":
		return validIdentifier(args[1])
	case "-g":
		return validGeometry(args[1])
	}
	return false
}

// validIdentifier accepts printable ASCII, as the toplevel list protocol
// promises, and refuses anything grim could read as another flag.
func validIdentifier(id string) bool {
	if id == "" || len(id) > 256 || id[0] == '-' {
		return false
	}
	for i := range len(id) {
		if id[i] < 0x21 || id[i] > 0x7e {
			return false
		}
	}
	return true
}

// validGeometry accepts exactly "x,y wxh" with integer x, y and positive w, h.
func validGeometry(g string) bool {
	var x, y, w, h int
	n, err := fmt.Sscanf(g, "%d,%d %dx%d", &x, &y, &w, &h)
	if err != nil || n != 4 || w <= 0 || h <= 0 {
		return false
	}
	return g == strconv.Itoa(x)+","+strconv.Itoa(y)+" "+strconv.Itoa(w)+"x"+strconv.Itoa(h)
}

// covered reports whether any other window on the same output can be drawn
// over the focused window w. Compositors report no stacking order, so this is
// conservative: a fullscreen window anywhere on the output covers, and a
// floating window over a tiled one covers. A focused floating window is
// raised by both compositors, so other floating windows sit under it.
func covered(w window.Window, all []window.Window) bool {
	for _, o := range all {
		if o.ID == w.ID || o.Output != w.Output || o.Minimized || !o.Visible {
			continue
		}
		if o.Fullscreen && !w.Fullscreen {
			return true
		}
		if o.Box == nil {
			continue
		}
		if _, hit := intersect(*w.Box, *o.Box); hit && o.Floating && !w.Floating {
			return true
		}
	}
	return false
}

func intersect(a, b window.Box) (window.Box, bool) {
	x0 := math.Max(a.X, b.X)
	y0 := math.Max(a.Y, b.Y)
	x1 := math.Min(a.X+a.W, b.X+b.W)
	y1 := math.Min(a.Y+a.H, b.Y+b.H)
	if x1 <= x0 || y1 <= y0 {
		return window.Box{}, false
	}
	return window.Box{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}, true
}
