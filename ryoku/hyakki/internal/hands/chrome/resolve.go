package chrome

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// MinBrowserBytes is the smallest file taken for a browser itself. Every
// Chromium-family browser program is far larger; a launcher, a script or a
// shim is far smaller, and is never started.
const MinBrowserBytes = 8 << 20

// shells start other commands; none is ever the browser.
var shells = []string{
	"sh", "bash", "dash", "zsh", "fish", "ksh", "mksh", "tcsh", "csh", "env", "busybox", "toybox",
	"sudo", "doas", "pkexec", "systemd-run", "nohup", "setsid", "xdg-open", "gtk-launch", "firejail", "bwrap",
}

// sandboxes cannot be handed the pipe's descriptors.
var sandboxes = []string{"flatpak", "snap"}

var sandboxDirs = []string{"/snap/", "/var/lib/snapd/", "/var/lib/flatpak/", "/.local/share/flatpak/"}

// realPrograms names, for each launcher's file name, where its real
// browser program lives on the distributions that ship one.
var realPrograms = map[string][]string{
	"chromium":               {"/usr/lib/chromium/chromium", "/usr/lib64/chromium/chromium", "/usr/lib/chromium-browser/chromium-browser", "/usr/lib64/chromium-browser/chromium-browser"},
	"chromium-browser":       {"/usr/lib64/chromium-browser/chromium-browser", "/usr/lib/chromium-browser/chromium-browser", "/usr/lib/chromium/chromium", "/usr/lib64/chromium/chromium"},
	"google-chrome":          {"/opt/google/chrome/chrome"},
	"google-chrome-stable":   {"/opt/google/chrome/chrome"},
	"google-chrome-beta":     {"/opt/google/chrome-beta/chrome"},
	"google-chrome-unstable": {"/opt/google/chrome-unstable/chrome"},
	"brave":                  {"/opt/brave-bin/brave", "/opt/brave.com/brave/brave", "/usr/lib/brave-bin/brave"},
	"brave-browser":          {"/opt/brave.com/brave/brave", "/opt/brave-bin/brave", "/usr/lib/brave-bin/brave"},
	"brave-browser-stable":   {"/opt/brave.com/brave/brave"},
	"vivaldi":                {"/opt/vivaldi/vivaldi-bin"},
	"vivaldi-stable":         {"/opt/vivaldi/vivaldi-bin"},
	"microsoft-edge":         {"/opt/microsoft/msedge/msedge"},
	"microsoft-edge-stable":  {"/opt/microsoft/msedge/msedge"},
	"thorium-browser":        {"/opt/chromium.org/thorium/thorium"},
}

// realBinary finds the browser program prog stands for: prog itself when
// it is one, else the real program its launcher would start.
func (r resolver) realBinary(prog string, env []string) (string, error) {
	if err := refuseName(filepath.Base(prog)); err != nil {
		return "", err
	}
	path, err := r.find(prog, env)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrExec, err)
	}
	for _, p := range []string{path, resolved} {
		if err := refuseName(filepath.Base(p)); err != nil {
			return "", err
		}
		if sandboxed(p) {
			return "", ErrSandboxed
		}
	}
	if r.browser(resolved) {
		return resolved, nil
	}
	var names []string
	for _, p := range []string{prog, path, resolved} {
		if n := filepath.Base(p); !slices.Contains(names, n) {
			names = append(names, n)
		}
	}
	for _, n := range names {
		for _, c := range realPrograms[n] {
			real, err := filepath.EvalSymlinks(r.root + c)
			if err == nil && !sandboxed(real) && refuseName(filepath.Base(real)) == nil && r.browser(real) {
				return real, nil
			}
		}
	}
	return "", fmt.Errorf("%w: %s", ErrWrapper, prog)
}

func refuseName(base string) error {
	switch {
	case slices.Contains(sandboxes, base):
		return ErrSandboxed
	case slices.Contains(shells, base):
		return ErrShell
	}
	return nil
}

func sandboxed(p string) bool {
	for _, d := range sandboxDirs {
		if strings.Contains(p, d) {
			return true
		}
	}
	return false
}

// find locates prog: an absolute path as given, a bare name on PATH.
func (r resolver) find(prog string, env []string) (string, error) {
	if strings.Contains(prog, "/") {
		if !filepath.IsAbs(prog) || filepath.Clean(prog) != prog {
			return "", ErrExec
		}
		if !executable(r.root + prog) {
			return "", fmt.Errorf("%w: %s is not a program", ErrExec, prog)
		}
		return r.root + prog, nil
	}
	pathVar := r.path
	if pathVar == "" {
		if env == nil {
			env = os.Environ()
		}
		for _, kv := range env {
			if v, ok := strings.CutPrefix(kv, "PATH="); ok {
				pathVar = v
			}
		}
	}
	for _, dir := range filepath.SplitList(pathVar) {
		if !filepath.IsAbs(dir) {
			continue
		}
		if p := filepath.Join(r.root+dir, prog); executable(p) {
			return p, nil
		}
	}
	return "", fmt.Errorf("%w: %s not found", ErrExec, prog)
}

func executable(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular() && fi.Mode().Perm()&0o111 != 0
}

// browser says whether p is a browser program itself: a large ELF file.
func (r resolver) browser(p string) bool {
	fi, err := os.Stat(p)
	if err != nil || !fi.Mode().IsRegular() || fi.Mode().Perm()&0o111 == 0 {
		return false
	}
	min := r.min
	if min == 0 {
		min = MinBrowserBytes
	}
	if fi.Size() < min {
		return false
	}
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 4)
	if _, err := f.Read(head); err != nil {
		return false
	}
	return bytes.Equal(head, []byte("\x7fELF"))
}
