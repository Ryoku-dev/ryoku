package skills

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// Root is the folder the sets lie in on disk, written out the first time it
// is asked for: whole in a temporary folder and then renamed to the hash of
// what it holds, so a reader never sees half a tree and a newer Rashin
// writes a new one. Folders an older Rashin wrote are removed.
func (l *Library) Root() (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.root != "" {
		if _, err := os.Stat(l.root); err == nil {
			return l.root, nil
		}
	}
	hash, err := contentHash(l.files)
	if err != nil {
		return "", err
	}
	root := filepath.Join(l.dir, hash)
	if _, err := os.Stat(root); errors.Is(err, fs.ErrNotExist) {
		if err := l.writeOut(root); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	l.prune(hash)
	l.root = root
	return root, nil
}

// contentHash names a tree by every path and every byte in it.
func contentHash(files fs.FS) (string, error) {
	sum := sha256.New()
	err := fs.WalkDir(files, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		body, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		sum.Write([]byte(name))
		sum.Write([]byte{0})
		sum.Write(body)
		sum.Write([]byte{0})
		return nil
	})
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil))[:16], nil
}

// writeOut writes the tree to root through a temporary folder beside it.
func (l *Library) writeOut(root string) error {
	if err := os.MkdirAll(l.dir, 0o700); err != nil {
		return err
	}
	temporary, err := os.MkdirTemp(l.dir, ".writing-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	if err := os.Chmod(temporary, 0o755); err != nil {
		return err
	}
	err = fs.WalkDir(l.files, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil || name == "." {
			return err
		}
		target := filepath.Join(temporary, filepath.FromSlash(name))
		if entry.IsDir() {
			return os.Mkdir(target, 0o755)
		}
		body, err := fs.ReadFile(l.files, name)
		if err != nil {
			return err
		}
		return os.WriteFile(target, body, fileMode(body))
	})
	if err != nil {
		return err
	}
	err = os.Rename(temporary, root)
	if err != nil {
		// another writer may have finished the same tree first
		if _, statErr := os.Stat(root); statErr == nil {
			return nil
		}
	}
	return err
}

// fileMode is how a written file may be used: the program embedding the
// tree keeps no modes, so a script is told by its #! line.
func fileMode(body []byte) os.FileMode {
	if bytes.HasPrefix(body, []byte("#!")) {
		return 0o755
	}
	return 0o644
}

// prune removes what else lies in the skills folder: trees an older Rashin
// wrote and any write that never finished.
func (l *Library) prune(keep string) {
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.Name() != keep {
			_ = os.RemoveAll(filepath.Join(l.dir, entry.Name()))
		}
	}
}
