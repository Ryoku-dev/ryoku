package env

import (
	"time"

	"ryoku-i18n"

	"ryoku-rashin/internal/app/look"
	"ryoku-rashin/internal/protocol"
)

// what the router answered, read once each time a page that shows it opens and never per frame
type Usage struct {
	result protocol.GatewayUsage
	trend  look.Trend
	asked  bool
	loaded bool
}

// the router counts its days in UTC, so now is taken there; a week reads as this week, anything longer as two weeks
func (u *Usage) Load(e *Env, days int, now time.Time) {
	if u.asked {
		return
	}
	u.asked = true
	var result protocol.GatewayUsage
	e.Call(protocol.MGatewayUsage, protocol.UsageParams{Days: days}, &result, func(err error) {
		if err != nil {
			u.asked = false
			return
		}
		u.result, u.trend, u.loaded = result, answersTrend(result, look.LastDays(now.UTC(), days), days <= 7), true
	})
}

// the next time the page opens it reads again
func (u *Usage) Forget() { u.asked = false }

// false until the first reading arrives; a trend with no values means nothing was asked
func (u *Usage) Trend() (look.Trend, bool) { return u.trend, u.loaded }

// says how much a provider was used, or nothing when it was not
func (u *Usage) Words(provider string) string {
	answered, failed := 0, 0
	for _, row := range u.result.Rows {
		if row.Provider == provider {
			answered += row.OK
			failed += row.Failed
		}
	}
	switch {
	case answered == 0 && failed == 0:
		return ""
	case failed == 0:
		return i18n.Tf("%d answers this week", answered)
	}
	return i18n.Tf("%d answers and %d failures this week", answered, failed)
}

func answersTrend(result protocol.GatewayUsage, days []string, week bool) look.Trend {
	byDay := make(map[string]float64, len(days))
	requests, failed := 0, 0
	for _, row := range result.Rows {
		byDay[row.Day] += float64(row.Requests)
		requests += row.Requests
		failed += row.Failed
	}
	trend := look.Trend{Title: i18n.T("Answers"), Ticks: look.DayTicks(days)}
	// nothing asked draws no chart at all: a home that invites a question is no place for an empty line
	if requests == 0 {
		return trend
	}
	trend.Values = look.DayValues(days, byDay)
	switch {
	case week && failed == 0:
		trend.Proof = i18n.Tf("%d answers this week, none failed", requests)
	case week:
		trend.Proof = i18n.Tf("%d answers this week; %d failed", requests, failed)
	case failed == 0:
		trend.Proof = i18n.Tf("%d answers in two weeks, none failed", requests)
	default:
		trend.Proof = i18n.Tf("%d answers in two weeks; %d failed", requests, failed)
	}
	return trend
}
