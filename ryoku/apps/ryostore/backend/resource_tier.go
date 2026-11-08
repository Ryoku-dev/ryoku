package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// tiers come from the catalogue's lab runs against Orpheus (ryostore-revamped measurements/)
const (
	tierLight    = "light"
	tierStandard = "standard"
	tierHeavy    = "heavy"
)

func validResourceTier(tier string) bool {
	return tier == tierLight || tier == tierStandard || tier == tierHeavy
}

type ProductMetadata struct {
	ResourceTier  string         `json:"resourceTier"`
	ResourceDelta *ResourceDelta `json:"resourceDelta,omitempty"`
}

type ResourceDelta struct {
	PSSMB          float64 `json:"pssMB"`
	GPUMB          float64 `json:"gpuMB"`
	CPUPct         float64 `json:"cpuPct"`
	WakeupsPerSec  float64 `json:"wakeupsPerSec"`
	LaunchesPerMin float64 `json:"launchesPerMin"`
}

func (m *ProductMetadata) UnmarshalJSON(data []byte) error {
	if err := strictJSONFields(data, map[string]bool{"resourceTier": true, "resourceDelta": true}); err != nil {
		return err
	}
	type plain ProductMetadata
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*m = ProductMetadata(decoded)
	return nil
}

func (d *ResourceDelta) UnmarshalJSON(data []byte) error {
	if err := strictJSONFields(data, map[string]bool{
		"pssMB": true, "gpuMB": true, "cpuPct": true, "wakeupsPerSec": true, "launchesPerMin": true,
	}); err != nil {
		return err
	}
	type plain ResourceDelta
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*d = ResourceDelta(decoded)
	return nil
}

func validateProductMetadata(category string, metadata *ProductMetadata) error {
	if metadata == nil {
		return nil
	}
	if !shellCategory(category) {
		return fmt.Errorf("metadata is only defined for subshells and bar styles")
	}
	if !validResourceTier(metadata.ResourceTier) {
		return fmt.Errorf("invalid resource tier %q", metadata.ResourceTier)
	}
	return nil
}

func itemMetadata(metadata *ProductMetadata) map[string]any {
	if metadata == nil {
		return nil
	}
	out := map[string]any{"resourceTier": metadata.ResourceTier}
	if metadata.ResourceDelta != nil {
		out["resourceDelta"] = *metadata.ResourceDelta
	}
	return out
}

func itemResourceTier(item *Item) string {
	tier, _ := item.Metadata["resourceTier"].(string)
	if validResourceTier(tier) {
		return tier
	}
	return ""
}

// the shipped base leads since every tier is measured against it
func recommendRank(item *Item, machine Machine) int {
	if !machine.Modest || isBuiltinCatalogItem(*item) {
		return 0
	}
	switch itemResourceTier(item) {
	case tierLight:
		return 0
	case tierStandard:
		return 1
	case tierHeavy:
		return 3
	default:
		return 2
	}
}

// the store and the Hub gallery both list a category in this order
func adviseShellItems(items []Item, machine Machine) {
	for i := range items {
		items[i].NotRecommended = machine.Modest && itemResourceTier(&items[i]) == tierHeavy
	}
	sort.SliceStable(items, func(a, b int) bool {
		ra := recommendRank(&items[a], machine)
		rb := recommendRank(&items[b], machine)
		if ra != rb {
			return ra < rb
		}
		na, nb := strings.ToLower(displayName(items[a])), strings.ToLower(displayName(items[b]))
		if na != nb {
			return na < nb
		}
		return items[a].ID < items[b].ID
	})
}

func displayName(item Item) string {
	if item.Name != "" {
		return item.Name
	}
	return item.ID
}
