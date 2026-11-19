package chat

import (
	"image"
	"strings"
	"testing"
)

func TestMindmapOutline(t *testing.T) {
	nodes, err := parseMindmap("Project\r\n  Read\r\n    Notes\r\n\r\n  Make\r\n    Page\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 5 || nodes[0].parent != -1 || nodes[2].parent != 1 || nodes[4].parent != 3 || nodes[2].depth != 2 {
		t.Fatalf("wrong outline: %+v", nodes)
	}
	if len(nodes[0].children) != 2 || nodes[0].children[1] != 3 {
		t.Fatalf("root branches: %v", nodes[0].children)
	}
	m := mindmapVisual{nodes: nodes, folded: map[int]bool{1: true}}
	_, visible, _ := m.positions()
	if visible[2] || !visible[1] || !visible[4] {
		t.Fatalf("folded visibility: %v", visible)
	}
	delete(m.folded, 1)
	_, visible, _ = m.positions()
	if !visible[2] {
		t.Fatal("unfold did not restore the child")
	}
}

func TestMindmapOutlineRejectsMalformedInput(t *testing.T) {
	for _, source := range []string{"", "  Root", "Root\nOther root", "Root\n Child", "Root\n    Jumped", "Root\n\tTabbed", "Root\n  Child\n      Jumped", "Root\n" + strings.Repeat("  ", 17) + "Deep", strings.Repeat("x", 65<<10)} {
		if _, err := parseMindmap(source); err == nil {
			t.Errorf("accepted malformed outline %q", source)
		}
	}
}

func TestMindmapFenceKeepsOriginalCode(t *testing.T) {
	blocks := parseMarkdown("```mindmap\nRoot\n  Child\n```\n\n```html\n<h1>Page</h1>\n```")
	if len(blocks) != 2 || blocks[0].kind != markdownCode || blocks[0].language != "mindmap" || blocks[0].text != "Root\n  Child" || blocks[1].language != "html" {
		t.Fatalf("fences changed: %+v", blocks)
	}
	var state viewState
	if state.feature("mindmap") || state.feature("artifacts") {
		t.Fatal("new transcript features are not off by default")
	}
}

func TestMindmapStartsFittedToVisibleBounds(t *testing.T) {
	nodes, err := parseMindmap("A small project\n  Understand\n    The question\n    The people\n  Make\n    A clear page\n    A small interaction\n  Check\n    Read it\n    Try it")
	if err != nil {
		t.Fatal(err)
	}
	m := mindmapVisual{nodes: nodes, folded: make(map[int]bool)}
	m.prepare()
	scale, height := mindmapFit(m.bounds, 640)
	if scale > 1 || scale < .7 || height < 240 || height > 480 {
		t.Fatalf("invalid fitted viewport: scale %v, height %d", scale, height)
	}
	if float32(m.bounds.Dx())*scale > 608 || float32(m.bounds.Dy())*scale > float32(height-32) {
		t.Fatalf("demo tree does not fit: bounds %v, scale %v, height %d", m.bounds, scale, height)
	}
	for index, center := range m.points {
		if m.visible[index] && !mindmapNodeBounds(m.nodes[index], center).In(m.bounds) {
			t.Fatalf("node %d is outside the tree bounds", index)
		}
	}
	scale, height = mindmapFit(image.Rect(0, 0, 164, 60), 640)
	if scale != 1 || height != 240 {
		t.Fatalf("small map was enlarged: scale %v, height %d", scale, height)
	}
}

func TestMindmapNodeRectanglesNeverIntersect(t *testing.T) {
	for _, source := range []string{
		"A small project\n  Understand\n    The question\n    The people\n  Make\n    A clear page\n    A small interaction\n  Check\n    Read it\n    Try it",
		"A longer root label that wraps onto two lines\n  A wide first branch with several words\n    One long leaf label with additional detail\n    Another long leaf label with additional detail\n    A third leaf with a broad label\n  A second wide branch with several words\n    Notes and observations from the project\n    Questions to answer before building\n  A third wide branch for checking the result\n    Read and compare all the results\n    Try every interaction on the page",
	} {
		nodes, err := parseMindmap(source)
		if err != nil {
			t.Fatal(err)
		}
		m := mindmapVisual{nodes: nodes, folded: make(map[int]bool)}
		check := func() {
			t.Helper()
			positions, visible, _ := m.positions()
			for index, node := range nodes {
				if !visible[index] {
					continue
				}
				box := mindmapNodeBounds(node, positions[index])
				for other := range index {
					if visible[other] && box.Inset(-10).Overlaps(mindmapNodeBounds(nodes[other], positions[other]).Inset(-10)) {
						t.Fatalf("nodes %q and %q overlap or lose their gap: %v, %v", node.text, nodes[other].text, box, mindmapNodeBounds(nodes[other], positions[other]))
					}
				}
			}
		}
		check()
		m.folded[1] = true
		check()
		delete(m.folded, 1)
		check()
	}
}
