package settings

import (
	"slices"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	i18n "ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// oniMCPServe is the MCP bridge's group: its switch, a switch per tool and
// the programs approved or waiting, read from the daemon so it says what
// is connected now.
type oniMCPServe struct {
	enabled kit.Toggle
	tools   map[string]*kit.Toggle
	refresh kit.Plaque
	// one Approve or Remove per program, by name
	answer map[string]*kit.Plaque

	status  protocol.OniMCPStatus
	loaded  bool
	loading bool
	problem string
}

// read asks the daemon what the bridge is now.
func (m *oniMCPServe) read(e *env.Env) {
	if m.loading {
		return
	}
	m.loading = true
	result := new(protocol.OniMCPStatus)
	e.Call(protocol.MOniMCP, protocol.Empty{}, result, func(err error) {
		m.loading, m.loaded = false, true
		m.problem = ""
		if err != nil {
			m.problem = err.Error()
			return
		}
		m.status = *result
	})
}

// forget drops what was read, so coming back reads it again.
func (m *oniMCPServe) forget() {
	m.loaded, m.status, m.problem = false, protocol.OniMCPStatus{}, ""
}

// toolTitle and toolDetail say what each tool lets an approved program do.
func mcpToolTitle(id string) string {
	switch id {
	case protocol.OniMCPRemindersList:
		return i18n.T("List reminders")
	case protocol.OniMCPRemindersAdd:
		return i18n.T("Set reminders")
	case protocol.OniMCPNotesSearch:
		return i18n.T("Search memory about you")
	case protocol.OniMCPNotesAdd:
		return i18n.T("Suggest notes")
	case protocol.OniMCPFocusStatus:
		return i18n.T("Read the focus guard")
	case protocol.OniMCPNotify:
		return i18n.T("Show a line on Oni")
	case protocol.OniMCPScreenAsk:
		return i18n.T("Ask about the perched window")
	}
	return id
}

func mcpToolDetail(id string) string {
	switch id {
	case protocol.OniMCPRemindersList:
		return i18n.T("Reads your reminders and timers with their words. Needs reminders on.")
	case protocol.OniMCPRemindersAdd:
		return i18n.T("Sets a reminder or timer within the usual limits. It never cancels one. Needs reminders on.")
	case protocol.OniMCPNotesSearch:
		return i18n.T("Searches what Hyakki remembers about you; sensitive notes stay hidden. Needs memory on.")
	case protocol.OniMCPNotesAdd:
		return i18n.T("A note waits in the Inbox, marked with the program's name, until you keep or dismiss it. Needs memory on.")
	case protocol.OniMCPFocusStatus:
		return i18n.T("Says whether the focus guard is on and for how long. It never starts or stops it.")
	case protocol.OniMCPNotify:
		return i18n.T("One short line in Oni's bubble, headed with the program's name; at most three a minute, no links.")
	case protocol.OniMCPScreenAsk:
		return i18n.T("Oni asks you on screen before any picture is sent; the program gets only the answer's words. Needs looking at windows on.")
	}
	return ""
}

// oniMCPRows are the bridge's settings: the switch, the tools and the
// programs. Only the app changes them.
func (p *Page) oniMCPRows(gtx layout.Context, e *env.Env, oni, defaults protocol.OniSettings) []look.Setting {
	th := e.Theme
	m := &p.oni.mcp
	if !m.loaded {
		m.read(e)
	}
	if m.answer == nil {
		m.answer, m.tools = map[string]*kit.Plaque{}, map[string]*kit.Toggle{}
	}
	settings := oni.MCP
	m.enabled.Value = settings.Enabled
	enabled := i18n.T("Serve Oni's tools to other programs")
	detail := i18n.T("Off. A program you add hyakki mcp to, such as a coding agent, can use the tools you turn on below once you approve it by name. It runs over the program's own pipe; nothing listens on the network.")
	if settings.Enabled {
		detail = i18n.T("On. Each program that starts hyakki mcp waits below until you approve it.")
	}
	rows := []look.Setting{{Title: enabled, Detail: detail,
		Changed: settings.Enabled != defaults.MCP.Enabled, Reset: func() {
			set(e, "oni", map[string]any{"mcp": map[string]any{"enabled": defaults.MCP.Enabled}})
			m.forget()
		},
		Control: func(gtx layout.Context) layout.Dimensions {
			return namedSwitch(gtx, th, enabled, &m.enabled, func(on bool) {
				set(e, "oni", map[string]any{"mcp": map[string]any{"enabled": on}})
				m.forget()
			})
		}}}
	off := ""
	if !settings.Enabled {
		off = i18n.T("Turn on Serve Oni's tools to other programs first.")
	}
	for _, id := range protocol.OniMCPTools {
		toggle := m.tools[id]
		if toggle == nil {
			toggle = &kit.Toggle{}
			m.tools[id] = toggle
		}
		toggle.Value = settings.Tools[id]
		title := mcpToolTitle(id)
		rows = append(rows, look.Setting{Title: title, Detail: mcpToolDetail(id), Unavailable: off,
			Changed: settings.Tools[id] != defaults.MCP.Tools[id], Reset: func() {
				set(e, "oni", map[string]any{"mcp": map[string]any{"tools": map[string]any{id: defaults.MCP.Tools[id]}}})
			},
			Control: func(gtx layout.Context) layout.Dimensions {
				if off != "" {
					gtx = gtx.Disabled()
				}
				return namedSwitch(gtx, th, title, toggle, func(on bool) {
					set(e, "oni", map[string]any{"mcp": map[string]any{"tools": map[string]any{id: on}}})
				})
			}})
	}
	clients := m.status.Clients
	if !m.loaded {
		clients = nil
		for _, name := range settings.Clients {
			clients = append(clients, protocol.OniMCPClient{Name: name, State: protocol.OniMCPClientApproved})
		}
	}
	for _, client := range clients {
		name := client.Name
		plaque := m.answer[name]
		if plaque == nil {
			plaque = &kit.Plaque{}
			m.answer[name] = plaque
		}
		plaque.Kind, plaque.Small = kit.PlaqueTonal, true
		state := i18n.T("Approved. Not connected now.")
		label := i18n.T("Remove")
		approve := false
		switch {
		case client.State == protocol.OniMCPClientWaiting:
			state, label, approve = i18n.T("Connected and waiting for your approval. Its calls are refused until then."), i18n.T("Approve"), true
		case client.Connected:
			state = i18n.T("Approved. Connected now.")
		}
		plaque.Label = label
		rows = append(rows, look.Setting{Title: name, Detail: state,
			Control: func(gtx layout.Context) layout.Dimensions {
				if plaque.Clicked(gtx) {
					next := slices.DeleteFunc(slices.Clone(settings.Clients), func(n string) bool { return n == name })
					if approve {
						next = append(next, name)
					}
					set(e, "oni", map[string]any{"mcp": map[string]any{"clients": next}})
					m.forget()
				}
				return look.Named(gtx, label+" "+name, func(gtx layout.Context) layout.Dimensions { return plaque.Layout(gtx, th.Theme) })
			}})
	}
	if len(clients) == 0 {
		rows = append(rows, look.Setting{Title: i18n.T("Programs"), Detail: i18n.T("None yet. Add hyakki mcp as a stdio server in a program's MCP settings; it shows here when it connects.")})
	}
	if m.problem != "" {
		rows = append(rows, look.Setting{Title: i18n.T("Bridge state"), Detail: m.problem})
	}
	m.refresh.Kind, m.refresh.Small, m.refresh.Label = kit.PlaqueTonal, true, i18n.T("Refresh")
	rows = append(rows, look.Setting{Title: i18n.T("Connected programs"), Detail: i18n.T("Read again what is connected now."),
		Control: func(gtx layout.Context) layout.Dimensions {
			if m.refresh.Clicked(gtx) {
				m.forget()
			}
			return look.Named(gtx, i18n.T("Refresh the connected programs"), func(gtx layout.Context) layout.Dimensions { return m.refresh.Layout(gtx, th.Theme) })
		}})
	return rows
}
