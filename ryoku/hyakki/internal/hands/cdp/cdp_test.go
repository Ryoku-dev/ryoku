package cdp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type end struct {
	r *io.PipeReader
	w *io.PipeWriter
}

func (e end) Read(p []byte) (int, error)  { return e.r.Read(p) }
func (e end) Write(p []byte) (int, error) { return e.w.Write(p) }
func (e end) Close() error                { e.r.Close(); return e.w.Close() }

// browser is the far side of the pipe.
type browser struct {
	end
	br *bufio.Reader
}

func pair(t *testing.T) (*Conn, *browser) {
	t.Helper()
	ar, bw := io.Pipe()
	br, aw := io.Pipe()
	c := Dial(end{ar, aw})
	b := &browser{end: end{br, bw}}
	b.br = bufio.NewReader(b)
	t.Cleanup(func() { c.Close(); b.Close() })
	return c, b
}

func (b *browser) next(t *testing.T) map[string]any {
	t.Helper()
	line, err := b.br.ReadBytes(0)
	if err != nil {
		t.Fatalf("browser read: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(line[:len(line)-1], &m); err != nil {
		t.Fatalf("message %q: %v", line, err)
	}
	return m
}

func (b *browser) send(t *testing.T, v any) {
	t.Helper()
	raw, _ := json.Marshal(v)
	if _, err := b.Write(append(raw, 0)); err != nil {
		t.Fatalf("browser write: %v", err)
	}
}

func TestCallSpeaksNULFramedJSONWithFlatSessions(t *testing.T) {
	c, b := pair(t)
	type got struct {
		res struct {
			Data string `json:"data"`
		}
		err error
	}
	done := make(chan got, 1)
	go func() {
		var g got
		g.err = c.Call(context.Background(), "S1", "Page.captureScreenshot", map[string]any{"format": "png"}, &g.res)
		done <- g
	}()
	m := b.next(t)
	if m["method"] != "Page.captureScreenshot" || m["sessionId"] != "S1" || m["params"].(map[string]any)["format"] != "png" {
		t.Fatalf("sent %v", m)
	}
	b.send(t, map[string]any{"id": m["id"], "sessionId": "S1", "result": map[string]any{"data": "AAA"}})
	g := <-done
	if g.err != nil || g.res.Data != "AAA" {
		t.Fatalf("result %+v", g)
	}

	go func() { done <- got{err: c.Call(context.Background(), "", "Target.getTargets", nil, nil)} }()
	m = b.next(t)
	if _, ok := m["sessionId"]; ok {
		t.Fatalf("browser command carries a session: %v", m)
	}
	b.send(t, map[string]any{"id": m["id"], "error": map[string]any{"code": -32000, "message": "no"}})
	var pe *Error
	if g := <-done; !errors.As(g.err, &pe) || pe.Code != -32000 {
		t.Fatalf("protocol error = %v", g.err)
	}
}

func TestARefusedCommandIsNeverWritten(t *testing.T) {
	c, b := pair(t)
	for _, m := range []string{"Runtime.evaluate", "Network.getAllCookies", "Page.navigate", "Browser.close"} {
		if err := c.Call(context.Background(), "S", m, map[string]any{"expression": "1"}, nil); !errors.Is(err, ErrRefused) {
			t.Fatalf("%s: %v", m, err)
		}
	}
	// The next thing on the wire is the allowed call, nothing before it.
	go c.Call(context.Background(), "", "Target.getTargets", nil, nil)
	if m := b.next(t); m["method"] != "Target.getTargets" {
		t.Fatalf("first message %v", m)
	}
}

func TestOnlyListedEventsAreForwarded(t *testing.T) {
	c, b := pair(t)
	b.send(t, map[string]any{"method": "Runtime.consoleAPICalled", "params": map[string]any{}})
	b.send(t, map[string]any{"method": "Network.requestWillBeSent", "params": map[string]any{}})
	b.send(t, map[string]any{"method": "Page.javascriptDialogOpening", "sessionId": "S1", "params": map[string]any{"type": "alert"}})
	select {
	case e := <-c.Events():
		if e.Method != "Page.javascriptDialogOpening" || e.Session != "S1" {
			t.Fatalf("event %+v", e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no event")
	}
}

func TestAnEventFloodEndsTheConnection(t *testing.T) {
	c, b := pair(t)
	go func() {
		for range EventBuffer + 10 {
			raw, _ := json.Marshal(map[string]any{"method": "Page.windowOpen", "params": map[string]any{}})
			if _, err := b.Write(append(raw, 0)); err != nil {
				return
			}
		}
	}()
	select {
	case <-c.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("flood did not end the connection")
	}
	if !errors.Is(c.Err(), ErrFlood) {
		t.Fatalf("err = %v", c.Err())
	}
}

func TestAnOversizedMessageEndsTheConnection(t *testing.T) {
	c, b := pair(t)
	go func() {
		big := strings.Repeat("a", 1<<20)
		for range MaxMessage>>20 + 2 {
			if _, err := io.WriteString(b, big); err != nil {
				return
			}
		}
	}()
	select {
	case <-c.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("oversized message did not end the connection")
	}
	if !errors.Is(c.Err(), ErrTooLarge) {
		t.Fatalf("err = %v", c.Err())
	}
}

func TestCloseReleasesWaitingCallsAndEvents(t *testing.T) {
	c, b := pair(t)
	done := make(chan error, 1)
	go func() { done <- c.Call(context.Background(), "", "Target.getTargets", nil, nil) }()
	b.next(t)
	b.Close()
	if err := <-done; !errors.Is(err, ErrClosed) {
		t.Fatalf("waiting call = %v", err)
	}
	if _, ok := <-c.Events(); ok {
		t.Fatal("events open after close")
	}
	if err := c.Call(context.Background(), "", "Target.getTargets", nil, nil); !errors.Is(err, ErrClosed) {
		t.Fatalf("call after close = %v", err)
	}
}

func TestCallHonoursItsContext(t *testing.T) {
	c, b := pair(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.Call(ctx, "", "Target.getTargets", nil, nil) }()
	b.next(t)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestRefusedByName(t *testing.T) {
	for _, m := range []string{
		"Runtime.evaluate", "Runtime.compileScript", "Runtime.runScript", "Debugger.evaluateOnCallFrame",
		"Network.getCookies", "Network.getAllCookies", "Network.setCookie", "Network.enable", "Storage.getCookies",
		"Storage.clearDataForOrigin", "Browser.close", "Browser.grantPermissions", "Browser.setDownloadBehavior",
		"Fetch.enable", "Fetch.fulfillRequest", "Target.closeTarget", "Target.createTarget", "Target.activateTarget",
		"Target.exposeDevToolsProtocol", "Target.createBrowserContext", "Page.navigate", "Page.reload",
		"Page.setDownloadBehavior", "Page.handleJavaScriptDialog", "Page.printToPDF", "Page.close", "Page.bringToFront",
		"Page.setInterceptFileChooserDialog", "Input.dispatchKeyEvent", "Input.dispatchTouchEvent",
		"Input.imeSetComposition", "DOM.setFileInputFiles", "DOM.setAttributeValue", "DOM.setOuterHTML",
		"DOM.getOuterHTML", "IndexedDB.requestData", "DOMStorage.getDOMStorageItems", "Emulation.setUserAgentOverride",
		"SystemInfo.getProcessInfo", "Tethering.bind", "", "page.enable",
	} {
		if Allowed(m, nil) || Allowed(m, map[string]any{}) {
			t.Errorf("%q allowed", m)
		}
	}
}

func TestShapesAreExact(t *testing.T) {
	fill, _ := FunctionCall(FnReadValue, "obj-1", 0)
	scroll, _ := FunctionCall(FnScroll, "", 7, 300)
	good := []struct {
		m string
		p any
	}{
		{"Target.getTargets", nil},
		{"Target.setDiscoverTargets", map[string]any{"discover": true}},
		{"Target.attachToTarget", map[string]any{"targetId": "T", "flatten": true}},
		{"Accessibility.getRootAXNode", map[string]any{"frameId": "F"}},
		{"Accessibility.getChildAXNodes", map[string]any{"id": "N", "frameId": "F"}},
		{"Page.captureScreenshot", map[string]any{"format": "png"}},
		{"Page.captureScreenshot", nil},
		{"Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": WatchScript, "worldName": World}},
		{"Page.createIsolatedWorld", map[string]any{"frameId": "F", "worldName": World}},
		{"Runtime.addBinding", map[string]any{"name": Binding, "executionContextName": World}},
		{"DOM.resolveNode", map[string]any{"backendNodeId": 4, "executionContextId": 9}},
		{"DOM.describeNode", map[string]any{"backendNodeId": 4, "depth": 0, "pierce": false}},
		{"Input.dispatchMouseEvent", map[string]any{"type": "mousePressed", "x": 10.5, "y": 3, "button": "left", "clickCount": 1}},
		{"Input.insertText", map[string]any{"text": "hello"}},
		{"Runtime.callFunctionOn", fill},
		{"Runtime.callFunctionOn", scroll},
	}
	for _, g := range good {
		if !Allowed(g.m, g.p) {
			t.Errorf("%s %v refused", g.m, g.p)
		}
	}
	bad := []struct {
		m string
		p any
	}{
		{"Target.getTargets", map[string]any{"filter": []any{}}},
		{"Target.attachToTarget", map[string]any{"targetId": "T"}},
		{"Target.attachToTarget", map[string]any{"targetId": "T", "flatten": false}},
		{"Accessibility.getFullAXTree", nil},
		{"Accessibility.getRootAXNode", nil},
		{"Accessibility.getRootAXNode", map[string]any{"frameId": ""}},
		{"Accessibility.getChildAXNodes", map[string]any{"id": "N"}},
		{"Accessibility.getChildAXNodes", map[string]any{"id": "N", "frameId": "F", "depth": -1}},
		{"Page.captureScreenshot", map[string]any{"captureBeyondViewport": true}},
		{"Page.captureScreenshot", map[string]any{"clip": map[string]any{"x": 0}}},
		{"Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": "alert(1)", "worldName": World}},
		{"Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": WatchScript}},
		{"Page.createIsolatedWorld", map[string]any{"frameId": "F", "worldName": World, "grantUniveralAccess": true}},
		{"Page.createIsolatedWorld", map[string]any{"frameId": "F", "worldName": "main"}},
		{"Runtime.addBinding", map[string]any{"name": "steal"}},
		{"Runtime.addBinding", map[string]any{"name": Binding}},
		{"DOM.resolveNode", map[string]any{"backendNodeId": 4}},
		{"DOM.resolveNode", map[string]any{"backendNodeId": -1, "executionContextId": 9}},
		{"DOM.describeNode", map[string]any{"backendNodeId": 4, "depth": -1, "pierce": false}},
		{"DOM.describeNode", map[string]any{"backendNodeId": 4, "depth": 0, "pierce": true}},
		{"DOM.describeNode", map[string]any{"nodeId": 4, "depth": 0, "pierce": false}},
		{"DOM.focus", map[string]any{"backendNodeId": 1.5}},
		{"Input.dispatchMouseEvent", map[string]any{"type": "mouseWheel", "x": 1, "y": 1}},
		{"Input.dispatchMouseEvent", map[string]any{"type": "mousePressed", "x": 1, "y": 1, "modifiers": 4}},
		{"Input.dispatchMouseEvent", map[string]any{"type": "mousePressed", "x": 1, "y": 1, "button": "right"}},
		{"Input.dispatchMouseEvent", map[string]any{"type": "mousePressed", "x": 1, "y": 1, "clickCount": 2}},
		{"Input.dispatchMouseEvent", map[string]any{"type": "mousePressed", "x": -1, "y": 1}},
		{"Input.insertText", map[string]any{"text": strings.Repeat("a", MaxText+1)}},
		{"Input.insertText", map[string]any{"text": nil}},
		{"Runtime.callFunctionOn", map[string]any{"functionDeclaration": "function(){return document.cookie}", "objectId": "o", "returnByValue": true}},
		{"Runtime.callFunctionOn", map[string]any{"functionDeclaration": mustSource(FnReadValue), "objectId": "o", "executionContextId": 3, "returnByValue": true}},
		{"Runtime.callFunctionOn", map[string]any{"functionDeclaration": mustSource(FnReadValue), "returnByValue": true}},
		{"Runtime.callFunctionOn", map[string]any{"functionDeclaration": mustSource(FnReadValue), "objectId": "o", "returnByValue": false}},
		{"Runtime.callFunctionOn", map[string]any{"functionDeclaration": mustSource(FnReadValue), "objectId": "o", "returnByValue": true, "awaitPromise": true}},
		{"Runtime.callFunctionOn", map[string]any{"functionDeclaration": mustSource(FnChoose), "objectId": "o", "returnByValue": true,
			"arguments": []any{map[string]any{"objectId": "window"}}}},
		{"Runtime.callFunctionOn", map[string]any{"functionDeclaration": mustSource(FnChoose), "objectId": "o", "returnByValue": true,
			"arguments": []any{map[string]any{"value": map[string]any{"x": 1}}}}},
		{"Runtime.enable", map[string]any{"maxScriptsCacheSize": 1}},
		{"Page.enable", "nope"},
		{"Page.enable", []any{}},
	}
	for _, b := range bad {
		if Allowed(b.m, b.p) {
			t.Errorf("%s %v allowed", b.m, b.p)
		}
	}
}

func mustSource(f Function) string {
	s, ok := f.Source()
	if !ok {
		panic(f)
	}
	return s
}

func TestEveryTableFunctionIsCallableAndNothingElse(t *testing.T) {
	for f := range functions {
		p, ok := FunctionCall(f, "o", 0, "x")
		if !ok || !Allowed("Runtime.callFunctionOn", p) {
			t.Errorf("%s not callable", f)
		}
	}
	if _, ok := FunctionCall("eval", "o", 0); ok {
		t.Fatal("a name outside the table built a call")
	}
	if _, ok := FunctionCall(FnHit, "o", 4); ok {
		t.Fatal("object and context together built a call")
	}
	if _, ok := FunctionCall(FnHit, "", 0); ok {
		t.Fatal("a call with no target was built")
	}
}
