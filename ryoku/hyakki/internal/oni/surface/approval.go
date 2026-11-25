package surface

import (
	"context"
	"slices"
	"strings"

	i18n "ryoku-i18n"

	"ryoku-rashin/internal/oni/feature"
	"ryoku-rashin/internal/oni/render"
	"ryoku-rashin/internal/protocol"
)

// approvalFrame takes an approval card: an open one joins the cards
// waiting, or replaces the one with its id; a closed one leaves. The
// first waiting shows in the handoff card's place while no handoff card
// does. Nothing of the card's words reaches the ledger.
func (h *host) approvalFrame(f *feature.ApprovalFrame) {
	reason := "closed"
	if f.Open {
		reason = "open"
	}
	h.led.Add(Entry{Ev: EvApproval, Event: f.ID, Reason: reason, Kind: f.Scope})
	i := slices.IndexFunc(h.approvals, func(a feature.ApprovalFrame) bool { return a.ID == f.ID })
	switch {
	case f.Open && i >= 0:
		h.approvals[i] = *f
	case f.Open:
		h.approvals = append(h.approvals, *f)
	case i >= 0:
		h.approvals = slices.Delete(h.approvals, i, i+1)
	default:
		return
	}
	h.layoutHands()
}

// approvalContent is the card for an approval: who asks, what, where a
// Hands request would work, and its buttons in the frame's order. A card
// the pill may not answer offers only the app, and says so.
func approvalContent(a *feature.ApprovalFrame) render.CardContent {
	kicker := i18n.T("A team member asks")
	if strings.HasPrefix(a.ID, feature.ApprovalHandsPrefix) {
		kicker = i18n.T("Hands for a team member")
	}
	hint := ""
	switch a.Scope {
	case protocol.OniHandsScopeWindow:
		hint = i18n.T("In the window you picked, once.")
	case protocol.OniHandsScopeNested:
		hint = i18n.T("On Hyakki's separate desktop, once.")
	}
	if len(a.Options) == 1 && a.Options[0] == feature.ApprovalInApp {
		hint = i18n.T("Only the Hyakki app can answer this one.")
	}
	pills := make([]render.CardPill, 0, len(a.Options))
	for _, o := range a.Options {
		pills = append(pills, render.CardPill{Action: o, Text: approvalLabel(o), Primary: o == feature.ApprovalOnce})
	}
	return render.CardContent{Kicker: kicker, Text: a.Title, Hint: hint, Pills: pills}
}

// approvalLabel is a button's words
func approvalLabel(option string) string {
	switch option {
	case feature.ApprovalOnce:
		return i18n.T("Allow once")
	case feature.ApprovalAlways:
		return i18n.T("Allow in this chat")
	case feature.ApprovalNo:
		return i18n.T("Deny")
	case feature.ApprovalInApp:
		return i18n.T("Answer in the app")
	}
	return option
}

// handoffName is the part the card's place shows: an approval or the
// task's handoff card
func (h *host) handoffName() string {
	if h.handoffApproval != "" {
		return PartApproval
	}
	return PartHandoff
}

// approvalPress answers the approval showing with the option pressed, one
// press per layout: the daemon takes the card down with a closed frame,
// and a refused answer frees the buttons again. Only the card's own
// options are sent.
func (h *host) approvalPress(part *handsPart, option string) {
	l, id := h.link, h.handoffApproval
	if l == nil || part.sent {
		return
	}
	i := slices.IndexFunc(h.approvals, func(a feature.ApprovalFrame) bool { return a.ID == id })
	if i < 0 || !slices.Contains(h.approvals[i].Options, option) {
		return
	}
	part.sent = true
	h.led.Add(Entry{Ev: EvApprovalPress, Event: id, Reason: option})
	go func() {
		ctx, cancel := context.WithTimeout(h.ctx, callTimeout)
		defer cancel()
		if err := l.Answer(ctx, id, option); err != nil {
			h.be.post(func() { part.sent = false })
		}
	}()
}
