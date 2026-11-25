package launch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func writeEntry(t *testing.T, dir, id, body string) {
	t.Helper()
	p := filepath.Join(dir, id)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func app(name, exec string, extra ...string) string {
	return "[Desktop Entry]\nType=Application\nName=" + name + "\nExec=" + exec + "\n" + strings.Join(extra, "\n") + "\n"
}

func scan(t *testing.T, max int, dirs ...string) *Catalog {
	t.Helper()
	return Scan(ScanOptions{Dirs: dirs, Max: max, Desktops: []string{"niri"}})
}

func TestExecFieldCodesAreStripped(t *testing.T) {
	dir := t.TempDir()
	writeEntry(t, dir, "dev.example.Notes.desktop",
		app("Lindqvist Notes", `/opt/notes/bin/notes --title "My \"best\" notes" %U %i --file=%f -x 100%% %c %k`)+
			"Name[de]=Notizen\n[Desktop Action other]\nName=Other\nExec=other\n")
	cat := scan(t, 200, dir)
	e, ok := cat.Lookup("lindqvist notes")
	if !ok {
		t.Fatalf("entry not catalogued: %+v", cat.Entries())
	}
	want := []string{"/opt/notes/bin/notes", "--title", `My "best" notes`, "--file=", "-x", "100%"}
	if got := e.Argv(); !slices.Equal(got, want) {
		t.Fatalf("Argv = %q, want %q", got, want)
	}
	for _, a := range e.Argv() {
		if strings.Contains(a, "%U") || strings.Contains(a, "%i") || strings.Contains(a, "%c") {
			t.Fatalf("field code survived in %q", a)
		}
	}
	if e.Name != "Lindqvist Notes" {
		t.Fatalf("a localised or action Name leaked: %q", e.Name)
	}
}

func TestWithURLTakesTheAddressSlotOrAppends(t *testing.T) {
	dir := t.TempDir()
	writeEntry(t, dir, "slot.desktop", app("Slot Browser", "slotb --profile main %u %U", "Categories=Network;WebBrowser;"))
	writeEntry(t, dir, "bare.desktop", app("Bare Browser", "bareb --new-instance", "MimeType=text/html;x-scheme-handler/https;"))
	cat := scan(t, 200, dir)
	slot, _ := cat.ByID("slot.desktop")
	bare, _ := cat.ByID("bare.desktop")
	if got, want := slot.WithURL("about:blank"), []string{"slotb", "--profile", "main", "about:blank"}; !slices.Equal(got, want) {
		t.Fatalf("slot WithURL = %q, want %q", got, want)
	}
	if got, want := bare.WithURL("https://example.org/a"), []string{"bareb", "--new-instance", "https://example.org/a"}; !slices.Equal(got, want) {
		t.Fatalf("bare WithURL = %q, want %q", got, want)
	}
	if !slot.Browser() || !bare.Browser() {
		t.Fatal("WebBrowser category and https handler must both mark a browser")
	}
}

func TestScanLeavesOutWhatMustNotLaunch(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"shell.desktop":       app("Shell Thing", `sh -c "echo hi > /tmp/x"`),
		"bash.desktop":        app("Bash Thing", `/usr/bin/bash -c run`),
		"env.desktop":         app("Env Thing", `env FOO=1 thing`),
		"sudo.desktop":        app("Root Thing", `pkexec thing`),
		"term.desktop":        app("Term Thing", `thing`, "Terminal=true"),
		"nodisplay.desktop":   app("Quiet Thing", `thing`, "NoDisplay=true"),
		"hidden.desktop":      app("Gone Thing", `thing`, "Hidden=true"),
		"link.desktop":        "[Desktop Entry]\nType=Link\nName=Link Thing\nURL=https://example.org\n",
		"quote.desktop":       app("Broken Quote", `thing "unterminated`),
		"code.desktop":        app("Bad Code", `thing %z`),
		"dash.desktop":        app("Dash Thing", `-thing`),
		"onlyshow.desktop":    app("Other Desktop", `thing`, "OnlyShowIn=KDE;"),
		"notshow.desktop":     app("Not Here", `thing`, "NotShowIn=niri;"),
		"tryexec.desktop":     app("Not Installed", `thing`, "TryExec=missing-binary"),
		"noexec.desktop":      "[Desktop Entry]\nType=Application\nName=No Exec\n",
		"nogroup.desktop":     "[Other Group]\nType=Application\nName=No Group\nExec=thing\n",
		"emptyexec.desktop":   app("Only Codes", `%U`),
		"ok.desktop":          app("Fine App", `fine-app %F`),
		"tryexec-ok.desktop":  app("Installed App", `installed-app`, "TryExec=installed-app"),
		"onlyshow-ok.desktop": app("Niri App", `niri-app`, "OnlyShowIn=GNOME;niri;"),
	}
	for id, body := range cases {
		writeEntry(t, dir, id, body)
	}
	cat := Scan(ScanOptions{Dirs: []string{dir}, Max: 200, Desktops: []string{"niri"},
		LookPath: func(name string) (string, error) {
			if name == "installed-app" {
				return "/usr/bin/installed-app", nil
			}
			return "", errors.New("not found")
		}})
	var got []string
	for _, e := range cat.Entries() {
		got = append(got, e.ID)
	}
	slices.Sort(got)
	want := []string{"ok.desktop", "onlyshow-ok.desktop", "tryexec-ok.desktop"}
	if !slices.Equal(got, want) {
		t.Fatalf("catalog = %q, want %q", got, want)
	}
}

func TestCatalogIsBoundedAndUserEntriesWin(t *testing.T) {
	user, system := t.TempDir(), t.TempDir()
	for i := range 250 {
		writeEntry(t, system, fmt.Sprintf("app%03d.desktop", i), app(fmt.Sprintf("System App %03d", i), fmt.Sprintf("app%03d", i)))
	}
	writeEntry(t, user, "app000.desktop", app("Mine", "my-app"))
	writeEntry(t, user, "app001.desktop", app("Masked", "masked", "Hidden=true"))
	writeEntry(t, user, "sub/tool.desktop", app("Sub Tool", "sub-tool"))

	cat := scan(t, 200, user, system)
	if cat.Len() != 200 {
		t.Fatalf("catalog holds %d apps, want the bound 200", cat.Len())
	}
	if cat.Dropped != 50 {
		t.Fatalf("Dropped = %d, want 50 (250 launchable after the mask, bound 200)", cat.Dropped)
	}
	if e, ok := cat.ByID("app000.desktop"); !ok || e.Name != "Mine" {
		t.Fatalf("the user entry must override the system one: %+v %v", e, ok)
	}
	if _, ok := cat.ByID("app001.desktop"); ok {
		t.Fatal("a Hidden user entry must mask the system entry with its ID")
	}
	if _, ok := cat.ByID("sub-tool.desktop"); !ok {
		t.Fatal("a subdirectory entry's ID joins its path with '-'")
	}
	again := scan(t, 200, user, system)
	if !slices.EqualFunc(cat.Entries(), again.Entries(), func(a, b Entry) bool { return a.ID == b.ID }) {
		t.Fatal("which 200 apps are kept must not vary between scans")
	}
	if scan(t, 3, user, system).Len() != 3 {
		t.Fatal("the bound is the caller's catalogMax")
	}
}

func TestResolveAndLookup(t *testing.T) {
	dir := t.TempDir()
	writeEntry(t, dir, "dev.ryoku.OniNotes.desktop", app("Lindqvist Notes", "notes-app"))
	writeEntry(t, dir, "org.example.Calculator.desktop", app("Calculator", "calc-app"))
	cat := scan(t, 200, dir)

	name, exec, score, ok := cat.Resolve("lindqvist notes")
	if !ok || name != "Lindqvist Notes" || exec != "notes-app" || score != 1 {
		t.Fatalf("exact name: %q %q %v %v", name, exec, score, ok)
	}
	if name, _, _, ok := cat.Resolve("OniNotes"); !ok || name != "Lindqvist Notes" {
		t.Fatalf("the ID's last part is an alias: %q %v", name, ok)
	}
	if name, _, _, ok := cat.Resolve("lindkvist notes"); !ok || name != "Lindqvist Notes" {
		t.Fatalf("a near spelling must resolve: %q %v", name, ok)
	}
	if _, _, _, ok := cat.Resolve("spreadsheet"); ok {
		t.Fatal("an unrelated word must not resolve to an app")
	}
	if e, ok := cat.Lookup("CALCULATOR"); !ok || e.ID != "org.example.Calculator.desktop" {
		t.Fatalf("Lookup ignores case: %+v %v", e, ok)
	}
	if _, ok := cat.Lookup("calculater"); ok {
		t.Fatal("Lookup is exact, never fuzzy")
	}
	if !cat.Alias("browser", "org.example.Calculator.desktop") || cat.Alias("browser", "missing.desktop") {
		t.Fatal("Alias reports whether the ID is catalogued")
	}
	if e, ok := cat.Lookup("browser"); !ok || e.Name != "Calculator" {
		t.Fatal("an added alias must look up")
	}
}

func browserCatalog(t *testing.T) *Catalog {
	t.Helper()
	dir := t.TempDir()
	writeEntry(t, dir, "notes.desktop", app("Lindqvist Notes", "notes-app --x %U"))
	writeEntry(t, dir, "web.desktop", app("Web", "webb --profile main %u", "Categories=Network;WebBrowser;"))
	return scan(t, 200, dir)
}

func TestAllowedIsExact(t *testing.T) {
	cat := browserCatalog(t)
	scoped := func(argv ...string) []string { return append(slices.Clone(scopePrefix), argv...) }
	allowed := [][]string{
		{"notes-app", "--x"},
		scoped("notes-app", "--x"),
		{"webb", "--profile", "main"},
		{"webb", "--profile", "main", "about:blank"},
		{"webb", "--profile", "main", "https://example.org/path?q=a%20b"},
		scoped("webb", "--profile", "main", "https://example.org/"),
	}
	for _, argv := range allowed {
		if !cat.Allowed(argv[0], argv[1:]...) {
			t.Errorf("refused %q", argv)
		}
	}
	refused := [][]string{
		{"notes-app"},
		{"notes-app", "--x", "--extra"},
		{"notes-app", "--y"},
		{"sh", "-c", "notes-app --x"},
		{"bash", "-c", "notes-app"},
		{"systemd-run", "--user", "--scope", "--", "notes-app", "--x"},
		{"systemd-run", "--user", "--scope", "--collect", "--", "rm", "-rf", "/"},
		scoped(),
		{"notes-app", "--x", "https://example.org/"},
		{"webb", "--profile", "main", "http://example.org/"},
		{"webb", "--profile", "main", "file:///etc/passwd"},
		{"webb", "--profile", "main", "javascript:alert(1)"},
		{"webb", "--profile", "main", "https://example.org/a b"},
		{"webb", "--profile", "main", "https://user:pw@example.org/"},
		{"webb", "--profile", "main", "--remote-debugging-port=9"},
		{"webb", "--profile", "other", "about:blank"},
		{"webb", "--profile", "main", "about:blank", "about:blank"},
		{"curl", "https://example.org/"},
	}
	for _, argv := range refused {
		if cat.Allowed(argv[0], argv[1:]...) {
			t.Errorf("allowed %q", argv)
		}
	}
}

type exitErr int

func (e exitErr) Error() string { return fmt.Sprintf("exit status %d", int(e)) }
func (e exitErr) ExitCode() int { return int(e) }

// fakeSpawn records every argv; each call takes the next behaviour: an
// exit to deliver at once, a start error, or (none left) a running app.
type fakeSpawn struct {
	calls [][]string
	steps []func() (<-chan error, error)
}

func (f *fakeSpawn) spawn(argv []string) (<-chan error, error) {
	f.calls = append(f.calls, slices.Clone(argv))
	if len(f.steps) == 0 {
		return make(chan error), nil // still running
	}
	step := f.steps[0]
	f.steps = f.steps[1:]
	return step()
}

func exits(err error) func() (<-chan error, error) {
	return func() (<-chan error, error) {
		ch := make(chan error, 1)
		ch <- err
		return ch, nil
	}
}

func fails(err error) func() (<-chan error, error) {
	return func() (<-chan error, error) { return nil, err }
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newLauncher(cat *Catalog, f *fakeSpawn, systemd bool, c *clock) *Launcher {
	return New(cat, Options{
		Spawn: f.spawn, Systemd: func() bool { return systemd },
		Cooldown: 4 * time.Second, Grace: 20 * time.Millisecond, Now: c.now,
	})
}

func TestLaunchArgvPerInit(t *testing.T) {
	ctx := context.Background()
	bare := []string{"notes-app", "--x"}
	scoped := []string{"systemd-run", "--user", "--scope", "--collect", "--", "notes-app", "--x"}
	cases := []struct {
		name    string
		systemd bool
		steps   []func() (<-chan error, error)
		calls   [][]string
		scope   string
		reason  string
		err     error
	}{
		{"systemd, app keeps running", true, nil, [][]string{scoped}, ScopeSystemd, "", nil},
		{"systemd, app hands off and exits clean", true, []func() (<-chan error, error){exits(nil)}, [][]string{scoped}, ScopeSystemd, "", nil},
		{"systemd refuses the scope", true, []func() (<-chan error, error){exits(exitErr(1))}, [][]string{scoped, bare}, ScopeFallback, "scope refused", nil},
		{"systemd-run missing", true, []func() (<-chan error, error){fails(errors.New("executable file not found"))}, [][]string{scoped, bare}, ScopeFallback, "scope unavailable", nil},
		{"no user manager", false, nil, [][]string{bare}, ScopeFallback, "no systemd user manager", nil},
		{"app dies at once in its scope", true, []func() (<-chan error, error){exits(exitErr(127))}, [][]string{scoped}, ScopeSystemd, "", ErrExited},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeSpawn{steps: c.steps}
			l := newLauncher(browserCatalog(t), f, c.systemd, &clock{time.Unix(1000, 0)})
			rec, err := l.Launch(ctx, "Lindqvist Notes")
			if !errors.Is(err, c.err) || (c.err == nil && err != nil) {
				t.Fatalf("err = %v, want %v", err, c.err)
			}
			if !slices.EqualFunc(f.calls, c.calls, slices.Equal) {
				t.Fatalf("spawned %q, want %q", f.calls, c.calls)
			}
			if rec.Scope != c.scope || rec.Reason != c.reason {
				t.Fatalf("record scope=%q reason=%q, want %q %q", rec.Scope, rec.Reason, c.scope, c.reason)
			}
			if err == nil && !slices.Equal(rec.Argv, c.calls[len(c.calls)-1]) {
				t.Fatalf("record argv %q is not what started", rec.Argv)
			}
		})
	}
}

func TestLaunchCooldownIsDistinctFromFailure(t *testing.T) {
	ctx := context.Background()
	f := &fakeSpawn{}
	c := &clock{time.Unix(1000, 0)}
	l := newLauncher(browserCatalog(t), f, false, c)

	if _, err := l.Launch(ctx, "Lindqvist Notes"); err != nil {
		t.Fatal(err)
	}
	c.t = c.t.Add(3999 * time.Millisecond)
	_, err := l.Launch(ctx, "Lindqvist Notes")
	if !errors.Is(err, ErrCooldown) {
		t.Fatalf("a second launch inside 4 s: %v, want ErrCooldown", err)
	}
	if errors.Is(err, ErrExited) || errors.Is(err, ErrNotFound) {
		t.Fatal("a cooldown refusal must not read as a failure")
	}
	if len(f.calls) != 1 {
		t.Fatalf("a cooldown refusal started something: %q", f.calls)
	}
	for range 42 { // the rest of a 43-times repeated utterance
		if _, err := l.Launch(ctx, "Lindqvist Notes"); !errors.Is(err, ErrCooldown) {
			t.Fatalf("repeat: %v", err)
		}
	}
	if len(f.calls) != 1 {
		t.Fatalf("43 repeats launched %d times", len(f.calls))
	}
	if _, err := l.OpenURL(ctx, "web.desktop", "about:blank"); err != nil {
		t.Fatalf("another app is not under this app's cooldown: %v", err)
	}
	c.t = c.t.Add(time.Millisecond)
	if _, err := l.Launch(ctx, "Lindqvist Notes"); err != nil {
		t.Fatalf("after the cooldown: %v", err)
	}
	if _, err := l.Launch(ctx, "No Such App"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown app: %v", err)
	}
}

func TestOpenURLRefusesBadAddressesWithoutStarting(t *testing.T) {
	ctx := context.Background()
	f := &fakeSpawn{}
	l := newLauncher(browserCatalog(t), f, false, &clock{time.Unix(1000, 0)})
	for _, u := range []string{"http://example.org", "file:///etc/passwd", "-x", "https://", "about:config"} {
		if _, err := l.OpenURL(ctx, "web.desktop", u); !errors.Is(err, ErrBadURL) {
			t.Errorf("%q: %v, want ErrBadURL", u, err)
		}
	}
	if _, err := l.OpenURL(ctx, "notes.desktop", "about:blank"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a non-browser must not take an address: %v", err)
	}
	if len(f.calls) != 0 {
		t.Fatalf("started %q", f.calls)
	}
	rec, err := l.OpenURL(ctx, "web.desktop", "about:blank")
	if err != nil || !slices.Equal(rec.Argv, []string{"webb", "--profile", "main", "about:blank"}) {
		t.Fatalf("new tab argv %q, %v", rec.Argv, err)
	}
}

func TestDirs(t *testing.T) {
	got := Dirs([]string{"HOME=/home/u", "XDG_DATA_DIRS=/usr/share:relative:/usr/share:/opt/share"})
	want := []string{"/home/u/.local/share/applications", "/usr/share/applications", "/opt/share/applications"}
	if !slices.Equal(got, want) {
		t.Fatalf("Dirs = %q, want %q", got, want)
	}
	got = Dirs([]string{"XDG_DATA_HOME=/d"})
	want = []string{"/d/applications", "/usr/local/share/applications", "/usr/share/applications"}
	if !slices.Equal(got, want) {
		t.Fatalf("Dirs = %q, want %q", got, want)
	}
}
