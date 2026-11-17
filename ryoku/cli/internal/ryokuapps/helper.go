package ryokuapps

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"regexp"
	"strings"
	"syscall"
	"time"

	"ryoku-cli/internal/apprelease"
	"ryoku-cli/internal/host"
	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// Exit codes of `ryoku-host apps`, matching the rest of ryoku-host.
const (
	exitOK      = host.ExitOK
	exitFailure = host.ExitFailure
	exitUsage   = host.ExitUsage
	// exitRefused: a request the helper will not run here (AUR needs a
	// terminal; the desktop's own packages stay installed).
	exitRefused = host.ExitFalse
	// exitCancelled: stopped by a signal or a stdin cancel (128 + SIGINT, as gpk).
	exitCancelled = 130
)

// packageNamePattern admits the names pacman, DNF and XBPS use and nothing
// that could read as an option.
var packageNamePattern = regexp.MustCompile(`^[A-Za-z0-9@_+][A-Za-z0-9@._+-]{0,127}$`)

const maxPackages = 64

type queryFunc func(name string, args ...string) (stdout string, code int)

// Helper runs the root side of app installs and removals. Every package
// transaction goes through gpk with privilege escalation off (the helper is
// already root) and a gpk home of its own, so nothing reads or writes a user's
// gpk state.
type Helper struct {
	Manager host.PackageManager
	Line    string
	Gpk     string
	GpkHome string
	Stdout  io.Writer
	Stderr  io.Writer
	Query   queryFunc
	// Release installs or updates a GitHub-released app (update true) and is
	// the apprelease lane's Ensure or Upgrade in production.
	Release func(ctx context.Context, app apprelease.App, update bool, opts apprelease.Options) (apprelease.Status, error)
	// Signals delivers SIGINT, SIGTERM and SIGHUP sent to the helper; they are
	// passed on to the running transaction.
	Signals <-chan os.Signal
	// Stdin is read only under --stdin-cancel: its end, or a line "cancel",
	// stops the transaction. A caller that cannot signal a root process (the
	// pkexec child of a desktop app) cancels this way.
	Stdin io.Reader

	events *events
	cancel chan os.Signal
}

// HelperMain is `ryoku-host apps ...` on the real system.
func HelperMain(args []string) int {
	manager, err := host.Default().PackageManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ryoku-host: %v\n", err)
		return exitFailure
	}
	signals := make(chan os.Signal, 4)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(signals)
	h := Helper{
		Manager: manager,
		Line:    callerLine(os.Getenv),
		Gpk:     "/usr/bin/gpk",
		GpkHome: "/var/cache/ryoku-host/gpk",
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Stdin:   os.Stdin,
		Query:   systemQuery,
		Release: func(ctx context.Context, app apprelease.App, update bool, opts apprelease.Options) (apprelease.Status, error) {
			if update {
				return app.UpgradeWith(ctx, opts)
			}
			return app.EnsureWith(ctx, opts)
		},
		Signals: signals,
	}
	return h.Run(args)
}

// callerLine is the release line of the account that asked: pkexec and sudo
// name it, a direct call is its own.
func callerLine(getenv func(string) string) string {
	for _, key := range []string{"PKEXEC_UID", "SUDO_UID"} {
		if uid := getenv(key); uid != "" {
			account, err := user.LookupId(uid)
			if err != nil {
				return ""
			}
			return sys.SourceLineAt(account.HomeDir)
		}
	}
	if os.Geteuid() != 0 {
		return sys.SourceLine()
	}
	return ""
}

// systemQuery runs a read-only package query in the C locale.
func systemQuery(name string, args ...string) (string, int) {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return string(out), 0
	case errors.As(err, &exit):
		return string(out), exit.ExitCode()
	}
	return "", -1
}

// Run dispatches one `ryoku-host apps` call.
func (h *Helper) Run(args []string) int {
	h.events = &events{w: h.Stdout}
	if len(args) > 0 && args[0] == "--stdin-cancel" {
		args = args[1:]
		h.cancel = make(chan os.Signal, 1)
		go h.watchStdin()
	}
	if len(args) == 0 {
		fmt.Fprint(h.Stderr, host.AppsUsage)
		return exitUsage
	}
	switch args[0] {
	case "install", "remove":
		return h.packages(args[0], args[1:])
	case "ryoku":
		return h.ryoku(args[1:])
	}
	fmt.Fprint(h.Stderr, host.AppsUsage)
	return exitUsage
}

// watchStdin asks for a stop when stdin ends or says "cancel".
func (h *Helper) watchStdin() {
	if h.Stdin != nil {
		lines := bufio.NewScanner(h.Stdin)
		for lines.Scan() {
			if strings.TrimSpace(lines.Text()) == "cancel" {
				break
			}
		}
	}
	h.cancel <- syscall.SIGTERM
}

// fail ends the event stream with a refusal or an error, saying why on stderr
// as well for a caller that only reads the exit status.
func (h *Helper) fail(exit int, message string) int {
	fmt.Fprintf(h.Stderr, "ryoku-host: %s\n", message)
	h.events.done(false, exit, message)
	return exit
}

func (h *Helper) packages(action string, args []string) int {
	manager, withDeps, names, err := parsePackageArgs(action, args)
	if err != nil {
		return h.fail(exitUsage, err.Error())
	}
	switch manager {
	case "aur":
		return h.fail(exitRefused, i18n.T("AUR packages build from source and need a terminal."))
	case "flatpak":
		return h.fail(exitUsage, i18n.T("Flatpak apps install for your account, not through ryoku-host."))
	case string(host.Pacman), string(host.DNF), string(host.XBPS):
	default:
		return h.fail(exitUsage, i18n.Tf("Unknown package manager %q.", manager))
	}
	if host.PackageManager(manager) != h.Manager {
		return h.fail(exitUsage, i18n.Tf("This system installs packages with %s, not %s.", h.Manager, manager))
	}
	if action == "remove" {
		protected := protectedPackages(h.Manager, h.Query)
		for _, name := range names {
			if protected.covers(name) {
				return h.fail(exitRefused, i18n.Tf("%s is part of the Ryoku desktop and stays installed.", name))
			}
		}
	}
	return h.gpk(action, h.Manager, names, withDeps)
}

func parsePackageArgs(action string, args []string) (manager string, withDeps bool, names []string, err error) {
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "--manager" || arg == "-m":
			if i+1 >= len(args) || manager != "" {
				return "", false, nil, errors.New(i18n.T("Name one package manager with --manager."))
			}
			manager = args[i+1]
			i++
		case arg == "--with-deps" && action == "remove":
			withDeps = true
		case packageNamePattern.MatchString(arg):
			if !seen[arg] {
				seen[arg] = true
				names = append(names, arg)
			}
		default:
			return "", false, nil, errors.New(i18n.Tf("%q is not a package name.", arg))
		}
	}
	switch {
	case manager == "":
		return "", false, nil, errors.New(i18n.T("Name one package manager with --manager."))
	case len(names) == 0:
		return "", false, nil, errors.New(i18n.T("Name at least one package."))
	case len(names) > maxPackages:
		return "", false, nil, errors.New(i18n.Tf("At most %d packages at a time.", maxPackages))
	}
	return manager, withDeps, names, nil
}

func (h *Helper) ryoku(args []string) int {
	if len(args) != 2 {
		return h.fail(exitUsage, strings.TrimSpace(host.AppsUsage))
	}
	verb, id := args[0], args[1]
	if verb != "install" && verb != "update" && verb != "remove" {
		return h.fail(exitUsage, i18n.Tf("Unknown action %q.", verb))
	}
	app, ok := Lookup(id)
	if !ok {
		return h.fail(exitUsage, i18n.Tf("%q is not a Ryoku app.", id))
	}
	if app.Builtin {
		return h.fail(exitRefused, i18n.Tf("%s is part of the Ryoku desktop: it updates with Ryoku and is not installed or removed on its own.", app.Name))
	}
	if verb == "remove" {
		return h.gpk("remove", h.removalManager(app), []string{app.Package}, false)
	}
	if app.LaneOn(h.Manager) == LaneAppRelease {
		return h.release(app, verb == "update")
	}
	action := "install"
	if verb == "update" {
		action = "upgrade"
	}
	return h.gpk(action, h.Manager, []string{app.Package}, false)
}

// removalManager is the gpk manager that owns an installed app. A package the
// release lane installed with pacman -U is foreign to the sync databases, and
// gpk files foreign pacman packages under aur; removing one builds nothing.
func (h *Helper) removalManager(app App) host.PackageManager {
	if h.Manager == host.Pacman && app.Release != nil {
		if _, code := h.Query("pacman", "-Qqm", "--", app.Package); code == 0 {
			return "aur"
		}
	}
	return h.Manager
}

// release installs or updates a GitHub-released app through the verified
// apprelease lane, in the same event shapes gpk prints.
func (h *Helper) release(app App, update bool) int {
	action := "install"
	if update {
		action = "upgrade"
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop := h.relay(func(os.Signal) { cancel() })
	defer stop()

	logs := &logWriter{events: h.events}
	planned := false
	st, err := h.Release(ctx, app.ReleaseOn(h.Line), update, apprelease.Options{
		Output: logs,
		Resolved: func(_, latest string) {
			h.events.plan(action, []Step{{Manager: string(LaneAppRelease), Name: app.Package, Version: latest}})
			h.events.stepStart(0)
			planned = true
		},
	})
	logs.Flush()
	switch {
	case err != nil && ctx.Err() != nil:
		if planned {
			h.events.stepDone(0, exitCancelled)
		}
		return h.fail(exitCancelled, i18n.T("cancelled"))
	case err != nil:
		if planned {
			h.events.stepDone(0, exitFailure)
		}
		return h.fail(exitFailure, err.Error())
	case st.Updated:
		h.events.stepDone(0, exitOK)
	case update && st.Installed == "":
		return h.fail(exitFailure, i18n.Tf("%s is not installed.", app.Name))
	default:
		h.events.plan(action, nil)
		h.events.log(0, i18n.Tf("%s %s is the newest release.", app.Name, st.Installed))
	}
	h.events.done(true, exitOK, "")
	return exitOK
}

// gpk runs one gpk transaction and relays its NDJSON events. A line that is
// not an event becomes a log event, and a run that ends without a done event
// (gpk missing or killed) still ends the stream with one.
func (h *Helper) gpk(action string, manager host.PackageManager, names []string, withDeps bool) int {
	args := append([]string{action}, names...)
	args = append(args, "--manager", string(manager))
	if withDeps {
		args = append(args, "--with-deps")
	}
	args = append(args, "--yes", "--events", "--privilege", "none")

	if err := os.MkdirAll(h.GpkHome, 0o700); err != nil {
		return h.fail(exitFailure, err.Error())
	}
	cmd := exec.Command(h.Gpk, args...)
	cmd.Env = gpkEnvironment(h.GpkHome)
	relay := &gpkRelay{events: h.events}
	cmd.Stdout, cmd.Stderr = relay, h.Stderr
	// A daemon a package hook starts can inherit gpk's output; once gpk has
	// exited, its result does not wait on that daemon.
	cmd.WaitDelay = gpkWaitDelay
	if err := cmd.Start(); err != nil {
		return h.fail(127, i18n.Tf("Could not start gpk: %v", err))
	}
	stopped := make(chan struct{}, 1)
	stop := h.relay(func(sig os.Signal) {
		select {
		case stopped <- struct{}{}:
		default:
		}
		_ = cmd.Process.Signal(sig)
	})
	_ = cmd.Wait()
	stop()
	relay.Flush()
	code, message := exitFailure, ""
	switch {
	case cmd.ProcessState != nil && cmd.ProcessState.ExitCode() >= 0:
		code = cmd.ProcessState.ExitCode()
	case len(stopped) > 0:
		// Stopped before gpk could report it itself.
		code, message = exitCancelled, i18n.T("cancelled")
	}
	if code != 0 && message == "" {
		message = i18n.Tf("gpk %s ended with exit %d.", action, code)
	}
	h.events.done(code == 0, code, message)
	return code
}

const gpkWaitDelay = 5 * time.Second

// gpkRelay passes gpk's NDJSON events through as they arrive. A line that is
// not an event becomes a log event; gpk's own done event ends the stream.
type gpkRelay struct {
	events  *events
	pending []byte
}

func (r *gpkRelay) Write(p []byte) (int, error) {
	r.pending = append(r.pending, p...)
	for {
		cut := bytes.IndexByte(r.pending, '\n')
		if cut < 0 {
			if len(r.pending) > maxEventBytes {
				r.line(r.pending)
				r.pending = r.pending[:0]
			}
			return len(p), nil
		}
		r.line(r.pending[:cut])
		r.pending = r.pending[cut+1:]
	}
}

// Flush passes on a last line that had no newline.
func (r *gpkRelay) Flush() {
	r.line(r.pending)
	r.pending = nil
}

// maxEventBytes bounds one buffered line from gpk.
const maxEventBytes = 4 << 20

func (r *gpkRelay) line(line []byte) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return
	}
	var probe struct {
		Event string `json:"event"`
	}
	if json.Unmarshal(line, &probe) != nil || probe.Event == "" {
		r.events.log(0, string(line))
		return
	}
	if probe.Event == "done" {
		r.events.mu.Lock()
		r.events.finished = true
		r.events.mu.Unlock()
	}
	r.events.raw(line)
}

// relay passes every signal the helper receives, and a stdin cancel, to act
// until stop is called.
func (h *Helper) relay(act func(os.Signal)) (stop func()) {
	if h.Signals == nil && h.cancel == nil {
		return func() {}
	}
	quit := make(chan struct{})
	go func() {
		for {
			select {
			case sig := <-h.Signals:
				act(sig)
			case sig := <-h.cancel:
				act(sig)
			case <-quit:
				return
			}
		}
	}()
	return func() { close(quit) }
}

// gpkEnvironment is the whole environment gpk runs with as root: a fixed
// PATH, its own home and XDG dirs, escalation off, and the caller's language.
func gpkEnvironment(home string) []string {
	env := []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/bin:/usr/sbin:/bin:/sbin",
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + home + "/config",
		"XDG_DATA_HOME=" + home + "/data",
		"XDG_CACHE_HOME=" + home + "/cache",
		"XDG_STATE_HOME=" + home + "/state",
		"GPK_PRIVILEGE=none",
		"NO_COLOR=1",
	}
	for _, key := range []string{"LANG", "LANGUAGE", "LC_ALL", "LC_MESSAGES"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}
