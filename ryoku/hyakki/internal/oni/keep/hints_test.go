package keep

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/spec"
)

func newHints(t *testing.T) *Hints {
	t.Helper()
	return NewHints(filepath.Join(t.TempDir(), "state", "hyakki", "oni-hints.json"))
}

// a morning, in the local zone, far from midnight
var hintMorning = time.Date(2026, 11, 22, 9, 0, 0, 0, time.Local)

func offer(t *testing.T, h *Hints, app string, keys []string, at time.Time) string {
	t.Helper()
	got, err := h.Offer(app, keys, at)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestHintNumbersComeFromTheSpec(t *testing.T) {
	s, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	if HintCooldown != time.Duration(s.Guide.Hints.CooldownS*float64(time.Second)) || HintOncePerDay != s.Guide.Hints.OncePerKeyPerDay {
		t.Fatalf("hint numbers %v %v, spec %+v", HintCooldown, HintOncePerDay, s.Guide.Hints)
	}
	if HintCooldown != 20*time.Minute || !HintOncePerDay {
		t.Fatalf("hint numbers %v %v, want 20m and once a day", HintCooldown, HintOncePerDay)
	}
}

// One hint every cooldown, each kept walkthrough once a day, the next one
// in line once the first was offered, and all of them again the next day.
func TestHintsKeepTheCooldownAndOncePerKeyPerDay(t *testing.T) {
	h := newHints(t)
	save, export := KeyFor(app, "save this file"), KeyFor(app, "export to pdf")
	keys := []string{save, export}
	if got := offer(t, h, app, keys, hintMorning); got != save {
		t.Fatalf("first hint %q, want %q", got, save)
	}
	if got := offer(t, h, app, keys, hintMorning.Add(HintCooldown-time.Second)); got != "" {
		t.Fatalf("a hint inside the cooldown: %q", got)
	}
	if got := offer(t, h, app, keys, hintMorning.Add(HintCooldown)); got != export {
		t.Fatalf("after the cooldown %q, want %q (save was hinted today)", got, export)
	}
	if got := offer(t, h, app, keys, hintMorning.Add(3*HintCooldown)); got != "" {
		t.Fatalf("every key was hinted today, yet %q", got)
	}
	// reopening reads the same record
	if got := offer(t, NewHints(h.Path()), app, keys, hintMorning.Add(4*HintCooldown)); got != "" {
		t.Fatalf("reopened, %q", got)
	}
	if got := offer(t, h, app, keys, hintMorning.Add(24*time.Hour)); got != save {
		t.Fatalf("the next day %q, want %q", got, save)
	}
}

// Not now snoozes the app for the rest of the day: none of its kept
// walkthroughs is offered, another app's still is, and the next day it is
// back.
func TestNotNowSnoozesTheAppForTheDay(t *testing.T) {
	h := newHints(t)
	save, other := KeyFor(app, "save this file"), KeyFor("org.example.Draw", "export to pdf")
	if got := offer(t, h, app, []string{save}, hintMorning); got != save {
		t.Fatalf("first hint %q", got)
	}
	if err := h.Snooze(" "+app+" ", hintMorning.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	later := hintMorning.Add(2 * HintCooldown)
	if got := offer(t, h, app, []string{KeyFor(app, "print it")}, later); got != "" {
		t.Fatalf("a snoozed app was hinted: %q", got)
	}
	if got := offer(t, h, "org.example.Draw", []string{other}, later); got != other {
		t.Fatalf("another app %q, want %q", got, other)
	}
	if got := offer(t, h, app, []string{save}, hintMorning.Add(24*time.Hour)); got != save {
		t.Fatalf("the next day %q, want %q", got, save)
	}
	if err := h.Snooze("", hintMorning); err == nil {
		t.Fatal("a snooze with no app was taken")
	}
}

// The record is private, holds only keys, apps and times, keeps only
// today's marks and never more than the keeps' bound of either, the newest.
func TestTheHintRecordIsPrivateAndBounded(t *testing.T) {
	h := newHints(t)
	if got := offer(t, h, app, []string{KeyFor(app, "yesterday's goal")}, hintMorning.Add(-24*time.Hour)); got == "" {
		t.Fatal("no first hint")
	}
	at := hintMorning
	for i := range MaxEntries + 5 {
		at = at.Add(time.Second)
		if err := h.Snooze(fmt.Sprintf("org.example.App%d", i), at); err != nil {
			t.Fatal(err)
		}
	}
	// more hints than a day's cooldowns allow, written as one record
	record, err := h.read()
	if err != nil {
		t.Fatal(err)
	}
	for i := range MaxEntries + 5 {
		record.Shown = append(record.Shown, shownMark{Key: KeyFor(app, fmt.Sprintf("goal number %d", i)), At: stampOf(at.Add(time.Duration(i) * time.Second))})
	}
	if err := h.write(record, at); err != nil {
		t.Fatal(err)
	}
	if got := offer(t, h, app, []string{KeyFor(app, "one more")}, at.Add(HintCooldown)); got == "" {
		t.Fatal("no hint after the cooldown")
	}
	info, err := os.Stat(h.Path())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("hints file mode %v, want 0600", info.Mode().Perm())
	}
	if dir, err := os.Stat(filepath.Dir(h.Path())); err != nil || dir.Mode().Perm() != 0o700 {
		t.Errorf("hints directory %v, %v", dir, err)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(h.Path()), ".oni-hints.json.*")); len(leftovers) != 0 {
		t.Errorf("temporary files left: %v", leftovers)
	}
	data, err := os.ReadFile(h.Path())
	if err != nil {
		t.Fatal(err)
	}
	var file map[string]json.RawMessage
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	if keys := sortedKeys(file); !slices.Equal(keys, []string{"last", "shown", "snoozed", "v"}) {
		t.Errorf("file fields %v", keys)
	}
	var shown, snoozed []map[string]json.RawMessage
	if err := json.Unmarshal(file["shown"], &shown); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(file["snoozed"], &snoozed); err != nil {
		t.Fatal(err)
	}
	if len(shown) != MaxEntries || len(snoozed) != MaxEntries {
		t.Fatalf("%d shown and %d snoozed marks, bound %d", len(shown), len(snoozed), MaxEntries)
	}
	if keys := sortedKeys(shown[0]); !slices.Equal(keys, []string{"at", "key"}) {
		t.Errorf("shown fields %v", keys)
	}
	if keys := sortedKeys(snoozed[0]); !slices.Equal(keys, []string{"app", "at"}) {
		t.Errorf("snoozed fields %v", keys)
	}
	if bytes.Contains(data, []byte("yesterday")) || bytes.Contains(data, []byte("goal-number-0\"")) {
		t.Error("the record kept a mark it should have dropped")
	}
}

// A hints file that cannot be read is left byte for byte as it is, and no
// hint is due while it stays so.
func TestADamagedHintRecordIsNeverOverwritten(t *testing.T) {
	key := KeyFor(app, "save this file")
	bodies := map[string][]byte{
		"not json":            []byte(`{"v":1,"shown":[`),
		"another version":     []byte(`{"v":2,"shown":[],"snoozed":[]}`),
		"an unknown field":    []byte(`{"v":1,"shown":[],"snoozed":[],"title":"Quarterly"}`),
		"a bad key":           []byte(`{"v":1,"shown":[{"key":"no-app","at":"2026-11-22T09:00:00Z"}],"snoozed":[]}`),
		"a twice-kept key":    []byte(`{"v":1,"shown":[{"key":"` + key + `","at":"2026-11-22T09:00:00Z"},{"key":"` + key + `","at":"2026-11-22T09:00:01Z"}],"snoozed":[]}`),
		"a mark with no time": []byte(`{"v":1,"shown":[],"snoozed":[{"app":"x"}]}`),
		"trailing data":       []byte(`{"v":1,"shown":[],"snoozed":[]} {}`),
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			h := newHints(t)
			if err := os.MkdirAll(filepath.Dir(h.Path()), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(h.Path(), body, 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := h.Offer(app, []string{key}, hintMorning)
			if got != "" || reason(err) != ReasonHintsDamaged {
				t.Errorf("Offer = %q, %v", got, err)
			}
			if err := h.Snooze(app, hintMorning); reason(err) != ReasonHintsDamaged {
				t.Errorf("Snooze = %v", err)
			}
			after, err := os.ReadFile(h.Path())
			if err != nil || !bytes.Equal(after, body) {
				t.Fatalf("the damaged file changed: %q, %v", after, err)
			}
		})
	}
}

func TestDefaultHintsPathIsBesideTheKeeps(t *testing.T) {
	if filepath.Dir(DefaultHintsPath()) != filepath.Dir(DefaultStorePath()) || filepath.Base(DefaultHintsPath()) != "oni-hints.json" {
		t.Fatalf("hints at %s, keeps at %s", DefaultHintsPath(), DefaultStorePath())
	}
}
