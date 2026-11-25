package keep

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/screenask"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

const app = "org.example.Notes"

// clock is a store clock that moves only when told.
type clock struct{ at time.Time }

func (c *clock) now() time.Time { return c.at }
func (c *clock) tick()          { c.at = c.at.Add(time.Second) }

func open(t *testing.T, path string, c *clock) *Store {
	t.Helper()
	s := NewStore(path)
	s.now = c.now
	return s
}

func newStore(t *testing.T) (*Store, *clock) {
	t.Helper()
	c := &clock{at: time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)}
	return open(t, filepath.Join(t.TempDir(), "state", "hyakki", "oni-keeps.json"), c), c
}

func step(label string, box feature.Box) Step {
	return Step{Text: "Click " + label + ".", Label: label, Role: "button", Box: box, Mode: feature.StrandModeLasso, Trusted: true}
}

func entry(goal string, steps ...Step) Entry {
	if len(steps) == 0 {
		steps = []Step{step("File", feature.Box{X: 0.02, Y: 0.02, W: 0.05, H: 0.03}), step("Save", feature.Box{X: 0.1, Y: 0.1, W: 0.06, H: 0.04})}
	}
	return Entry{App: app, Goal: goal, Steps: steps}
}

func reason(err error) string {
	var perr *protocol.Error
	if errors.As(err, &perr) {
		return perr.Reason
	}
	return ""
}

func code(err error) string {
	var perr *protocol.Error
	if errors.As(err, &perr) {
		return perr.Code
	}
	return ""
}

func TestNumbersComeFromTheSpec(t *testing.T) {
	sp, err := spec.Load()
	if err != nil {
		t.Fatal(err)
	}
	k := sp.Guide.Keep
	if MaxEntries != k.MaxEntries || MatchMin != k.MatchMin || FindMin != k.FindMin || MissesToStale != k.MissesToStale || MaxSteps != sp.Guide.MaxSteps {
		t.Fatalf("keep numbers %d %g %d %d %d differ from the spec's %+v, %d steps", MaxEntries, MatchMin, FindMin, MissesToStale, MaxSteps, k, sp.Guide.MaxSteps)
	}
	if MaxEntries != 80 || MatchMin != 0.75 || FindMin != 80 || MissesToStale != 2 {
		t.Fatalf("keep numbers %d %g %d %d, want 80 0.75 80 2", MaxEntries, MatchMin, FindMin, MissesToStale)
	}
}

func TestTheLeastRecentlyUsedEntryGoesPastTheBound(t *testing.T) {
	s, c := newStore(t)
	goal := func(i int) string { return fmt.Sprintf("open panel %d", i) }
	for i := range MaxEntries {
		if err := s.Put(entry(goal(i))); err != nil {
			t.Fatalf("Put %d: %v", i, err)
		}
		c.tick()
	}
	// The first kept is used again, so the second is now the oldest used.
	if err := s.Hit(KeyFor(app, goal(0))); err != nil {
		t.Fatal(err)
	}
	c.tick()
	if err := s.Put(entry(goal(MaxEntries))); err != nil {
		t.Fatal(err)
	}
	reopened := open(t, s.Path(), c)
	list, err := reopened.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != MaxEntries {
		t.Fatalf("%d entries kept, want %d", len(list), MaxEntries)
	}
	keys := make([]string, len(list))
	for i, e := range list {
		keys[i] = e.Key
	}
	if slices.Contains(keys, KeyFor(app, goal(1))) {
		t.Errorf("the oldest-used entry %q survived", goal(1))
	}
	for _, i := range []int{0, 2, MaxEntries} {
		if !slices.Contains(keys, KeyFor(app, goal(i))) {
			t.Errorf("entry %q went, but it was not the oldest used", goal(i))
		}
	}
	if list[0].Key != KeyFor(app, goal(MaxEntries)) || list[1].Key != KeyFor(app, goal(0)) {
		t.Errorf("List order %q, %q; want the newest used first", list[0].Key, list[1].Key)
	}
}

func TestAGoalMatchesItsVariantsAndNothingBelowTheLine(t *testing.T) {
	s, _ := newStore(t)
	if err := s.Put(entry("How can I save this file?")); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(entry("export the file as pdf")); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		app, goal string
		want      string // the goal matched, or "" for none
	}{
		{app, "Save this file", "How can I save this file?"},
		{app, "save the files", "How can I save this file?"},
		{app, "how do I save my file", "How can I save this file?"},
		{"ORG.EXAMPLE.NOTES", "SAVE FILE!", "How can I save this file?"},
		{app, "export file as pdf quickly", "export the file as pdf"},
		{app, "save this file as pdf", ""},
		{app, "save as", ""},
		{app, "file save", ""},
		{app, "pdf as file export", ""},
		{app, "print this file", ""},
		{"org.example.Other", "save this file", ""},
		{app, "how do I", ""},
	}
	for _, c := range cases {
		e, score, ok := s.Match(c.app, c.goal)
		if c.want == "" {
			if ok {
				t.Errorf("Match(%q, %q) = %q at %.2f, want no match", c.app, c.goal, e.Goal, score)
			} else if score >= MatchMin {
				t.Errorf("Match(%q, %q) refused at %.2f, at or above the line", c.app, c.goal, score)
			}
			continue
		}
		if !ok || e.Goal != c.want || score < MatchMin {
			t.Errorf("Match(%q, %q) = %q at %.2f (%v), want %q", c.app, c.goal, e.Goal, score, ok, c.want)
		}
	}
	if _, score, ok := s.Match(app, "export file as pdf quickly"); !ok || score >= 1 {
		t.Errorf("a near variant scored %.2f (%v); want a match below 1", score, ok)
	}
}

func TestScoreCountsWholeWordsOnly(t *testing.T) {
	cases := []struct {
		saved, found string
		want         func(int) bool
		says         string
	}{
		{"Save", "Save", func(n int) bool { return n == 100 }, "100"},
		{"Save", "SAVE", func(n int) bool { return n == 100 }, "100"},
		{"Save As…", "Save As...", func(n int) bool { return n == 100 }, "100"},
		{"Save", "Save As", func(n int) bool { return n < FindMin }, "below FindMin"},
		{"Save As", "Save", func(n int) bool { return n < FindMin }, "below FindMin"},
		{"Save", "Autosave", func(n int) bool { return n == 0 }, "0"},
		{"Save", "Saved", func(n int) bool { return n == 0 }, "0"},
		{"Save", "Sav", func(n int) bool { return n == 0 }, "0"},
		{"Export", "", func(n int) bool { return n == 0 }, "0"},
	}
	for _, c := range cases {
		if got := Score(c.saved, c.found); !c.want(got) {
			t.Errorf("Score(%q, %q) = %d, want %s", c.saved, c.found, got, c.says)
		}
	}
}

// row lays words out left to right at y, each word's box sized by its runes
// at 0.01 per character, separated by the gaps given (one fewer than words).
func row(y float64, x float64, text []string, gaps []float64) []screenask.Word {
	var out []screenask.Word
	for i, w := range text {
		width := 0.01 * float64(len([]rune(w)))
		out = append(out, screenask.Word{Text: w, Box: feature.Box{X: x, Y: y, W: width, H: 0.02}})
		x += width
		if i < len(gaps) {
			x += gaps[i]
		}
	}
	return out
}

func TestFindTellsSaveFromSaveAs(t *testing.T) {
	save := step("Save", feature.Box{X: 0.1, Y: 0.1, W: 0.06, H: 0.04})
	// Two buttons, "Save As…" and "Export", a button's padding apart: no
	// plain Save on the screen.
	onlySaveAs := row(0.1, 0.1, []string{"Save", "As…", "Export"}, []float64{0.006, 0.05})
	if box, score, ok := Find(save, onlySaveAs); ok || score >= FindMin {
		t.Fatalf("Find(Save) on a screen with only Save As = %+v at %d (%v); want a miss", box, score, ok)
	}
	autosave := row(0.1, 0.1, []string{"Autosave"}, nil)
	if _, score, ok := Find(save, autosave); ok || score != 0 {
		t.Fatalf("Find(Save) matched Autosave at %d (%v)", score, ok)
	}
	// Both buttons: the plain one is found, not the Save inside Save As.
	both := row(0.1, 0.3, []string{"Save", "As…", "Save"}, []float64{0.006, 0.05})
	box, score, ok := Find(save, both)
	if !ok || score != 100 {
		t.Fatalf("Find(Save) with both buttons = %d (%v), want 100", score, ok)
	}
	if want := both[2].Box; box != want {
		t.Errorf("found %+v, want the plain Save at %+v", box, want)
	}
	saveAs := step("Save As…", feature.Box{X: 0.3, Y: 0.1, W: 0.08, H: 0.04})
	if box, score, ok := Find(saveAs, both); !ok || score != 100 || box.X != 0.3 || box.W < 0.07 {
		t.Errorf("Find(Save As) = %+v at %d (%v), want the two-word label", box, score, ok)
	}
}

func TestFindPrefersTheControlNearestWhereItWasKept(t *testing.T) {
	kept := step("Open", feature.Box{X: 0.7, Y: 0.7, W: 0.05, H: 0.03})
	ws := append(row(0.1, 0.1, []string{"Open"}, nil), row(0.7, 0.7, []string{"Open"}, nil)...)
	box, _, found := Find(kept, ws)
	if !found || box.Y != 0.7 {
		t.Fatalf("found %+v (%v), want the Open near the kept box", box, found)
	}
}

func TestFindRefusesAControlThatBecameAPrivateField(t *testing.T) {
	cont := step("Continue", feature.Box{X: 0.5, Y: 0.5, W: 0.08, H: 0.03})
	plain := row(0.5, 0.5, []string{"Continue"}, nil)
	if _, _, ok := Find(cont, plain); !ok {
		t.Fatal("a plain Continue was not found")
	}
	// The same label, now under a caption naming a private field.
	caption := append(row(0.47, 0.5, []string{"Card", "number"}, []float64{0.006}), plain...)
	f, ok := Locate(cont, caption)
	if ok || f.Refused == "" {
		t.Fatalf("Locate beside a card number caption = %+v (%v), want refused", f, ok)
	}
	if box, _, ok := Find(cont, caption); ok || box != (feature.Box{}) {
		t.Fatalf("Find beside a card number caption = %+v (%v), want refused with no box", box, ok)
	}
	// A kept label that now reads as a private field.
	pin := row(0.5, 0.5, []string{"Continue", "PIN"}, []float64{0.006})
	if _, _, ok := Find(step("Continue PIN", cont.Box), pin); ok {
		t.Fatal("a control labelled with a PIN was found")
	}
	// The kept control's place relabelled as a private field: no kept label
	// anywhere, yet what sits where it was names a PIN, so it is refused,
	// not merely missed.
	save := step("Save", feature.Box{X: 0.05, Y: 0.2, W: 0.3, H: 0.1})
	relabelled := append(row(0.24, 0.12, []string{"PIN"}, nil), row(0.36, 0.12, []string{"Export"}, nil)...)
	if f, ok := Locate(save, relabelled); ok || f.Refused == "" || f.Box != save.Box {
		t.Fatalf("Locate(Save) over a control relabelled PIN = %+v (%v), want refused at the kept place", f, ok)
	}
	// relabelled as anything else, it is only a miss
	other := append(row(0.24, 0.12, []string{"Print"}, nil), row(0.36, 0.12, []string{"Export"}, nil)...)
	if f, ok := Locate(save, other); ok || f.Refused != "" {
		t.Fatalf("Locate(Save) over a control relabelled Print = %+v (%v), want a plain miss", f, ok)
	}
}

func TestTheSecondMissRetiresTheEntryForGood(t *testing.T) {
	s, c := newStore(t)
	key := KeyFor(app, "save this file")
	if err := s.Put(entry("save this file")); err != nil {
		t.Fatal(err)
	}
	stale, err := s.Miss(key)
	if err != nil || stale {
		t.Fatalf("first Miss = %v, %v; want kept", stale, err)
	}
	reopened := open(t, s.Path(), c)
	e, _, ok := reopened.Match(app, "save this file")
	if !ok || e.Misses != 1 {
		t.Fatalf("after one miss and a reopen: %+v (%v), want one miss kept", e, ok)
	}
	stale, err = reopened.Miss(key)
	if err != nil || !stale {
		t.Fatalf("second Miss = %v, %v; want retired", stale, err)
	}
	again := open(t, s.Path(), c)
	if _, _, ok := again.Match(app, "save this file"); ok {
		t.Fatal("a retired entry still matches after a reopen")
	}
	if list, err := again.List(); err != nil || len(list) != 0 {
		t.Fatalf("List after retiring = %v, %v", list, err)
	}
	if _, err := again.Miss(key); code(err) != protocol.CodeNotFound {
		t.Fatalf("Miss on a retired key = %v, want not_found", err)
	}
}

func TestAHitClearsTheMisses(t *testing.T) {
	s, _ := newStore(t)
	key := KeyFor(app, "save this file")
	if err := s.Put(entry("save this file")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Miss(key); err != nil {
		t.Fatal(err)
	}
	if err := s.Hit(key); err != nil {
		t.Fatal(err)
	}
	if stale, err := s.Miss(key); err != nil || stale {
		t.Fatalf("a miss after a hit = %v, %v; want kept", stale, err)
	}
}

func TestPutRefusesPrivateFieldsAndUncheckedSteps(t *testing.T) {
	box := feature.Box{X: 0.2, Y: 0.2, W: 0.1, H: 0.04}
	unchecked := step("Save", box)
	unchecked.Trusted = false
	role := step("Field", box)
	role.Role = "password"
	cases := map[string]Step{
		"a PIN label":          step("Account PIN", box),
		"a password label":     step("New password", box),
		"a card number label":  step("Card number", box),
		"a password role":      role,
		"an unchecked step":    unchecked,
		"a box off the window": step("Save", feature.Box{X: 0.95, Y: 0.2, W: 0.1, H: 0.04}),
		"no label":             step("…", box),
	}
	for name, bad := range cases {
		t.Run(name, func(t *testing.T) {
			s, _ := newStore(t)
			err := s.Put(entry("sign in", step("Account", box), bad))
			if code(err) != protocol.CodeBadRequest {
				t.Fatalf("Put = %v, want bad_request", err)
			}
			if _, err := os.Stat(s.Path()); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("a refused Put left a file: %v", err)
			}
		})
	}
	s, _ := newStore(t)
	err := s.Put(entry("sign in", step("Account PIN", box)))
	var perr *protocol.Error
	if !errors.As(err, &perr) || perr.Message != screenask.RefusalLine {
		t.Fatalf("a private field is refused with %v, want the refusal line", err)
	}
}

func TestPutReplacesTheSameGoal(t *testing.T) {
	s, c := newStore(t)
	if err := s.Put(entry("save this file")); err != nil {
		t.Fatal(err)
	}
	c.tick()
	if err := s.Put(entry("Save the files", step("Save", feature.Box{X: 0.4, Y: 0.4, W: 0.05, H: 0.03}))); err != nil {
		t.Fatal(err)
	}
	list, err := s.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("List = %v, %v; want one entry", list, err)
	}
	if len(list[0].Steps) != 1 || !list[0].Kept.Equal(c.at) {
		t.Fatalf("the second keep did not replace the first: %+v", list[0])
	}
	if !list[0].Steps[0].Trusted {
		t.Fatal("a step read back is not Trusted")
	}
}

func TestMatchReturnsACopy(t *testing.T) {
	s, _ := newStore(t)
	region := feature.Box{X: 0.1, Y: 0.1, W: 0.5, H: 0.5}
	st := step("Sidebar", feature.Box{X: 0.1, Y: 0.1, W: 0.1, H: 0.1})
	st.Mode, st.Region = feature.StrandModeRegion, &region
	if err := s.Put(entry("open sidebar", st)); err != nil {
		t.Fatal(err)
	}
	e, _, _ := s.Match(app, "open sidebar")
	e.Steps[0].Label = "changed"
	e.Steps[0].Region.W = 0.9
	again, _, _ := s.Match(app, "open sidebar")
	if again.Steps[0].Label != "Sidebar" || again.Steps[0].Region.W != 0.5 {
		t.Fatalf("changing a matched entry changed the store: %+v", again.Steps[0])
	}
}

func TestTheFileIsPrivateAndHoldsOnlyTheSteps(t *testing.T) {
	s, _ := newStore(t)
	// What the window showed: its title, a field with what the user typed,
	// and the File and Save controls. Only the controls' labels are kept.
	title := "Quarterly-forecast-draft.odt"
	typed := "hunter2 meet me at the docks"
	ws := append(row(0.01, 0.3, []string{title}, nil), row(0.5, 0.2, strings.Fields(typed), []float64{0.006, 0.006, 0.006, 0.006, 0.006})...)
	ws = append(ws, row(0.05, 0.02, []string{"File"}, nil)...)
	ws = append(ws, row(0.1, 0.1, []string{"Save"}, nil)...)
	var steps []Step
	for _, label := range []string{"File", "Save"} {
		f, ok := Locate(step(label, feature.Box{X: 0.02, Y: 0.05, W: 0.04, H: 0.02}), ws)
		if !ok {
			t.Fatalf("%s not found", label)
		}
		steps = append(steps, step(f.Text, f.Box))
	}
	if err := s.Put(entry("save this file", steps...)); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("keeps file mode %v, want 0600", info.Mode().Perm())
	}
	dir, err := os.Stat(filepath.Dir(s.Path()))
	if err != nil {
		t.Fatal(err)
	}
	if dir.Mode().Perm() != 0o700 {
		t.Errorf("keeps directory mode %v, want 0700", dir.Mode().Perm())
	}
	if leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(s.Path()), ".oni-keeps.json.*")); len(leftovers) != 0 {
		t.Errorf("temporary files left: %v", leftovers)
	}

	data, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	lower := bytes.ToLower(data)
	for _, word := range append([]string{"quarterly", "forecast", ".odt"}, strings.Fields(typed)...) {
		if len(word) > 2 && bytes.Contains(lower, []byte(strings.ToLower(word))) && word != "the" {
			t.Errorf("the keeps file holds %q from the window", word)
		}
	}
	for _, magic := range []string{"\x89PNG", "\xff\xd8\xff", "iVBORw0KGgo", "/9j/", "data:image"} {
		if bytes.Contains(data, []byte(magic)) {
			t.Errorf("the keeps file holds picture bytes %q", magic)
		}
	}
	if run := regexp.MustCompile(`[A-Za-z0-9+/=]{64,}`).Find(data); run != nil {
		t.Errorf("the keeps file holds a long encoded run %.20q…", run)
	}

	// Structurally: no field beyond the goal and the steps.
	var file map[string]json.RawMessage
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	if keys := sortedKeys(file); !slices.Equal(keys, []string{"entries", "v"}) {
		t.Errorf("file fields %v", keys)
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(file["entries"], &entries); err != nil || len(entries) != 1 {
		t.Fatalf("entries %v, %v", entries, err)
	}
	if keys := sortedKeys(entries[0]); !slices.Equal(keys, []string{"app", "goal", "kept", "key", "steps", "used"}) {
		t.Errorf("entry fields %v", keys)
	}
	var stored []map[string]json.RawMessage
	if err := json.Unmarshal(entries[0]["steps"], &stored); err != nil {
		t.Fatal(err)
	}
	for _, st := range stored {
		for _, k := range sortedKeys(st) {
			if !slices.Contains([]string{"box", "irreversible", "label", "mode", "region", "role", "text"}, k) {
				t.Errorf("step field %q", k)
			}
		}
	}
}

func sortedKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func TestADamagedFileIsNeverOverwritten(t *testing.T) {
	good, _ := json.Marshal(storeFile{V: storeVersion, Entries: []Entry{}})
	bodies := map[string][]byte{
		"not json":         []byte("{\"v\":1,\"entries\":["),
		"another version":  []byte(`{"v":2,"entries":[]}`),
		"an unknown field": []byte(`{"v":1,"entries":[],"title":"Quarterly"}`),
		"a wrong key":      []byte(`{"v":1,"entries":[{"key":"x::y","app":"x","goal":"save file","steps":[{"text":"Click Save.","label":"Save","box":{"x":0.1,"y":0.1,"w":0.1,"h":0.1},"mode":"point"}],"kept":"2026-11-22T09:00:00Z","used":"2026-11-22T09:00:00Z"}]}`),
		"a private step":   []byte(`{"v":1,"entries":[{"key":"x::save-file","app":"x","goal":"save file","steps":[{"text":"Type here.","label":"Password","box":{"x":0.1,"y":0.1,"w":0.1,"h":0.1},"mode":"point"}],"kept":"2026-11-22T09:00:00Z","used":"2026-11-22T09:00:00Z"}]}`),
		"trailing data":    append(append([]byte{}, good...), []byte(" {}")...),
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			s, _ := newStore(t)
			if err := os.MkdirAll(filepath.Dir(s.Path()), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(s.Path(), body, 0o600); err != nil {
				t.Fatal(err)
			}
			key := KeyFor(app, "save this file")
			calls := map[string]error{
				"Put":       s.Put(entry("save this file")),
				"Hit":       s.Hit(key),
				"Forget":    s.Forget(key),
				"ForgetAll": s.ForgetAll(),
			}
			_, calls["Miss"] = s.Miss(key)
			_, calls["List"] = s.List()
			for call, err := range calls {
				if reason(err) != ReasonDamaged {
					t.Errorf("%s = %v, want %s", call, err, ReasonDamaged)
				}
			}
			if _, _, ok := s.Match("x", "save file"); ok {
				t.Error("a damaged store matched")
			}
			after, err := os.ReadFile(s.Path())
			if err != nil || !bytes.Equal(after, body) {
				t.Fatalf("the damaged file changed: %q, %v", after, err)
			}
		})
	}
}

func TestResetMovesTheFileAside(t *testing.T) {
	s, _ := newStore(t)
	if err := s.Reset(); err != nil {
		t.Fatalf("Reset with no file = %v", err)
	}
	body := []byte("not a keeps file")
	if err := os.MkdirAll(filepath.Dir(s.Path()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.Path(), body, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Reset(); err != nil {
		t.Fatal(err)
	}
	aside, err := os.ReadFile(s.DamagedPath())
	if err != nil || !bytes.Equal(aside, body) {
		t.Fatalf("aside = %q, %v; want the old file", aside, err)
	}
	if list, err := s.List(); err != nil || len(list) != 0 {
		t.Fatalf("List after Reset = %v, %v; want empty", list, err)
	}
	if err := s.Put(entry("save this file")); err != nil {
		t.Fatal(err)
	}
	if aside, _ := os.ReadFile(s.DamagedPath()); !bytes.Equal(aside, body) {
		t.Fatal("a Put after Reset touched the file set aside")
	}
}

func TestForgetDropsEntries(t *testing.T) {
	s, _ := newStore(t)
	for _, g := range []string{"save this file", "print this file"} {
		if err := s.Put(entry(g)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Forget(KeyFor(app, "save this file")); err != nil {
		t.Fatal(err)
	}
	if err := s.Forget(KeyFor(app, "save this file")); code(err) != protocol.CodeNotFound {
		t.Fatalf("Forget twice = %v, want not_found", err)
	}
	if list, _ := s.List(); len(list) != 1 || list[0].Key != KeyFor(app, "print this file") {
		t.Fatalf("List after Forget = %+v", list)
	}
	if err := s.ForgetAll(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ForgetAll left the file: %v", err)
	}
	if err := s.ForgetAll(); err != nil {
		t.Fatalf("ForgetAll with no file = %v", err)
	}
}

func TestDefaultStorePathIsInTheStateDirectory(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/hyakki-keep-test-state")
	if got, want := NewStore("").Path(), "/tmp/hyakki-keep-test-state/hyakki/oni-keeps.json"; got != want {
		t.Fatalf("default path %q, want %q", got, want)
	}
}
