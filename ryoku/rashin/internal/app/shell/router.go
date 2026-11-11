package shell

import (
	"slices"
	"strings"

	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/layout"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// escape isn't one of these, the frame only asks for it while it has something to close so pages still hear it
var shortcutKeys = []event.Filter{
	key.Filter{Name: "K", Required: key.ModShortcut},
	key.Filter{Name: "N", Required: key.ModShortcut},
	key.Filter{Name: "B", Required: key.ModShortcut},
	key.Filter{Name: "1", Required: key.ModShortcut},
	key.Filter{Name: "2", Required: key.ModShortcut},
	key.Filter{Name: "3", Required: key.ModShortcut},
	key.Filter{Name: "4", Required: key.ModShortcut},
	key.Filter{Name: "5", Required: key.ModShortcut},
	key.Filter{Name: "6", Required: key.ModShortcut},
	key.Filter{Name: "T", Required: key.ModShortcut},
	key.Filter{Name: ",", Required: key.ModShortcut},
	key.Filter{Name: "\\", Required: key.ModShortcut},
	key.Filter{Name: "+", Required: key.ModShortcut},
	key.Filter{Name: "=", Required: key.ModShortcut},
	key.Filter{Name: "-", Required: key.ModShortcut},
	key.Filter{Name: "0", Required: key.ModShortcut},
	key.Filter{Name: key.NameLeftArrow, Required: key.ModAlt},
	key.Filter{Name: key.NameRightArrow, Required: key.ModAlt},
}

var escapeKey = key.Filter{Name: key.NameEscape}

// each mode's destinations in the order Ctrl 1 onwards reaches them
var (
	rashinTabs = []string{protocol.TabAsk, protocol.TabAgent, protocol.TabProjects, protocol.TabIndex}
	teamsTabs  = []string{protocol.TabInbox, protocol.TabTeams, protocol.TabWork, protocol.TabRoutines, protocol.TabVault, protocol.TabActivity}
)

func teamsTab(tab string) bool { return slices.Contains(teamsTabs, tab) }

// every page, each draws the dialogs it owns whichever one is shown
var pageTabs = slices.Concat(rashinTabs, []string{protocol.TabSettings}, teamsTabs)

type Place struct {
	Tab             string
	Session         string
	Project         string
	ProjectView     string
	Worktree        string
	SettingsSection string
	// what a Teams destination has open, as env.TeamsPlace names it
	Team       string
	TeamView   string
	TeamTask   string
	Routine    string
	Credential string
	Draft      string
	Template   string
}

// a project's tabs share one key, and so do a team's
func (place Place) key() string {
	return strings.Join([]string{
		place.Tab, place.Session, place.Project, place.Team, place.TeamTask, place.Routine, place.Credential, place.Draft, place.Template,
	}, "\x00")
}

// sessions of one destination share it, their page moves between them itself and a first question must not blink the pane
func (place Place) arrival() string {
	return strings.Join([]string{place.Tab, place.Project, place.Team, place.TeamTask, place.Routine, place.Credential, place.Draft}, "\x00")
}

type Router struct {
	Tab         string
	Session     string
	NeedsYou    int
	Inbox       int
	PaletteOpen bool
	// a page had a dialog open last frame, so the frame's shortcuts stand down and can't change what's under it
	Modal bool

	invalidate func()
	paletteTag any
	history    []Place
	historyAt  int
	applying   bool
	// Teams mode stands while a Teams destination is shown, settings keeps the mode it was opened from
	teams bool
	// where each mode stood last, so switching goes back there
	lastRashin, lastTeams Place

	OnPlace       func(Place)
	OnNew         func()
	OnToggleFold  func()
	OnTogglePanel func()
	OnZoom        func(int)
	// CanEscape says whether OnEscape would close anything
	CanEscape func() bool
	OnEscape  func() bool
}

func NewRouter(tab, session string, invalidate func()) *Router {
	if !validTab(tab) {
		tab = protocol.TabAsk
	}
	place := Place{Tab: tab, Session: session}
	router := &Router{Tab: tab, Session: session, invalidate: invalidate, history: []Place{place}}
	router.remember(place)
	return router
}

func (r *Router) Current() Place {
	if len(r.history) == 0 {
		return Place{Tab: r.Tab, Session: r.Session}
	}
	return r.history[r.historyAt]
}

// TeamsMode says whether the frame shows Teams' destinations
func (r *Router) TeamsMode() bool { return r.teams }

// goes to where the other mode stood last, Ask or the Inbox the first time
func (r *Router) SwitchMode() {
	place, home := r.lastTeams, protocol.TabInbox
	if r.teams {
		place, home = r.lastRashin, protocol.TabAsk
	}
	if place.Tab == "" {
		place = Place{Tab: home}
	}
	r.Push(place)
}

func (r *Router) remember(place Place) {
	switch {
	case place.Tab == protocol.TabSettings:
	case teamsTab(place.Tab):
		r.teams, r.lastTeams = true, place
	default:
		r.teams, r.lastRashin = false, place
	}
}

func (r *Router) Navigate(tab, session string) {
	place := r.Current()
	if validTab(tab) && tab != place.Tab {
		place.Team, place.TeamView, place.TeamTask, place.Routine, place.Credential, place.Draft, place.Template = "", "", "", "", "", "", ""
		place.Tab = tab
	}
	place.Session = session
	if place.Tab != protocol.TabProjects {
		place.Project, place.ProjectView, place.Worktree = "", "", ""
	}
	r.Push(place)
}

func (r *Router) Push(place Place) {
	if !validTab(place.Tab) {
		place.Tab = protocol.TabAsk
	}
	if current := r.Current(); current == place {
		r.apply(place)
		return
	}
	if r.historyAt+1 < len(r.history) {
		r.history = append([]Place(nil), r.history[:r.historyAt+1]...)
	}
	r.history = append(r.history, place)
	r.historyAt = len(r.history) - 1
	r.apply(place)
}

func (r *Router) Replace(place Place) {
	if !validTab(place.Tab) {
		place.Tab = protocol.TabAsk
	}
	if len(r.history) == 0 {
		r.history = []Place{place}
		r.historyAt = 0
	} else {
		r.history[r.historyAt] = place
	}
	r.apply(place)
}

func (r *Router) CanBack() bool    { return r.historyAt > 0 }
func (r *Router) CanForward() bool { return r.historyAt+1 < len(r.history) }

func (r *Router) Back() bool {
	if !r.CanBack() {
		return false
	}
	r.historyAt--
	r.apply(r.history[r.historyAt])
	return true
}

func (r *Router) Forward() bool {
	if !r.CanForward() {
		return false
	}
	r.historyAt++
	r.apply(r.history[r.historyAt])
	return true
}

func (r *Router) apply(place Place) {
	r.Tab, r.Session = place.Tab, place.Session
	r.remember(place)
	if r.OnPlace != nil && !r.applying {
		r.applying = true
		r.OnPlace(place)
		r.applying = false
	}
	if r.invalidate != nil {
		r.invalidate()
	}
}

func (r *Router) OpenPalette() {
	r.PaletteOpen = true
	if r.invalidate != nil {
		r.invalidate()
	}
}

func (r *Router) Dismiss(overlays *look.Overlays) bool {
	if overlays != nil {
		if tag, ok := overlays.Top(); ok {
			overlays.DismissTop()
			if tag == r.paletteTag {
				r.PaletteOpen = false
			}
			r.invalidateNow()
			return true
		}
	}
	if r.PaletteOpen {
		r.PaletteOpen = false
		r.invalidateNow()
		return true
	}
	if r.OnEscape != nil && r.OnEscape() {
		r.invalidateNow()
		return true
	}
	return false
}

func (r *Router) invalidateNow() {
	if r.invalidate != nil {
		r.invalidate()
	}
}

func (r *Router) HandleKeys(gtx layout.Context, overlays *look.Overlays) {
	menu := false
	if overlays != nil {
		_, menu = overlays.Top()
	}
	var filters []event.Filter
	switch {
	case r.Modal && !menu:
		// a dialog owns the keyboard, escape included
		return
	case r.Modal:
		// only a menu opened from inside the dialog stands above it
		filters = []event.Filter{escapeKey}
	case menu || r.PaletteOpen || (r.CanEscape != nil && r.CanEscape()):
		filters = append(append(filters, shortcutKeys...), escapeKey)
	default:
		filters = shortcutKeys
	}
	for {
		raw, ok := gtx.Event(filters...)
		if !ok {
			return
		}
		e, ok := raw.(key.Event)
		if !ok || e.State != key.Press {
			continue
		}
		shortcut := e.Modifiers.Contain(key.ModShortcut)
		switch {
		case e.Name == "K" && shortcut:
			r.OpenPalette()
		case e.Name == "N" && shortcut && r.OnNew != nil:
			r.OnNew()
		case e.Name == "B" && shortcut && r.OnToggleFold != nil:
			r.OnToggleFold()
		case e.Name == "\\" && shortcut && r.OnTogglePanel != nil:
			r.OnTogglePanel()
		case e.Name == "T" && shortcut:
			r.SwitchMode()
		case shortcut && e.Name >= "1" && e.Name <= "6":
			tabs := rashinTabs
			if r.teams {
				tabs = teamsTabs
			}
			if at := int(e.Name[0] - '1'); at < len(tabs) {
				r.Navigate(tabs[at], "")
			}
		case e.Name == "," && shortcut:
			r.Navigate(protocol.TabSettings, "")
		case (e.Name == "+" || e.Name == "=") && shortcut && r.OnZoom != nil:
			r.OnZoom(10)
		case e.Name == "-" && shortcut && r.OnZoom != nil:
			r.OnZoom(-10)
		case e.Name == "0" && shortcut && r.OnZoom != nil:
			r.OnZoom(0)
		case e.Name == key.NameLeftArrow && e.Modifiers.Contain(key.ModAlt):
			r.Back()
		case e.Name == key.NameRightArrow && e.Modifiers.Contain(key.ModAlt):
			r.Forward()
		case e.Name == key.NameEscape:
			r.Dismiss(overlays)
		}
	}
}

func validTab(tab string) bool {
	return tab == protocol.TabSettings || slices.Contains(rashinTabs, tab) || teamsTab(tab)
}
