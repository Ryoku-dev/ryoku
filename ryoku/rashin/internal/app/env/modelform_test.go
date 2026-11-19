package env

import (
	"maps"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestModelFormStartsOnAutoAndReadOnly(t *testing.T) {
	var form ModelForm
	model, options := form.Effective()
	if model != protocol.RouteAuto || options != nil || form.Approvals() != protocol.ApprovalsReadOnly {
		t.Fatalf("zero form: model %q options %v approvals %q", model, options, form.Approvals())
	}
	if _, changed := form.Changed(); changed {
		t.Fatal("a zero form has a pick")
	}
}

func TestModelFormInheritsOnlyUnsetPartsAndKeepsPlanSeparate(t *testing.T) {
	form := ModelForm{
		Parts:   ModelOptions | ModelPlan,
		Inherit: &ModelInherit{Label: "Teams default", Model: "rashin-models/lfm3-omni-2b", Options: map[string]string{protocol.OptionEffort: "medium", "format": "short"}},
	}
	form.Set("", map[string]string{protocol.OptionEffort: "high", protocol.OptionPlan: protocol.OptionOn})
	model, stored := form.Value()
	if model != "" || !maps.Equal(stored, map[string]string{protocol.OptionEffort: "high"}) {
		t.Fatalf("stored values changed: %q %v", model, stored)
	}
	model, effective := form.Effective()
	if model != "rashin-models/lfm3-omni-2b" || !maps.Equal(effective, map[string]string{protocol.OptionEffort: "high", "format": "short"}) {
		t.Fatalf("inheritance changed: %q %v", model, effective)
	}
	form.SetPlan(true)
	if got := form.SessionOptions(); !maps.Equal(got, map[string]string{protocol.OptionEffort: "high", "format": "short", protocol.OptionPlan: protocol.OptionOn}) {
		t.Fatalf("session options = %v", got)
	}
	if _, changed := form.Changed(); changed {
		t.Fatal("seeding the form counted as a pick")
	}
	delete(stored, protocol.OptionEffort)
	delete(effective, "format")
	if _, options := form.Effective(); options[protocol.OptionEffort] != "high" || options["format"] != "short" {
		t.Fatal("a returned options map changed the form")
	}
}

func TestAgentReadinessRequiresFreeOrLocalTools(t *testing.T) {
	menu := ModelMenu{loaded: true, models: protocol.ModelsResult{Models: []protocol.ModelInfo{{Tier: protocol.TierPaid, Available: true, Tools: true}}}}
	if menu.agentReady() {
		t.Fatal("a paid-only model cannot satisfy an automatic route")
	}
	menu.models.Models = append(menu.models.Models, protocol.ModelInfo{Tier: protocol.TierLocal, Available: true, Tools: true})
	if !menu.agentReady() {
		t.Fatal("an available local tool model should run agents")
	}
}
