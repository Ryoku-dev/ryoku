package apprules

import (
	"bytes"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"unicode"
)

// File is what the executor observed of one search result: its path after
// every link was resolved, its mode, and its first bytes (a few hundred are
// enough). Nothing here opens it.
type File struct {
	Path string
	Mode fs.FileMode
	Head []byte
}

// Why a file is not shown.
var (
	ErrOutside = errors.New("the path is outside the chosen folders")
	ErrPath    = errors.New("the path is not a plain absolute path")
	ErrKind    = errors.New("the path is not a file or folder")
	ErrQuery   = errors.New("the search words are not one plain line")
)

// MaxQueryRunes bounds one file search.
const MaxQueryRunes = 80

// runnable are name endings of programs, scripts, installers and launchers.
var runnable = []string{
	".desktop", ".appimage", ".sh", ".bash", ".zsh", ".fish", ".csh", ".ksh", ".run", ".bin", ".out",
	".py", ".pyw", ".pl", ".rb", ".php", ".lua", ".js", ".mjs", ".jar", ".exe", ".msi", ".bat", ".cmd",
	".com", ".ps1", ".vbs", ".deb", ".rpm", ".apk", ".flatpak", ".flatpakref", ".snap", ".command",
	".x86_64", ".elf", ".so", ".ko",
}

// Executable says whether f is something that runs: it carries an execute
// bit, starts with "#!" or an ELF header, or its name ends like a program, a
// script, an installer or a launcher. A folder is not.
func Executable(f File) bool {
	if f.Mode.IsDir() {
		return false
	}
	if f.Mode.Perm()&0o111 != 0 {
		return true
	}
	if bytes.HasPrefix(f.Head, []byte("#!")) || bytes.HasPrefix(f.Head, []byte("\x7fELF")) {
		return true
	}
	low := strings.ToLower(filepath.Base(f.Path))
	for _, ext := range runnable {
		if strings.HasSuffix(low, ext) {
			return true
		}
	}
	return false
}

// Within says whether path lies in one of roots. Both must be clean
// absolute paths; a path that is not is never within.
func Within(path string, roots []string) bool {
	if !plain(path) {
		return false
	}
	for _, root := range roots {
		if !plain(root) {
			continue
		}
		if path == root || root == "/" || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

// plain says whether p is a clean absolute path with no parent steps.
func plain(p string) bool {
	return p != "" && filepath.IsAbs(p) && filepath.Clean(p) == p && !strings.ContainsRune(p, 0) &&
		!strings.Contains("/"+p+"/", "/../")
}

// Show says which folder to open in the file manager for f: the folder f
// sits in, or f itself when it is a folder. Whatever f is, a document, a
// program or a script, that folder is all Hands ever opens: nothing found
// by a search is run or opened by Hands. A path outside roots, one that is
// not plain, and anything but a regular file or a folder is refused.
func Show(f File, roots []string) (string, error) {
	if !plain(f.Path) {
		return "", ErrPath
	}
	if !Within(f.Path, roots) {
		return "", ErrOutside
	}
	switch {
	case f.Mode.IsDir():
		return f.Path, nil
	case f.Mode.IsRegular():
		dir := filepath.Dir(f.Path)
		if !Within(dir, roots) {
			// a chosen folder that is itself a file's path: show nothing above it
			return "", ErrOutside
		}
		return dir, nil
	}
	return "", ErrKind
}

// Query checks the words of one file search: one line of at most
// MaxQueryRunes, with no path separator, no parent step and no control
// character, so a search names files and never a place.
func Query(q string) error {
	q = strings.TrimSpace(q)
	if q == "" || len([]rune(q)) > MaxQueryRunes || q == "." || q == ".." ||
		strings.ContainsAny(q, "/\\") || strings.Contains(q, "..") {
		return ErrQuery
	}
	for _, r := range q {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return ErrQuery
		}
	}
	return nil
}
