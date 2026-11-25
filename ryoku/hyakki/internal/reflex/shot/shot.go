// Package shot hands a reflex's "take a screenshot" to a screenshot tool
// the user already has. Hyakki has no capture path of its own for this:
// when neither tool below is installed it says so and takes nothing.
//
//   - the desktop's own capture tool, opened for the user to pick an area;
//   - else a whole-screen grab written as one PNG into the screenshots
//     folder.
package shot

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"ryoku-rashin/internal/reflex/proc"
)

const (
	// picker is the desktop's interactive capture tool, started detached.
	picker = "ryoku-cmd-ryoshot"
	// grabber writes the whole screen to the file it is given.
	grabber = "grim"
)

// ErrUnavailable means no screenshot tool is installed.
var ErrUnavailable = errors.New("shot: no screenshot tool installed")

// Result is what a screenshot did.
type Result struct {
	// Picking is true when the capture tool opened for the user to pick an
	// area; it saves the shot itself.
	Picking bool
	// Path is the PNG written by a whole-screen grab.
	Path string
}

// Options configures a Shooter.
type Options struct {
	// Dir is the absolute screenshots folder a grab writes into, usually
	// PicturesDir(env)/Screenshots. Empty offers no grab.
	Dir string
	// LookPath finds an installed tool by name; nil finds none.
	LookPath func(string) (string, error)
	// Run runs the grab (proc.Exec(env)); Spawn starts the capture tool
	// detached (proc.Detached).
	Run   proc.Runner
	Spawn proc.Spawner
	// Now names the grab's file; default time.Now.
	Now func() time.Time
}

// Shooter takes screenshots through an installed tool.
type Shooter struct {
	dir      string
	lookPath func(string) (string, error)
	run      proc.Runner
	spawn    proc.Spawner
	now      func() time.Time
}

// New returns a Shooter. Every command is checked against s.Allowed.
func New(o Options) *Shooter {
	if o.Now == nil {
		o.Now = time.Now
	}
	s := &Shooter{dir: filepath.Clean(o.Dir), lookPath: o.LookPath, now: o.Now}
	s.run = proc.Guard(s.Allowed, o.Run)
	s.spawn = proc.GuardSpawn(s.Allowed, o.Spawn)
	return s
}

var shotName = regexp.MustCompile(`^screenshot-\d{8}-\d{6}\.png$`)

// Allowed reports whether name and args are one of this package's
// commands: the capture tool with no arguments, or a grab into exactly one
// new PNG named screenshot-YYYYMMDD-HHMMSS.png directly in the screenshots
// folder.
func (s *Shooter) Allowed(name string, args ...string) bool {
	switch name {
	case picker:
		return len(args) == 0
	case grabber:
		if len(args) != 1 || !filepath.IsAbs(s.dir) {
			return false
		}
		p := args[0]
		return filepath.IsAbs(p) && filepath.Clean(p) == p && filepath.Dir(p) == s.dir && shotName.MatchString(filepath.Base(p))
	}
	return false
}

// Available reports whether a screenshot tool is installed.
func (s *Shooter) Available() bool {
	return s.found(picker) || (s.found(grabber) && filepath.IsAbs(s.dir))
}

func (s *Shooter) found(tool string) bool {
	if s.lookPath == nil {
		return false
	}
	_, err := s.lookPath(tool)
	return err == nil
}

// Take opens the capture tool, or else grabs the whole screen into the
// screenshots folder, or answers ErrUnavailable.
func (s *Shooter) Take(ctx context.Context) (Result, error) {
	if s.found(picker) {
		if _, err := s.spawn([]string{picker}); err != nil {
			return Result{}, err
		}
		return Result{Picking: true}, nil
	}
	if !s.found(grabber) || !filepath.IsAbs(s.dir) {
		return Result{}, ErrUnavailable
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return Result{}, err
	}
	p := filepath.Join(s.dir, "screenshot-"+s.now().Format("20060102-150405")+".png")
	if _, err := os.Lstat(p); err == nil {
		return Result{}, fmt.Errorf("shot: %s exists", filepath.Base(p))
	}
	if _, err := s.run(ctx, grabber, p); err != nil {
		return Result{}, err
	}
	return Result{Path: p}, nil
}

// PicturesDir is the user's pictures folder for env: XDG_PICTURES_DIR from
// user-dirs.dirs when it names an absolute folder, else $HOME/Pictures.
// Empty when there is no home.
func PicturesDir(env []string) string {
	home := lookup(env, "HOME")
	config := lookup(env, "XDG_CONFIG_HOME")
	if config == "" && home != "" {
		config = filepath.Join(home, ".config")
	}
	if f, err := os.Open(filepath.Join(config, "user-dirs.dirs")); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(io.LimitReader(f, 64<<10))
		for sc.Scan() {
			v, ok := strings.CutPrefix(strings.TrimSpace(sc.Text()), "XDG_PICTURES_DIR=")
			if !ok {
				continue
			}
			v = strings.Trim(v, `"`)
			if rest, ok := strings.CutPrefix(v, "$HOME"); ok && home != "" {
				v = home + rest
			}
			if filepath.IsAbs(v) {
				return filepath.Clean(v)
			}
		}
	}
	if !filepath.IsAbs(home) {
		return ""
	}
	return filepath.Join(home, "Pictures")
}

func lookup(env []string, key string) string {
	for i := len(env) - 1; i >= 0; i-- {
		if k, v, ok := strings.Cut(env[i], "="); ok && k == key {
			return v
		}
	}
	return ""
}
