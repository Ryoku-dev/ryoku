package main

import "testing"

// find the flow index of a step by key; the disk strategy step moves as the
// wizard evolves, so tests locate it instead of hardcoding an index.
func stepIdx(t *testing.T, m model, key string) int {
	t.Helper()
	for i, s := range m.flow {
		if s.key == key {
			return i
		}
	}
	t.Fatalf("no %q step in the flow", key)
	return -1
}

// The regression that erased-by-default: loadStep swaps the per-disk strategy
// list into the PICKER only, while the step keeps its static placeholder.
// The commit used to read the step's list, so selecting the displayed
// "alongside" stored "whole" and Review offered to erase the disk. The commit
// must come from the picker's own items - the list the user actually saw.
func TestSelectCommitsDisplayedItem(t *testing.T) {
	m := newModel()
	m.state, m.w, m.h, m.enterPos = "wizard", 112, 42, 1
	m.idx = stepIdx(t, m, "disk")

	// what loadStep does for a populated ryoku disk: the swapped, reordered
	// list lands in the picker; the step's static items stay behind.
	swapped := diskStrategiesFor(diskLayout{
		parts:        []part{{dev: "/dev/vda1"}, {dev: "/dev/vda2"}},
		gpt:          true,
		espKind:      "ryoku",
		probeVerdict: "ok",
	})
	if len(swapped) < 2 || swapped[0].key != "alongside" {
		t.Fatalf("precondition: alongside must lead the swapped list, got %+v", swapped)
	}
	m.pick = newPicker(swapped, true)

	// Enter on the highlighted first row = the displayed "alongside".
	mm, _ := m.onKey("enter")
	got := mm.(model).picks["disk"]
	if got != "alongside" {
		t.Fatalf("selected the displayed alongside but committed %q", got)
	}
}

func TestLoginShellNumberCommitsDisplayedChoice(t *testing.T) {
	m := newModel()
	m.state, m.w, m.h, m.enterPos = "wizard", 112, 42, 1
	m.idx = stepIdx(t, m, "login-shell")
	m.loadStep()

	next, _ := m.onKey("2")
	got := next.(model)
	if got.picks["login-shell"] != "zsh" {
		t.Fatalf("number 2 committed %q, want zsh", got.picks["login-shell"])
	}
	if got.cur().key != "apps" {
		t.Fatalf("login-shell commit advanced to %q, want apps", got.cur().key)
	}
}
