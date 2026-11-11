package vault

import (
	"regexp"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/hostrule"
	"ryoku-rashin/internal/protocol"
)

// how a credential is sent, as the dialog offers it
const (
	sentBearer = iota
	sentHeader
	sentMail
)

var credentialName = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,63}$`)

// adds one credential; its value is typed once here and never shown again
type addDialog struct {
	shown   bool
	dialog  kit.Dialog
	scroll  look.Scroller
	sent    look.Select
	how     int
	teams   map[string]*kit.Toggle
	busy    bool
	problem string
	added   func(protocol.Credential)

	name, purpose, value, header, prefix, hosts kit.TextField
	save, cancel                                kit.Plaque
}

func (d *addDialog) open(use string, added func(protocol.Credential)) {
	d.shown, d.dialog.Open, d.dialog.Width = true, true, 600
	d.busy, d.problem, d.added = false, "", added
	d.how = sentBearer
	if use == protocol.CredentialMail {
		d.how = sentMail
	}
	d.clear()
}

// nothing typed outlives the dialog, the value least of all
func (d *addDialog) clear() {
	for _, field := range []*kit.TextField{&d.name, &d.purpose, &d.value, &d.header, &d.prefix, &d.hosts} {
		field.SetText("")
	}
	d.teams = nil
}

func (d *addDialog) layout(gtx layout.Context, e *env.Env) {
	if !d.shown {
		return
	}
	th := e.Theme
	snapshot := e.Snapshot()
	d.upperName()
	params := d.params(snapshot.Teams)
	ready := !d.busy && d.problemWith(params) == ""
	d.cancel.Kind, d.cancel.Label = kit.PlaqueQuiet, i18n.T("Cancel")
	d.save.Kind, d.save.Label, d.save.Busy = kit.PlaqueGold, i18n.T("Add to the vault"), d.busy
	if d.cancel.Clicked(gtx) {
		d.clear()
		d.dialog.Dismiss(gtx)
	}
	if d.save.Clicked(gtx) && ready {
		d.submit(e, params)
	}
	d.dialog.Layout(gtx, th.Theme, i18n.T("Add a credential"),
		func(gtx layout.Context) layout.Dimensions {
			// the form takes what the window gives once the title and the actions have theirs, so the team toggles at its end show
			gtx.Constraints.Max.Y = max(gtx.Dp(240), gtx.Constraints.Max.Y-gtx.Dp(160))
			gtx.Constraints.Min.Y = 0
			rows := d.rows(e, snapshot.Teams, params)
			return d.scroll.Layout(gtx, th, len(rows), func(gtx layout.Context, index int) layout.Dimensions {
				return layout.Inset{Bottom: th.Sp.S3, Right: th.Sp.S3}.Layout(gtx, rows[index])
			})
		},
		func(gtx layout.Context) layout.Dimensions { return d.cancel.Layout(gtx, th.Theme) },
		func(gtx layout.Context) layout.Dimensions {
			if !ready {
				gtx = gtx.Disabled()
			}
			return d.save.Layout(gtx, th.Theme)
		},
	)
	if d.dialog.Closed() {
		d.clear()
		d.shown, d.added = false, nil
	}
}

// names are upper case, so they are made so while typed
func (d *addDialog) upperName() {
	text := d.name.Text()
	if upper := strings.ToUpper(text); upper != text {
		start, end := d.name.Selection()
		d.name.SetText(upper)
		d.name.SetCaret(start, end)
	}
}

func (d *addDialog) rows(e *env.Env, teams []protocol.Team, params protocol.VaultAddParams) []layout.Widget {
	th := e.Theme
	field := func(input *kit.TextField, label, placeholder, help string, mono bool) layout.Widget {
		input.Label, input.Placeholder, input.Help, input.Mono = label, placeholder, help, mono
		return func(gtx layout.Context) layout.Dimensions { return input.Layout(gtx, th.Theme) }
	}
	d.value.Secret = true
	d.sent.Label = i18n.T("How it is sent")
	options := []look.SelectOption{
		{Label: i18n.T("As a bearer token"), Detail: i18n.T("Authorization: Bearer and the value.")},
		{Label: i18n.T("As a header you name"), Detail: i18n.T("A header and an optional prefix before the value.")},
		{Label: i18n.T("As the mail account password"), Detail: i18n.T("Used only to log in to the mail server.")},
	}
	rows := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			d.name.Label, d.name.Placeholder, d.name.Mono = i18n.T("Name"), "GITHUB_TOKEN", true
			d.name.Help, d.name.Error = i18n.T("Agents name it this way; they never see its value."), ""
			// a name is judged once the field is left, not while it is typed
			if params.Name != "" && !credentialName.MatchString(params.Name) && !gtx.Focused(&d.name.Editor) {
				d.name.Error = i18n.T("Capital letters, digits and _, starting with a letter, such as GITHUB_TOKEN.")
			}
			return d.name.Layout(gtx, th.Theme)
		},
		field(&d.purpose, i18n.T("What it is for"), i18n.T("Reads pull requests and issues of my organisation"), "", false),
		field(&d.value, i18n.T("Value"), "", i18n.T("Stored once and never shown again."), true),
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			dimensions := d.sent.LayoutOptions(gtx, th, e.Overlays, options, d.how)
			if index, changed := d.sent.Changed(); changed {
				d.how = index
			}
			return dimensions
		},
	}
	if d.how == sentHeader {
		rows = append(rows,
			field(&d.header, i18n.T("Header name"), "X-Api-Key", "", true),
			field(&d.prefix, i18n.T("Prefix"), "", i18n.T("Stands before the value, such as Token and a space. Leave it empty for none."), true),
		)
	}
	if d.how != sentMail {
		rows = append(rows,
			field(&d.hosts, i18n.T("Where it may go"), "api.github.com", i18n.T("Hosts, separated by commas or spaces. *.example.org covers its subdomains."), true),
			chips(th, params.Hosts),
		)
	}
	rows = append(rows, look.Paragraph(th, th.T.Caption, th.R.Fg3, i18n.T("Which teams may use it")))
	if len(teams) == 0 {
		rows = append(rows, look.Paragraph(th, th.T.Body, th.R.Fg3, i18n.T("No team yet. You can let teams use it later.")))
	}
	if d.teams == nil {
		d.teams = make(map[string]*kit.Toggle)
	}
	for _, team := range teams {
		toggle := d.teams[team.ID]
		if toggle == nil {
			toggle = new(kit.Toggle)
			d.teams[team.ID] = toggle
		}
		toggle.Label = team.Name
		rows = append(rows, func(gtx layout.Context) layout.Dimensions { return toggle.Layout(gtx, th.Theme) })
	}
	// what is missing is said once something was typed, an empty dialog needs no scolding
	if missing := d.problemWith(params); missing != "" && (params.Name != "" || params.Value != "") {
		rows = append(rows, look.Paragraph(th, th.T.Caption, th.R.Fg3, missing))
	}
	if d.problem != "" {
		rows = append(rows, look.Paragraph(th, th.T.Caption, th.R.Amber, d.problem))
	}
	return rows
}

func (d *addDialog) params(teams []protocol.Team) protocol.VaultAddParams {
	params := protocol.VaultAddParams{
		Name:        strings.TrimSpace(d.name.Text()),
		Description: strings.TrimSpace(d.purpose.Text()),
		Use:         protocol.CredentialHeader,
		Value:       d.value.Text(),
	}
	switch d.how {
	case sentBearer:
		params.Header, params.Prefix = "Authorization", "Bearer "
	case sentHeader:
		params.Header, params.Prefix = strings.TrimSpace(d.header.Text()), d.prefix.Text()
	case sentMail:
		params.Use = protocol.CredentialMail
	}
	if params.Use == protocol.CredentialHeader {
		params.Hosts = hostrule.Split(d.hosts.Text())
	}
	for _, team := range teams {
		if toggle := d.teams[team.ID]; toggle != nil && toggle.Value {
			params.Teams = append(params.Teams, team.ID)
		}
	}
	return params
}

// what still keeps the credential from being added, empty when nothing does
func (d *addDialog) problemWith(params protocol.VaultAddParams) string {
	switch {
	case !credentialName.MatchString(params.Name):
		return i18n.T("It needs a name.")
	case strings.TrimSpace(params.Value) == "":
		return i18n.T("It needs a value.")
	case params.Use == protocol.CredentialHeader && params.Header == "":
		return i18n.T("It needs a header name.")
	case params.Use == protocol.CredentialHeader && len(params.Hosts) == 0:
		return i18n.T("It needs at least one host it may go to.")
	}
	return ""
}

func (d *addDialog) submit(e *env.Env, params protocol.VaultAddParams) {
	// the value leaves the field the moment it is sent; a refusal asks for it again
	d.value.SetText("")
	d.busy, d.problem = true, ""
	added := d.added
	var credential protocol.Credential
	e.Call(protocol.MVaultAdd, params, &credential, func(err error) {
		d.busy = false
		if err != nil {
			d.problem = err.Error()
			return
		}
		d.clear()
		d.dialog.Open = false
		if added != nil {
			added(credential)
		}
	})
}
