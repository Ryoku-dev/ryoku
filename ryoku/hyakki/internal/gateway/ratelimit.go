package gateway

import (
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"ryoku-rashin/internal/protocol"
)

// headerSpans is the window a provider's unsuffixed x-ratelimit-*-requests
// and x-ratelimit-*-tokens headers count over, where it is not a minute.
// Groq documents its request headers as requests per day and its token
// headers as tokens per minute (console.groq.com/docs/rate-limits).
var headerSpans = map[string]map[string]string{
	"groq": {protocol.LimitUnitRequests: protocol.LimitSpanDay},
}

// partialWindow gathers the headers of one span and unit before they become
// a window.
type partialWindow struct {
	limit, remaining int64
	hasLimit, hasRem bool
	reset            time.Time
	// period is the window length a header named, which settles the span of
	// headers that carry none
	period time.Duration
}

// headerWindows reads the rate-limit headers of one answer: the OpenAI and
// Groq x-ratelimit-{limit,remaining,reset}-{requests,tokens} family, the
// suffixed -minute/-day/-month variants Cerebras and SambaNova send, GitHub
// Models' renewalperiod and timeremaining, the bare x-ratelimit-limit trio
// GitHub and OpenRouter send, anthropic-ratelimit-*, and the IETF RateLimit
// fields in both their draft forms. A window whose span is not a minute, a
// day or a month is left out.
func headerWindows(providerID string, header http.Header, now time.Time) []protocol.LimitWindow {
	found := map[[2]string]*partialWindow{}
	at := func(span, unit string) *partialWindow {
		key := [2]string{span, unit}
		if found[key] == nil {
			found[key] = &partialWindow{}
		}
		return found[key]
	}
	// the span of an unsuffixed header is settled once every header is read
	unsuffixed := map[string]*partialWindow{}
	bare := &partialWindow{}
	for name, values := range header {
		if len(values) == 0 {
			continue
		}
		value := strings.TrimSpace(values[0])
		lower := strings.ToLower(name)
		switch {
		case strings.HasPrefix(lower, "anthropic-ratelimit-"):
			readAnthropic(strings.TrimPrefix(lower, "anthropic-ratelimit-"), value, now, at)
		case lower == "x-ratelimit-limit" || lower == "x-ratelimit-remaining" || lower == "x-ratelimit-reset":
			setField(bare, strings.TrimPrefix(lower, "x-ratelimit-"), value, now)
		case strings.HasPrefix(lower, "x-ratelimit-"):
			field, unit, span, ok := splitRateName(strings.TrimPrefix(lower, "x-ratelimit-"))
			if !ok {
				continue
			}
			if span == "" {
				if unsuffixed[unit] == nil {
					unsuffixed[unit] = &partialWindow{}
				}
				setField(unsuffixed[unit], field, value, now)
				continue
			}
			setField(at(span, unit), field, value, now)
		}
	}
	for unit, w := range unsuffixed {
		span := spanOfPeriod(w.period)
		if span == "" {
			span = protocol.LimitSpanMinute
			if named := headerSpans[providerID][unit]; named != "" {
				span = named
			}
		}
		merge(at(span, unit), w)
	}
	if bare.hasLimit || bare.hasRem {
		if span := spanUntil(bare.reset, now); span != "" {
			merge(at(span, protocol.LimitUnitRequests), bare)
		}
	}
	readIETF(header, now, at)
	return finishWindows(found, now)
}

// splitRateName splits limit-requests-day into its field, unit and span. The
// Kong form OVHcloud sends, limit-minute, names no unit and counts requests.
// An unknown span (an hour, a second) is refused rather than misnamed.
func splitRateName(name string) (field, unit, span string, ok bool) {
	parts := strings.Split(name, "-")
	if len(parts) < 2 || len(parts) > 3 {
		return "", "", "", false
	}
	field = parts[0]
	switch field {
	case "limit", "remaining", "reset", "renewalperiod", "timeremaining":
	default:
		return "", "", "", false
	}
	if span, isSpan := spanWord(parts[1]); isSpan {
		return field, protocol.LimitUnitRequests, span, len(parts) == 2 && span != ""
	}
	switch parts[1] {
	case "requests", "request", "req":
		unit = protocol.LimitUnitRequests
	case "tokens", "token":
		unit = protocol.LimitUnitTokens
	default:
		return "", "", "", false
	}
	if len(parts) == 3 {
		span, isSpan := spanWord(parts[2])
		if !isSpan || span == "" {
			return "", "", "", false
		}
		return field, unit, span, true
	}
	return field, unit, "", true
}

// spanWord says whether a header word names a window, and which of the three
// it is; an hour or a second is a window with no span here.
func spanWord(word string) (span string, isSpan bool) {
	switch word {
	case "minute", "min":
		return protocol.LimitSpanMinute, true
	case "day":
		return protocol.LimitSpanDay, true
	case "month":
		return protocol.LimitSpanMonth, true
	case "hour", "second", "sec", "week", "year":
		return "", true
	}
	return "", false
}

func setField(w *partialWindow, field, value string, now time.Time) {
	switch field {
	case "limit":
		if n, ok := parseCount(value); ok {
			w.limit, w.hasLimit = n, true
		}
	case "remaining":
		if n, ok := parseCount(value); ok {
			w.remaining, w.hasRem = n, true
		}
	case "reset", "timeremaining":
		if at, ok := parseReset(value, now); ok {
			w.reset = at
		}
	case "renewalperiod":
		if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds > 0 {
			w.period = time.Duration(seconds * float64(time.Second))
		}
	}
}

// readAnthropic reads anthropic-ratelimit-{requests,tokens}-{limit,remaining,reset};
// Anthropic counts both per minute (docs.anthropic.com, rate limits).
func readAnthropic(name, value string, now time.Time, at func(span, unit string) *partialWindow) {
	unitName, field, ok := strings.Cut(name, "-")
	if !ok {
		return
	}
	var unit string
	switch unitName {
	case "requests":
		unit = protocol.LimitUnitRequests
	case "tokens":
		unit = protocol.LimitUnitTokens
	default:
		return
	}
	setField(at(protocol.LimitSpanMinute, unit), field, value, now)
}

// readIETF reads the IETF RateLimit fields: the first draft's RateLimit-Limit,
// -Remaining and -Reset with a RateLimit-Policy such as 100;w=60, and the
// later structured RateLimit: "p";r=50;t=30 beside RateLimit-Policy:
// "p";q=100;w=60. Only a request quota is read.
func readIETF(header http.Header, now time.Time, at func(span, unit string) *partialWindow) {
	w := &partialWindow{}
	policy := firstItem(header.Get("RateLimit-Policy"))
	if q, ok := policy["q"]; ok {
		setField(w, "limit", q, now)
	} else if limit := firstItem(header.Get("RateLimit-Limit"))[""]; limit != "" {
		setField(w, "limit", limit, now)
	}
	if unit, ok := policy["qu"]; ok && strings.Trim(unit, `"`) != "requests" {
		return
	}
	if seconds, err := strconv.ParseFloat(policy["w"], 64); err == nil && seconds > 0 {
		w.period = time.Duration(seconds * float64(time.Second))
	}
	structured := firstItem(header.Get("RateLimit"))
	if r, ok := structured["r"]; ok {
		setField(w, "remaining", r, now)
	} else if remaining := header.Get("RateLimit-Remaining"); remaining != "" {
		setField(w, "remaining", remaining, now)
	}
	if t, ok := structured["t"]; ok {
		w.reset = now.Add(secondsDuration(t))
	} else if reset := header.Get("RateLimit-Reset"); reset != "" {
		w.reset = now.Add(secondsDuration(reset))
	}
	if !w.hasLimit && !w.hasRem {
		return
	}
	span := spanOfPeriod(w.period)
	if span == "" && w.period == 0 {
		span = spanUntil(w.reset, now)
	}
	if span != "" {
		merge(at(span, protocol.LimitUnitRequests), w)
	}
}

// firstItem reads the first list member of a structured field value as its
// parameters; a bare number lands under "".
func firstItem(value string) map[string]string {
	params := map[string]string{}
	item, _, _ := strings.Cut(value, ",")
	for i, part := range strings.Split(item, ";") {
		part = strings.TrimSpace(part)
		key, val, ok := strings.Cut(part, "=")
		switch {
		case ok:
			params[strings.TrimSpace(key)] = strings.TrimSpace(val)
		case i == 0 && part != "":
			params[""] = part
		}
	}
	return params
}

func secondsDuration(value string) time.Duration {
	seconds, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || seconds < 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

func merge(into, from *partialWindow) {
	if from.hasLimit {
		into.limit, into.hasLimit = from.limit, true
	}
	if from.hasRem {
		into.remaining, into.hasRem = from.remaining, true
	}
	if !from.reset.IsZero() {
		into.reset = from.reset
	}
}

// parseCount reads a count, which some providers send with a fraction.
func parseCount(value string) (int64, bool) {
	n, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || n < 0 || math.IsInf(n, 0) || math.IsNaN(n) {
		return 0, false
	}
	return int64(n), true
}

// parseReset reads when a window refills: a Go-style duration as OpenAI and
// Groq send (2m59.56s, 6ms), an RFC 3339 time as Anthropic sends, epoch
// milliseconds or seconds, or plain seconds from now.
func parseReset(value string, now time.Time) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	if at, err := time.Parse(time.RFC3339, value); err == nil {
		return at, true
	}
	if n, err := strconv.ParseFloat(value, 64); err == nil && n >= 0 {
		switch {
		case n > 1e12:
			return time.UnixMilli(int64(n)), true
		case n > 1e9:
			return time.Unix(int64(n), 0), true
		default:
			return now.Add(time.Duration(n * float64(time.Second))), true
		}
	}
	if d, err := time.ParseDuration(value); err == nil && d >= 0 {
		return now.Add(d), true
	}
	return time.Time{}, false
}

// spanOfPeriod names a window by its length, and only one of the three.
func spanOfPeriod(period time.Duration) string {
	switch {
	case period == time.Minute:
		return protocol.LimitSpanMinute
	case period == 24*time.Hour:
		return protocol.LimitSpanDay
	case period >= 28*24*time.Hour && period <= 31*24*time.Hour:
		return protocol.LimitSpanMonth
	}
	return ""
}

// spanUntil guesses a window from how far off its reset is, for headers that
// name no window at all.
func spanUntil(reset, now time.Time) string {
	if reset.IsZero() {
		return ""
	}
	switch until := reset.Sub(now); {
	case until <= time.Minute:
		return protocol.LimitSpanMinute
	case until <= 24*time.Hour:
		return protocol.LimitSpanDay
	case until <= 31*24*time.Hour:
		return protocol.LimitSpanMonth
	}
	return ""
}

func finishWindows(found map[[2]string]*partialWindow, now time.Time) []protocol.LimitWindow {
	windows := make([]protocol.LimitWindow, 0, len(found))
	for key, w := range found {
		if !w.hasLimit && !w.hasRem {
			continue
		}
		seen := now
		window := protocol.LimitWindow{Span: key[0], Unit: key[1], Limit: w.limit, Source: protocol.LimitFromHeaders, Seen: &seen}
		if w.hasRem {
			remaining := w.remaining
			window.Remaining = &remaining
		}
		if !w.reset.IsZero() {
			reset := w.reset
			window.Reset = &reset
		}
		windows = append(windows, window)
	}
	sortWindows(windows)
	return windows
}

var spanOrder = map[string]int{protocol.LimitSpanMinute: 0, protocol.LimitSpanDay: 1, protocol.LimitSpanMonth: 2}

// sortWindows orders windows minute, day, month, requests before tokens.
func sortWindows(windows []protocol.LimitWindow) {
	sort.Slice(windows, func(i, j int) bool {
		if windows[i].Span != windows[j].Span {
			return spanOrder[windows[i].Span] < spanOrder[windows[j].Span]
		}
		return windows[i].Unit < windows[j].Unit
	})
}
