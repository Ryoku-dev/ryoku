package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"ryoku-rashin/internal/protocol"
)

const (
	// listingTTL is how long a provider's own model listing is trusted before
	// it is read again.
	listingTTL = 6 * time.Hour
	// listingRetry is how long a listing that could not be read waits before
	// it is asked for again.
	listingRetry = 15 * time.Minute
	// listingPace is the least time between two reads of one provider's
	// listing forced by Read the list again, whoever asks.
	listingPace = time.Minute
	// listingBudget bounds reading one listing, every page of it.
	listingBudget = 20 * time.Second
	// listingBytes bounds one page: OpenRouter's whole listing is under 1 MB.
	listingBytes = 16 << 20
	listingPages = 20
	// listingReaders is how many listings are read at once.
	listingReaders = 4
)

var errNoListingKey = errors.New("provider listing needs a key")

// listingRecord is what the store keeps of a provider's listing.
type listingRecord struct {
	// Read is when the listing was last read; Tried is the last attempt,
	// which may have failed.
	Read  time.Time `json:"read"`
	Tried time.Time `json:"tried"`
	// Keyed is a listing read with the user's key, which may list more than
	// the public one.
	Keyed  bool          `json:"keyed,omitempty"`
	Models []listedModel `json:"models"`
}

// listings keeps each catalogue provider's last listing in memory, read from
// the store once, the provider it makes merged with the catalogue, and the
// reads under way.
type listings struct {
	mu      sync.Mutex
	loaded  bool
	entries map[string]*listingEntry
	readers chan struct{}
}

type listingEntry struct {
	record listingRecord
	// merged is base, the catalogue provider, with the listing's models; it
	// is rebuilt when either changes and is never changed after, so a route
	// may hold it while the listing is read again
	base   *provider
	merged *provider
	// reading is closed when the read under way ends, nil when none is
	reading chan struct{}
	forced  time.Time
}

func newListings() listings {
	return listings{entries: make(map[string]*listingEntry), readers: make(chan struct{}, listingReaders)}
}

// listed returns catalogue provider p with the models its own listing adds,
// and starts reading the listing in the background when it is due. It never
// waits on the network, so no chat turn waits on a listing.
func (g *Gateway) listed(p *provider) *provider {
	readable, keyed := g.listingReadable(p)
	g.listings.mu.Lock()
	defer g.listings.mu.Unlock()
	entry := g.listingEntry(p.ID)
	if readable && g.listingDue(entry, keyed) {
		g.startListingRead(p, entry)
	}
	if entry.merged == nil || entry.base != p {
		entry.base, entry.merged = p, mergeListing(p, entry.record)
	}
	return entry.merged
}

// refreshListings reads the listings of the named catalogue providers now
// and waits for them within ctx. Each is read at most once a minute however
// often it is asked; a second ask in that minute is answered from what the
// first read.
func (g *Gateway) refreshListings(ctx context.Context, ids []string) {
	var waits []chan struct{}
	for _, id := range ids {
		p, ok := g.catalog.findProvider(id)
		if !ok {
			continue
		}
		readable, _ := g.listingReadable(p)
		if !readable {
			continue
		}
		g.listings.mu.Lock()
		entry := g.listingEntry(p.ID)
		if entry.reading == nil && g.now().Sub(entry.forced) >= listingPace {
			entry.forced = g.now()
			g.startListingRead(p, entry)
		}
		if entry.reading != nil {
			waits = append(waits, entry.reading)
		}
		g.listings.mu.Unlock()
	}
	for _, wait := range waits {
		select {
		case <-wait:
		case <-ctx.Done():
			return
		}
	}
}

// listingRead is when a provider's listing was last read, zero when never.
func (g *Gateway) listingRead(id string) time.Time {
	g.listings.mu.Lock()
	defer g.listings.mu.Unlock()
	return g.listingEntry(id).record.Read
}

// listingEntry is the entry for id; the caller holds listings.mu.
func (g *Gateway) listingEntry(id string) *listingEntry {
	if !g.listings.loaded {
		g.listings.loaded = true
		// a store that cannot be read leaves every listing to be read again
		records, _ := g.store.listingRecords()
		for provider, record := range records {
			g.listings.entries[provider] = &listingEntry{record: record}
		}
	}
	entry := g.listings.entries[id]
	if entry == nil {
		entry = &listingEntry{}
		g.listings.entries[id] = entry
	}
	return entry
}

// listingReadable says whether Rashin can read p's listing, and whether it
// would read it with a key.
func (g *Gateway) listingReadable(p *provider) (readable, keyed bool) {
	if p.Tier == protocol.TierLocal {
		return false, false
	}
	if !p.Keyless {
		keyed, _ = g.hasUsableKey(p)
	}
	return p.Keyless || p.Listing.Public || keyed, keyed
}

// listingDue says a listing is stale, or was read without the key there is
// now; one that failed waits listingRetry. The caller holds listings.mu.
func (g *Gateway) listingDue(entry *listingEntry, keyed bool) bool {
	if entry.reading != nil {
		return false
	}
	now := g.now()
	if !entry.record.Tried.IsZero() && now.Sub(entry.record.Tried) < listingRetry {
		return false
	}
	return entry.record.Read.IsZero() || now.Sub(entry.record.Read) >= listingTTL || (keyed && !entry.record.Keyed)
}

// startListingRead reads p's listing in the background. The caller holds
// listings.mu.
func (g *Gateway) startListingRead(p *provider, entry *listingEntry) {
	if g.ctx.Err() != nil {
		return
	}
	done := make(chan struct{})
	entry.reading = done
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		changed := false
		defer func() {
			g.listings.mu.Lock()
			close(done)
			entry.reading = nil
			g.listings.mu.Unlock()
			if changed {
				g.notifyChange()
			}
		}()
		select {
		case g.listings.readers <- struct{}{}:
			defer func() { <-g.listings.readers }()
		case <-g.ctx.Done():
			return
		}
		ctx, cancel := context.WithTimeout(g.ctx, listingBudget)
		defer cancel()
		models, keyed, err := g.readListing(ctx, p)
		now := g.now()
		g.listings.mu.Lock()
		record := entry.record
		record.Tried = now
		if err == nil {
			// a new read time is news too: Settings says when the list was read
			changed = true
			record.Read, record.Keyed, record.Models = now, keyed, models
			entry.merged = nil
		}
		entry.record = record
		g.listings.mu.Unlock()
		// what was read stays in memory when it cannot be kept
		_ = g.store.putListing(p.ID, record)
	}()
}

// readListing reads every page of p's listing, with its first usable key, or
// none when it needs none.
func (g *Gateway) readListing(ctx context.Context, p *provider) ([]listedModel, bool, error) {
	secret, keyed, err := g.listingSecret(p)
	if err != nil {
		return nil, false, err
	}
	page, header, err := listingRequest(p, secret)
	if err != nil {
		return nil, false, err
	}
	var all []listedModel
	for pages := 0; page != nil && pages < listingPages; pages++ {
		body, err := g.fetchListing(ctx, page, header)
		if err != nil {
			return nil, false, err
		}
		models, next, err := parseListing(p.Listing.Shape, body, page)
		if err != nil {
			return nil, false, err
		}
		for _, m := range models {
			all = appendListed(all, m)
		}
		page = next
	}
	return all, keyed, nil
}

// listingSecret is the key a listing is read with: the first one that works,
// or none for a keyless provider or a public listing.
func (g *Gateway) listingSecret(p *provider) (string, bool, error) {
	if p.Keyless {
		return "", false, nil
	}
	records, err := g.store.listKeys(p.ID)
	if err != nil {
		return "", false, err
	}
	for _, record := range records {
		if record.Status == protocol.KeyError {
			continue
		}
		if _, cooling, err := g.store.cooldown(keyScope(record.ID), g.now()); err != nil || cooling {
			continue
		}
		secret, _, err := g.store.reveal(record.ID)
		if err == nil && secret != "" {
			return secret, true, nil
		}
	}
	if p.Listing.Public {
		return "", false, nil
	}
	return "", false, errNoListingKey
}

// listingRequest is the first page of p's listing and the headers it is read
// with. The key goes to the provider's own address only.
func listingRequest(p *provider, secret string) (*url.URL, http.Header, error) {
	address := p.Listing.URL
	if address == "" {
		address = strings.TrimRight(p.BaseURL, "/") + "/models"
	}
	token := secret
	if p.Auth == "cloudflare" {
		account, key, ok := strings.Cut(secret, ":")
		if !ok || strings.TrimSpace(account) == "" {
			return nil, nil, errNoListingKey
		}
		address = strings.ReplaceAll(address, "{account_id}", url.PathEscape(strings.TrimSpace(account)))
		token = strings.TrimSpace(key)
	}
	page, err := url.Parse(address)
	if err != nil {
		return nil, nil, fmt.Errorf("provider listing address: %w", err)
	}
	header := make(http.Header)
	header.Set("Accept", "application/json")
	for key, value := range p.ExtraHeaders {
		header.Set(key, value)
	}
	switch {
	case p.Listing.Shape == shapeAnthropic:
		header.Set("anthropic-version", "2023-06-01")
		if token != "" {
			header.Set("x-api-key", token)
		}
	case token != "":
		header.Set("Authorization", "Bearer "+token)
	}
	return page, header, nil
}

// fetchListing reads one page. A refusal is reported by its status alone:
// the body may echo the request.
func (g *Gateway) fetchListing(ctx context.Context, page *url.URL, header http.Header) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, page.String(), nil)
	if err != nil {
		return nil, err
	}
	request.Header = header
	response, err := g.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("provider listing answered %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, listingBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > listingBytes {
		return nil, errors.New("provider listing is too large")
	}
	return body, nil
}

// mergeListing is p with the models its listing adds after the curated ones.
// A curated model the listing names keeps its curated facts, and takes the
// listing's context when it had none.
func mergeListing(p *provider, record listingRecord) *provider {
	if len(record.Models) == 0 {
		return p
	}
	merged := *p
	merged.Models = make([]model, len(p.Models), len(p.Models)+len(record.Models))
	copy(merged.Models, p.Models)
	curated := make(map[string]int, len(p.Models))
	for i := range p.Models {
		curated[strings.ToLower(p.Models[i].ID)] = i
	}
	for _, m := range record.Models {
		if i, ok := curated[strings.ToLower(m.ID)]; ok {
			if merged.Models[i].Context == 0 {
				merged.Models[i].Context = m.Context
			}
			continue
		}
		tier, offered := listedTier(p, m)
		if !offered {
			continue
		}
		merged.Models = append(merged.Models, model{
			ID: m.ID, Name: firstText(m.Name, m.ID), Tier: tier, Context: m.Context,
			Tools: m.ToolsKnown && m.Tools, Reasoning: m.Reasoning, Vision: m.Vision,
			listed: true, toolsUnknown: !m.ToolsKnown,
		})
	}
	return &merged
}

// listedTier is the tier a listed model is routed by, and whether Rashin
// offers it at all. Free stays honest: a mixed provider's model is free only
// when its listing says so, and a free provider's model the listing prices
// is paid. A provider of renewed credits spends them on what it prices.
func listedTier(p *provider, m listedModel) (string, bool) {
	switch {
	case p.Tier == protocol.TierPaid:
		return protocol.TierPaid, true
	case m.Free:
		return p.Tier, true
	case p.Keyless && p.Listing.Mixed:
		// without a key only the free models answer
		return "", false
	case p.Keyless:
		// with no key there is no account to bill: the prices are a keyed plan's
		return p.Tier, true
	case p.Listing.Mixed, m.Priced && p.Tier == protocol.TierFree:
		return protocol.TierPaid, true
	default:
		return p.Tier, true
	}
}
