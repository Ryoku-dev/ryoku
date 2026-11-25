package launch

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"ryoku-rashin/internal/reflex/proc"
)

// Scope records where a launched app runs.
const (
	// ScopeSystemd: inside a transient scope of the systemd user manager,
	// so it outlives the daemon's own unit.
	ScopeSystemd = "systemd"
	// ScopeFallback: plainly detached in its own session, because there is
	// no systemd user manager or it refused the scope.
	ScopeFallback = "fallback"
)

// scopePrefix wraps an app's argv in a transient user scope that is
// collected when the app ends.
var scopePrefix = []string{"systemd-run", "--user", "--scope", "--collect", "--"}

var (
	// ErrCooldown means the same app (or the same browser action) was
	// started less than the cooldown ago. Nothing was started; this is not
	// a failure.
	ErrCooldown = errors.New("launch: started a moment ago")
	// ErrNotFound means the name is not in the catalog.
	ErrNotFound = errors.New("launch: no such app in the catalog")
	// ErrBadURL means an address is not one a browser may be handed.
	ErrBadURL = errors.New("launch: not an address a browser may open")
	// ErrExited means the app ended with an error straight after starting.
	ErrExited = errors.New("launch: the app ended at once")
)

// Record is what one start did, for the ledger and the lab.
type Record struct {
	ID, Name string
	Argv     []string // the argv that was started, scope wrapper included
	Scope    string   // ScopeSystemd or ScopeFallback
	Reason   string   // why the fallback was taken, empty otherwise
}

// Options configures a Launcher. Zero values take the real defaults.
type Options struct {
	// Env is the environment apps start with.
	Env []string
	// Spawn starts a detached argv; default proc.Detached(Env, $HOME).
	Spawn proc.Spawner
	// Systemd reports whether the systemd user manager runs; default
	// SystemdUser(Env).
	Systemd func() bool
	// Cooldown is reflex.launchCooldown; zero means 4 s.
	Cooldown time.Duration
	// Grace is how long a scoped start is watched for the scope's refusal;
	// zero means 250 ms.
	Grace time.Duration
	// Now is the clock; default time.Now.
	Now func() time.Time
}

// Launcher starts catalog apps.
type Launcher struct {
	cat      *Catalog
	spawn    proc.Spawner
	systemd  func() bool
	cooldown time.Duration
	grace    time.Duration
	now      func() time.Time

	mu   sync.Mutex
	last map[string]time.Time
}

// New returns a Launcher for cat. Every start is checked against
// cat.Allowed before it happens.
func New(cat *Catalog, o Options) *Launcher {
	if o.Spawn == nil {
		home, _ := os.UserHomeDir()
		o.Spawn = proc.Detached(o.Env, home)
	}
	if o.Systemd == nil {
		env := o.Env
		o.Systemd = func() bool { return SystemdUser(env) }
	}
	if o.Cooldown <= 0 {
		o.Cooldown = 4 * time.Second
	}
	if o.Grace <= 0 {
		o.Grace = 250 * time.Millisecond
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	return &Launcher{
		cat: cat, spawn: proc.GuardSpawn(cat.Allowed, o.Spawn), systemd: o.Systemd,
		cooldown: o.Cooldown, grace: o.Grace, now: o.Now, last: map[string]time.Time{},
	}
}

// Catalog is the catalog the launcher starts apps from.
func (l *Launcher) Catalog() *Catalog { return l.cat }

// Launch opens the catalog app called name.
func (l *Launcher) Launch(ctx context.Context, name string) (Record, error) {
	e, ok := l.cat.Lookup(name)
	if !ok {
		return Record{}, fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	return l.start(ctx, "app:"+e.ID, e, e.Argv())
}

// OpenURL hands address to the catalog browser with the given ID through
// the browser's own Exec line: a running browser opens it in a new tab.
// address is "about:blank" or an https address.
func (l *Launcher) OpenURL(ctx context.Context, id, address string) (Record, error) {
	e, ok := l.cat.ByID(id)
	if !ok || !e.Browser() {
		return Record{}, fmt.Errorf("%w: browser %q", ErrNotFound, id)
	}
	if !AllowedURL(address) {
		return Record{}, ErrBadURL
	}
	return l.start(ctx, "url:"+e.ID+"\x00"+address, e, e.WithURL(address))
}

// start runs argv for e, unless key was started within the cooldown.
// Where the user manager runs, argv goes inside a transient scope; when the
// scope is refused (systemd-run ends with status 1 inside the grace, before
// the app could run) or cannot start, argv starts plainly detached and the
// record says so.
func (l *Launcher) start(ctx context.Context, key string, e Entry, argv []string) (Record, error) {
	l.mu.Lock()
	now := l.now()
	if at, ok := l.last[key]; ok && now.Sub(at) < l.cooldown {
		l.mu.Unlock()
		return Record{}, ErrCooldown
	}
	l.last[key] = now
	l.mu.Unlock()

	rec := Record{ID: e.ID, Name: e.Name}
	if l.systemd() {
		scoped := append(slices.Clone(scopePrefix), argv...)
		exited, err := l.spawn(scoped)
		if err == nil {
			rec.Argv, rec.Scope = scoped, ScopeSystemd
			timer := time.NewTimer(l.grace)
			defer timer.Stop()
			select {
			case err := <-exited:
				switch {
				case err == nil:
					return rec, nil
				case exitStatus(err) == 1:
					rec.Reason = "scope refused"
				default:
					return rec, fmt.Errorf("%w: %v", ErrExited, err)
				}
			case <-timer.C:
				return rec, nil
			case <-ctx.Done():
				return rec, nil
			}
		} else {
			if errors.Is(err, proc.ErrNotAllowed) {
				return Record{}, err
			}
			rec.Reason = "scope unavailable"
		}
	} else {
		rec.Reason = "no systemd user manager"
	}
	if _, err := l.spawn(argv); err != nil {
		return Record{}, err
	}
	rec.Argv, rec.Scope = slices.Clone(argv), ScopeFallback
	return rec, nil
}

func exitStatus(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	var coded interface{ ExitCode() int }
	if errors.As(err, &coded) {
		return coded.ExitCode()
	}
	return -1
}

// Allowed reports whether name and args are a start this catalog permits:
// exactly one app's command with its field codes stripped, or a browser's
// command carrying one allowed address where it takes one, each either bare
// or inside the scope wrapper. Nothing else, and never a shell.
func (c *Catalog) Allowed(name string, args ...string) bool {
	argv := append([]string{name}, args...)
	if len(argv) > len(scopePrefix) && slices.Equal(argv[:len(scopePrefix)], scopePrefix) {
		argv = argv[len(scopePrefix):]
	}
	for _, e := range c.entries {
		if slices.Equal(argv, e.Argv()) {
			return true
		}
		if !e.Browser() {
			continue
		}
		const marker = "\x00"
		want := e.WithURL(marker)
		at := slices.Index(want, marker)
		if len(argv) != len(want) || at < 0 || !AllowedURL(argv[at]) {
			continue
		}
		if slices.Equal(argv[:at], want[:at]) && slices.Equal(argv[at+1:], want[at+1:]) {
			return true
		}
	}
	return false
}

// maxURL bounds an address handed to a browser.
const maxURL = 2048

// AllowedURL reports whether a browser may be handed address: the blank
// page, or an https address with a host, no credentials, no whitespace or
// control characters, and not starting with a dash.
func AllowedURL(address string) bool {
	if address == "about:blank" {
		return true
	}
	if len(address) > maxURL || strings.HasPrefix(address, "-") ||
		strings.IndexFunc(address, func(r rune) bool { return r <= ' ' || r == 0x7f }) >= 0 {
		return false
	}
	u, err := url.Parse(address)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Opaque == ""
}

// SystemdUser reports whether a systemd user manager runs for env's
// session: the system was booted by systemd and the user manager's private
// socket is present. It starts no process.
func SystemdUser(env []string) bool {
	if fi, err := os.Stat("/run/systemd/system"); err != nil || !fi.IsDir() {
		return false
	}
	runtime := lookup(env, "XDG_RUNTIME_DIR")
	if runtime == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(runtime, "systemd", "private"))
	return err == nil
}

// Dirs are the applications directories for env, most important first:
// $XDG_DATA_HOME/applications, then each of $XDG_DATA_DIRS.
func Dirs(env []string) []string {
	home := lookup(env, "XDG_DATA_HOME")
	if home == "" {
		if h := lookup(env, "HOME"); h != "" {
			home = filepath.Join(h, ".local", "share")
		}
	}
	var dirs []string
	if filepath.IsAbs(home) {
		dirs = append(dirs, filepath.Join(home, "applications"))
	}
	data := lookup(env, "XDG_DATA_DIRS")
	if data == "" {
		data = "/usr/local/share:/usr/share"
	}
	for _, d := range strings.Split(data, ":") {
		if filepath.IsAbs(d) {
			if p := filepath.Join(d, "applications"); !slices.Contains(dirs, p) {
				dirs = append(dirs, p)
			}
		}
	}
	return dirs
}

func lookup(env []string, key string) string {
	for i := len(env) - 1; i >= 0; i-- {
		if k, v, ok := strings.Cut(env[i], "="); ok && k == key {
			return v
		}
	}
	return ""
}
