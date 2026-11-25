package routine

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/reflex"
	"ryoku-rashin/internal/protocol"
)

var t0 = time.Date(2026, 11, 23, 9, 0, 0, 0, time.UTC)

// intents picks registry ids by what the registry says of them, so the tests
// follow the registry rather than a copy of it.
func intents(t *testing.T, keep func(reflex.Info) bool) []reflex.Intent {
	t.Helper()
	var out []reflex.Intent
	for _, info := range reflex.Registry() {
		if keep(info) {
			out = append(out, info.ID)
		}
	}
	if len(out) == 0 {
		t.Fatal("the registry has no such intent")
	}
	return out
}

func replayable(t *testing.T) []reflex.Intent {
	return intents(t, func(i reflex.Info) bool { return i.Replayable && !i.PressOnly })
}

func notReplayable(t *testing.T) []reflex.Intent {
	return intents(t, func(i reflex.Info) bool { return !i.Replayable || i.PressOnly })
}

func open(app string) reflex.Action {
	return reflex.Action{Intent: reflex.OpenApp, Arg: app, Confidence: 0.97}
}

func TestOfferAfterExactlyThreeStepsWithinGap(t *testing.T) {
	r := NewRecorder(OfferMinSteps, Gap)
	acts := []reflex.Action{open("Files"), open("Editor"), open("Music")}
	at := []time.Time{t0, t0.Add(Gap), t0.Add(2*Gap - time.Second)}
	for i := range OfferMinSteps - 1 {
		if offer, ok := r.Observe(acts[i], at[i]); ok || offer != nil {
			t.Fatalf("offered after %d steps: %v", i+1, offer)
		}
	}
	offer, ok := r.Observe(acts[2], at[2])
	if !ok {
		t.Fatal("no offer after three steps within the gap")
	}
	want := []Step{{reflex.OpenApp, "Files"}, {reflex.OpenApp, "Editor"}, {reflex.OpenApp, "Music"}}
	if !slices.Equal(offer, want) {
		t.Fatalf("offer = %v, want %v", offer, want)
	}
	if !r.Offered() {
		t.Fatal("Offered is false after the offer")
	}
}

func TestNoOfferWhenAGapIsTooLong(t *testing.T) {
	r := NewRecorder(OfferMinSteps, Gap)
	r.Observe(open("Files"), t0)
	r.Observe(open("Editor"), t0.Add(time.Second))
	if _, ok := r.Observe(open("Music"), t0.Add(time.Second+Gap+time.Millisecond)); ok {
		t.Fatal("offered across a gap longer than the spec's")
	}
	// The long pause started a new run: two more close steps complete it.
	r.Observe(open("Mail"), t0.Add(2*Gap))
	offer, ok := r.Observe(open("Notes"), t0.Add(2*Gap+time.Second))
	if !ok || len(offer) != OfferMinSteps || offer[0].Arg != "Music" {
		t.Fatalf("offer = %v %v, want the three steps after the pause", offer, ok)
	}
}

func TestClockGoingBackStartsAgain(t *testing.T) {
	r := NewRecorder(OfferMinSteps, Gap)
	r.Observe(open("Files"), t0)
	r.Observe(open("Editor"), t0.Add(time.Second))
	if _, ok := r.Observe(open("Music"), t0.Add(-time.Second)); ok {
		t.Fatal("offered across a clock that went back")
	}
}

func TestOfferedOncePerRun(t *testing.T) {
	r := NewRecorder(OfferMinSteps, Gap)
	at := t0
	offers := 0
	for i := range 4 * OfferMinSteps {
		if _, ok := r.Observe(open("App"+string(rune('A'+i))), at); ok {
			offers++
		}
		at = at.Add(time.Second)
	}
	if offers != 1 {
		t.Fatalf("offers = %d, want 1", offers)
	}
	// A fresh recorder is a fresh run.
	r = NewRecorder(OfferMinSteps, Gap)
	for i := range OfferMinSteps {
		if _, ok := r.Observe(open("App"), t0.Add(time.Duration(i)*time.Second)); ok != (i == OfferMinSteps-1) {
			t.Fatalf("step %d: ok = %v", i, ok)
		}
	}
}

func TestNonReplayableStepIsNotRecorded(t *testing.T) {
	r := NewRecorder(OfferMinSteps, Gap)
	at := t0
	step := func(a reflex.Action) ([]Step, bool) {
		at = at.Add(time.Second)
		return r.Observe(a, at)
	}
	step(open("Files"))
	for _, id := range notReplayable(t) {
		if offer, ok := step(reflex.Action{Intent: id, Arg: "x", Confidence: 1}); ok {
			t.Fatalf("%s completed an offer: %v", id, offer)
		}
	}
	step(open("Editor"))
	offer, ok := step(open("Music"))
	if !ok {
		t.Fatal("no offer after three replayable steps around non-replayable ones")
	}
	for _, st := range offer {
		if !Replayable(st.Intent) {
			t.Fatalf("offer recorded %s", st.Intent)
		}
	}
	if len(offer) != OfferMinSteps {
		t.Fatalf("offer has %d steps, want %d", len(offer), OfferMinSteps)
	}
	if Replayable(reflex.Intent("send_message")) {
		t.Fatal("an id outside the registry is replayable")
	}
}

func TestEveryReplayableIntentIsRecorded(t *testing.T) {
	for _, id := range replayable(t) {
		r := NewRecorder(1, Gap)
		arg := "Files"
		if _, ok := r.Observe(reflex.Action{Intent: id, Arg: arg}, t0); !ok {
			t.Errorf("%s was not recorded", id)
		}
	}
}

func newStore(t *testing.T) (*Store, *time.Time) {
	t.Helper()
	s := NewStore(filepath.Join(t.TempDir(), "state", "oni-reflex-routines.json"))
	now := t0
	s.now = func() time.Time { return now }
	return s, &now
}

func three() []Step {
	return []Step{{reflex.OpenApp, "Files"}, {reflex.OpenApp, "Editor"}, {reflex.OpenApp, "Music"}}
}

func TestNothingIsWrittenWithoutAccept(t *testing.T) {
	s, _ := newStore(t)
	r := NewRecorder(OfferMinSteps, Gap)
	var offer []Step
	for i, a := range []reflex.Action{open("Files"), open("Editor"), open("Music")} {
		offer, _ = r.Observe(a, t0.Add(time.Duration(i)*time.Second))
	}
	if len(offer) == 0 {
		t.Fatal("no offer")
	}
	if _, ok := s.Latest(); ok {
		t.Fatal("Latest found a routine nobody kept")
	}
	if list, err := s.List(); err != nil || len(list) != 0 {
		t.Fatalf("List = %v, %v", list, err)
	}
	if _, found, err := s.Find("Routine 1"); found || err != nil {
		t.Fatalf("Find = %v, %v", found, err)
	}
	if _, err := os.Stat(filepath.Dir(s.Path())); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the state directory exists before any accept: %v", err)
	}
	// The accept is the write.
	if err := s.Put(Routine{Steps: offer}); err != nil {
		t.Fatal(err)
	}
	got, ok := s.Latest()
	if !ok || got.Name != "Routine 1" || !slices.Equal(got.Steps, offer) || !got.Saved.Equal(t0) || !got.Used.Equal(t0) {
		t.Fatalf("Latest = %+v %v", got, ok)
	}
}

func TestStoreFileIsPrivateAndHoldsOnlySteps(t *testing.T) {
	s, _ := newStore(t)
	if err := s.Put(Routine{Name: "  Morning   start ", Steps: three()}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(s.Path())
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %v, %v", info, err)
	}
	dir, err := os.Stat(filepath.Dir(s.Path()))
	if err != nil || dir.Mode().Perm() != 0o700 {
		t.Fatalf("directory mode = %v, %v", dir, err)
	}
	data, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	keys := map[string]bool{}
	var walk func(any)
	walk = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			for k, x := range v {
				keys[k] = true
				walk(x)
			}
		case []any:
			for _, x := range v {
				walk(x)
			}
		}
	}
	walk(raw)
	for k := range keys {
		if !slices.Contains([]string{"v", "routines", "name", "steps", "intent", "arg", "saved", "used"}, k) {
			t.Errorf("the file holds %q", k)
		}
	}
	got, found, err := s.Find("morning START")
	if err != nil || !found || got.Name != "Morning start" {
		t.Fatalf("Find = %+v %v %v", got, found, err)
	}
	entries, _ := os.ReadDir(filepath.Dir(s.Path()))
	if len(entries) != 1 {
		t.Fatalf("the directory holds %d files, want only the store", len(entries))
	}
}

func TestPutRefusesNonReplayableAndOutOfBounds(t *testing.T) {
	s, _ := newStore(t)
	for _, id := range notReplayable(t) {
		steps := append(three(), Step{Intent: id, Arg: "x"})
		if err := s.Put(Routine{Name: "Bad", Steps: steps}); codeOf(err) != protocol.CodeBadRequest {
			t.Errorf("%s: Put = %v, want bad request", id, err)
		}
	}
	long := make([]Step, MaxSteps+1)
	for i := range long {
		long[i] = Step{reflex.OpenApp, "Files"}
	}
	bad := []Routine{
		{Name: "Empty"},
		{Name: "Long", Steps: long},
		{Name: strings.Repeat("n", MaxNameRunes+1), Steps: three()},
		{Name: "Arg", Steps: []Step{{reflex.OpenApp, strings.Repeat("a", MaxArgRunes+1)}}},
		{Name: "Control", Steps: []Step{{reflex.OpenApp, "Fi\u200bles"}}},
		{Name: "No app", Steps: []Step{{reflex.OpenApp, " "}}},
		{Name: "Bell\a", Steps: three()},
	}
	for _, r := range bad {
		if err := s.Put(r); codeOf(err) != protocol.CodeBadRequest {
			t.Errorf("%q: Put = %v, want bad request", r.Name, err)
		}
	}
	if _, err := os.Stat(s.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused routine wrote the file: %v", err)
	}
	if err := s.Put(Routine{Name: "Full", Steps: long[:MaxSteps]}); err != nil {
		t.Fatalf("a routine of MaxSteps was refused: %v", err)
	}
}

func TestLeastRecentlyUsedGoes(t *testing.T) {
	s, now := newStore(t)
	for i := range MaxRoutines {
		*now = t0.Add(time.Duration(i) * time.Minute)
		if err := s.Put(Routine{Steps: three()}); err != nil {
			t.Fatal(err)
		}
	}
	// Running the oldest makes the second oldest the least recently used.
	*now = t0.Add(time.Hour)
	if err := s.Ran("Routine 1"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Latest(); got.Name != "Routine 1" {
		t.Fatalf("Latest = %q after running it", got.Name)
	}
	*now = t0.Add(2 * time.Hour)
	if err := s.Put(Routine{Name: "New", Steps: three()}); err != nil {
		t.Fatal(err)
	}
	list, err := s.List()
	if err != nil || len(list) != MaxRoutines {
		t.Fatalf("List = %d, %v; want %d", len(list), err, MaxRoutines)
	}
	if list[0].Name != "New" || list[1].Name != "Routine 1" {
		t.Fatalf("order = %q, %q", list[0].Name, list[1].Name)
	}
	if _, found, _ := s.Find("Routine 2"); found {
		t.Fatal("the least recently used routine is still kept")
	}
	// Keeping a name again replaces it rather than evicting another.
	if err := s.Put(Routine{Name: "new", Steps: three()[:1]}); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.List(); len(list) != MaxRoutines {
		t.Fatalf("List = %d after replacing", len(list))
	}
	if got, _, _ := s.Find("NEW"); len(got.Steps) != 1 {
		t.Fatalf("replaced routine = %+v", got)
	}
}

func TestForgetAndForgetAll(t *testing.T) {
	s, now := newStore(t)
	for range 3 {
		if err := s.Put(Routine{Steps: three()}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Forget("routine 2"); err != nil {
		t.Fatal(err)
	}
	if err := s.Forget("Routine 2"); codeOf(err) != protocol.CodeNotFound {
		t.Fatalf("Forget twice = %v", err)
	}
	if err := s.Ran("Routine 2"); codeOf(err) != protocol.CodeNotFound {
		t.Fatalf("Ran forgotten = %v", err)
	}
	if list, _ := s.List(); len(list) != 2 {
		t.Fatalf("List = %d, want 2", len(list))
	}
	// The next free name fills the gap.
	*now = t0.Add(time.Minute)
	if err := s.Put(Routine{Steps: three()}); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Latest(); got.Name != "Routine 2" {
		t.Fatalf("new name = %q", got.Name)
	}
	if err := s.ForgetAll(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ForgetAll left the file: %v", err)
	}
	if _, ok := s.Latest(); ok {
		t.Fatal("Latest after ForgetAll")
	}
	if err := s.ForgetAll(); err != nil {
		t.Fatalf("ForgetAll on nothing = %v", err)
	}
}

func TestDamagedFileIsNeverOverwritten(t *testing.T) {
	s, _ := newStore(t)
	if err := s.Put(Routine{Steps: three()}); err != nil {
		t.Fatal(err)
	}
	good, _ := os.ReadFile(s.Path())
	cases := map[string][]byte{
		"syntax":     []byte("{"),
		"unknown":    []byte(`{"v":1,"routines":[],"words":"hello"}`),
		"version":    []byte(`{"v":2,"routines":[]}`),
		"trailing":   append(slices.Clone(good), []byte(`{}`)...),
		"not kept":   []byte(strings.Replace(string(good), `"open_app"`, `"send_message"`, 1)),
		"no name":    []byte(strings.Replace(string(good), `"Routine 1"`, `""`, 1)),
		"no stamp":   []byte(strings.Replace(string(good), `"saved"`, `"x"`, 1)),
		"duplicates": dupe(t, good),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(s.Path(), data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := s.List(); reasonOf(err) != ReasonDamaged {
				t.Fatalf("List = %v, want damaged", err)
			}
			if _, ok := s.Latest(); ok {
				t.Fatal("Latest read a damaged file")
			}
			if err := s.Put(Routine{Steps: three()}); reasonOf(err) != ReasonDamaged {
				t.Fatalf("Put = %v, want damaged", err)
			}
			if err := s.ForgetAll(); reasonOf(err) != ReasonDamaged {
				t.Fatalf("ForgetAll = %v, want damaged", err)
			}
			if got, _ := os.ReadFile(s.Path()); string(got) != string(data) {
				t.Fatal("a damaged file was overwritten")
			}
			if err := s.Reset(); err != nil {
				t.Fatal(err)
			}
			if got, _ := os.ReadFile(s.DamagedPath()); string(got) != string(data) {
				t.Fatal("Reset did not keep the old file")
			}
			if list, err := s.List(); err != nil || len(list) != 0 {
				t.Fatalf("List after Reset = %v, %v", list, err)
			}
		})
	}
}

func dupe(t *testing.T, good []byte) []byte {
	t.Helper()
	var f storeFile
	if err := json.Unmarshal(good, &f); err != nil {
		t.Fatal(err)
	}
	other := f.Routines[0]
	other.Name = "ROUTINE 1"
	f.Routines = append(f.Routines, other)
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func reasonOf(err error) string {
	var perr *protocol.Error
	if errors.As(err, &perr) {
		return perr.Reason
	}
	return ""
}

func codeOf(err error) string {
	var perr *protocol.Error
	if errors.As(err, &perr) {
		return perr.Code
	}
	return ""
}

type catalog map[string]bool

func (c catalog) Resolve(spoken string) (string, string, float64, bool) {
	if c[spoken] {
		return spoken, strings.ToLower(spoken), 1, true
	}
	return "", "", 0, false
}

func TestPlanSkipsUninstalledAppsWithALine(t *testing.T) {
	others := replayable(t)
	var other reflex.Intent
	for _, id := range others {
		if id != reflex.OpenApp {
			other = id
			break
		}
	}
	r := Routine{Name: "Morning", Steps: []Step{
		{reflex.OpenApp, "Files"},
		{reflex.OpenApp, "Gone"},
		{other, "news"},
		{reflex.OpenApp, "Music"},
	}}
	got := Plan(r, catalog{"Files": true, "Music": true})
	want := []reflex.Action{
		{Intent: reflex.OpenApp, Arg: "Files", Confidence: 1},
		{Intent: other, Arg: "news", Confidence: 1},
		{Intent: reflex.OpenApp, Arg: "Music", Confidence: 1},
	}
	if !slices.Equal(got.Actions, want) {
		t.Fatalf("Actions = %v, want %v", got.Actions, want)
	}
	if len(got.Lines) != 1 || !strings.Contains(got.Lines[0], "Gone") {
		t.Fatalf("Lines = %q, want one naming the missing app", got.Lines)
	}
	none := Plan(r, nil)
	if len(none.Actions) != 1 || none.Actions[0].Intent != other || len(none.Lines) != 3 {
		t.Fatalf("Plan with no catalog = %+v", none)
	}
}

func TestDefaultStorePathIsItsOwnFile(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if got := filepath.Base(DefaultStorePath()); got != "oni-reflex-routines.json" {
		t.Fatalf("DefaultStorePath = %q", got)
	}
	if NewStore("").Path() != DefaultStorePath() {
		t.Fatal("an empty path is not the default")
	}
}
