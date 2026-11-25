package look

import (
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestEverySheetPlaceHasOneNumber(t *testing.T) {
	places := Places()
	if len(places) != 14 {
		t.Fatalf("sheet has %d places, want 14", len(places))
	}
	seen := make(map[string]bool, len(places))
	for index, place := range places {
		if seen[place.Number] {
			t.Errorf("number %q is shared", place.Number)
		}
		seen[place.Number] = true
		if place.Number != []string{"01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11", "12", "13", "14"}[index] {
			t.Errorf("place %d is numbered %q", index, place.Number)
		}
		if place.Title() == "" || place.Gloss == "" || place.PagePlace() != "P "+place.Number+"/14" {
			t.Errorf("incomplete sheet place %+v", place)
		}
		if found, ok := PlaceFor(place.ID); !ok || found != place {
			t.Errorf("cannot find sheet place %+v", place)
		}
	}
	if _, ok := PlaceFor("outside"); ok {
		t.Error("an unknown tab has a place")
	}
	if number, gloss := Numbered("outside"); number != "" || gloss != "" {
		t.Errorf("an unknown tab has number %q and gloss %q", number, gloss)
	}
}

func TestSheetPartAndCrumbs(t *testing.T) {
	page := PartPage(protocol.TabProjects, 2, "Graph", GlossGraph)
	place, _ := PlaceFor(protocol.TabProjects)
	if page.Number != "03.2" || page.Where != Where(place.Title(), "Graph") || page.Gloss != "案・譜" {
		t.Errorf("graph page = %+v", page)
	}
	if got := Crumb("Projects", "", "hyperion", "Graph"); got != "Projects › hyperion › Graph" {
		t.Errorf("crumb = %q", got)
	}
	if got := Where("", "Projects", "Graph", ""); got != "Projects  /  Graph" {
		t.Errorf("where = %q", got)
	}
	if got := JoinGloss("", "案", "", "譜"); got != "案・譜" {
		t.Errorf("gloss = %q", got)
	}
}
