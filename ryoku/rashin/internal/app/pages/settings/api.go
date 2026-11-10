package settings

import (
	"io"
	"net"
	"strconv"
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/layout"
	"gioui.org/widget"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/palette"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// the address and key other programs use to reach the providers through Rashin
type apiPanel struct {
	serve   kit.Toggle
	copyURL look.IconAction
	reveal  look.IconAction
	copyKey look.IconAction
	renew   kit.Plaque
	confirm kit.Confirm
	asking  bool
	port    kit.TextField
	exports kit.Plaque

	// the key while it is shown, empty again once it is hidden
	key string
	// only a frame can reach the clipboard, so text waits here for the next one
	clip, copied string
	// the port the daemon had at the last frame
	portSeen int
}

func (p *Page) layoutAPI(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	a := &p.api
	snapshot := e.Snapshot()
	settings, info := snapshot.Settings.API, snapshot.Status.API
	a.update(gtx, e, settings)
	rows := []look.Setting{{
		Title:  i18n.T("Serve the local API"),
		Detail: i18n.T("Rashin keeps running in the background while this is on, so the address always answers."),
		Control: func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if !settings.Enabled {
						return layout.Dimensions{}
					}
					word, tone := apiState(info)
					return layout.Inset{Right: th.Sp.S3}.Layout(gtx, kit.Claim(th.Theme, tone, word))
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return namedSwitch(gtx, th, i18n.T("Serve the local API"), &a.serve, func(on bool) {
						set(e, "api", map[string]any{"enabled": on})
					})
				}),
			)
		},
	}}
	if settings.Enabled {
		rows = append(rows, a.rows(e, settings, info)...)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(kit.Text(th.Theme, th.T.Section, th.R.Fg, i18n.T("Local API"))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S1}.Layout),
		layout.Rigid(look.Paragraph(th, th.T.Caption, th.R.Fg3,
			i18n.T("Other programs on this machine can use your providers through Rashin: one address and one key, in the shape of OpenAI's API."))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return look.SettingsSlab(gtx, th, rows) }),
	)
}

func apiState(info protocol.APIInfo) (string, palette.Tone) {
	switch {
	case info.Error != "":
		return i18n.T("Not answering"), palette.Failed
	case info.Listening:
		return i18n.T("Answering"), palette.Alive
	}
	return i18n.T("Starting"), palette.Paused
}

func (a *apiPanel) rows(e *env.Env, settings protocol.APISettings, info protocol.APIInfo) []look.Setting {
	th := e.Theme
	address := apiAddress(settings.Port)
	// where it is not answering, the reason stands where the address is read
	detail := i18n.T("Give a program this as its base URL.")
	if info.Error != "" {
		detail = info.Error
	}
	shown := a.key
	if shown == "" {
		shown = "rashin-" + strings.Repeat("•", 24)
	}
	reveal, revealWord := look.Icon.Show, i18n.T("Show the key")
	if a.key != "" {
		reveal, revealWord = look.Icon.Hide, i18n.T("Hide the key")
	}
	a.reveal.Icon, a.reveal.Description = reveal, revealWord
	return []look.Setting{
		{
			Title: i18n.T("Address"), Detail: detail,
			Control: func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(look.TokenChip(th, address)),
					layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.copyURL.Layout(gtx, th, th.R.Fg3) }),
				)
			},
		},
		{
			Title:  i18n.T("Key"),
			Detail: i18n.T("A program sends it with every request. Whoever has it can use your providers from this machine."),
			Below:  true,
			Control: func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(look.TokenChip(th, shown)),
					layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.reveal.Layout(gtx, th, th.R.Fg3) }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return a.copyKey.Layout(gtx, th, th.R.Fg3) }),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return a.renew.Layout(gtx, th.Theme) })
					}),
				)
			},
		},
		{
			Title:  i18n.T("Model name"),
			Detail: i18n.T("auto lets Rashin pick a model that is up. The other two lean towards speed or depth, and a full name from the Ask list gets that model alone."),
			Control: func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(look.TokenChip(th, protocol.RouteAuto)),
					layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
					layout.Rigid(look.TokenChip(th, protocol.RouteFast)),
					layout.Rigid(layout.Spacer{Width: th.Sp.S1}.Layout),
					layout.Rigid(look.TokenChip(th, protocol.RouteSmart)),
				)
			},
		},
		{
			Title:  i18n.T("Port"),
			Detail: i18n.T("Change it when another program already uses this one. Enter applies it."),
			Control: func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.X = gtx.Dp(104)
				return look.Named(gtx, i18n.T("Port"), func(gtx layout.Context) layout.Dimensions {
					return a.port.Layout(gtx, th.Theme)
				})
			},
		},
		{
			Title:  i18n.T("Set up a terminal"),
			Detail: i18n.T("Copies two export lines, OPENAI_BASE_URL and OPENAI_API_KEY, that most tools read."),
			Control: func(gtx layout.Context) layout.Dimensions {
				return a.exports.Layout(gtx, th.Theme)
			},
		},
	}
}

func (a *apiPanel) update(gtx layout.Context, e *env.Env, settings protocol.APISettings) {
	if a.clip != "" {
		gtx.Execute(clipboard.WriteCmd{Type: "text/plain", Data: io.NopCloser(strings.NewReader(a.clip))})
		e.Notify(a.copied)
		a.clip, a.copied = "", ""
	}
	a.serve.Value = settings.Enabled
	if !settings.Enabled {
		a.key = ""
		return
	}
	address := apiAddress(settings.Port)
	a.copyURL.Icon, a.copyURL.Description = look.Icon.Copy, i18n.T("Copy the address")
	a.copyKey.Icon, a.copyKey.Description = look.Icon.Copy, i18n.T("Copy the key")
	a.renew.Kind, a.renew.Small, a.renew.Label = kit.PlaqueQuiet, true, i18n.T("New key")
	a.exports.Kind, a.exports.Small, a.exports.Label, a.exports.Icon = kit.PlaqueQuiet, true, i18n.T("Copy exports"), look.Icon.Copy
	if a.copyURL.Clicked(gtx) {
		a.clip, a.copied = address, i18n.T("Copied the address")
	}
	if a.reveal.Clicked(gtx) {
		if a.key != "" {
			a.key = ""
		} else {
			a.fetch(e, false, func(key string) { a.key = key })
		}
	}
	if a.copyKey.Clicked(gtx) {
		a.fetch(e, false, func(key string) { a.clip, a.copied = key, i18n.T("Copied the key") })
	}
	if a.exports.Clicked(gtx) {
		a.fetch(e, false, func(key string) {
			a.clip = "export OPENAI_BASE_URL=" + address + "\nexport OPENAI_API_KEY=" + key + "\n"
			a.copied = i18n.T("Copied two export lines")
		})
	}
	if a.renew.Clicked(gtx) {
		a.asking, a.confirm.Dialog.Open, a.confirm.Danger = true, true, true
	}
	a.updatePort(gtx, e, settings.Port)
}

// the field shows the port in use unless it is being typed in, so it never claims one that was not applied
func (a *apiPanel) updatePort(gtx layout.Context, e *env.Env, current int) {
	a.port.Mono = true
	a.port.Editor.Filter, a.port.Editor.MaxLen, a.port.Editor.Submit = "0123456789", 5, true
	submitted := false
	for {
		event, ok := a.port.Editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := event.(widget.SubmitEvent); ok {
			submitted = true
		}
	}
	if !submitted {
		// a port set from somewhere else, the command for one, replaces what is half typed
		moved := current != a.portSeen
		if (moved || !gtx.Focused(&a.port.Editor)) && a.port.Text() != strconv.Itoa(current) {
			a.port.SetText(strconv.Itoa(current))
			a.port.Error = ""
		}
		a.portSeen = current
		return
	}
	port, err := strconv.Atoi(a.port.Text())
	if err != nil || port < 1024 || port > 65535 {
		a.port.Error = i18n.T("1024 to 65535")
		return
	}
	a.port.Error = ""
	if port != current {
		set(e, "api", map[string]any{"port": port})
	}
}

func (a *apiPanel) fetch(e *env.Env, rotate bool, then func(key string)) {
	result := new(protocol.APIKey)
	e.Call(protocol.MGatewayAPIKey, protocol.APIKeyParams{Rotate: rotate}, result, func(err error) {
		if err != nil {
			e.Notify(err.Error())
			return
		}
		then(result.Key)
	})
}

func (a *apiPanel) overlay(gtx layout.Context, e *env.Env) bool {
	if !a.asking {
		return false
	}
	body := i18n.T("Every program that uses the key you have now stops working until you give it the new one.")
	if a.confirm.Layout(gtx, e.Theme.Theme, i18n.T("Make a new key?"), body, i18n.T("Make a new key"), "") {
		a.fetch(e, true, func(key string) {
			a.key = key
			e.Notify(i18n.T("The old key no longer works"))
		})
	}
	if a.confirm.Closed() {
		a.asking = false
	}
	return a.confirm.Dialog.Open
}

func apiAddress(port int) string {
	return "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(port)) + "/v1"
}
