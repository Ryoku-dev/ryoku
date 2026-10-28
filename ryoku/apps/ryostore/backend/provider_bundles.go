// The bundles provider adapts the ryostore bundle registry into the store
// contract. Browsing fetches bundles/registry.json only: each entry carries its
// inline components (type, name, detect, tier, interactive, summary), so no
// per-bundle definition or installer is fetched to render the catalogue.
// Installed state is joined from `ryostore-install status`, which reads the
// same inline components. Install launches the actuator in a floating terminal,
// which fetches the selected bundle's full definition on demand and owns the
// privileged package work; Settings owns installed bundle status and removal.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type registryEntry struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Tagline     string            `json:"tagline,omitempty"`
	Sources     string            `json:"sources,omitempty"`
	Icon        string            `json:"icon,omitempty"`
	Accent      string            `json:"accent,omitempty"`
	Preview     string            `json:"preview,omitempty"`
	Screenshots []string          `json:"screenshots,omitempty"`
	Path        string            `json:"path"`
	Components  []bundleComponent `json:"components"`
	// PackageManagers limits the whole bundle to some editions, as on products.
	PackageManagers       []string `json:"packageManagers,omitempty"`
	PackageManagersReason string   `json:"packageManagersReason,omitempty"`
}

type registry struct {
	Version int             `json:"version"`
	Bundles []registryEntry `json:"bundles"`
}

// bundleComponent is one item as carried inline in the registry: enough to
// render the catalogue and detect installed state without fetching bundle.json.
type bundleComponent struct {
	Type                  string   `json:"type"`
	Name                  string   `json:"name"`
	Detect                string   `json:"detect,omitempty"`
	Tier                  string   `json:"tier,omitempty"`
	Group                 string   `json:"group,omitempty"`
	Interactive           bool     `json:"interactive,omitempty"`
	Summary               string   `json:"summary,omitempty"`
	PackageManagers       []string `json:"packageManagers,omitempty"`
	PackageManagersReason string   `json:"packageManagersReason,omitempty"`
}

// partState is one bundle part as the actuator reports it on this host:
// present, absent, or unavailable with the edition's reason.
type partState struct {
	State  string
	Reason string
}

// bundleStatus indexes part states by bundle id, then part name.
type bundleStatus map[string]map[string]partState

type bundleProvider struct {
	cache *Cache
	// status reads every part's state from the actuator, which names packages
	// for this edition through the cached package table (overlay path).
	status func(ctx context.Context, overlay string) bundleStatus
	launch func(id string, only []string) error
}

func (bundleProvider) Category() Category {
	return Category{
		ID:          "bundles",
		Name:        "Bundles",
		Group:       "EXTEND",
		Description: "Curated sets of packages, scripts, and guests installed together.",
	}
}

func (p bundleProvider) Load(ctx context.Context, refresh bool) ([]Item, SourceState, error) {
	raw, state, err := p.cache.Fetch(ctx, "bundles/registry.json", refresh)
	if err != nil {
		return nil, state, err
	}
	var reg registry
	if err := json.Unmarshal(raw, &reg); err != nil {
		return nil, state, fmt.Errorf("bundles/registry.json: %w", err)
	}

	type built struct {
		entry registryEntry
		item  Item
		items []bundleComponent
	}
	out := make([]built, 0, len(reg.Bundles))
	for _, e := range reg.Bundles {
		if !validComponent(e.ID) {
			return nil, state, fmt.Errorf("bundle has invalid id %q", e.ID)
		}
		path := e.Path
		if path == "" {
			path = "bundles/" + e.ID
		}
		if !validLocalPath(path) {
			return nil, state, fmt.Errorf("bundle %q has invalid path %q", e.ID, path)
		}
		if len(e.Components) == 0 {
			return nil, state, fmt.Errorf("bundle %q has no components", e.ID)
		}

		md := map[string]any{}
		if e.Sources != "" {
			md["sources"] = e.Sources
		}
		if e.Icon != "" {
			md["icon"] = e.Icon
		}
		if e.Accent != "" {
			md["accent"] = e.Accent
		}
		comps := make([]map[string]any, len(e.Components))
		for i, c := range e.Components {
			comp := map[string]any{"type": c.Type, "name": c.Name}
			if c.Summary != "" {
				comp["summary"] = c.Summary
			}
			if c.Tier != "" {
				comp["tier"] = c.Tier
			}
			if c.Group != "" {
				comp["group"] = c.Group
			}
			comps[i] = comp
		}
		md["items"] = comps

		out = append(out, built{
			entry: e,
			item: Item{
				ID:          e.ID,
				Category:    "bundles",
				Name:        e.Name,
				Summary:     e.Tagline,
				Description: e.Description,
				Art:         resolveAsset(extrasBase(), path, e.Preview),
				Screenshots: resolveAssets(extrasBase(), path, e.Screenshots),
				TotalCount:  len(e.Components),
				Metadata:    md,
				// Carried whatever the host, so the revision moves when a
				// bundle gains or drops an edition gate.
				RequiredPackageManagers: e.PackageManagers,
			},
			items: e.Components,
		})
	}

	// Join each part's state on this host; the actuator reads the same inline
	// components from the cached registry and names packages for this edition
	// through the cached package table. A missing or failed status source
	// degrades to nothing installed rather than an error. A part this host cannot
	// have stays listed, greyed with its reason, and leaves the counts: a bundle
	// is complete when every part this edition can have is installed.
	var status bundleStatus
	if p.status != nil {
		status = p.status(ctx, ensureBundlePackageTable(ctx, p.cache, refresh))
	}
	items := make([]Item, len(out))
	for i, b := range out {
		it := b.item
		parts := status[it.ID]
		components, _ := it.Metadata["items"].([]map[string]any)
		it.TotalCount = 0
		for j, comp := range b.items {
			part := parts[comp.Name]
			if ok, _ := productPackageManagerGate(comp.PackageManagers); !ok {
				part = partState{State: "unavailable", Reason: packageManagerReason(comp.PackageManagersReason)}
			}
			installed := part.State == "present"
			available := part.State != "unavailable"
			if available {
				it.TotalCount++
			}
			if installed {
				it.InstalledCount++
			}
			if j < len(components) {
				components[j]["installed"] = installed
				components[j]["available"] = available
				if !available {
					components[j]["unavailableReason"] = part.Reason
				}
			}
		}
		it.Installed = it.TotalCount > 0 && it.InstalledCount == it.TotalCount
		if ok, _ := productPackageManagerGate(b.entry.PackageManagers); !ok {
			it.Unavailable = true
			it.UnavailableEdition = true
			it.UnavailableReason = packageManagerReason(b.entry.PackageManagersReason)
		} else if len(b.items) > 0 && it.TotalCount == 0 {
			it.Unavailable = true
			it.UnavailableEdition = true
			it.UnavailableReason = "None of this bundle's parts can be installed on this edition of Ryoku."
		}
		items[i] = it
	}
	return items, state, nil
}

func (p bundleProvider) Install(ctx context.Context, id string) error {
	reportProgressNow(ctx, "apply", 0, 0)
	return p.launch(id, nil)
}

// InstallComponents installs only the named components (the store's manual
// selection); an empty list falls back to the whole-bundle install.
func (p bundleProvider) InstallComponents(ctx context.Context, id string, only []string) error {
	reportProgressNow(ctx, "apply", 0, 0)
	return p.launch(id, only)
}

// defaultBundleStatus queries the actuator for every bundle's part states and
// indexes them by bundle id then part name. Any failure yields no status rather
// than an error, so a broken actuator cannot blank the catalogue. overlay is the
// cached package table for this edition, handed over so the actuator need not
// call back into the store for it.
func defaultBundleStatus(ctx context.Context, overlay string) bundleStatus {
	cmd := exec.CommandContext(ctx, "ryostore-install", "status")
	if overlay != "" {
		cmd.Env = append(os.Environ(), "RYOKU_HOST_PKG_OVERLAY="+overlay)
	}
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var parsed struct {
		Bundles []struct {
			ID    string `json:"id"`
			Items []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
				Reason string `json:"reason"`
			} `json:"items"`
		} `json:"bundles"`
	}
	if json.Unmarshal(out, &parsed) != nil {
		return nil
	}
	m := make(bundleStatus, len(parsed.Bundles))
	for _, b := range parsed.Bundles {
		parts := make(map[string]partState, len(b.Items))
		for _, it := range b.Items {
			parts[it.Name] = partState{State: it.Status, Reason: it.Reason}
		}
		m[b.ID] = parts
	}
	return m
}

// launchBundleInstall runs the actuator's bundle install in a floating kitty
// terminal, in its own session so it owns the sudo prompt and long-running
// output. It waits for that terminal to close: the store refreshes the
// catalogue when the install really ends, not the instant kitty is spawned,
// and an install-all queue runs one bundle at a time instead of a terminal
// per item. The separate session keeps the install alive if the store closes
// mid-run.
func launchBundleInstall(id string, only []string) error {
	args := []string{"--class", "ryostore", "-e", "ryostore-install", "install", "bundle", id}
	if len(only) > 0 {
		args = append(args, "--only", strings.Join(only, ","))
	}
	cmd := exec.Command("kitty", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Run()
}
