package cdp

import (
	"bytes"
	"encoding/json"
	"math"
	"net/url"
	"slices"
)

// MaxText bounds one Input.insertText or one string argument.
const MaxText = 64 << 10

type check func(json.RawMessage) bool

type field struct {
	need bool
	ok   check
}

type shape map[string]field

// methods is the whole list: a method is allowed only with exactly these
// keys, the needed ones present and every value passing its check. Nothing
// reads cookies or storage, switches tabs, navigates, evaluates text, or
// touches downloads. Focus guard alone closes its authorized tab and
// reopens an address it retained from that close.
var methods = map[string]shape{
	// The browser session.
	"Target.setDiscoverTargets": {"discover": {true, isBool}},
	"Target.getTargets":         {},
	"Target.attachToTarget":     {"targetId": {true, isString}, "flatten": {true, isTrue}},
	"Target.detachFromTarget":   {"sessionId": {true, isString}},
	"Target.closeTarget":        {"targetId": {true, isString}},
	"Target.createTarget":       {"url": {true, isWebURL}},

	// The one authorized tab's session.
	"Page.enable":       {},
	"Page.getFrameTree": {},
	"Page.captureScreenshot": {
		"format":                {false, oneOf("png", "jpeg")},
		"quality":               {false, intIn(0, 100)},
		"captureBeyondViewport": {false, isFalse},
	},
	"Page.addScriptToEvaluateOnNewDocument": {"source": {true, equals(WatchScript)}, "worldName": {true, equals(World)}},
	"Page.createIsolatedWorld":              {"frameId": {true, isString}, "worldName": {true, equals(World)}},
	"Accessibility.enable":                  {},
	"Accessibility.getRootAXNode":           {"frameId": {true, isString}},
	"Accessibility.getChildAXNodes":         {"id": {true, isString}, "frameId": {true, isString}},
	"DOM.enable":                            {},
	"DOM.describeNode":                      {"backendNodeId": {true, intIn(1, math.MaxInt32)}, "depth": {true, intIn(0, 0)}, "pierce": {true, isFalse}},
	"DOM.resolveNode":                       {"backendNodeId": {true, intIn(1, math.MaxInt32)}, "executionContextId": {true, intIn(1, math.MaxInt32)}},
	"DOM.focus":                             {"backendNodeId": {true, intIn(1, math.MaxInt32)}},
	"DOM.scrollIntoViewIfNeeded":            {"backendNodeId": {true, intIn(1, math.MaxInt32)}},
	"DOM.getBoxModel":                       {"backendNodeId": {true, intIn(1, math.MaxInt32)}},
	"Input.dispatchMouseEvent": {
		"type":       {true, oneOf("mouseMoved", "mousePressed", "mouseReleased")},
		"x":          {true, coordinate},
		"y":          {true, coordinate},
		"button":     {false, oneOf("left", "none")},
		"clickCount": {false, intIn(0, 1)},
	},
	"Input.insertText":   {"text": {true, textOf(MaxText)}},
	"Runtime.enable":     {},
	"Runtime.addBinding": {"name": {true, equals(Binding)}, "executionContextName": {true, equals(World)}},
	"Runtime.callFunctionOn": {
		"functionDeclaration": {true, isFunction},
		"objectId":            {false, isString},
		"executionContextId":  {false, intIn(1, math.MaxInt32)},
		"arguments":           {false, isArgs},
		"returnByValue":       {true, isTrue},
	},
	"Runtime.releaseObject": {"objectId": {true, isString}},
}

// Methods lists every allowed method, sorted.
func Methods() []string {
	out := make([]string, 0, len(methods))
	for m := range methods {
		out = append(out, m)
	}
	slices.Sort(out)
	return out
}

// Allowed says whether method may be sent with params (nil is no
// parameters): an exact method and an exact shape.
func Allowed(method string, params any) bool {
	s, ok := methods[method]
	if !ok {
		return false
	}
	m := map[string]json.RawMessage{}
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return false
		}
		if !bytes.Equal(b, []byte("null")) {
			if b[0] != '{' || json.Unmarshal(b, &m) != nil {
				return false
			}
		}
	}
	for k, v := range m {
		f, ok := s[k]
		if !ok || !f.ok(v) {
			return false
		}
	}
	for k, f := range s {
		if _, ok := m[k]; f.need && !ok {
			return false
		}
	}
	if method == "Runtime.callFunctionOn" {
		_, obj := m["objectId"]
		_, ctx := m["executionContextId"]
		if obj == ctx {
			return false
		}
	}
	return true
}

func str(v json.RawMessage) (string, bool) {
	var s string
	if len(v) == 0 || v[0] != '"' || json.Unmarshal(v, &s) != nil {
		return "", false
	}
	return s, true
}

func isString(v json.RawMessage) bool {
	s, ok := str(v)
	return ok && s != "" && len(s) <= 256
}

func isWebURL(v json.RawMessage) bool {
	s, ok := str(v)
	if !ok || len(s) > 8192 {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Hostname() != "" && u.User == nil
}

func isBool(v json.RawMessage) bool { return isTrue(v) || isFalse(v) }

func isTrue(v json.RawMessage) bool  { return bytes.Equal(v, []byte("true")) }
func isFalse(v json.RawMessage) bool { return bytes.Equal(v, []byte("false")) }

func equals(want string) check {
	return func(v json.RawMessage) bool {
		s, ok := str(v)
		return ok && s == want
	}
}

func oneOf(want ...string) check {
	return func(v json.RawMessage) bool {
		s, ok := str(v)
		return ok && slices.Contains(want, s)
	}
}

func textOf(max int) check {
	return func(v json.RawMessage) bool {
		s, ok := str(v)
		return ok && len(s) <= max
	}
}

func number(v json.RawMessage) (float64, bool) {
	var f float64
	if json.Unmarshal(v, &f) != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return f, true
}

func intIn(lo, hi float64) check {
	return func(v json.RawMessage) bool {
		f, ok := number(v)
		return ok && f == math.Trunc(f) && f >= lo && f <= hi
	}
}

func coordinate(v json.RawMessage) bool {
	f, ok := number(v)
	return ok && f >= 0 && f <= 1<<16
}

func isFunction(v json.RawMessage) bool {
	s, ok := str(v)
	if !ok {
		return false
	}
	for _, src := range functions {
		if s == src {
			return true
		}
	}
	return false
}

func isArgs(v json.RawMessage) bool {
	var args []map[string]json.RawMessage
	if json.Unmarshal(v, &args) != nil || len(args) > 3 {
		return false
	}
	for _, a := range args {
		if len(a) != 1 {
			return false
		}
		val, ok := a["value"]
		if !ok {
			return false
		}
		if s, ok := str(val); ok {
			if len(s) > MaxText {
				return false
			}
			continue
		}
		if _, ok := number(val); ok || isBool(val) {
			continue
		}
		return false
	}
	return true
}
