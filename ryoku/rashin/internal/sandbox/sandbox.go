// Package sandbox runs a program inside a bubblewrap sandbox: the host
// filesystem read-only except the paths the sandbox is given, its own PID and
// IPC namespaces, and an empty private runtime directory that holds only the
// sockets placed in it. Team agents and the programs they open run there, so
// they reach the daemon only through their own socket.
package sandbox

import (
	"cmp"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	i18n "ryoku-i18n"
)

// Bind places a host path inside the sandbox.
type Bind struct{ Source, Target string }

// Spec is one sandbox.
type Spec struct {
	// Folder is the working folder, read-write, and the start directory.
	Folder string
	// Writable are more read-write paths, directories or files, that already
	// exist on the host.
	Writable []string
	// Hidden are host paths the sandbox sees as an empty directory or an
	// empty file. Missing paths are skipped.
	Hidden []string
	// ReadOnly are host paths shown read-only again over the private /tmp
	// and /dev/shm or a Hidden directory, such as the directory of a daemon
	// run from /tmp whose own command line the sandbox must still run.
	// Missing paths are skipped.
	ReadOnly []string
	// RuntimeDir is the user's XDG_RUNTIME_DIR, replaced inside by an empty
	// private one (0700).
	RuntimeDir string
	// Binds are sockets or directories placed inside, read-write. A target
	// may sit inside RuntimeDir or a Hidden directory.
	Binds []Bind
	// Devices also binds /dev/dri when present, for programs on a screen.
	Devices bool
	// Env is set inside.
	Env map[string]string
	// ClearEnv starts from an empty environment, so only Env is set inside.
	ClearEnv bool
	// Unset is removed inside, and wins over Env.
	Unset []string
}

// mount is one filesystem step, applied at target inside the sandbox.
type mount struct {
	target string
	args   []string
}

// baseArgs is the sandbox every Spec starts from: the host filesystem
// read-only, fresh /proc and /dev, private /tmp and /dev/shm, its own PID and
// IPC namespaces and terminal session. The network stays shared.
func baseArgs() []string {
	return []string{
		"--ro-bind", "/", "/",
		"--proc", "/proc",
		"--dev", "/dev",
		"--tmpfs", "/tmp",
		"--tmpfs", "/dev/shm",
		"--unshare-pid",
		"--unshare-ipc",
		"--new-session",
		"--die-with-parent",
	}
}

// Command returns the full argv that runs argv inside the sandbox. Every path
// in spec must be absolute; Folder, Writable and the Bind sources must exist.
func Command(spec Spec, argv []string) ([]string, error) {
	if len(argv) == 0 {
		return nil, errors.New("sandbox: no command to run")
	}
	mounts, err := spec.mounts()
	if err != nil {
		return nil, err
	}
	args := append([]string{"bwrap"}, baseArgs()...)
	if spec.Devices {
		args = append(args, "--dev-bind-try", "/dev/dri", "/dev/dri")
	}
	for _, m := range mounts {
		args = append(args, m.args...)
	}
	if spec.ClearEnv {
		args = append(args, "--clearenv")
	}
	for _, key := range slices.Sorted(maps.Keys(spec.Env)) {
		args = append(args, "--setenv", key, spec.Env[key])
	}
	for _, key := range spec.Unset {
		args = append(args, "--unsetenv", key)
	}
	args = append(args, "--chdir", filepath.Clean(spec.Folder), "--")
	return append(args, argv...), nil
}

// mounts lists the spec's filesystem steps, a parent before anything inside
// it, so the most specific path wins: a Folder inside a Hidden directory is
// visible again, and a Hidden directory inside a Writable one stays hidden.
// Steps on the same path keep the order Hidden, ReadOnly, RuntimeDir, Folder,
// Writable, Binds, so the private RuntimeDir covers a ReadOnly path equal to
// it.
func (spec Spec) mounts() ([]mount, error) {
	folder, err := existing(spec.Folder)
	if err != nil {
		return nil, err
	}
	runtimeDir, err := absolute(spec.RuntimeDir)
	if err != nil {
		return nil, err
	}
	binds := make([]Bind, 0, len(spec.Binds))
	for _, bind := range spec.Binds {
		source, err := existing(bind.Source)
		if err != nil {
			return nil, err
		}
		target, err := absolute(bind.Target)
		if err != nil {
			return nil, err
		}
		binds = append(binds, Bind{Source: source, Target: target})
	}

	var mounts []mount
	for _, path := range spec.Hidden {
		path, info, err := optional(path)
		if err != nil {
			return nil, err
		}
		if info == nil {
			continue
		}
		mounts = append(mounts, hide(path, info.IsDir()))
		// A bind that places the hidden path somewhere else carries it along.
		for _, bind := range binds {
			if bind.Source != bind.Target && inside(path, bind.Source) {
				moved := filepath.Join(bind.Target, strings.TrimPrefix(path, bind.Source))
				mounts = append(mounts, hide(moved, info.IsDir()))
			}
		}
	}
	for _, path := range spec.ReadOnly {
		path, info, err := optional(path)
		if err != nil {
			return nil, err
		}
		if info != nil {
			mounts = append(mounts, mount{path, []string{"--ro-bind", path, path}})
		}
	}
	mounts = append(mounts, mount{runtimeDir, []string{"--perms", "0700", "--tmpfs", runtimeDir}})
	mounts = append(mounts, bindMount(folder, folder))
	for _, path := range spec.Writable {
		path, err := existing(path)
		if err != nil {
			return nil, err
		}
		mounts = append(mounts, bindMount(path, path))
	}
	for _, bind := range binds {
		mounts = append(mounts, bindMount(bind.Source, bind.Target))
	}
	slices.SortStableFunc(mounts, func(a, b mount) int {
		return cmp.Compare(depth(a.target), depth(b.target))
	})
	return mounts, nil
}

// hide covers path with an empty directory, or a file with /dev/null. The
// device bind is what lets /dev/null open there: a plain bind is nodev.
func hide(path string, dir bool) mount {
	if dir {
		return mount{path, []string{"--tmpfs", path}}
	}
	return mount{path, []string{"--dev-bind", "/dev/null", path}}
}

// bindMount places host path source at target, read-write.
func bindMount(source, target string) mount {
	return mount{target, []string{"--bind", source, target}}
}

// absolute cleans path and refuses a relative one.
func absolute(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", errors.New(i18n.Tf("The sandbox path %q is not absolute.", path))
	}
	return filepath.Clean(path), nil
}

// existing is absolute for a path that must already exist on the host.
func existing(path string) (string, error) {
	path, err := absolute(path)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err != nil {
		return "", errors.New(i18n.Tf("The sandbox needs %s, which does not exist.", path))
	}
	return path, nil
}

// optional is absolute for a path that may be missing on the host, which
// gives a nil info.
func optional(path string) (string, fs.FileInfo, error) {
	path, err := absolute(path)
	if err != nil {
		return "", nil, err
	}
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return path, nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	return path, info, nil
}

// inside reports whether path lies strictly inside dir.
func inside(path, dir string) bool {
	return path != dir && strings.HasPrefix(path, strings.TrimSuffix(dir, "/")+"/")
}

// depth is the number of components in a clean absolute path.
func depth(path string) int {
	return strings.Count(path, "/")
}
