package sandbox

import "path/filepath"

// Hides reports whether the host's own path cannot be read inside the
// sandbox: whether the most specific of the spec's steps that holds it,
// taken in the order mounts applies them, is one that covers it (a Hidden
// path, the private runtime dir, /tmp, /dev/shm, or a bind that places
// something else there) rather than one that shows it again (ReadOnly,
// Folder, Writable or a bind of itself). A Hidden path counts whether or not
// it exists yet, so a file that comes later is hidden all the same. Paths
// are compared as written, cleaned.
func (spec Spec) Hides(path string) bool {
	path = filepath.Clean(path)
	hidden, best := false, -1
	step := func(target string, hides bool) {
		if target == "" {
			return
		}
		target = filepath.Clean(target)
		if (target == path || inside(path, target)) && depth(target) >= best {
			hidden, best = hides, depth(target)
		}
	}
	for _, private := range []string{"/tmp", "/dev/shm"} {
		step(private, true)
	}
	for _, target := range spec.Hidden {
		step(target, true)
	}
	for _, target := range spec.ReadOnly {
		step(target, false)
	}
	step(spec.RuntimeDir, true)
	step(spec.Folder, false)
	for _, target := range spec.Writable {
		step(target, false)
	}
	for _, bind := range spec.Binds {
		step(bind.Target, filepath.Clean(bind.Source) != filepath.Clean(bind.Target))
	}
	return hidden
}
