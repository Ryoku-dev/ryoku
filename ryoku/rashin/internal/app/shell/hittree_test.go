package shell

import (
	"fmt"
	"image"
	"reflect"

	"gioui.org/f32"
	"gioui.org/io/input"
)

// hitTree is a copy of the router's pointer hit tree for one frame.
//
// Gio keeps the tree to itself. Its SemanticAt stops at the first area that
// listens for presses, also when that area lets them pass on, as the scroll
// and hover trackers over every list do, so it cannot say what a press
// reaches. This copy is read with reflect and walked the way the router walks
// it. A Gio update that renames one of these fields fails here by name.
type hitTree struct {
	nodes []hitNode
	areas []hitArea
}

type hitNode struct {
	next, area    int
	handler, pass bool
}

type hitArea struct {
	parent  int
	inverse [6]float32
	rect    image.Rectangle
	ellipse bool
}

func (s *stage) hitTree() hitTree {
	s.t.Helper()
	field := func(value reflect.Value, name string) reflect.Value {
		found := value.FieldByName(name)
		if !found.IsValid() {
			s.t.Fatalf("Gio's router has no %s in %s any more; hittree_test.go reads it by name", name, value.Type())
		}
		return found
	}
	queue := field(field(reflect.ValueOf(&s.router).Elem(), "pointer"), "queue")
	nodes, areas := field(queue, "hitTree"), field(queue, "areas")
	tree := hitTree{nodes: make([]hitNode, nodes.Len()), areas: make([]hitArea, areas.Len())}
	for index := range tree.nodes {
		node := nodes.Index(index)
		tree.nodes[index] = hitNode{
			next: int(field(node, "next").Int()), area: int(field(node, "area").Int()),
			handler: !field(node, "tag").IsNil(), pass: field(node, "pass").Bool(),
		}
	}
	for index := range tree.areas {
		area := areas.Index(index)
		shape, transform := field(area, "area"), field(area, "trans")
		part := func(name string) float32 { return float32(field(transform, name).Float()) }
		// Gio stores the matrix with the identity taken off, so its zero value is no transform
		sx, hx, ox, hy, sy, oy := part("a")+1, part("b"), part("c"), part("d"), part("e")+1, part("f")
		determinant := sx*sy - hx*hy
		rect := field(shape, "rect")
		corner := func(name string) image.Point {
			point := field(rect, name)
			return image.Pt(int(field(point, "X").Int()), int(field(point, "Y").Int()))
		}
		tree.areas[index] = hitArea{
			parent: int(field(area, "parent").Int()),
			inverse: [6]float32{
				sy / determinant, -hx / determinant, (hx*oy - sy*ox) / determinant,
				-hy / determinant, sx / determinant, (hy*ox - sx*oy) / determinant,
			},
			rect:    image.Rectangle{Min: corner("Min"), Max: corner("Max")},
			ellipse: field(shape, "kind").Uint() != 0,
		}
	}
	return tree
}

// inside reports whether a point is in an area and in everything that clips it
func (tree hitTree) inside(area int, at f32.Point) bool {
	for ; area != -1; area = tree.areas[area].parent {
		a := &tree.areas[area]
		x := a.inverse[0]*at.X + a.inverse[1]*at.Y + a.inverse[2] - float32(a.rect.Min.X)
		y := a.inverse[3]*at.X + a.inverse[4]*at.Y + a.inverse[5] - float32(a.rect.Min.Y)
		width, height := float32(a.rect.Dx()), float32(a.rect.Dy())
		if x < 0 || x >= width || y < 0 || y >= height {
			return false
		}
		if a.ellipse {
			rx, ry := width/2, height/2
			dx, dy := (x-rx)/rx, (y-ry)/ry
			if dx*dx+dy*dy > 1 {
				return false
			}
		}
	}
	return true
}

// pressed lists the areas whose handlers a press at one point is delivered to, front first
func (tree hitTree) pressed(at f32.Point) []int {
	var areas []int
	pass := true
	for index := len(tree.nodes) - 1; index >= 0; {
		node := tree.nodes[index]
		if !tree.inside(node.area, at) {
			index--
			continue
		}
		pass = pass && node.pass
		if pass {
			index--
		} else {
			index = node.next
		}
		if node.handler {
			areas = append(areas, node.area)
		}
	}
	return areas
}

func (tree hitTree) within(area, ancestor int) bool {
	for ; area != -1; area = tree.areas[area].parent {
		if area == ancestor {
			return true
		}
	}
	return false
}

// reaches reports whether a press somewhere on a control gets to it, and
// whether any of it is in sight at all: a scrolled list hides rows it still lays out.
func (tree hitTree) reaches(area int, bounds, window image.Rectangle) (reached, visible bool) {
	bounds = bounds.Intersect(window)
	const grid = 7
	for row := 1; row <= grid; row++ {
		for column := 1; column <= grid; column++ {
			at := f32.Pt(
				float32(bounds.Min.X)+float32(bounds.Dx()*column)/(grid+1),
				float32(bounds.Min.Y)+float32(bounds.Dy()*row)/(grid+1),
			)
			if !tree.inside(area, at) {
				continue
			}
			visible = true
			for _, handler := range tree.pressed(at) {
				if tree.within(handler, area) {
					return true, true
				}
			}
		}
	}
	return false, visible
}

// unreachable lists the controls that are drawn where the pointer cannot get
// to them: something in front takes the press at every point of them.
func (s *stage) unreachable() []string {
	s.t.Helper()
	nodes := s.nodes()
	tree := s.hitTree()
	window := image.Rectangle{Max: s.size}
	var dead []string
	for index := 1; index < len(nodes); index++ {
		node := nodes[index]
		// Gio marks every area whose handler listens for presses, so this is each of them, named or not
		if node.Desc.Gestures&input.ClickGesture == 0 || node.Desc.Disabled {
			continue
		}
		area := int(reflect.ValueOf(node).FieldByName("areaIdx").Int())
		if reached, visible := tree.reaches(area, node.Desc.Bounds, window); visible && !reached {
			dead = append(dead, fmt.Sprintf("%q at %v", nodeName(node), node.Desc.Bounds))
		}
	}
	return dead
}
