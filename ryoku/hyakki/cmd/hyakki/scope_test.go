package main

import "testing"

// rashin scope takes a command after "--" and runs nothing without one or
// for one that is not there.
func TestScopeRefusesWithoutACommand(t *testing.T) {
	ran := false
	saved := scopeExec
	scopeExec = func(string, []string, []string) error { ran = true; return nil }
	t.Cleanup(func() { scopeExec = saved })
	for _, args := range [][]string{nil, {"--"}, {"sh"}} {
		if code := runScope(args); code != 2 {
			t.Errorf("rashin scope %q exits %d, want 2", args, code)
		}
	}
	if code := runScope([]string{"--", "rashin-no-such-command"}); code != 127 {
		t.Errorf("a missing command exits %d, want 127", code)
	}
	if ran {
		t.Fatal("something ran")
	}
}
