package window

import (
	"context"
	"slices"
	"sync"

	"ryoku-rashin/internal/oni/spec"
)

// Wlr is the degraded provider for a generic wlroots-family desktop with no
// IPC Hyakki reads. Its window list comes from wlr-foreign-toplevel, which the
// Oni surface already binds and hands over through Feed. That protocol
// carries identity, focus and state but no position, so every window is
// unplaced: Oni can name windows here but never perch on one.
type Wlr struct {
	mu   sync.Mutex
	snap Snapshot
	subs map[chan struct{}]struct{}
}

// NewWlr returns an empty degraded provider.
func NewWlr() *Wlr { return &Wlr{subs: map[chan struct{}]struct{}{}} }

// Feed replaces the window list. Any Box is dropped, since a toplevel list
// cannot place a window, and every window carries the no-geometry reason.
func (p *Wlr) Feed(s Snapshot) {
	s.Outputs = slices.Clone(s.Outputs)
	s.Windows = slices.Clone(s.Windows)
	for i := range s.Windows {
		s.Windows[i].Box = nil
		s.Windows[i].Unplaced = UnplacedNoGeom
	}
	sortWindows(s.Windows)
	p.mu.Lock()
	p.snap = s
	for c := range p.subs {
		select {
		case c <- struct{}{}:
		default:
		}
	}
	p.mu.Unlock()
}

func (p *Wlr) Caps() Caps { return Caps{Provider: "wlr", Geometry: GeomNone, Events: true} }

func (p *Wlr) Snapshot(context.Context) (Snapshot, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.snap, nil
}

// Watch delivers the current list and then every Feed until ctx ends.
func (p *Wlr) Watch(ctx context.Context, fn func(Snapshot)) error {
	c := make(chan struct{}, 1)
	c <- struct{}{}
	p.mu.Lock()
	p.subs[c] = struct{}{}
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.subs, c)
		p.mu.Unlock()
	}()
	e := emitter{fn: fn}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c:
			s, _ := p.Snapshot(ctx)
			e.emit(s)
		}
	}
}

func (p *Wlr) Geometry(ID) {}

func (p *Wlr) Pointer(context.Context) (spec.Point, bool, error) {
	return spec.Point{}, false, nil
}
