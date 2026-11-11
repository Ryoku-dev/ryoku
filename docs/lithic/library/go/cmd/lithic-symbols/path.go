package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type point struct{ x, y float64 }

// segment is one absolute drawing step: 'M' and 'L' use p[0], 'Q' p[0] and p[1], 'C' all three, 'Z' none.
type segment struct {
	op byte
	p  [3]point
}

// parsePath resolves SVG path data into absolute moves, lines, quadratic and cubic curves.
// H and V become lines, S and T get their reflected control point, and arcs become cubics,
// so the output never depends on how a decoder treats those forms.
func parsePath(data string) ([]segment, error) {
	scanner := &pathScanner{data: data}
	var out []segment
	var current, start, cubicControl, quadControl point
	var command byte
	closed, afterCubic, afterQuad := false, false, false
	for {
		scanner.skip()
		if scanner.atEnd() {
			break
		}
		if next := scanner.data[scanner.at]; strings.IndexByte("MmZzLlHhVvCcSsQqTtAa", next) >= 0 {
			command = next
			scanner.at++
		} else if command == 0 || command == 'Z' || command == 'z' {
			return nil, fmt.Errorf("path data: number without a command at offset %d", scanner.at)
		}
		upper := command &^ 0x20
		if len(out) == 0 && upper != 'M' {
			return nil, errors.New("path data must start with a move")
		}
		base := point{}
		if command != upper {
			base = current
		}
		if closed && upper != 'M' && upper != 'Z' {
			out = append(out, segment{op: 'M', p: [3]point{start}})
			closed = false
		}
		wasCubic, wasQuad := afterCubic, afterQuad
		afterCubic, afterQuad = false, false
		switch upper {
		case 'M':
			to, err := scanner.pair(base)
			if err != nil {
				return nil, err
			}
			out = append(out, segment{op: 'M', p: [3]point{to}})
			current, start, closed = to, to, false
			// coordinates repeated after a move are lines
			command = 'L' | command&0x20
		case 'Z':
			out = append(out, segment{op: 'Z'})
			current, closed = start, true
		case 'L':
			to, err := scanner.pair(base)
			if err != nil {
				return nil, err
			}
			out = append(out, segment{op: 'L', p: [3]point{to}})
			current = to
		case 'H', 'V':
			value, err := scanner.number()
			if err != nil {
				return nil, err
			}
			to := current
			if upper == 'H' {
				to.x = base.x + value
			} else {
				to.y = base.y + value
			}
			out = append(out, segment{op: 'L', p: [3]point{to}})
			current = to
		case 'C', 'S':
			first := current
			if upper == 'S' && wasCubic {
				first = mirror(cubicControl, current)
			}
			points := 2
			if upper == 'C' {
				points = 3
			}
			read, err := scanner.pairs(base, points)
			if err != nil {
				return nil, err
			}
			if upper == 'C' {
				first, read = read[0], read[1:]
			}
			out = append(out, segment{op: 'C', p: [3]point{first, read[0], read[1]}})
			current, cubicControl, afterCubic = read[1], read[0], true
		case 'Q':
			read, err := scanner.pairs(base, 2)
			if err != nil {
				return nil, err
			}
			out = append(out, segment{op: 'Q', p: [3]point{read[0], read[1]}})
			current, quadControl, afterQuad = read[1], read[0], true
		case 'T':
			control := current
			if wasQuad {
				control = mirror(quadControl, current)
			}
			to, err := scanner.pair(base)
			if err != nil {
				return nil, err
			}
			out = append(out, segment{op: 'Q', p: [3]point{control, to}})
			current, quadControl, afterQuad = to, control, true
		case 'A':
			radii, err := scanner.pair(point{})
			if err != nil {
				return nil, err
			}
			rotation, err := scanner.number()
			if err != nil {
				return nil, err
			}
			large, err := scanner.flag()
			if err != nil {
				return nil, err
			}
			sweep, err := scanner.flag()
			if err != nil {
				return nil, err
			}
			to, err := scanner.pair(base)
			if err != nil {
				return nil, err
			}
			out = append(out, arc(current, radii.x, radii.y, rotation, large, sweep, to)...)
			current = to
		}
	}
	return out, nil
}

// mirror reflects control through at, the smooth continuation S and T draw.
func mirror(control, at point) point {
	return point{2*at.x - control.x, 2*at.y - control.y}
}

type pathScanner struct {
	data string
	at   int
}

func (s *pathScanner) atEnd() bool { return s.at >= len(s.data) }

// skip passes whitespace and commas, which separate path data tokens.
func (s *pathScanner) skip() {
	for !s.atEnd() && strings.IndexByte(" \t\n\r\f,", s.data[s.at]) >= 0 {
		s.at++
	}
}

func (s *pathScanner) digits() int {
	count := 0
	for !s.atEnd() && s.data[s.at] >= '0' && s.data[s.at] <= '9' {
		s.at++
		count++
	}
	return count
}

// number reads the longest SVG number, so "120-580" is two numbers and "0.5.5" is 0.5 then .5.
func (s *pathScanner) number() (float64, error) {
	s.skip()
	begin := s.at
	if !s.atEnd() && (s.data[s.at] == '+' || s.data[s.at] == '-') {
		s.at++
	}
	digits := s.digits()
	if !s.atEnd() && s.data[s.at] == '.' {
		s.at++
		digits += s.digits()
	}
	if digits == 0 {
		return 0, fmt.Errorf("path data: want a number at offset %d", begin)
	}
	if !s.atEnd() && (s.data[s.at] == 'e' || s.data[s.at] == 'E') {
		mark := s.at
		s.at++
		if !s.atEnd() && (s.data[s.at] == '+' || s.data[s.at] == '-') {
			s.at++
		}
		if s.digits() == 0 {
			s.at = mark
		}
	}
	value, err := strconv.ParseFloat(s.data[begin:s.at], 64)
	if err != nil {
		return 0, fmt.Errorf("path data: %w", err)
	}
	return value, nil
}

func (s *pathScanner) pair(base point) (point, error) {
	x, err := s.number()
	if err != nil {
		return point{}, err
	}
	y, err := s.number()
	if err != nil {
		return point{}, err
	}
	return point{base.x + x, base.y + y}, nil
}

// pairs reads count points, all relative to the same base: a relative curve's control points
// and end point share the segment's start point.
func (s *pathScanner) pairs(base point, count int) ([]point, error) {
	out := make([]point, count)
	for index := range out {
		read, err := s.pair(base)
		if err != nil {
			return nil, err
		}
		out[index] = read
	}
	return out, nil
}

// flag reads one arc flag, which may sit directly against the next token ("a5 5 0 0110 10").
func (s *pathScanner) flag() (bool, error) {
	s.skip()
	if !s.atEnd() {
		switch s.data[s.at] {
		case '0':
			s.at++
			return false, nil
		case '1':
			s.at++
			return true, nil
		}
	}
	return false, fmt.Errorf("path data: want an arc flag at offset %d", s.at)
}
