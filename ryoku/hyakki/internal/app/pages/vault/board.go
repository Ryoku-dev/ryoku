package vault

import (
	"time"

	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/app/pages/teamsui"
	"ryoku-rashin/internal/app/state"
	"ryoku-rashin/internal/protocol"
)

// the vault with nothing selected: its state, what waits on the user, the
// credentials, the mail account and what used them lately
type board struct {
	header look.PaneHeader
	scroll look.Scroller
	sheet  look.Sheet
	rows   int
	// the credentials teams asked for, answered here on the Inbox's cards
	requests teamsui.NeedCards
	waiting  []teamsui.Need
	tablets  map[string]*tablet
	mail     mailSlab
	recent   recent
}

type boardItem uint8

const (
	itemLead boardItem = iota
	itemRequests
	itemCredentials
	itemMail
	itemRecent
	itemColophon
)

func (b *board) layout(gtx layout.Context, e *env.Env, p *Page) layout.Dimensions {
	snapshot := e.Snapshot()
	vault := snapshot.Vault
	b.recent.ensure(e, "", vault)
	open := vault.State == protocol.VaultOpen
	items := []boardItem{itemLead}
	b.waiting = b.requests.Shown(requested(snapshot))
	if len(b.waiting) > 0 {
		items = append(items, itemRequests)
	}
	if open {
		items = append(items, itemCredentials, itemMail, itemRecent)
	}
	items = append(items, itemColophon)
	b.rows = len(items)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			content := look.PaneHeaderContent{
				Title:    i18n.T("Vault"),
				Actions:  []look.HeaderAction{{Icon: look.Icon.Add, Label: i18n.T("Add credential"), Description: i18n.T("Add credential"), Disabled: !open}},
				Scrolled: b.scroll.Position.First > 0 || b.scroll.Position.Offset > 0,
			}
			dimensions := b.header.Layout(gtx, e.Theme, e.Overlays, content)
			if _, ok := b.header.ActionClicked(); ok {
				p.openAdd(e, protocol.CredentialHeader, nil)
			}
			return dimensions
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return b.sheet.Layout(gtx, e.Theme, look.PageFor(protocol.TabVault), func(gtx layout.Context) layout.Dimensions {
				return b.scroll.Layout(gtx, e.Theme, len(items), func(gtx layout.Context, index int) layout.Dimensions {
					return look.ReadingColumn(gtx, e.Class, func(gtx layout.Context) layout.Dimensions {
						return b.layoutItem(gtx, e, p, snapshot, items[index], index == len(items)-1)
					})
				})
			})
		}),
	)
}

func (b *board) sectionNumber(number int) int {
	if len(b.waiting) > 0 {
		return number + 1
	}
	return number
}

func (b *board) sectionTotal() int { return b.sectionNumber(3) }

// the credential requests among what needs the user
func requested(snapshot state.Snapshot) []teamsui.Need {
	var requests []teamsui.Need
	for _, need := range teamsui.Needs(snapshot) {
		if need.Approval != nil && need.Approval.Kind == protocol.ApprovalRequest {
			requests = append(requests, need)
		}
	}
	return requests
}

func (b *board) layoutItem(gtx layout.Context, e *env.Env, p *Page, snapshot state.Snapshot, item boardItem, last bool) layout.Dimensions {
	th := e.Theme
	inset := layout.Inset{Bottom: th.Sp.S5}
	if last {
		inset.Bottom = th.Sp.S6
	}
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		switch item {
		case itemColophon:
			return b.sheet.Colophon(gtx, th, look.PageFor(protocol.TabVault))
		case itemLead:
			return layout.Inset{Top: th.Sp.S5}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return look.Monument(gtx, th, teamsui.VaultState(snapshot.Vault), leadProof(gtx.Now, snapshot.Vault))
			})
		case itemRequests:
			return b.layoutRequests(gtx, e)
		case itemCredentials:
			return b.layoutCredentials(gtx, e, p, snapshot)
		case itemMail:
			return b.mail.layout(gtx, e, p, snapshot.Vault, b.sectionNumber(2), b.sectionTotal())
		default:
			return b.layoutRecent(gtx, e, snapshot)
		}
	})
}

// credentials teams asked for, each answered by pasting its value here
func (b *board) layoutRequests(gtx layout.Context, e *env.Env) layout.Dimensions {
	th := e.Theme
	children := []layout.FlexChild{
		layout.Rigid(look.SectionHead(th, look.Head{Title: i18n.T("Waiting for you"), Number: 1, Of: b.sectionTotal(), Count: b.requests.Open(b.waiting)})),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
	}
	for index, need := range b.waiting {
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return b.requests.Layout(gtx, e, need)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (b *board) layoutCredentials(gtx layout.Context, e *env.Env, p *Page, snapshot state.Snapshot) layout.Dimensions {
	th := e.Theme
	credentials := sortedCredentials(snapshot.Vault)
	if b.tablets == nil {
		b.tablets = make(map[string]*tablet)
	}
	children := []layout.FlexChild{
		layout.Rigid(look.SectionHead(th, look.Head{Title: i18n.T("Credentials"), Number: b.sectionNumber(1), Of: b.sectionTotal(), Count: len(credentials)})),
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
	}
	if len(credentials) == 0 {
		children = append(children, layout.Rigid(look.Paragraph(th, th.T.Body, th.R.Fg3,
			i18n.T("Add a token or a key your teams need, such as GITHUB_TOKEN. A team that lacks one can ask you for it."))))
	}
	for index, credential := range credentials {
		item := b.tablets[credential.ID]
		if item == nil {
			item = new(tablet)
			b.tablets[credential.ID] = item
		}
		if index > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout))
		}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return item.layout(gtx, e, p, snapshot.Teams, credential)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (b *board) layoutRecent(gtx layout.Context, e *env.Env, snapshot state.Snapshot) layout.Dimensions {
	th := e.Theme
	children := []layout.FlexChild{
		layout.Rigid(look.SectionHead(th, look.Head{Title: i18n.T("Recent use"), Number: b.sectionNumber(3), Of: b.sectionTotal()})),
		layout.Rigid(layout.Spacer{Height: th.Sp.S2}.Layout),
	}
	if len(b.recent.events) == 0 {
		children = append(children, layout.Rigid(kit.Text(th.Theme, th.T.Body, th.R.Fg3, i18n.T("No team has used a credential yet."))))
	}
	for _, event := range b.recent.events {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutEvent(gtx, th, snapshot.Teams, event, true)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// how many credentials teams used this week, then the promise the vault keeps
func leadProof(now time.Time, vault protocol.VaultInfo) string {
	promise := i18n.T("Hyakki mediates HTTP and mail without giving values to agents. A separately trusted Runtime process can read and retain its granted secret.")
	if vault.State != protocol.VaultOpen || len(vault.Credentials) == 0 {
		return promise
	}
	used := 0
	for _, credential := range vault.Credentials {
		if credential.LastUsed != nil && now.Sub(*credential.LastUsed) < 7*24*time.Hour {
			used++
		}
	}
	switch used {
	case 0:
		return i18n.Tf("None used this week. %s", promise)
	case 1:
		return i18n.Tf("One used this week. %s", promise)
	}
	return i18n.Tf("%s used this week. %s", look.CountStart(used), promise)
}
