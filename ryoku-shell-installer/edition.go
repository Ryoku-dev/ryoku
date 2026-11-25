package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"ryoku-i18n"
	installerplan "ryoku-installerplan"
)

type editionCompositorPlan struct {
	edition string
	choices installerplan.EditionPlan
}

type planOverrides struct {
	browser      string
	shell        string
	fileManager  string
	hyakki       bool
	gaming       bool
	cachyosLayer bool
	without      []string
}

func compositorLabel(name string) string {
	if choice, ok := installerplan.ChoiceByID(installerplan.Compositor, name); ok {
		return choice.Label
	}
	return name
}

func editionPlanFromChoices(choices installerplan.EditionPlan) editionCompositorPlan {
	return editionCompositorPlan{
		edition: choices.Edition,
		choices: choices,
	}
}

func parseEditionCompositors(data []byte) (editionCompositorPlan, error) {
	choices, err := installerplan.ResolveEdition(data)
	if err != nil {
		return editionCompositorPlan{}, err
	}
	return editionPlanFromChoices(choices), nil
}

func sourceEditionChoices(name string) editionCompositorPlan {
	return editionPlanFromChoices(installerplan.ResolveSource(name))
}

func validateEditionChoice(kind, id string, plan editionCompositorPlan) error {
	availability, ok := plan.choices.Lookup(kind, id)
	if !ok {
		return nil
	}
	if availability.Available {
		return nil
	}
	return fmt.Errorf(i18n.T("%s choice %q is unavailable on %s: %s"),
		availability.Choice.Label, id, plan.edition, availability.Reason)
}

func validateEditionFlags(overrides planOverrides, plan editionCompositorPlan) error {
	if err := validateBrowser(overrides.browser); err != nil {
		return err
	}
	if err := validateEditionChoice(installerplan.Browser, overrides.browser, plan); err != nil {
		return err
	}
	if overrides.shell != "" {
		if err := validateLoginShell(overrides.shell); err != nil {
			return err
		}
		if err := validateEditionChoice(installerplan.LoginShell, overrides.shell, plan); err != nil {
			return err
		}
	}
	if overrides.fileManager != "" {
		if err := validateFileManager(overrides.fileManager); err != nil {
			return err
		}
		if err := validateEditionChoice(installerplan.FileManager, overrides.fileManager, plan); err != nil {
			return err
		}
	}
	if err := validateComponentFlag("hyakki", overrides.hyakki, plan); err != nil {
		return err
	}
	if overrides.cachyosLayer {
		if err := validateComponentFlag("cachyos-layer", true, plan); err != nil {
			return err
		}
	}
	if overrides.gaming {
		component, ok := plan.choices.GamingComponent()
		if !ok || component.Component.ID == "cachyos-layer" {
			return fmt.Errorf(i18n.T("--gaming is unavailable on %s"), plan.edition)
		}
		if err := validateComponentFlag(component.Component.ID, true, plan); err != nil {
			return err
		}
	}
	for _, id := range overrides.without {
		if err := validateComponentFlag(id, false, plan); err != nil {
			return err
		}
	}
	return nil
}

func validateComponentFlag(id string, selected bool, plan editionCompositorPlan) error {
	if _, ok := installerplan.ComponentByID(id); !ok {
		var available []string
		for _, component := range plan.choices.Components() {
			if component.Available && !component.Locked {
				available = append(available, component.Component.ID)
			}
		}
		return fmt.Errorf(i18n.T("unknown component %q; choose one of: %s"), id, strings.Join(available, ", "))
	}
	availability, ok := plan.choices.Component(id)
	if !ok || !availability.Available {
		reason := i18n.T("not shipped on this edition")
		if ok && availability.Reason != "" {
			reason = availability.Reason
		}
		return fmt.Errorf(i18n.T("component %q is unavailable on %s: %s"), id, plan.edition, reason)
	}
	if !selected && availability.Locked {
		return fmt.Errorf(i18n.T("component %q is required on %s: %s"), id, plan.edition, availability.Reason)
	}
	return nil
}

func applyPlanOverrides(p *plan, overrides planOverrides) {
	applyPlanChoices(p, overrides.browser, overrides.shell, overrides.fileManager)
	p.setComponent("hyakki", overrides.hyakki)
	if overrides.cachyosLayer {
		p.setComponent("cachyos-layer", true)
	}
	if overrides.gaming {
		if component, ok := p.choiceEdition().GamingComponent(); ok && component.Component.ID != "cachyos-layer" {
			p.setComponent(component.Component.ID, true)
		}
	}
	for _, id := range overrides.without {
		p.setComponent(id, false)
	}
}

func sourceEditionForHost(d *distro) string {
	if d == nil {
		return "Arch"
	}
	if d.id != "arch" {
		return d.name
	}
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return d.name
	}
	id, _, _ := parseOSRelease(string(data))
	switch strings.ToLower(id) {
	case "arch":
		return "Arch"
	case "cachyos":
		return "CachyOS"
	default:
		return id
	}
}

func loadEditionCompositors(client httpDoer, d *distro, ref, payload string) (editionCompositorPlan, error) {
	if d == nil {
		return sourceEditionChoices("Arch"), nil
	}
	if d.id != "fedora" && d.id != "void" {
		return sourceEditionChoices(sourceEditionForHost(d)), nil
	}
	if payload != "" {
		path := filepath.Join(payload, d.id, "packages", "translations.tsv")
		data, err := os.ReadFile(path)
		if err != nil {
			return editionCompositorPlan{}, fmt.Errorf(i18n.T("read %s installer choices: %w"), d.name, err)
		}
		return parseEditionCompositors(data)
	}
	url := "https://raw.githubusercontent.com/ryoku-dev/ryoku/" + ref + "/" + d.id + "/packages/translations.tsv"
	resp, err := httpStatusOK(client, http.MethodGet, url)
	if err != nil {
		return editionCompositorPlan{}, fmt.Errorf(i18n.T("read %s installer choices from %s: %w"), d.name, url, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return editionCompositorPlan{}, err
	}
	return parseEditionCompositors(data)
}
