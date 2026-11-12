package teamsui

import (
	"slices"
	"testing"

	"ryoku-rashin/internal/protocol"
)

// light, medium and advanced in that order whatever order the daemon sends, the blank one and any level unknown here last
func TestTemplatesGroupByLevelWithTheBlankLast(t *testing.T) {
	templates := []protocol.TeamTemplate{
		{ID: "blank"},
		{ID: "news", Level: protocol.LevelAdvanced},
		{ID: "repo", Level: protocol.LevelMedium},
		{ID: "system", Level: protocol.LevelLight},
		{ID: "future", Level: "expert"},
		{ID: "research", Level: protocol.LevelMedium},
		{ID: "mail", Level: protocol.LevelLight},
	}
	var levels []string
	var ids [][]string
	for _, group := range templateGroups(templates) {
		levels = append(levels, group.level)
		var inGroup []string
		for _, template := range group.templates {
			inGroup = append(inGroup, template.ID)
		}
		ids = append(ids, inGroup)
	}
	wantLevels := []string{protocol.LevelLight, protocol.LevelMedium, protocol.LevelAdvanced, ""}
	wantIDs := [][]string{{"system", "mail"}, {"repo", "research"}, {"news"}, {"blank", "future"}}
	if !slices.Equal(levels, wantLevels) {
		t.Fatalf("levels = %v, want %v", levels, wantLevels)
	}
	for index := range wantIDs {
		if !slices.Equal(ids[index], wantIDs[index]) {
			t.Fatalf("group %d = %v, want %v", index, ids[index], wantIDs[index])
		}
	}
	if groups := templateGroups([]protocol.TeamTemplate{{ID: "system", Level: protocol.LevelLight}}); len(groups) != 1 {
		t.Fatalf("an empty level is shown: %d groups", len(groups))
	}
}
