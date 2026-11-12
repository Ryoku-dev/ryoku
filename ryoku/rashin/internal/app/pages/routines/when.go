package routines

import (
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/cron"
	"ryoku-rashin/internal/protocol"
)

// how many next runs the aside lists
const previewCount = 5

// the presets build the five-field schedule and say it in words under the fields
type whenBuilder struct {
	plan    plan
	preset  look.Select
	minute  kit.Stepper
	day     kit.Stepper
	clock   kit.TextField
	custom  kit.TextField
	weekday [7]kit.Plaque
	// the clock as it was set, so an error waits until the user wrote something
	clockSet string
	reading  reading
}

// a schedule read once per change: its words, why it is no schedule, its next runs
type reading struct {
	ready      bool
	expression string
	sentence   string
	problem    string
	next       []time.Time
}

func (w *whenBuilder) set(next plan) {
	w.plan = next
	w.minute.Value, w.day.Value = next.minute, next.day
	w.clockSet = clockText(next.hour, next.minute)
	w.clock.SetText(w.clockSet)
	w.custom.SetText(next.custom)
	w.reading = reading{}
}

// the schedule the fields make now, or why they make none
func (w *whenBuilder) expression() (string, string) {
	made := w.plan
	switch made.preset {
	case everyHour:
		made.minute = w.minute.Value
	case custom:
		made.custom = w.custom.Text()
	default:
		hour, minute, ok := readClock(w.clock.Text())
		if !ok {
			return "", i18n.T("Write the time as HH:MM, such as 09:00.")
		}
		made.hour, made.minute, made.day = hour, minute, w.day.Value
	}
	return made.expression()
}

// reads the schedule again only when it changed or its next run has passed
func (r *reading) update(expression, problem string, now time.Time) {
	if problem != "" {
		*r = reading{problem: problem}
		return
	}
	if r.ready && r.expression == expression && (len(r.next) == 0 || now.Before(r.next[0])) {
		return
	}
	*r = reading{ready: true, expression: expression}
	schedule, err := cron.Parse(expression)
	if err != nil {
		r.problem = err.Error()
		return
	}
	r.sentence = schedule.Sentence()
	if schedule.MinInterval() < protocol.RoutineMinInterval {
		r.problem = i18n.T("A routine runs at most every 15 minutes.")
		return
	}
	preview := cron.Preview(expression, now, previewCount)
	r.next, r.problem = preview.Next, preview.Error
}

func (w *whenBuilder) layout(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	w.configure()
	w.clickDays(gtx)
	expression, problem := w.expression()
	w.reading.update(expression, problem, gtx.Now)
	rows := []layout.Widget{
		kit.Text(th.Theme, th.T.Title, th.R.Fg, i18n.T("When")),
		func(gtx layout.Context) layout.Dimensions { return w.layoutPreset(gtx, e, expression) },
	}
	rows = append(rows, w.fields(th)...)
	rows = append(rows, func(gtx layout.Context) layout.Dimensions {
		if w.reading.problem != "" {
			return look.Paragraph(th, th.T.Detail, th.R.Amber, w.reading.problem)(gtx)
		}
		return look.Paragraph(th, th.T.Detail, th.R.Fg2, w.reading.sentence)(gtx)
	})
	// the limit is said once: in amber when this schedule breaks it, quietly otherwise
	if limit := i18n.T("A routine runs at most every 15 minutes."); w.reading.problem != limit {
		rows = append(rows, look.Paragraph(th, th.T.Caption, th.R.Fg3, limit))
	}
	children := make([]layout.FlexChild, 0, 2*len(rows))
	for index, row := range rows {
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout))
		}
		children = append(children, layout.Rigid(row))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (w *whenBuilder) configure() {
	w.preset.Form = look.SelectQuiet
	w.preset.Label = i18n.T("Runs")
	w.minute.Min, w.minute.Max, w.minute.Step = 0, 59, 5
	w.minute.Unit = i18n.T("minutes past the hour")
	w.day.Min, w.day.Max, w.day.Step = 1, 28, 1
	w.day.Unit = i18n.T("day of the month")
	w.clock.Label, w.clock.Placeholder = i18n.T("At"), "09:00"
	w.custom.Label, w.custom.Placeholder, w.custom.Mono = i18n.T("Schedule"), "30 8 * * 1-5", true
	w.custom.Help = i18n.T("minute hour day month weekday, such as 30 8 * * 1-5")
}

func (w *whenBuilder) clickDays(gtx layout.Context) {
	for index := range w.weekday {
		if w.weekday[index].Clicked(gtx) {
			w.plan.days[index] = !w.plan.days[index]
		}
	}
}

// a new preset keeps the time; Custom starts from the schedule the preset made
func (w *whenBuilder) layoutPreset(gtx layout.Context, e *env.Env, expression string) layout.Dimensions {
	dimensions := w.preset.Layout(gtx, e.Theme, e.Overlays, presetLabels(), int(w.plan.preset))
	if index, changed := w.preset.Changed(); changed && preset(index) != w.plan.preset {
		if preset(index) == custom && expression != "" {
			w.custom.SetText(expression)
		}
		w.plan.preset = preset(index)
	}
	return dimensions
}

// the fields the preset needs
func (w *whenBuilder) fields(th *look.Theme) []layout.Widget {
	clock := func(gtx layout.Context) layout.Dimensions {
		w.clock.Error = ""
		if _, _, ok := readClock(w.clock.Text()); !ok && w.clock.Text() != w.clockSet && !gtx.Focused(&w.clock.Editor) {
			w.clock.Error = i18n.T("Write the time as HH:MM, such as 09:00.")
		}
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(160))
		return w.clock.Layout(gtx, th.Theme)
	}
	stepper := func(stepper *kit.Stepper) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions { return stepper.Layout(gtx, th.Theme) }
	}
	switch w.plan.preset {
	case everyHour:
		return []layout.Widget{stepper(&w.minute)}
	case everyDay, everyWeekday:
		return []layout.Widget{clock}
	case everyWeek:
		return []layout.Widget{w.layoutDays(th), clock}
	case everyMonth:
		return []layout.Widget{stepper(&w.day), clock}
	}
	return []layout.Widget{func(gtx layout.Context) layout.Dimensions { return w.custom.Layout(gtx, th.Theme) }}
}

// the days of the week as chips, any number of them on
func (w *whenBuilder) layoutDays(th *look.Theme) layout.Widget {
	labels := weekdayLabels()
	return func(gtx layout.Context) layout.Dimensions {
		return look.Flow{Gap: 4, LineGap: 4}.Layout(gtx, len(w.weekday), func(gtx layout.Context, index int) layout.Dimensions {
			chip := &w.weekday[index]
			chip.Small, chip.Label, chip.Kind = true, labels[index], kit.PlaqueTonal
			if w.plan.days[index] {
				chip.Kind = kit.PlaqueOn
			}
			return chip.Layout(gtx, th.Theme)
		})
	}
}
