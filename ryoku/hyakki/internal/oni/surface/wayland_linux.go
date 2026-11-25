//go:build linux

package surface

// The protocol glue is wayland-scanner output from the XML in protocols/,
// regenerated with `go generate` (wayland-scanner 1.26 made the vendored copy).
//go:generate sh -c "for x in protocols/*.xml; do n=proto_$(basename $x .xml | tr - _); wayland-scanner client-header $x $n.h && wayland-scanner private-code $x $n.c && sed -i '1s;^;//go:build linux\\n\\n;' $n.c; done"

/*
#cgo LDFLAGS: -lwayland-client

#include <stdlib.h>
#include <wayland-client.h>
#include "proto_wlr_layer_shell_unstable_v1.h"
#include "proto_wlr_foreign_toplevel_management_unstable_v1.h"
#include "proto_xdg_output_unstable_v1.h"
#include "proto_viewporter.h"
#include "proto_fractional_scale_v1.h"

extern const struct wl_registry_listener oni_registry_listener;
extern const struct wl_output_listener oni_output_listener;
extern const struct zxdg_output_v1_listener oni_xdg_output_listener;
extern const struct wl_seat_listener oni_seat_listener;
extern const struct wl_pointer_listener oni_pointer_listener;
extern const struct wl_surface_listener oni_surface_listener;
extern const struct wl_callback_listener oni_callback_listener;
extern const struct wl_buffer_listener oni_buffer_listener;
extern const struct zwlr_layer_surface_v1_listener oni_layer_surface_listener;
extern const struct wp_fractional_scale_v1_listener oni_fractional_scale_listener;
extern const struct zwlr_foreign_toplevel_manager_v1_listener oni_toplevel_manager_listener;
extern const struct zwlr_foreign_toplevel_handle_v1_listener oni_toplevel_listener;

static uint32_t oni_array_u32(struct wl_array *a, size_t i) { return ((uint32_t *)a->data)[i]; }

static void oni_output_release(struct wl_output *o) {
	if (wl_output_get_version(o) >= WL_OUTPUT_RELEASE_SINCE_VERSION) wl_output_release(o); else wl_output_destroy(o);
}
static void oni_pointer_release(struct wl_pointer *p) {
	if (wl_pointer_get_version(p) >= WL_POINTER_RELEASE_SINCE_VERSION) wl_pointer_release(p); else wl_pointer_destroy(p);
}
static void oni_seat_release(struct wl_seat *s) {
	if (wl_seat_get_version(s) >= WL_SEAT_RELEASE_SINCE_VERSION) wl_seat_release(s); else wl_seat_destroy(s);
}
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"slices"
	"sort"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"ryoku-rashin/internal/oni/placement"
	"ryoku-rashin/internal/oni/render"
	"ryoku-rashin/internal/oni/spec"
)

const (
	btnLeft  = 0x110
	btnRight = 0x111
)

// wl is the one display connection of this process; the exported callbacks
// find their state through it.
var wl *wlBackend

type wlBackend struct {
	disp *C.struct_wl_display
	reg  *C.struct_wl_registry
	sink sink

	compositor *C.struct_wl_compositor
	subcomp    *C.struct_wl_subcompositor
	shm        *C.struct_wl_shm
	layerShell *C.struct_zwlr_layer_shell_v1
	viewporter *C.struct_wp_viewporter
	fracMgr    *C.struct_wp_fractional_scale_manager_v1
	topMgr     *C.struct_zwlr_foreign_toplevel_manager_v1
	xdgOutMgr  *C.struct_zxdg_output_manager_v1
	seat       *C.struct_wl_seat
	seatName   C.uint32_t
	pointer    *C.struct_wl_pointer

	advertised map[string]uint32
	outs       map[C.uint32_t]*wlOutput
	tops       map[*C.struct_zwlr_foreign_toplevel_handle_v1]*wlToplevel
	sprite     *wlSprite
	pointerIn  bool
	// pointerOv is the overlay the pointer is over while pointerOnOv (the
	// perch chip or the guide card)
	pointerOv   overlay
	pointerOnOv bool

	// picks are the pick overlay's surfaces by output name, mapped only
	// while the host has armed them; pointerPick is the one under the
	// pointer, pickAt where on it and pickDown a left press on it
	picks       map[string]*wlPick
	pointerPick *wlPick
	pickAt      spec.Point
	pickDown    bool

	// glows are the Hands glow's surfaces by output name, only while a
	// Hands task runs
	glows map[string]*wlGlow

	qmu   sync.Mutex
	queue []func()
	wake  chan struct{}

	// the toplevel manager's global, bound once the outputs are
	topName      C.uint32_t
	topVersion   uint32
	outputsBound bool
}

type wlOutput struct {
	reg     C.uint32_t
	o       *C.struct_wl_output
	xdg     *C.struct_zxdg_output_v1
	name    string
	geom    image.Point
	mode    image.Point
	scale   int
	logical image.Rectangle
	ready   bool
}

type wlToplevel struct {
	outputs              map[C.uint32_t]bool
	fullscreen           bool
	minimized            bool
	pendingFS, pendingMn bool
}

// wlSprite is the character on screen: surf is the layer surface, which is
// placed on the output and takes the pointer; child is a subsurface of it
// carrying the pixels (see spriteOrigin). strand, while the thread is out, is
// a second subsurface below child. The layer surface is sprite-sized, and
// grows by layerSize while the strand or an overlay is out.
type wlSprite struct {
	surf   *C.struct_wl_surface
	layer  *C.struct_zwlr_layer_surface_v1
	pview  *C.struct_wp_viewport
	frac   *C.struct_wp_fractional_scale_v1
	child  *C.struct_wl_surface
	sub    *C.struct_wl_subsurface
	view   *C.struct_wp_viewport
	output C.uint32_t

	configured bool
	// frame is the one pending frame callback, on child or on the strand
	// (frameOnStrand); either done steps the same clock.
	frame         *C.struct_wl_callback
	frameOnStrand bool
	confirm       *C.struct_wl_callback

	preferred float64
	entered   C.uint32_t

	geo  spriteOrigin
	size image.Point
	// sprite is the character's size, which the layer surface has while no
	// part is out
	sprite    image.Point
	childAt   image.Point
	clear     *shmBuffer
	clearDest image.Point
	input     image.Rectangle
	hasInput  bool
	bufScale  int
	buffers   []*shmBuffer

	strand *wlStrand
	// overlays are the perch chip and the guide card (above the sprite)
	// and the drag highlight (below everything), by overlay.
	overlays [overlayCount]*wlStrand
}

// wlPick is one pick surface: a transparent layer surface on the overlay
// layer covering its output, anchored to all four edges, that takes the
// pointer over its whole face and never the keyboard. It draws one
// transparent pixel (stretched by the viewport, else a buffer the output's
// size) and never asks for a frame callback. The compositor's cursor stays
// as it was; the surface names none.
type wlPick struct {
	name   string
	output C.uint32_t
	surf   *C.struct_wl_surface
	layer  *C.struct_zwlr_layer_surface_v1
	view   *C.struct_wp_viewport
	buf    *shmBuffer
	size   image.Point
}

// wlGlow is the Hands glow on one output: one layer surface per strip on
// the overlay layer, placed by its margins from the output's top left, with
// an empty input region and no keyboard, so the pointer and every key,
// Esc among them, go where they would without it. It never asks for a frame
// callback: the host paces the breath.
type wlGlow struct {
	name   string
	output C.uint32_t
	strips [render.GlowStrips]*wlGlowStrip
}

type wlGlowStrip struct {
	surf       *C.struct_wl_surface
	layer      *C.struct_zwlr_layer_surface_v1
	view       *C.struct_wp_viewport
	rect       image.Rectangle
	configured bool
	bufScale   int
	buffers    []*shmBuffer
}

// wlStrand is the strand's pixels: a subsurface of the sprite's layer
// surface, stacked just below the sprite's own subsurface so the character
// draws over the thread's root. It spans the thread's bbox, the layer
// surface grows to cover it (layerSize), and its empty input region lets
// the pointer through. The perch overlays are the same kind of surface.
type wlStrand struct {
	surf *C.struct_wl_surface
	sub  *C.struct_wl_subsurface
	view *C.struct_wp_viewport
	// origin is where the surface is, output-local logical px; at is that
	// relative to the layer surface's requested origin; rect is the whole
	// surface, output-local.
	origin   image.Point
	at       image.Point
	rect     image.Rectangle
	placed   bool
	dest     image.Point
	bufScale int
	buffers  []*shmBuffer
	input    image.Rectangle
}

type shmBuffer struct {
	buf  *C.struct_wl_buffer
	data []byte
	w, h int
	busy bool
	dead bool
}

// connectWayland opens the display named by WAYLAND_DISPLAY and binds the
// globals. A nil sink only lists them.
func connectWayland(s sink) (*wlBackend, error) {
	if wl != nil {
		return nil, errors.New("wayland already connected")
	}
	disp := C.wl_display_connect(nil)
	if disp == nil {
		return nil, errors.New("no Wayland display (WAYLAND_DISPLAY)")
	}
	b := &wlBackend{
		disp:       disp,
		sink:       s,
		advertised: map[string]uint32{},
		outs:       map[C.uint32_t]*wlOutput{},
		tops:       map[*C.struct_zwlr_foreign_toplevel_handle_v1]*wlToplevel{},
		picks:      map[string]*wlPick{},
		glows:      map[string]*wlGlow{},
	}
	b.wake = make(chan struct{}, 1)
	wl = b
	b.reg = C.wl_display_get_registry(disp)
	C.wl_registry_add_listener(b.reg, &C.oni_registry_listener, nil)
	for i := range 2 {
		if C.wl_display_roundtrip(disp) < 0 {
			b.close()
			return nil, errors.New("wayland roundtrip failed")
		}
		// The toplevel manager announces the windows that already exist as
		// soon as it is bound, with the outputs they are on, and a compositor
		// can name only outputs this client has bound by then. Bound in
		// registry order, Hyprland's came first, so every existing window
		// arrived without its output and a fullscreen one never covered the
		// character; it is bound once the outputs are.
		if i == 0 {
			b.outputsBound = true
			if b.sink != nil && b.topName != 0 {
				b.bindToplevels(b.topName, b.topVersion)
			}
		}
	}
	return b, nil
}

// required reports the protocols the surface cannot run without.
func (b *wlBackend) required() error {
	var missing []string
	if b.compositor == nil {
		missing = append(missing, "wl_compositor")
	}
	if b.shm == nil {
		missing = append(missing, "wl_shm")
	}
	if b.layerShell == nil {
		missing = append(missing, "zwlr_layer_shell_v1")
	}
	if b.subcomp == nil {
		missing = append(missing, "wl_subcompositor")
	}
	if len(missing) > 0 {
		return fmt.Errorf("the compositor lacks %v", missing)
	}
	return nil
}

func (b *wlBackend) globals() Globals {
	g := Globals{Display: os.Getenv("WAYLAND_DISPLAY"), Bound: map[string]uint32{}, Missing: []string{}}
	for _, name := range Protocols {
		if v, ok := b.advertised[name]; ok {
			g.Bound[name] = v
		} else {
			g.Missing = append(g.Missing, name)
		}
	}
	return g
}

func (b *wlBackend) post(fn func()) {
	b.qmu.Lock()
	b.queue = append(b.queue, fn)
	b.qmu.Unlock()
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

func (b *wlBackend) runQueue() {
	b.qmu.Lock()
	q := b.queue
	b.queue = nil
	b.qmu.Unlock()
	for _, fn := range q {
		fn()
	}
}

// run is the display loop. It waits for the display through Go's netpoller,
// not a blocking poll(2): a goroutine sitting in a system call keeps its P,
// and the runtime's monitor then retook it every 10 ms and fell back to
// 20 us naps, about 2000 wakeups a second while the character moved.
func (b *wlBackend) run(ctx context.Context) error {
	stop := context.AfterFunc(ctx, func() { b.post(func() {}) })
	defer stop()
	fd := int(C.wl_display_get_fd(b.disp))
	// a duplicate, so the netpoller owns a descriptor libwayland does not
	dup, err := unix.Dup(fd)
	if err != nil {
		return err
	}
	if err := unix.SetNonblock(dup, true); err != nil {
		unix.Close(dup)
		return err
	}
	file := os.NewFile(uintptr(dup), "wayland")
	defer file.Close()
	rc, err := file.SyscallConn()
	if err != nil {
		return err
	}
	ready, resume, done := make(chan int16), make(chan struct{}), make(chan struct{})
	defer close(done)
	go func() {
		defer close(ready)
		for {
			var rev int16
			if err := rc.Read(func(fd uintptr) bool {
				pfd := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
				if n, _ := unix.Poll(pfd, 0); n > 0 {
					rev = pfd[0].Revents
					return true
				}
				return false
			}); err != nil {
				return
			}
			select {
			case ready <- rev:
			case <-done:
				return
			}
			select {
			case <-resume:
			case <-done:
				return
			}
		}
	}()
	for {
		b.runQueue()
		if ctx.Err() != nil {
			return nil
		}
		for C.wl_display_prepare_read(b.disp) != 0 {
			if C.wl_display_dispatch_pending(b.disp) < 0 {
				return errors.New("wayland dispatch failed")
			}
		}
		// a full socket is rare: wait for it briefly the plain way
		for {
			r, err := C.wl_display_flush(b.disp)
			if r >= 0 {
				break
			}
			if !errors.Is(err, unix.EAGAIN) {
				C.wl_display_cancel_read(b.disp)
				return fmt.Errorf("wayland flush: %w", err)
			}
			if _, err := unix.Poll([]unix.PollFd{{Fd: int32(fd), Events: unix.POLLOUT}}, 100); err != nil && !errors.Is(err, unix.EINTR) {
				C.wl_display_cancel_read(b.disp)
				return err
			}
		}
		select {
		case rev, ok := <-ready:
			if !ok || rev&(unix.POLLERR|unix.POLLHUP) != 0 {
				C.wl_display_cancel_read(b.disp)
				return errors.New("the Wayland display closed")
			}
			if C.wl_display_read_events(b.disp) < 0 {
				return errors.New("wayland read failed")
			}
			resume <- struct{}{}
		case <-b.wake:
			C.wl_display_cancel_read(b.disp)
		}
		if C.wl_display_dispatch_pending(b.disp) < 0 {
			return errors.New("wayland dispatch failed")
		}
	}
}

func (b *wlBackend) close() {
	if b.sprite != nil {
		b.destroySprite()
	}
	for _, p := range b.picks {
		b.destroyPick(p)
	}
	b.hideGlow(nil)
	if b.disp != nil {
		C.wl_display_disconnect(b.disp)
		b.disp = nil
	}
	wl = nil
}

func (b *wlBackend) foreignToplevels() bool { return b.topMgr != nil }

func (b *wlBackend) outputs() []placement.Output {
	var out []placement.Output
	for _, o := range b.outs {
		if !o.ready || o.logical.Empty() {
			continue
		}
		out = append(out, placement.Output{ID: o.name, Rect: o.logical, Scale: b.outputScale(o)})
	}
	sort.Slice(out, func(i, j int) bool {
		a, c := out[i].Rect.Min, out[j].Rect.Min
		if a.X != c.X {
			return a.X < c.X
		}
		return a.Y < c.Y
	})
	return out
}

func (b *wlBackend) outputNamed(name string) *wlOutput {
	for _, o := range b.outs {
		if o.name == name {
			return o
		}
	}
	return nil
}

func (b *wlBackend) outputScale(o *wlOutput) float64 {
	if s := b.sprite; s != nil && s.output == o.reg && s.preferred > 0 {
		return s.preferred
	}
	return float64(max(o.scale, 1))
}

// scale is the device-pixel ratio the sprite draws at on output: the
// fractional preferred scale through a viewport, else the integer buffer
// scale.
func (b *wlBackend) scale(output string) float64 {
	o := b.outputNamed(output)
	if o == nil {
		return 1
	}
	s := b.outputScale(o)
	if b.viewporter == nil {
		return math.Ceil(s)
	}
	return s
}

func (b *wlBackend) covered(output string) bool {
	o := b.outputNamed(output)
	if o == nil {
		return false
	}
	for _, t := range b.tops {
		if t.fullscreen && !t.minimized && t.outputs[o.reg] {
			return true
		}
	}
	return false
}

func (b *wlBackend) hide() {
	if b.sprite != nil {
		b.destroySprite()
	}
}

func (b *wlBackend) destroySprite() {
	s := b.sprite
	b.sprite = nil
	for _, cb := range []*C.struct_wl_callback{s.frame, s.confirm} {
		if cb != nil {
			C.wl_callback_destroy(cb)
		}
	}
	if s.strand != nil {
		destroyStrand(s.strand)
		s.strand = nil
	}
	for i, t := range s.overlays {
		if t != nil {
			destroyStrand(t)
			s.overlays[i] = nil
		}
	}
	if s.view != nil {
		C.wp_viewport_destroy(s.view)
	}
	C.wl_subsurface_destroy(s.sub)
	C.wl_surface_destroy(s.child)
	if s.pview != nil {
		C.wp_viewport_destroy(s.pview)
	}
	if s.frac != nil {
		C.wp_fractional_scale_v1_destroy(s.frac)
	}
	C.zwlr_layer_surface_v1_destroy(s.layer)
	C.wl_surface_destroy(s.surf)
	for _, buf := range s.buffers {
		buf.destroy()
	}
	if s.clear != nil {
		s.clear.destroy()
	}
	b.pointerIn, b.pointerOnOv = false, false
}

func (b *wlBackend) newSprite(o *wlOutput, p presentation) {
	s := &wlSprite{output: o.reg, size: p.size, sprite: p.size, geo: spriteOrigin{applied: p.origin}}
	s.surf = C.wl_compositor_create_surface(b.compositor)
	C.wl_surface_add_listener(s.surf, &C.oni_surface_listener, nil)
	if b.fracMgr != nil {
		s.frac = C.wp_fractional_scale_manager_v1_get_fractional_scale(b.fracMgr, s.surf)
		C.wp_fractional_scale_v1_add_listener(s.frac, &C.oni_fractional_scale_listener, nil)
	}
	if b.viewporter != nil {
		s.pview = C.wp_viewporter_get_viewport(b.viewporter, s.surf)
	}
	ns := C.CString(Namespace)
	s.layer = C.zwlr_layer_shell_v1_get_layer_surface(b.layerShell, s.surf, o.o, C.ZWLR_LAYER_SHELL_V1_LAYER_TOP, ns)
	C.free(unsafe.Pointer(ns))
	C.zwlr_layer_surface_v1_add_listener(s.layer, &C.oni_layer_surface_listener, nil)
	C.zwlr_layer_surface_v1_set_anchor(s.layer, C.ZWLR_LAYER_SURFACE_V1_ANCHOR_TOP|C.ZWLR_LAYER_SURFACE_V1_ANCHOR_LEFT)
	// -1 places the margins against the output edges, ignoring bars.
	C.zwlr_layer_surface_v1_set_exclusive_zone(s.layer, -1)
	C.zwlr_layer_surface_v1_set_keyboard_interactivity(s.layer, 0)
	C.zwlr_layer_surface_v1_set_size(s.layer, C.uint32_t(p.size.X), C.uint32_t(p.size.Y))
	C.zwlr_layer_surface_v1_set_margin(s.layer, C.int32_t(p.origin.Y), 0, 0, C.int32_t(p.origin.X))
	b.setInput(s.surf, image.Rectangle{})
	s.child = C.wl_compositor_create_surface(b.compositor)
	s.sub = C.wl_subcompositor_get_subsurface(b.subcomp, s.child, s.surf)
	if b.viewporter != nil {
		s.view = C.wp_viewporter_get_viewport(b.viewporter, s.child)
	}
	// the pointer goes to the layer surface, whose origin spriteOrigin tracks
	b.setInput(s.child, image.Rectangle{})
	C.wl_surface_commit(s.child)
	C.wl_surface_commit(s.surf)
	b.sprite = s
}

func (b *wlBackend) setInput(surf *C.struct_wl_surface, rect image.Rectangle) {
	r := C.wl_compositor_create_region(b.compositor)
	if !rect.Empty() {
		C.wl_region_add(r, C.int32_t(rect.Min.X), C.int32_t(rect.Min.Y), C.int32_t(rect.Dx()), C.int32_t(rect.Dy()))
	}
	C.wl_surface_set_input_region(surf, r)
	C.wl_region_destroy(r)
}

func (b *wlBackend) present(p presentation) (image.Point, bool, error) {
	o := b.outputNamed(p.output)
	if o == nil {
		return image.Point{}, false, fmt.Errorf("output %q is gone", p.output)
	}
	if b.sprite != nil && b.sprite.output != o.reg {
		b.destroySprite()
	}
	s := b.sprite
	if s == nil {
		b.newSprite(o, p)
		return p.origin, false, nil
	}
	if !s.configured {
		return s.geo.requested().Add(s.childAt), false, nil
	}
	moved := s.geo.place(p)
	if moved {
		C.zwlr_layer_surface_v1_set_margin(s.layer, C.int32_t(p.origin.Y), 0, 0, C.int32_t(p.origin.X))
	}
	at := s.geo.requested()
	if at == p.origin {
		s.sprite = p.size
	}
	if err := b.fitLayer(s); err != nil {
		return at.Add(s.childAt), false, err
	}
	if child := p.origin.Sub(at); child != s.childAt {
		C.wl_subsurface_set_position(s.sub, C.int32_t(child.X), C.int32_t(child.Y))
		s.childAt = child
	}
	// the strand and the overlays stay where they are on the output while
	// the layer surface moves
	if t := s.strand; t != nil {
		placeStrand(t, at)
	}
	for _, t := range s.overlays {
		if t != nil {
			placeStrand(t, at)
		}
	}
	// a held press keeps the input region it was pressed in
	if input := p.input.Add(s.childAt); !p.grab && (!s.hasInput || input != s.input) {
		b.setInput(s.surf, input)
		s.input, s.hasInput = input, true
	}
	if p.img != nil {
		if err := b.attach(s.child, s.view, &s.bufScale, &s.buffers, p.img, p.size); err != nil {
			return at.Add(s.childAt), false, err
		}
	}
	// the confirmation first, so its done arrives before the frame's step
	if moved {
		s.confirm = C.wl_surface_frame(s.child)
		C.wl_callback_add_listener(s.confirm, &C.oni_callback_listener, nil)
	}
	if p.arm && s.frame == nil {
		s.frame = C.wl_surface_frame(s.child)
		s.frameOnStrand = false
		C.wl_callback_add_listener(s.frame, &C.oni_callback_listener, nil)
	}
	C.wl_surface_commit(s.child)
	C.wl_surface_commit(s.surf)
	return p.origin, true, nil
}

// fitLayer sizes the layer surface by layerSize for the sprite and every
// part out, and gives it its transparent buffer; the caller commits it.
func (b *wlBackend) fitLayer(s *wlSprite) error {
	var out image.Point
	if o := b.outs[s.output]; o != nil {
		out = o.logical.Size()
	}
	var parts [1 + overlayCount]image.Rectangle
	if s.strand != nil {
		parts[0] = s.strand.rect
	}
	for i, t := range s.overlays {
		if t != nil {
			parts[1+i] = t.rect
		}
	}
	if want := layerSize(s.sprite, s.geo.requested(), out, parts[:]...); want != s.size {
		C.zwlr_layer_surface_v1_set_size(s.layer, C.uint32_t(want.X), C.uint32_t(want.Y))
		s.size = want
	}
	return b.clearParent(s)
}

// clearParent gives the layer surface its transparent buffer: one pixel
// stretched over it through the viewport, else one of its size.
func (b *wlBackend) clearParent(s *wlSprite) error {
	want := image.Pt(1, 1)
	if s.pview == nil {
		want = s.size
	}
	if s.clear == nil || s.clear.w != want.X || s.clear.h != want.Y {
		buf, err := b.newBuffer(want.X, want.Y)
		if err != nil {
			return err
		}
		if s.clear != nil {
			s.clear.destroy()
		}
		s.clear = buf
		C.wl_surface_attach(s.surf, buf.buf, 0, 0)
		C.wl_surface_damage_buffer(s.surf, 0, 0, C.int32_t(want.X), C.int32_t(want.Y))
	}
	if s.pview != nil && s.clearDest != s.size {
		C.wp_viewport_set_destination(s.pview, C.int32_t(s.size.X), C.int32_t(s.size.Y))
		// Hyprland sizes a surface anew only when a buffer comes with the
		// commit, so a new destination brings the same pixel again
		C.wl_surface_attach(s.surf, s.clear.buf, 0, 0)
		C.wl_surface_damage_buffer(s.surf, 0, 0, 1, 1)
		s.clearDest = s.size
	}
	return nil
}

// attach copies img into a free buffer of pool and attaches it to surf,
// shown at dest logical px: through the viewport when there is one, else at
// the integer buffer scale img's width implies.
func (b *wlBackend) attach(surf *C.struct_wl_surface, view *C.struct_wp_viewport, bufScale *int, pool *[]*shmBuffer, img *image.NRGBA, dest image.Point) error {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if w <= 0 || h <= 0 {
		return nil
	}
	var buf *shmBuffer
	live := (*pool)[:0]
	for _, x := range *pool {
		if x.w != w || x.h != h {
			if x.busy {
				x.dead = true
				live = append(live, x)
			} else {
				x.destroy()
			}
			continue
		}
		live = append(live, x)
		if buf == nil && !x.busy {
			buf = x
		}
	}
	*pool = live
	if buf == nil {
		var err error
		if buf, err = b.newBuffer(w, h); err != nil {
			return err
		}
		*pool = append(*pool, buf)
	}
	toARGB8888(buf.data, img)
	if view != nil {
		C.wp_viewport_set_destination(view, C.int32_t(dest.X), C.int32_t(dest.Y))
	} else {
		scale := max(1, int(math.Round(float64(w)/float64(max(dest.X, 1)))))
		if scale != *bufScale {
			C.wl_surface_set_buffer_scale(surf, C.int32_t(scale))
			*bufScale = scale
		}
	}
	C.wl_surface_attach(surf, buf.buf, 0, 0)
	C.wl_surface_damage_buffer(surf, 0, 0, C.int32_t(w), C.int32_t(h))
	buf.busy = true
	return nil
}

// strandScale caps the strand's device scale at limit; without a viewporter
// a buffer scale must be a whole number, so the cap rounds down.
func (b *wlBackend) strandScale(output string, limit float64) float64 {
	s := b.scale(output)
	if !(limit >= 1) {
		limit = 1
	}
	if b.viewporter == nil {
		return max(1, min(s, math.Floor(limit)))
	}
	return min(s, limit)
}

func (b *wlBackend) presentStrand(p strandPresentation) (bool, error) {
	o := b.outputNamed(p.output)
	if o == nil {
		return false, fmt.Errorf("output %q is gone", p.output)
	}
	s := b.sprite
	if s == nil || s.output != o.reg || !s.configured || p.rect.Empty() {
		return false, nil
	}
	t := s.strand
	if t == nil {
		t = &wlStrand{}
		t.surf = C.wl_compositor_create_surface(b.compositor)
		t.sub = C.wl_subcompositor_get_subsurface(b.subcomp, t.surf, s.surf)
		C.wl_subsurface_place_below(t.sub, s.child)
		if b.viewporter != nil {
			t.view = C.wp_viewporter_get_viewport(b.viewporter, t.surf)
		}
		b.setInput(t.surf, image.Rectangle{})
		s.strand = t
	}
	t.origin, t.rect = p.rect.Min, p.rect
	placeStrand(t, s.geo.requested())
	if p.img != nil {
		if err := b.attach(t.surf, t.view, &t.bufScale, &t.buffers, p.img, p.rect.Size()); err != nil {
			return false, err
		}
	}
	if p.arm && s.frame == nil {
		s.frame = C.wl_surface_frame(t.surf)
		s.frameOnStrand = true
		C.wl_callback_add_listener(s.frame, &C.oni_callback_listener, nil)
	}
	C.wl_surface_commit(t.surf)
	if err := b.fitLayer(s); err != nil {
		return false, err
	}
	// the subsurface's position and stacking apply with the parent's commit
	C.wl_surface_commit(s.surf)
	return true, nil
}

func (b *wlBackend) hideStrand() {
	s := b.sprite
	if s == nil || s.strand == nil {
		return
	}
	if s.frame != nil && s.frameOnStrand {
		C.wl_callback_destroy(s.frame)
		s.frame = C.wl_surface_frame(s.child)
		s.frameOnStrand = false
		C.wl_callback_add_listener(s.frame, &C.oni_callback_listener, nil)
		C.wl_surface_commit(s.child)
	}
	destroyStrand(s.strand)
	s.strand = nil
	b.shrinkLayer(s)
	C.wl_surface_commit(s.surf)
}

func (b *wlBackend) presentOverlay(which overlay, p overlayPresentation) (bool, error) {
	o := b.outputNamed(p.output)
	if o == nil {
		return false, fmt.Errorf("output %q is gone", p.output)
	}
	s := b.sprite
	if s == nil || s.output != o.reg || !s.configured || p.rect.Empty() {
		return false, nil
	}
	t := s.overlays[which]
	if t == nil {
		t = &wlStrand{}
		t.surf = C.wl_compositor_create_surface(b.compositor)
		t.sub = C.wl_subcompositor_get_subsurface(b.subcomp, t.surf, s.surf)
		switch {
		case which == overlayChip || which == overlayCard || which == overlayPill || which == overlayHandoff || which == overlayNotice:
			C.wl_subsurface_place_above(t.sub, s.child)
		case s.strand != nil:
			C.wl_subsurface_place_below(t.sub, s.strand.surf)
		default:
			C.wl_subsurface_place_below(t.sub, s.child)
		}
		if b.viewporter != nil {
			t.view = C.wp_viewporter_get_viewport(b.viewporter, t.surf)
		}
		b.setInput(t.surf, p.input)
		t.input = p.input
		s.overlays[which] = t
	} else if p.input != t.input {
		b.setInput(t.surf, p.input)
		t.input = p.input
	}
	t.origin, t.rect = p.rect.Min, p.rect
	placeStrand(t, s.geo.requested())
	if p.img != nil {
		if err := b.attach(t.surf, t.view, &t.bufScale, &t.buffers, p.img, p.rect.Size()); err != nil {
			return false, err
		}
	}
	C.wl_surface_commit(t.surf)
	if err := b.fitLayer(s); err != nil {
		return false, err
	}
	C.wl_surface_commit(s.surf)
	return true, nil
}

func (b *wlBackend) hideOverlay(which overlay) {
	s := b.sprite
	if s == nil || s.overlays[which] == nil {
		return
	}
	if b.pointerOnOv && b.pointerOv == which {
		b.pointerOnOv = false
	}
	destroyStrand(s.overlays[which])
	s.overlays[which] = nil
	b.shrinkLayer(s)
	C.wl_surface_commit(s.surf)
}

// setPick maps a pick surface on each of outputs and destroys the rest.
func (b *wlBackend) setPick(outputs []string) {
	for name, p := range b.picks {
		if !slices.Contains(outputs, name) {
			b.destroyPick(p)
		}
	}
	for _, name := range outputs {
		if b.picks[name] != nil {
			continue
		}
		if o := b.outputNamed(name); o != nil {
			b.newPick(o)
		}
	}
}

func (b *wlBackend) newPick(o *wlOutput) {
	p := &wlPick{name: o.name, output: o.reg}
	p.surf = C.wl_compositor_create_surface(b.compositor)
	if b.viewporter != nil {
		p.view = C.wp_viewporter_get_viewport(b.viewporter, p.surf)
	}
	ns := C.CString(Namespace + "-pick")
	p.layer = C.zwlr_layer_shell_v1_get_layer_surface(b.layerShell, p.surf, o.o, C.ZWLR_LAYER_SHELL_V1_LAYER_OVERLAY, ns)
	C.free(unsafe.Pointer(ns))
	C.zwlr_layer_surface_v1_add_listener(p.layer, &C.oni_layer_surface_listener, nil)
	C.zwlr_layer_surface_v1_set_anchor(p.layer, C.ZWLR_LAYER_SURFACE_V1_ANCHOR_TOP|C.ZWLR_LAYER_SURFACE_V1_ANCHOR_BOTTOM|C.ZWLR_LAYER_SURFACE_V1_ANCHOR_LEFT|C.ZWLR_LAYER_SURFACE_V1_ANCHOR_RIGHT)
	C.zwlr_layer_surface_v1_set_exclusive_zone(p.layer, -1)
	C.zwlr_layer_surface_v1_set_keyboard_interactivity(p.layer, 0)
	C.zwlr_layer_surface_v1_set_size(p.layer, 0, 0)
	C.wl_surface_commit(p.surf)
	b.picks[o.name] = p
}

// configurePick gives a configured pick surface its transparent pixels at
// the size the compositor gave it (the output's, when it gave none).
func (b *wlBackend) configurePick(p *wlPick, size image.Point) {
	if size.X <= 0 || size.Y <= 0 {
		if o := b.outs[p.output]; o != nil {
			size = o.logical.Size()
		}
	}
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	want := image.Pt(1, 1)
	if p.view == nil {
		want = size
	}
	if p.buf == nil || p.buf.w != want.X || p.buf.h != want.Y {
		buf, err := b.newBuffer(want.X, want.Y)
		if err != nil {
			return
		}
		if p.buf != nil {
			p.buf.destroy()
		}
		p.buf = buf
	}
	if p.view != nil && size != p.size {
		C.wp_viewport_set_destination(p.view, C.int32_t(size.X), C.int32_t(size.Y))
	}
	p.size = size
	C.wl_surface_attach(p.surf, p.buf.buf, 0, 0)
	C.wl_surface_damage_buffer(p.surf, 0, 0, C.int32_t(want.X), C.int32_t(want.Y))
	C.wl_surface_commit(p.surf)
}

func (b *wlBackend) destroyPick(p *wlPick) {
	if b.pointerPick == p {
		b.pointerPick, b.pickDown = nil, false
	}
	if p.view != nil {
		C.wp_viewport_destroy(p.view)
	}
	C.zwlr_layer_surface_v1_destroy(p.layer)
	C.wl_surface_destroy(p.surf)
	if p.buf != nil {
		p.buf.destroy()
	}
	delete(b.picks, p.name)
}

func (b *wlBackend) pickBySurface(surf *C.struct_wl_surface) *wlPick {
	for _, p := range b.picks {
		if p.surf == surf {
			return p
		}
	}
	return nil
}

func (b *wlBackend) pickByLayer(ls *C.struct_zwlr_layer_surface_v1) *wlPick {
	for _, p := range b.picks {
		if p.layer == ls {
			return p
		}
	}
	return nil
}

// glowScale caps the glow's device scale at glowDPR; without a viewporter
// a buffer scale is whole, so the glow draws at one px per logical px.
func (b *wlBackend) glowScale(output string) float64 {
	if b.viewporter == nil {
		return 1
	}
	return min(b.scale(output), glowDPR)
}

func (b *wlBackend) presentGlow(p glowPresentation) (bool, error) {
	o := b.outputNamed(p.output)
	if o == nil {
		return false, fmt.Errorf("output %q is gone", p.output)
	}
	g := b.glows[p.output]
	if g != nil && g.output != o.reg {
		b.destroyGlow(g)
		g = nil
	}
	if g == nil {
		g = &wlGlow{name: o.name, output: o.reg}
		b.glows[o.name] = g
	}
	ready := true
	for i, r := range p.rects {
		t := g.strips[i]
		if t != nil && t.rect != r {
			b.destroyGlowStrip(t)
			g.strips[i], t = nil, nil
		}
		if r.Empty() {
			continue
		}
		if t == nil {
			t = b.newGlowStrip(o, r)
			g.strips[i] = t
		}
		ready = ready && t.configured
	}
	if !ready {
		return false, nil
	}
	for i, t := range g.strips {
		if t == nil || p.imgs[i] == nil {
			continue
		}
		if err := b.attach(t.surf, t.view, &t.bufScale, &t.buffers, p.imgs[i], t.rect.Size()); err != nil {
			return false, err
		}
		C.wl_surface_commit(t.surf)
	}
	return true, nil
}

func (b *wlBackend) newGlowStrip(o *wlOutput, r image.Rectangle) *wlGlowStrip {
	t := &wlGlowStrip{rect: r}
	t.surf = C.wl_compositor_create_surface(b.compositor)
	if b.viewporter != nil {
		t.view = C.wp_viewporter_get_viewport(b.viewporter, t.surf)
	}
	ns := C.CString(Namespace + "-glow")
	t.layer = C.zwlr_layer_shell_v1_get_layer_surface(b.layerShell, t.surf, o.o, C.ZWLR_LAYER_SHELL_V1_LAYER_OVERLAY, ns)
	C.free(unsafe.Pointer(ns))
	C.zwlr_layer_surface_v1_add_listener(t.layer, &C.oni_layer_surface_listener, nil)
	C.zwlr_layer_surface_v1_set_anchor(t.layer, C.ZWLR_LAYER_SURFACE_V1_ANCHOR_TOP|C.ZWLR_LAYER_SURFACE_V1_ANCHOR_LEFT)
	C.zwlr_layer_surface_v1_set_exclusive_zone(t.layer, -1)
	C.zwlr_layer_surface_v1_set_keyboard_interactivity(t.layer, 0)
	C.zwlr_layer_surface_v1_set_size(t.layer, C.uint32_t(r.Dx()), C.uint32_t(r.Dy()))
	C.zwlr_layer_surface_v1_set_margin(t.layer, C.int32_t(r.Min.Y), 0, 0, C.int32_t(r.Min.X))
	b.setInput(t.surf, image.Rectangle{})
	C.wl_surface_commit(t.surf)
	return t
}

func (b *wlBackend) hideGlow(keep []string) {
	for name, g := range b.glows {
		if !slices.Contains(keep, name) {
			b.destroyGlow(g)
		}
	}
}

func (b *wlBackend) destroyGlow(g *wlGlow) {
	for i, t := range g.strips {
		if t != nil {
			b.destroyGlowStrip(t)
			g.strips[i] = nil
		}
	}
	delete(b.glows, g.name)
}

func (b *wlBackend) destroyGlowStrip(t *wlGlowStrip) {
	if t.view != nil {
		C.wp_viewport_destroy(t.view)
	}
	C.zwlr_layer_surface_v1_destroy(t.layer)
	C.wl_surface_destroy(t.surf)
	for _, buf := range t.buffers {
		buf.destroy()
	}
}

// glowByLayer is the glow and its strip whose layer surface is ls.
func (b *wlBackend) glowByLayer(ls *C.struct_zwlr_layer_surface_v1) (*wlGlow, *wlGlowStrip) {
	for _, g := range b.glows {
		for _, t := range g.strips {
			if t != nil && t.layer == ls {
				return g, t
			}
		}
	}
	return nil, nil
}

// ovPointer and ovButton hand the pointer over an overlay to its sink
// call; the highlight takes no pointer.
func (b *wlBackend) ovPointer(which overlay, p *spec.Point) {
	switch which {
	case overlayChip:
		b.sink.chipPointer(p)
	case overlayCard:
		b.sink.cardPointer(p)
	case overlayPill, overlayHandoff, overlayNotice:
		b.sink.handsPointer(which, p)
	}
}

func (b *wlBackend) ovButton(which overlay, pressed bool) {
	switch which {
	case overlayChip:
		b.sink.chipButton(pressed)
	case overlayCard:
		b.sink.cardButton(pressed)
	case overlayPill, overlayHandoff, overlayNotice:
		b.sink.handsButton(which, pressed)
	}
}

// shrinkLayer refits the layer surface once a part is gone. Only a layer
// surface without a viewport needs a new buffer to shrink; when that fails
// it keeps the larger transparent one, which shows nothing and takes no
// pointer events, until the next present fits it.
func (b *wlBackend) shrinkLayer(s *wlSprite) {
	_ = b.fitLayer(s)
}

// placeStrand keeps the strand at its output position relative to a layer
// surface whose requested origin is parent.
func placeStrand(t *wlStrand, parent image.Point) {
	if at := t.origin.Sub(parent); !t.placed || at != t.at {
		C.wl_subsurface_set_position(t.sub, C.int32_t(at.X), C.int32_t(at.Y))
		t.at, t.placed = at, true
	}
}

func destroyStrand(t *wlStrand) {
	if t.view != nil {
		C.wp_viewport_destroy(t.view)
	}
	C.wl_subsurface_destroy(t.sub)
	C.wl_surface_destroy(t.surf)
	for _, buf := range t.buffers {
		buf.destroy()
	}
}

func (b *wlBackend) newBuffer(w, h int) (*shmBuffer, error) {
	size := w * h * 4
	fd, err := unix.MemfdCreate("hyakki-oni", unix.MFD_CLOEXEC)
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)
	if err := unix.Ftruncate(fd, int64(size)); err != nil {
		return nil, err
	}
	data, err := unix.Mmap(fd, 0, size, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return nil, err
	}
	pool := C.wl_shm_create_pool(b.shm, C.int32_t(fd), C.int32_t(size))
	buf := C.wl_shm_pool_create_buffer(pool, 0, C.int32_t(w), C.int32_t(h), C.int32_t(w*4), C.WL_SHM_FORMAT_ARGB8888)
	C.wl_shm_pool_destroy(pool)
	C.wl_buffer_add_listener(buf, &C.oni_buffer_listener, nil)
	return &shmBuffer{buf: buf, data: data, w: w, h: h}, nil
}

func (x *shmBuffer) destroy() {
	C.wl_buffer_destroy(x.buf)
	unix.Munmap(x.data)
}

// toGlobal turns a pointer position on the layer surface into global logical
// px; false while a move of it is unconfirmed (see spriteOrigin).
func (b *wlBackend) toGlobal(sx, sy C.wl_fixed_t) (spec.Point, bool) {
	s := b.sprite
	if s == nil {
		return spec.Point{}, false
	}
	o := b.outs[s.output]
	if o == nil {
		return spec.Point{}, false
	}
	return s.geo.global(o.logical.Min, float64(C.wl_fixed_to_double(sx)), float64(C.wl_fixed_to_double(sy)))
}

func (b *wlBackend) outputChanged(o *wlOutput) {
	if o.name == "" {
		o.name = fmt.Sprintf("output-%d", o.reg)
	}
	if o.xdg == nil || o.logical.Empty() {
		s := max(o.scale, 1)
		o.logical = image.Rectangle{Min: o.geom, Max: o.geom.Add(image.Pt(o.mode.X/s, o.mode.Y/s))}
	}
	o.ready = true
	if b.sink != nil {
		b.sink.outputsChanged()
	}
}

func (b *wlBackend) bindXdgOutput(o *wlOutput) {
	if b.xdgOutMgr == nil || o.xdg != nil {
		return
	}
	o.xdg = C.zxdg_output_manager_v1_get_xdg_output(b.xdgOutMgr, o.o)
	C.zxdg_output_v1_add_listener(o.xdg, &C.oni_xdg_output_listener, nil)
}

func (b *wlBackend) outputByProxy(p *C.struct_wl_output) *wlOutput {
	for _, o := range b.outs {
		if o.o == p {
			return o
		}
	}
	return nil
}

func (b *wlBackend) outputByXdg(p *C.struct_zxdg_output_v1) *wlOutput {
	for _, o := range b.outs {
		if o.xdg == p {
			return o
		}
	}
	return nil
}

func bindVersion(advertised, want uint32) C.uint32_t { return C.uint32_t(min(advertised, want)) }

func (b *wlBackend) bindToplevels(name C.uint32_t, version uint32) {
	b.topMgr = (*C.struct_zwlr_foreign_toplevel_manager_v1)(C.wl_registry_bind(b.reg, name, &C.zwlr_foreign_toplevel_manager_v1_interface, bindVersion(version, 3)))
	C.zwlr_foreign_toplevel_manager_v1_add_listener(b.topMgr, &C.oni_toplevel_manager_listener, nil)
}

//export oni_onGlobal
func oni_onGlobal(data unsafe.Pointer, reg *C.struct_wl_registry, name C.uint32_t, iface *C.char, version C.uint32_t) {
	b := wl
	ifn := C.GoString(iface)
	v := uint32(version)
	if slices.Contains(Protocols, ifn) {
		b.advertised[ifn] = max(b.advertised[ifn], v)
	}
	if b.sink == nil {
		return
	}
	switch ifn {
	case "wl_compositor":
		if v < 4 {
			return
		}
		b.compositor = (*C.struct_wl_compositor)(C.wl_registry_bind(reg, name, &C.wl_compositor_interface, 4))
	case "wl_shm":
		b.shm = (*C.struct_wl_shm)(C.wl_registry_bind(reg, name, &C.wl_shm_interface, 1))
	case "wl_subcompositor":
		b.subcomp = (*C.struct_wl_subcompositor)(C.wl_registry_bind(reg, name, &C.wl_subcompositor_interface, 1))
	case "zwlr_layer_shell_v1":
		b.layerShell = (*C.struct_zwlr_layer_shell_v1)(C.wl_registry_bind(reg, name, &C.zwlr_layer_shell_v1_interface, bindVersion(v, 4)))
	case "wp_viewporter":
		b.viewporter = (*C.struct_wp_viewporter)(C.wl_registry_bind(reg, name, &C.wp_viewporter_interface, 1))
	case "wp_fractional_scale_manager_v1":
		b.fracMgr = (*C.struct_wp_fractional_scale_manager_v1)(C.wl_registry_bind(reg, name, &C.wp_fractional_scale_manager_v1_interface, 1))
	case "zwlr_foreign_toplevel_manager_v1":
		if b.topMgr != nil {
			return
		}
		b.topName, b.topVersion = name, v
		if b.outputsBound {
			b.bindToplevels(name, v)
		}
	case "zxdg_output_manager_v1":
		b.xdgOutMgr = (*C.struct_zxdg_output_manager_v1)(C.wl_registry_bind(reg, name, &C.zxdg_output_manager_v1_interface, bindVersion(v, 3)))
		for _, o := range b.outs {
			b.bindXdgOutput(o)
		}
	case "wl_seat":
		if b.seat != nil {
			return
		}
		b.seat = (*C.struct_wl_seat)(C.wl_registry_bind(reg, name, &C.wl_seat_interface, bindVersion(v, 5)))
		b.seatName = name
		C.wl_seat_add_listener(b.seat, &C.oni_seat_listener, nil)
	case "wl_output":
		o := &wlOutput{reg: name}
		o.o = (*C.struct_wl_output)(C.wl_registry_bind(reg, name, &C.wl_output_interface, bindVersion(v, 4)))
		C.wl_output_add_listener(o.o, &C.oni_output_listener, nil)
		b.outs[name] = o
		b.bindXdgOutput(o)
	}
}

//export oni_onGlobalRemove
func oni_onGlobalRemove(data unsafe.Pointer, reg *C.struct_wl_registry, name C.uint32_t) {
	b := wl
	if o, ok := b.outs[name]; ok {
		if b.sprite != nil && b.sprite.output == name {
			b.destroySprite()
		}
		for _, pk := range b.picks {
			if pk.output == name {
				b.destroyPick(pk)
			}
		}
		for _, g := range b.glows {
			if g.output == name {
				b.destroyGlow(g)
			}
		}
		if o.xdg != nil {
			C.zxdg_output_v1_destroy(o.xdg)
		}
		C.oni_output_release(o.o)
		delete(b.outs, name)
		for _, t := range b.tops {
			delete(t.outputs, name)
		}
		if b.sink != nil {
			b.sink.outputsChanged()
			b.sink.coveredChanged()
		}
	}
	if b.seat != nil && name == b.seatName {
		if b.pointer != nil {
			C.oni_pointer_release(b.pointer)
			b.pointer = nil
		}
		C.oni_seat_release(b.seat)
		b.seat = nil
	}
}

//export oni_onOutputGeometry
func oni_onOutputGeometry(data unsafe.Pointer, out *C.struct_wl_output, x, y, pw, ph, subpixel C.int32_t, mk, model *C.char, transform C.int32_t) {
	if o := wl.outputByProxy(out); o != nil {
		o.geom = image.Pt(int(x), int(y))
	}
}

//export oni_onOutputMode
func oni_onOutputMode(data unsafe.Pointer, out *C.struct_wl_output, flags C.uint32_t, w, h, refresh C.int32_t) {
	if flags&C.WL_OUTPUT_MODE_CURRENT == 0 {
		return
	}
	if o := wl.outputByProxy(out); o != nil {
		o.mode = image.Pt(int(w), int(h))
	}
}

//export oni_onOutputScale
func oni_onOutputScale(data unsafe.Pointer, out *C.struct_wl_output, scale C.int32_t) {
	if o := wl.outputByProxy(out); o != nil {
		o.scale = int(scale)
	}
}

//export oni_onOutputName
func oni_onOutputName(data unsafe.Pointer, out *C.struct_wl_output, name *C.char) {
	if o := wl.outputByProxy(out); o != nil {
		o.name = C.GoString(name)
	}
}

//export oni_onOutputDescription
func oni_onOutputDescription(data unsafe.Pointer, out *C.struct_wl_output, desc *C.char) {}

//export oni_onOutputDone
func oni_onOutputDone(data unsafe.Pointer, out *C.struct_wl_output) {
	if o := wl.outputByProxy(out); o != nil {
		wl.outputChanged(o)
		if s := wl.sprite; s != nil && s.output == o.reg && s.preferred == 0 && wl.sink != nil {
			wl.sink.scaleChanged()
		}
	}
}

//export oni_onXdgOutputPosition
func oni_onXdgOutputPosition(data unsafe.Pointer, xo *C.struct_zxdg_output_v1, x, y C.int32_t) {
	if o := wl.outputByXdg(xo); o != nil {
		o.logical = image.Rectangle{Min: image.Pt(int(x), int(y)), Max: image.Pt(int(x), int(y)).Add(o.logical.Size())}
	}
}

//export oni_onXdgOutputSize
func oni_onXdgOutputSize(data unsafe.Pointer, xo *C.struct_zxdg_output_v1, w, h C.int32_t) {
	if o := wl.outputByXdg(xo); o != nil {
		o.logical.Max = o.logical.Min.Add(image.Pt(int(w), int(h)))
	}
}

//export oni_onXdgOutputDone
func oni_onXdgOutputDone(data unsafe.Pointer, xo *C.struct_zxdg_output_v1) {
	if o := wl.outputByXdg(xo); o != nil && o.ready {
		wl.outputChanged(o)
	}
}

//export oni_onXdgOutputName
func oni_onXdgOutputName(data unsafe.Pointer, xo *C.struct_zxdg_output_v1, name *C.char) {
	if o := wl.outputByXdg(xo); o != nil && o.name == "" {
		o.name = C.GoString(name)
	}
}

//export oni_onXdgOutputDescription
func oni_onXdgOutputDescription(data unsafe.Pointer, xo *C.struct_zxdg_output_v1, desc *C.char) {}

//export oni_onSeatCapabilities
func oni_onSeatCapabilities(data unsafe.Pointer, seat *C.struct_wl_seat, caps C.uint32_t) {
	b := wl
	has := caps&C.WL_SEAT_CAPABILITY_POINTER != 0
	switch {
	case has && b.pointer == nil:
		b.pointer = C.wl_seat_get_pointer(seat)
		C.wl_pointer_add_listener(b.pointer, &C.oni_pointer_listener, nil)
	case !has && b.pointer != nil:
		C.oni_pointer_release(b.pointer)
		b.pointer = nil
	}
}

//export oni_onSeatName
func oni_onSeatName(data unsafe.Pointer, seat *C.struct_wl_seat, name *C.char) {}

//export oni_onPointerEnter
func oni_onPointerEnter(data unsafe.Pointer, p *C.struct_wl_pointer, serial C.uint32_t, surf *C.struct_wl_surface, sx, sy C.wl_fixed_t) {
	b := wl
	if pk := b.pickBySurface(surf); pk != nil {
		b.pointerPick, b.pickDown = pk, false
		b.pickAt = *chipLocal(sx, sy)
		return
	}
	b.pointerIn = b.sprite != nil && surf == b.sprite.surf
	if pt, ok := b.toGlobal(sx, sy); ok && b.pointerIn {
		b.sink.pointerMotion(pt)
	}
	if s := b.sprite; s != nil {
		for _, which := range []overlay{overlayChip, overlayCard, overlayPill, overlayHandoff, overlayNotice} {
			if t := s.overlays[which]; t != nil && surf == t.surf {
				b.pointerOv, b.pointerOnOv = which, true
				b.ovPointer(which, chipLocal(sx, sy))
			}
		}
	}
}

// chipLocal is a pointer position on an overlay or a pick surface,
// surface-local logical px.
func chipLocal(sx, sy C.wl_fixed_t) *spec.Point {
	return &spec.Point{X: float64(C.wl_fixed_to_double(sx)), Y: float64(C.wl_fixed_to_double(sy))}
}

//export oni_onPointerLeave
func oni_onPointerLeave(data unsafe.Pointer, p *C.struct_wl_pointer, serial C.uint32_t, surf *C.struct_wl_surface) {
	wl.pointerPick, wl.pickDown = nil, false
	if wl.pointerIn {
		wl.pointerIn = false
		wl.sink.pointerLeave()
	}
	if wl.pointerOnOv {
		wl.pointerOnOv = false
		wl.ovPointer(wl.pointerOv, nil)
	}
}

//export oni_onPointerMotion
func oni_onPointerMotion(data unsafe.Pointer, p *C.struct_wl_pointer, t C.uint32_t, sx, sy C.wl_fixed_t) {
	if wl.pointerPick != nil {
		wl.pickAt = *chipLocal(sx, sy)
		return
	}
	if wl.pointerOnOv {
		wl.ovPointer(wl.pointerOv, chipLocal(sx, sy))
		return
	}
	if !wl.pointerIn {
		return
	}
	if pt, ok := wl.toGlobal(sx, sy); ok {
		wl.sink.pointerMotion(pt)
	}
}

//export oni_onPointerButton
func oni_onPointerButton(data unsafe.Pointer, p *C.struct_wl_pointer, serial, t, button, state C.uint32_t) {
	pressed := state == C.WL_POINTER_BUTTON_STATE_PRESSED
	if pk := wl.pointerPick; pk != nil {
		switch {
		case button == btnLeft && pressed:
			wl.pickDown = true
		case button == btnLeft && wl.pickDown:
			wl.pickDown = false
			wl.sink.pickClick(pk.name, wl.pickAt)
		case button == btnRight && !pressed:
			wl.sink.pickCancel()
		}
		return
	}
	if button != btnLeft {
		return
	}
	switch {
	case wl.pointerOnOv:
		wl.ovButton(wl.pointerOv, pressed)
	case wl.pointerIn:
		wl.sink.button(pressed)
	}
}

//export oni_onPointerAxis
func oni_onPointerAxis(data unsafe.Pointer, p *C.struct_wl_pointer, t, axis C.uint32_t, value C.wl_fixed_t) {
}

//export oni_onPointerFrame
func oni_onPointerFrame(data unsafe.Pointer, p *C.struct_wl_pointer) {}

//export oni_onPointerAxisSource
func oni_onPointerAxisSource(data unsafe.Pointer, p *C.struct_wl_pointer, source C.uint32_t) {}

//export oni_onPointerAxisStop
func oni_onPointerAxisStop(data unsafe.Pointer, p *C.struct_wl_pointer, t, axis C.uint32_t) {}

//export oni_onPointerAxisDiscrete
func oni_onPointerAxisDiscrete(data unsafe.Pointer, p *C.struct_wl_pointer, axis C.uint32_t, discrete C.int32_t) {
}

//export oni_onSurfaceEnter
func oni_onSurfaceEnter(data unsafe.Pointer, surf *C.struct_wl_surface, out *C.struct_wl_output) {
	if s := wl.sprite; s != nil && s.surf == surf {
		if o := wl.outputByProxy(out); o != nil {
			s.entered = o.reg
		}
	}
}

//export oni_onSurfaceLeave
func oni_onSurfaceLeave(data unsafe.Pointer, surf *C.struct_wl_surface, out *C.struct_wl_output) {}

//export oni_onCallbackDone
func oni_onCallbackDone(data unsafe.Pointer, cb *C.struct_wl_callback, t C.uint32_t) {
	b := wl
	s := b.sprite
	C.wl_callback_destroy(cb)
	if s == nil {
		return
	}
	switch cb {
	case s.frame:
		s.frame = nil
		b.sink.frameDone(time.Now())
	case s.confirm:
		s.confirm = nil
		s.geo.rendered()
	}
}

//export oni_onBufferRelease
func oni_onBufferRelease(data unsafe.Pointer, buf *C.struct_wl_buffer) {
	for _, g := range wl.glows {
		for _, t := range g.strips {
			if t != nil && release(&t.buffers, buf) {
				return
			}
		}
	}
	s := wl.sprite
	if s == nil {
		return
	}
	if release(&s.buffers, buf) || (s.strand != nil && release(&s.strand.buffers, buf)) {
		return
	}
	for _, t := range s.overlays {
		if t != nil && release(&t.buffers, buf) {
			return
		}
	}
}

// release marks buf free if pool holds it, destroying it if it was retired.
func release(pool *[]*shmBuffer, buf *C.struct_wl_buffer) bool {
	for i, x := range *pool {
		if x.buf != buf {
			continue
		}
		x.busy = false
		if x.dead {
			x.destroy()
			*pool = slices.Delete(*pool, i, i+1)
		}
		return true
	}
	return false
}

//export oni_onLayerConfigure
func oni_onLayerConfigure(data unsafe.Pointer, ls *C.struct_zwlr_layer_surface_v1, serial, w, h C.uint32_t) {
	if pk := wl.pickByLayer(ls); pk != nil {
		C.zwlr_layer_surface_v1_ack_configure(ls, serial)
		wl.configurePick(pk, image.Pt(int(w), int(h)))
		return
	}
	if g, t := wl.glowByLayer(ls); t != nil {
		C.zwlr_layer_surface_v1_ack_configure(ls, serial)
		if t.configured {
			return
		}
		t.configured = true
		for _, x := range g.strips {
			if x != nil && !x.configured {
				return
			}
		}
		wl.sink.glowConfigured(g.name)
		return
	}
	s := wl.sprite
	if s == nil || s.layer != ls {
		return
	}
	C.zwlr_layer_surface_v1_ack_configure(ls, serial)
	if s.configured {
		return
	}
	s.configured = true
	wl.sink.configured()
}

//export oni_onLayerClosed
func oni_onLayerClosed(data unsafe.Pointer, ls *C.struct_zwlr_layer_surface_v1) {
	if pk := wl.pickByLayer(ls); pk != nil {
		wl.destroyPick(pk)
		return
	}
	if g, _ := wl.glowByLayer(ls); g != nil {
		wl.destroyGlow(g)
		return
	}
	if s := wl.sprite; s != nil && s.layer == ls {
		wl.destroySprite()
		wl.sink.surfaceClosed()
	}
}

//export oni_onPreferredScale
func oni_onPreferredScale(data unsafe.Pointer, fs *C.struct_wp_fractional_scale_v1, scale C.uint32_t) {
	s := wl.sprite
	if s == nil || s.frac != fs {
		return
	}
	v := float64(scale) / 120
	if v != s.preferred {
		s.preferred = v
		wl.sink.scaleChanged()
	}
}

//export oni_onToplevel
func oni_onToplevel(data unsafe.Pointer, mgr *C.struct_zwlr_foreign_toplevel_manager_v1, h *C.struct_zwlr_foreign_toplevel_handle_v1) {
	wl.tops[h] = &wlToplevel{outputs: map[C.uint32_t]bool{}}
	C.zwlr_foreign_toplevel_handle_v1_add_listener(h, &C.oni_toplevel_listener, nil)
}

//export oni_onToplevelManagerFinished
func oni_onToplevelManagerFinished(data unsafe.Pointer, mgr *C.struct_zwlr_foreign_toplevel_manager_v1) {
	C.zwlr_foreign_toplevel_manager_v1_destroy(mgr)
	wl.topMgr = nil
}

//export oni_onToplevelTitle
func oni_onToplevelTitle(data unsafe.Pointer, h *C.struct_zwlr_foreign_toplevel_handle_v1, title *C.char) {
}

//export oni_onToplevelAppID
func oni_onToplevelAppID(data unsafe.Pointer, h *C.struct_zwlr_foreign_toplevel_handle_v1, id *C.char) {
}

//export oni_onToplevelOutputEnter
func oni_onToplevelOutputEnter(data unsafe.Pointer, h *C.struct_zwlr_foreign_toplevel_handle_v1, out *C.struct_wl_output) {
	if t, o := wl.tops[h], wl.outputByProxy(out); t != nil && o != nil {
		t.outputs[o.reg] = true
	}
}

//export oni_onToplevelOutputLeave
func oni_onToplevelOutputLeave(data unsafe.Pointer, h *C.struct_zwlr_foreign_toplevel_handle_v1, out *C.struct_wl_output) {
	if t, o := wl.tops[h], wl.outputByProxy(out); t != nil && o != nil {
		delete(t.outputs, o.reg)
	}
}

//export oni_onToplevelState
func oni_onToplevelState(data unsafe.Pointer, h *C.struct_zwlr_foreign_toplevel_handle_v1, st *C.struct_wl_array) {
	t := wl.tops[h]
	if t == nil {
		return
	}
	t.pendingFS, t.pendingMn = false, false
	for i := range C.size_t(st.size / 4) {
		switch C.oni_array_u32(st, i) {
		case C.ZWLR_FOREIGN_TOPLEVEL_HANDLE_V1_STATE_FULLSCREEN:
			t.pendingFS = true
		case C.ZWLR_FOREIGN_TOPLEVEL_HANDLE_V1_STATE_MINIMIZED:
			t.pendingMn = true
		}
	}
}

//export oni_onToplevelDone
func oni_onToplevelDone(data unsafe.Pointer, h *C.struct_zwlr_foreign_toplevel_handle_v1) {
	t := wl.tops[h]
	if t == nil {
		return
	}
	t.fullscreen, t.minimized = t.pendingFS, t.pendingMn
	if wl.sink != nil {
		wl.sink.coveredChanged()
	}
}

//export oni_onToplevelClosed
func oni_onToplevelClosed(data unsafe.Pointer, h *C.struct_zwlr_foreign_toplevel_handle_v1) {
	delete(wl.tops, h)
	C.zwlr_foreign_toplevel_handle_v1_destroy(h)
	if wl.sink != nil {
		wl.sink.coveredChanged()
	}
}

//export oni_onToplevelParent
func oni_onToplevelParent(data unsafe.Pointer, h, parent *C.struct_zwlr_foreign_toplevel_handle_v1) {
}

// ListGlobals connects, lists the protocols the compositor offers and
// disconnects.
func ListGlobals() (Globals, error) {
	b, err := connectWayland(nil)
	if err != nil {
		return Globals{}, err
	}
	defer b.close()
	return b.globals(), nil
}
