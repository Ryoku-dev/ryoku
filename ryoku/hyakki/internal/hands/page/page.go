// Package page drives the one tab the person authorized, over the DevTools
// pipe: it lists tabs, binds one by target id and origin, reads the page
// through its accessibility tree within the spec's bounds, takes pictures
// of that tab's viewport only, and runs the steps the consent rules call
// safe. Every step is classified again here, against what the element
// really is, and anything not safe is refused: a handoff is the person's
// press, never this package's.
//
// What the tab does by itself (a navigation, a dialog, a popup, a
// download, being hidden, closing or crashing) arrives as an Event the
// moment the browser reports it; nothing polls.
package page

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"ryoku-rashin/internal/hands/cdp"
	"ryoku-rashin/internal/oni/hands/consent"
)

var (
	// ErrNotAuthorized means no tab is authorized.
	ErrNotAuthorized = errors.New("page: no tab is authorized")
	// ErrUnknownTab means the tab is not one the browser lists, or its
	// address has no origin Hands may bind.
	ErrUnknownTab = errors.New("page: that tab cannot be authorized")
	// ErrOrigin means the tab is no longer on the authorized origin.
	ErrOrigin = errors.New("page: the tab left its authorized origin")
	// ErrAway means the tab is hidden, closed, crashed or showing a dialog.
	ErrAway = errors.New("page: the tab is not available")
	// ErrStale means the control is not in the current page reading.
	ErrStale = errors.New("page: that control is gone; read the page again")
	// ErrNotSafe means the consent rules did not call the step safe.
	ErrNotSafe = errors.New("page: that step is not Hands' to take")
	// ErrText means text that may not be typed there.
	ErrText = errors.New("page: that text cannot be typed there")
	// ErrCovered means something else sits on top of the control.
	ErrCovered = errors.New("page: something covers that control")
	// ErrNoOption means the select has no such option.
	ErrNoOption = errors.New("page: no such option")
	// ErrLimits means limits that are not all positive.
	ErrLimits = errors.New("page: limits must be positive")
)

// Conn is the protocol connection; *cdp.Conn is one.
type Conn interface {
	Call(ctx context.Context, session, method string, params, out any) error
	Events() <-chan cdp.Event
}

// Limits bound one reading and one step; the daemon fills them from the
// spec's hands section.
type Limits struct {
	MaxControls  int
	MaxNameRunes int
	MaxTextRunes int
	MaxFillRunes int
	Act          time.Duration // one step, read-back included
}

func (l Limits) valid() bool {
	return l.MaxControls > 0 && l.MaxNameRunes > 0 && l.MaxTextRunes > 0 && l.MaxFillRunes > 0 && l.Act > 0
}

// Tab is one tab the browser has. Title is page text.
type Tab struct {
	ID     string
	Origin string
	Title  string
}

// Scope is the authorized tab.
type Scope struct {
	TabID  string
	Origin string
}

// Control is one control of a reading. ID is valid until the next reading
// or navigation. Class and Reason are a first sorting by role and name
// only; Act sorts again against the element itself.
type Control struct {
	ID     string
	Role   string
	Name   string
	Value  string
	Class  consent.Class
	Reason consent.Reason
}

// Snapshot is one reading of the authorized tab.
type Snapshot struct {
	Origin    string
	Controls  []Control
	Text      string // visible text, page text, bounded
	Frames    int    // embedded frames; their controls are never offered
	Truncated bool   // a bound cut controls, names or text
	Picture   []byte // PNG of the tab's viewport
	Print     string // fingerprint of the reading, for "did anything change"
}

// Act is one step.
type Act struct {
	Verb    string // consent.VerbClick, VerbFill, VerbSelect, VerbScroll or VerbWait
	Control string // Control.ID for click, fill and select
	Text    string // fill
	Option  string // select: the option's label
	DY      int    // scroll, CSS pixels
	Ms      int    // wait
	Claim   string // the model's claim; "" is consent.ClaimNormal
}

// MaxScroll bounds one scroll.
const MaxScroll = 10000

// Result is what a step did. It holds no typed words.
type Result struct {
	Class    consent.Class
	Reason   consent.Reason
	Done     bool
	ReadBack bool // fill and select: the page holds what was asked
	ScrollY  int
}

// Kind is what happened to the tab.
type Kind string

const (
	Navigated Kind = "navigated"
	Hidden    Kind = "hidden"
	Visible   Kind = "visible"
	Closed    Kind = "closed"
	Crashed   Kind = "crashed"
	Dialog    Kind = "dialog"
	Popup     Kind = "popup"
	Download  Kind = "download"
	// Pressed: the person pressed the control an Await armed. It comes
	// only from the page's own trusted events, never from Hyakki.
	Pressed Kind = "pressed"
)

// Event is one thing the tab did by itself.
type Event struct {
	Kind    Kind
	Origin  string // Navigated: the new origin
	Control string // Pressed: the control id Await armed
	// Away says Hands must pause: anything but the tab coming back into
	// view or a navigation within the authorized origin.
	Away bool
}

// Driver is what the daemon drives.
type Driver interface {
	Tabs(ctx context.Context) ([]Tab, error)
	Authorize(ctx context.Context, tabID string) (Scope, error)
	Page(ctx context.Context) (Snapshot, error)
	Act(ctx context.Context, a Act) (Result, error)
	Shot(ctx context.Context) ([]byte, error)
	Events() <-chan Event
	Await(ctx context.Context, control string) error
	Close() error
}

var _ Driver = (*Session)(nil)

// Session is the Driver over one browser connection.
type Session struct {
	conn Conn
	lim  Limits

	mu        sync.Mutex
	scope     Scope
	session   string
	frame     string // main frame id
	origin    string // main frame's current origin
	world     int    // isolated world's context id, 0 until made
	gen       int
	nodes     map[string]control
	hidden    bool
	dialog    bool
	gone      Kind // Closed or Crashed once the tab is lost
	arms      int  // Await tokens handed out
	armed     armed
	closedTab *guardClosedTab // one address, retained only for guard undo

	events chan Event
	done   chan struct{}
	once   sync.Once
}

// armed is the one control Await watches; token is empty when none.
type armed struct {
	token   string
	control string
}

type control struct {
	backend int
	c       Control
}

// Open starts watching the browser's targets on conn.
func Open(ctx context.Context, conn Conn, lim Limits) (*Session, error) {
	if !lim.valid() {
		return nil, ErrLimits
	}
	s := &Session{conn: conn, lim: lim, events: make(chan Event, 64), done: make(chan struct{})}
	if err := conn.Call(ctx, "", "Target.setDiscoverTargets", map[string]any{"discover": true}, nil); err != nil {
		return nil, err
	}
	go s.pump()
	return s, nil
}

// Events delivers what the tab does by itself; it closes after Close or
// when the browser goes.
func (s *Session) Events() <-chan Event { return s.events }

// Close stops driving. The browser stays as it is.
func (s *Session) Close() error {
	s.once.Do(func() {
		s.mu.Lock()
		sess := s.session
		s.session, s.scope, s.nodes, s.armed = "", Scope{}, nil, armed{}
		s.closedTab = nil
		s.mu.Unlock()
		if sess != "" {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			_ = s.conn.Call(ctx, "", "Target.detachFromTarget", map[string]any{"sessionId": sess}, nil)
			cancel()
		}
		close(s.done)
	})
	return nil
}

type targetInfo struct {
	TargetID string `json:"targetId"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	OpenerID string `json:"openerId"`
}

// Tabs lists the browser's tabs whose address has an origin Hands may bind.
func (s *Session) Tabs(ctx context.Context) ([]Tab, error) {
	var r struct {
		TargetInfos []targetInfo `json:"targetInfos"`
	}
	if err := s.conn.Call(ctx, "", "Target.getTargets", nil, &r); err != nil {
		return nil, err
	}
	var out []Tab
	for _, t := range r.TargetInfos {
		if t.Type != "page" {
			continue
		}
		if o := Origin(t.URL); o != "" {
			out = append(out, Tab{ID: t.TargetID, Origin: o, Title: clean(t.Title, s.lim.MaxNameRunes)})
		}
	}
	return out, nil
}

// Origin is scheme://host[:port] of an http or https address, "file://"
// for a file, and "" for anything Hands may not bind.
func Origin(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	switch u.Scheme {
	case "http", "https":
		if u.Host == "" || u.User != nil {
			return ""
		}
		return u.Scheme + "://" + strings.ToLower(u.Host)
	case "file":
		return "file://"
	}
	return ""
}

// Authorize binds the tab tabID by its target id and its origin now; any
// tab bound before is let go.
func (s *Session) Authorize(ctx context.Context, tabID string) (Scope, error) {
	tabs, err := s.Tabs(ctx)
	if err != nil {
		return Scope{}, err
	}
	var tab *Tab
	for i := range tabs {
		if tabs[i].ID == tabID {
			tab = &tabs[i]
		}
	}
	if tab == nil {
		return Scope{}, ErrUnknownTab
	}
	s.mu.Lock()
	old := s.session
	s.session, s.scope, s.nodes, s.world, s.gen = "", Scope{}, nil, 0, s.gen+1
	s.hidden, s.dialog, s.gone, s.armed = false, false, "", armed{}
	s.mu.Unlock()
	if old != "" {
		_ = s.conn.Call(ctx, "", "Target.detachFromTarget", map[string]any{"sessionId": old}, nil)
	}
	var att struct {
		SessionID string `json:"sessionId"`
	}
	if err := s.conn.Call(ctx, "", "Target.attachToTarget", map[string]any{"targetId": tab.ID, "flatten": true}, &att); err != nil {
		return Scope{}, err
	}
	sess := att.SessionID
	fail := func(err error) (Scope, error) {
		_ = s.conn.Call(ctx, "", "Target.detachFromTarget", map[string]any{"sessionId": sess}, nil)
		return Scope{}, err
	}
	s.mu.Lock()
	s.session = sess
	s.mu.Unlock()
	for _, m := range []string{"Page.enable", "DOM.enable", "Accessibility.enable", "Runtime.enable"} {
		if err := s.conn.Call(ctx, sess, m, nil, nil); err != nil {
			return fail(err)
		}
	}
	var tree struct {
		FrameTree struct {
			Frame struct {
				ID  string `json:"id"`
				URL string `json:"url"`
			} `json:"frame"`
		} `json:"frameTree"`
	}
	if err := s.conn.Call(ctx, sess, "Page.getFrameTree", nil, &tree); err != nil {
		return fail(err)
	}
	if Origin(tree.FrameTree.Frame.URL) != tab.Origin {
		return fail(ErrOrigin)
	}
	if err := s.conn.Call(ctx, sess, "Runtime.addBinding", map[string]any{"name": cdp.Binding, "executionContextName": cdp.World}, nil); err != nil {
		return fail(err)
	}
	if err := s.conn.Call(ctx, sess, "Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": cdp.WatchScript, "worldName": cdp.World}, nil); err != nil {
		return fail(err)
	}
	s.mu.Lock()
	s.scope = Scope{TabID: tab.ID, Origin: tab.Origin}
	s.frame, s.origin = tree.FrameTree.Frame.ID, tab.Origin
	s.mu.Unlock()
	if _, err := s.ensureWorld(ctx); err != nil {
		s.mu.Lock()
		s.session, s.scope = "", Scope{}
		s.mu.Unlock()
		return fail(err)
	}
	return Scope{TabID: tab.ID, Origin: tab.Origin}, nil
}

// ready checks the tab may be read or acted on, and returns its session.
func (s *Session) ready() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.session == "":
		return "", ErrNotAuthorized
	case s.gone != "" || s.hidden || s.dialog:
		return "", ErrAway
	case s.origin != s.scope.Origin:
		return "", ErrOrigin
	}
	return s.session, nil
}

// ensureWorld makes the isolated world in the current document when there
// is none yet, and starts its visibility watch.
func (s *Session) ensureWorld(ctx context.Context) (int, error) {
	s.mu.Lock()
	sess, frame, world, gen := s.session, s.frame, s.world, s.gen
	s.mu.Unlock()
	if world != 0 {
		return world, nil
	}
	var w struct {
		ExecutionContextID int `json:"executionContextId"`
	}
	if err := s.conn.Call(ctx, sess, "Page.createIsolatedWorld", map[string]any{"frameId": frame, "worldName": cdp.World}, &w); err != nil {
		return 0, err
	}
	if w.ExecutionContextID == 0 {
		return 0, ErrAway
	}
	var ok bool
	if err := s.call(ctx, sess, cdp.FnWatch, "", w.ExecutionContextID, &ok); err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gen != gen || s.session != sess {
		return 0, ErrStale
	}
	s.world = w.ExecutionContextID
	return s.world, nil
}

// call runs one table function and decodes its value into out.
func (s *Session) call(ctx context.Context, sess string, fn cdp.Function, objectID string, contextID int, out any, args ...any) error {
	p, ok := cdp.FunctionCall(fn, objectID, contextID, args...)
	if !ok {
		return cdp.ErrRefused
	}
	var r cdp.Result
	if err := s.conn.Call(ctx, sess, "Runtime.callFunctionOn", p, &r); err != nil {
		return err
	}
	if r.ExceptionDetails != nil {
		return fmt.Errorf("page: %s failed in the page", fn)
	}
	if out == nil || len(r.Result.Value) == 0 {
		return nil
	}
	return json.Unmarshal(r.Result.Value, out)
}

// Shot is a PNG of the authorized tab's viewport, nothing beyond it.
func (s *Session) Shot(ctx context.Context) ([]byte, error) {
	sess, err := s.ready()
	if err != nil {
		return nil, err
	}
	var r struct {
		Data string `json:"data"`
	}
	if err := s.conn.Call(ctx, sess, "Page.captureScreenshot", map[string]any{"format": "png"}, &r); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(r.Data)
}

// Page reads the authorized tab: its controls, its visible text and a
// picture, each within the limits. It replaces the previous reading's
// control ids.
func (s *Session) Page(ctx context.Context) (Snapshot, error) {
	sess, err := s.ready()
	if err != nil {
		return Snapshot{}, err
	}
	s.mu.Lock()
	gen, frame, origin := s.gen, s.frame, s.origin
	s.mu.Unlock()
	world, err := s.ensureWorld(ctx)
	var parsed bool
	if err == nil {
		err = s.call(ctx, sess, cdp.FnReady, "", world, &parsed)
	}
	if err != nil {
		var protocolErr *cdp.Error
		if !errors.As(err, &protocolErr) || protocolErr.Code != -32000 || protocolErr.Message != "Cannot find context with specified id" {
			return Snapshot{}, err
		}
		// Navigation can destroy the cached world before its frame event arrives.
		// Invalidate that reading, never an action or a lost scope.
		s.mu.Lock()
		switch {
		case s.session != sess:
			err = ErrNotAuthorized
		case s.gone != "" || s.hidden || s.dialog:
			err = ErrAway
		case s.origin != origin || s.origin != s.scope.Origin:
			err = ErrOrigin
		default:
			if s.world == world {
				s.world, s.nodes, s.armed = 0, nil, armed{}
				s.gen++
			}
			err = ErrStale
		}
		s.mu.Unlock()
		return Snapshot{}, err
	}
	if !parsed {
		return Snapshot{}, ErrStale
	}
	var root struct {
		Node axNode `json:"node"`
	}
	if err := s.conn.Call(ctx, sess, "Accessibility.getRootAXNode", map[string]any{"frameId": frame}, &root); err != nil {
		return Snapshot{}, err
	}
	r := reading{ids: map[string]control{}, frame: frame, lim: s.lim}
	stack := []axNode{root.Node}
	seen := map[string]bool{}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n.NodeID == "" {
			return Snapshot{}, ErrStale
		}
		if seen[n.NodeID] {
			continue
		}
		seen[n.NodeID] = true
		role := strings.ToLower(n.Role.text())
		descend := r.add(n, role) && len(n.ChildIDs) > 0
		if r.full() && (len(stack) > 0 || descend) {
			r.snap.Truncated = true
			break
		}
		if !descend {
			continue
		}
		if leaf, err := s.textLeaf(ctx, sess, n, role, &r); err != nil {
			return Snapshot{}, err
		} else if leaf {
			if r.full() && len(stack) > 0 {
				r.snap.Truncated = true
				break
			}
			continue
		}
		var children struct {
			Nodes []axNode `json:"nodes"`
		}
		if err := s.conn.Call(ctx, sess, "Accessibility.getChildAXNodes", map[string]any{"id": n.NodeID, "frameId": frame}, &children); err != nil {
			return Snapshot{}, err
		}
		byID := make(map[string]int, len(children.Nodes))
		for i := range children.Nodes {
			byID[children.Nodes[i].NodeID] = i
		}
		for i := len(n.ChildIDs) - 1; i >= 0; i-- {
			child, ok := byID[n.ChildIDs[i]]
			if !ok {
				return Snapshot{}, ErrStale
			}
			stack = append(stack, children.Nodes[child])
		}
	}
	snap, nodes := r.snap, r.ids
	snap.Text = r.text.String()
	snap.Origin = origin
	pic, err := s.Shot(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	snap.Picture = pic
	snap.Print = fingerprint(snap)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gen != gen || s.session != sess {
		return Snapshot{}, ErrStale
	}
	s.nodes, s.armed = nodes, armed{}
	return snap, nil
}

// fingerprint fingerprints what a reading says, picture aside.
func fingerprint(snap Snapshot) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%d\x00", snap.Origin, snap.Frames)
	for _, c := range snap.Controls {
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00", c.Role, c.Name, c.Value)
	}
	h.Write([]byte(snap.Text))
	return hex.EncodeToString(h.Sum(nil)[:16])
}

// Act re-checks the tab, sorts the step against the element it lands on,
// and runs it only when the consent rules call it safe.
func (s *Session) Act(ctx context.Context, a Act) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, s.lim.Act)
	defer cancel()
	sess, err := s.ready()
	if err != nil {
		return Result{}, err
	}
	claim := a.Claim
	if claim == "" {
		claim = consent.ClaimNormal
	}
	switch a.Verb {
	case consent.VerbWait, consent.VerbScroll:
		cls, why := consent.Classify(a.Verb, consent.Subject{}, claim)
		res := Result{Class: cls, Reason: why}
		if cls != consent.Safe {
			return res, ErrNotSafe
		}
		if a.Verb == consent.VerbWait {
			d := time.Duration(max(a.Ms, 0)) * time.Millisecond
			t := time.NewTimer(min(d, s.lim.Act))
			defer t.Stop()
			select {
			case <-t.C:
			case <-ctx.Done():
			}
			res.Done = true
			return res, nil
		}
		if a.DY < -MaxScroll || a.DY > MaxScroll {
			return res, ErrText
		}
		world, err := s.ensureWorld(ctx)
		if err != nil {
			return res, err
		}
		if err := s.call(ctx, sess, cdp.FnScroll, "", world, &res.ScrollY, a.DY); err != nil {
			return res, err
		}
		res.Done = true
		return res, nil
	case consent.VerbClick, consent.VerbFill, consent.VerbSelect:
	default:
		cls, why := consent.Classify(a.Verb, consent.Subject{}, claim)
		return Result{Class: cls, Reason: why}, ErrNotSafe
	}

	s.mu.Lock()
	node, ok := s.nodes[a.Control]
	s.mu.Unlock()
	if !ok {
		return Result{}, ErrStale
	}
	world, err := s.ensureWorld(ctx)
	if err != nil {
		return Result{}, err
	}
	id, release, err := s.resolve(ctx, sess, node.backend, world)
	if err != nil {
		return Result{}, err
	}
	defer release()
	var d struct {
		OK           bool   `json:"ok"`
		Type         string `json:"type"`
		Autocomplete string `json:"autocomplete"`
		Submits      bool   `json:"submits"`
		Pays         bool   `json:"pays"`
		Editable     bool   `json:"editable"`
		Multiline    bool   `json:"multiline"`
		Disabled     bool   `json:"disabled"`
		Hidden       bool   `json:"hidden"`
	}
	if err := s.call(ctx, sess, cdp.FnDescribe, id, 0, &d); err != nil {
		return Result{}, err
	}
	if !d.OK {
		return Result{}, ErrStale
	}
	subject := consent.Subject{
		Role: node.c.Role, Name: node.c.Name, InputType: d.Type, Autocomplete: d.Autocomplete,
		Submits: d.Submits, PaymentForm: d.Pays, Hidden: d.Hidden,
	}
	if a.Verb == consent.VerbSelect {
		subject.Option = a.Option
	}
	cls, why := consent.Classify(a.Verb, subject, claim)
	res := Result{Class: cls, Reason: why}
	if cls != consent.Safe {
		return res, ErrNotSafe
	}
	if d.Disabled {
		return res, ErrStale
	}
	switch a.Verb {
	case consent.VerbFill:
		if !d.Editable || !typable(a.Text, d.Multiline, s.lim.MaxFillRunes) {
			return res, ErrText
		}
		if err := s.conn.Call(ctx, sess, "DOM.scrollIntoViewIfNeeded", map[string]any{"backendNodeId": node.backend}, nil); err != nil {
			return res, err
		}
		var focused bool
		if err := s.call(ctx, sess, cdp.FnPrepare, id, 0, &focused); err != nil {
			return res, err
		}
		if !focused {
			return res, ErrCovered
		}
		if a.Text == "" {
			err = s.call(ctx, sess, cdp.FnErase, id, 0, nil)
		} else {
			err = s.conn.Call(ctx, sess, "Input.insertText", map[string]any{"text": a.Text}, nil)
		}
		if err != nil {
			return res, err
		}
		res.Done = true
		var got string
		if err := s.call(ctx, sess, cdp.FnReadValue, id, 0, &got); err != nil {
			return res, nil
		}
		res.ReadBack = got == a.Text
		return res, nil
	case consent.VerbSelect:
		var chosen *string
		if err := s.call(ctx, sess, cdp.FnChoose, id, 0, &chosen, a.Option); err != nil {
			return res, err
		}
		if chosen == nil {
			return res, ErrNoOption
		}
		res.Done = true
		var got string
		if err := s.call(ctx, sess, cdp.FnReadValue, id, 0, &got); err != nil {
			return res, nil
		}
		res.ReadBack = got == *chosen
		return res, nil
	}
	// click
	if err := s.conn.Call(ctx, sess, "DOM.scrollIntoViewIfNeeded", map[string]any{"backendNodeId": node.backend}, nil); err != nil {
		return res, err
	}
	var box struct {
		Model struct {
			Content []float64 `json:"content"`
		} `json:"model"`
	}
	if err := s.conn.Call(ctx, sess, "DOM.getBoxModel", map[string]any{"backendNodeId": node.backend}, &box); err != nil {
		return res, ErrStale
	}
	q := box.Model.Content
	if len(q) != 8 {
		return res, ErrStale
	}
	x, y := (q[0]+q[2]+q[4]+q[6])/4, (q[1]+q[3]+q[5]+q[7])/4
	var hit bool
	if err := s.call(ctx, sess, cdp.FnHit, id, 0, &hit, x, y); err != nil {
		return res, err
	}
	if !hit {
		return res, ErrCovered
	}
	for _, ev := range []map[string]any{
		{"type": "mouseMoved", "x": x, "y": y, "button": "none", "clickCount": 0},
		{"type": "mousePressed", "x": x, "y": y, "button": "left", "clickCount": 1},
		{"type": "mouseReleased", "x": x, "y": y, "button": "left", "clickCount": 1},
	} {
		if err := s.conn.Call(ctx, sess, "Input.dispatchMouseEvent", ev, nil); err != nil {
			return res, err
		}
	}
	res.Done = true
	return res, nil
}

// typable says whether text may be typed: within the bound, no control
// characters, and line breaks or tabs only where the field has lines.
func typable(text string, multiline bool, maxRunes int) bool {
	if !utf8.ValidString(text) || utf8.RuneCountInString(text) > maxRunes {
		return false
	}
	for _, r := range text {
		if r == '\n' || r == '\t' {
			if !multiline {
				return false
			}
			continue
		}
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// resolve finds a reading's node as an object in the world; release lets
// the object go.
func (s *Session) resolve(ctx context.Context, sess string, backend, world int) (string, func(), error) {
	var obj struct {
		Object struct {
			ObjectID string `json:"objectId"`
		} `json:"object"`
	}
	if err := s.conn.Call(ctx, sess, "DOM.resolveNode", map[string]any{"backendNodeId": backend, "executionContextId": world}, &obj); err != nil {
		return "", nil, ErrStale
	}
	id := obj.Object.ObjectID
	if id == "" {
		return "", nil, ErrStale
	}
	return id, func() {
		rctx, rcancel := context.WithTimeout(context.Background(), time.Second)
		_ = s.conn.Call(rctx, sess, "Runtime.releaseObject", map[string]any{"objectId": id}, nil)
		rcancel()
	}, nil
}

// Await watches one control of the current reading for the person's own
// press and reports it once as a Pressed event. It never presses anything:
// only the page's trusted events count, which nothing Hyakki sends to this
// control can make. A later Await replaces it; a new reading, a navigation,
// a new authorization or Close drops it.
func (s *Session) Await(ctx context.Context, control string) error {
	sess, err := s.ready()
	if err != nil {
		return err
	}
	s.mu.Lock()
	node, ok := s.nodes[control]
	gen := s.gen
	s.mu.Unlock()
	if !ok {
		return ErrStale
	}
	world, err := s.ensureWorld(ctx)
	if err != nil {
		return err
	}
	id, release, err := s.resolve(ctx, sess, node.backend, world)
	if err != nil {
		return err
	}
	defer release()
	// Record the arming before the page holds it, so a press that comes at
	// once is not missed.
	s.mu.Lock()
	if s.gen != gen || s.session != sess {
		s.mu.Unlock()
		return ErrStale
	}
	s.arms++
	token := "a" + strconv.Itoa(s.arms)
	s.armed = armed{token: token, control: control}
	s.mu.Unlock()
	disarm := func() {
		s.mu.Lock()
		if s.armed.token == token {
			s.armed = armed{}
		}
		s.mu.Unlock()
	}
	var ok2 bool
	if err := s.call(ctx, sess, cdp.FnAwait, id, 0, &ok2, token); err != nil {
		disarm()
		return err
	}
	if !ok2 {
		disarm()
		return ErrStale
	}
	return nil
}
