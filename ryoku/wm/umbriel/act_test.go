package main

import (
	"encoding/json"
	"testing"
)

// act pins the exact dialect this Umbriel build accepts: {"cmd":"msg",
// "arg":"action:args"}, not the typed action request from newer docs. A wrong
// action name fails silently on a live compositor, so these strings matter.
func TestActMsgDialect(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{"focus", []string{"window.focus", "abc"}, "window-focus:abc"},
		{"close", []string{"window.close", "abc"}, "window-close:abc"},
		{"overview", []string{"overview.toggle"}, "overview-toggle"},
		{"output on", []string{"output.enable", "eDP-1", "on"}, "output-enable:eDP-1"},
		{"output off", []string{"output.enable", "eDP-1", "off"}, "output-disable:eDP-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			old := request
			request = func(req any) (json.RawMessage, error) {
				b, err := json.Marshal(req)
				if err != nil {
					t.Fatal(err)
				}
				var m map[string]any
				if err := json.Unmarshal(b, &m); err != nil {
					t.Fatal(err)
				}
				if m["cmd"] != "msg" {
					t.Fatalf("cmd = %v, want msg", m["cmd"])
				}
				got, _ = m["arg"].(string)
				return json.RawMessage(`null`), nil
			}
			defer func() { request = old }()

			if err := runAct(tc.argv); err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("arg = %q, want %q", got, tc.want)
			}
		})
	}
}
