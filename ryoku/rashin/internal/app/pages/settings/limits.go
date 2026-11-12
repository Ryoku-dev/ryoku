package settings

import (
	"fmt"
	"math"
	"time"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/ryoku-dev/ryoku/docs/lithic/library/go/kit"
	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// the head's inset, so the line under it starts where its title does
const limitInset = unit.Dp(13)

// a folded provider says its allowance in one line under its head; unfolded, the breakdown says more
func withLimitLine(gtx layout.Context, th *look.Theme, provider protocol.ProviderInfo, open bool, row layout.Widget) layout.Dimensions {
	line := limitLine(provider, gtx.Now)
	if open || line == "" {
		return row(gtx)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(row),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Left: limitInset, Right: limitInset, Bottom: th.Sp.S2}.Layout(gtx,
				look.Paragraph(th, th.T.Caption, th.R.Fg2, line))
		}),
	)
}

// the one plain sentence about what is left: a live daily figure first, then credits, then a live
// monthly or minute one; without any, the documented limit beside what Rashin sent
func limitLine(provider protocol.ProviderInfo, now time.Time) string {
	limits := provider.Limits
	if provider.Local || limits == nil {
		return ""
	}
	free := provider.Tier != protocol.TierPaid
	if w, ok := liveWindow(limits, protocol.LimitSpanDay); ok {
		return i18n.Tf("%s, as of %s.", windowLeft(w, free), look.Ago(now, *w.Seen))
	}
	if limits.Credits != nil {
		if amount := creditsLeft(*limits.Credits); amount != "" {
			return i18n.Tf("Credits: %s.", amount)
		}
	}
	for _, span := range []string{protocol.LimitSpanMonth, protocol.LimitSpanMinute} {
		if w, ok := liveWindow(limits, span); ok {
			return i18n.Tf("%s, as of %s.", windowLeft(w, free), look.Ago(now, *w.Seen))
		}
	}
	// a provider nobody set up and nothing was sent to has nothing worth a line
	if !provider.Configured && limits.Today.Requests == 0 {
		return ""
	}
	if w, ok := documentedWindow(limits, protocol.LimitSpanDay); ok {
		return i18n.Tf("Documented free limit: %s. Rashin sent %s today.", windowRule(w), sentWords(limits.Today, w.Unit))
	}
	if w, ok := documentedWindow(limits, protocol.LimitSpanMonth); ok {
		return i18n.Tf("Documented free limit: %s. Rashin sent %s this month.", windowRule(w), sentWords(limits.Month, w.Unit))
	}
	if w, ok := documentedWindow(limits, protocol.LimitSpanMinute); ok {
		return i18n.Tf("Documented free limit: %s. Rashin sent %s today.", windowRule(w), sentWords(limits.Today, w.Unit))
	}
	unit := protocol.LimitUnitTokens
	if limits.Today.Tokens == 0 {
		unit = protocol.LimitUnitRequests
	}
	return i18n.Tf("%s does not report a daily limit. Rashin sent %s today.", provider.Name, sentWords(limits.Today, unit))
}

// a live window that says what is left, tokens before requests
func liveWindow(limits *protocol.ProviderLimits, span string) (protocol.LimitWindow, bool) {
	for _, unit := range []string{protocol.LimitUnitTokens, protocol.LimitUnitRequests} {
		for _, w := range limits.Windows {
			if w.Span == span && w.Unit == unit && w.Source != protocol.LimitFromDocumented && w.Remaining != nil && w.Seen != nil {
				return w, true
			}
		}
	}
	return protocol.LimitWindow{}, false
}

// a documented window, requests before tokens: requests are what a free tier is usually counted in
func documentedWindow(limits *protocol.ProviderLimits, span string) (protocol.LimitWindow, bool) {
	for _, unit := range []string{protocol.LimitUnitRequests, protocol.LimitUnitTokens} {
		for _, w := range limits.Windows {
			if w.Span == span && w.Unit == unit && w.Source == protocol.LimitFromDocumented && w.Limit > 0 {
				return w, true
			}
		}
	}
	return protocol.LimitWindow{}, false
}

// "Free tokens today: 412,300 of 500,000 left"
func windowLeft(w protocol.LimitWindow, free bool) string {
	left := look.Grouped(int(*w.Remaining))
	if w.Limit > 0 {
		return i18n.Tf("%s: %s of %s left", windowTitle(w, free), left, look.Grouped(int(w.Limit)))
	}
	return i18n.Tf("%s: %s left", windowTitle(w, free), left)
}

func windowTitle(w protocol.LimitWindow, free bool) string {
	tokens := w.Unit == protocol.LimitUnitTokens
	switch {
	case w.Span == protocol.LimitSpanDay && tokens && free:
		return i18n.T("Free tokens today")
	case w.Span == protocol.LimitSpanDay && tokens:
		return i18n.T("Tokens today")
	case w.Span == protocol.LimitSpanDay && free:
		return i18n.T("Free requests today")
	case w.Span == protocol.LimitSpanDay:
		return i18n.T("Requests today")
	case w.Span == protocol.LimitSpanMonth && tokens && free:
		return i18n.T("Free tokens this month")
	case w.Span == protocol.LimitSpanMonth && tokens:
		return i18n.T("Tokens this month")
	case w.Span == protocol.LimitSpanMonth && free:
		return i18n.T("Free requests this month")
	case w.Span == protocol.LimitSpanMonth:
		return i18n.T("Requests this month")
	case tokens:
		return i18n.T("Tokens this minute")
	default:
		return i18n.T("Requests this minute")
	}
}

// "1,000 requests a day"
func windowRule(w protocol.LimitWindow) string {
	n := look.Grouped(int(w.Limit))
	tokens := w.Unit == protocol.LimitUnitTokens
	switch {
	case w.Span == protocol.LimitSpanDay && tokens:
		return i18n.Tf("%s tokens a day", n)
	case w.Span == protocol.LimitSpanDay:
		return i18n.Tf("%s requests a day", n)
	case w.Span == protocol.LimitSpanMonth && tokens:
		return i18n.Tf("%s tokens a month", n)
	case w.Span == protocol.LimitSpanMonth:
		return i18n.Tf("%s requests a month", n)
	case tokens:
		return i18n.Tf("%s tokens a minute", n)
	default:
		return i18n.Tf("%s requests a minute", n)
	}
}

// what Rashin sent, counted in the unit the limit is
func sentWords(use protocol.LimitUse, unit string) string {
	switch {
	case use.Requests == 0:
		return i18n.T("nothing")
	case unit == protocol.LimitUnitTokens && use.Tokens == 1:
		return i18n.T("1 token")
	case unit == protocol.LimitUnitTokens:
		return i18n.Tf("%s tokens", look.Grouped(int(use.Tokens)))
	case use.Requests == 1:
		return i18n.T("1 request")
	default:
		return i18n.Tf("%s requests", look.Grouped(use.Requests))
	}
}

// "$4.21 of $10 left", or what the account says when it gives less
func creditsLeft(c protocol.LimitCredits) string {
	switch {
	case c.Remaining != nil && c.Limit != nil:
		return i18n.Tf("%s of %s left", money(*c.Remaining, c.Currency), money(*c.Limit, c.Currency))
	case c.Remaining != nil:
		return i18n.Tf("%s left", money(*c.Remaining, c.Currency))
	case c.Used > 0:
		return i18n.Tf("%s used", money(c.Used, c.Currency))
	}
	return ""
}

// whole amounts drop their cents, as a price on a card does
func money(value float64, currency string) string {
	amount := fmt.Sprintf("%.2f", value)
	if value == math.Trunc(value) {
		amount = fmt.Sprintf("%.0f", value)
	}
	switch currency {
	case "USD":
		return "$" + amount
	case "CNY":
		return "¥" + amount
	case "EUR":
		return "€" + amount
	}
	return amount + " " + currency
}

// the unfolded provider's breakdown: each window with where it came from, the credits, and what Rashin sent
func limitsDetail(th *look.Theme, provider protocol.ProviderInfo) []layout.FlexChild {
	limits := provider.Limits
	if provider.Local || limits == nil {
		return nil
	}
	children := []layout.FlexChild{
		layout.Rigid(layout.Spacer{Height: th.Sp.S3}.Layout),
		layout.Rigid(kit.Text(th.Theme, th.T.Body, th.R.Fg, i18n.T("Allowance"))),
	}
	fact := func(label string, value func(now time.Time) string) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: th.Sp.S1}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}.Layout(gtx,
					layout.Flexed(2, kit.Text(th.Theme, th.T.Caption, th.R.Fg2, label)),
					layout.Flexed(3, look.Paragraph(th, th.T.Caption, th.R.Fg, value(gtx.Now))),
				)
			})
		})
	}
	for _, w := range limits.Windows {
		children = append(children, fact(windowName(w), func(now time.Time) string { return windowFact(w, now) }))
	}
	if c := limits.Credits; c != nil {
		children = append(children, fact(i18n.T("Credits"), func(now time.Time) string {
			text := creditsLeft(*c)
			if c.FreeTier {
				text = look.Clauses(text, i18n.T("never topped up"))
			}
			return look.Clauses(text, look.Ago(now, c.Seen))
		}))
	}
	children = append(children,
		fact(i18n.T("Rashin sent today"), func(time.Time) string { return useWords(limits.Today) }),
		fact(i18n.T("Rashin sent this month"), func(time.Time) string { return useWords(limits.Month) }),
	)
	if len(limits.Windows) == 0 && limits.Credits == nil {
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: th.Sp.S1}.Layout(gtx, look.Paragraph(th, th.T.Caption, th.R.Fg3,
				i18n.Tf("%s reports no limits and none is documented.", provider.Name)))
		}))
	}
	return children
}

// "Requests a day"
func windowName(w protocol.LimitWindow) string {
	tokens := w.Unit == protocol.LimitUnitTokens
	switch {
	case w.Span == protocol.LimitSpanDay && tokens:
		return i18n.T("Tokens a day")
	case w.Span == protocol.LimitSpanDay:
		return i18n.T("Requests a day")
	case w.Span == protocol.LimitSpanMonth && tokens:
		return i18n.T("Tokens a month")
	case w.Span == protocol.LimitSpanMonth:
		return i18n.T("Requests a month")
	case tokens:
		return i18n.T("Tokens a minute")
	default:
		return i18n.T("Requests a minute")
	}
}

// "412 of 1,000 left, refills in 3 h, from its answers 2 min ago"
func windowFact(w protocol.LimitWindow, now time.Time) string {
	if w.Source == protocol.LimitFromDocumented {
		return i18n.Tf("%s, documented", look.Grouped(int(w.Limit)))
	}
	amount := look.Grouped(int(w.Limit))
	switch {
	case w.Remaining != nil && w.Limit > 0:
		amount = i18n.Tf("%s of %s left", look.Grouped(int(*w.Remaining)), look.Grouped(int(w.Limit)))
	case w.Remaining != nil:
		amount = i18n.Tf("%s left", look.Grouped(int(*w.Remaining)))
	}
	refills := ""
	if w.Reset != nil && w.Reset.After(now) {
		refills = i18n.Tf("refills %s", look.Until(now, *w.Reset))
	}
	from := ""
	if w.Seen != nil {
		from = i18n.Tf("from its answers %s", look.Ago(now, *w.Seen))
		if w.Source == protocol.LimitFromAccount {
			from = i18n.Tf("from its account %s", look.Ago(now, *w.Seen))
		}
	}
	return look.Clauses(amount, refills, from)
}

// "18 requests, 18,200 tokens"
func useWords(use protocol.LimitUse) string {
	if use.Tokens == 0 {
		return sentWords(use, protocol.LimitUnitRequests)
	}
	return look.Clauses(sentWords(use, protocol.LimitUnitRequests), sentWords(use, protocol.LimitUnitTokens))
}
