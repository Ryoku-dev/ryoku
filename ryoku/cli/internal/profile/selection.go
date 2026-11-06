package profile

import (
	"fmt"
	"sort"
)

func (m *Manifest) Targets(sel Selection) (comps []Component, extras []string, err error) {
	byName := map[string]Component{}
	selected := map[string]bool{}
	without := map[string]bool{}
	for _, name := range sel.Without {
		without[name] = true
	}
	for _, c := range m.Components {
		byName[c.Name] = c
		if c.Tier == "core" && !without[c.Name] {
			selected[c.Name] = true
		}
	}
	for _, name := range sel.Without {
		if _, ok := byName[name]; !ok {
			return nil, nil, fmt.Errorf("unknown component %q", name)
		}
	}
	for _, name := range sel.Components {
		if _, ok := byName[name]; !ok {
			return nil, nil, fmt.Errorf("unknown component %q", name)
		}
		selected[name] = true
	}
	for name := range selected {
		comps = append(comps, byName[name])
	}
	sort.Slice(comps, func(i, j int) bool { return comps[i].Name < comps[j].Name })
	seen := map[string]bool{}
	for _, name := range sel.Extras {
		if !packagePattern.MatchString(name) {
			return nil, nil, fmt.Errorf("unknown extra %q: invalid package name", name)
		}
		if _, isComponent := byName[name]; isComponent {
			return nil, nil, fmt.Errorf("extra %q is already a profile component", name)
		}
		if !seen[name] {
			seen[name] = true
			extras = append(extras, name)
		}
	}
	sort.Strings(extras)
	return comps, extras, nil
}
