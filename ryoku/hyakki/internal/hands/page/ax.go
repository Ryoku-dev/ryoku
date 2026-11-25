package page

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"ryoku-rashin/internal/hands/cdp"
	"ryoku-rashin/internal/oni/hands/consent"
)

type axValue struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

func (v *axValue) text() string {
	if v == nil || len(v.Value) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(v.Value, &s) == nil {
		return s
	}
	return string(v.Value)
}

type axNode struct {
	NodeID     string   `json:"nodeId"`
	ParentID   string   `json:"parentId"`
	ChildIDs   []string `json:"childIds"`
	Ignored    bool     `json:"ignored"`
	Role       *axValue `json:"role"`
	Name       *axValue `json:"name"`
	Value      *axValue `json:"value"`
	Properties []struct {
		Name  string  `json:"name"`
		Value axValue `json:"value"`
	} `json:"properties"`
	BackendDOMNodeID int    `json:"backendDOMNodeId"`
	FrameID          string `json:"frameId"`
}

func (n axNode) property(name string) bool {
	for _, p := range n.Properties {
		if p.Name == name {
			return p.Value.text() == "true"
		}
	}
	return false
}

// controlRoles are the roles offered as controls.
var controlRoles = map[string]bool{
	"button": true, "link": true, "tab": true, "menuitem": true, "menuitemcheckbox": true, "menuitemradio": true,
	"checkbox": true, "radio": true, "switch": true, "option": true, "treeitem": true, "textbox": true,
	"searchbox": true, "combobox": true, "spinbutton": true, "listbox": true, "disclosuretriangle": true,
}

// hintVerb is the step a control's role suggests, for the first sorting.
func hintVerb(role string) string {
	switch role {
	case "textbox", "searchbox", "spinbutton":
		return consent.VerbFill
	case "combobox", "listbox":
		return consent.VerbSelect
	}
	return consent.VerbClick
}

type reading struct {
	snap      Snapshot
	ids       map[string]control
	text      strings.Builder
	textRunes int
	frame     string
	lim       Limits
}

// static text already holds the string, its inline boxes only repeat it
func (r *reading) add(n axNode, role string) bool {
	if role == "iframe" {
		r.snap.Frames++
		return false
	}
	if n.FrameID != "" && r.frame != "" && n.FrameID != r.frame {
		return false
	}
	if role == "inlinetextbox" {
		return false
	}
	if n.Ignored {
		return role != "statictext"
	}
	if role == "statictext" {
		if r.textRunes >= r.lim.MaxTextRunes {
			r.snap.Truncated = true
			return false
		}
		room := r.lim.MaxTextRunes - r.textRunes
		if r.text.Len() > 0 {
			room--
		}
		piece, cut := clip(n.Name.text(), room)
		r.appendText(piece, utf8.RuneCountInString(piece), cut)
		return false
	}
	if !controlRoles[role] || n.BackendDOMNodeID <= 0 || n.property("disabled") || n.property("hidden") {
		return true
	}
	if len(r.snap.Controls) >= r.lim.MaxControls {
		r.snap.Truncated = true
		return true
	}
	name, cutName := clip(n.Name.text(), r.lim.MaxNameRunes)
	value, cutValue := clip(n.Value.text(), r.lim.MaxNameRunes)
	if cutName || cutValue {
		r.snap.Truncated = true
	}
	c := Control{ID: "c" + strconv.Itoa(len(r.snap.Controls)+1), Role: role, Name: name, Value: value}
	c.Class, c.Reason = consent.Classify(hintVerb(role), consent.Subject{Role: role, Name: name}, consent.ClaimNormal)
	r.snap.Controls = append(r.snap.Controls, c)
	r.ids[c.ID] = control{backend: n.BackendDOMNodeID, c: c}
	return true
}

func (r *reading) full() bool {
	return len(r.snap.Controls) >= r.lim.MaxControls && r.textRunes >= r.lim.MaxTextRunes
}

func (r *reading) appendText(piece string, runes int, cut bool) {
	if cut {
		r.snap.Truncated = true
	}
	if piece == "" {
		return
	}
	if r.text.Len() > 0 {
		r.text.WriteByte(' ')
		r.textRunes++
	}
	r.text.WriteString(piece)
	r.textRunes += runes
}

// large text nodes spend the read deadline serializing inline boxes
func (s *Session) textLeaf(ctx context.Context, sess string, n axNode, role string, r *reading) (bool, error) {
	if n.Ignored || n.BackendDOMNodeID <= 0 || len(n.ChildIDs) != 1 || controlRoles[role] {
		return false, nil
	}
	var dom struct {
		Node struct {
			ShadowRoots []json.RawMessage `json:"shadowRoots"`
		} `json:"node"`
	}
	if err := s.conn.Call(ctx, sess, "DOM.describeNode", map[string]any{"backendNodeId": n.BackendDOMNodeID, "depth": 0, "pierce": false}, &dom); err != nil {
		return false, err
	}
	// a closed shadow root can replace the host's light text
	if len(dom.Node.ShadowRoots) > 0 {
		return false, nil
	}
	world, err := s.ensureWorld(ctx)
	if err != nil {
		return false, err
	}
	object, release, err := s.resolve(ctx, sess, n.BackendDOMNodeID, world)
	if err != nil {
		return false, err
	}
	defer release()
	room := r.lim.MaxTextRunes - r.textRunes
	if r.text.Len() > 0 {
		room--
	}
	var leaf struct {
		OK        bool   `json:"ok"`
		Text      string `json:"text"`
		Runes     int    `json:"runes"`
		Truncated bool   `json:"truncated"`
	}
	if err := s.call(ctx, sess, cdp.FnTextLeaf, object, 0, &leaf, max(0, room)); err != nil {
		return false, err
	}
	if leaf.OK {
		r.appendText(leaf.Text, leaf.Runes, leaf.Truncated)
	}
	return leaf.OK, nil
}

// clean makes page text safe to show and bounded: control and direction
// characters become spaces, runs of space fold to one, and at most max
// runes remain.
func clean(s string, max int) string {
	c, _ := clip(s, max)
	return c
}

// clip is clean, also saying whether anything was cut.
func clip(s string, max int) (string, bool) {
	var b strings.Builder
	runes, space := 0, false
	for _, r := range s {
		if unicode.IsControl(r) || unicode.IsSpace(r) || unicode.Is(unicode.Bidi_Control, r) || r == utf8.RuneError {
			space = b.Len() > 0
			continue
		}
		need := 1
		if space {
			need = 2
		}
		if runes+need > max {
			return b.String(), true
		}
		if space {
			b.WriteByte(' ')
			runes++
			space = false
		}
		b.WriteRune(r)
		runes++
	}
	return b.String(), false
}
