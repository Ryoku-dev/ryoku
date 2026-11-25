package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	i18n "ryoku-i18n"
	"ryoku-rashin/internal/protocol"
)

// Names tells memory what its scopes are called and where projects live.
// The daemon implements it from Work and Teams.
type Names interface {
	// Project is the Work project with this id: its root folder, its name
	// and its ProjectSettings.Memory.
	Project(id string) (root, name, mode string, ok bool)
	// ProjectAt is the Work project whose root is this folder.
	ProjectAt(root string) (id, name, mode string, ok bool)
	// Team is the team with this id.
	Team(id string) (protocol.Team, bool)
}

// project is a repository as memory knows it: its scope, name and whether
// its memory is kept to itself.
type project struct {
	Scope string
	Name  string
	Kept  bool
}

// projectKeys caches a repository root's key and a folder's repository
// root; the git calls behind them are not repeated for the life of the
// engine.
type projectKeys struct {
	mu    sync.Mutex
	keys  map[string]string
	roots map[string]string
}

// gitTimeout bounds each git call that finds a project's key.
const gitTimeout = 3 * time.Second

// key is the 12 hex characters naming a repository: its oldest root commit,
// else a hash of its origin URL, else a hash of its path. A folder that is
// not a repository is named by its path.
func (k *projectKeys) key(root string) string {
	k.mu.Lock()
	defer k.mu.Unlock()
	if key, ok := k.keys[root]; ok {
		return key
	}
	key := ""
	if out := git(root, "rev-list", "--max-parents=0", "HEAD"); out != "" {
		lines := strings.Fields(out)
		key = lines[len(lines)-1]
	} else if url := git(root, "config", "--get", "remote.origin.url"); url != "" {
		key = hashHex(url)
	} else {
		key = hashHex(filepath.Clean(root))
	}
	if len(key) > 12 {
		key = key[:12]
	}
	if k.keys == nil {
		k.keys = map[string]string{}
	}
	k.keys[root] = key
	return key
}

// root is the top of the repository holding dir, or "" outside one.
func (k *projectKeys) root(dir string) string {
	if dir == "" {
		return ""
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	if root, ok := k.roots[dir]; ok {
		return root
	}
	root := git(dir, "rev-parse", "--show-toplevel")
	if k.roots == nil {
		k.roots = map[string]string{}
	}
	k.roots[dir] = root
	return root
}

// git runs one read-only git command in dir and returns its trimmed output,
// "" on any failure.
func git(dir string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// hashHex is the hex sha256 of s.
func hashHex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// projectAt names the repository at root: the Work project's name and mode
// when it is one, else the folder's name.
func (e *Engine) projectAt(root string) project {
	p := project{Scope: protocol.MemoryProjectPrefix + e.keys.key(root), Name: filepath.Base(root)}
	mode := protocol.ProjectMemoryShared
	if e.deps.Names != nil {
		if _, name, projectMode, ok := e.deps.Names.ProjectAt(root); ok {
			if name != "" {
				p.Name = name
			}
			mode = projectMode
		}
	}
	p.Kept = mode == protocol.ProjectMemoryKept
	e.mu.Lock()
	e.modes[p.Scope] = mode
	e.mu.Unlock()
	return p
}

// sessionProject is the project a session works in: its Work project's root,
// else the repository holding its folder. ok is false outside any project.
func (e *Engine) sessionProject(info protocol.SessionInfo) (project, bool) {
	root := ""
	if info.Project != "" && e.deps.Names != nil {
		if r, _, _, ok := e.deps.Names.Project(info.Project); ok {
			root = r
		}
	}
	if root == "" {
		root = e.keys.root(info.Cwd)
	}
	if root == "" {
		return project{}, false
	}
	p := e.projectAt(root)
	if st, err := e.store(); err == nil {
		st.nameScope(p.Scope, p.Name)
	}
	return p, true
}

// sessionScope is the project scope of a session, "" outside a project, and
// whether that project keeps its memory to itself.
func (e *Engine) sessionScope(info protocol.SessionInfo) (scope string, kept bool) {
	p, ok := e.sessionProject(info)
	if !ok {
		return "", false
	}
	return p.Scope, p.Kept
}

// scopeName says a scope in words: "About you", a project's or a team's
// name.
func (e *Engine) scopeName(scope string) string {
	if st, err := e.store(); err == nil {
		return st.scopeName(scope)
	}
	return defaultScopeName(scope, e.deps.Names)
}

// defaultScopeName names a scope without the ledger: "About you", the
// team's name, or the project's key.
func defaultScopeName(scope string, names Names) string {
	switch {
	case scope == protocol.MemoryYou:
		return i18n.T("About you")
	case strings.HasPrefix(scope, protocol.MemoryTeamPrefix):
		id := strings.TrimPrefix(scope, protocol.MemoryTeamPrefix)
		if names != nil {
			if team, ok := names.Team(id); ok && team.Name != "" {
				return team.Name
			}
		}
		return id
	default:
		return strings.TrimPrefix(scope, protocol.MemoryProjectPrefix)
	}
}

var (
	projectKey = regexp.MustCompile(`^[0-9a-f]{12}$`)
	teamID     = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
)

// validScope reports an exact scope memory can store.
func validScope(scope string) bool {
	switch {
	case scope == protocol.MemoryYou:
		return true
	case strings.HasPrefix(scope, protocol.MemoryProjectPrefix):
		return projectKey.MatchString(strings.TrimPrefix(scope, protocol.MemoryProjectPrefix))
	case strings.HasPrefix(scope, protocol.MemoryTeamPrefix):
		return teamID.MatchString(strings.TrimPrefix(scope, protocol.MemoryTeamPrefix))
	}
	return false
}

// inScope reports whether a memory's scope is selected by filter: "" for
// all, MemoryProjects or MemoryTeams for a group, or one exact scope.
func inScope(scope, filter string) bool {
	switch filter {
	case "":
		return true
	case protocol.MemoryProjects:
		return strings.HasPrefix(scope, protocol.MemoryProjectPrefix)
	case protocol.MemoryTeams:
		return strings.HasPrefix(scope, protocol.MemoryTeamPrefix)
	}
	return scope == filter
}

// inScopes reports whether scope is selected by any filter; no filters
// select everything.
func inScopes(scope string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	for _, filter := range filters {
		if inScope(scope, filter) {
			return true
		}
	}
	return false
}

// slug makes a file-name part from a name: lower-case letters and digits
// joined by dashes, at most 40 characters.
func slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if b.Len() > 0 && !dash {
			b.WriteByte('-')
			dash = true
		}
		if b.Len() >= 40 {
			break
		}
	}
	return strings.Trim(b.String(), "-")
}
