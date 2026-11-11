package main

import "math"

// arc converts one SVG elliptical arc into cubic segments of at most a quarter turn each,
// following the endpoint to centre conversion of the SVG implementation notes (B.2.4 and B.2.5).
func arc(from point, rx, ry, degrees float64, large, sweep bool, to point) []segment {
	if from == to {
		return nil
	}
	rx, ry = math.Abs(rx), math.Abs(ry)
	if rx == 0 || ry == 0 {
		return []segment{{op: 'L', p: [3]point{to}}}
	}
	sin, cos := math.Sincos(degrees * math.Pi / 180)
	halfX, halfY := (from.x-to.x)/2, (from.y-to.y)/2
	x1 := cos*halfX + sin*halfY
	y1 := -sin*halfX + cos*halfY
	// radii too small to reach the end point grow until they just do
	if lambda := x1*x1/(rx*rx) + y1*y1/(ry*ry); lambda > 1 {
		grow := math.Sqrt(lambda)
		rx, ry = rx*grow, ry*grow
	}
	numerator := rx*rx*ry*ry - rx*rx*y1*y1 - ry*ry*x1*x1
	factor := 0.0
	if numerator > 0 {
		factor = math.Sqrt(numerator / (rx*rx*y1*y1 + ry*ry*x1*x1))
	}
	if large == sweep {
		factor = -factor
	}
	centreX1, centreY1 := factor*rx*y1/ry, -factor*ry*x1/rx
	centre := point{
		cos*centreX1 - sin*centreY1 + (from.x+to.x)/2,
		sin*centreX1 + cos*centreY1 + (from.y+to.y)/2,
	}
	startX, startY := (x1-centreX1)/rx, (y1-centreY1)/ry
	endX, endY := (-x1-centreX1)/rx, (-y1-centreY1)/ry
	start := math.Atan2(startY, startX)
	delta := math.Atan2(startX*endY-startY*endX, startX*endX+startY*endY)
	if !sweep && delta > 0 {
		delta -= 2 * math.Pi
	} else if sweep && delta < 0 {
		delta += 2 * math.Pi
	}
	pieces := int(math.Ceil(math.Abs(delta)/(math.Pi/2) - 1e-9))
	if pieces < 1 {
		pieces = 1
	}
	step := delta / float64(pieces)
	// handle length that keeps a cubic closest to a circular arc of this step
	handle := 4.0 / 3.0 * math.Tan(step/4)
	onEllipse := func(unitX, unitY float64) point {
		return point{
			centre.x + rx*cos*unitX - ry*sin*unitY,
			centre.y + rx*sin*unitX + ry*cos*unitY,
		}
	}
	out := make([]segment, 0, pieces)
	for index := range pieces {
		sin0, cos0 := math.Sincos(start + step*float64(index))
		sin1, cos1 := math.Sincos(start + step*float64(index+1))
		end := onEllipse(cos1, sin1)
		if index == pieces-1 {
			// land exactly on the end point the path data named
			end = to
		}
		out = append(out, segment{op: 'C', p: [3]point{
			onEllipse(cos0-handle*sin0, sin0+handle*cos0),
			onEllipse(cos1+handle*sin1, sin1-handle*cos1),
			end,
		}})
	}
	return out
}
