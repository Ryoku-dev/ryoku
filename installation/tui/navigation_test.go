package main

import "testing"

// esc must walk back out of the text steps (hostname, username). They set the
// `typing` flag, which used to skip the global esc->back and left them as
// one-way dead ends even though the footer advertised "esc back".
func TestEscGoesBackFromInputSteps(t *testing.T) {
	for _, key := range []string{"hostname", "username"} {
		m := model{state: "wizard", flow: steps(), picks: map[string]string{}}
		m.idx = flowIndex(m.flow, key)
		if m.idx <= 0 {
			t.Fatalf("step %q not reachable in flow", key)
		}
		m.loadStep()
		m.input = "typed"
		before := m.idx
		nm, _ := m.onKey("esc")
		got := nm.(model)
		if got.idx >= before {
			t.Fatalf("%s: esc did not go back (idx %d -> %d)", key, before, got.idx)
		}
		if !got.stepActive(got.idx) {
			t.Fatalf("%s: esc landed on an inactive step (idx %d)", key, got.idx)
		}
	}
}

// a printable key on a text step must still edit the field, not navigate.
func TestInputStepStillTypes(t *testing.T) {
	m := model{state: "wizard", flow: steps(), picks: map[string]string{}}
	m.idx = flowIndex(m.flow, "hostname")
	m.loadStep()
	before := m.idx
	nm, _ := m.onKey("a")
	got := nm.(model)
	if got.idx != before {
		t.Fatalf("typing moved the step (idx %d -> %d)", before, got.idx)
	}
	if got.input != "a" {
		t.Fatalf("typing 'a' did not edit the field (input %q)", got.input)
	}
}

// returning to a text step shows what was entered before, not a blank field.
func TestInputStepRestoresValueOnBack(t *testing.T) {
	m := model{state: "wizard", flow: steps(), picks: map[string]string{"hostname": "box"}}
	m.idx = flowIndex(m.flow, "hostname")
	m.loadStep()
	if m.input != "box" {
		t.Fatalf("loadStep did not restore saved value (input %q)", m.input)
	}
}
