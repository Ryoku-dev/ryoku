package lint

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func TestValidatesBoolTrue(t *testing.T) {
	if !(Setting{Kind: Bool, Name: "A", State: true}).Validate() {
		t.Fatal("invalid true")
	}
}
func TestValidatesBoolFalse(t *testing.T) {
	if !(Setting{Kind: Bool, Name: "A", State: false}).Validate() {
		t.Fatal("invalid false")
	}
}
func TestValidatesValidOneOfManySome(t *testing.T) {
	if !(Setting{Kind: OneOfMany, Names: []string{"A", "B"}, Labels: []string{"C", "D"}, Choice: new(1)}).Validate() {
		t.Fatal("invalid selection")
	}
}
func TestValidatesValidOneOfManyNone(t *testing.T) {
	if !(Setting{Kind: OneOfMany, Names: []string{"A", "B"}, Labels: []string{"C", "D"}}).Validate() {
		t.Fatal("invalid none")
	}
}
func TestValidatesInvalidOneOfManySomeTooLarge(t *testing.T) {
	if (Setting{Kind: OneOfMany, Names: []string{"A", "B"}, Labels: []string{"C", "D"}, Choice: new(2)}).Validate() {
		t.Fatal("accepted oversized choice")
	}
}
func TestValidatesInvalidOneOfManyInconsistentLen(t *testing.T) {
	if (Setting{Kind: OneOfMany, Names: []string{"A", "B"}, Labels: []string{"C", "D", "E"}, Choice: new(1)}).Validate() {
		t.Fatal("accepted inconsistent labels")
	}
}
func TestConvertsOnlyBools(t *testing.T) {
	s := StructuredConfig{Settings: []Setting{{Kind: Bool, Name: "A"}, {Kind: Bool, Name: "B", State: true}, {Kind: Bool, Name: "C"}}}
	c, ok := s.ToFlatConfig()
	if !ok || c.IsRuleEnabled("A") || !c.IsRuleEnabled("B") || c.IsRuleEnabled("C") {
		t.Fatal(c)
	}
}
func TestConvertsOnlyOneOfMany(t *testing.T) {
	s := StructuredConfig{Settings: []Setting{{Kind: OneOfMany, Names: []string{"A", "B", "C"}, Labels: []string{"A", "B", "C"}, Choice: new(2)}}}
	c, ok := s.ToFlatConfig()
	if !ok || c.IsRuleEnabled("A") || c.IsRuleEnabled("B") || !c.IsRuleEnabled("C") {
		t.Fatal(c)
	}
}
func TestCanPullSimpleConfigFromFlatConfig(t *testing.T) {
	s := StructuredConfig{Settings: []Setting{{Kind: OneOfMany, Names: []string{"A", "B", "C"}, Labels: []string{"A", "B", "C"}}}}
	c := NewFlatConfig()
	c.SetRuleEnabled("A", false)
	c.SetRuleEnabled("B", false)
	c.SetRuleEnabled("C", true)
	s.CopyFromFlatConfig(c)
	if s.Settings[0].Choice == nil || *s.Settings[0].Choice != 2 {
		t.Fatal(s)
	}
}
func TestValidatesGroupWithLabel(t *testing.T) {
	s := Setting{Kind: Group, Label: "Group", Description: "Description", Child: &StructuredConfig{Settings: []Setting{{Kind: Bool, Name: "A", State: true}}}}
	if !s.Validate() {
		t.Fatal(s)
	}
}
func TestCuratedIsValid(t *testing.T) {
	if !CuratedStructuredConfig().Validate() {
		t.Fatal("invalid curated")
	}
}
func humanFixture() HumanReadableStructuredConfig {
	return HumanReadableStructuredConfig{Settings: []HumanReadableSetting{{Kind: Bool, Name: new("A"), State: true, Label: new("Rule A")}, {Kind: OneOfMany, Names: []string{"B", "C"}, Name: new("C"), Labels: []string{"Rule B", "Rule C"}}, {Kind: Group, Label: new("Group D"), Description: "Description D", Child: &HumanReadableStructuredConfig{Settings: []HumanReadableSetting{{Kind: Bool, Name: new("D")}}}}}}
}
func structuredFixture() StructuredConfig {
	return StructuredConfig{Settings: []Setting{{Kind: Bool, Name: "A", State: true}, {Kind: OneOfMany, Names: []string{"B", "C"}, Labels: []string{"Rule B", "Rule C"}, Choice: new(1)}, {Kind: Group, Label: "Group D", Description: "Description D", Child: &StructuredConfig{Settings: []Setting{{Kind: Bool, Name: "D"}}}}}}
}
func TestHumanReadableConfigRoundTripsJSON(t *testing.T) {
	original := humanFixture()
	b, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var decoded HumanReadableStructuredConfig
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, decoded) {
		t.Fatalf("%+v != %+v", original, decoded)
	}
}
func TestConvertsToStructuredConfig(t *testing.T) {
	actual, ok := humanFixture().ToStructuredConfig()
	if !ok || !reflect.DeepEqual(actual, structuredFixture()) {
		t.Fatalf("%+v", actual)
	}
}
func TestConvertsFromStructuredConfig(t *testing.T) {
	actual := HumanReadableStructuredConfigFromStructuredConfig(structuredFixture())
	expected := humanFixture()
	expected.Settings[0].Label = nil
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("%+v != %+v", actual, expected)
	}
}
func TestRejectsInvalidOneOfManySelection(t *testing.T) {
	s := HumanReadableStructuredConfig{Settings: []HumanReadableSetting{{Kind: OneOfMany, Names: []string{"A", "B"}, Name: new("C")}}}
	if s.Validate() {
		t.Fatal("accepted")
	}
	if _, ok := s.ToStructuredConfig(); ok {
		t.Fatal("converted")
	}
}
func TestRejectsInvalidOneOfManyLabels(t *testing.T) {
	s := HumanReadableStructuredConfig{Settings: []HumanReadableSetting{{Kind: OneOfMany, Names: []string{"A", "B"}, Name: new("B"), Labels: []string{"Only one"}}}}
	if s.Validate() {
		t.Fatal("accepted")
	}
	if _, ok := s.ToStructuredConfig(); ok {
		t.Fatal("converted")
	}
}
func TestUsesRuleNamesAsLabelsWhenNoneAreProvided(t *testing.T) {
	s := HumanReadableStructuredConfig{Settings: []HumanReadableSetting{{Kind: OneOfMany, Names: []string{"A", "B"}, Name: new("B")}}}
	actual, ok := s.ToStructuredConfig()
	expected := StructuredConfig{Settings: []Setting{{Kind: OneOfMany, Names: []string{"A", "B"}, Labels: []string{"A", "B"}, Choice: new(1)}}}
	if !ok || !reflect.DeepEqual(actual, expected) {
		t.Fatal(actual)
	}
}
func TestDefaultConfigMatchesCuratedFlatConfig(t *testing.T) {
	settings := CuratedStructuredConfig()
	actual, ok := settings.ToFlatConfig()
	if !ok || !settings.Validate() {
		t.Fatal("invalid")
	}
	curated := NewCuratedFlatConfig()
	for name := range actual {
		if actual.IsRuleEnabled(name) != curated.IsRuleEnabled(name) {
			t.Fatal(name)
		}
	}
}
func TestFlatConfigUnsetMergeAndClear(t *testing.T) {
	c := NewFlatConfig()
	c.SetRuleEnabled("A", true)
	c.SetRuleEnabledIfUnset("A", false)
	if !c.IsRuleEnabled("A") {
		t.Fatal(c)
	}
	other := NewFlatConfig()
	other["A"] = nil
	other.SetRuleEnabled("B", false)
	c.MergeFrom(other)
	if !c.IsRuleEnabled("A") || !c.HasRule("B") {
		t.Fatal(c)
	}
	clone := c.Clone()
	clone.SetRuleEnabled("A", false)
	if !c.IsRuleEnabled("A") {
		t.Fatal("clone aliases")
	}
	c.Clear()
	if c.IsRuleEnabled("A") || !c.HasRule("A") {
		t.Fatal(c)
	}
	c.UnsetRuleEnabled("A")
	if c.HasRule("A") {
		t.Fatal(c)
	}
	c.FillWithCurated()
	if !c.IsRuleEnabled("SpellCheck") {
		t.Fatal("curated missing")
	}
	names := []string{}
	for k := range c {
		names = append(names, k)
	}
	slices.Sort(names)
	encoded, err := json.Marshal(c)
	if err != nil || len(encoded) == 0 || len(names) == 0 {
		t.Fatal(err)
	}
}
