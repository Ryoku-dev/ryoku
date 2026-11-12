package harness

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"ryoku-i18n"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// teamhome.go keeps the team homes: one HOME per team and harness instance,
// which that team's sessions and Checks run the harness with, signed in apart
// from the user's own setup and from every other team. The user's own setup
// and every other home stay hidden from a team sandbox. When the user
// finishes a sign-in or the setup in a home, Rashin keeps a copy of the paths
// that tell a harness what to do or run (guardedPaths). While a team task
// runs they are read-only, and before every task they go back as they were
// signed in, with what changed moved into a quarantine, so a run cannot leave
// instructions or code its team's next run, or the next sign-in, would
// follow. Runtime files (logins, sessions, caches, logs) are never touched.
// Nothing a sign-in made is ever copied between homes: a login copied to a
// second home would spend a rotating refresh token twice.
//
// A home is written through os.Root, so a link a team planted cannot lead a
// copy, a removal or a move out of it.

// commonGuarded are guarded in every team home whatever the harness: what git,
// shells, ssh and package managers read from HOME and would run or trust, and
// the shared skills folder several harnesses read.
var commonGuarded = []string{
	".gitconfig", ".config/git", ".agents", ".bashrc", ".bash_profile", ".bash_login", ".profile",
	".zshrc", ".zshenv", ".zprofile", ".zlogin", ".config/fish", ".ssh", ".npmrc", ".yarnrc",
	".config/pip", ".pip", ".cargo/config.toml", ".cargo/config",
}

// notCopied are guarded paths a setup never copies from another team's home:
// they hold keys of the user's, not the harness's config.
var notCopied = []string{".ssh"}

// quarantineKept is how many quarantine folders a home keeps, the newest.
const quarantineKept = 10

// guardedPaths are the paths guarded in every team home, relative to it,
// sorted: the common ones and every harness's own. Harnesses read each
// other's instructions and skills, so a home guards them all.
func guardedPaths() []string {
	out := slices.Clone(commonGuarded)
	for _, def := range definitions {
		out = append(out, def.guarded...)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// homeDirs are created in every team home, so the XDG base dirs its harness
// is pointed at exist.
var homeDirs = []string{".config", ".local/share", ".local/state", ".cache"}

// HomeOf is team's home for harness instance id.
func HomeOf(id, team string) string {
	return filepath.Join(paths.HarnessHomes(), instanceName(id), instanceName(team))
}

// guardDir holds what Rashin keeps about a team home: the signed-in record,
// the blessed copy and the quarantine.
func guardDir(id, team string) string {
	return filepath.Join(paths.HarnessHomeGuard(), instanceName(id), instanceName(team))
}

func signInFile(id, team string) string { return filepath.Join(guardDir(id, team), "signed-in.json") }

func blessedDir(id, team string) string { return filepath.Join(guardDir(id, team), "blessed") }

// QuarantineDir is where what a run changed in a team home goes, one folder
// per restore.
func QuarantineDir(id, team string) string { return filepath.Join(guardDir(id, team), "quarantine") }

// homeLocks serialise everything that touches one team home's guard.
var homeLocks sync.Map

func lockHome(id, team string) func() {
	value, _ := homeLocks.LoadOrStore(id+"\x00"+team, &sync.Mutex{})
	mu := value.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// homeEnv points a harness at its team home: HOME and the XDG base dirs.
func homeEnv(home string) map[string]string {
	return map[string]string{
		"HOME":            home,
		"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"XDG_DATA_HOME":   filepath.Join(home, ".local", "share"),
		"XDG_STATE_HOME":  filepath.Join(home, ".local", "state"),
		"XDG_CACHE_HOME":  filepath.Join(home, ".cache"),
	}
}

// prepareHome creates a team home (0700) with its XDG dirs and a git config
// that includes the user's, so commits made there carry the user's name;
// git skips an include that is missing. It returns the home resolved.
func prepareHome(id, team string) (string, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(team) == "" {
		return "", errors.New(i18n.T("A team home needs a harness and a team."))
	}
	home := HomeOf(id, team)
	if err := os.MkdirAll(home, 0o700); err != nil {
		return "", err
	}
	for _, dir := range []string{home, filepath.Dir(home), paths.HarnessHomes()} {
		if err := os.Chmod(dir, 0o700); err != nil {
			return "", err
		}
	}
	root, err := os.OpenRoot(home)
	if err != nil {
		return "", err
	}
	defer root.Close()
	for _, dir := range homeDirs {
		if err := root.MkdirAll(dir, 0o700); err != nil {
			return "", err
		}
	}
	if _, err := root.Lstat(".gitconfig"); errors.Is(err, fs.ErrNotExist) {
		include := fmt.Sprintf("# Rashin: your git identity, read from your own config.\n[include]\n\tpath = %s\n\tpath = %s\n",
			filepath.Join(paths.Home(), ".gitconfig"), filepath.Join(paths.ConfigHome(), "git", "config"))
		if err := root.WriteFile(".gitconfig", []byte(include), 0o600); err != nil {
			return "", err
		}
	}
	return paths.Resolved(home), nil
}

// signInRecord is when the user last finished a sign-in or the setup.
type signInRecord struct {
	At    time.Time `json:"at"`
	Setup bool      `json:"setup,omitempty"`
}

// SignedIn reports when team last signed in to harness id; false until the
// user finishes a sign-in or the setup in its home, and while one is open.
func SignedIn(id, team string) (time.Time, bool) {
	data, err := os.ReadFile(signInFile(id, team))
	if err != nil {
		return time.Time{}, false
	}
	var record signInRecord
	if json.Unmarshal(data, &record) != nil || record.At.IsZero() {
		return time.Time{}, false
	}
	return record.At, true
}

// teamsOf is how teams run a harness as harnesses.list shows it: every team
// home that exists or has a terminal open, by team, with what the harness
// last reported there (capabilities, nil for a caller that keeps none).
func teamsOf(id string, login []string, capabilities func(team string) *protocol.HarnessCapabilities) *protocol.HarnessTeams {
	teams := &protocol.HarnessTeams{Login: login, Guarded: guardedPaths()}
	seen := make(map[string]bool)
	add := func(team string) {
		if team == "" || seen[team] {
			return
		}
		seen[team] = true
		home := protocol.TeamHome{Team: team, Home: HomeOf(id, team), Opening: Opening(id, team)}
		if capabilities != nil {
			home.Capabilities = capabilities(team)
		}
		if at, ok := SignedIn(id, team); ok {
			home.SignedIn = &at
		}
		teams.Homes = append(teams.Homes, home)
	}
	// a home whose sign-in never finished reads as no home at all
	entries, _ := os.ReadDir(filepath.Join(paths.HarnessHomeGuard(), instanceName(id)))
	for _, entry := range entries {
		team, err := url.PathUnescape(entry.Name())
		if err != nil || !entry.IsDir() {
			continue
		}
		if _, signed := SignedIn(id, team); signed {
			add(team)
		}
	}
	openings.Lock()
	var open []string
	for _, o := range openings.byToken {
		if o.id == id {
			open = append(open, o.team)
		}
	}
	openings.Unlock()
	for _, team := range open {
		add(team)
	}
	slices.SortFunc(teams.Homes, func(a, b protocol.TeamHome) int { return strings.Compare(a.Team, b.Team) })
	return teams
}

// opening is a sign-in or setup terminal Rashin opened and waits to hear the
// end of.
type opening struct {
	id, team string
	setup    bool
}

var openings = struct {
	sync.Mutex
	byToken map[string]opening
}{byToken: make(map[string]opening)}

// Opening reports whether a sign-in or setup terminal is open for team's home
// of harness id.
func Opening(id, team string) bool {
	openings.Lock()
	defer openings.Unlock()
	for _, open := range openings.byToken {
		if open.id == id && open.team == team {
			return true
		}
	}
	return false
}

// Opened is a sign-in or setup readied in a team home: the token its terminal
// hands back when it ends, and what runs there.
type Opened struct {
	Token string
	Home  string
	Run   []string
	Setup bool
}

// OpenHome readies team's home for harness id for a sign-in, or with setup a
// shell: the home exists, it reads as not signed in until the terminal ends,
// and the token that ends it is the only one open for it. A setup with
// copyFrom first copies the guarded config another team signed in with on
// the same harness, never anything its sign-in made. The caller restores the
// home first, so the terminal starts from what was signed in.
func OpenHome(id, team string, settings protocol.Settings, setup bool, copyFrom string) (Opened, error) {
	def, _ := definitionOf(id, instanceCommand(id, settings))
	run := []string{"/bin/sh", "-c", `exec "${SHELL:-/bin/sh}"`}
	if !setup {
		if len(def.login) == 0 {
			return Opened{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("Rashin knows no sign-in for %s. Open its setup instead.", Name(id, settings)))
		}
		if copyFrom != "" {
			return Opened{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("Only a setup copies another team's config."))
		}
		run = slices.Clone(def.login)
	}
	if copyFrom != "" {
		if copyFrom == team {
			return Opened{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.T("A team cannot copy its own config."))
		}
		if _, ok := SignedIn(id, copyFrom); !ok {
			return Opened{}, protocol.Errorf(protocol.CodeBadRequest, "%s", i18n.Tf("The team to copy from is not signed in to %s.", Name(id, settings)))
		}
	}
	unlock := lockHome(id, team)
	defer unlock()
	home, err := prepareHome(id, team)
	if err != nil {
		return Opened{}, err
	}
	if err := os.Remove(signInFile(id, team)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Opened{}, err
	}
	if copyFrom != "" {
		if err := copyConfig(id, copyFrom, team); err != nil {
			return Opened{}, err
		}
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return Opened{}, err
	}
	token := hex.EncodeToString(raw)
	openings.Lock()
	for other, open := range openings.byToken {
		if open.id == id && open.team == team {
			delete(openings.byToken, other)
		}
	}
	openings.byToken[token] = opening{id: id, team: team, setup: setup}
	openings.Unlock()
	return Opened{Token: token, Home: home, Run: run, Setup: setup}, nil
}

// copyConfig copies the guarded config from's sign-in kept, which no run can
// have changed, into to's home, over what is there. Nothing else of from's
// home is read: its logins stay its own.
func copyConfig(id, from, to string) error {
	root, err := os.OpenRoot(HomeOf(id, to))
	if err != nil {
		return err
	}
	defer root.Close()
	kept := blessedDir(id, from)
	for _, rel := range guardedPaths() {
		if slices.Contains(notCopied, rel) {
			continue
		}
		if _, err := os.Lstat(filepath.Join(kept, rel)); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if dir := filepath.Dir(rel); dir != "." {
			if err := root.MkdirAll(dir, 0o700); err != nil {
				return err
			}
		}
		if err := root.RemoveAll(rel); err != nil {
			return err
		}
		if err := copyIn(root, filepath.Join(kept, rel), rel); err != nil {
			return err
		}
	}
	return nil
}

// teamsTerminal is what a sign-in or setup terminal runs: what Opened says
// with HOME and the XDG base dirs at the team home, then the rashin command
// telling the daemon it ended and how, then a pause so its answer can be read.
const teamsTerminal = `home=$1 rashin=$2 token=$3 intro=$4 close=$5
shift 5
cd "$home" || exit 1
printf '%s\n\n' "$intro"
HOME=$home XDG_CONFIG_HOME=$home/.config XDG_DATA_HOME=$home/.local/share XDG_STATE_HOME=$home/.local/state XDG_CACHE_HOME=$home/.cache "$@"
status=$?
printf '\n'
"$rashin" teams home-done "$token" "$status"
printf '\n%s' "$close"
read -r _`

// Argv is the command a terminal runs for o: name is the harness as the user
// reads it, team the team's name, rashin the rashin command that tells the
// daemon it ended.
func (o Opened) Argv(name, team, rashin string) []string {
	intro := i18n.Tf("Signing %s in for the team %s. This terminal's home is the team's own, %s; your own setup and other teams' are not touched.", name, team, o.Home)
	if o.Setup {
		intro = i18n.Tf("Setting %s up for the team %s. This shell's home is the team's own, %s. Change what the team should use, then type exit; Rashin keeps it as you leave it.", name, team, o.Home)
	}
	argv := []string{"sh", "-c", teamsTerminal, "sh", o.Home, rashin, o.Token, intro, i18n.T("Press Enter to close.")}
	return append(argv, o.Run...)
}

// FinishHome ends the sign-in or setup token was handed to. A setup, and a
// sign-in whose command succeeded, signs the team in to the harness: Rashin
// keeps a copy of the home's guarded paths as they are now. It returns the
// harness id and team and whether they are signed in.
func FinishHome(token string, status int) (string, string, bool, error) {
	openings.Lock()
	open, ok := openings.byToken[token]
	delete(openings.byToken, token)
	openings.Unlock()
	if !ok || token == "" {
		return "", "", false, protocol.Errorf(protocol.CodeNotFound, "%s", i18n.T("This sign-in is no longer open. Open it again from Rashin."))
	}
	if !open.setup && status != 0 {
		return open.id, open.team, false, nil
	}
	unlock := lockHome(open.id, open.team)
	defer unlock()
	if err := bless(open.id, open.team, open.setup); err != nil {
		return open.id, open.team, false, err
	}
	return open.id, open.team, true, nil
}

// bless keeps a copy of a home's guarded paths as they are now and records
// the sign-in.
func bless(id, team string, setup bool) error {
	root, err := os.OpenRoot(HomeOf(id, team))
	if err != nil {
		return err
	}
	defer root.Close()
	dir := guardDir(id, team)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	fresh, err := os.MkdirTemp(dir, "blessing-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(fresh)
	for _, rel := range guardedPaths() {
		if _, err := root.Lstat(rel); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return fmt.Errorf("%s", i18n.Tf("Rashin could not keep %s from the team home: %v", rel, err))
		}
		if err := copyOut(root, rel, filepath.Join(fresh, rel)); err != nil {
			return err
		}
	}
	blessed := blessedDir(id, team)
	if err := os.RemoveAll(blessed); err != nil {
		return err
	}
	if err := os.Rename(fresh, blessed); err != nil {
		return err
	}
	data, err := json.Marshal(signInRecord{At: time.Now().UTC(), Setup: setup})
	if err != nil {
		return err
	}
	return writeFileAtomic(signInFile(id, team), data)
}

// ReadOnlyGuarded are the guarded folders and files that exist in a team home,
// resolved, for a team task to see read-only: a run cannot change its own
// team's instructions, skills or config while it works. Links are left out,
// as binding one would bind where it leads.
func ReadOnlyGuarded(id, team string) []string {
	home := paths.Resolved(HomeOf(id, team))
	var out []string
	for _, rel := range guardedPaths() {
		path := filepath.Join(home, rel)
		info, err := os.Lstat(path)
		if err != nil || (!info.IsDir() && !info.Mode().IsRegular()) {
			continue
		}
		if paths.Resolved(path) != path {
			continue
		}
		out = append(out, path)
	}
	return out
}

// Restoration is what a restore put back as it was signed in and what it
// moved into Folder, both relative to the team home.
type Restoration struct {
	PutBack     []string
	Quarantined []string
	Folder      string
}

// Empty reports a restore that found the home as it was signed in.
func (r Restoration) Empty() bool { return len(r.PutBack) == 0 && len(r.Quarantined) == 0 }

// Restore puts a team home's guarded paths back as they were signed in: a
// changed one goes back to the kept copy, and one that was not there, or a
// link where a folder of them should be, moves into a new quarantine folder;
// the home keeps its newest quarantineKept of those. A home that is not
// signed in has nothing kept and is left alone.
func Restore(id, team string) (Restoration, error) {
	unlock := lockHome(id, team)
	defer unlock()
	if _, ok := SignedIn(id, team); !ok {
		return Restoration{}, nil
	}
	if _, err := prepareHome(id, team); err != nil {
		return Restoration{}, err
	}
	root, err := os.OpenRoot(HomeOf(id, team))
	if err != nil {
		return Restoration{}, err
	}
	defer root.Close()
	r := &restorer{
		root:       root,
		blessed:    blessedDir(id, team),
		quarantine: filepath.Join(QuarantineDir(id, team), time.Now().UTC().Format("20060102-150405.000000000")),
	}
	for _, rel := range guardedPaths() {
		if err := r.parents(rel); err != nil {
			return r.result(), err
		}
		if err := r.sync(rel); err != nil {
			return r.result(), err
		}
	}
	return r.result(), pruneQuarantine(QuarantineDir(id, team))
}

// pruneQuarantine removes all but the newest quarantineKept folders of dir;
// their names are times, so they sort oldest first.
func pruneQuarantine(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for len(entries) > quarantineKept {
		if err := os.RemoveAll(filepath.Join(dir, entries[0].Name())); err != nil {
			return err
		}
		entries = entries[1:]
	}
	return nil
}

// LegacyHomes are the harness instances whose team home still has the shape
// of one home shared by every team: a signed-in record right in the
// instance's guard folder.
func LegacyHomes() []string {
	entries, _ := os.ReadDir(paths.HarnessHomeGuard())
	var out []string
	for _, entry := range entries {
		if _, err := os.Stat(filepath.Join(paths.HarnessHomeGuard(), entry.Name(), "signed-in.json")); err != nil {
			continue
		}
		if id, err := url.PathUnescape(entry.Name()); err == nil {
			out = append(out, id)
		}
	}
	return out
}

// MigrateHome makes a shared home of harness id, and what Rashin kept about
// it, team's own. Each step moves one folder into a new one beside it, so a
// migration cut short is finished by the next.
func MigrateHome(id, team string) error {
	unlock := lockHome(id, team)
	defer unlock()
	for _, base := range []string{paths.HarnessHomes(), paths.HarnessHomeGuard()} {
		old := filepath.Join(base, instanceName(id))
		moving := old + ".moving"
		if _, err := os.Stat(moving); errors.Is(err, fs.ErrNotExist) {
			if _, err := os.Stat(old); errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err := os.Rename(old, moving); err != nil {
				return err
			}
		}
		if err := os.MkdirAll(old, 0o700); err != nil {
			return err
		}
		if err := os.Rename(moving, filepath.Join(old, instanceName(team))); err != nil {
			return err
		}
	}
	return nil
}

type restorer struct {
	root        *os.Root
	blessed     string
	quarantine  string
	putBack     []string
	quarantined []string
}

func (r *restorer) result() Restoration {
	out := Restoration{PutBack: r.putBack, Quarantined: r.quarantined}
	if len(r.quarantined) > 0 {
		out.Folder = r.quarantine
	}
	return out
}

// parents moves away whatever stands where a folder above rel should be: a
// link, which could lead the harness or a later restore elsewhere, or a file.
func (r *restorer) parents(rel string) error {
	dir := filepath.Dir(rel)
	if dir == "." {
		return nil
	}
	parts := strings.Split(dir, string(filepath.Separator))
	for i := range parts {
		prefix := filepath.Join(parts[:i+1]...)
		info, err := r.root.Lstat(prefix)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.IsDir() {
			continue
		}
		return r.move(prefix)
	}
	return nil
}

// sync makes rel in the home what the kept copy holds.
func (r *restorer) sync(rel string) error {
	home, err := r.root.Lstat(rel)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	kept, keptErr := os.Lstat(filepath.Join(r.blessed, rel))
	if keptErr != nil && !errors.Is(keptErr, fs.ErrNotExist) {
		return keptErr
	}
	switch {
	case home == nil && kept == nil:
		return nil
	case kept == nil:
		return r.move(rel)
	case home == nil:
		return r.put(rel)
	case home.Mode().Type() != kept.Mode().Type() || !kept.Mode().IsDir() && !kept.Mode().IsRegular() && kept.Mode().Type() != fs.ModeSymlink:
		if err := r.move(rel); err != nil {
			return err
		}
		return r.put(rel)
	case kept.IsDir():
		return r.syncDir(rel, home, kept)
	case kept.Mode().Type() == fs.ModeSymlink:
		target, err := r.root.Readlink(rel)
		if err != nil {
			return err
		}
		want, err := os.Readlink(filepath.Join(r.blessed, rel))
		if err != nil {
			return err
		}
		if target == want {
			return nil
		}
	default:
		same, err := r.sameFile(rel, home, kept)
		if err != nil || same {
			return err
		}
	}
	if err := r.move(rel); err != nil {
		return err
	}
	return r.put(rel)
}

func (r *restorer) syncDir(rel string, home, kept fs.FileInfo) error {
	dir, err := r.root.Open(rel)
	if err != nil {
		return err
	}
	entries, err := dir.ReadDir(-1)
	dir.Close()
	if err != nil {
		return err
	}
	keptEntries, err := os.ReadDir(filepath.Join(r.blessed, rel))
	if err != nil {
		return err
	}
	names := make(map[string]bool, len(keptEntries))
	for _, entry := range keptEntries {
		names[entry.Name()] = true
	}
	for _, entry := range entries {
		if !names[entry.Name()] {
			if err := r.move(filepath.Join(rel, entry.Name())); err != nil {
				return err
			}
		}
	}
	for _, entry := range keptEntries {
		if err := r.sync(filepath.Join(rel, entry.Name())); err != nil {
			return err
		}
	}
	if home.Mode().Perm() != kept.Mode().Perm() {
		return r.root.Chmod(rel, kept.Mode().Perm())
	}
	return nil
}

// sameFile reports a home file that matches its kept copy in permissions and
// bytes.
func (r *restorer) sameFile(rel string, home, kept fs.FileInfo) (bool, error) {
	if home.Size() != kept.Size() || home.Mode().Perm() != kept.Mode().Perm() {
		return false, nil
	}
	got, err := r.root.ReadFile(rel)
	if err != nil {
		return false, err
	}
	want, err := os.ReadFile(filepath.Join(r.blessed, rel))
	if err != nil {
		return false, err
	}
	return bytes.Equal(got, want), nil
}

// move takes rel out of the home into the quarantine folder. The rename goes
// through the folder rel sits in, opened inside the home, so a link planted
// above it cannot send anything else away.
func (r *restorer) move(rel string) error {
	target := filepath.Join(r.quarantine, rel)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	parent, err := r.root.Open(filepath.Dir(rel))
	if err != nil {
		return err
	}
	defer parent.Close()
	into, err := os.Open(filepath.Dir(target))
	if err != nil {
		return err
	}
	defer into.Close()
	name := filepath.Base(rel)
	err = syscall.Renameat(int(parent.Fd()), name, int(into.Fd()), name)
	if errors.Is(err, syscall.EXDEV) {
		if err = copyOut(r.root, rel, target); err == nil {
			err = r.root.RemoveAll(rel)
		}
	}
	if err != nil {
		return err
	}
	r.quarantined = append(r.quarantined, rel)
	return nil
}

// put copies rel from the kept copy into the home, where nothing stands now.
func (r *restorer) put(rel string) error {
	if dir := filepath.Dir(rel); dir != "." {
		if err := r.root.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	if err := copyIn(r.root, filepath.Join(r.blessed, rel), rel); err != nil {
		return err
	}
	r.putBack = append(r.putBack, rel)
	return nil
}

// copyOut copies rel from inside root to dst, links as links. Anything but
// folders, files and links is left out.
func copyOut(root *os.Root, rel, dst string) error {
	info, err := root.Lstat(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	switch {
	case info.IsDir():
		if err := os.Mkdir(dst, info.Mode().Perm()|0o700); err != nil {
			return err
		}
		dir, err := root.Open(rel)
		if err != nil {
			return err
		}
		entries, err := dir.ReadDir(-1)
		dir.Close()
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copyOut(root, filepath.Join(rel, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
				return err
			}
		}
		return os.Chmod(dst, info.Mode().Perm())
	case info.Mode().Type() == fs.ModeSymlink:
		target, err := root.Readlink(rel)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	case info.Mode().IsRegular():
		src, err := root.OpenFile(rel, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
		if err != nil {
			return err
		}
		defer src.Close()
		out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, src); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	}
	return nil
}

// copyIn copies src, a kept copy Rashin made, to rel inside root.
func copyIn(root *os.Root, src, rel string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	switch {
	case info.IsDir():
		if err := root.Mkdir(rel, 0o700); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copyIn(root, filepath.Join(src, entry.Name()), filepath.Join(rel, entry.Name())); err != nil {
				return err
			}
		}
		return root.Chmod(rel, info.Mode().Perm())
	case info.Mode().Type() == fs.ModeSymlink:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return root.Symlink(target, rel)
	case info.Mode().IsRegular():
		in, err := os.Open(src)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, info.Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	}
	return nil
}

// instanceCommand is the command a harness instance runs as its settings
// give it, nil for a built-in harness.
func instanceCommand(id string, settings protocol.Settings) []string {
	if builtin(id) {
		return nil
	}
	for _, instance := range settings.Agent.Instances {
		if strings.TrimSpace(instance.ID) == id {
			return instance.Command
		}
	}
	return nil
}
