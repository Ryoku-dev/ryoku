package main

import (
	"slices"
	"sort"
)

// binderView is the binder with when the gate last answered for it and when it will be asked
// again; for a badge holder, how many early refreshes the gate has left them today (-1 before it
// has said) and when, at the gate's next UTC midnight, they come back
type binderView struct {
	Cards       []binderCard `json:"cards"`
	Checked     int64        `json:"checked"`
	Next        int64        `json:"next"`
	Paused      string       `json:"paused"`
	Holder      bool         `json:"holder"`
	HolderLeft  int          `json:"holderLeft"`
	HolderReset int64        `json:"holderReset"`
}

// listCards shows the binder, asking the gate when its next check is due; refresh is the reader
// pressing Refresh, which a badge holder may spend an early refresh on
func listCards(refresh bool) (binderView, error) {
	cards, e := storedCards()
	if e != nil {
		return binderView{}, e
	}
	ids := make([]string, len(cards))
	for i, c := range cards {
		ids[i] = c.ID
	}
	holder := badgeHolder()
	s, e := binderPull(ids, refresh && holder)
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
	return binderView{Cards: cards, Checked: s.Checked, Next: s.Next, Paused: s.Paused,
		Holder: holder, HolderLeft: s.holderLeft(), HolderReset: (s.now()/86400 + 1) * 86400}, nil
}

// recordEnrichCard shows the card as its owner's latest verified record when the binder holds
// one, live while the gate still has it and kept once the gate let it go; the badge, and with it
// the verified mark, team design, card style and the owner's own words, comes only from what the
// gate vouched for
func recordEnrichCard(entry binderCard, s pullState, list teamVerifiedList) binderCard {
	entry.Badge, entry.Design, entry.Verified, entry.BadgeCheck, entry.Style, entry.StandName = nil, nil, false, "none", "", ""
	entry.Signed, entry.Publication, entry.ApprovalState = false, "", ""
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
		if r.V == 5 {
			entry.DesignError = e.Error()
		}
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
	if len(picture) > 0 && r.V != 5 {
		path, e := recordStorePicture(entry.ID, picture)
		if e != nil {
			return entry
		}
		// the file is rewritten in place, so its revision tells Settings to load it again
		safe.Hero = cardHero{Kind: "custom", File: path, Rev: r.Picture[:12]}
	}
	safe.Signed = true
	if r.V == 5 {
		if err := cardAnimationEnrich(&safe, r); err != nil {
			entry.DesignError = err.Error()
			return entry
		}
	}
	safe.Record, safe.RecordUpdated = "live", r.Updated
	if slices.Contains(s.Gone, entry.ID) {
		safe.Record = "kept"
	}
	if r.V == 5 && safe.Record == "kept" {
		safe.ApprovalState = "historical"
	}
	if r.V == 5 && safe.ApprovalState == "current" && (s.Paused == "offline" || s.now() >= s.Next) {
		safe.ApprovalState = "last-known"
	}
	if !badged {
		return safe
	}
	safe.Badge = badgeViewFor(b, false)
	if r.V != 5 {
		if design, err := badgeDesignFor(list, b); err != nil {
			safe.DesignError = err.Error()
		} else if design != nil {
			safe.Design = &badgeCardDesign{SHA: design.SHA, Dir: design.Dir, Name: design.Name, Spec: design.Spec}
		}
	}
	safe.Verified = true
	safe.BadgeCheck = "ok"
	// recordCached verified this record through the badge's own device, so the words are the
	// holder's; the style is drawn only when that badge's login owns it, and a style this binder
	// doesn't know yet is drawn classic
	if styleOwnedBy(r.Style, b.Login) {
		safe.Style = r.Style
	}
	if r.Tagline != "" {
		safe.Tagline = r.Tagline
	}
	safe.StandName = r.StandName
	return safe
}
