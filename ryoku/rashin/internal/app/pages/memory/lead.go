package memory

import (
	"time"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// memory is off until the first-run card is answered, and off again whenever Use memory is
func memoryOn(settings protocol.MemorySettings) bool { return settings.Asked && settings.Use }

func leadWords(status protocol.MemoryStatus, settings protocol.MemorySettings) (claim, proof string) {
	counts := status.Counts
	if !memoryOn(settings) {
		return i18n.T("Memory is off."), i18n.T("What Rashin remembers stays here until you forget it.")
	}
	claim = i18n.Tf("Rashin remembers %s things.", look.Count(counts.All))
	switch counts.All {
	case 0:
		claim = i18n.T("Rashin remembers nothing yet.")
	case 1:
		claim = i18n.T("Rashin remembers one thing.")
	}
	proof = i18n.Tf("It learned %s this month and you forgot %s.", look.Count(counts.LearnedMonth), look.Count(counts.ForgotMonth))
	if counts.LearnedMonth == 0 && counts.ForgotMonth == 0 {
		proof = i18n.T("Nothing changed this month.")
	}
	switch {
	case counts.Suggested == 1:
		proof += " " + i18n.T("One suggestion waits for you.")
	case counts.Suggested > 1:
		proof += " " + i18n.Tf("%s suggestions wait for you.", look.CountStart(counts.Suggested))
	}
	return claim, proof
}

// the page's one monument, with Add memory as the view's gold beside it
func (p *Page) layoutLead(gtx layout.Context, e *env.Env, status protocol.MemoryStatus, withActs bool) layout.Dimensions {
	th := e.Theme
	claim, proof := leadWords(status, e.Snapshot().Settings.Memory)
	p.add.Kind, p.add.Label = kit.PlaqueGold, i18n.T("Add memory")
	// while the editor is open its Save holds the gold
	if p.editor.IsOpen() {
		p.add.Kind = kit.PlaqueTonal
	}
	p.sees.Kind, p.sees.Label = kit.PlaqueTonal, i18n.T("What Ask sees")
	acts := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.sees.Layout(gtx, th.Theme) }),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.add.Layout(gtx, th.Theme) }),
		)
	}
	monument := func(gtx layout.Context) layout.Dimensions { return look.Monument(gtx, th, claim, proof) }
	if !withActs {
		return monument(gtx)
	}
	if gtx.Constraints.Max.X < gtx.Dp(600) {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(monument),
			layout.Rigid(layout.Spacer{Height: th.Sp.S4}.Layout),
			layout.Rigid(acts),
		)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}.Layout(gtx,
		layout.Flexed(1, monument),
		layout.Rigid(layout.Spacer{Width: th.Sp.S4}.Layout),
		layout.Rigid(acts),
	)
}

// an engraved line of the memories in use at the end of each of the last 30 days
func (p *Page) layoutTrend(gtx layout.Context, e *env.Env, status protocol.MemoryStatus) layout.Dimensions {
	th := e.Theme
	days := status.Days
	if len(days) < 7 {
		started := i18n.T("Rashin started remembering today. The chart starts after a week.")
		if len(days) > 1 {
			started = i18n.Tf("Rashin started remembering %s days ago. The chart starts after a week.", look.Count(len(days)-1))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(kit.Text(th.Theme, th.T.Section, th.R.Fg, i18n.T("Over time"))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg3, started)),
		)
	}
	names := make([]string, len(days))
	values := make([]float64, len(days))
	busiest, learned := "", 0
	for index, day := range days {
		names[index], values[index] = day.Day, float64(day.InUse)
		if day.Learned > learned {
			busiest, learned = day.Day, day.Learned
		}
	}
	return look.Trend{
		Title:  i18n.T("Over time"),
		Proof:  trendProof(days, busiest, learned),
		Values: values,
		Ticks:  look.DayTicks(names),
		Height: unit.Dp(96),
	}.Layout(gtx, th)
}

func trendProof(days []protocol.MemoryDay, busiest string, learned int) string {
	last, first := days[len(days)-1].InUse, days[0].InUse
	proof := i18n.Tf("%d memories in use, %d more than a month ago.", last, last-first)
	switch {
	case last < first:
		proof = i18n.Tf("%d memories in use, %d fewer than a month ago.", last, first-last)
	case last == first:
		proof = i18n.Tf("%d memories in use, as many as a month ago.", last)
	}
	if day, err := time.Parse("2006-01-02", busiest); err == nil && learned > 0 {
		proof += " " + i18n.Tf("The busiest day was %s, when Rashin learned %s.", day.Format("2 January"), look.Count(learned))
	}
	return proof
}

// the list's title, its search and sort, and the chips that filter it
func (p *Page) layoutTools(gtx layout.Context, e *env.Env, status protocol.MemoryStatus) layout.Dimensions {
	th := e.Theme
	p.search.Placeholder, p.search.Hint = i18n.T("Search memory"), "Ctrl F"
	p.filters.Labels = p.filters.Labels[:0]
	p.filters.Counts = p.filters.Counts[:0]
	for _, filter := range env.MemoryFilters {
		p.filters.Labels = append(p.filters.Labels, filterLabel(filter))
		p.filters.Counts = append(p.filters.Counts, filterCount(status.Counts, filter))
	}
	p.sort.Form, p.sort.Label = look.SelectQuiet, i18n.T("Sort")
	sortAt := 0
	for index, sort := range sorts {
		if sort == p.sorting {
			sortAt = index
		}
	}
	controls := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(280))
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return p.search.Layout(gtx, th.Theme)
		}),
		layout.Flexed(1, layout.Spacer{}.Layout),
	}
	// with the sidebar folded the index is gone, so the scopes it listed are picked here
	if !p.listed {
		options, at := p.scopeChoices(status)
		p.scopes.Form, p.scopes.Label = look.SelectQuiet, i18n.T("Applies to")
		controls = append(controls,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				dimensions := p.scopes.LayoutOptions(gtx, th, e.Overlays, options, at)
				if picked, ok := p.scopes.Changed(); ok {
					p.pickScope(e, status, picked)
				}
				return dimensions
			}),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
		)
	}
	controls = append(controls, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return p.sort.Layout(gtx, th, e.Overlays, sortLabels(), sortAt)
	}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(look.SectionTitle(th, i18n.T("What Rashin remembers"), 0)),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, controls...)
		}),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.filters.Layout(gtx, th.Theme) }),
	)
}

func (p *Page) scopeChoices(status protocol.MemoryStatus) ([]look.SelectOption, int) {
	options := []look.SelectOption{{Label: i18n.T("Every scope")}}
	at := 0
	for index, row := range scopeRows(status.Scopes) {
		options = append(options, look.SelectOption{Label: row.Title})
		if "scope:"+p.place.Scope == row.ID {
			at = index + 1
		}
	}
	return options, at
}

func (p *Page) pickScope(e *env.Env, status protocol.MemoryStatus, picked int) {
	rows := scopeRows(status.Scopes)
	if picked <= 0 || picked > len(rows) {
		e.OpenMemory(env.MemoryPlace{Filter: p.place.Filter})
		return
	}
	scope := rows[picked-1].ID[len("scope:"):]
	e.OpenMemory(env.MemoryPlace{Filter: scopeFilter(scope), Scope: scope})
}
