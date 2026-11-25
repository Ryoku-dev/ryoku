package shell

import (
	"image"
	"io"
	"net/url"
	"strings"
	"time"

	"gioui.org/io/clipboard"
	"gioui.org/layout"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// the places a Ryoku and Hyakki install comes from, shown in the security note
const (
	officialSite = "https://ryoku.dev"
	officialOrg  = "https://github.com/ryoku-dev"
)

// the Ryoku Account: who is signed in and the server's balance, in a popover
// that hangs from the top bar's account icon. It shows only what the daemon
// says; it never keeps a balance of its own, and buying and the subscription
// are on the website Manage account opens.
type accountPanel struct {
	// the top bar's account icon: where the popover hangs and who gets the keyboard back
	opener any
	anchor image.Rectangle
	// the frame's environment, so the popover draws what the store holds now
	env *env.Env
	// wanted is an open the next frame carries out once the icon has a place
	wanted, shown bool

	server                                kit.TextField
	save, signIn, copyCode, openPage, add wideAct
	cancel, signOut, emberKey             kit.Plaque
	refresh                               kit.IconPlaque
	manage, site, org                     rowClick
	rise                                  look.Rise
	problem, opened, copied, account      string
	// the longest the current code was seen to have left, the countdown bar's whole
	span    time.Duration
	spanFor string
}

func (p *accountPanel) open(e *env.Env) {
	p.wanted = true
	p.problem = ""
	p.call(e, protocol.MAccountStatus, func() {
		// a signed-in balance is read again from the server each time the account opens
		if e.Snapshot().Account.State == protocol.AccountSignedIn {
			p.call(e, protocol.MAccountRefresh, nil)
		}
	})
}

// call runs an account method whose answer is the account state
func (p *accountPanel) call(e *env.Env, method string, then func()) {
	var st protocol.AccountState
	e.Call(method, protocol.Empty{}, &st, func(err error) {
		if err != nil {
			p.problem = err.Error()
			return
		}
		e.Store.SetAccount(st)
		if then != nil {
			then()
		}
	})
}

func (p *accountPanel) layout(gtx layout.Context, e *env.Env) {
	p.env = e
	if p.opener == nil {
		return
	}
	open := e.Overlays.IsOpen(p.opener)
	if p.wanted && !p.anchor.Empty() {
		p.wanted = false
		if !open {
			placement := look.Placement{Prefer: look.Below, Align: look.End, Gap: 8, Width: 360, Frame: true, Flush: true, Return: p.opener}
			e.Overlays.Open(p.opener, p.anchor, placement, p.view)
			open = true
		}
	}
	if p.shown && !open {
		p.problem, p.copied = "", ""
	}
	p.shown = open
	if !open {
		return
	}
	st := e.Snapshot().Account
	// another account, or none: nothing of the last one may stay on screen
	who := ""
	if st.Account != nil {
		who = st.Account.ID
	}
	if who != p.account {
		if p.account != "" {
			p.copied = ""
		}
		p.account = who
	}
	// the server field is the only part that takes typing, and only while no server is set
	var holder any
	if st.State == protocol.AccountUnconfigured {
		holder = &p.server.Editor
	}
	e.Overlays.Hold(p.opener, holder)
	p.handle(gtx, e, st)
}

func firstText(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (p *accountPanel) handle(gtx layout.Context, e *env.Env, st protocol.AccountState) {
	if p.save.Clicked(gtx) {
		server := strings.TrimSpace(p.server.Text())
		p.problem = ""
		e.SetSettings(map[string]any{"providers": map[string]any{"account": server}}, func(err error) {
			if err != nil {
				p.problem = err.Error()
				return
			}
			p.call(e, protocol.MAccountRefresh, nil)
		})
	}
	if p.signIn.Clicked(gtx) {
		p.problem = ""
		p.call(e, protocol.MAccountSignIn, nil)
	}
	// the approval page opens once for each code the daemon is waiting on
	if st.State == protocol.AccountSigningIn && st.Code != nil && p.opened != st.Code.UserCode {
		p.opened = st.Code.UserCode
		if page := signInPage(st.Code); page != "" {
			e.Open(page)
		}
	}
	if p.openPage.Clicked(gtx) {
		if page := signInPage(st.Code); page != "" {
			e.Open(page)
		}
	}
	if p.copyCode.Clicked(gtx) && st.Code != nil {
		gtx.Execute(clipboard.WriteCmd{Type: "text/plain", Data: io.NopCloser(strings.NewReader(st.Code.UserCode))})
		p.copied = st.Code.UserCode
		e.Notify(i18n.T("Code copied"))
	}
	if p.cancel.Clicked(gtx) {
		p.call(e, protocol.MAccountCancel, nil)
	}
	if p.refresh.Clicked(gtx) {
		p.problem = ""
		p.call(e, protocol.MAccountRefresh, nil)
	}
	if p.signOut.Clicked(gtx) {
		p.problem = ""
		p.call(e, protocol.MAccountSignOut, nil)
	}
	if p.manage.Clicked(gtx) {
		if page := protocol.AccountManage(st.Manage); page != "" {
			e.Open(page)
		}
	}
	if p.add.Clicked(gtx) {
		if page := protocol.AccountManageAt(st.Manage, protocol.AccountPagePlans); page != "" {
			e.Open(page)
		}
	}
	if p.emberKey.Clicked(gtx) {
		if page := protocol.AccountManageAt(st.Manage, protocol.AccountPageKeys); page != "" {
			e.Open(page)
		}
	}
	if p.site.Clicked(gtx) {
		e.Open(officialSite)
	}
	if p.org.Clicked(gtx) {
		e.Open(officialOrg)
	}
}

// signInPage is the approval page to open, in the form a parser gives back.
// The daemon cleaned it already; anything that is not plain https is never
// opened from here either.
func signInPage(code *protocol.AccountCode) string {
	if code == nil {
		return ""
	}
	for _, raw := range []string{code.VerificationURIComplete, code.VerificationURI} {
		if u, err := url.Parse(raw); err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil {
			return u.String()
		}
	}
	return ""
}

// hostOf is where an address points, for the user to check it against
// where they expect to go
func hostOf(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}
