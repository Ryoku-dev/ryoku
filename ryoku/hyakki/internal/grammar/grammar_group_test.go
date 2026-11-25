package grammar

import (
	"ryoku-rashin/internal/grammar/dict"
	"ryoku-rashin/internal/grammar/rules"
	"testing"
)

func TestRulesMetadataMatchesCuratedGroup(t *testing.T) {
	group := rules.NewCurated(dict.NewMutableDictionary(), dict.American)
	descriptions := group.AllDescriptions()
	metadata := Rules()
	if len(metadata) != len(descriptions) {
		t.Fatalf("metadata %d rules, runtime %d", len(metadata), len(descriptions))
	}
	for _, info := range metadata {
		description, ok := descriptions[info.Name]
		if !ok || description != info.Description || group.Config.IsRuleEnabled(info.Name) != info.DefaultOn {
			t.Errorf("metadata drift for %s: %+v; runtime description %q, enabled %v", info.Name, info, description, group.Config.IsRuleEnabled(info.Name))
		}
	}
}
