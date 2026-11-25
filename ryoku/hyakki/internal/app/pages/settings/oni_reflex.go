package settings

import (
	"fmt"
	"sync"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/reflex"
	"ryoku-rashin/internal/oni/reflex/decide"
	"ryoku-rashin/internal/oni/spec"
	"ryoku-rashin/internal/protocol"
)

// oniReflex is Settings' part of Oni's quick commands: their switches, the
// reminders and timers kept with Cancel, the routines kept with Run and
// Forget, and the ledger of what each command came to. Each list is read
// once per visit while its feature may run, and again after a change; none
// holds or shows what was said.
type oniReflex struct {
	reflex, routines, reminders, notes, decisions kit.Toggle

	// the decisions question: the switch turns on only past it
	disclose    kit.Dialog
	disclosing  bool
	discloseYes kit.Plaque
	discloseNo  kit.Plaque

	reminderList reflexList[protocol.OniReminderInfo]
	// noWake: this computer cannot start Oni for a reminder, so Oni stays
	// running while one is set
	noWake    bool
	cancels   map[string]*kit.Plaque
	cancelAll kit.Plaque

	routineList reflexList[protocol.OniRoutineInfo]
	runs        map[string]*kit.Plaque
	forgets     map[string]*kit.Plaque
	forgetAll   kit.Plaque

	log   reflexList[protocol.OniReflexRow]
	again kit.Plaque

	// what the last look for the local decision server found, and the
	// address it looked at; looked for once per visit while the route can
	// reach it, and again when the address changes
	local       protocol.DecisionLocalStatus
	localRead   bool
	localFor    string
	localBusy   bool
	localFailed string
}

// reflexList is one list the daemon keeps, as last read: whether it was
// read and under which switch, a call out, and why it could not be read
type reflexList[T any] struct {
	items   []T
	read    bool
	readOn  bool
	busy    bool
	problem string
}

// due says whether the list needs reading: never while its feature is off,
// and again once the feature turns back on
func (l *reflexList[T]) due(on bool) bool {
	if on != l.readOn {
		l.read, l.readOn = false, on
	}
	if !on {
		l.items, l.problem = nil, ""
		return false
	}
	return !l.read && !l.busy
}

// forget reads every list again the next time the section shows
func (r *oniReflex) forget() {
	r.reminderList.read, r.routineList.read, r.log.read = false, false, false
	r.localRead = false
}

// reflexAllowed says whether a reflex feature may run under these settings
func reflexAllowed(settings protocol.Settings, id string) bool {
	return feature.Decide(feature.Request{Feature: id}, protocol.EffectiveOni(settings)).Allow
}

var reflexNumbers = sync.OnceValues(func() (spec.Reflex, error) {
	loaded, err := spec.Load()
	if err != nil {
		return spec.Reflex{}, err
	}
	return loaded.Reflex, nil
})

func reflexSet(e *env.Env, values map[string]any) {
	set(e, "oni", map[string]any{"reflexSettings": values})
}

// remindersCall lists or cancels reminders; the answer is the list after it
func (r *oniReflex) remindersCall(e *env.Env, params protocol.OniRemindersParams, done string) {
	l := &r.reminderList
	l.busy = true
	result := new(protocol.OniRemindersResult)
	e.Call(protocol.MOniReminders, params, result, func(err error) {
		l.busy, l.read = false, true
		if err != nil {
			l.problem = err.Error()
			return
		}
		l.items, l.problem, r.noWake = result.Reminders, "", result.NoWake
		if done != "" && e.Notify != nil {
			e.Notify(done)
		}
	})
}

// routinesCall lists, runs or forgets routines; the answer is the list
// after it
func (r *oniReflex) routinesCall(e *env.Env, params protocol.OniRoutinesParams, done string) {
	l := &r.routineList
	l.busy = true
	result := new(protocol.OniRoutinesResult)
	e.Call(protocol.MOniRoutines, params, result, func(err error) {
		l.busy, l.read = false, true
		if err != nil {
			if params.Action == protocol.OniRoutinesList {
				l.problem = err.Error()
			} else {
				e.Report(err)
			}
			return
		}
		l.items, l.problem = result.Routines, ""
		if done != "" && e.Notify != nil {
			e.Notify(done)
		}
	})
}

func (r *oniReflex) readLog(e *env.Env) {
	l := &r.log
	l.busy = true
	result := new(protocol.OniReflexLogResult)
	e.Call(protocol.MOniReflexLog, protocol.Empty{}, result, func(err error) {
		l.busy, l.read = false, true
		if err != nil {
			l.problem = err.Error()
			return
		}
		l.items, l.problem = result.Rows, ""
	})
}

// oniReflexBlocks are the Quick commands group with its switches, and the
// reminders, routines and log groups
func (p *Page) oniReflexBlocks(e *env.Env, oni, defaults protocol.OniSettings) (commands, reminders, routines, log []look.Setting) {
	r := &p.oni.reflex
	settings := e.Snapshot().Settings
	if r.reminderList.due(reflexAllowed(settings, feature.Reminders)) {
		r.remindersCall(e, protocol.OniRemindersParams{Action: protocol.OniRemindersList}, "")
	}
	if r.routineList.due(reflexAllowed(settings, feature.Routines)) {
		r.routinesCall(e, protocol.OniRoutinesParams{Action: protocol.OniRoutinesList}, "")
	}
	if r.log.due(reflexAllowed(settings, feature.Reflex)) {
		r.readLog(e)
	}
	r.lookLocal(e, settings)
	return r.switchRows(e, settings, oni, defaults), r.reminderRows(e, settings), r.routineRows(e, settings), r.logRows(e, settings)
}

// localReached says whether the decision route can reach the local
// decision server: pinned there, or automatic or Ember with a local model
// to fall back on
func localReached(cfg protocol.DecisionRoutingSettings) bool {
	switch decisionRoute(cfg) {
	case protocol.DecisionLocal:
		return true
	case protocol.DecisionAuto, protocol.DecisionEmber:
		return cfg.LocalModel != ""
	}
	return false
}

// lookLocal asks the daemon to look for the local decision server, once per
// visit while Oni is on and the route can reach it, and again when the
// address changes; nothing polls it
func (r *oniReflex) lookLocal(e *env.Env, settings protocol.Settings) {
	cfg := settings.Providers.Decisions
	if !protocol.EffectiveOni(settings).Master || !localReached(cfg) || r.localBusy || (r.localRead && r.localFor == cfg.LocalAddress) {
		return
	}
	address := cfg.LocalAddress
	r.localBusy = true
	result := new(protocol.DecisionLocalStatus)
	e.Call(protocol.MFastDecideLocal, protocol.Empty{}, result, func(err error) {
		r.localBusy, r.localRead, r.localFor = false, true, address
		if err != nil {
			r.localFailed = err.Error()
			return
		}
		r.local, r.localFailed = *result, ""
	})
}

// localLine says what the last look found: detected, not found, or this
// computer too small; nothing while the route cannot reach it
func (r *oniReflex) localLine(settings protocol.Settings) string {
	cfg := settings.Providers.Decisions
	if !protocol.EffectiveOni(settings).Master || !localReached(cfg) {
		return ""
	}
	if !r.localRead || r.localFor != cfg.LocalAddress {
		return "\n" + i18n.T("Looking for the local decision server…")
	}
	if r.localFailed != "" {
		return "\n" + i18n.Tf("Could not look for the local decision server: %s", r.localFailed)
	}
	switch r.local.State {
	case protocol.DecisionLocalDetected:
		return "\n" + i18n.Tf("Local decision server: detected at %s.", r.local.Address)
	case protocol.DecisionLocalAbsent:
		return "\n" + i18n.T("Local decision server: not found.") + " " + r.local.Reason
	}
	// too small, or an address off this computer: the reason says which
	return "\n" + r.local.Reason
}

// switchRows are the five switches, each saying what it needs while that
// is off
func (r *oniReflex) switchRows(e *env.Env, settings protocol.Settings, oni, defaults protocol.OniSettings) []look.Setting {
	th := e.Theme
	rs, ds := oni.ReflexSettings, defaults.ReflexSettings
	r.reflex.Value = oni.Reflex
	r.routines.Value, r.reminders.Value, r.notes.Value, r.decisions.Value = rs.Routines, rs.Reminders, rs.Notes, rs.Decisions
	numbers, numbersErr := reflexNumbers()

	off := ""
	if !protocol.EffectiveOni(settings).Master {
		off = i18n.T("Oni is off, so it cannot carry out a command.")
	}
	quick := i18n.T("Quick commands")
	quickDetail := i18n.T("Hold the talk chord and say a short command, such as open the browser, snap this left or pause the music. Oni does it at once and says what it did. Switching this off stops every quick command.")
	if !oni.Voice {
		quickDetail += "\n" + i18n.T("Needs Talk to Oni: quick commands are only ever spoken, so until it is on they do nothing.")
	}
	needsQuick := "\n" + i18n.T("Needs Quick commands; until it is on, this does nothing.")

	remindName := i18n.T("Reminders and timers")
	remindDetail := i18n.T("Say remind me in 20 minutes to stretch, or set a timer for 5 minutes. Oni says each when it comes due, or shows a notification while it is hidden.")
	if numbersErr == nil {
		remindDetail = i18n.Tf("Say remind me in 20 minutes to stretch, or set a timer for 5 minutes. A reminder reaches %d days ahead at most and a timer %s. Oni says each when it comes due, or shows a notification while it is hidden.", numbers.Reminders.MaxDays, spokenSeconds(numbers.Reminders.MaxTimerH*3600))
	}
	routineName := i18n.T("Routines")
	routineDetail := i18n.T("After a few quick commands in a row, Oni offers once to keep them as a routine; nothing is kept unless you say yes while holding the chord or press Keep. Say run my routine to replay it.")
	if numbersErr == nil {
		routineDetail = i18n.Tf("After %d quick commands in a row, Oni offers once to keep them as a routine; nothing is kept unless you say yes while holding the chord or press Keep. Say run my routine to replay it.", numbers.Routines.OfferMinSteps)
	}
	notesName := i18n.T("Notes")
	notesDetail := i18n.T("Hold the chord and say remember that, and Oni keeps the rest in memory as asking would. Card numbers, passwords and ID numbers are refused.")
	if m := settings.Memory; !m.Asked || !m.Use {
		notesDetail += "\n" + i18n.T("Memory is off, so Oni says it cannot keep a note until memory is on.")
	}
	decideName := i18n.T("Let a decision model sort commands")
	decideDetail := i18n.T("When the built-in rules miss a command, the Fast decisions route under Providers may pick one of Oni's fixed commands. It picks a command, never a permission.") + "\n" + decisionRouteLine(settings.Providers.Decisions) + r.localLine(settings)

	if !oni.Reflex {
		remindDetail += needsQuick
		routineDetail += needsQuick
		notesDetail += needsQuick
		decideDetail += needsQuick
	}
	return []look.Setting{
		{Title: quick, Detail: quickDetail, Unavailable: off,
			Changed: oni.Reflex != defaults.Reflex, Reset: func() { set(e, "oni", map[string]any{"reflex": defaults.Reflex}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, quick, &r.reflex, func(on bool) { set(e, "oni", map[string]any{"reflex": on}) })
			}},
		{Title: remindName, Detail: remindDetail, Unavailable: off,
			Changed: rs.Reminders != ds.Reminders, Reset: func() { reflexSet(e, map[string]any{"reminders": ds.Reminders}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, remindName, &r.reminders, func(on bool) { reflexSet(e, map[string]any{"reminders": on}) })
			}},
		{Title: routineName, Detail: routineDetail, Unavailable: off,
			Changed: rs.Routines != ds.Routines, Reset: func() { reflexSet(e, map[string]any{"routines": ds.Routines}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, routineName, &r.routines, func(on bool) { reflexSet(e, map[string]any{"routines": on}) })
			}},
		{Title: notesName, Detail: notesDetail, Unavailable: off,
			Changed: rs.Notes != ds.Notes, Reset: func() { reflexSet(e, map[string]any{"notes": ds.Notes}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, notesName, &r.notes, func(on bool) { reflexSet(e, map[string]any{"notes": on}) })
			}},
		{Title: decideName, Detail: decideDetail, Unavailable: off,
			Changed: rs.Decisions != ds.Decisions, Reset: func() { reflexSet(e, map[string]any{"decisions": ds.Decisions}) },
			Control: func(gtx layout.Context) layout.Dimensions {
				return namedSwitch(gtx, th, decideName, &r.decisions, func(on bool) {
					if !on {
						reflexSet(e, map[string]any{"decisions": false})
						return
					}
					// on only past the question; until then the setting holds it off
					r.disclosing, r.disclose.Open, r.disclose.Width = true, true, 560
				})
			}},
	}
}

// decisionRoute is the route quick commands are decided on: their own
// feature's, else the route chosen for every typed decision, else the
// built-in rules
func decisionRoute(cfg protocol.DecisionRoutingSettings) string {
	if route := cfg.Features[decide.Feature]; route != "" {
		return route
	}
	if cfg.Route != "" {
		return cfg.Route
	}
	return protocol.DecisionRules
}

// decisionRouteLine says where a command the rules miss would go now
func decisionRouteLine(cfg protocol.DecisionRoutingSettings) string {
	switch decisionRoute(cfg) {
	case protocol.DecisionLocal:
		return i18n.T("The route is a local decision model, so nothing leaves this computer.")
	case protocol.DecisionAuto:
		if cfg.AllowHosted {
			return i18n.T("The route is automatic: the local decision model first, then Ryoku Ember, which asks on a card each time before it is used.")
		}
		return i18n.T("The route is automatic and Ryoku Ember is not allowed there, so nothing leaves this computer.")
	case protocol.DecisionEmber:
		return i18n.T("The route is Ryoku Ember, which asks on a card each time before it is used.")
	}
	return i18n.T("The route is Built-in rules, so this changes nothing until a decision model is chosen there.")
}

// decisionRouteLabel names a route as Providers does
func decisionRouteLabel(route string) string {
	switch route {
	case protocol.DecisionLocal:
		return i18n.T("Local decision model")
	case protocol.DecisionAuto:
		return i18n.T("Automatic local-first")
	case protocol.DecisionEmber:
		return i18n.T("Ryoku Ember")
	}
	return i18n.T("Built-in rules")
}

// emberMayDecide says whether the route may reach Ryoku Ember at all
func emberMayDecide(cfg protocol.DecisionRoutingSettings) bool {
	route := decisionRoute(cfg)
	return route == protocol.DecisionEmber || (route == protocol.DecisionAuto && cfg.AllowHosted)
}

// overlay is the decisions question
func (r *oniReflex) overlay(gtx layout.Context, e *env.Env) bool {
	if !r.disclose.Open && !r.disclosing {
		return false
	}
	th := e.Theme
	r.discloseNo.Kind, r.discloseNo.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	r.discloseYes.Kind, r.discloseYes.Label = kit.PlaqueGold, i18n.T("Let it sort commands")
	if r.discloseNo.Clicked(gtx) {
		r.disclose.Dismiss(gtx)
	}
	if r.discloseYes.Clicked(gtx) && r.disclose.Open {
		reflexSet(e, map[string]any{"decisions": true})
		r.disclose.Dismiss(gtx)
	}
	cfg := e.Snapshot().Settings.Providers.Decisions
	facts := []look.FactRow{{Label: i18n.T("Route now"), Value: decisionRouteLabel(decisionRoute(cfg))}}
	ember := i18n.T("Not used: the route keeps every decision on this computer")
	if emberMayDecide(cfg) {
		ember = i18n.T("Asks first each time, on a card you answer with the chord or a click, naming the account whose Ember credits it spends")
	}
	facts = append(facts, look.FactRow{Label: i18n.T("Ryoku Ember"), Value: ember})
	if numbers, err := reflexNumbers(); err == nil {
		facts = append(facts,
			look.FactRow{Label: i18n.T("Sure enough"), Value: i18n.Tf("A command is taken only at %d%% or more", int(numbers.Decisions.MinConfidence*100+0.5))},
			look.FactRow{Label: i18n.T("Ember at most"), Value: i18n.Tf("%d decisions a minute", numbers.Decisions.HostedPerMin)},
		)
	}
	r.disclose.Layout(gtx, th.Theme, i18n.T("Let a decision model sort commands?"), func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(look.Paragraph(th, th.T.Row, th.R.Fg, i18n.T("When the built-in rules do not recognise a command, Oni asks the Fast decisions route chosen under Providers to pick one of its fixed commands, or none. Only the words of that one command and the names of the commands are sent: never the screen, a window, its title or anything said before."))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.Facts(gtx, th, facts) }),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, i18n.T("A decision picks a command; it never grants a permission, and a command that needs your press still needs it. When the route is down, slow or out of credits, the built-in rules carry on and nothing is spent."))),
		)
	},
		func(gtx layout.Context) layout.Dimensions { return r.discloseNo.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions { return r.discloseYes.Layout(gtx, th.Theme) },
	)
	if r.disclose.Closed() {
		r.disclosing = false
	}
	return r.disclose.Open
}

// plaqueFor keeps one plaque per listed key, dropping those no longer listed
func plaqueFor(plaques *map[string]*kit.Plaque, key string) *kit.Plaque {
	if *plaques == nil {
		*plaques = map[string]*kit.Plaque{}
	}
	plaque := (*plaques)[key]
	if plaque == nil {
		plaque = new(kit.Plaque)
		(*plaques)[key] = plaque
	}
	return plaque
}

func prunePlaques(plaques map[string]*kit.Plaque, listed map[string]bool) {
	for key := range plaques {
		if !listed[key] {
			delete(plaques, key)
		}
	}
}

// reminderRows list the reminders and timers kept, soonest first, each with
// Cancel: a reminder by its own words, a timer by when it is due
func (r *oniReflex) reminderRows(e *env.Env, settings protocol.Settings) []look.Setting {
	th := e.Theme
	l := &r.reminderList
	if !reflexAllowed(settings, feature.Reminders) {
		return []look.Setting{{Title: i18n.T("Nothing to list"), Detail: i18n.T("Turn on Quick commands and Reminders and timers to set reminders and list them here.")}}
	}
	var rows []look.Setting
	if r.noWake {
		rows = append(rows, look.Setting{Title: i18n.T("Oni stays running while a reminder is set"),
			Detail: i18n.T("This computer cannot start Oni on its own when a reminder comes due, so Oni keeps running from when one is set until the last is said or cancelled. With none set, Oni rests as usual.")})
	}
	if l.problem != "" {
		return append(rows, look.Setting{Title: i18n.T("Reminders cannot be read"), Detail: l.problem})
	}
	if !l.read {
		return append(rows, look.Setting{Title: i18n.T("Reading reminders…")})
	}
	if len(l.items) == 0 {
		return append(rows, look.Setting{Title: i18n.T("Nothing set"), Detail: i18n.T("Hold the chord and say remind me in 20 minutes to stretch, or set a timer for 5 minutes.")})
	}
	listed := make(map[string]bool, len(l.items))
	for _, item := range l.items {
		listed[item.ID] = true
		plaque := plaqueFor(&r.cancels, item.ID)
		plaque.Kind, plaque.Small, plaque.Label = kit.PlaqueQuiet, true, i18n.T("Cancel")
		due := item.Due.Local().Format("Mon 2 Jan, 15:04")
		title, name := item.Text, i18n.Tf("Cancel “%s”", item.Text)
		detail := i18n.Tf("Reminder · due %s", due)
		if item.Kind == protocol.OniReminderKindTimer || item.Text == "" {
			title, name = i18n.T("Timer"), i18n.Tf("Cancel the timer due %s", due)
			detail = i18n.Tf("Due %s", due)
		}
		if item.Late {
			detail += " · " + i18n.T("came due late")
		}
		id, said := item.ID, title
		rows = append(rows, look.Setting{Title: title, Detail: detail,
			Control: func(gtx layout.Context) layout.Dimensions {
				if plaque.Clicked(gtx) && !l.busy {
					r.remindersCall(e, protocol.OniRemindersParams{Action: protocol.OniRemindersCancel, ID: id}, i18n.Tf("Cancelled “%s”.", said))
				}
				if l.busy {
					gtx = gtx.Disabled()
				}
				return look.Named(gtx, name, func(gtx layout.Context) layout.Dimensions { return plaque.Layout(gtx, th.Theme) })
			}})
	}
	prunePlaques(r.cancels, listed)
	r.cancelAll.Kind, r.cancelAll.Small, r.cancelAll.Label = kit.PlaqueTonal, true, i18n.T("Cancel all")
	return append(rows, look.Setting{Title: i18n.T("Cancel every reminder and timer"), Detail: i18n.T("None of them is said. They cannot be brought back."),
		Control: func(gtx layout.Context) layout.Dimensions {
			if r.cancelAll.Clicked(gtx) && !l.busy {
				r.remindersCall(e, protocol.OniRemindersParams{Action: protocol.OniRemindersCancelAll}, i18n.T("Cancelled every reminder and timer."))
			}
			if l.busy {
				gtx = gtx.Disabled()
			}
			return r.cancelAll.Layout(gtx, th.Theme)
		}})
}

// routineRows list the routines kept, the most recently used first, each
// with Run and Forget; a routine shows its name and length, never its words
func (r *oniReflex) routineRows(e *env.Env, settings protocol.Settings) []look.Setting {
	th := e.Theme
	l := &r.routineList
	if !reflexAllowed(settings, feature.Routines) {
		return []look.Setting{{Title: i18n.T("Nothing to list"), Detail: i18n.T("Turn on Quick commands and Routines to keep routines and list them here.")}}
	}
	if l.problem != "" {
		return []look.Setting{{Title: i18n.T("Routines cannot be read"), Detail: l.problem}}
	}
	if !l.read {
		return []look.Setting{{Title: i18n.T("Reading routines…")}}
	}
	if len(l.items) == 0 {
		return []look.Setting{{Title: i18n.T("No routine kept"), Detail: i18n.T("Give a few quick commands in a row and say yes when Oni offers to keep them.")}}
	}
	var rows []look.Setting
	listed := make(map[string]bool, len(l.items))
	for _, item := range l.items {
		listed[item.Name] = true
		run, forget := plaqueFor(&r.runs, item.Name), plaqueFor(&r.forgets, item.Name)
		run.Kind, run.Small, run.Label = kit.PlaqueTonal, true, i18n.T("Run")
		forget.Kind, forget.Small, forget.Label = kit.PlaqueQuiet, true, i18n.T("Forget")
		steps := i18n.T("1 step")
		if item.Steps != 1 {
			steps = i18n.Tf("%d steps", item.Steps)
		}
		detail := i18n.Tf("%s · saved %s · last used %s", steps, item.Saved.Local().Format("2 Jan 2006"), item.Used.Local().Format("2 Jan 2006"))
		name := item.Name
		rows = append(rows, look.Setting{Title: name, Detail: detail,
			Control: func(gtx layout.Context) layout.Dimensions {
				if run.Clicked(gtx) && !l.busy {
					r.routinesCall(e, protocol.OniRoutinesParams{Action: protocol.OniRoutinesRun, Name: name}, i18n.Tf("Ran “%s”.", name))
				}
				if forget.Clicked(gtx) && !l.busy {
					r.routinesCall(e, protocol.OniRoutinesParams{Action: protocol.OniRoutinesForget, Name: name}, i18n.Tf("Forgot “%s”.", name))
				}
				if l.busy {
					gtx = gtx.Disabled()
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return look.Named(gtx, i18n.Tf("Run “%s”", name), func(gtx layout.Context) layout.Dimensions { return run.Layout(gtx, th.Theme) })
					}),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return look.Named(gtx, i18n.Tf("Forget “%s”", name), func(gtx layout.Context) layout.Dimensions { return forget.Layout(gtx, th.Theme) })
					}),
				)
			}})
	}
	prunePlaques(r.runs, listed)
	prunePlaques(r.forgets, listed)
	r.forgetAll.Kind, r.forgetAll.Small, r.forgetAll.Label = kit.PlaqueTonal, true, i18n.T("Forget all")
	return append(rows, look.Setting{Title: i18n.T("Forget every routine"), Detail: i18n.T("Oni stops replaying them. They cannot be brought back."),
		Control: func(gtx layout.Context) layout.Dimensions {
			if r.forgetAll.Clicked(gtx) && !l.busy {
				r.routinesCall(e, protocol.OniRoutinesParams{Action: protocol.OniRoutinesForgetAll}, i18n.T("Forgot every routine."))
			}
			if l.busy {
				gtx = gtx.Disabled()
			}
			return r.forgetAll.Layout(gtx, th.Theme)
		}})
}

// logRows are the ledger, newest first: which command, the route that
// named it, how long and how sure, and what came of it. No row holds words.
func (r *oniReflex) logRows(e *env.Env, settings protocol.Settings) []look.Setting {
	th := e.Theme
	l := &r.log
	if !reflexAllowed(settings, feature.Reflex) {
		return []look.Setting{{Title: i18n.T("Nothing to list"), Detail: i18n.T("The log is read while Quick commands are on.")}}
	}
	r.again.Kind, r.again.Small, r.again.Label = kit.PlaqueQuiet, true, i18n.T("Read again")
	head := look.Setting{Title: i18n.T("What quick commands came to"), Detail: i18n.T("The newest first. Each row names a command, never the words said."),
		Control: func(gtx layout.Context) layout.Dimensions {
			if r.again.Clicked(gtx) && !l.busy {
				r.readLog(e)
			}
			if l.busy {
				gtx = gtx.Disabled()
			}
			return look.Named(gtx, i18n.T("Read the log again"), func(gtx layout.Context) layout.Dimensions { return r.again.Layout(gtx, th.Theme) })
		}}
	rows := []look.Setting{head}
	switch {
	case l.problem != "":
		return append(rows, look.Setting{Title: i18n.T("The log cannot be read"), Detail: l.problem})
	case !l.read:
		return rows
	case len(l.items) == 0:
		return append(rows, look.Setting{Title: i18n.T("No quick command has run yet")})
	}
	for _, row := range l.items {
		detail := fmt.Sprintf("%s · %s · %s · %s · %s", row.At.Local().Format("Mon 15:04:05"), decisionRouteLabel(row.Route),
			i18n.Tf("%d ms", row.Ms), i18n.Tf("%d%% sure", int(row.Confidence*100+0.5)), outcomeLabel(row.Outcome))
		rows = append(rows, look.Setting{Title: intentLabel(row.Intent), Detail: detail})
	}
	return rows
}

// intentLabel names a registry command in the user's terms
func intentLabel(id string) string {
	switch reflex.Intent(id) {
	case reflex.OpenApp:
		return i18n.T("Open an app")
	case reflex.OpenSite:
		return i18n.T("Open a site")
	case reflex.WebSearch:
		return i18n.T("Search the web")
	case reflex.NewTab:
		return i18n.T("Open a new tab")
	case reflex.SnapLeft:
		return i18n.T("Snap a window left")
	case reflex.SnapRight:
		return i18n.T("Snap a window right")
	case reflex.Maximize:
		return i18n.T("Maximize a window")
	case reflex.Minimize:
		return i18n.T("Minimize a window")
	case reflex.VolumeUp:
		return i18n.T("Turn the volume up")
	case reflex.VolumeDown:
		return i18n.T("Turn the volume down")
	case reflex.Mute:
		return i18n.T("Mute or unmute")
	case reflex.PlayPause:
		return i18n.T("Play or pause")
	case reflex.Screenshot:
		return i18n.T("Take a screenshot")
	case reflex.RememberNote:
		return i18n.T("Remember a note")
	case reflex.RunRoutine:
		return i18n.T("Run a routine")
	case reflex.SetTimer:
		return i18n.T("Set a timer")
	case reflex.SetReminder:
		return i18n.T("Set a reminder")
	case reflex.CancelTimer:
		return i18n.T("Cancel a reminder or timer")
	case reflex.ListReminders:
		return i18n.T("List reminders")
	case reflex.SelfMove:
		return i18n.T("Move Oni")
	case reflex.SelfSize:
		return i18n.T("Resize Oni")
	case reflex.SelfHome:
		return i18n.T("Send Oni home")
	case reflex.SettingFlip:
		return i18n.T("Change a setting")
	}
	return id
}

// outcomeLabel says what became of a command
func outcomeLabel(outcome string) string {
	switch outcome {
	case protocol.OniReflexDone:
		return i18n.T("Done")
	case protocol.OniReflexUnsupported:
		return i18n.T("Not possible on this desktop")
	case protocol.OniReflexRefused:
		return i18n.T("Refused")
	case protocol.OniReflexCooldown:
		return i18n.T("Held by the launch pause")
	case protocol.OniReflexSkipped:
		return i18n.T("Skipped: the app is not installed")
	case protocol.OniReflexFailed:
		return i18n.T("Failed")
	}
	return outcome
}
