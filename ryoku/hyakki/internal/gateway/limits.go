package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"go.etcd.io/bbolt"
	"ryoku-rashin/internal/protocol"
)

var bucketLimits = []byte("limits")

// limitRecord is what one provider last said of its allowance, kept in the
// gateway store so it survives a restart. Read is when its account endpoint
// was last asked, which paces the next read across restarts too.
type limitRecord struct {
	Headers []protocol.LimitWindow `json:"headers,omitempty"`
	Account []protocol.LimitWindow `json:"account,omitempty"`
	Credits *protocol.LimitCredits `json:"credits,omitempty"`
	Read    time.Time              `json:"read,omitempty"`
}

// limitBook holds every provider's limitRecord in memory once read from the
// store, and which account reads are running.
type limitBook struct {
	mu      sync.Mutex
	records map[string]*limitRecord
	reading map[string]bool
}

// limitRecordLocked returns a provider's record, reading it from the store
// the first time. The caller holds g.limits.mu.
func (g *Gateway) limitRecordLocked(providerID string) *limitRecord {
	if g.limits.records == nil {
		g.limits.records = map[string]*limitRecord{}
	}
	if record := g.limits.records[providerID]; record != nil {
		return record
	}
	record := &limitRecord{}
	_ = g.store.db.View(func(tx *bbolt.Tx) error {
		if b := tx.Bucket(bucketLimits); b != nil {
			if value := b.Get([]byte(providerID)); value != nil {
				return json.Unmarshal(value, record)
			}
		}
		return nil
	})
	g.limits.records[providerID] = record
	return record
}

// saveLimitRecordLocked writes a provider's record to the store. The caller
// holds g.limits.mu.
func (g *Gateway) saveLimitRecordLocked(providerID string, record *limitRecord) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return g.store.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketLimits)
		if err != nil {
			return err
		}
		return b.Put([]byte(providerID), data)
	})
}

// observeLimits keeps the rate-limit headers of a provider's answer, success
// or refusal. It says nothing to the app: an answer is not a provider change,
// and the next providers.list or providers.changed carries the figures.
func (g *Gateway) observeLimits(p *provider, header http.Header) {
	if p.Tier == protocol.TierLocal {
		return
	}
	windows := headerWindows(p.ID, header, g.now())
	if len(windows) == 0 {
		return
	}
	g.limits.mu.Lock()
	defer g.limits.mu.Unlock()
	record := g.limitRecordLocked(p.ID)
	record.Headers = replaceWindows(record.Headers, windows)
	_ = g.saveLimitRecordLocked(p.ID, record)
}

// replaceWindows puts each fresh window in place of the kept one of the same
// span and unit; a window the latest answer did not mention is kept.
func replaceWindows(kept, fresh []protocol.LimitWindow) []protocol.LimitWindow {
	out := append([]protocol.LimitWindow(nil), fresh...)
	for _, old := range kept {
		if !hasWindow(out, old.Span, old.Unit) {
			out = append(out, old)
		}
	}
	sortWindows(out)
	return out
}

func hasWindow(windows []protocol.LimitWindow, span, unit string) bool {
	for _, w := range windows {
		if w.Span == span && w.Unit == unit {
			return true
		}
	}
	return false
}

// providerLimits is what ProviderInfo.Limits says of a provider: per span
// and unit the latest live window, refilled once its reset has passed, then
// the documented limits no live window covers, the credits, and what Rashin
// sent there today and this month. A local provider has none.
func (g *Gateway) providerLimits(p *provider) *protocol.ProviderLimits {
	if p.Tier == protocol.TierLocal {
		return nil
	}
	now := g.now()
	limits := &protocol.ProviderLimits{}
	g.limits.mu.Lock()
	record := g.limitRecordLocked(p.ID)
	for _, w := range append(append([]protocol.LimitWindow(nil), record.Account...), record.Headers...) {
		if i := windowIndex(limits.Windows, w.Span, w.Unit); i >= 0 {
			if seenAfter(w, limits.Windows[i]) {
				limits.Windows[i] = w
			}
			continue
		}
		limits.Windows = append(limits.Windows, w)
	}
	if record.Credits != nil {
		credits := *record.Credits
		limits.Credits = &credits
	}
	g.limits.mu.Unlock()
	for i := range limits.Windows {
		w := &limits.Windows[i]
		if w.Reset != nil && !now.Before(*w.Reset) && w.Limit > 0 {
			full := w.Limit
			w.Remaining, w.Reset = &full, nil
		}
	}
	for _, d := range documentedLimits[p.ID] {
		if !hasWindow(limits.Windows, d.Span, d.Unit) {
			limits.Windows = append(limits.Windows, protocol.LimitWindow{Span: d.Span, Unit: d.Unit, Limit: d.Limit, Source: protocol.LimitFromDocumented})
		}
	}
	sortWindows(limits.Windows)
	limits.Today, limits.Month, _ = g.store.providerUse(p.ID, now)
	return limits
}

func windowIndex(windows []protocol.LimitWindow, span, unit string) int {
	for i, w := range windows {
		if w.Span == span && w.Unit == unit {
			return i
		}
	}
	return -1
}

func seenAfter(a, b protocol.LimitWindow) bool {
	return a.Seen != nil && (b.Seen == nil || a.Seen.After(*b.Seen))
}

// providerUse sums what was sent to one provider on the UTC day of now and
// in its UTC month. Usage rows are keyed by day first, so only this month's
// rows are visited and only this provider's are decoded.
func (s *store) providerUse(providerID string, now time.Time) (today, month protocol.LimitUse, err error) {
	utc := now.UTC()
	day := []byte(utc.Format("2006-01-02"))
	prefix := []byte(utc.Format("2006-01"))
	middle := []byte("\x00" + providerID + "\x00")
	err = s.db.View(func(tx *bbolt.Tx) error {
		c := tx.Bucket(bucketUsage).Cursor()
		for key, value := c.Seek(prefix); key != nil && bytes.HasPrefix(key, prefix); key, value = c.Next() {
			if len(key) <= len(day) || !bytes.HasPrefix(key[len(day):], middle) {
				continue
			}
			var record usageRecord
			if err := json.Unmarshal(value, &record); err != nil {
				return err
			}
			use := protocol.LimitUse{Requests: record.Requests, Tokens: record.InputTokens + record.OutputTokens}
			month.Requests += use.Requests
			month.Tokens += use.Tokens
			if bytes.HasPrefix(key, day) {
				today.Requests += use.Requests
				today.Tokens += use.Tokens
			}
		}
		return nil
	})
	return today, month, err
}
