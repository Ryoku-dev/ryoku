package gateway

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

//go:embed catalog.json
var catalogBytes []byte

type catalog struct {
	Providers []provider
	byID      map[string]*provider
}

type provider struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Tier         string            `json:"tier"`
	BaseURL      string            `json:"baseURL"`
	Auth         string            `json:"auth"`
	Keyless      bool              `json:"keyless"`
	Signup       string            `json:"signup"`
	KeyHint      string            `json:"keyHint"`
	ExtraHeaders map[string]string `json:"extraHeaders"`
	Note         string            `json:"note"`
	Models       []model           `json:"models"`
}

type model struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Context     int    `json:"context"`
	Tools       bool   `json:"tools"`
	Reasoning   bool   `json:"reasoning"`
	Vision      bool   `json:"vision"`
	QualityRank int    `json:"qualityRank"`
	SpeedRank   int    `json:"speedRank"`
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
		if p.ID == "" || p.Name == "" || p.BaseURL == "" || len(p.Models) == 0 {
			return catalog{}, fmt.Errorf("provider catalogue entry %q is incomplete", p.ID)
		}
		if p.Auth != "bearer" && p.Auth != "none" && p.Auth != "cloudflare" {
			return catalog{}, fmt.Errorf("provider %q has unsupported auth %q", p.ID, p.Auth)
		}
		if p.Keyless && p.Auth != "none" {
			return catalog{}, fmt.Errorf("keyless provider %q must use auth none", p.ID)
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
