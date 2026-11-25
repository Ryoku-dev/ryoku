package feature_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/reflex"
	"ryoku-rashin/internal/oni/remind"
	"ryoku-rashin/internal/oni/routine"
	"ryoku-rashin/internal/protocol"
)

// The wire names what the stores keep: a reminder the store kept is
// cancelled by its own id, the kinds are the store's, and a routine name
// the store accepts is one the wire accepts.
func TestReflexWireNamesWhatTheStoresKeep(t *testing.T) {
	if protocol.OniReminderKindReminder != remind.KindReminder || protocol.OniReminderKindTimer != remind.KindTimer {
		t.Fatalf("reminder kinds %q %q, the store's %q %q", protocol.OniReminderKindReminder, protocol.OniReminderKindTimer, remind.KindReminder, remind.KindTimer)
	}
	if protocol.OniRoutineNameMax != routine.MaxNameRunes {
		t.Fatalf("the wire bounds a routine name at %d, the store at %d", protocol.OniRoutineNameMax, routine.MaxNameRunes)
	}
	loc := time.UTC
	now := time.Date(2026, 11, 23, 9, 0, 0, 0, loc)
	r, _, ok := remind.Parse("remind me in 20 minutes to stretch", now, loc)
	if !ok || r.Kind == "" {
		t.Fatalf("the parser claimed nothing: %+v", r)
	}
	store := remind.NewStore(filepath.Join(t.TempDir(), "oni-reminders.json"))
	if err := store.Add(r); err != nil {
		t.Fatal(err)
	}
	kept, err := store.List()
	if err != nil || len(kept) != 1 {
		t.Fatalf("kept %+v, %v", kept, err)
	}
	raw, err := json.Marshal(protocol.OniRemindersParams{Action: protocol.OniRemindersCancel, ID: kept[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := feature.DecodeCall(protocol.MOniReminders, raw); err != nil {
		t.Fatalf("the kept reminder's id %q does not decode: %v", kept[0].ID, err)
	}
	routines := routine.NewStore(filepath.Join(t.TempDir(), "oni-reflex-routines.json"))
	steps := []routine.Step{{Intent: reflex.VolumeUp}, {Intent: reflex.Mute}, {Intent: reflex.PlayPause}}
	if err := routines.Put(routine.Routine{Steps: steps}); err != nil {
		t.Fatal(err)
	}
	latest, ok := routines.Latest()
	if !ok {
		t.Fatal("the store kept no routine")
	}
	raw, err = json.Marshal(protocol.OniRoutinesParams{Action: protocol.OniRoutinesForget, Name: latest.Name})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := feature.DecodeCall(protocol.MOniRoutines, raw); err != nil {
		t.Fatalf("the kept routine's name %q does not decode: %v", latest.Name, err)
	}
}
