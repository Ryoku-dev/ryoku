package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	wm "ryoku-wm"
)

// serveLines runs the serve verb over the given request lines and decodes every
// reply it wrote before stdin ended.
func serveLines(t *testing.T, lines ...string) []wm.ServeReply {
	t.Helper()
	prevOut := stdout
	stdout = bufio.NewWriter(&bytes.Buffer{})
	defer func() { stdout = prevOut }()
	var out bytes.Buffer
	if err := runServe(strings.NewReader(strings.Join(lines, "\n")+"\n"), &out); err != nil {
		t.Fatalf("serve returned %v at EOF, want nil", err)
	}
	var replies []wm.ServeReply
	dec := json.NewDecoder(&out)
	for dec.More() {
		var r wm.ServeReply
		if err := dec.Decode(&r); err != nil {
			t.Fatalf("reply stream: %v", err)
		}
		replies = append(replies, r)
	}
	return replies
}

// A warm provider must answer exactly as the one-shot verbs do: same request to
// wayfire, output kept to the request that printed it, the act error text
// unchanged, and verbs it does not serve refused rather than run.
func TestServeRunsActAndCapsInOrder(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	replies := stageReplies()
	replies["input/list-devices"] = `[{"id":42,"name":"ELAN1200:00 04f3:3098 Touchpad","type":"pointer","enabled":true}]`
	replies["wayfire/get-keyboard-state"] = `{"possible-layouts":["us","br"],"layout":"us"}`
	calls := stubAct(t, replies)

	wantErr := runAct([]string{"window.focus"}).Error()
	*calls = nil
	got := serveLines(t,
		`{"id":1,"argv":["act","input.touchpad","status"]}`,
		`{"id":2,"argv":["act","keyboard.cycleLayout"]}`,
		`{"id":3,"argv":["act","window.focus"]}`,
		`{"id":4,"argv":["watch"]}`,
		`{"id":5,"argv":["caps"]}`,
		`{"id":6,"argv":["act","input.touchpad","status"]}`,
	)
	if len(got) != 6 {
		t.Fatalf("got %d replies, want 6: %+v", len(got), got)
	}
	for i, r := range got {
		if r.ID != uint64(i+1) {
			t.Fatalf("reply %d has id %d: answers must keep request order", i, r.ID)
		}
	}
	if r := got[0]; !r.OK || r.Out != "on\n" {
		t.Errorf("status reply = %+v, want ok with on", r)
	}
	if r := got[1]; !r.OK || r.Out != "" {
		t.Errorf("keyboard.cycleLayout reply = %+v, want ok and silent", r)
	}
	wantCall(t, *calls, "wayfire/set-keyboard-state", `{"layout-index":1}`)
	if r := got[2]; r.OK || r.Err != wantErr {
		t.Errorf("window.focus reply = %+v, want the one-shot error %q", r, wantErr)
	}
	if r := got[3]; r.OK || !strings.Contains(r.Err, `"watch" is not served`) {
		t.Errorf("watch reply = %+v, want a refusal", r)
	}
	var caps wm.Caps
	if r := got[4]; !r.OK || json.Unmarshal([]byte(r.Out), &caps) != nil || caps.Name != wm.ProviderWayfire {
		t.Errorf("caps reply = %+v, want the wayfire manifest", r)
	}
	if r := got[5]; !r.OK || r.Out != "on\n" {
		t.Errorf("second status reply = %+v, want its own on and nothing earlier", r)
	}
}
