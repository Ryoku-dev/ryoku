package main

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func newTestKbMemory(t *testing.T, fail bool) (*kbLayoutMemory, chan struct{}) {
	t.Helper()
	cycles := make(chan struct{}, 8)
	m := newKbLayoutMemory(filepath.Join(t.TempDir(), "keyboard-layout.json"), func() error {
		cycles <- struct{}{}
		if fail {
			return errors.New("no switch")
		}
		return nil
	})
	return m, cycles
}

func waitCycles(t *testing.T, cycles chan struct{}, want int) {
	t.Helper()
	for i := range want {
		select {
		case <-cycles:
		case <-time.After(2 * time.Second):
			t.Fatalf("got %d cycles, want %d", i, want)
		}
	}
	select {
	case <-cycles:
		t.Fatalf("more than %d cycles", want)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestKbLayoutRestoredOnNewSession(t *testing.T) {
	list := []string{"English (US)", "Finnish"}
	m, cycles := newTestKbMemory(t, false)
	m.newStream()
	m.observe("English (US)", list)
	m.observe("Finnish", list) // the user picks Finnish in the bar
	waitCycles(t, cycles, 0)

	// next login: same file, xkb starts on the first layout again
	m = newKbLayoutMemory(m.path, m.cycle)
	m.newStream()
	m.observe("English (US)", list)
	waitCycles(t, cycles, 1)
	m.observe("Finnish", list)
	if m.restoring || m.saved.Layout != "Finnish" {
		t.Fatalf("restore did not settle on Finnish: restoring=%v saved=%q", m.restoring, m.saved.Layout)
	}

	// a later switch back is remembered, not undone
	m.observe("English (US)", list)
	waitCycles(t, cycles, 0)
	if m.saved.Layout != "English (US)" {
		t.Fatalf("saved = %q, want English (US)", m.saved.Layout)
	}
}

func TestKbLayoutRestoreWalksAThreeLayoutSet(t *testing.T) {
	list := []string{"us", "fi", "de"}
	m, cycles := newTestKbMemory(t, false)
	m.saved = kbLayoutState{Layout: "German", Layouts: list}
	m.newStream()
	m.observe("English (US)", list)
	waitCycles(t, cycles, 1)
	m.observe("Finnish", list)
	waitCycles(t, cycles, 1)
	m.observe("German", list)
	waitCycles(t, cycles, 0)
	if m.restoring || m.saved.Layout != "German" {
		t.Fatalf("restoring=%v saved=%q, want settled on German", m.restoring, m.saved.Layout)
	}
}

func TestKbLayoutChangedSetIsNotRestored(t *testing.T) {
	m, cycles := newTestKbMemory(t, false)
	m.saved = kbLayoutState{Layout: "Finnish", Layouts: []string{"us", "fi"}}
	m.newStream()
	m.observe("Finnish", []string{"fi", "us"})
	m.observe("English (US)", []string{"fi", "us"})
	waitCycles(t, cycles, 0)
	if m.saved.Layout != "English (US)" || !slices.Equal(m.saved.Layouts, []string{"fi", "us"}) {
		t.Fatalf("saved = %+v, want the new set and its active layout", m.saved)
	}
}

func TestKbLayoutFailedSwitchKeepsRemembering(t *testing.T) {
	list := []string{"us", "fi"}
	m, cycles := newTestKbMemory(t, true)
	m.saved = kbLayoutState{Layout: "Finnish", Layouts: list}
	m.newStream()
	m.observe("English (US)", list)
	waitCycles(t, cycles, 1)
	deadline := time.Now().Add(2 * time.Second)
	for {
		m.mu.Lock()
		restoring := m.restoring
		m.mu.Unlock()
		if !restoring {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("a failed switch left the restore armed")
		}
		time.Sleep(10 * time.Millisecond)
	}
	m.observe("Finnish", list)
	m.observe("English (US)", list)
	if m.saved.Layout != "English (US)" {
		t.Fatalf("saved = %q, want the switch made after the failed restore", m.saved.Layout)
	}
}
