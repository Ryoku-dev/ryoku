package host

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"ryoku-cli/internal/profile"
)

func stageProfilePayload(root string, manifest *profile.Manifest, source profile.Source) (string, error) {
	destination := profile.StagedPayloadDir(root, manifest.ID, manifest.Version)
	if stagedPayloadMatches(destination, source.Files) {
		return destination, nil
	}
	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}
	temporary, err := os.MkdirTemp(parent, ".payload-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temporary)
	if err := os.Chmod(temporary, 0o755); err != nil {
		return "", err
	}
	for _, file := range source.Files {
		rel, err := safeProfileRelative(file.Path)
		if err != nil {
			return "", err
		}
		from, err := regularFileBelow(source.Dir, rel)
		if err != nil {
			return "", err
		}
		to := filepath.Join(temporary, rel)
		if err := copyStagedFile(from, to); err != nil {
			return "", err
		}
		digest, err := hashFile(to)
		if err != nil || digest != file.SHA256 {
			return "", fmt.Errorf("staged payload hash mismatch for %s", file.Path)
		}
	}
	backup := destination + ".previous"
	if err := os.RemoveAll(backup); err != nil {
		return "", err
	}
	hadDestination := false
	if _, err := os.Lstat(destination); err == nil {
		hadDestination = true
		if err := os.Rename(destination, backup); err != nil {
			return "", err
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.Rename(temporary, destination); err != nil {
		if hadDestination {
			_ = os.Rename(backup, destination)
		}
		return "", err
	}
	if hadDestination {
		_ = os.RemoveAll(backup)
	}
	return destination, nil
}

func copyStagedFile(from, to string) error {
	info, err := os.Lstat(from)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("profile payload entry is not a regular file")
	}
	mode := os.FileMode(0o644)
	if info.Mode().Perm()&0o111 != 0 {
		mode = 0o755
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	fd, err := syscall.Open(from, syscall.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	source := os.NewFile(uintptr(fd), from)
	if source == nil {
		syscall.Close(fd)
		return fmt.Errorf("open %s", from)
	}
	defer source.Close()
	destination, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(destination, source)
	closeErr := destination.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func stagedPayloadMatches(root string, files []profile.FileHash) bool {
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	for _, file := range files {
		rel, err := safeProfileRelative(file.Path)
		if err != nil {
			return false
		}
		path, err := regularFileBelow(root, rel)
		if err != nil {
			return false
		}
		digest, err := hashFile(path)
		if err != nil || digest != file.SHA256 {
			return false
		}
	}
	return true
}
