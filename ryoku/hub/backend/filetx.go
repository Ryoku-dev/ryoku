package main

import (
	"errors"
	"io/fs"
	"os"
)

// fileSnapshot holds files as they were before a multi-step write, so a failed
// step can put every one back byte for byte and the write leaves no trace
type fileSnapshot []savedFile

type savedFile struct {
	path   string
	data   []byte
	mode   fs.FileMode
	exists bool
}

// snapshotFiles reads each path; a missing one is recorded so restore removes it
func snapshotFiles(paths ...string) (fileSnapshot, error) {
	snap := make(fileSnapshot, 0, len(paths))
	for _, path := range paths {
		info, err := os.Stat(path)
		if errors.Is(err, fs.ErrNotExist) {
			snap = append(snap, savedFile{path: path})
			continue
		}
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		snap = append(snap, savedFile{path: path, data: data, mode: info.Mode().Perm(), exists: true})
	}
	return snap, nil
}

// restore puts every file back as snapshotted and removes the ones that were missing
func (s fileSnapshot) restore() error {
	var errs []error
	for _, f := range s {
		if !f.exists {
			errs = append(errs, removeIfPresent(f.path))
			continue
		}
		errs = append(errs, atomicWrite(f.path, f.data, f.mode))
	}
	return errors.Join(errs...)
}

func removeIfPresent(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
