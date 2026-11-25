package config

import (
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/protocol"
)

func TestMemoryProfileValidation(t *testing.T) {
	m := defaultMemory()
	m.Profile = protocol.MemoryProfile{Name: " Rae ", Work: "Design\x00", About: strings.Repeat("a", 700), Traits: []string{"brief", "brief", "unknown", "with examples"}}
	validateMemory(&m, defaultMemory())
	if m.Profile.Name != "Rae" || m.Profile.Work != "Design" || len(m.Profile.About) != 600 || !slices.Equal(m.Profile.Traits, []string{"brief", "with examples"}) {
		t.Fatalf("profile not normalized: %+v", m.Profile)
	}
	if err := checkMemory(m); err != nil {
		t.Fatal(err)
	}
	m.Profile.Name = strings.Repeat("n", 81)
	if err := checkMemory(m); err == nil {
		t.Fatal("accepted long profile name")
	}
	m.Profile.Name, m.Profile.Traits = "Rae", []string{"unknown"}
	if err := checkMemory(m); err == nil {
		t.Fatal("accepted unknown answer trait")
	}
}
