// Package proto is the one shape a model reply may take while Hands works:
// exactly one JSON object naming one of the tools offered for the scope
// Hands works in, with arguments that point only at what Hands listed.
//
// The scopes are a browser tab, one live window the person picked, and the
// separate nested desktop Hyakki owns (reached as "nested" from the app
// stage and as "desktop" from the desktop stage). Each has a fixed tool set;
// a tool offered in another scope is as unknown as an invented one.
//
// Parse is strict on purpose. Prose, two objects, an array, a tool nobody
// offered, a plan, a permission outside the three words, or an id that is not
// in the list are all refused; the caller may ask once more with Corrective
// and then gives up. A parsed reply is still only a request: what Hands may
// do with it is decided by package consent, never by the reply's own
// permission or confirm fields. There is no drag and no paste in any scope.
package proto

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"ryoku-rashin/internal/oni/hands/apprules"
	"ryoku-rashin/internal/oni/hands/consent"
)

// Reply types.
const (
	TypeResponse = "response"  // words for the person and nothing done
	TypeToolCall = "tool_call" // one tool from the scope's set
)

// The scopes Hands works in.
const (
	ScopeBrowser = "browser" // one browser tab
	ScopeWindow  = "window"  // one live window the person picked
	ScopeNested  = "nested"  // the separate nested desktop, from the app stage
	ScopeDesktop = "desktop" // the separate nested desktop, from the desktop stage
)

// Scopes is every scope, in a fixed order.
var Scopes = []string{ScopeBrowser, ScopeWindow, ScopeNested, ScopeDesktop}

// KnownScope says whether s is one of Scopes.
func KnownScope(s string) bool { return slices.Contains(Scopes, s) }

// Nested says whether scope is the separate nested desktop.
func Nested(scope string) bool { return scope == ScopeNested || scope == ScopeDesktop }

// The tools a reply may name. The acting tools are consent's verbs.
const (
	ToolClick     = consent.VerbClick
	ToolFill      = consent.VerbFill
	ToolSelect    = consent.VerbSelect
	ToolScroll    = consent.VerbScroll
	ToolWait      = consent.VerbWait
	ToolType      = consent.VerbType
	ToolPress     = consent.VerbPress
	ToolOpenApp   = consent.VerbOpenApp
	ToolFindFiles = consent.VerbFindFiles
	ToolShowFile  = consent.VerbShowFile
	ToolDone      = "done"
	ToolAskUser   = "ask_user"
)

// Tools is the browser tab's fixed set, in the order the prompt lists them.
var Tools = []string{ToolClick, ToolFill, ToolSelect, ToolScroll, ToolWait, ToolDone, ToolAskUser}

// windowTools act in one live window: no app is opened and no file is
// found there, because those reach past the window.
var windowTools = []string{ToolClick, ToolType, ToolPress, ToolScroll, ToolWait, ToolDone, ToolAskUser}

// nestedTools act on the separate nested desktop.
var nestedTools = []string{ToolClick, ToolType, ToolPress, ToolScroll, ToolWait, ToolOpenApp, ToolFindFiles,
	ToolShowFile, ToolDone, ToolAskUser}

// ToolsFor is scope's fixed tool set, in prompt order; nil for a scope that
// is not one of Scopes.
func ToolsFor(scope string) []string {
	switch scope {
	case ScopeBrowser:
		return slices.Clone(Tools)
	case ScopeWindow:
		return slices.Clone(windowTools)
	case ScopeNested, ScopeDesktop:
		return slices.Clone(nestedTools)
	}
	return nil
}

// MaxAppNameRunes bounds the app name open_app may carry.
const MaxAppNameRunes = 80

// Scroll directions.
const (
	DirUp   = "up"
	DirDown = "down"
)

// Args are a tool's arguments. Which ones a tool takes is fixed: click{id},
// fill{id,text}, select{id,option}, scroll{dir, optional id}, wait{ms},
// type{text}, press{keys}, open_app{name}, find_files{query},
// show_file{index}, done{text}, ask_user{text}.
type Args struct {
	ID     string `json:"id,omitempty"`
	Text   string `json:"text,omitempty"`
	Option string `json:"option,omitempty"`
	Dir    string `json:"dir,omitempty"`
	Ms     int    `json:"ms,omitempty"`
	Keys   string `json:"keys,omitempty"`
	Name   string `json:"name,omitempty"`
	Query  string `json:"query,omitempty"`
	Index  int    `json:"index,omitempty"`
}

// Response is one parsed reply. Text is the reply's words for a response;
// a tool's words travel in Args.Text. Confirm and Permission are what the
// model said; they can raise caution and never lower it.
type Response struct {
	Type       string
	Tool       string
	Args       Args
	Text       string
	Confirm    bool
	Permission string
}

// Offer is what the prompt this reply answers offered: the scope (""
// is the browser tab), the ids of the controls or screen regions it listed,
// and how many found files it numbered for show_file.
type Offer struct {
	Scope    string
	Controls []string
	Files    int
}

// Why a reply is refused.
var (
	ErrJSON       = errors.New("not one JSON object")
	ErrMultiple   = errors.New("more than one action")
	ErrInvented   = errors.New("a tool, plan or type nobody offered")
	ErrPermission = errors.New("permission is not normal, sensitive or critical")
	ErrArgs       = errors.New("arguments do not fit the tool")
	ErrShape      = errors.New("fields missing, unknown or of the wrong kind")
)

// wire is the reply as written; pointers tell a missing field from an empty
// one.
type wire struct {
	Type       *string         `json:"type"`
	Tool       *string         `json:"tool"`
	Plan       *string         `json:"plan"`
	Args       json.RawMessage `json:"args"`
	Text       *string         `json:"text"`
	Confirm    *bool           `json:"confirm"`
	Permission *string         `json:"permission"`
}

type wireArgs struct {
	ID     *string `json:"id"`
	Text   *string `json:"text"`
	Option *string `json:"option"`
	Dir    *string `json:"dir"`
	Ms     *int    `json:"ms"`
	Keys   *string `json:"keys"`
	Name   *string `json:"name"`
	Query  *string `json:"query"`
	Index  *int    `json:"index"`
}

// Parse reads raw as one reply to a prompt that offered o: a tool must be
// one of the scope's, and may point only at what o lists.
func Parse(raw string, o Offer) (Response, error) {
	if o.Scope == "" {
		o.Scope = ScopeBrowser
	}
	tools := ToolsFor(o.Scope)
	if tools == nil {
		return Response{}, fmt.Errorf("%w: scope %q", ErrInvented, o.Scope)
	}
	body, err := one(raw)
	if err != nil {
		return Response{}, err
	}
	var w wire
	if err := strict(body, &w); err != nil {
		return Response{}, err
	}
	if w.Type == nil {
		return Response{}, fmt.Errorf("%w: no type", ErrShape)
	}
	switch *w.Type {
	case TypeResponse:
		return response(w)
	case TypeToolCall:
		return toolCall(w, tools, o)
	default:
		return Response{}, fmt.Errorf("%w: type %q", ErrInvented, *w.Type)
	}
}

func response(w wire) (Response, error) {
	if w.Tool != nil || w.Plan != nil || w.Args != nil {
		return Response{}, fmt.Errorf("%w: a response carries no tool", ErrShape)
	}
	if w.Text == nil || said(*w.Text) == "" {
		return Response{}, fmt.Errorf("%w: a response needs text", ErrShape)
	}
	r := Response{Type: TypeResponse, Text: said(*w.Text), Permission: consent.ClaimNormal}
	if w.Permission != nil {
		if !permission(*w.Permission) {
			return Response{}, fmt.Errorf("%w: %q", ErrPermission, *w.Permission)
		}
		r.Permission = *w.Permission
	}
	if w.Confirm != nil {
		r.Confirm = *w.Confirm
	}
	return r, nil
}

func toolCall(w wire, tools []string, o Offer) (Response, error) {
	if w.Plan != nil {
		return Response{}, fmt.Errorf("%w: no plan is offered", ErrInvented)
	}
	if w.Tool == nil {
		return Response{}, fmt.Errorf("%w: no tool", ErrShape)
	}
	if !slices.Contains(tools, *w.Tool) {
		return Response{}, fmt.Errorf("%w: tool %q", ErrInvented, *w.Tool)
	}
	if w.Text != nil {
		return Response{}, fmt.Errorf("%w: a tool's words go in its args", ErrShape)
	}
	if w.Permission == nil || !permission(*w.Permission) {
		got := "nothing"
		if w.Permission != nil {
			got = fmt.Sprintf("%q", *w.Permission)
		}
		return Response{}, fmt.Errorf("%w: got %s", ErrPermission, got)
	}
	if w.Confirm == nil {
		return Response{}, fmt.Errorf("%w: no confirm", ErrShape)
	}
	if w.Args == nil {
		return Response{}, fmt.Errorf("%w: no args", ErrArgs)
	}
	var a wireArgs
	if err := strict(w.Args, &a); err != nil {
		return Response{}, fmt.Errorf("%w: %v", ErrArgs, err)
	}
	args, err := fit(*w.Tool, a, o)
	if err != nil {
		return Response{}, err
	}
	return Response{Type: TypeToolCall, Tool: *w.Tool, Args: args, Confirm: *w.Confirm, Permission: *w.Permission}, nil
}

func permission(p string) bool {
	return p == consent.ClaimNormal || p == consent.ClaimSensitive || p == consent.ClaimCritical
}

// need is the arguments each tool takes; scroll may also name a list.
var need = map[string][]string{
	ToolClick: {"id"}, ToolFill: {"id", "text"}, ToolSelect: {"id", "option"},
	ToolScroll: {"dir"}, ToolWait: {"ms"}, ToolDone: {"text"}, ToolAskUser: {"text"},
	ToolType: {"text"}, ToolPress: {"keys"}, ToolOpenApp: {"name"}, ToolFindFiles: {"query"}, ToolShowFile: {"index"},
}

// fit checks a tool's arguments: exactly the ones it takes, and every id one
// of the offer's.
func fit(tool string, a wireArgs, o Offer) (Args, error) {
	have := map[string]bool{
		"id": a.ID != nil, "text": a.Text != nil, "option": a.Option != nil, "dir": a.Dir != nil, "ms": a.Ms != nil,
		"keys": a.Keys != nil, "name": a.Name != nil, "query": a.Query != nil, "index": a.Index != nil,
	}
	for _, k := range need[tool] {
		if !have[k] {
			return Args{}, fmt.Errorf("%w: %s needs %s", ErrArgs, tool, k)
		}
		delete(have, k)
	}
	if tool == ToolScroll {
		delete(have, "id") // a scroll may name the list it scrolls
	}
	for k, set := range have {
		if set {
			return Args{}, fmt.Errorf("%w: %s takes no %s", ErrArgs, tool, k)
		}
	}
	var out Args
	if a.ID != nil {
		if !slices.Contains(o.Controls, *a.ID) {
			return Args{}, fmt.Errorf("%w: id %q is not in the control list", ErrArgs, *a.ID)
		}
		out.ID = *a.ID
	}
	switch tool {
	case ToolFill:
		out.Text = *a.Text // typed as given; never cleaned or kept
	case ToolType:
		if strings.TrimSpace(*a.Text) == "" {
			return Args{}, fmt.Errorf("%w: type needs text", ErrArgs)
		}
		out.Text = *a.Text // typed as given; consent judges it, nothing keeps it
	case ToolSelect:
		if strings.TrimSpace(*a.Option) == "" {
			return Args{}, fmt.Errorf("%w: select needs an option", ErrArgs)
		}
		out.Option = *a.Option
	case ToolScroll:
		if *a.Dir != DirUp && *a.Dir != DirDown {
			return Args{}, fmt.Errorf("%w: dir %q", ErrArgs, *a.Dir)
		}
		out.Dir = *a.Dir
	case ToolWait:
		if *a.Ms <= 0 {
			return Args{}, fmt.Errorf("%w: ms must be above zero", ErrArgs)
		}
		out.Ms = *a.Ms
	case ToolPress:
		c, err := apprules.ParseChord(*a.Keys)
		if err != nil {
			return Args{}, fmt.Errorf("%w: keys are not one chord", ErrArgs)
		}
		out.Keys = c.String()
	case ToolOpenApp:
		if out.Name = said(*a.Name); out.Name == "" || utf8.RuneCountInString(out.Name) > MaxAppNameRunes {
			return Args{}, fmt.Errorf("%w: open_app needs one short name", ErrArgs)
		}
	case ToolFindFiles:
		if err := apprules.Query(*a.Query); err != nil {
			return Args{}, fmt.Errorf("%w: %v", ErrArgs, err)
		}
		out.Query = strings.TrimSpace(*a.Query)
	case ToolShowFile:
		if *a.Index < 1 || *a.Index > o.Files {
			return Args{}, fmt.Errorf("%w: index is not one of the files listed", ErrArgs)
		}
		out.Index = *a.Index
	case ToolDone, ToolAskUser:
		if out.Text = said(*a.Text); out.Text == "" {
			return Args{}, fmt.Errorf("%w: %s needs words", ErrArgs, tool)
		}
	}
	return out, nil
}

// said makes model words fit to show and speak: no hidden runes, one line.
func said(s string) string {
	s = strings.ToValidUTF8(s, "")
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// one returns the single JSON object raw holds. Surrounding white space and
// one code fence around the whole reply are tolerated; any other text, a
// second value or an array is not.
func one(raw string) ([]byte, error) {
	if !utf8.ValidString(raw) {
		return nil, fmt.Errorf("%w: not text", ErrJSON)
	}
	s := strings.TrimSpace(raw)
	if rest, ok := strings.CutPrefix(s, "```"); ok {
		if i := strings.IndexByte(rest, '\n'); i >= 0 && strings.HasSuffix(rest, "```") {
			if lang := strings.TrimSpace(rest[:i]); lang == "" || lang == "json" {
				s = strings.TrimSpace(strings.TrimSuffix(rest[i+1:], "```"))
			}
		}
	}
	switch {
	case s == "":
		return nil, fmt.Errorf("%w: empty", ErrJSON)
	case s[0] == '[':
		return nil, fmt.Errorf("%w: a list", ErrMultiple)
	case s[0] != '{':
		return nil, fmt.Errorf("%w: text before the object", ErrJSON)
	}
	dec := json.NewDecoder(strings.NewReader(s))
	var obj json.RawMessage
	if err := dec.Decode(&obj); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrJSON, err)
	}
	if rest := strings.TrimSpace(s[dec.InputOffset():]); rest != "" {
		return nil, fmt.Errorf("%w: more after the object", ErrMultiple)
	}
	if dup(obj) {
		return nil, fmt.Errorf("%w: a field given twice", ErrMultiple)
	}
	return obj, nil
}

// strict decodes body into v, refusing unknown fields and wrong kinds.
func strict(body []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: %v", ErrShape, err)
	}
	return nil
}

// dup reports an object anywhere in body that names one key twice: the
// decoder would quietly keep the last, and the reply would mean two things.
func dup(body []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(body))
	var walk func() bool
	walk = func() bool {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		switch tok {
		case json.Delim('{'):
			seen := map[string]bool{}
			for dec.More() {
				k, err := dec.Token()
				if err != nil {
					return false
				}
				key, _ := k.(string)
				if seen[key] {
					return true
				}
				seen[key] = true
				if walk() {
					return true
				}
			}
			_, _ = dec.Token()
		case json.Delim('['):
			for dec.More() {
				if walk() {
					return true
				}
			}
			_, _ = dec.Token()
		}
		return false
	}
	return walk()
}

// Corrective is the one line sent back when a reply is refused, asking for
// the shape once more. It names what was wrong and never repeats the reply.
func Corrective(err error) string {
	why := "That reply could not be used."
	switch {
	case errors.Is(err, ErrMultiple):
		why = "That reply held more than one action."
	case errors.Is(err, ErrInvented):
		why = "That reply named a tool, plan or type that was not offered."
	case errors.Is(err, ErrPermission):
		why = "That reply's permission was not normal, sensitive or critical."
	case errors.Is(err, ErrArgs):
		why = "That reply's arguments did not fit its tool or named an id that is not in the list."
	case errors.Is(err, ErrShape):
		why = "That reply was missing a field or carried one that does not belong."
	case errors.Is(err, ErrJSON):
		why = "That reply was not a single JSON object."
	}
	return why + " Answer again with exactly one JSON object in the shape described, and nothing else."
}
