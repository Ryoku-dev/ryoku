package browser

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"ryoku-rashin/internal/reflex/launch"
)

func write(t *testing.T, p, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func catalog(t *testing.T) *launch.Catalog {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "a-web.desktop"), "[Desktop Entry]\nType=Application\nName=First Web\nExec=firstweb %u\nCategories=WebBrowser;\n")
	write(t, filepath.Join(dir, "dev.ryoku.OniBrowser.desktop"), "[Desktop Entry]\nType=Application\nName=Lab Browser\nExec=python3 /lab/tabwin.py --out /lab/out %u\nMimeType=x-scheme-handler/http;x-scheme-handler/https;\n")
	write(t, filepath.Join(dir, "notes.desktop"), "[Desktop Entry]\nType=Application\nName=Notes\nExec=notes %U\nMimeType=text/plain;\n")
	return launch.Scan(launch.ScanOptions{Dirs: []string{dir}, Max: 200})
}

func TestDefaultFollowsMimeappsThenCategory(t *testing.T) {
	cat := catalog(t)
	dir := t.TempDir()
	user := filepath.Join(dir, "user.list")
	system := filepath.Join(dir, "system.list")
	write(t, user, "[Added Associations]\nx-scheme-handler/https=a-web.desktop;\n[Default Applications]\nx-scheme-handler/https=missing.desktop;notes.desktop;dev.ryoku.OniBrowser.desktop;\n")
	write(t, system, "[Default Applications]\nx-scheme-handler/https=a-web.desktop;\n")

	if e, ok := Default(cat, []string{user, system}); !ok || e.ID != "dev.ryoku.OniBrowser.desktop" {
		t.Fatalf("the first catalogued browser in the user's defaults wins, skipping a missing or non-browser id: %+v", e)
	}
	if e, ok := Default(cat, []string{system, user}); !ok || e.ID != "a-web.desktop" {
		t.Fatalf("files are read most important first: %+v", e)
	}
	if e, ok := Default(cat, []string{filepath.Join(dir, "absent.list")}); !ok || e.ID != "a-web.desktop" {
		t.Fatalf("with no defaults, the first catalogued browser: %+v", e)
	}
	empty := launch.Scan(launch.ScanOptions{Dirs: []string{t.TempDir()}, Max: 200})
	if _, ok := Default(empty, nil); ok {
		t.Fatal("no browser in the catalog must say so")
	}
}

type recorder struct{ calls [][]string }

func (r *recorder) spawn(argv []string) (<-chan error, error) {
	r.calls = append(r.calls, slices.Clone(argv))
	return make(chan error), nil
}

func TestTabsGoThroughTheBrowsersOwnLauncher(t *testing.T) {
	ctx := context.Background()
	cat := catalog(t)
	r := &recorder{}
	now := time.Unix(1000, 0)
	l := launch.New(cat, launch.Options{Spawn: r.spawn, Systemd: func() bool { return false }, Now: func() time.Time { return now }})
	if _, err := New(l, "notes.desktop", ""); !errors.Is(err, ErrNoBrowser) {
		t.Fatalf("a non-browser entry: %v", err)
	}
	if _, err := New(l, "dev.ryoku.OniBrowser.desktop", "http://search.example/?q=%s"); err == nil {
		t.Fatal("a search address that is not https must be refused")
	}
	b, err := New(l, "dev.ryoku.OniBrowser.desktop", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.NewTab(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Open(ctx, "https://example.org/page"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Search(ctx, "salt and pepper"); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Open(ctx, "http://example.org/"); !errors.Is(err, launch.ErrBadURL) {
		t.Fatalf("plain http: %v", err)
	}
	if _, err := b.Search(ctx, "  "); !errors.Is(err, launch.ErrBadURL) {
		t.Fatalf("empty search: %v", err)
	}
	if _, err := b.NewTab(ctx); !errors.Is(err, launch.ErrCooldown) {
		t.Fatalf("a second new tab inside the cooldown: %v", err)
	}
	base := []string{"python3", "/lab/tabwin.py", "--out", "/lab/out"}
	want := [][]string{
		append(slices.Clone(base), "about:blank"),
		append(slices.Clone(base), "https://example.org/page"),
		append(slices.Clone(base), "https://duckduckgo.com/?q=salt+and+pepper"),
	}
	if !slices.EqualFunc(r.calls, want, slices.Equal) {
		t.Fatalf("started %q, want %q", r.calls, want)
	}
}

func TestMimeappsOrder(t *testing.T) {
	got := Mimeapps([]string{"HOME=/h", "XDG_CONFIG_DIRS=/etc/a:/etc/b", "XDG_DATA_DIRS=/usr/share"})
	want := []string{
		"/h/.config/mimeapps.list", "/etc/a/mimeapps.list", "/etc/b/mimeapps.list",
		"/h/.local/share/applications/mimeapps.list", "/usr/share/applications/mimeapps.list",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Mimeapps = %q, want %q", got, want)
	}
}
