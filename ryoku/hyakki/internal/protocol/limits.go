package protocol

import "time"

// Limit windows, units and sources. A window is the span a provider counts
// over; the source says how Rashin learned the figure.
const (
	LimitSpanMinute = "minute"
	LimitSpanDay    = "day"
	LimitSpanMonth  = "month"

	LimitUnitRequests = "requests"
	LimitUnitTokens   = "tokens"

	// LimitFromHeaders is read from the rate-limit headers of the provider's
	// last answer.
	LimitFromHeaders = "headers"
	// LimitFromAccount is read from the provider's account endpoint.
	LimitFromAccount = "account"
	// LimitFromDocumented is the provider's published free limit, as the
	// free-provider lists Rashin's catalogue comes from record it.
	LimitFromDocumented = "documented"
)

// ProviderLimits is what Rashin knows of a provider's allowance: the windows
// it counts over, the credits left, and Rashin's own count of what it sent
// there today and this month (UTC).
type ProviderLimits struct {
	Windows []LimitWindow `json:"windows,omitempty"`
	Credits *LimitCredits `json:"credits,omitempty"`
	Today   LimitUse      `json:"today"`
	Month   LimitUse      `json:"month"`
}

// LimitWindow is one allowance: Limit of Unit per Span. Remaining is nil
// when only the limit is known, as for a documented one. Seen is when Rashin
// read it, nil for a documented one.
type LimitWindow struct {
	Span      string     `json:"span"`
	Unit      string     `json:"unit"`
	Limit     int64      `json:"limit,omitempty"`
	Remaining *int64     `json:"remaining,omitempty"`
	Reset     *time.Time `json:"reset,omitempty"`
	Source    string     `json:"source"`
	Seen      *time.Time `json:"seen,omitempty"`
}

// LimitCredits is a provider's account balance. Currency is an ISO code
// such as USD or CNY, or the provider's own unit such as pollen. Limit and
// Remaining are nil when the provider does not say; Used may then be the
// only figure. FreeTier marks an account that never bought credits.
type LimitCredits struct {
	Limit     *float64  `json:"limit,omitempty"`
	Remaining *float64  `json:"remaining,omitempty"`
	Used      float64   `json:"used"`
	Currency  string    `json:"currency"`
	FreeTier  bool      `json:"freeTier,omitempty"`
	Source    string    `json:"source"`
	Seen      time.Time `json:"seen"`
}

// LimitUse is what Rashin sent to a provider over a span.
type LimitUse struct {
	Requests int   `json:"requests"`
	Tokens   int64 `json:"tokens"`
}
