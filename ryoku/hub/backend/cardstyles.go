package main

import (
	"maps"
	"slices"
	"strings"
)

// cardStyleOwners says whose each built-in card style is: a style is one badge holder's own look,
// never a choice for every holder, and other holders make theirs through a team design. Keys are
// style ids, values lowercase GitHub logins. team.ryoku.dev keeps the same table
// (CARD_STYLE_OWNERS in release/team-badges/src/codec.js); the shared test vectors compare them.
var cardStyleOwners = map[string][]string{
	"eidolon": {"aethctl"},
	"stand":   {"neur0map"},
}

// styleOwnedBy is whether login, compared without case, owns the built-in style
func styleOwnedBy(style, login string) bool {
	return login != "" && slices.Contains(cardStyleOwners[style], strings.ToLower(login))
}

// stylesOwnedBy lists the built-in styles login owns, in id order
func stylesOwnedBy(login string) []string {
	owned := []string{}
	for _, style := range slices.Sorted(maps.Keys(cardStyleOwners)) {
		if styleOwnedBy(style, login) {
			owned = append(owned, style)
		}
	}
	return owned
}

// ownStyles are the styles this machine's badge may wear: none without a held badge
func ownStyles() []string {
	b, held, _, _ := ownBadge(false)
	if !held {
		return []string{}
	}
	return stylesOwnedBy(b.Login)
}
