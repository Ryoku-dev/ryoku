// apps.go adapts the shared installer choices and component catalogue to TUI
// rows. Package ownership and edition availability stay in installerplan so the
// ISO and script installers render the same choices.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ryoku-i18n"
	installerplan "ryoku-installerplan"
)

// flowIndex finds the wizard index of a step by key; the flow shifts when
// steps are added, so nothing may index it by literal position.
func flowIndex(flow []step, key string) int {
	for i, s := range flow {
		if s.key == key {
			return i
		}
	}
	return -1
}

func installerChoicesForVariant(variant string) installerplan.EditionPlan {
	if !isVoidVariant(variant) {
		return installerplan.ResolveSource(variant)
	}
	var paths []string
	if root := os.Getenv("RYOKU_REPO"); root != "" {
		paths = append(paths, filepath.Join(root, "void/packages/translations.tsv"))
	}
	paths = append(paths,
		"/usr/share/ryoku/void/packages/translations.tsv",
		filepath.Join("..", "..", "void", "packages", "translations.tsv"),
	)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		plan, err := installerplan.ResolveEdition(data)
		if err != nil {
			panic(err)
		}
		return plan
	}
	panic(fmt.Sprintf("Void package table not found in %s", strings.Join(paths, ", ")))
}

func choiceItems(kind string, plan installerplan.EditionPlan, availableOnly bool) []item {
	var items []item
	for _, availability := range plan.Choices(kind) {
		if availableOnly && !availability.Available {
			continue
		}
		choice := availability.Choice
		hint := i18n.T(choice.TUIHint)
		if !availability.Available {
			hint = i18n.Tf("Unavailable on %s · %s", plan.Edition, availability.Reason)
		}
		items = append(items, item{choice.ID, i18n.T(choice.Label), hint})
	}
	return items
}

func browsers() []item {
	return choiceItems(installerplan.Browser, installerplan.ResolveSource("Arch"), true)
}

// cachyosBrowserHints replaces the shared hint of a browser the CachyOS
// repositories serve, which that edition installs from the repo, not the AUR.
var cachyosBrowserHints = map[string]string{"zen": "vertical tabs · CachyOS repo"}

func browsersForVariant(variant string) []item {
	items := choiceItems(installerplan.Browser, installerChoicesForVariant(variant), true)
	if variant == variantCachyOS {
		for i := range items {
			if hint, ok := cachyosBrowserHints[items[i].key]; ok {
				items[i].hint = i18n.T(hint)
			}
		}
	}
	return items
}

// shippedBrowserCount is how many browsers the edition can install: the None
// row is a choice, not a browser, so it stays out of the step's description.
func shippedBrowserCount(variant string) int {
	n := 0
	for _, availability := range installerChoicesForVariant(variant).Choices(installerplan.Browser) {
		if availability.Available && !availability.Choice.NoPackages {
			n++
		}
	}
	return n
}

// browserLabel is the display name for a browser key, for Review and the rail.
func browserLabel(key string) string {
	for _, b := range browsers() {
		if b.key == key {
			return b.label
		}
	}
	return key
}

// loginShells returns the account shells in shared manifest order.
func loginShells() []item {
	return choiceItems(installerplan.LoginShell, installerplan.ResolveSource("Arch"), true)
}

func loginShellsForVariant(variant string) []item {
	return choiceItems(installerplan.LoginShell, installerChoicesForVariant(variant), true)
}

// loginShellLabel is the display name for a shell key, for Review and the rail.
func loginShellLabel(key string) string {
	for _, s := range loginShells() {
		if s.key == key {
			return s.label
		}
	}
	return key
}

func fileManagers() []item {
	return choiceItems(installerplan.FileManager, installerplan.ResolveSource("Arch"), true)
}

func fileManagersForVariant(variant string) []item {
	return choiceItems(installerplan.FileManager, installerChoicesForVariant(variant), true)
}

func fileManagerLabel(key string) string {
	for _, manager := range fileManagers() {
		if manager.key == key {
			return manager.label
		}
	}
	return key
}

// appRow adapts one shared component for the Apps & tools checklist.
type appRow struct {
	ID    string
	Name  string
	Pkgs  []string
	Def   bool
	Req   string
	Sub   string
	Group string
}

func appRows() []appRow {
	return appRowsForVariant("plain")
}

func appRowsForVariant(variant string) []appRow {
	plan := installerChoicesForVariant(variant)
	var rows []appRow
	lastGroup := ""
	for _, availability := range plan.Components() {
		component := availability.Component
		if component.Step != installerplan.ComponentApps || !availability.Available {
			continue
		}
		group := ""
		if component.Group != lastGroup {
			group = i18n.T(component.GroupLabel)
			lastGroup = component.Group
		}
		required := ""
		if availability.Locked {
			required = i18n.T(availability.Reason)
		}
		rows = append(rows, appRow{
			ID: component.ID, Name: i18n.T(component.Label),
			Pkgs: append([]string(nil), component.Packages...),
			Def:  component.DefaultOn || availability.Locked,
			Req:  required, Sub: i18n.T(component.Detail), Group: group,
		})
	}
	return rows
}

// appGroups returns the rows grouped for display: each group's rows in table
// order. A row carrying a Group header opens a new group.
func appGroups() [][]appRow {
	return appGroupsForVariant("")
}

func appGroupsForVariant(variant string) [][]appRow {
	var out [][]appRow
	for _, r := range appRowsForVariant(variant) {
		if r.Group != "" || len(out) == 0 {
			out = append(out, []appRow{r})
			continue
		}
		out[len(out)-1] = append(out[len(out)-1], r)
	}
	return out
}

// appRowByID finds a row by its stable id.
func appRowByID(id string) (appRow, bool) {
	return appRowByIDForVariant("", id)
}

func appRowByIDForVariant(variant, id string) (appRow, bool) {
	for _, r := range appRowsForVariant(variant) {
		if r.ID == id {
			return r, true
		}
	}
	return appRow{}, false
}

// appDefaults is the initial keep map: every row at its shipped default.
func appDefaults() map[string]bool {
	return appDefaultsForVariant("")
}

func appDefaultsForVariant(variant string) map[string]bool {
	m := map[string]bool{}
	for _, r := range appRowsForVariant(variant) {
		m[r.ID] = r.Def
	}
	return m
}

// appsSummary is the Review row for the app choice: "all" when nothing was
// removed, else the ids of the dropped rows.
func appsSummary(keep map[string]bool) string {
	return appsSummaryForVariant("", keep)
}

func appsSummaryForVariant(variant string, keep map[string]bool) string {
	var names []string
	for _, r := range appRowsForVariant(variant) {
		if r.Req == "" && !keep[r.ID] {
			names = append(names, r.ID)
		}
	}
	if len(names) == 0 {
		return i18n.T("all")
	}
	return i18n.Tf("minus %s", strings.Join(names, ", "))
}

// appsReviewCell is the Review row for the app choice. The keep map is nil
// until the apps step first loads, which reads as the shipped defaults.
func (m model) appsReviewCell() string {
	if m.keep == nil {
		return i18n.T("all")
	}
	return appsSummaryForVariant(m.variant, m.keep)
}

// selectedApps is the keep map, with the shipped defaults standing in for a
// wizard that never visited the apps step (a scripted handoff keeps every app).
func (m model) selectedApps() map[string]bool {
	if m.keep == nil {
		return appDefaultsForVariant(m.variant)
	}
	return m.keep
}

func (m model) componentSelections() map[string]bool {
	plan := installerChoicesForVariant(m.variant)
	selected := plan.DefaultComponents()
	for id, on := range m.selectedApps() {
		selected[id] = on
	}
	selected["rashin"] = def(m.picks["rashin"], "0") == "1"
	if gaming, ok := plan.GamingComponent(); ok {
		selected[gaming.Component.ID] = m.picks["gaming"] == "1"
	}
	for _, availability := range plan.Components() {
		if availability.Locked {
			selected[availability.Component.ID] = true
		}
	}
	return selected
}
