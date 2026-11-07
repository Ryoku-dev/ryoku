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
// niri, output kept to the request that printed it, the act error text
// unchanged, and verbs it does not serve refused rather than run.
func TestServeRunsActAndCapsInOrder(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var sent []string
	restore := stubRequest(t, func(req any) (json.RawMessage, error) {
		b, _ := json.Marshal(req)
		sent = append(sent, string(b))
		return json.RawMessage(`"Handled"`), nil
	})
	defer restore()

	wantErr := runAct([]string{"window.focus"}).Error()
	sent = nil
	replies := serveLines(t,
		`{"id":1,"argv":["act","input.touchpad","status"]}`,
		`{"id":2,"argv":["act","workspace.focus","9"]}`,
		`{"id":3,"argv":["act","window.focus"]}`,
		`{"id":4,"argv":["watch"]}`,
		`{"id":5,"argv":["caps"]}`,
		`{"id":6,"argv":["act","input.touchpad","status"]}`,
	)
	if len(replies) != 6 {
		t.Fatalf("got %d replies, want 6: %+v", len(replies), replies)
	}
	for i, r := range replies {
		if r.ID != uint64(i+1) {
			t.Fatalf("reply %d has id %d: answers must keep request order", i, r.ID)
		}
	}
	if r := replies[0]; !r.OK || r.Out != "on\n" {
		t.Errorf("status reply = %+v, want ok with on", r)
	}
	if r := replies[1]; !r.OK || r.Out != "" {
		t.Errorf("workspace.focus reply = %+v, want ok and silent", r)
	}
	focus := `{"Action":{"FocusWorkspace":{"reference":{"Id":9}}}}`
	found := false
	for _, s := range sent {
		found = found || s == focus
	}
	if !found {
		t.Errorf("niri requests = %q, want %s among them", sent, focus)
	}
	if r := replies[2]; r.OK || r.Err != wantErr {
		t.Errorf("window.focus reply = %+v, want the one-shot error %q", r, wantErr)
	}
	if r := replies[3]; r.OK || !strings.Contains(r.Err, `"watch" is not served`) {
		t.Errorf("watch reply = %+v, want a refusal", r)
	}
	var caps wm.Caps
	if r := replies[4]; !r.OK || json.Unmarshal([]byte(r.Out), &caps) != nil || caps.Name != wm.ProviderNiri {
		t.Errorf("caps reply = %+v, want the niri manifest", r)
	}
	if r := replies[5]; !r.OK || r.Out != "on\n" {
		t.Errorf("second status reply = %+v, want its own on and nothing earlier", r)
	}
}
