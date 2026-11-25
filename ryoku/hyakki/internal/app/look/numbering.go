package look

import (
	"strconv"
	"strings"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/protocol"
)

// The motto at the sheet's foot and the gloss of a project's graph.
const (
	GlossGraph = "譜"
	Motto      = "力と美のために"
)

// SheetPlace identifies one numbered destination in Hyakki's sheet
type SheetPlace struct {
	ID, Number, Gloss string

	order int
}

// one sequence over both modes and the standalone pages, so no two places share a number
var places = func() []SheetPlace {
	rows := []SheetPlace{
		{ID: protocol.TabAsk, Gloss: "問"},
		{ID: protocol.TabAgent, Gloss: "働"},
		{ID: protocol.TabProjects, Gloss: "案"},
		{ID: protocol.TabIndex, Gloss: "索"},
		{ID: protocol.TabInbox, Gloss: "便"},
		{ID: protocol.TabTeams, Gloss: "組"},
		{ID: protocol.TabDreams, Gloss: "夢"},
		{ID: protocol.TabWork, Gloss: "務"},
		{ID: protocol.TabRoutines, Gloss: "暦"},
		{ID: protocol.TabVault, Gloss: "蔵"},
		{ID: protocol.TabActivity, Gloss: "録"},
		{ID: protocol.TabMemory, Gloss: "憶"},
		{ID: protocol.TabLearning, Gloss: "習"},
		{ID: protocol.TabSettings, Gloss: "設"},
	}
	for index := range rows {
		rows[index].order = index + 1
		rows[index].Number = kit.Pad2(index + 1)
	}
	return rows
}()

// kept whole so a masthead at rest builds no string
var pagePlaces = func() []string {
	out := make([]string, len(places)+1)
	for at := 1; at <= len(places); at++ {
		out[at] = "P " + kit.Pad2(at) + "/" + kit.Pad2(len(places))
	}
	return out
}()

const partsKept = 33

var partNumbers = func() [][partsKept]string {
	out := make([][partsKept]string, len(places)+1)
	for _, place := range places {
		for n := 1; n < partsKept; n++ {
			out[place.order][n] = place.Number + "." + strconv.Itoa(n)
		}
	}
	return out
}()

// Places lists every place in the sheet's order.
func Places() []SheetPlace { return places }

// PlaceFor is the place a tab (protocol.Tab*) has on the sheet.
func PlaceFor(tab string) (SheetPlace, bool) {
	for _, place := range places {
		if place.ID == tab {
			return place, true
		}
	}
	return SheetPlace{}, false
}

// Numbered is a tab's number and gloss, both empty for a tab the sheet does
// not know.
func Numbered(tab string) (number, gloss string) {
	place, _ := PlaceFor(tab)
	return place.Number, place.Gloss
}

// Title is the place's name as the sidebar says it.
func (p SheetPlace) Title() string {
	switch p.ID {
	case protocol.TabAsk:
		return i18n.T("Ask")
	case protocol.TabAgent:
		return i18n.T("Agents")
	case protocol.TabProjects:
		return i18n.T("Projects")
	case protocol.TabIndex:
		return i18n.T("Index")
	case protocol.TabInbox:
		return i18n.T("Inbox")
	case protocol.TabTeams:
		return i18n.T("Teams")
	case protocol.TabDreams:
		return i18n.T("Dreams")
	case protocol.TabWork:
		return i18n.T("Work")
	case protocol.TabRoutines:
		return i18n.T("Routines")
	case protocol.TabVault:
		return i18n.T("Vault")
	case protocol.TabActivity:
		return i18n.T("Activity")
	case protocol.TabMemory:
		return i18n.T("Memory")
	case protocol.TabLearning:
		return i18n.T("Learning")
	case protocol.TabSettings:
		return i18n.T("Settings")
	}
	return ""
}

// Part is the number of the place's nth part, from 1: "03.2".
func (p SheetPlace) Part(n int) string {
	if p.order > 0 && n > 0 && n < partsKept {
		return partNumbers[p.order][n]
	}
	if n <= 0 {
		return p.Number
	}
	return p.Number + "." + strconv.Itoa(n)
}

// PagePlace is the place's position on the sheet as a masthead writes it,
// "P 03/14".
func (p SheetPlace) PagePlace() string {
	if p.order <= 0 || p.order >= len(pagePlaces) {
		return ""
	}
	return pagePlaces[p.order]
}

// PageFor is the sheet page of a place itself.
func PageFor(tab string) SheetPage {
	place, _ := PlaceFor(tab)
	return SheetPage{Number: place.Number, Where: place.Title(), Gloss: place.Gloss, Code: place.ID}
}

// PartPage is the sheet page of a place's nth part, from 1: a project's view,
// a section of Settings. name arrives translated; gloss may be empty.
func PartPage(tab string, n int, name, gloss string) SheetPage {
	place, _ := PlaceFor(tab)
	page := SheetPage{
		Number: place.Part(n), Where: Where(place.Title(), name),
		Gloss: JoinGloss(place.Gloss, gloss), Code: place.ID,
	}
	if n > 0 && place.ID != "" {
		page.Code = place.ID + "." + strconv.Itoa(n)
	}
	return page
}

// ProjectView is the place a view of a project (env.View*) has under
// Projects: its part number, its translated name and its gloss.
func ProjectView(view string) (n int, name, gloss string) {
	switch view {
	case "overview":
		return 1, i18n.T("Overview"), ""
	case "graph":
		return 2, i18n.T("Graph"), GlossGraph
	case "changes":
		return 3, i18n.T("Changes"), ""
	case "tasks":
		return 4, i18n.T("Tasks"), ""
	case "activity":
		return 5, i18n.T("Activity"), ""
	}
	return 0, "", ""
}

// Where joins the parts of a page's place as a masthead reads them,
// "Projects  /  Graph"; empty parts are left out.
func Where(parts ...string) string { return joinKept(parts, "  /  ") }

// Crumb joins the parts of a path to a place, "Projects › hyperion › Graph";
// empty parts are left out.
func Crumb(parts ...string) string { return joinKept(parts, " › ") }

// JoinGloss joins glosses with the Hub's middle dot, "案・譜"; empty parts are
// left out.
func JoinGloss(parts ...string) string { return joinKept(parts, "・") }

func joinKept(parts []string, separator string) string {
	kept, only := 0, ""
	for _, part := range parts {
		if part != "" {
			kept++
			only = part
		}
	}
	switch kept {
	case 0:
		return ""
	case 1:
		return only
	}
	var joined strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		if joined.Len() > 0 {
			joined.WriteString(separator)
		}
		joined.WriteString(part)
	}
	return joined.String()
}

// the place a page's number belongs to, "03.2" under "03"
func placeOfNumber(number string) (SheetPlace, bool) {
	if len(number) < 2 {
		return SheetPlace{}, false
	}
	for _, place := range places {
		if place.Number == number[:2] {
			return place, true
		}
	}
	return SheetPlace{}, false
}
