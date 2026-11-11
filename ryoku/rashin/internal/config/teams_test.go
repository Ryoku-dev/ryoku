package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

func writeConfig(t *testing.T, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(paths.ConfigFile()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.ConfigFile(), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func saveAndLoad(t *testing.T, s protocol.Settings) protocol.Settings {
	t.Helper()
	if err := Save(s); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	return loaded
}

func TestTeamsSettingsAFileNeverNamedTakeTheirDefaults(t *testing.T) {
	isolateConfig(t)
	writeConfig(t, `{"version":2,"teams":{"maxRunning":4}}`)
	settings, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	teams := settings.Teams
	if teams.MaxRunning != 4 || teams.PerTeam != protocol.DefaultTeamsPerTeam || !teams.RoutinesWake || teams.ApprovalHours != protocol.DefaultApprovalHours {
		t.Fatalf("teams = %+v", teams)
	}
	want := protocol.TeamDefaults{
		Approvals: protocol.ApprovalsAsk,
		Budget:    protocol.TeamBudget{MonthTokens: 2_000_000, WarnPercent: 80, TaskTurns: 20, TaskMinutes: 60},
		Notify:    protocol.NotifyAll,
	}
	if teams.Paused || teams.KeepDays != 0 || teams.QuietHours != nil || teams.Hidden != nil {
		t.Fatalf("teams = %+v", teams)
	}
	if teams.Defaults.Approvals != want.Approvals || teams.Defaults.Budget != want.Budget || teams.Defaults.Notify != want.Notify {
		t.Fatalf("defaults = %+v, want %+v", teams.Defaults, want)
	}
}

func TestRoutinesWakeTurnedOffStaysOff(t *testing.T) {
	isolateConfig(t)
	next, err := Merge(Defaults(), json.RawMessage(`{"teams":{"routinesWake":false}}`))
	if err != nil {
		t.Fatal(err)
	}
	if next.Teams.RoutinesWake {
		t.Fatal("the patch left routinesWake on")
	}
	if loaded := saveAndLoad(t, next); loaded.Teams.RoutinesWake {
		t.Fatal("routinesWake came back on after a save and a load")
	}
	again, err := Merge(next, json.RawMessage(`{"teams":{"maxRunning":2}}`))
	if err != nil {
		t.Fatal(err)
	}
	if again.Teams.RoutinesWake {
		t.Fatal("another patch turned routinesWake back on")
	}
}

func TestNoTimeLimitForNewTeamsSurvivesASave(t *testing.T) {
	isolateConfig(t)
	next, err := Merge(Defaults(), json.RawMessage(`{"teams":{"defaults":{"budget":{"taskMinutes":0}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if loaded := saveAndLoad(t, next); loaded.Teams.Defaults.Budget.TaskMinutes != 0 {
		t.Fatalf("minutes per task after a save and a load = %d", loaded.Teams.Defaults.Budget.TaskMinutes)
	}
}

func TestKeepDaysSnapsWithoutTurningForeverIntoADefault(t *testing.T) {
	for days, want := range map[int]int{0: 0, -5: 0, 10: 30, 30: 30, 60: 90, 100: 365, 365: 365, 1000: 365} {
		settings := Defaults()
		settings.Teams.KeepDays = days
		Validate(&settings)
		if settings.Teams.KeepDays != want {
			t.Fatalf("keepDays %d became %d, want %d", days, settings.Teams.KeepDays, want)
		}
	}
}

func TestPerTeamFollowsALoweredMaxRunning(t *testing.T) {
	next, err := Merge(Defaults(), json.RawMessage(`{"teams":{"maxRunning":1}}`))
	if err != nil {
		t.Fatal(err)
	}
	if next.Teams.PerTeam != 1 {
		t.Fatalf("perTeam = %d under maxRunning 1", next.Teams.PerTeam)
	}
}

func TestQuietHoursMayCrossMidnight(t *testing.T) {
	next, err := Merge(Defaults(), json.RawMessage(`{"teams":{"quietHours":{"from":"22:00","to":"07:30"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if next.Teams.QuietHours == nil || *next.Teams.QuietHours != (protocol.QuietHours{From: "22:00", To: "07:30"}) {
		t.Fatalf("quiet hours = %+v", next.Teams.QuietHours)
	}
	settings := Defaults()
	settings.Teams.QuietHours = &protocol.QuietHours{From: "7:30", To: "22:00"}
	Validate(&settings)
	if settings.Teams.QuietHours != nil {
		t.Fatalf("unreadable quiet hours were kept: %+v", settings.Teams.QuietHours)
	}
}

func TestHiddenPathsAreKeptShortAndAnEmptyListSurvives(t *testing.T) {
	isolateConfig(t)
	patch, err := json.Marshal(map[string]any{"teams": map[string]any{
		"hidden": []string{"~/.ssh/", filepath.Join(paths.Home(), ".aws"), "/srv/secrets", "~/.ssh"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	next, err := Merge(Defaults(), patch)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"~/.ssh", "~/.aws", "/srv/secrets"}; !slices.Equal(next.Teams.Hidden, want) {
		t.Fatalf("hidden = %q, want %q", next.Teams.Hidden, want)
	}
	empty, err := Merge(next, json.RawMessage(`{"teams":{"hidden":[]}}`))
	if err != nil {
		t.Fatal(err)
	}
	if loaded := saveAndLoad(t, empty); loaded.Teams.Hidden == nil || len(loaded.Teams.Hidden) != 0 {
		t.Fatalf("an emptied list came back as %#v", loaded.Teams.Hidden)
	}
	reset, err := Merge(empty, json.RawMessage(`{"teams":{"hidden":null}}`))
	if err != nil {
		t.Fatal(err)
	}
	if reset.Teams.Hidden != nil {
		t.Fatalf("a reset list is %#v, want nil for the built-in list", reset.Teams.Hidden)
	}
}

func TestMergeRejectsBadTeamsSettings(t *testing.T) {
	isolateConfig(t)
	above, err := json.Marshal(map[string]any{"teams": map[string]any{"hidden": []string{filepath.Dir(paths.Home())}}})
	if err != nil {
		t.Fatal(err)
	}
	patches := []string{
		`{"teams":{"maxRunning":7}}`,
		`{"teams":{"perTeam":0}}`,
		`{"teams":{"perTeam":5}}`,
		`{"teams":{"approvalHours":0}}`,
		`{"teams":{"approvalHours":169}}`,
		`{"teams":{"keepDays":45}}`,
		`{"teams":{"quietHours":{"from":"25:00","to":"07:00"}}}`,
		`{"teams":{"quietHours":{"from":"7:30","to":"08:00"}}}`,
		`{"teams":{"quietHours":{"from":"22:00","to":"22:00"}}}`,
		`{"teams":{"hidden":["relative/path"]}}`,
		`{"teams":{"hidden":["/"]}}`,
		`{"teams":{"hidden":["~"]}}`,
		string(above),
		`{"teams":{"defaults":{"model":"two words"}}}`,
		`{"teams":{"defaults":{"options":{"effort":""}}}}`,
		`{"teams":{"defaults":{"options":{"a":"1","b":"1","c":"1","d":"1","e":"1","f":"1","g":"1","h":"1","i":"1"}}}}`,
		`{"teams":{"defaults":{"approvals":"maybe"}}}`,
		`{"teams":{"defaults":{"notify":"loud"}}}`,
		`{"teams":{"defaults":{"budget":{"monthTokens":-1}}}}`,
		`{"teams":{"defaults":{"budget":{"warnPercent":120}}}}`,
		`{"teams":{"defaults":{"budget":{"taskTurns":500}}}}`,
		`{"teams":{"defaults":{"budget":{"taskMinutes":2000}}}}`,
	}
	for _, patch := range patches {
		_, err := Merge(Defaults(), json.RawMessage(patch))
		var protocolErr *protocol.Error
		if !errors.As(err, &protocolErr) || protocolErr.Code != protocol.CodeBadRequest {
			t.Fatalf("patch %s error = %#v", patch, err)
		}
	}
	hidden := make([]string, protocol.TeamsHiddenMax+1)
	for i := range hidden {
		hidden[i] = filepath.Join("/srv", string(rune('a'+i%26)), string(rune('a'+i/26)))
	}
	tooMany, err := json.Marshal(map[string]any{"teams": map[string]any{"hidden": hidden}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Merge(Defaults(), tooMany); err == nil {
		t.Fatalf("%d hidden paths were accepted", len(hidden))
	}
}
