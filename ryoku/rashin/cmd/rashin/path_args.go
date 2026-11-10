package main

import "path/filepath"

func absoluteUserPath(path string) (string, error) {
	return filepath.Abs(path)
}
