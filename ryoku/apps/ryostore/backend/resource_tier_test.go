package main

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestRegistryResourceMetadataIsStrict(t *testing.T) {
	var entry ProductEntry
	good := `{"id":"obi","metadata":{"resourceTier":"heavy","resourceDelta":{"pssMB":96.5,"gpuMB":40,"cpuPct":0.7,"wakeupsPerSec":3.2,"launchesPerMin":0}}}`
	if err := json.Unmarshal([]byte(good), &entry); err != nil {
		t.Fatalf("decode measured entry: %v", err)
	}
	if entry.Metadata == nil || entry.Metadata.ResourceTier != tierHeavy || entry.Metadata.ResourceDelta.PSSMB != 96.5 {
		t.Fatalf("metadata = %+v", entry.Metadata)
	}
	for name, raw := range map[string]string{
		"unknown metadata key": `{"id":"obi","metadata":{"resourceTier":"heavy","featured":true}}`,
		"unknown delta key":    `{"id":"obi","metadata":{"resourceTier":"heavy","resourceDelta":{"pssMB":1,"vramMB":2}}}`,
	} {
		if err := json.Unmarshal([]byte(raw), &entry); err == nil {
			t.Errorf("%s: decoded, want an error", name)
		}
	}
	if err := validateProductMetadata("barstyles", &ProductMetadata{ResourceTier: "huge"}); err == nil {
		t.Error("an unknown tier must fail the registry")
	}
	if err := validateProductMetadata("rices", &ProductMetadata{ResourceTier: tierLight}); err == nil {
		t.Error("metadata outside subshells and bar styles must fail the registry")
	}
	if err := validateProductMetadata("subshells", nil); err != nil {
		t.Errorf("an unmeasured item is valid: %v", err)
	}
}

// names picked so A to Z differs from tier order
func measuredBarstyles(t *testing.T) {
	f := newShellFixture(t)
	for id, tier := range map[string]string{
		"aurora": tierHeavy, "birch": "", "cedar": tierStandard, "dune": tierLight, "ember": tierHeavy, "fjord": tierLight,
	} {
		f.addCatalogueItem("barstyles", id, testShellManifest(), defaultShellFiles(), func(entry *ProductEntry) {
			if tier != "" {
				entry.Metadata = &ProductMetadata{ResourceTier: tier, ResourceDelta: &ResourceDelta{PSSMB: 12, GPUMB: 4, CPUPct: 0.1}}
			}
		})
	}
	f.writeConfig("shell.json", map[string]any{"barStyle": "orpheus"})
}

func catalogueOrder(cat Catalog) (ids []string, flagged []string) {
	for _, item := range cat.Items {
		ids = append(ids, item.ID)
		if item.NotRecommended {
			flagged = append(flagged, item.ID)
		}
	}
	return ids, flagged
}

func TestModestMachineListsLightFirstAndFlagsHeavy(t *testing.T) {
	measuredBarstyles(t)
	useMachine(t, writeMeminfo(t, "5950000"), writeDRMTree(t, apu))
	cat := BuildCatalog(context.Background(), []Provider{newShellItemProvider("barstyles", nil)}, false)
	if !cat.Machine.Modest || cat.Machine.MemoryGB != 6 || cat.Machine.Graphics != graphicsIntegrated {
		t.Fatalf("machine = %+v", cat.Machine)
	}
	ids, flagged := catalogueOrder(cat)
	if want := []string{"dune", "fjord", "cedar", "birch", "aurora", "ember"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("order = %v, want light, standard, unmeasured, heavy: %v", ids, want)
	}
	if want := []string{"aurora", "ember"}; !reflect.DeepEqual(flagged, want) {
		t.Fatalf("not recommended = %v, want the heavy ones %v", flagged, want)
	}
	for _, item := range cat.Items {
		if item.NotRecommended && (item.Unavailable || item.DownloadPaused) {
			t.Fatalf("%s is gated, advice must never block: %+v", item.ID, item)
		}
	}
	raw, err := json.Marshal(cat.Items[len(cat.Items)-1])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"notRecommended":true`, `"resourceTier":"heavy"`, `"resourceDelta":{"pssMB":12,"gpuMB":4,"cpuPct":0.1,"wakeupsPerSec":0,"launchesPerMin":0}`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("item JSON lacks %s: %s", want, raw)
		}
	}
}

func TestCapableMachineKeepsAToZ(t *testing.T) {
	measuredBarstyles(t)
	useMachine(t, writeMeminfo(t, "16303420"), writeDRMTree(t, nvidia, apu))
	cat := BuildCatalog(context.Background(), []Provider{newShellItemProvider("barstyles", nil)}, false)
	if cat.Machine.Modest {
		t.Fatalf("machine = %+v, want headroom", cat.Machine)
	}
	ids, flagged := catalogueOrder(cat)
	if want := []string{"aurora", "birch", "cedar", "dune", "ember", "fjord"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("order = %v, want A to Z %v", ids, want)
	}
	if len(flagged) != 0 {
		t.Fatalf("not recommended = %v on a machine with headroom", flagged)
	}
}

func TestShippedBaseLeadsOnModestMachine(t *testing.T) {
	items := []Item{
		{ID: "tansu", Name: "Tansu", Metadata: map[string]any{"resourceTier": tierHeavy}},
		{ID: "python", Name: "Python", Metadata: map[string]any{"resourceTier": tierStandard}},
		{ID: "orpheus", Name: "Orpheus", Metadata: map[string]any{"core": true}},
		{ID: "caelestia", Name: "Caelestia"},
		{ID: "kairos", Name: "Kairos", Metadata: map[string]any{"core": true}},
	}
	adviseShellItems(items, Machine{MemoryGB: 4, Graphics: graphicsIntegrated, Modest: true})
	ids, flagged := catalogueOrder(Catalog{Items: items})
	if want := []string{"kairos", "orpheus", "python", "caelestia", "tansu"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("order = %v, want %v", ids, want)
	}
	if !reflect.DeepEqual(flagged, []string{"tansu"}) {
		t.Fatalf("not recommended = %v, want [tansu]", flagged)
	}
}

func TestSnapshotRebuildsWhenMachineChanges(t *testing.T) {
	useMachine(t, writeMeminfo(t, "16303420"), writeDRMTree(t, nvidia))
	current, err := json.Marshal(Catalog{Machine: probeMachine()})
	if err != nil {
		t.Fatal(err)
	}
	if snapshotMachineChanged(current) {
		t.Fatal("a snapshot from this machine must be served")
	}
	if !snapshotMachineChanged([]byte(`{"items":[]}`)) {
		t.Fatal("a snapshot that never recorded its machine must be rebuilt")
	}
	useMachine(t, writeMeminfo(t, "16303420"), writeDRMTree(t, apu))
	if !snapshotMachineChanged(current) {
		t.Fatal("a snapshot from before a GPU change must be rebuilt")
	}
}

func TestRevisionMovesWithTierOnly(t *testing.T) {
	item := Item{Category: "barstyles", ID: "obi", Version: "1.0.0", ManifestSHA256: "x"}
	unmeasured := catalogRevision(Catalog{Items: []Item{item}})
	item.Metadata = map[string]any{"resourceTier": tierStandard, "resourceDelta": ResourceDelta{PSSMB: 50}}
	standard := catalogRevision(Catalog{Items: []Item{item}})
	if standard == unmeasured {
		t.Fatal("gaining a tier must change the revision")
	}
	item.Metadata = map[string]any{"resourceTier": tierStandard, "resourceDelta": ResourceDelta{PSSMB: 61}}
	if catalogRevision(Catalog{Items: []Item{item}}) != standard {
		t.Fatal("a re-measured delta inside the same tier must not change the revision")
	}
	item.Metadata = map[string]any{"resourceTier": tierHeavy}
	if catalogRevision(Catalog{Items: []Item{item}}) == standard {
		t.Fatal("moving to another tier must change the revision")
	}
}
