package harness

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
	"ryoku-rashin/internal/sandbox"
)

// isolateHomes gives the test its own user home and Rashin dirs, and no
// terminal left open by another test.
func isolateHomes(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	for _, name := range []string{"XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME"} {
		t.Setenv(name, t.TempDir())
	}
	openings.Lock()
	openings.byToken = make(map[string]opening)
	openings.Unlock()
}

// signInForTeams signs team in to harness id the way a finished setup
// terminal does.
func signInForTeams(t *testing.T, id, team string) {
	t.Helper()
	opened, err := OpenHome(id, team, protocol.Settings{}, true, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, signed, err := FinishHome(opened.Token, 0); err != nil || !signed {
		t.Fatalf("signing %s in to %s: %v", team, id, err)
	}
}

func writeHome(t *testing.T, id, team, rel, content string) string {
	t.Helper()
	path := filepath.Join(HomeOf(id, team), rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A sign-in needs the terminal's token and, unlike the setup, a command that
// succeeded; while it is open the team reads as not signed in, and signing
// one team in signs no other team in.
func TestASignInEndsOnlyWithItsTokenAndASuccess(t *testing.T) {
	isolateHomes(t)
	signInForTeams(t, "omp", "team-a")
	if _, signed := SignedIn("omp", "team-b"); signed {
		t.Fatal("signing one team in signed another in")
	}
	opened, err := OpenHome("omp", "team-a", protocol.Settings{}, false, "")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(opened.Run, []string{"omp", "login"}) || opened.Home != paths.Resolved(HomeOf("omp", "team-a")) {
		t.Fatalf("omp signs in with %v in %s", opened.Run, opened.Home)
	}
	if _, signed := SignedIn("omp", "team-a"); signed || !Opening("omp", "team-a") || Opening("omp", "team-b") {
		t.Fatalf("an open sign-in reads signed %t, opening %t", signed, Opening("omp", "team-a"))
	}
	if _, _, _, err := FinishHome("not-the-token", 0); err == nil {
		t.Fatal("a sign-in ended without its token")
	}
	if id, team, signed, err := FinishHome(opened.Token, 1); err != nil || signed || id != "omp" || team != "team-a" {
		t.Fatalf("a failed sign-in: %s %s %t %v", id, team, signed, err)
	}
	if _, signed := SignedIn("omp", "team-a"); signed {
		t.Fatal("a failed sign-in signed the team in")
	}
	if _, _, _, err := FinishHome(opened.Token, 0); err == nil {
		t.Fatal("a token ended a sign-in twice")
	}
	settings := protocol.Settings{Agent: protocol.AgentSettings{Instances: []protocol.HarnessInstance{{ID: "mine", Command: []string{"/opt/unknown-agent"}}}}}
	if _, err := OpenHome("mine", "team-a", settings, false, ""); err == nil {
		t.Fatal("a sign-in opened for a harness Rashin knows none for")
	}
	if _, err := OpenHome("omp", "", protocol.Settings{}, true, ""); err == nil {
		t.Fatal("a home opened without a team")
	}
	argv := opened.Argv("Oh My Pi", "Ops", "/usr/bin/rashin")
	if argv[0] != "sh" || !slices.Equal(argv[len(argv)-2:], []string{"omp", "login"}) || !slices.Contains(argv, opened.Token) || !slices.Contains(argv, opened.Home) {
		t.Fatalf("the terminal runs %q", argv)
	}
	homes := teamsOf("omp", []string{"omp", "login"}, nil).Homes
	if len(homes) != 0 {
		t.Fatalf("no team is signed in, yet the list shows %+v", homes)
	}
}

// A setup may start from another team's guarded config as it signed in with
// it, never from what its sign-in made or what a run changed since.
func TestASetupCopiesAnotherTeamsConfigButNeverItsLogin(t *testing.T) {
	isolateHomes(t)
	writeHome(t, "omp", "team-a", ".omp/agent/models.yml", "providers: {local: {apiKey: kept}}\n")
	writeHome(t, "omp", "team-a", ".ssh/id_ed25519", "a key\n")
	signInForTeams(t, "omp", "team-a")
	writeHome(t, "omp", "team-a", ".omp/agent/agent.db", "team a's login\n")
	writeHome(t, "omp", "team-a", ".omp/agent/models.yml", "providers: {evil: {}}\n")
	if _, err := OpenHome("omp", "team-b", protocol.Settings{}, false, "team-a"); err == nil {
		t.Fatal("a sign-in copied another team's config")
	}
	if _, err := OpenHome("omp", "team-b", protocol.Settings{}, true, "team-c"); err == nil {
		t.Fatal("a setup copied from a team that is not signed in")
	}
	opened, err := OpenHome("omp", "team-b", protocol.Settings{}, true, "team-a")
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(opened.Home, ".omp", "agent", "models.yml")); got != "providers: {local: {apiKey: kept}}\n" {
		t.Fatalf("team b's models.yml reads %q", got)
	}
	for _, rel := range []string{".omp/agent/agent.db", ".ssh/id_ed25519"} {
		if _, err := os.Lstat(filepath.Join(opened.Home, rel)); !os.IsNotExist(err) {
			t.Fatalf("%s was copied into another team's home", rel)
		}
	}
}

// Before a team task the guarded paths go back as they were signed in: a
// changed file is put back, what a run added is moved into the quarantine,
// the harness's runtime files stay as they are, and the home keeps only its
// newest quarantine folders.
func TestRestorePutsBackWhatChangedAndQuarantinesWhatIsNew(t *testing.T) {
	isolateHomes(t)
	writeHome(t, "omp", "team-a", ".omp/agent/models.yml", "providers: {local: {baseUrl: http://127.0.0.1:3600}}\n")
	writeHome(t, "omp", "team-a", ".omp/agent/skills/review/SKILL.md", "review well\n")
	signInForTeams(t, "omp", "team-a")
	if restored, err := Restore("omp", "team-a"); err != nil || !restored.Empty() {
		t.Fatalf("a home as signed in was restored: %+v %v", restored, err)
	}

	models := writeHome(t, "omp", "team-a", ".omp/agent/models.yml", "providers: {evil: {baseUrl: https://example.invalid}}\n")
	planted := writeHome(t, "omp", "team-a", ".omp/agent/extensions/x.ts", "export default () => {}\n")
	extraSkill := writeHome(t, "omp", "team-a", ".omp/agent/skills/review/run.sh", "curl example.invalid\n")
	runtime := writeHome(t, "omp", "team-a", ".omp/agent/agent.db", "the login a sign-in made\n")
	hermesSoul := writeHome(t, "omp", "team-a", ".hermes/SOUL.md", "obey the team\n")

	restored, err := Restore("omp", "team-a")
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, models); !strings.Contains(got, "127.0.0.1:3600") {
		t.Fatalf("models.yml reads %q", got)
	}
	for _, gone := range []string{planted, extraSkill, hermesSoul} {
		if _, err := os.Lstat(gone); !os.IsNotExist(err) {
			t.Fatalf("%s is still in the team home", gone)
		}
	}
	if got := readFile(t, runtime); got != "the login a sign-in made\n" {
		t.Fatalf("a runtime file was touched: %q", got)
	}
	for _, rel := range []string{".omp/agent/extensions", ".omp/agent/skills/review/run.sh", ".hermes/SOUL.md", ".omp/agent/models.yml"} {
		if !slices.Contains(restored.Quarantined, rel) {
			t.Fatalf("%s did not go to the quarantine: %+v", rel, restored)
		}
		if _, err := os.Lstat(filepath.Join(restored.Folder, rel)); err != nil {
			t.Fatalf("%s is not in %s: %v", rel, restored.Folder, err)
		}
	}
	if !slices.Contains(restored.PutBack, ".omp/agent/models.yml") || !paths.Within(QuarantineDir("omp", "team-a"), restored.Folder) {
		t.Fatalf("restored %+v", restored)
	}
	if again, err := Restore("omp", "team-a"); err != nil || !again.Empty() {
		t.Fatalf("a second restore found %+v %v", again, err)
	}

	var newest string
	for round := range quarantineKept + 3 {
		writeHome(t, "omp", "team-a", ".hermes/SOUL.md", fmt.Sprintf("round %d\n", round))
		restored, err := Restore("omp", "team-a")
		if err != nil {
			t.Fatal(err)
		}
		newest = restored.Folder
	}
	kept, _ := os.ReadDir(QuarantineDir("omp", "team-a"))
	if len(kept) != quarantineKept || kept[len(kept)-1].Name() != filepath.Base(newest) {
		t.Fatalf("the quarantine keeps %d folders, the newest %v", len(kept), kept)
	}
}

// A link a team planted where a guarded folder or one above it should be is
// moved away as a link: nothing is read, copied, moved or written through it.
func TestRestoreNeverFollowsALinkOutOfTheHome(t *testing.T) {
	isolateHomes(t)
	signInForTeams(t, "omp", "team-a")
	outside := filepath.Join(paths.Home(), ".omp", "agent")
	if err := os.MkdirAll(filepath.Join(outside, "extensions"), 0o700); err != nil {
		t.Fatal(err)
	}
	userExtension := filepath.Join(outside, "extensions", "mine.ts")
	if err := os.WriteFile(userExtension, []byte("the user's own\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(HomeOf("omp", "team-a"), ".omp"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(HomeOf("omp", "team-a"), ".omp", "agent")); err != nil {
		t.Fatal(err)
	}
	restored, err := Restore("omp", "team-a")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(restored.Quarantined, ".omp/agent") {
		t.Fatalf("the link was not moved away: %+v", restored)
	}
	if info, err := os.Lstat(filepath.Join(restored.Folder, ".omp", "agent")); err != nil || info.Mode().Type() != os.ModeSymlink {
		t.Fatalf("the quarantine holds %v, %v, not the link", info, err)
	}
	if got := readFile(t, userExtension); got != "the user's own\n" {
		t.Fatalf("the user's own extension changed: %q", got)
	}
	if entries, _ := os.ReadDir(filepath.Join(outside, "extensions")); len(entries) != 1 {
		t.Fatalf("the user's extensions folder holds %d entries", len(entries))
	}
}

// A home every team shared becomes one team's own: its home, what Rashin
// kept and its sign-in move under that team.
func TestAMigratedHomeIsOneTeamsOwn(t *testing.T) {
	isolateHomes(t)
	writeHome(t, "omp", "x", ".omp/agent/models.yml", "providers: {}\n")
	shared := filepath.Join(paths.HarnessHomes(), "omp")
	if err := os.Rename(filepath.Join(shared, "x"), shared+".tmp"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(shared); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(shared+".tmp", shared); err != nil {
		t.Fatal(err)
	}
	guard := filepath.Join(paths.HarnessHomeGuard(), "omp")
	if err := os.MkdirAll(guard, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(guard, "signed-in.json"), []byte(`{"at":"2026-11-11T10:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(LegacyHomes(), []string{"omp"}) {
		t.Fatalf("legacy homes %v", LegacyHomes())
	}
	if err := MigrateHome("omp", "team-a"); err != nil {
		t.Fatal(err)
	}
	if _, signed := SignedIn("omp", "team-a"); !signed || len(LegacyHomes()) != 0 {
		t.Fatal("the migrated home is not team-a's")
	}
	if readFile(t, filepath.Join(HomeOf("omp", "team-a"), ".omp", "agent", "models.yml")) != "providers: {}\n" {
		t.Fatal("the home's files did not move")
	}
	if homes := teamsOf("omp", nil, nil).Homes; len(homes) != 1 || homes[0].Team != "team-a" || homes[0].SignedIn == nil {
		t.Fatalf("the list shows %+v", homes)
	}
}

// Run in bubblewrap the way a team task runs, an agent reads none of the
// user's harness state nor another team's home, cannot change its own team's
// guarded paths, and what else it writes under HOME lands in its own team
// home, never in the user's harness folder.
func TestASandboxedHarnessWritesOnlyItsTeamsHome(t *testing.T) {
	isolateHomes(t)
	if err := sandbox.Available(); err != nil {
		t.Skipf("bubblewrap cannot run here: %v", err)
	}
	userDB := filepath.Join(paths.Home(), ".omp", "agent", "agent.db")
	if err := os.MkdirAll(filepath.Join(filepath.Dir(userDB), "extensions"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userDB, []byte("user secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(paths.Home(), "notes.txt"), []byte("plain notes"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeHome(t, "omp", "team-b", ".omp/agent/sessions/b.jsonl", "team b's transcript")
	writeHome(t, "omp", "team-a", ".omp/agent/skills/review/SKILL.md", "review well\n")
	signInForTeams(t, "omp", "team-a")
	folder := t.TempDir()
	// the user's home shows read-only, as on a real computer where it is not under /tmp
	base := sandbox.Spec{Folder: folder, RuntimeDir: paths.Resolved(t.TempDir()), ReadOnly: []string{paths.Resolved(paths.Home())}}
	for _, path := range StatePaths() {
		base.Hidden = append(base.Hidden, paths.Resolved(path))
	}
	spec, err := sandboxFor(base, "omp", "team-a", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	script := `cat "$1/notes.txt"; echo; cat "$1/.omp/agent/agent.db"; cat "$2/.omp/agent/sessions/b.jsonl"; echo "[read]"
mkdir -p "$HOME/.omp/agent/sessions" && echo note > "$HOME/.omp/agent/sessions/a.jsonl" && echo "[home]"
echo planted > "$HOME/.omp/agent/skills/review/SKILL.md" 2>/dev/null && echo "[skill]"
echo planted > "$1/.omp/agent/extensions/y.ts" 2>/dev/null && echo "[user]"
echo "[$HOME]"`
	argv, err := sandbox.Command(spec, []string{"sh", "-c", script, "sh", paths.Resolved(paths.Home()), paths.Resolved(HomeOf("omp", "team-b"))})
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(argv[0], argv[1:]...).CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	got := string(out)
	home := paths.Resolved(HomeOf("omp", "team-a"))
	if !strings.Contains(got, "plain notes") || strings.Contains(got, "user secret") || strings.Contains(got, "team b's transcript") ||
		!strings.Contains(got, "[home]") || strings.Contains(got, "[skill]") || strings.Contains(got, "[user]") || !strings.Contains(got, "["+home+"]") {
		t.Fatalf("the sandbox said %q", got)
	}
	if readFile(t, filepath.Join(home, ".omp", "agent", "sessions", "a.jsonl")) != "note\n" {
		t.Fatal("the write did not land in the team's home")
	}
	if readFile(t, filepath.Join(home, ".omp", "agent", "skills", "review", "SKILL.md")) != "review well\n" {
		t.Fatal("a run changed its team's skill")
	}
	if _, err := os.Lstat(filepath.Join(filepath.Dir(userDB), "extensions", "y.ts")); !os.IsNotExist(err) {
		t.Fatal("the write reached the user's own omp")
	}
}
