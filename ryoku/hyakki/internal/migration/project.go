package migration

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"ryoku-rashin/internal/paths"
)

const projectMarkerContents = "{\"version\":1}\n"

// Project imports legacy rules and setup configuration once. The original
// repository files remain untouched, and a current namespace always wins.
func Project(project string) error {
	if !filepath.IsAbs(project) || filepath.Clean(project) != project {
		return fmt.Errorf("project must be a clean absolute folder")
	}
	canonical, err := filepath.EvalSymlinks(project)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(canonical)
	if err != nil {
		return err
	}
	defer root.Close()
	current, err := root.Lstat(".hyakki")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if current != nil && !current.IsDir() {
		return fmt.Errorf("project configuration %s must be a real directory", filepath.Join(project, ".hyakki"))
	}
	legacy, err := root.Lstat(".rashin")
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(canonical))
	marker := filepath.Join(paths.StateDir(), "migrations", "projects", hex.EncodeToString(sum[:])+".json")
	if raw, err := os.ReadFile(marker); err == nil {
		if string(raw) != projectMarkerContents {
			return fmt.Errorf("unrecognized project migration record %s", marker)
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if current != nil {
		return markProject(marker)
	}
	if !legacy.IsDir() {
		return fmt.Errorf("legacy project configuration %s must be a real directory", filepath.Join(project, ".rashin"))
	}
	old, err := root.OpenRoot(".rashin")
	if err != nil {
		return err
	}
	defer old.Close()
	files, err := projectFiles(old)
	if err != nil || len(files) == 0 {
		return err
	}

	stageName := ".hyakki-migration-" + rand.Text()
	if err := root.Mkdir(stageName, 0o700); err != nil {
		return err
	}
	defer root.RemoveAll(stageName)
	stage, err := root.OpenRoot(stageName)
	if err != nil {
		return err
	}
	for _, name := range files {
		if err := copyProjectFile(old, stage, name); err != nil {
			stage.Close()
			return err
		}
	}
	if err := stage.Close(); err != nil {
		return err
	}
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()
	// Publish the whole namespace at once, without replacing concurrent edits.
	err = unix.Renameat2(int(dir.Fd()), stageName, int(dir.Fd()), ".hyakki", unix.RENAME_NOREPLACE)
	if err != nil && !errors.Is(err, unix.EEXIST) {
		return err
	}
	if err != nil {
		info, readErr := root.Lstat(".hyakki")
		if readErr != nil {
			return readErr
		}
		if !info.IsDir() {
			return fmt.Errorf("project configuration changed during migration; original files were retained")
		}
	}
	if err := dir.Sync(); err != nil {
		return err
	}
	return markProject(marker)
}

func projectFiles(root *os.Root) ([]string, error) {
	var files []string
	if info, err := root.Lstat("setup"); err == nil {
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("legacy project setup must be a regular file")
		}
		files = append(files, "setup")
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	info, err := root.Lstat("rules")
	if errors.Is(err, os.ErrNotExist) {
		return files, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("legacy project rules must be a real directory")
	}
	dir, err := root.Open("rules")
	if err != nil {
		return nil, err
	}
	entries, err := dir.ReadDir(-1)
	dir.Close()
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".md") {
			files = append(files, filepath.Join("rules", entry.Name()))
		}
	}
	return files, nil
}

func copyProjectFile(source, destination *os.Root, name string) error {
	file, err := source.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	before, err := file.Stat()
	if err != nil {
		return err
	}
	if !before.Mode().IsRegular() {
		return fmt.Errorf("legacy project configuration %s must be a regular file", name)
	}
	if err := destination.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		return err
	}
	out, err := destination.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, before.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, file)
	syncErr := out.Sync()
	closeErr := out.Close()
	if err := errors.Join(copyErr, syncErr, closeErr); err != nil {
		return err
	}
	after, err := source.Lstat(name)
	if err != nil {
		return err
	}
	if !os.SameFile(before, after) || before.Size() != after.Size() || before.ModTime() != after.ModTime() {
		return fmt.Errorf("legacy project configuration %s changed during migration; retry after saving it", name)
	}
	return nil
}

func markProject(marker string) error {
	dir := filepath.Dir(marker)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".project-migration-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, writeErr := io.WriteString(file, projectMarkerContents)
	syncErr := file.Sync()
	closeErr := file.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), marker); err != nil {
		return err
	}
	parent, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer parent.Close()
	return parent.Sync()
}
