package shell

import (
	"image"
	"testing"

	"gioui.org/io/key"
	"gioui.org/layout"

	"ryoku-rashin/internal/app/chat"
	"ryoku-rashin/internal/app/env"
	"ryoku-rashin/internal/config"
	"ryoku-rashin/internal/protocol"
)

func (s *stage) accountState() protocol.AccountState {
	return s.runtime.store.Snapshot().Account
}

func (s *stage) shows(text string) bool {
	_, ok := s.named(text)
	return ok
}

const accountPanelName = "Account panel"

// the account icon's bounds, found while the popover is closed
func (s *stage) accountIcon() image.Rectangle {
	s.t.Helper()
	node, ok := s.named("Ryoku Account")
	if !ok {
		s.t.Fatal("the top bar has no account icon")
	}
	return node.Desc.Bounds
}

// openAccount clicks the icon and waits for the popover
func (s *stage) openAccount() image.Rectangle {
	s.t.Helper()
	icon := s.accountIcon()
	s.clickAt(centre(icon))
	s.await("the account popover opens", func() bool { return s.shows(accountPanelName) })
	return icon
}

func (s *stage) panelBounds() image.Rectangle {
	s.t.Helper()
	node, ok := s.named(accountPanelName)
	if !ok {
		s.t.Fatal("the account popover is not on screen")
	}
	return node.Desc.Bounds
}

// The popover hangs below the account icon with its right edge on the icon's,
// 8 dp away, and stays inside the window wherever the window is small.
func TestAccountPopoverHangsFromTheIcon(t *testing.T) {
	for _, size := range []image.Point{image.Pt(1440, 900), image.Pt(1000, 700), image.Pt(700, 760)} {
		stage := newStage(t, size)
		icon := stage.openAccount()
		panel := stage.panelBounds()
		if panel.Min.Y != icon.Max.Y+8 {
			t.Errorf("%v: the popover starts at y=%d, want %d under the icon %v", size, panel.Min.Y, icon.Max.Y+8, icon)
		}
		if panel.Max.X != icon.Max.X {
			t.Errorf("%v: the popover's right edge is at x=%d, the icon's at %d", size, panel.Max.X, icon.Max.X)
		}
		if panel.Dx() != 360 {
			t.Errorf("%v: the popover is %d wide, want 360", size, panel.Dx())
		}
		if !panel.In(image.Rectangle{Max: size}) {
			t.Errorf("%v: the popover %v is not inside the window", size, panel)
		}
	}
}

// The storyboard's account route opens the same popover.
func TestAccountRouteOpensThePopoverFromTheIcon(t *testing.T) {
	stage := newStage(t, image.Pt(1280, 820))
	icon := stage.accountIcon()
	stage.open("account")
	stage.await("the account popover opens", func() bool { return stage.shows(accountPanelName) })
	if panel := stage.panelBounds(); panel.Min.Y != icon.Max.Y+8 || panel.Max.X != icon.Max.X {
		t.Errorf("the route opened the popover at %v under the icon %v", panel, icon)
	}
}

// A press anywhere else, Escape, and the icon again each close it, and the
// keyboard goes back to the icon.
func TestAccountPopoverClosesAndReturnsFocusToTheIcon(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	opener := &stage.shell.topBar.account.Clickable
	gone := func() bool { return !stage.shows(accountPanelName) }

	closers := map[string]func(icon image.Rectangle){
		"a press outside": func(image.Rectangle) { stage.clickAt(image.Pt(40, 500)) },
		"escape":          func(image.Rectangle) { stage.press(key.NameEscape, 0) },
		"the icon again":  func(icon image.Rectangle) { stage.clickAt(centre(icon)) },
	}
	for name, closeIt := range closers {
		icon := stage.openAccount()
		closeIt(icon)
		stage.await("the popover closes on "+name, gone)
		if !stage.router.Source().Focused(opener) {
			t.Errorf("after %s the keyboard did not go back to the icon", name)
		}
	}

	// a press inside it keeps it open
	icon := stage.openAccount()
	panel := stage.panelBounds()
	stage.clickAt(image.Pt(panel.Min.X+30, panel.Min.Y+8))
	if gone() {
		t.Error("a press inside the popover closed it")
	}
	stage.clickAt(centre(icon))
	stage.await("the popover closes", gone)
}

// Every view says where Ryoku comes from, with the two official places as
// links that open in the browser.
func TestAccountSecurityNoteLinksToTheOfficialSources(t *testing.T) {
	for _, state := range []string{"signed_out", "signing_in", "signed_in", "unconfigured", "error"} {
		t.Run(state, func(t *testing.T) {
			t.Setenv("HYAKKI_DEMO_ACCOUNT", state)
			stage := newStage(t, image.Pt(1440, 1000))
			stage.openAccount()
			for link, page := range map[string]string{"ryoku.dev": "https://ryoku.dev", "github.com/ryoku-dev": "https://github.com/ryoku-dev"} {
				opened := len(stage.launched)
				stage.click(link)
				stage.await(link+" opens", func() bool { return len(stage.launched) > opened })
				if launched, _ := stage.lastLaunch(); launched != "xdg-open "+page {
					t.Errorf("%s opened %q, want %q", link, launched, "xdg-open "+page)
				}
			}
		})
	}
}

// Signing in shows the code the browser must show, opens the approval page,
// copies the code on request, and cancelling leaves the account signed out.
func TestSigningInShowsTheCodeOpensThePageAndCancels(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.openAccount()
	stage.await("the popover offers sign-in", func() bool { return stage.shows("Sign in to Ryoku") })

	stage.click("Sign in to Ryoku")
	stage.await("the daemon hands out a code", func() bool { return stage.accountState().State == protocol.AccountSigningIn })
	code := stage.accountState().Code
	if launched, _ := stage.lastLaunch(); launched != "xdg-open "+code.VerificationURIComplete {
		t.Errorf("the approval page was opened as %q", launched)
	}
	opened := len(stage.launched)
	if !stage.shows(code.UserCode) {
		t.Fatalf("the code %s is not on screen", code.UserCode)
	}
	if copied := stage.copied("Copy code"); copied != code.UserCode {
		t.Errorf("copying put %q on the clipboard, want %q", copied, code.UserCode)
	}
	if len(stage.launched) != opened {
		t.Error("the approval page opened again on a later frame")
	}
	stage.click("Open page")
	stage.await("Open page opens the approval page again", func() bool { return len(stage.launched) > opened })
	if launched, _ := stage.lastLaunch(); launched != "xdg-open "+code.VerificationURIComplete {
		t.Errorf("Open page opened %q", launched)
	}

	stage.click("Cancel")
	stage.await("cancelling signs nobody in", func() bool { return stage.accountState().State == protocol.AccountSignedOut })
	// Starting again proves cancellation restored an actionable signed-out view,
	// without depending on its explanatory copy.
	stage.click("Sign in to Ryoku")
	stage.await("sign-in is available again", func() bool { return stage.accountState().State == protocol.AccountSigningIn })
}

// A signed-in account shows who is signed in and the server's balance rounded
// down to two places; Manage account opens the website page the server
// names, Refresh reads the balance again, and Sign out forgets everything.
func TestSignedInAccountManagesOnTheWebsite(t *testing.T) {
	t.Setenv("HYAKKI_DEMO_ACCOUNT", "signed_in")
	stage := newStage(t, image.Pt(1440, 1000))
	stage.openAccount()
	stage.await("the balance arrives", func() bool { return stage.shows("187.42") })
	if !stage.shows("ada@example.com") || !stage.shows("ra_demo2b8kq4n7x3h5m9") {
		t.Error("the signed-in identity is not shown")
	}
	manage := stage.accountState().Manage
	if manage != "https://accounts.example.com/account" {
		t.Fatalf("the account names the page %q", manage)
	}
	if !stage.shows("Server") || !stage.shows("accounts.example.com") {
		t.Error("the popover does not show the website and the server it belongs to")
	}
	for _, bought := range []string{"Buy credits", "Payment pending"} {
		if stage.shows(bought) {
			t.Errorf("the popover still offers %q", bought)
		}
	}

	opened := len(stage.launched)
	stage.click("Manage account")
	stage.await("the account page opens in the browser", func() bool { return len(stage.launched) > opened })
	if launched, _ := stage.lastLaunch(); launched != "xdg-open "+manage {
		t.Errorf("Manage account opened %q", launched)
	}

	opened = len(stage.launched)
	stage.click("Get an Ember API key")
	stage.await("key management opens in the system browser", func() bool { return len(stage.launched) > opened })
	if launched, _ := stage.lastLaunch(); launched != "xdg-open "+manage+"#ember-api-keys" {
		t.Errorf("key management opened %q", launched)
	}

	// a stale reading is on screen; Refresh asks the daemon, which asks the server
	stale := stage.accountState()
	stale.Balance = &protocol.AccountBalance{Available: 50_000_000_000, Held: 52_374_000, Read: stale.Balance.Read}
	stage.runtime.store.SetAccount(stale)
	stage.await("the stale reading shows", func() bool { return stage.shows("50.00") && stage.shows("0.05 held for a request in progress") })
	stage.click("Refresh")
	stage.await("Refresh reads the balance again", func() bool { return stage.shows("187.42") && !stage.shows("50.00") })
	if stage.shows("0.05 held for a request in progress") {
		t.Error("the finished request still holds credits on screen")
	}

	stage.click("Sign out")
	stage.await("signing out forgets the account", func() bool { return stage.accountState().State == protocol.AccountSignedOut })
	if stage.shows("ada@example.com") || stage.shows("187.42") {
		t.Error("the signed-out popover still shows the last account")
	}
	if st := stage.accountState(); st.Account != nil || st.Balance != nil {
		t.Errorf("the account state keeps %+v %+v", st.Account, st.Balance)
	}
}

// A server that names no page for the account leaves Manage account off.
func TestManageAccountIsOffWhenTheServerNamesNoPage(t *testing.T) {
	t.Setenv("HYAKKI_DEMO_ACCOUNT", "signed_in")
	stage := newStage(t, image.Pt(1440, 1000))
	stage.openAccount()
	stage.await("the balance arrives", func() bool { return stage.shows("187.42") })
	state := stage.accountState()
	state.Manage = ""
	stage.runtime.store.SetAccount(state)
	stage.await("the popover says there is no page", func() bool { return stage.shows("The server names no page for managing the account.") })
	opened := len(stage.launched)
	stage.click("Manage account")
	if len(stage.launched) != opened {
		t.Error("Manage account opened something with no page named")
	}
	stage.click("Get an Ember API key")
	if len(stage.launched) != opened {
		t.Error("key management opened something with no safe page named")
	}
}

func TestAccountPagePartsPreserveConfiguredAddressAndRejectUnsafeURLs(t *testing.T) {
	for _, test := range []struct{ raw, part, want string }{
		{"https://accounts.example.test/custom/account?mode=private#billing", protocol.AccountPageKeys, "https://accounts.example.test/custom/account?mode=private#ember-api-keys"},
		{"https://accounts.example.test/a%2Fb?next=a%2Fb#old%20anchor", protocol.AccountPageKeys, "https://accounts.example.test/a%2Fb?next=a%2Fb#ember-api-keys"},
		{"https://account.ryoku.dev/account", protocol.AccountPagePlans, "https://account.ryoku.dev/account#plans"},
		{"https://account.ryoku.dev/account#plans", protocol.AccountPageTerms, "https://account.ryoku.dev/account#consent"},
		{"http://localhost:3000/account?test=1", protocol.AccountPageKeys, "http://localhost:3000/account?test=1#ember-api-keys"},
		{"", protocol.AccountPagePlans, ""},
		{"javascript:alert(1)", protocol.AccountPagePlans, ""},
		{"http://accounts.example.test/account", protocol.AccountPagePlans, ""},
		{"https://user:password@accounts.example.test/account", protocol.AccountPageTerms, ""},
		{"/account", protocol.AccountPageKeys, ""},
		{"https://", protocol.AccountPageKeys, ""},
	} {
		if got := protocol.AccountManageAt(test.raw, test.part); got != test.want {
			t.Errorf("AccountManageAt(%q, %q) = %q, want %q", test.raw, test.part, got, test.want)
		}
	}
}

// A signed-in account with no Ember credits leads with Add Ember credits,
// which opens the account page's plans; one with credits does not offer it.
func TestEmptyBalanceLeadsWithAddEmberCredits(t *testing.T) {
	t.Setenv("HYAKKI_DEMO_ACCOUNT", "signed_in_empty")
	stage := newStage(t, image.Pt(1440, 1000))
	stage.openAccount()
	stage.await("the empty balance arrives", func() bool { return stage.shows("0.00") && stage.shows("Add Ember credits") })
	manage := stage.accountState().Manage
	opened := len(stage.launched)
	stage.click("Add Ember credits")
	stage.await("the plans open in the browser", func() bool { return len(stage.launched) > opened })
	if launched, _ := stage.lastLaunch(); launched != "xdg-open "+manage+"#plans" {
		t.Errorf("Add Ember credits opened %q, want %q", launched, "xdg-open "+manage+"#plans")
	}

	funded := stage.accountState()
	funded.Balance = &protocol.AccountBalance{Available: 2_000_000_000, Read: funded.Balance.Read}
	stage.runtime.store.SetAccount(funded)
	stage.await("a balance to spend shows", func() bool { return stage.shows("2.00") })
	if stage.shows("Add Ember credits") {
		t.Error("an account with credits is still asked to add some first")
	}
	if !stage.shows("Get an Ember API key") {
		t.Error("the key page is no longer offered")
	}
}

// On Ryoku's own server the popover names no server: the account page's host
// is the one a user meets. Another server is named, so it can be checked.
func TestServerIsNamedOnlyWhenItIsNotRyokusOwn(t *testing.T) {
	t.Setenv("HYAKKI_DEMO_ACCOUNT", "signed_in")
	stage := newStage(t, image.Pt(1440, 1000))
	stage.openAccount()
	stage.await("the balance arrives", func() bool { return stage.shows("187.42") && stage.shows("Server") })
	own := stage.accountState()
	own.Server, own.Manage = config.DefaultAccountServer, "https://account.ryoku.dev/account"
	stage.runtime.store.SetAccount(own)
	stage.await("the account page's host shows", func() bool { return stage.shows("account.ryoku.dev") })
	if stage.shows("Server") || stage.shows("ai.ryoku.dev") {
		t.Error("the popover names Ryoku's own server")
	}
	if !stage.shows("Sign out") {
		t.Error("Sign out left with the server line")
	}
	stage.click("Sign out")
	stage.await("signed out", func() bool { return stage.accountState().State == protocol.AccountSignedOut })
	signedOut := stage.accountState()
	signedOut.Server = config.DefaultAccountServer
	stage.runtime.store.SetAccount(signedOut)
	stage.await("the signed-out view says what signing in gets", func() bool {
		return stage.shows("Sign in to use Ryoku Ember's hosted models, paid with pay-as-you-go Ember credits.")
	})
	if stage.shows("Server") || stage.shows("ai.ryoku.dev") {
		t.Error("the signed-out popover names Ryoku's own server")
	}
}

// A refusal only the account page answers opens that page's part; with no
// page known it opens the account popover instead.
func TestAccountActionOpensTheAccountPagePart(t *testing.T) {
	t.Setenv("HYAKKI_DEMO_ACCOUNT", "signed_in")
	stage := newStage(t, image.Pt(1440, 1000))
	stage.openAccount()
	stage.await("the balance arrives", func() bool { return stage.shows("187.42") })
	manage := stage.accountState().Manage
	stage.clickAt(image.Pt(40, 500))
	stage.await("the popover closes", func() bool { return !stage.shows(accountPanelName) })
	for part, fragment := range map[string]string{protocol.AccountPagePlans: "#plans", protocol.AccountPageTerms: "#consent"} {
		opened := len(stage.launched)
		env.HandleChatActions(layout.Context{}, stage.shell.environment(), "", []chat.Action{{Kind: chat.ActAccount, Value: part}})
		if len(stage.launched) == opened {
			t.Fatalf("the %s action opened nothing", part)
		}
		if launched, _ := stage.lastLaunch(); launched != "xdg-open "+manage+fragment {
			t.Errorf("the %s action opened %q, want %q", part, launched, "xdg-open "+manage+fragment)
		}
	}

	nameless := stage.accountState()
	nameless.Manage = ""
	stage.runtime.store.SetAccount(nameless)
	opened := len(stage.launched)
	env.HandleChatActions(layout.Context{}, stage.shell.environment(), "", []chat.Action{{Kind: chat.ActAccount, Value: protocol.AccountPagePlans}})
	stage.await("the account popover opens instead", func() bool { return stage.shows(accountPanelName) })
	if len(stage.launched) != opened {
		t.Error("an account with no page named opened something")
	}
}

// A fresh session has not read the account yet; the first refusal action
// reads it and opens the page, rather than the popover.
func TestAccountActionReadsAnUnknownAccountBeforeOpeningThePage(t *testing.T) {
	t.Setenv("HYAKKI_DEMO_ACCOUNT", "signed_in")
	stage := newStage(t, image.Pt(1440, 1000))
	if stage.runtime.store.Snapshot().AccountKnown {
		t.Fatal("the session knew the account before anything asked for it")
	}
	env.HandleChatActions(layout.Context{}, stage.shell.environment(), "", []chat.Action{{Kind: chat.ActAccount, Value: protocol.AccountPageTerms}})
	stage.await("the terms page opens", func() bool { return len(stage.launched) > 0 })
	manage := stage.accountState().Manage
	if launched, _ := stage.lastLaunch(); manage == "" || launched != "xdg-open "+manage+"#consent" {
		t.Errorf("the first action opened %q, want the terms part of %q", launched, manage)
	}
	if stage.shows(accountPanelName) {
		t.Error("the account popover opened as well as the page")
	}
}

// Signed out, the model picker lists Ryoku Ember as a row that opens the
// account popover; picking it leaves the model as it was.
func TestSignedOutEmberRowOpensTheAccountAndIsNeverPicked(t *testing.T) {
	stage := newStage(t, image.Pt(1440, 900))
	stage.open("ask/new")
	stage.await("the model list loads", func() bool { return stage.anyNamed("Model: Auto") != "" })
	stage.click("Model: Auto")
	const row = "Ryoku Ember — sign in to use"
	stage.await("the picker offers Ryoku Ember", func() bool { return stage.pickerButton(row) })
	stage.click(row)
	stage.await("the account popover opens", func() bool { return stage.shows(accountPanelName) })
	if stage.pickerButton(row) {
		t.Error("the model list stayed open")
	}
	if stage.anyNamed("Model: Auto") == "" || stage.anyNamed("Model: "+row) != "" {
		t.Errorf("picking the row changed the model; on screen: %v", stage.names())
	}
	if !stage.shows("Sign in to Ryoku") {
		t.Error("the popover does not offer sign-in")
	}
}

// The signing-in view names the server the account belongs to and the page
// the browser was sent to, so a user can check where they sign in.
func TestSigningInNamesTheServerAndThePage(t *testing.T) {
	t.Setenv("HYAKKI_DEMO_ACCOUNT", "signing_in")
	stage := newStage(t, image.Pt(1440, 1000))
	stage.openAccount()
	stage.await("the sign-in view names both hosts", func() bool {
		return stage.shows("Sign-in page") && stage.shows("Server") && stage.shows("accounts.example.com")
	})
}

// A failure is told inside the view it belongs to, not on a screen of its own.
func TestAccountErrorShowsInsideTheView(t *testing.T) {
	t.Setenv("HYAKKI_DEMO_ACCOUNT", "error")
	stage := newStage(t, image.Pt(1440, 1000))
	stage.openAccount()
	stage.await("the error shows beside the sign-in", func() bool {
		account := stage.accountState()
		return stage.shows(accountPanelName) && account.State == protocol.AccountSignedOut && account.Error != "" && stage.shows(account.Error)
	})
}
