package vault

import (
	"encoding/json"
	"slices"
	"strings"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/hostrule"
	"ryoku-rashin/internal/protocol"
)

// one credential's page: what it is, where it may go, who may use it, the
// hosts teams may send it to without asking, and its own use record
type detail struct {
	header look.PaneHeader
	scroll look.Scroller
	rise   look.Rise
	recent recent

	// the credential as the form was filled from it
	base           protocol.Credential
	seeded         bool
	purpose, hosts kit.TextField
	teams          map[string]*kit.Toggle
	trust          map[protocol.CredentialTrust]*kit.IconPlaque
	save, discard  kit.Plaque
	busy           bool
}

type detailItem uint8

const (
	itemForm detailItem = iota
	itemTrust
	itemUse
)

func (d *detail) layout(gtx layout.Context, e *env.Env, p *Page, credential protocol.Credential) layout.Dimensions {
	snapshot := e.Snapshot()
	d.recent.ensure(e, credential.ID, snapshot.Vault)
	if !d.busy && d.stale(snapshot.Teams, credential) {
		d.seed(credential)
	}
	items := []detailItem{itemForm, itemTrust, itemUse}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.layoutHeader(gtx, e, p, credential) }),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return d.rise.Layout(gtx, e.Theme, credential.ID, 8, func(gtx layout.Context) layout.Dimensions {
				return d.scroll.Layout(gtx, e.Theme, len(items), func(gtx layout.Context, index int) layout.Dimensions {
					return look.ReadingColumn(gtx, e.Class, func(gtx layout.Context) layout.Dimensions {
						return d.layoutItem(gtx, e, snapshot, credential, items[index])
					})
				})
			})
		}),
	)
}

// the form follows the credential while nothing in it was changed
func (d *detail) stale(teams []protocol.Team, credential protocol.Credential) bool {
	if !d.seeded || d.base.ID != credential.ID {
		return true
	}
	_, edited := d.patch(teams, d.base)
	return !edited && !sameForm(d.base, credential)
}

func sameForm(a, b protocol.Credential) bool {
	return a.Description == b.Description && slices.Equal(a.Hosts, b.Hosts) && slices.Equal(a.Teams, b.Teams)
}

func (d *detail) seed(credential protocol.Credential) {
	d.base, d.seeded = credential, true
	d.purpose.SetText(credential.Description)
	d.hosts.SetText(strings.Join(credential.Hosts, " "))
	d.teams = make(map[string]*kit.Toggle)
	for _, team := range credential.Teams {
		d.teams[team] = &kit.Toggle{Value: true}
	}
}

func (d *detail) layoutHeader(gtx layout.Context, e *env.Env, p *Page, credential protocol.Credential) layout.Dimensions {
	facts := make([]look.HeaderFact, 0, len(credential.Hosts)+3)
	for _, host := range credential.Hosts {
		facts = append(facts, look.HeaderFact{Text: host, Kind: look.FactToken})
	}
	if credential.Use == protocol.CredentialMail {
		facts = append(facts, look.HeaderFact{Text: i18n.T("Mail account password")})
	}
	facts = append(facts,
		look.HeaderFact{Text: teamCount(len(credential.Teams))},
		look.HeaderFact{Text: i18n.Tf("fp %s", credential.Fingerprint), Kind: look.FactToken},
		look.HeaderFact{Text: i18n.Tf("version %d", credential.Version)},
	)
	content := look.PaneHeaderContent{
		Title:    credential.Name,
		Facts:    facts,
		Actions:  []look.HeaderAction{{Icon: look.Icon.Key, Label: i18n.T("Replace value"), Description: i18n.T("Replace value")}},
		Menu:     []kit.MenuItem{{Icon: look.Icon.Delete, Label: i18n.T("Remove credential"), Danger: true}},
		Scrolled: d.scroll.Position.First > 0 || d.scroll.Position.Offset > 0,
	}
	dimensions := d.header.Layout(gtx, e.Theme, e.Overlays, content)
	if _, ok := d.header.ActionClicked(); ok {
		p.replace.open(credential)
	}
	if _, ok := d.header.MenuClicked(); ok {
		p.remove.open(credential)
	}
	return dimensions
}

func (d *detail) layoutItem(gtx layout.Context, e *env.Env, snapshot state.Snapshot, credential protocol.Credential, item detailItem) layout.Dimensions {
	th := e.Theme
	top := th.Sp.S4
	if item == itemForm {
		top = th.Sp.S5
	}
	return layout.Inset{Top: top, Bottom: th.Sp.S3}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		switch item {
		case itemForm:
			return d.layoutForm(gtx, e, snapshot.Teams, credential)
		case itemTrust:
			return d.layoutTrust(gtx, e, snapshot.Teams, credential)
		default:
			children := []layout.FlexChild{
				layout.Rigid(look.SectionTitle(th, i18n.T("Its use"), 0)),
				layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			}
			if len(d.recent.events) == 0 {
				children = append(children, layout.Rigid(kit.Text(th.Theme, th.T.Body, th.R.Fg3, i18n.T("No team has used it yet."))))
			}
			for _, event := range d.recent.events {
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layoutEvent(gtx, th, snapshot.Teams, event, false)
				}))
			}
			return layout.Inset{Bottom: th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
			})
		}
	})
}

// what it is for, where it may go and which teams may use it, saved together
func (d *detail) layoutForm(gtx layout.Context, e *env.Env, teams []protocol.Team, credential protocol.Credential) layout.Dimensions {
	th := e.Theme
	header := credential.Use == protocol.CredentialHeader
	patch, changed := d.patch(teams, credential)
	hosts := hostrule.Split(d.hosts.Text())
	ready := changed && !d.busy && (!header || len(hosts) > 0)
	d.save.Kind, d.save.Label, d.save.Busy = kit.PlaqueGold, i18n.T("Save"), d.busy
	d.discard.Kind, d.discard.Label = kit.PlaqueQuiet, i18n.T("Discard changes")
	if d.save.Clicked(gtx) && ready {
		d.submit(e, credential.ID, patch)
	}
	if d.discard.Clicked(gtx) && !d.busy {
		d.seeded = false
	}
	d.purpose.Label, d.purpose.Placeholder = i18n.T("What it is for"), i18n.T("What teams use it for")
	d.hosts.Label, d.hosts.Mono = i18n.T("Where it may go"), true
	d.hosts.Help = i18n.T("Hosts, separated by commas or spaces. *.example.org covers its subdomains.")
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.purpose.Layout(gtx, th.Theme) }),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
	}
	if header {
		children = append(children,
			layout.Rigid(look.Paragraph(th, th.T.Detail, th.R.Fg2, sentAs(credential))),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.hosts.Layout(gtx, th.Theme) }),
			layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
			layout.Rigid(chips(th, hosts)),
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		)
	}
	children = append(children,
		layout.Rigid(kit.Text(th.Theme, th.T.Detail, th.R.Fg2, i18n.T("Which teams may use it"))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
	)
	if len(teams) == 0 {
		children = append(children, layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg3, i18n.T("No team yet."))))
	}
	for _, team := range teams {
		toggle := d.teams[team.ID]
		if toggle == nil {
			toggle = new(kit.Toggle)
			d.teams[team.ID] = toggle
		}
		toggle.Label = team.Name
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: th.Sp.S2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return toggle.Layout(gtx, th.Theme) })
		}))
	}
	if changed {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if !ready {
							gtx = gtx.Disabled()
						}
						return d.save.Layout(gtx, th.Theme)
					}),
					layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.discard.Layout(gtx, th.Theme) }),
				)
			}),
		)
	}
	return kit.SlabContent(gtx, th.Theme, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// how a header credential travels, so the user knows what a host receives
func sentAs(credential protocol.Credential) string {
	header := credential.Header
	if header == "" {
		header = "Authorization"
	}
	if prefix := strings.TrimSpace(credential.Prefix); prefix != "" {
		return i18n.Tf("Sent in the %s header after %s, only over HTTPS to the hosts below.", header, prefix)
	}
	return i18n.Tf("Sent in the %s header, only over HTTPS to the hosts below.", header)
}

// the form's changes as a merge patch, and whether there are any
func (d *detail) patch(teams []protocol.Team, credential protocol.Credential) (map[string]any, bool) {
	patch := make(map[string]any)
	if purpose := strings.TrimSpace(d.purpose.Text()); purpose != credential.Description {
		patch["description"] = purpose
	}
	if credential.Use == protocol.CredentialHeader {
		if hosts := hostrule.Split(d.hosts.Text()); !slices.Equal(hosts, credential.Hosts) {
			patch["hosts"] = hosts
		}
	}
	allowed := make([]string, 0, len(teams))
	for _, team := range teams {
		if toggle := d.teams[team.ID]; toggle != nil && toggle.Value {
			allowed = append(allowed, team.ID)
		}
	}
	kept := make([]string, 0, len(credential.Teams))
	for _, team := range credential.Teams {
		if _, known := teamsui.TeamByID(teams, team); known {
			kept = append(kept, team)
		}
	}
	slices.Sort(allowed)
	slices.Sort(kept)
	if !slices.Equal(allowed, kept) {
		patch["teams"] = allowed
	}
	return patch, len(patch) > 0
}

func (d *detail) submit(e *env.Env, id string, patch map[string]any) {
	raw, err := json.Marshal(patch)
	if err != nil {
		e.Notify(err.Error())
		return
	}
	d.busy = true
	var updated protocol.Credential
	e.Call(protocol.MVaultUpdate, protocol.VaultUpdateParams{ID: id, Patch: raw}, &updated, func(err error) {
		d.busy = false
		if err != nil {
			e.Notify(err.Error())
			return
		}
		d.seed(updated)
	})
}

// the hosts each team may send it to without asking, each one taken back with one press
func (d *detail) layoutTrust(gtx layout.Context, e *env.Env, teams []protocol.Team, credential protocol.Credential) layout.Dimensions {
	th := e.Theme
	if d.trust == nil {
		d.trust = make(map[protocol.CredentialTrust]*kit.IconPlaque)
	}
	children := []layout.FlexChild{
		layout.Rigid(look.SectionTitle(th, i18n.T("Sent without asking"), len(credential.Trusted))),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
	}
	if len(credential.Trusted) == 0 {
		children = append(children, layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg3,
			i18n.T("A team's first use on each host asks you. Answering Always adds that team and host here."))))
	}
	for _, trust := range credential.Trusted {
		remove := d.trust[trust]
		if remove == nil {
			remove = new(kit.IconPlaque)
			d.trust[trust] = remove
		}
		remove.Icon, remove.Small, remove.Kind = look.Icon.Close, true, kit.PlaqueQuiet
		remove.Description = i18n.T("Ask again before sending it there")
		if remove.Clicked(gtx) && !d.busy {
			d.untrust(e, credential, trust)
		}
		team := teamsui.TeamName(teams, trust.Team)
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: 4, Bottom: 4}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
							layout.Rigid(kit.Text(th.Theme, th.T.Body, th.R.Fg, team)),
							layout.Rigid(layout.Spacer{Width: th.Sp.S2}.Layout),
							layout.Rigid(chip(th, trust.Host)),
						)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return remove.Layout(gtx, th.Theme) }),
				)
			})
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (d *detail) untrust(e *env.Env, credential protocol.Credential, gone protocol.CredentialTrust) {
	rest := make([]protocol.CredentialTrust, 0, len(credential.Trusted))
	for _, trust := range credential.Trusted {
		if trust != gone {
			rest = append(rest, trust)
		}
	}
	raw, err := json.Marshal(map[string]any{"trusted": rest})
	if err != nil {
		e.Notify(err.Error())
		return
	}
	e.Call(protocol.MVaultUpdate, protocol.VaultUpdateParams{ID: credential.ID, Patch: raw}, nil, func(err error) {
		if err != nil {
			e.Notify(err.Error())
		}
	})
}
