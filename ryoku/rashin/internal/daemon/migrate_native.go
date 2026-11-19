package daemon

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// nativeEngine marks the move from external ACP harnesses to Rashin's own
// engine (MigrateNativeEngine).
const nativeEngine = "native-engine"

// legacySkillsKept are the entries of the former skills dir that are the
// skills of today: the built-in sets and the managed skills.
var legacySkillsKept = []string{"builtin", "managed"}

// MigrateNativeEngine removes what the external harnesses left behind: the
// teams' copies of their logins (harness homes) and the guard state beside
// them, the capability cache and the probe cache, and the content-hash
// folders the built-in skills were once written to. Each team whose logins
// were deleted gets one line in the teams' ledger, so the user knows why a
// sign-in is gone. It returns those teams' ids. Running it again finds
// nothing and changes nothing; the daemon runs it once at start and `rashin
// migrate` runs it on a machine where the daemon is off. Settings and team
// state need no step: their harness keys drop when they are next saved.
func MigrateNativeEngine() ([]string, error) {
	legacy := paths.LegacyHarnessState()
	var present []string
	if len(legacy) > 0 {
		present = signedInTeams(legacy[0])
	}
	var failed []error
	for _, path := range legacy {
		if err := paths.RemoveWritableTree(path); err != nil {
			failed = append(failed, err)
		}
	}
	var teams []string
	if len(legacy) > 0 {
		remaining := signedInTeams(legacy[0])
		for _, team := range present {
			if !slices.Contains(remaining, team) {
				teams = append(teams, team)
			}
		}
	}
	if err := pruneLegacySkills(paths.LegacySkillsDir()); err != nil {
		failed = append(failed, err)
	}
	if len(teams) > 0 {
		if err := recordRemovedLogins(teams); err != nil {
			failed = append(failed, err)
		}
	}
	return teams, errors.Join(failed...)
}

// signedInTeams are the teams that had a home for any harness under homes
// (<homes>/<harness>/<team>, both names path-escaped), sorted.
func signedInTeams(homes string) []string {
	harnesses, err := os.ReadDir(homes)
	if err != nil {
		return nil
	}
	var teams []string
	for _, harness := range harnesses {
		if !harness.IsDir() {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(homes, harness.Name()))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			team, err := url.PathUnescape(entry.Name())
			if err != nil || team == "" {
				continue
			}
			teams = append(teams, team)
		}
	}
	slices.Sort(teams)
	return slices.Compact(teams)
}

// pruneLegacySkills removes every entry of the former skills dir except the
// built-in and managed skills. A dir that does not exist is nothing to do.
func pruneLegacySkills(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var failed []error
	for _, entry := range entries {
		if slices.Contains(legacySkillsKept, entry.Name()) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			failed = append(failed, err)
		}
	}
	return errors.Join(failed...)
}

// recordRemovedLogins appends one ledger line per team to the teams' ledger,
// the file Teams reads its activity from, named for the team where its state
// still holds it.
func recordRemovedLogins(teams []string) error {
	dir := paths.TeamsDir()
	if err := paths.EnsureDir(dir); err != nil {
		return err
	}
	names := teamNames(dir)
	file, err := os.OpenFile(filepath.Join(dir, "ledger.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	now := time.Now()
	var lines []byte
	for _, team := range teams {
		target := names[team]
		if target == "" {
			target = team
		}
		line, err := json.Marshal(protocol.Activity{
			Time: now, Actor: "Rashin", Action: "team.logins", Target: target, OK: true, Team: team,
			Initiator: protocol.InitiatorRashin,
			Detail:    i18n.T("Rashin now runs every task with its own engine, so the sign-ins this team kept for external coding agents were deleted."),
		})
		if err != nil {
			_ = file.Close()
			return err
		}
		lines = append(append(lines, line...), '\n')
	}
	_, writeErr := file.Write(lines)
	return errors.Join(writeErr, file.Close())
}

// teamNames maps team ids to names from the teams' state.json, empty when
// it cannot be read.
func teamNames(dir string) map[string]string {
	names := make(map[string]string)
	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		return names
	}
	var state struct {
		Teams []protocol.Team `json:"teams"`
	}
	if json.Unmarshal(data, &state) != nil {
		return names
	}
	for _, team := range state.Teams {
		names[team.ID] = team.Name
	}
	return names
}
