package main

import (
	"slices"
	"sort"
)

// binderView is the binder with when the gate last answered for it and when it will be asked again
type binderView struct {
	Cards   []binderCard `json:"cards"`
	Checked int64        `json:"checked"`
	Next    int64        `json:"next"`
	Paused  string       `json:"paused"`
}

func listCards() (binderView, error) {
	cards, e := storedCards()
	if e != nil {
		return binderView{}, e
	}
	ids := make([]string, len(cards))
	for i, c := range cards {
		ids[i] = c.ID
	}
	s, e := binderPull(ids)
	if e != nil {
		return binderView{}, e
	}
	list := s.team()
	for i := range cards {
		cards[i] = recordEnrichCard(cards[i], s, list)
	}
	sort.Slice(cards, func(i, j int) bool {
		if cards[i].Added.Equal(cards[j].Added) {
			return cards[i].ID < cards[j].ID
		}
		return cards[i].Added.After(cards[j].Added)
	})
	return binderView{Cards: cards, Checked: s.Checked, Next: s.Next, Paused: s.Paused}, nil
}

// recordEnrichCard shows the card as its owner's latest verified record when the binder holds
// one, live while the gate still has it and kept once the gate let it go; the badge, and with it
// the verified mark and team design, comes only from what the gate vouched for
func recordEnrichCard(entry binderCard, s pullState, list teamVerifiedList) binderCard {
	entry.Badge, entry.Design, entry.Verified, entry.BadgeCheck = nil, nil, false, "none"
	entry.Record, entry.RecordUpdated = "", 0
	if s.Checked == 0 {
		entry.BadgeCheck = "offline"
		return entry
	}
	b, badged := teamBadgeForCardID(list, entry.ID)
	if badged {
		entry.BadgeCheck = "unverified"
	}
	r, d, picture, e := recordCached(entry.ID, list)
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
		// the file is rewritten in place, so its revision tells Settings to load it again
		safe.Hero = cardHero{Kind: "custom", File: path, Rev: r.Picture[:12]}
	}
	safe.Record, safe.RecordUpdated = "live", r.Updated
	if slices.Contains(s.Gone, entry.ID) {
		safe.Record = "kept"
	}
	if !badged {
		return safe
	}
	safe.Badge = badgeViewFor(b, false)
	if design := badgeDesignFor(list, b); design != nil {
		safe.Design = &badgeCardDesign{SHA: design.SHA, Dir: design.Dir, Name: design.Name, Spec: design.Spec}
	}
	safe.Verified = true
	safe.BadgeCheck = "ok"
	return safe
}
