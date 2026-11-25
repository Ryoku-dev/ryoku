package demo

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/reflex"
	"ryoku-rashin/internal/protocol"
)

// FeatureOniReflex is no feature: it starts the demo with Oni, talking to
// Oni and quick commands on, with routines, reminders and notes, and the
// reflex fixtures below to list. Decisions stay off, so turning them on
// shows its disclosure.
const FeatureOniReflex = "oni-reflex"

// FeatureOniReflexNoWake is FeatureOniReflex on a machine whose wake unit
// cannot be armed, so Oni stays running while a reminder is set and
// oni.reminders says so.
const FeatureOniReflexNoWake = "oni-reflex-no-wake"

// reflexFixtures is what the demo's reflex keeps: two reminders, two
// routines and a short ledger, all told from Epoch so every run answers the
// same. Reminders are soonest first, routines and rows newest first. noWake
// is a machine whose wake unit cannot be armed.
type reflexFixtures struct {
	reminders []protocol.OniReminderInfo
	routines  []protocol.OniRoutineInfo
	rows      []protocol.OniReflexRow
	noWake    bool
}

func newReflexFixtures() reflexFixtures {
	return reflexFixtures{
		reminders: []protocol.OniReminderInfo{
			{ID: "5d2c9a0e7b41f386", Kind: protocol.OniReminderKindTimer, Due: Epoch.Add(5 * time.Minute)},
			{ID: "a07e3c18d95b42f6", Kind: protocol.OniReminderKindReminder, Due: Epoch.Add(24*time.Hour + 30*time.Minute), Text: "call the dentist"},
		},
		routines: []protocol.OniRoutineInfo{
			{Name: "Morning", Steps: 3, Saved: Epoch.Add(-72 * time.Hour), Used: Epoch.Add(-24 * time.Hour)},
			{Name: "Wind down", Steps: 2, Saved: Epoch.Add(-96 * time.Hour), Used: Epoch.Add(-48 * time.Hour)},
		},
		rows: []protocol.OniReflexRow{
			{At: Epoch.Add(-2 * time.Minute), Intent: string(reflex.OpenApp), Route: protocol.DecisionRules, Ms: 2, Confidence: 0.97, Outcome: protocol.OniReflexCooldown},
			{At: Epoch.Add(-3 * time.Minute), Intent: string(reflex.Minimize), Route: protocol.DecisionRules, Ms: 1, Confidence: 0.95, Outcome: protocol.OniReflexUnsupported},
			{At: Epoch.Add(-4 * time.Minute), Intent: string(reflex.VolumeUp), Route: protocol.DecisionLocal, Ms: 140, Confidence: 0.82, Outcome: protocol.OniReflexDone},
			{At: Epoch.Add(-5 * time.Minute), Intent: string(reflex.OpenApp), Route: protocol.DecisionRules, Ms: 3, Confidence: 0.97, Outcome: protocol.OniReflexDone},
		},
	}
}

// the demo's reflex calls answer as the daemon will: each needs its
// feature, and nothing in them carries what was said
func (s *server) callOniReflex(method string, raw json.RawMessage) (any, error) {
	r := &s.data.oni.reflex
	settings := protocol.EffectiveOni(s.data.snapshot.Settings)
	params, err := feature.DecodeCall(method, raw)
	if err != nil {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The Oni call is invalid: %v.", err))
	}
	switch p := params.(type) {
	case *protocol.OniRemindersParams:
		if err := demoRefusal(settings, feature.Reminders); err != nil {
			return nil, err
		}
		return r.remindersCall(*p)
	case *protocol.OniRoutinesParams:
		if err := demoRefusal(settings, feature.Routines); err != nil {
			return nil, err
		}
		return r.routinesCall(*p)
	}
	if method != protocol.MOniReflexLog {
		return nil, notFound(method)
	}
	if err := demoRefusal(settings, feature.Reflex); err != nil {
		return nil, err
	}
	return protocol.OniReflexLogResult{Rows: slices.Clone(r.rows)}, nil
}

func (r *reflexFixtures) remindersCall(p protocol.OniRemindersParams) (protocol.OniRemindersResult, error) {
	switch p.Action {
	case protocol.OniRemindersAdd:
		id := fmt.Sprintf("%016x", uint64(p.Due.Unix())^uint64(len(r.reminders)+1))
		r.reminders = append(r.reminders, protocol.OniReminderInfo{ID: id, Kind: p.Kind, Due: p.Due, Text: p.Text})
		slices.SortStableFunc(r.reminders, func(a, b protocol.OniReminderInfo) int { return a.Due.Compare(b.Due) })
	case protocol.OniRemindersCancel:
		i := slices.IndexFunc(r.reminders, func(x protocol.OniReminderInfo) bool { return x.ID == p.ID })
		if i < 0 {
			return protocol.OniRemindersResult{}, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("That reminder is not set."))
		}
		r.reminders = slices.Delete(r.reminders, i, i+1)
	case protocol.OniRemindersCancelAll:
		r.reminders = nil
	}
	return protocol.OniRemindersResult{Reminders: append([]protocol.OniReminderInfo{}, r.reminders...), NoWake: r.noWake}, nil
}

func (r *reflexFixtures) routinesCall(p protocol.OniRoutinesParams) (protocol.OniRoutinesResult, error) {
	find := func(name string) int {
		key := strings.ToLower(strings.Join(strings.Fields(name), " "))
		return slices.IndexFunc(r.routines, func(x protocol.OniRoutineInfo) bool {
			return strings.ToLower(strings.Join(strings.Fields(x.Name), " ")) == key
		})
	}
	missing := protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("There is no routine by that name."))
	ran := false
	switch p.Action {
	case protocol.OniRoutinesRun:
		i := 0
		if p.Name != "" {
			i = find(p.Name)
		}
		if i < 0 || len(r.routines) == 0 {
			return protocol.OniRoutinesResult{}, missing
		}
		// a run makes the routine the most recently used, at the demo's
		// fixed moment
		used := r.routines[i]
		used.Used = Epoch
		r.routines = append([]protocol.OniRoutineInfo{used}, slices.Delete(r.routines, i, i+1)...)
		ran = true
	case protocol.OniRoutinesForget:
		i := find(p.Name)
		if i < 0 {
			return protocol.OniRoutinesResult{}, missing
		}
		r.routines = slices.Delete(r.routines, i, i+1)
	case protocol.OniRoutinesForgetAll:
		r.routines = nil
	}
	return protocol.OniRoutinesResult{Routines: append([]protocol.OniRoutineInfo{}, r.routines...), Ran: ran}, nil
}
