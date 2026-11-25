// Package gio shows a render.Character in a Gio layout, for in-app previews:
// it paints the same raster the native surface commits.
package gio

import (
	"image"
	"maps"
	"math"
	"sync"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"ryoku-rashin/internal/oni/motion"
	"ryoku-rashin/internal/oni/render"
)

// last is the one raster kept between frames, so a preview that has not
// changed repaints without drawing again and its texture is reused.
var last struct {
	sync.Mutex
	c     *render.Character
	pose  motion.Pose
	look  render.Look
	side  int
	nrgba *image.NRGBA
	op    paint.ImageOp
}

// Layout draws c in a size-by-size pixel square fitted to the character's
// frame and returns that square.
func Layout(gtx layout.Context, c *render.Character, pose motion.Pose, look render.Look, size float32) layout.Dimensions {
	side := int(math.Ceil(float64(size)))
	dims := layout.Dimensions{Size: image.Pt(side, side)}
	if c == nil || side <= 0 {
		return dims
	}
	frame := c.Extent(1).X
	if frame <= 0 {
		return dims
	}
	last.Lock()
	if last.c != c || last.pose != pose || last.side != side || !sameLook(last.look, look) {
		if last.nrgba == nil || last.nrgba.Bounds().Dx() != side {
			last.nrgba = image.NewNRGBA(image.Rect(0, 0, side, side))
		}
		c.Draw(last.nrgba, pose, look, float64(side)/float64(frame), 1)
		last.c, last.pose, last.side = c, pose, side
		last.look = look
		last.look.Accent = maps.Clone(look.Accent)
		last.op = paint.NewImageOp(premultiplied(last.nrgba))
		last.op.Filter = paint.FilterNearest
	}
	op := last.op
	last.Unlock()
	defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
	op.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return dims
}

func sameLook(a, b render.Look) bool {
	return a.Ink == b.Ink && a.White == b.White && a.Body == b.Body && a.Dark == b.Dark && maps.Equal(a.Accent, b.Accent)
}

// premultiplied is src as the premultiplied RGBA Gio uploads unchanged.
func premultiplied(src *image.NRGBA) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	for i := 0; i < len(src.Pix); i += 4 {
		a := uint32(src.Pix[i+3])
		dst.Pix[i] = uint8((uint32(src.Pix[i])*a + 127) / 255)
		dst.Pix[i+1] = uint8((uint32(src.Pix[i+1])*a + 127) / 255)
		dst.Pix[i+2] = uint8((uint32(src.Pix[i+2])*a + 127) / 255)
		dst.Pix[i+3] = uint8(a)
	}
	return dst
}
