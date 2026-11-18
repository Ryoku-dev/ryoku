package settings

import (
	"encoding/json"
	"strings"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

type mcpActions struct{ edit, check, signin, remove kit.Plaque }
type mcpBlock struct {
	loaded, busy                                     bool
	servers                                          []protocol.MCPServerInfo
	rows                                             map[string]*mcpActions
	add, refresh, save, cancel                       kit.Plaque
	form                                             kit.Dialog
	confirm                                          kit.Confirm
	deleting                                         string
	editing                                          string
	name, transport, command, args, address, headers kit.TextField
	client, auth, token, scopes                      kit.TextField
	enabled                                          kit.Toggle
	list                                             layout.List
	err                                              string
}

func (m *mcpBlock) load(e *env.Env) {
	if m.busy {
		return
	}
	m.loaded, m.busy = true, true
	result := new(protocol.MCPListResult)
	e.Call(protocol.MMCPList, protocol.MCPListParams{}, result, func(err error) {
		m.busy = false
		if err != nil {
			m.err = err.Error()
			return
		}
		m.servers, m.err = result.Servers, ""
	})
}

func infraPlaque(gtx layout.Context, e *env.Env, plaque *kit.Plaque, label string) layout.Dimensions {
	plaque.Kind, plaque.Small, plaque.Label = kit.PlaqueTonal, true, label
	return plaque.Layout(gtx, e.Theme.Theme)
}

func (p *Page) mcpBlocks(gtx layout.Context, e *env.Env, blocks []layout.Widget) []layout.Widget {
	if !e.Feature("mcp") {
		return blocks
	}
	m, th := &p.mcp, e.Theme
	if !m.loaded {
		m.load(e)
	}
	if m.rows == nil {
		m.rows = make(map[string]*mcpActions)
	}
	rows := []look.Setting{{Title: i18n.T("Connect a tool server"), Detail: i18n.T("Ready servers join new agent sessions. Header values and sign-in tokens stay in the vault."), Control: func(gtx layout.Context) layout.Dimensions {
		if m.add.Clicked(gtx) {
			m.edit(protocol.MCPServerInfo{Type: "stdio", Enabled: true})
		}
		if m.refresh.Clicked(gtx) {
			m.load(e)
		}
		return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return infraPlaque(gtx, e, &m.add, i18n.T("Add server")) }),
			layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return infraPlaque(gtx, e, &m.refresh, i18n.T("Refresh servers"))
			}),
		)
	}}}
	for _, server := range m.servers {
		if m.rows[server.ID] == nil {
			m.rows[server.ID] = new(mcpActions)
		}
		a := m.rows[server.ID]
		detail := server.Detail
		if detail == "" {
			detail = server.URL
			if server.Type == "stdio" {
				detail = server.Command
			}
		}
		if len(server.HeaderNames) > 0 {
			detail += i18n.Tf(" · Saved headers: %s", strings.Join(server.HeaderNames, ", "))
		}
		rows = append(rows, look.Setting{Title: server.Name, Detail: detail, Below: true, Control: func(gtx layout.Context) layout.Dimensions {
			if a.edit.Clicked(gtx) {
				m.edit(server)
			}
			if a.check.Clicked(gtx) {
				a.check.Busy = true
				e.Call(protocol.MMCPCheck, protocol.MCPCheckParams{ID: server.ID}, new(protocol.MCPServerInfo), func(err error) { a.check.Busy = false; e.Report(err); m.load(e) })
			}
			if a.signin.Clicked(gtx) {
				result := new(protocol.MCPSignInResult)
				e.Call(protocol.MMCPSignIn, protocol.MCPSignInParams{ID: server.ID}, result, func(err error) {
					if err != nil {
						e.Report(err)
					} else {
						e.Open(result.URL)
					}
				})
			}
			if a.remove.Clicked(gtx) {
				m.deleting = server.ID
				m.confirm.Dialog.Open, m.confirm.Danger = true, true
			}
			word, tone := i18n.T("Failing"), palette.Failed
			switch server.State {
			case "ready":
				word, tone = i18n.Tf("Ready · %d tools", server.Tools), palette.Alive
			case "needs-signin":
				word, tone = i18n.T("Needs sign-in"), palette.Paused
			case "off":
				word, tone = i18n.T("Off"), palette.Paused
			}
			primarySignIn := server.State == "needs-signin" && server.OAuth != nil && server.Enabled
			primary := func(gtx layout.Context) layout.Dimensions {
				button, label := &a.check, i18n.T("Check")
				if primarySignIn {
					button, label = &a.signin, i18n.T("Sign in")
				}
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, kit.Claim(th.Theme, tone, word)),
					layout.Rigid(layout.Spacer{Width: th.Sp.S3}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return infraPlaque(gtx, e, button, label) }),
				)
			}
			secondary := func(gtx layout.Context) layout.Dimensions {
				type action struct {
					button *kit.Plaque
					label  string
				}
				actions := [3]action{{&a.edit, i18n.T("Edit")}}
				count := 1
				if primarySignIn {
					actions[count] = action{&a.check, i18n.T("Check")}
					count++
				} else if server.OAuth != nil && server.Enabled {
					actions[count] = action{&a.signin, i18n.T("Sign in")}
					count++
				}
				actions[count] = action{&a.remove, i18n.T("Delete")}
				count++
				return (look.Flow{Gap: th.Sp.S2, LineGap: th.Sp.S2}).Layout(gtx, count, func(gtx layout.Context, index int) layout.Dimensions {
					return infraPlaque(gtx, e, actions[index].button, actions[index].label)
				})
			}
			if gtx.Constraints.Max.X < gtx.Dp(560) {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(primary),
					layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
					layout.Rigid(secondary),
				)
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, primary),
				layout.Rigid(layout.Spacer{Width: th.Sp.S5}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Max.X = max(0, gtx.Constraints.Max.X-gtx.Dp(300))
					return secondary(gtx)
				}),
			)
		}})
	}
	return spacedBlocks(th, blocks, teamsGroup(th, i18n.T("MCP servers"), m.err, rows, nil))
}

func (m *mcpBlock) edit(server protocol.MCPServerInfo) {
	m.editing = server.ID
	m.name.SetText(server.Name)
	m.transport.SetText(server.Type)
	m.command.SetText(server.Command)
	m.address.SetText(server.URL)
	args, _ := json.Marshal(server.Args)
	if server.Args == nil {
		args = []byte("[]")
	}
	m.args.SetText(string(args))
	m.headers.SetText("")
	m.client.SetText("")
	m.auth.SetText("")
	m.token.SetText("")
	m.scopes.SetText("")
	if server.OAuth != nil {
		m.client.SetText(server.OAuth.ClientID)
		m.auth.SetText(server.OAuth.AuthURL)
		m.token.SetText(server.OAuth.TokenURL)
		m.scopes.SetText(server.OAuth.Scopes)
	}
	m.enabled.Value, m.form.Open = server.Enabled, true
	m.list.Axis = layout.Vertical
	m.err = ""
}

func (m *mcpBlock) overlay(gtx layout.Context, e *env.Env) bool {
	if !e.Feature("mcp") {
		m.form.Open, m.confirm.Dialog.Open = false, false
		m.headers.SetText("")
		return false
	}
	if m.confirm.Dialog.Open {
		if m.confirm.Layout(gtx, e.Theme.Theme, i18n.T("Delete this server?"), i18n.T("Its connection details, saved headers and sign-in are removed. Existing sessions keep their current connection until they stop."), i18n.T("Delete server"), "") {
			e.Call(protocol.MMCPDelete, protocol.MCPDeleteParams{ID: m.deleting}, new(protocol.Empty), func(err error) { e.Report(err); m.load(e) })
		}
	}
	if !m.form.Open {
		m.headers.SetText("")
		return m.confirm.Dialog.Open
	}
	fields := []struct {
		field      *kit.TextField
		name, help string
	}{
		{&m.name, i18n.T("Server name"), ""}, {&m.transport, i18n.T("Transport"), i18n.T("stdio, http or sse")},
		{&m.command, i18n.T("Command"), i18n.T("For stdio servers")}, {&m.args, i18n.T("Arguments"), i18n.T("A JSON list, such as [\"serve\"]")},
		{&m.address, i18n.T("Server URL"), i18n.T("For http and sse servers")}, {&m.headers, i18n.T("New header values"), i18n.T("JSON object. Blank keeps saved headers. An empty value removes a header. Saved values are never shown again.")},
		{&m.client, i18n.T("OAuth client ID"), i18n.T("Leave blank when sign-in is not needed")}, {&m.auth, i18n.T("Authorization URL"), ""},
		{&m.token, i18n.T("Token URL"), ""}, {&m.scopes, i18n.T("Scopes"), i18n.T("Separate scopes with spaces")},
	}
	m.headers.Secret = true
	if m.save.Clicked(gtx) && !m.save.Busy {
		p := protocol.MCPServer{ID: m.editing, Name: m.name.Text(), Type: strings.TrimSpace(m.transport.Text()), Command: m.command.Text(), URL: m.address.Text(), Enabled: m.enabled.Value}
		if err := json.Unmarshal([]byte(m.args.Text()), &p.Args); err != nil {
			m.err = i18n.T("Arguments must be a JSON list of strings.")
		} else if m.headers.Text() != "" && json.Unmarshal([]byte(m.headers.Text()), &p.Headers) != nil {
			m.err = i18n.T("Headers must be a JSON object of strings.")
		} else {
			if m.client.Text() != "" {
				p.OAuth = &protocol.MCPOAuth{ClientID: m.client.Text(), AuthURL: m.auth.Text(), TokenURL: m.token.Text(), Scopes: m.scopes.Text()}
			}
			m.save.Busy = true
			e.Call(protocol.MMCPSave, p, new(protocol.MCPServerInfo), func(err error) {
				m.save.Busy = false
				m.headers.SetText("")
				if err != nil {
					m.err = err.Error()
					return
				}
				m.form.Open = false
				m.load(e)
			})
		}
	}
	if m.cancel.Clicked(gtx) {
		m.form.Dismiss(gtx)
		m.headers.SetText("")
	}
	m.form.Width = unit.Dp(560)
	m.form.Layout(gtx, e.Theme.Theme, i18n.T("Tool server"), func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(460))
		return m.list.Layout(gtx, len(fields)+2, func(gtx layout.Context, index int) layout.Dimensions {
			if index == len(fields) {
				return namedSwitch(gtx, e.Theme, i18n.T("Enable server"), &m.enabled, func(bool) {})
			}
			if index > len(fields) {
				return look.Paragraph(e.Theme, e.Theme.T.Detail, e.Theme.R.Fg2, m.err)(gtx)
			}
			field := fields[index]
			field.field.Label, field.field.Help = field.name, field.help
			return layout.Inset{Bottom: e.Theme.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return field.field.Layout(gtx, e.Theme.Theme) })
		})
	}, func(gtx layout.Context) layout.Dimensions { return infraPlaque(gtx, e, &m.save, i18n.T("Save server")) }, func(gtx layout.Context) layout.Dimensions { return infraPlaque(gtx, e, &m.cancel, i18n.T("Cancel")) })
	return m.form.Open
}
