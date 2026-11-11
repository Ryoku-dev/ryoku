package teamsui

import (
	"encoding/json"
	"errors"
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/cron"
	"ryoku-rashin/internal/protocol"
)

// a run that is not a task: the scheduler skipped it
const lastSkipped = "skipped"

// Schedules reads each schedule once: a board and a list ask for the same
// few every frame.
type Schedules struct {
	parsed map[string]*cron.Schedule
}

// the schedule, or nil when it does not read
func (s *Schedules) Parse(expression string) *cron.Schedule {
	if s.parsed == nil {
		s.parsed = make(map[string]*cron.Schedule)
	}
	if schedule, ok := s.parsed[expression]; ok {
		return schedule
	}
	schedule, err := cron.Parse(expression)
	if err != nil {
		schedule = nil
	}
	s.parsed[expression] = schedule
	return schedule
}

// a routine's schedule in words: the daemon's, else read here
func (s *Schedules) Sentence(routine protocol.Routine) string {
	if routine.Sentence != "" {
		return routine.Sentence
	}
	if schedule := s.Parse(routine.Schedule); schedule != nil {
		return schedule.Sentence()
	}
	return routine.Schedule
}

// how a routine's last run went, in a few words, and its tone
func RoutineOutcome(snapshot state.Snapshot, routine protocol.Routine) (string, palette.Tone) {
	switch routine.LastStatus {
	case "":
		return i18n.T("No run yet"), palette.Idle
	case protocol.TaskDone:
		if task, ok := snapshot.TeamTasks[routine.LastTask]; ok && task.Report != nil {
			return i18n.Tf("Done: %s", task.Report.Title), palette.Alive
		}
		return i18n.T("Done"), palette.Alive
	case protocol.TaskBlocked, protocol.TaskStopped:
		return i18n.T("Failed"), palette.Failed
	case lastSkipped:
		return i18n.T("Skipped"), palette.Paused
	}
	if task, ok := snapshot.TeamTasks[routine.LastTask]; ok {
		word, tone, _ := TaskStanding(task)
		return word, tone
	}
	return i18n.T("Queued"), palette.Idle
}

// a routine's next run as a claim: when it runs, or why it does not
func RoutineClaim(now time.Time, snapshot state.Snapshot, routine protocol.Routine) (string, palette.Tone) {
	if word, tone := RoutineStanding(routine); word != "" {
		return word, tone
	}
	if team, ok := TeamByID(snapshot.Teams, routine.Team); ok && team.Paused {
		return i18n.T("Team paused"), palette.Paused
	}
	if routine.NextRun == nil {
		return "", palette.Idle
	}
	return look.Until(now, *routine.NextRun), palette.Idle
}

func EnabledRoutines(snapshot state.Snapshot) int {
	count := 0
	for _, routine := range snapshot.Routines {
		if routine.Enabled {
			count++
		}
	}
	return count
}

// starts one run now; the task shows on the Work board and in the routine's last runs
func RunRoutine(e *env.Env, routine protocol.Routine) {
	var task protocol.TeamTask
	e.Call(protocol.MTeamsRoutineRun, protocol.RoutineRef{ID: routine.ID}, &task, func(err error) {
		if err != nil {
			e.Notify(err.Error())
			return
		}
		e.Notify(i18n.Tf("%s started a run.", routine.Title))
	})
}

// patch holds only the fields that changed, as the daemon merges it into the routine
func UpdateRoutine(e *env.Env, id string, patch map[string]any, done func(error)) {
	raw, err := json.Marshal(patch)
	if err != nil {
		done(err)
		return
	}
	e.Call(protocol.MTeamsRoutineUpdate, protocol.RoutineUpdateParams{ID: id, Patch: raw}, new(protocol.Routine), done)
}

// turning one on past the limit is refused here, so a switch never pretends
func SetRoutineEnabled(e *env.Env, snapshot state.Snapshot, routine protocol.Routine, on bool, done func(error)) {
	if on && !routine.Enabled && EnabledRoutines(snapshot) >= protocol.RoutinesEnabledMax {
		done(errors.New(i18n.T("Up to 20 routines can be on at once.")))
		return
	}
	UpdateRoutine(e, routine.ID, map[string]any{"enabled": on}, done)
}

// RoutineSwitch turns a routine on and off; a flip holds until the daemon
// agrees, so the switch never snaps back mid-call.
type RoutineSwitch struct {
	Toggle  kit.Toggle
	enabled Held[bool]
}

// Layout draws the switch over the routine's state, named for a screen
// reader, and sends a flip by the user.
func (s *RoutineSwitch) Layout(gtx layout.Context, e *env.Env, routine protocol.Routine, name string) layout.Dimensions {
	s.Toggle.Value = s.enabled.Show(routine.Enabled)
	dimensions := look.Named(gtx, name, func(gtx layout.Context) layout.Dimensions { return s.Toggle.Layout(gtx, e.Theme.Theme) })
	if s.Toggle.Changed() {
		on := s.Toggle.Value
		s.enabled.Hold(on)
		SetRoutineEnabled(e, e.Snapshot(), routine, on, func(err error) {
			s.enabled.Drop(err)
			e.Report(err)
		})
	}
	return dimensions
}

// RoutineTablet is one routine on a board: its tablet that opens it, its
// switch, and Run now.
type RoutineTablet struct {
	tablet kit.Tablet
	edit   kit.Plaque
	run    kit.IconPlaque
	tip    look.Tip
	enable RoutineSwitch
}

func (r *RoutineTablet) Layout(gtx layout.Context, e *env.Env, schedules *Schedules, routine protocol.Routine, selected bool) layout.Dimensions {
	th := e.Theme
	snapshot := e.Snapshot()
	r.tablet.Pane = th.R.Stone2
	r.edit.Kind, r.edit.Small, r.edit.Label = kit.PlaqueLine, true, i18n.T("Edit")
	r.run.Kind, r.run.Small, r.run.Icon, r.run.Description = kit.PlaqueQuiet, true, look.Icon.Play, i18n.T("Run now")
	if r.tablet.Clicked(gtx) || r.tablet.DoubleClicked(gtx) || r.edit.Clicked(gtx) {
		OpenRoutine(e, routine.ID)
	}
	if r.run.Clicked(gtx) {
		RunRoutine(e, routine)
	}
	team := TeamName(snapshot.Teams, routine.Team)
	claim, claimTone := RoutineClaim(gtx.Now, snapshot, routine)
	last, _ := RoutineOutcome(snapshot, routine)
	content := kit.TabletContent{
		Title:  routine.Title,
		Facts:  i18n.Tf("%s · %s", team, schedules.Sentence(routine)),
		Claim:  claim,
		Proof:  last,
		Action: &r.edit,
		Mark:   Mark(th, team, 32, false),
	}
	// the notch is for a routine that failed, the claim already says one is off
	if claimTone == palette.Failed || claimTone == palette.Attention {
		content.Tone = claimTone
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return r.tablet.Layout(gtx, th.Theme, content, selected, false)
		}),
		layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return r.enable.Layout(gtx, e, routine, i18n.Tf("Turn %s on or off", routine.Title))
		}),
		layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return r.tip.Layout(gtx, th, e.Overlays, r.run.Description, func(gtx layout.Context) layout.Dimensions {
				return r.run.Layout(gtx, th.Theme)
			})
		}),
	)
}
