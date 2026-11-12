package memory

import (
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/memoryui"
	"ryoku-rashin/internal/protocol"
)

// the plaques the empty states offer
type emptyActs struct {
	turnOn, importing, add, projects, teams, clear, retry, settings kit.Plaque
	// why the last Try again failed, shown under the empty state
	failed string
}

func (acts *emptyActs) handle(gtx layout.Context, e *env.Env, p *Page) {
	if acts.turnOn.Clicked(gtx) {
		e.SetSettings(map[string]any{"memory": map[string]any{"asked": true, "use": true, "learn": true}}, func(err error) {
			if err != nil {
				e.Report(err)
				return
			}
			e.Notify(i18n.T("Memory is on. Rashin starts with your next conversation."))
		})
	}
	if acts.importing.Clicked(gtx) {
		p.importing.Open(p.place.Scope)
	}
	if acts.add.Clicked(gtx) {
		p.AddMemory()
	}
	if acts.projects.Clicked(gtx) {
		e.Navigate(protocol.TabProjects, "")
	}
	if acts.teams.Clicked(gtx) && e.NavigateTeams != nil {
		e.NavigateTeams(env.TeamsPlace{Tab: protocol.TabTeams})
	}
	if acts.clear.Clicked(gtx) {
		p.search.SetText("")
	}
	if acts.retry.Clicked(gtx) && !acts.retry.Busy {
		acts.retry.Busy = true
		var result protocol.MemoryEngineTest
		e.Call(protocol.MMemoryEngineTest, protocol.MemoryEngineParams{Engine: memoryStatus(e).Engine}, &result, func(err error) {
			switch {
			case err != nil:
				acts.retry.Busy, acts.failed = false, err.Error()
			case !result.OK:
				acts.retry.Busy, acts.failed = false, result.Detail
			default:
				// the test publishes nothing, so the page asks for the status the engine now has
				acts.failed = ""
				var status protocol.MemoryStatus
				e.Call(protocol.MMemoryStatus, protocol.Empty{}, &status, func(err error) {
					acts.retry.Busy = false
					if err != nil {
						e.Report(err)
						return
					}
					e.Store.SetMemoryStatus(status)
				})
			}
		})
	}
	if acts.settings.Clicked(gtx) {
		e.OpenSettings("memory")
	}
}

func (acts *emptyActs) plaque(plaque *kit.Plaque, kind kit.PlaqueKind, label string) *kit.Plaque {
	plaque.Kind, plaque.Label = kind, label
	return plaque
}

// the page stands empty when memory is off with nothing kept, when nothing was ever remembered, or when the engine can't be reached and nothing is cached
func (p *Page) pageEmpty(e *env.Env, status protocol.MemoryStatus) (layout.Widget, bool) {
	th, acts := e.Theme, &p.empty
	settings := e.Snapshot().Settings.Memory
	stored := status.Counts.All + status.Counts.Suggested + status.Counts.Replaced + status.Counts.Blocked
	var empty kit.Empty
	switch {
	case p.editor.IsOpen() || stored > 0:
		return nil, false
	case status.EngineState == protocol.EngineUnreachable:
		name, address := memoryui.EngineName(status.Engine), settings.Honcho.URL
		if status.Engine == protocol.MemoryHindsight {
			address = settings.Hindsight.URL
		}
		body := i18n.Tf("Rashin couldn't reach %s: %s. Start it, or switch back to Built in.", name, status.EngineDetail)
		if address != "" {
			body = i18n.Tf("Rashin couldn't reach %s at %s: %s. Start it, or switch back to Built in.", name, address, status.EngineDetail)
		}
		if acts.failed != "" {
			body += " " + i18n.Tf("It still doesn't answer: %s", acts.failed)
		}
		empty = kit.Empty{
			Icon:  look.Icon.Warning,
			Title: i18n.Tf("Can't reach %s", name),
			Body:  body,
			Actions: []*kit.Plaque{
				acts.plaque(&acts.retry, kit.PlaqueTonal, i18n.T("Try again")),
				acts.plaque(&acts.settings, kit.PlaqueQuiet, i18n.T("Settings")),
			},
		}
	case !memoryOn(settings):
		empty = kit.Empty{
			Icon:  look.Icon.Memory,
			Title: i18n.T("Memory is off"),
			Body:  i18n.T("Rashin keeps nothing between conversations. Turn it on to have it remember what helps, on this computer."),
			Actions: []*kit.Plaque{
				acts.plaque(&acts.turnOn, kit.PlaqueGold, i18n.T("Turn on memory")),
				acts.plaque(&acts.importing, kit.PlaqueTonal, i18n.T("Import from another assistant")),
			},
		}
	default:
		empty = kit.Empty{
			Icon:  look.Icon.Memory,
			Title: i18n.T("Nothing remembered yet"),
			Body:  i18n.T("Rashin remembers after a conversation goes quiet. You can also tell it: \"Remember that I use fish as my shell.\""),
			Actions: []*kit.Plaque{
				acts.plaque(&acts.add, kit.PlaqueGold, i18n.T("Add memory")),
				acts.plaque(&acts.importing, kit.PlaqueTonal, i18n.T("Import from another assistant")),
			},
		}
	}
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Inset{Top: th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return empty.Layout(gtx, th.Theme) })
	}, true
}

// a filter or a search that shows nothing says what would show there
func (p *Page) layoutListEmpty(gtx layout.Context, e *env.Env) layout.Dimensions {
	th, acts := e.Theme, &p.empty
	empty := kit.Empty{Icon: look.Icon.Memory}
	switch query := p.search.Text(); {
	case query != "":
		empty.Icon = look.Icon.Search
		empty.Title = i18n.Tf("Nothing matches \"%s\"", query)
		empty.Body = i18n.T("Try another word, or look under Replaced.")
		empty.Actions = []*kit.Plaque{acts.plaque(&acts.clear, kit.PlaqueQuiet, i18n.T("Clear search"))}
	case p.place.Filter == env.MemoryAbout:
		empty.Title = i18n.T("Nothing about you yet")
		empty.Body = i18n.T("Things you tell Rashin about yourself and how you like answers show here.")
		empty.Actions = []*kit.Plaque{acts.plaque(&acts.add, kit.PlaqueTonal, i18n.T("Add memory"))}
	case p.place.Filter == env.MemoryProjects:
		empty.Title = i18n.T("No project memories yet")
		empty.Body = i18n.T("When you work in a project, what Rashin learns about it stays with that project.")
		empty.Actions = []*kit.Plaque{acts.plaque(&acts.projects, kit.PlaqueTonal, i18n.T("Open Projects"))}
	case p.place.Filter == env.MemoryTeams:
		empty.Title = i18n.T("No team memories yet")
		empty.Body = i18n.T("Each team remembers between its runs. What it learns shows here and on the team's page.")
		empty.Actions = []*kit.Plaque{acts.plaque(&acts.teams, kit.PlaqueTonal, i18n.T("Open Teams"))}
	case p.place.Filter == env.MemorySuggested:
		empty.Title = i18n.T("Nothing waiting")
		empty.Body = i18n.T("Suggestions from teams, background agents and imports wait here until you keep or dismiss them.")
	case p.place.Filter == env.MemoryReplaced:
		empty.Title = i18n.T("Nothing replaced")
		empty.Body = i18n.Tf("When a newer fact replaces an older one, the older one waits here for %d days in case you want it back.", protocol.MemoryReplacedDays)
	default:
		empty.Title = i18n.T("Nothing remembered yet")
		empty.Body = i18n.T("Rashin remembers after a conversation goes quiet.")
	}
	return layout.Inset{Top: th.Sp.S4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return empty.Layout(gtx, th.Theme) })
}
