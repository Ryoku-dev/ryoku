package search

import (
	"strings"

	xhtml "golang.org/x/net/html"
)

// walk visits n depth first; returning false skips the node's children.
func walk(n *xhtml.Node, fn func(*xhtml.Node) bool) {
	if !fn(n) {
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		walk(c, fn)
	}
}

func find(n *xhtml.Node, match func(*xhtml.Node) bool) *xhtml.Node {
	var hit *xhtml.Node
	walk(n, func(c *xhtml.Node) bool {
		if hit != nil {
			return false
		}
		if c != n && c.Type == xhtml.ElementNode && match(c) {
			hit = c
			return false
		}
		return true
	})
	return hit
}

func attr(n *xhtml.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *xhtml.Node, class string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}

func textOf(n *xhtml.Node) string {
	var b strings.Builder
	walk(n, func(c *xhtml.Node) bool {
		if c.Type == xhtml.TextNode {
			b.WriteString(c.Data)
		}
		return true
	})
	return b.String()
}
