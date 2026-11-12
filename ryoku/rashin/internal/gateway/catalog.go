package gateway

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"ryoku-rashin/internal/protocol"
)

//go:embed catalog.json
var catalogBytes []byte

type catalog struct {
	Providers []provider
	byID      map[string]*provider
}

type provider struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Tier    string `json:"tier"`
	BaseURL string `json:"baseURL"`
	Auth    string `json:"auth"`
	Keyless bool   `json:"keyless"`
	// CompletionTokens sends the length limit as max_completion_tokens, which
	// reasoning models take instead of max_tokens.
	CompletionTokens bool              `json:"completionTokens"`
	Signup           string            `json:"signup"`
	KeyHint          string            `json:"keyHint"`
	ExtraHeaders     map[string]string `json:"extraHeaders"`
	Note             string            `json:"note"`
	Listing          listingSpec       `json:"listing"`
	// Models are the curated ones; the provider's own listing adds the rest
	// (see discover.go).
	Models []model `json:"models"`

	// running is a local provider's server answering when it was last asked
	running bool
}

// listingSpec says where and how a provider lists its models.
type listingSpec struct {
	// Shape is how the listing reads: openai (the default), anthropic,
	// cloudflare, cohere or github.
	Shape string `json:"shape"`
	// URL replaces {baseURL}/models.
	URL string `json:"url"`
	// Public is a listing anyone may read, without a key.
	Public bool `json:"public"`
	// Mixed is a provider that sells some models and gives others away: a
	// model it lists is free only when the listing says so.
	Mixed bool `json:"mixed"`
}

type model struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Tier is the model's own when it differs from its provider's: a free
	// provider may also serve models its key's owner pays for.
	Tier        string `json:"tier"`
	Context     int    `json:"context"`
	Tools       bool   `json:"tools"`
	Reasoning   bool   `json:"reasoning"`
	Vision      bool   `json:"vision"`
	QualityRank int    `json:"qualityRank"`
	SpeedRank   int    `json:"speedRank"`

	// listed is a model only the provider's own listing names
	listed bool
	// toolsUnknown is a listed model whose listing does not say whether it
	// takes tools: Auto, Fast and Smart pass it by, a pick by name reaches it
	toolsUnknown bool
}

// tierOf is the tier a model is routed and listed by.
func (p *provider) tierOf(m *model) string {
	if m.Tier != "" {
		return m.Tier
	}
	return p.Tier
}

// catalogueTier reports a tier the embedded catalogue may name; local
// providers are found on this computer, never listed.
func catalogueTier(tier string) bool {
	return tier == protocol.TierFree || tier == protocol.TierCredits || tier == protocol.TierPaid
}

func loadCatalog() (catalog, error) {
	var providers []provider
	if err := json.Unmarshal(catalogBytes, &providers); err != nil {
		return catalog{}, fmt.Errorf("decode embedded provider catalogue: %w", err)
	}
	c := catalog{Providers: providers, byID: make(map[string]*provider, len(providers))}
	for i := range c.Providers {
		p := &c.Providers[i]
		p.ID = strings.ToLower(strings.TrimSpace(p.ID))
		if p.ID == "" || p.Name == "" || p.BaseURL == "" {
			return catalog{}, fmt.Errorf("provider catalogue entry %q is incomplete", p.ID)
		}
		if !listingShape(p.Listing.Shape) {
			return catalog{}, fmt.Errorf("provider %q has unknown listing shape %q", p.ID, p.Listing.Shape)
		}
		if p.Auth != "bearer" && p.Auth != "none" && p.Auth != "cloudflare" {
			return catalog{}, fmt.Errorf("provider %q has unsupported auth %q", p.ID, p.Auth)
		}
		if p.Keyless && p.Auth != "none" {
			return catalog{}, fmt.Errorf("keyless provider %q must use auth none", p.ID)
		}
		if !catalogueTier(p.Tier) {
			return catalog{}, fmt.Errorf("provider %q has unknown tier %q", p.ID, p.Tier)
		}
		if p.Keyless && p.Tier == protocol.TierPaid {
			return catalog{}, fmt.Errorf("keyless provider %q cannot be paid", p.ID)
		}
		if _, exists := c.byID[p.ID]; exists {
			return catalog{}, fmt.Errorf("provider catalogue repeats %q", p.ID)
		}
		seen := make(map[string]struct{}, len(p.Models))
		for j := range p.Models {
			m := &p.Models[j]
			if m.ID == "" || m.Name == "" {
				return catalog{}, fmt.Errorf("provider %q has an incomplete model", p.ID)
			}
			if m.Tier != "" && !catalogueTier(m.Tier) {
				return catalog{}, fmt.Errorf("provider %q model %q has unknown tier %q", p.ID, m.ID, m.Tier)
			}
			if _, exists := seen[m.ID]; exists {
				return catalog{}, fmt.Errorf("provider %q repeats model %q", p.ID, m.ID)
			}
			if p.ID == "openrouter" && !strings.HasSuffix(m.ID, ":free") {
				return catalog{}, fmt.Errorf("OpenRouter model %q is not a free route", m.ID)
			}
			seen[m.ID] = struct{}{}
		}
		c.byID[p.ID] = p
	}
	sort.Slice(c.Providers, func(i, j int) bool { return c.Providers[i].Name < c.Providers[j].Name })
	c.byID = make(map[string]*provider, len(c.Providers))
	for i := range c.Providers {
		c.byID[c.Providers[i].ID] = &c.Providers[i]
	}
	return c, nil
}

func (c catalog) findProvider(id string) (*provider, bool) {
	p, ok := c.byID[strings.ToLower(strings.TrimSpace(id))]
	return p, ok
}

func (p *provider) findModel(id string) (*model, bool) {
	for i := range p.Models {
		if p.Models[i].ID == id {
			return &p.Models[i], true
		}
	}
	return nil, false
}
