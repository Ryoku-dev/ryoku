package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Resource tiers are measured by the catalogue, one record per subshell or bar
// style against the shipped Orpheus (ryostore-revamped measurements/), and
// published on the registry row as metadata.resourceTier with the measured
// metadata.resourceDelta. Ryostore shows them and, on a modest Machine, advises
// against the heavy ones; it never refuses one.
const (
	tierLight    = "light"
	tierStandard = "standard"
	tierHeavy    = "heavy"
)

func validResourceTier(tier string) bool {
	return tier == tierLight || tier == tierStandard || tier == tierHeavy
}

// ProductMetadata is the registry row's metadata object. Only the measured
// resource cost is defined; any other key fails the registry like any other
// unknown field.
type ProductMetadata struct {
	ResourceTier  string         `json:"resourceTier"`
	ResourceDelta *ResourceDelta `json:"resourceDelta,omitempty"`
}

// ResourceDelta is the measured difference from Orpheus on the lab machine:
// proportional memory, GPU memory, CPU, wakeups and process launches.
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

// itemMetadata is the item-side copy of a registry row's metadata, in the
// item's open metadata map the app and the Hub gallery read.
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

// recommendRank orders one shell category on a modest machine: the shipped
// base (Orpheus, Kairos: what every tier is measured against, held by the
// performance budget) and light items first, then standard, then items nobody
// has measured yet, then heavy. A machine with headroom ranks everything the
// same, so the category stays A to Z.
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

// adviseShellItems marks the heavy subshells or bar styles of one category not
// recommended on a modest machine and sorts the category into recommendation
// order (rank, then display name, then id), which is the order the store and
// the Hub gallery list them in.
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
