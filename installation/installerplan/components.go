package installerplan

import (
	_ "embed"
	"fmt"
	"strings"
	"sync"
)

type ComponentStep string

const (
	ComponentApps   ComponentStep = "apps"
	ComponentRashin ComponentStep = "rashin"
	StepGaming      ComponentStep = "gaming"
)

type Component struct {
	ID           string
	Label        string
	Group        string
	GroupLabel   string
	Packages     []string
	DesktopIDs   []string
	DefaultOn    bool
	Step         ComponentStep
	NeedsNetwork bool
	Editions     []string
	LockEditions []string
	LockReason   string
	Detail       string
}

type ComponentAvailability struct {
	Component Component
	Available bool
	Locked    bool
	Reason    string
}

//go:embed components.tsv
var componentsData []byte

var (
	componentsOnce   sync.Once
	sharedComponents []Component
	componentsErr    error
)

func loadComponents() ([]Component, error) {
	componentsOnce.Do(func() {
		sharedComponents, componentsErr = parseComponents(componentsData)
	})
	return sharedComponents, componentsErr
}

func parseComponents(data []byte) ([]Component, error) {
	var components []Component
	seen := map[string]bool{}
	header := false
	for lineNo, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if !header {
			header = true
			if len(fields) != 13 || fields[0] != "id" || fields[1] != "label" || fields[2] != "group" {
				return nil, fmt.Errorf("installer components line %d has an invalid header", lineNo+1)
			}
			continue
		}
		if len(fields) != 13 {
			return nil, fmt.Errorf("installer components line %d must have thirteen tab-separated columns", lineNo+1)
		}
		component := Component{
			ID: fields[0], Label: fields[1], Group: fields[2], GroupLabel: fields[3],
			Packages: splitList(fields[4]), DesktopIDs: splitList(fields[5]),
			DefaultOn: fields[6] == "on", Step: ComponentStep(fields[7]),
			NeedsNetwork: fields[8] == "true", Editions: splitList(fields[9]),
			LockEditions: splitList(fields[10]), LockReason: dashEmpty(fields[11]),
			Detail: dashEmpty(fields[12]),
		}
		if component.ID == "" || component.Label == "" || component.Group == "" || component.GroupLabel == "" {
			return nil, fmt.Errorf("installer components line %d is incomplete", lineNo+1)
		}
		if component.Step != ComponentApps && component.Step != ComponentRashin && component.Step != StepGaming {
			return nil, fmt.Errorf("installer components line %d has unknown step %q", lineNo+1, component.Step)
		}
		if fields[6] != "on" && fields[6] != "off" {
			return nil, fmt.Errorf("installer components line %d has invalid default %q", lineNo+1, fields[6])
		}
		if fields[8] != "true" && fields[8] != "false" {
			return nil, fmt.Errorf("installer components line %d has invalid network flag %q", lineNo+1, fields[8])
		}
		if len(component.Editions) == 0 {
			return nil, fmt.Errorf("installer components line %d has no editions", lineNo+1)
		}
		if len(component.Packages) == 0 && len(component.DesktopIDs) == 0 {
			return nil, fmt.Errorf("installer components line %d has no package or desktop id", lineNo+1)
		}
		if len(component.LockEditions) > 0 && component.LockReason == "" {
			return nil, fmt.Errorf("installer components line %d locks without a reason", lineNo+1)
		}
		if seen[component.ID] {
			return nil, fmt.Errorf("installer components line %d repeats %s", lineNo+1, component.ID)
		}
		seen[component.ID] = true
		components = append(components, component)
	}
	if !header {
		return nil, fmt.Errorf("installer components have no header")
	}
	if len(components) == 0 {
		return nil, fmt.Errorf("installer components have no entries")
	}
	return components, nil
}

func splitList(value string) []string {
	value = dashEmpty(value)
	if value == "" {
		return nil
	}
	return strings.Split(value, ",")
}

func Components() []Component {
	components, err := loadComponents()
	if err != nil {
		panic(err)
	}
	return append([]Component(nil), components...)
}

func ComponentByID(id string) (Component, bool) {
	for _, component := range Components() {
		if component.ID == id {
			return component, true
		}
	}
	return Component{}, false
}

func canonicalEdition(edition string) string {
	switch strings.ToLower(strings.TrimSpace(edition)) {
	case "", "plain", "arch":
		return "arch"
	case "cachyos":
		return "cachyos"
	default:
		return strings.ToLower(strings.TrimSpace(edition))
	}
}

func listedForEdition(editions []string, edition string) bool {
	for _, candidate := range editions {
		if candidate == "*" || candidate == edition {
			return true
		}
	}
	return false
}

func resolveComponents(edition string, translations map[string]translationRow) []ComponentAvailability {
	edition = canonicalEdition(edition)
	components := Components()
	out := make([]ComponentAvailability, 0, len(components))
	for _, component := range components {
		availability := ComponentAvailability{Component: component, Available: listedForEdition(component.Editions, edition)}
		if !availability.Available {
			availability.Reason = fmt.Sprintf("not shipped on %s", edition)
		} else if translations != nil {
			for _, pkg := range component.Packages {
				row, ok := translations[pkg]
				if ok && ((row.target != "" && row.target != "-") ||
					(row.target == "-" && strings.HasPrefix(row.reason, "Included in "))) {
					continue
				}
				availability.Available = false
				if ok && row.reason != "" {
					availability.Reason = row.reason
				} else {
					availability.Reason = fmt.Sprintf("%s is not listed in the %s edition table", pkg, edition)
				}
				break
			}
		}
		if availability.Available && listedForEdition(component.LockEditions, edition) {
			availability.Locked = true
			availability.Reason = component.LockReason
		}
		out = append(out, availability)
	}
	return out
}

func (p EditionPlan) Components() []ComponentAvailability {
	return append([]ComponentAvailability(nil), p.components...)
}

func (p EditionPlan) Component(id string) (ComponentAvailability, bool) {
	for _, component := range p.components {
		if component.Component.ID == id {
			return component, true
		}
	}
	return ComponentAvailability{}, false
}

// GamingComponent returns the one gaming choice offered by this edition.
func (p EditionPlan) GamingComponent() (ComponentAvailability, bool) {
	for _, component := range p.components {
		if component.Available && component.Component.Step == StepGaming {
			return component, true
		}
	}
	return ComponentAvailability{}, false
}

func (p EditionPlan) DefaultComponents() map[string]bool {
	selected := map[string]bool{}
	for _, availability := range p.components {
		if availability.Available {
			selected[availability.Component.ID] = availability.Component.DefaultOn || availability.Locked
		}
	}
	return selected
}

func componentSelected(availability ComponentAvailability, selected map[string]bool) bool {
	on := availability.Component.DefaultOn
	if value, ok := selected[availability.Component.ID]; ok {
		on = value
	}
	return on || availability.Locked
}

func (p EditionPlan) DropComponentPackages(selected map[string]bool) []string {
	var packages []string
	for _, availability := range p.components {
		component := availability.Component
		// Offline optional bundles are baked into images and named by the fresh
		// install transaction, so an off choice must remove them explicitly.
		dropWhenOff := component.DefaultOn || component.Step == ComponentRashin ||
			(component.Step == StepGaming && !component.NeedsNetwork)
		if !availability.Available || !dropWhenOff || componentSelected(availability, selected) {
			continue
		}
		packages = append(packages, component.Packages...)
	}
	return packages
}

func (p EditionPlan) HiddenDesktopIDs(selected map[string]bool) []string {
	var ids []string
	for _, availability := range p.components {
		if !availability.Available || !availability.Component.DefaultOn || componentSelected(availability, selected) {
			continue
		}
		ids = append(ids, availability.Component.DesktopIDs...)
	}
	return ids
}
