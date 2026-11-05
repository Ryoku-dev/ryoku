package main

import "testing"

func TestBrowserRoleCommandUsesZenPackageExecutable(t *testing.T) {
	if got := browserRoleCommand("zen"); got != "zen-browser" {
		t.Fatalf("browserRoleCommand(zen) = %q, want zen-browser", got)
	}
	if got := browserRoleCommand("chromium"); got != "chromium" {
		t.Fatalf("browserRoleCommand(chromium) = %q, want chromium", got)
	}
}
