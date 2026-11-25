package placement

import (
	"image"
	"math"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

// Flight plans the travel from one placement to another when a display
// changes: a cubic bowed toward centre, its duration from the spec speed
// within the spec clamp. The returned path takes progress t in [0,1] of the
// duration (clamped) and applies the spec ease itself. Equal endpoints need
// no flight and return a zero duration.
func Flight(from, to, centre image.Point, f spec.Flight) (time.Duration, func(t float64) image.Point) {
	ax, ay := float64(from.X), float64(from.Y)
	dx, dy := float64(to.X-from.X), float64(to.Y-from.Y)
	dist := math.Hypot(dx, dy)
	if dist == 0 {
		return 0, func(float64) image.Point { return to }
	}
	secs := min(max(dist/f.SpeedPxS, f.ClampS[0]), f.ClampS[1])

	nx, ny := -dy/dist, dx/dist
	side := (float64(centre.X)-(ax+dx/2))*nx + (float64(centre.Y)-(ay+dy/2))*ny
	bow := 0.0
	if side != 0 {
		bow = math.Copysign(min(f.BowMaxPx, dist*f.Bow), side)
	}
	c1x, c1y := ax+dx/3+nx*bow, ay+dy/3+ny*bow
	c2x, c2y := ax+2*dx/3+nx*bow, ay+2*dy/3+ny*bow
	bx, by := float64(to.X), float64(to.Y)

	path := func(t float64) image.Point {
		if !(t > 0) {
			return from
		}
		if t >= 1 {
			return to
		}
		u := f.Ease.At(t)
		v := 1 - u
		a, b, c, d := v*v*v, 3*v*v*u, 3*v*u*u, u*u*u
		return image.Pt(
			int(math.Round(a*ax+b*c1x+c*c2x+d*bx)),
			int(math.Round(a*ay+b*c1y+c*c2y+d*by)),
		)
	}
	return time.Duration(secs * float64(time.Second)), path
}
