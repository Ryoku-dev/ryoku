package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// fetcher reads symbol SVGs at the pinned commit, keeping each one in cache so later runs and -check work offline.
type fetcher struct {
	cache  string
	client *http.Client
}

func (f *fetcher) svg(name, file string) ([]byte, error) {
	path := filepath.Join(f.cache, file)
	if data, err := os.ReadFile(path); err == nil {
		return data, nil
	}
	url := upstream + "/" + commit + "/symbols/web/" + name + "/materialsymbolssharp/" + file
	response, err := f.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: %s", url, response.Status)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", url, err)
	}
	if err := store(path, data); err != nil {
		return nil, err
	}
	return data, nil
}

// store writes through a temporary file so an interrupted run never leaves a truncated SVG in the cache.
func store(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("cache %s: %w", path, err)
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		os.Remove(temp.Name())
		return fmt.Errorf("cache %s: %w", path, err)
	}
	if err := temp.Close(); err != nil {
		os.Remove(temp.Name())
		return fmt.Errorf("cache %s: %w", path, err)
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		os.Remove(temp.Name())
		return fmt.Errorf("cache %s: %w", path, err)
	}
	return nil
}
