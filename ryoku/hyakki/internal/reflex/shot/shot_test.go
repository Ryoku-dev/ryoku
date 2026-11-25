package shot

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func finder(installed ...string) func(string) (string, error) {
	return func(name string) (string, error) {
		if slices.Contains(installed, name) {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
}

type rec struct{ runs, spawns [][]string }

func (r *rec) run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.runs = append(r.runs, append([]string{name}, args...))
	return nil, nil
}

func (r *rec) spawn(argv []string) (<-chan error, error) {
	r.spawns = append(r.spawns, slices.Clone(argv))
	return make(chan error), nil
}

func shooter(r *rec, dir string, installed ...string) *Shooter {
	return New(Options{Dir: dir, LookPath: finder(installed...), Run: r.run, Spawn: r.spawn,
		Now: func() time.Time { return time.Date(2026, 11, 23, 9, 5, 7, 0, time.UTC) }})
}

func TestTakeDelegatesToAnInstalledTool(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "Screenshots")

	r := &rec{}
	res, err := shooter(r, dir, picker, grabber).Take(ctx)
	if err != nil || !res.Picking || !slices.EqualFunc(r.spawns, [][]string{{picker}}, slices.Equal) || len(r.runs) != 0 {
		t.Fatalf("the desktop's capture tool comes first: %+v %v %q %q", res, err, r.spawns, r.runs)
	}

	r = &rec{}
	res, err = shooter(r, dir, grabber).Take(ctx)
	want := filepath.Join(dir, "screenshot-20261123-090507.png")
	if err != nil || res.Path != want || !slices.EqualFunc(r.runs, [][]string{{grabber, want}}, slices.Equal) {
		t.Fatalf("a whole-screen grab: %+v %v %q", res, err, r.runs)
	}
	if fi, err := os.Stat(dir); err != nil || fi.Mode().Perm() != 0o700 {
		t.Fatalf("the screenshots folder is created private: %v", err)
	}

	r = &rec{}
	s := shooter(r, dir)
	if s.Available() {
		t.Fatal("no tool installed must not read as available")
	}
	if _, err := s.Take(ctx); !errors.Is(err, ErrUnavailable) || len(r.runs)+len(r.spawns) != 0 {
		t.Fatalf("no tool: %v, ran %q %q", err, r.runs, r.spawns)
	}
	if _, err := shooter(&rec{}, "", grabber).Take(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("a grab with no folder: %v", err)
	}
}

func TestAllowedIsExact(t *testing.T) {
	s := shooter(&rec{}, "/home/u/Pictures/Screenshots")
	ok := [][]string{
		{picker},
		{grabber, "/home/u/Pictures/Screenshots/screenshot-20261123-090507.png"},
	}
	for _, a := range ok {
		if !s.Allowed(a[0], a[1:]...) {
			t.Errorf("refused %q", a)
		}
	}
	bad := [][]string{
		{picker, "--region"},
		{grabber},
		{grabber, "-g", "0,0 10x10", "/home/u/Pictures/Screenshots/screenshot-20261123-090507.png"},
		{grabber, "/home/u/Pictures/Screenshots/../screenshot-20261123-090507.png"},
		{grabber, "/home/u/.ssh/screenshot-20261123-090507.png"},
		{grabber, "/home/u/Pictures/Screenshots/other.png"},
		{grabber, "screenshot-20261123-090507.png"},
		{grabber, "-"},
		{"import", "-window", "root", "x.png"},
		{"sh", "-c", "grim"},
	}
	for _, a := range bad {
		if s.Allowed(a[0], a[1:]...) {
			t.Errorf("allowed %q", a)
		}
	}
}

func TestPicturesDir(t *testing.T) {
	home := t.TempDir()
	if got := PicturesDir([]string{"HOME=" + home}); got != filepath.Join(home, "Pictures") {
		t.Fatalf("default: %q", got)
	}
	cfg := filepath.Join(home, ".config")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "user-dirs.dirs"), []byte("# x\nXDG_PICTURES_DIR=\"$HOME/Bilder\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := PicturesDir([]string{"HOME=" + home}); got != filepath.Join(home, "Bilder") {
		t.Fatalf("user-dirs: %q", got)
	}
	if got := PicturesDir(nil); got != "" {
		t.Fatalf("no home: %q", got)
	}
}
