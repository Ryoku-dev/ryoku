package main

import (
	"errors"
	"os"
)

func listCards() ([]binderCard, error) {
	if _, e := os.Stat(cardsDir()); os.IsNotExist(e) {
		return []binderCard{}, nil
	}
	list, e := teamList(false)
	return listCardsWithTeam(list, e)
}
func listCardsOffline() ([]binderCard, error) {
	return listCardsWithTeam(teamVerifiedList{}, errors.New("team directory offline"))
}
func recordEnrichCard(entry binderCard, list teamVerifiedList, listErr error) binderCard {
	entry.Badge = nil
	entry.Design = nil
	entry.Verified = false
	entry.BadgeCheck = "none"
	if listErr != nil {
		entry.BadgeCheck = "offline"
		return entry
	}
	b, ok := teamBadgeForCardID(list, entry.ID)
	if !ok {
		return entry
	}
	entry.BadgeCheck = "unverified"
	_, d, picture, e := recordFetch(b)
	if e != nil {
		return entry
	}
	levels, e := encodeLevels(d)
	if e != nil {
		return entry
	}
	safe := entryForDesign(d, levels)
	safe.Added = entry.Added
	if d.kind == 1 {
		safe.Hero = cardHero{Kind: "gallery", Source: d.gallerySrc + ".png"}
	}
	if len(picture) > 0 {
		path, e := recordStorePicture(entry.ID, picture)
		if e != nil {
			return entry
		}
		safe.Hero = cardHero{Kind: "custom", File: path}
	}
	safe.Badge = badgeViewFor(b, false)
	if design := badgeDesignFor(list, b); design != nil {
		safe.Design = &badgeCardDesign{SHA: design.SHA, Dir: design.Dir, Name: design.Name, Spec: design.Spec}
	}
	safe.Verified = true
	safe.BadgeCheck = "ok"
	return safe
}
