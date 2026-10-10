package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

//go:embed mousecatalog.json
var mouseCatalogueData []byte

type mouseCatalogueEntry struct {
	Vendor   string            `json:"vendor"`
	Products []string          `json:"products"`
	Aliases  []string          `json:"aliases"`
	Brand    string            `json:"brand"`
	Model    string            `json:"model"`
	Labels   map[string]string `json:"labels"`
	SideGrid bool              `json:"sideGrid,omitempty"`
}

var (
	mouseCatalogueOnce sync.Once
	mouseCatalogue     []mouseCatalogueEntry
	mouseCatalogueErr  error
)

func loadMouseCatalogue() ([]mouseCatalogueEntry, error) {
	mouseCatalogueOnce.Do(func() {
		if err := json.Unmarshal(mouseCatalogueData, &mouseCatalogue); err != nil {
			mouseCatalogueErr = fmt.Errorf("decode embedded mouse catalogue: %w", err)
			return
		}
		for i := range mouseCatalogue {
			entry := &mouseCatalogue[i]
			entry.Vendor = strings.ToLower(entry.Vendor)
			for j := range entry.Products {
				entry.Products[j] = strings.ToLower(entry.Products[j])
			}
			for j := range entry.Aliases {
				entry.Aliases[j] = normalizeMouseName(entry.Aliases[j])
			}
		}
	})
	return mouseCatalogue, mouseCatalogueErr
}

func normalizeMouseName(name string) string {
	name = strings.ToLower(name)
	var b strings.Builder
	space := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(r)
		default:
			space = true
		}
	}
	return b.String()
}

func matchMouseCatalogue(vendor, product uint16, name string) *mouseCatalogueEntry {
	catalogue, err := loadMouseCatalogue()
	if err != nil {
		return nil
	}
	vid := fmt.Sprintf("%04x", vendor)
	pid := fmt.Sprintf("%04x", product)
	normalized := normalizeMouseName(name)

	for i := range catalogue {
		entry := &catalogue[i]
		if entry.Vendor != vid {
			continue
		}
		for _, candidate := range entry.Products {
			if candidate == pid {
				return entry
			}
		}
	}
	for i := range catalogue {
		entry := &catalogue[i]
		for _, alias := range entry.Aliases {
			if alias != "" && strings.Contains(normalized, alias) {
				return entry
			}
		}
	}
	return nil
}

func copyMouseLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(labels))
	for code, label := range labels {
		if _, err := strconv.ParseUint(code, 10, 16); err == nil && strings.TrimSpace(label) != "" {
			out[code] = label
		}
	}
	return out
}
