package protocol

import "testing"

func TestLocalModelsSettingsEqualIncludesOverrides(t *testing.T) {
	left := LocalModelsSettings{Device: "Vulkan0", Overrides: map[string]LocalModelOverride{"first": {Context: 16384}}}
	right := LocalModelsSettings{Device: "Vulkan0", Overrides: map[string]LocalModelOverride{"first": {Context: 16384}}}
	if !left.Equal(right) {
		t.Fatal("equivalent independently allocated overrides differ")
	}
	right.Overrides["first"] = LocalModelOverride{Context: 32768}
	if left.Equal(right) {
		t.Fatal("changing one model's override did not refresh routing")
	}
}
