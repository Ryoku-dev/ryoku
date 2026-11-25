package main

import (
	"bytes"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestLocalSubcommandsKeepModelChangesInApp(t *testing.T) {
	for _, tc := range []struct {
		args   []string
		action string
		id     string
	}{
		{nil, "list", ""},
		{[]string{"list"}, "list", ""},
		{[]string{"pull", "lfm3-omni-2b"}, "pull", "lfm3-omni-2b"},
		{[]string{"rm", "lfm3-omni-2b"}, "rm", "lfm3-omni-2b"},
		{[]string{"runtime", "install"}, "runtime", ""},
	} {
		action, id, ok := localAction(tc.args)
		if !ok || action != tc.action || id != tc.id {
			t.Fatalf("%v: %q, %q, %t", tc.args, action, id, ok)
		}
	}
	for _, args := range [][]string{{"list", "extra"}, {"pull"}, {"pull", ""}, {"rm"}, {"runtime", "remove"}, {"runtime", "install", "extra"}} {
		if _, _, ok := localAction(args); ok {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestLocalListShowsInstalledModelsAndRuntimeWithoutStartingIt(t *testing.T) {
	var out bytes.Buffer
	printLocal(&out, protocol.LocalResult{
		Models:  []protocol.LocalModel{{ID: "lfm3-omni-2b", State: protocol.LocalInstalled, Files: []protocol.LocalFile{{Size: 100}, {Size: 25}}}},
		Runtime: protocol.LocalRuntime{State: protocol.RuntimeStopped, Version: "b12521"},
	})
	if got := out.String(); !strings.Contains(got, "lfm3-omni-2b\tinstalled\t125\n") || !strings.Contains(got, "runtime\tstopped\tb12521\n") {
		t.Fatalf("local list = %q", got)
	}
}
