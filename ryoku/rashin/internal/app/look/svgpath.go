package look

import (
	"fmt"
	"image"
	"strconv"
	"strings"
	"unicode"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
)

type svgPointOp struct {
	kind     byte
	point    f32.Point
	control1 f32.Point
	control2 f32.Point
}

func buildSVGPath(
	ops *op.Ops,
	data string,
	source f32.Point,
	size image.Point,
) (clip.PathSpec, error) {
	operations, err := parseSVGPath(data)
	if err != nil {
		return clip.PathSpec{}, err
	}
	scaleX, scaleY := float32(size.X)/source.X, float32(size.Y)/source.Y
	var path clip.Path
	path.Begin(ops)
	for _, operation := range operations {
		point := scaleSVGPoint(operation.point, scaleX, scaleY)
		switch operation.kind {
		case 'M':
			path.MoveTo(point)
		case 'L':
			path.LineTo(point)
		case 'C':
			path.CubeTo(
				scaleSVGPoint(operation.control1, scaleX, scaleY),
				scaleSVGPoint(operation.control2, scaleX, scaleY),
				point,
			)
		case 'Z':
			path.Close()
		}
	}
	return path.End(), nil
}

func scaleSVGPoint(point f32.Point, x, y float32) f32.Point {
	return f32.Pt(point.X*x, point.Y*y)
}

func parseSVGPath(data string) ([]svgPointOp, error) {
	scanner := svgScanner{data: data}
	var operations []svgPointOp
	var command byte
	var current, start f32.Point
	for {
		scanner.space()
		if scanner.atEnd() {
			break
		}
		if isSVGCommand(scanner.peek()) {
			command = scanner.take()
		} else if command == 0 {
			return nil, fmt.Errorf("svg path starts without a command")
		}
		relative := command >= 'a' && command <= 'z'
		switch command {
		case 'M', 'm', 'L', 'l':
			x, ok := scanner.number()
			if !ok {
				return nil, fmt.Errorf("svg %c needs x", command)
			}
			y, ok := scanner.number()
			if !ok {
				return nil, fmt.Errorf("svg %c needs y", command)
			}
			point := f32.Pt(float32(x), float32(y))
			if relative {
				point = point.Add(current)
			}
			kind := byte('L')
			if command == 'M' || command == 'm' {
				kind, start = 'M', point
				if command == 'M' {
					command = 'L'
				} else {
					command = 'l'
				}
			}
			operations = append(operations, svgPointOp{kind: kind, point: point})
			current = point
		case 'H', 'h':
			x, ok := scanner.number()
			if !ok {
				return nil, fmt.Errorf("svg %c needs x", command)
			}
			if relative {
				x += float64(current.X)
			}
			current.X = float32(x)
			operations = append(operations, svgPointOp{kind: 'L', point: current})
		case 'V', 'v':
			y, ok := scanner.number()
			if !ok {
				return nil, fmt.Errorf("svg %c needs y", command)
			}
			if relative {
				y += float64(current.Y)
			}
			current.Y = float32(y)
			operations = append(operations, svgPointOp{kind: 'L', point: current})
		case 'C', 'c':
			values := [6]float64{}
			for index := range values {
				value, ok := scanner.number()
				if !ok {
					return nil, fmt.Errorf("svg %c needs six values", command)
				}
				values[index] = value
			}
			control1 := f32.Pt(float32(values[0]), float32(values[1]))
			control2 := f32.Pt(float32(values[2]), float32(values[3]))
			point := f32.Pt(float32(values[4]), float32(values[5]))
			if relative {
				control1 = control1.Add(current)
				control2 = control2.Add(current)
				point = point.Add(current)
			}
			operations = append(operations, svgPointOp{
				kind: 'C', point: point, control1: control1, control2: control2,
			})
			current = point
		case 'Z', 'z':
			operations = append(operations, svgPointOp{kind: 'Z'})
			current, command = start, 0
		default:
			return nil, fmt.Errorf("unsupported svg command %q", command)
		}
	}
	return operations, nil
}

type svgScanner struct {
	data string
	at   int
}

func (scanner *svgScanner) atEnd() bool { return scanner.at >= len(scanner.data) }
func (scanner *svgScanner) peek() byte  { return scanner.data[scanner.at] }
func (scanner *svgScanner) take() byte {
	value := scanner.data[scanner.at]
	scanner.at++
	return value
}

func (scanner *svgScanner) space() {
	for !scanner.atEnd() {
		r := rune(scanner.peek())
		if !unicode.IsSpace(r) && r != ',' {
			return
		}
		scanner.at++
	}
}

func (scanner *svgScanner) number() (float64, bool) {
	scanner.space()
	if scanner.atEnd() {
		return 0, false
	}
	start := scanner.at
	if scanner.peek() == '+' || scanner.peek() == '-' {
		scanner.at++
	}
	for !scanner.atEnd() {
		value := scanner.peek()
		if (value >= '0' && value <= '9') || value == '.' || value == 'e' || value == 'E' {
			scanner.at++
			if (value == 'e' || value == 'E') && !scanner.atEnd() && (scanner.peek() == '+' || scanner.peek() == '-') {
				scanner.at++
			}
			continue
		}
		break
	}
	if scanner.at == start {
		return 0, false
	}
	value, err := strconv.ParseFloat(scanner.data[start:scanner.at], 64)
	return value, err == nil
}

func isSVGCommand(value byte) bool { return strings.ContainsRune("MmLlHhVvCcZz", rune(value)) }
