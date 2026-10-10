package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	wm "ryoku-wm"
)

type niriImportMetadata struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Supported        bool   `json:"supported"`
	DefaultSourceDir string `json:"defaultSourceDir"`
	Entry            string `json:"entry"`
}

type niriImportItem struct {
	Kind       string `json:"kind"`
	Raw        string `json:"raw"`
	Combo      string `json:"combo,omitempty"`
	Dispatcher string `json:"dispatcher,omitempty"`
	Ingestable bool   `json:"ingestable"`
}

type niriImportLoss struct {
	Raw    string `json:"raw"`
	Reason string `json:"reason"`
}

type niriImportConflict struct {
	Combo string `json:"combo"`
	Norm  string `json:"norm"`
	Ryoku struct {
		Action string `json:"action"`
		Desc   string `json:"desc"`
	} `json:"ryoku"`
	Mine struct {
		Raw  string `json:"raw"`
		Desc string `json:"desc"`
	} `json:"mine"`
	Kind string `json:"kind"`
}

type niriImportBind struct {
	Norm           string         `json:"norm"`
	Combo          string         `json:"combo"`
	Store          map[string]any `json:"store,omitempty"`
	RebindFrom     string         `json:"rebindFrom,omitempty"`
	Conflict       bool           `json:"conflict,omitempty"`
	ShadowsShipped bool           `json:"shadowsShipped,omitempty"`
}

type niriImportScan struct {
	Provider         string               `json:"provider"`
	Name             string               `json:"name"`
	Path             string               `json:"path"`
	DefaultSourceDir string               `json:"defaultSourceDir"`
	Items            []niriImportItem     `json:"items"`
	Conflicts        []niriImportConflict `json:"conflicts"`
	Losses           []niriImportLoss     `json:"losses"`
	Patch            map[string]any       `json:"patch"`
	Binds            []niriImportBind     `json:"binds"`
	Preserved        string               `json:"preserved,omitempty"`
	PreservePath     string               `json:"preservePath,omitempty"`
	SourceDirs       []string             `json:"sourceDirs,omitempty"`
}

func niriImporterMetadata() niriImportMetadata {
	return niriImportMetadata{
		ID:               wm.ProviderNiri,
		Name:             "niri KDL",
		Supported:        true,
		DefaultSourceDir: wm.ConfigDir(wm.ProviderNiri),
		Entry:            "config.kdl",
	}
}

func runImportVerb(args []string) error {
	if len(args) == 0 || args[0] == "metadata" {
		return json.NewEncoder(stdout).Encode(niriImporterMetadata())
	}
	if args[0] != "scan" || len(args) < 2 {
		return fmt.Errorf("import needs metadata or scan <source>")
	}
	scan, err := scanNiriImport(args[1])
	if err != nil {
		return err
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(scan)
}

func scanNiriImport(source string) (niriImportScan, error) {
	entry, root, err := findNiriImportEntry(source)
	if err != nil {
		return niriImportScan{}, err
	}
	nodes, err := readNiriImportTree(entry, root)
	if err != nil {
		return niriImportScan{}, err
	}
	scan := mapNiriImport(nodes)
	meta := niriImporterMetadata()
	scan.Provider = meta.ID
	scan.Name = meta.Name
	scan.Path = entry
	scan.DefaultSourceDir = meta.DefaultSourceDir
	scan.PreservePath = filepath.ToSlash(filepath.Join(meta.DefaultSourceDir, "user.kdl"))
	scan.SourceDirs = []string{filepath.Base(root)}
	if abs, absErr := filepath.Abs(source); absErr == nil && filepath.Clean(abs) == filepath.Clean(root) {
		scan.SourceDirs = []string{"."}
	}
	return scan, nil
}

func findNiriImportEntry(source string) (entry, root string, err error) {
	abs, err := filepath.Abs(source)
	if err != nil {
		return "", "", err
	}
	if st, statErr := os.Stat(abs); statErr == nil && !st.IsDir() {
		if filepath.Base(abs) != "config.kdl" {
			return "", "", fmt.Errorf("niri import source must be config.kdl or a config directory")
		}
		return abs, filepath.Dir(abs), nil
	}
	meta := niriImporterMetadata()
	candidates := []string{
		filepath.Join(abs, meta.Entry),
		filepath.Join(abs, meta.DefaultSourceDir, meta.Entry),
		filepath.Join(abs, ".config", meta.DefaultSourceDir, meta.Entry),
	}
	for _, candidate := range candidates {
		if st, statErr := os.Stat(candidate); statErr == nil && !st.IsDir() {
			return candidate, filepath.Dir(candidate), nil
		}
	}
	return "", "", fmt.Errorf("no %s found under %s", meta.Entry, abs)
}

func readNiriImportTree(entry, root string) ([]*kdlNode, error) {
	seen := map[string]bool{}
	var readOne func(string) ([]*kdlNode, error)
	readOne = func(path string) ([]*kdlNode, error) {
		path, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		path = filepath.Clean(path)
		if seen[path] {
			return nil, nil
		}
		seen[path] = true
		body, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read include %s: %w", path, err)
		}
		nodes, err := parseKDL(path, string(body))
		if err != nil {
			return nil, err
		}
		var out []*kdlNode
		for _, node := range nodes {
			if node.Name != "include" {
				out = append(out, node)
				continue
			}
			arg, ok := node.Arg(0)
			if !ok || strings.TrimSpace(arg.String()) == "" {
				return nil, fmt.Errorf("%s:%d: include needs a path", path, node.Line)
			}
			paths, err := resolveNiriIncludes(filepath.Dir(path), arg.String())
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", path, node.Line, err)
			}
			for _, included := range paths {
				more, err := readOne(included)
				if err != nil {
					return nil, err
				}
				out = append(out, more...)
			}
		}
		return out, nil
	}

	nodes, err := readOne(entry)
	if err != nil {
		return nil, err
	}
	var extras []string
	for _, dir := range []string{"config.d", "cfg"} {
		base := filepath.Join(root, dir)
		_ = filepath.WalkDir(base, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".kdl") {
				return nil
			}
			abs, _ := filepath.Abs(path)
			if !seen[filepath.Clean(abs)] {
				extras = append(extras, path)
			}
			return nil
		})
	}
	sort.Strings(extras)
	for _, extra := range extras {
		more, err := readOne(extra)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, more...)
	}
	return nodes, nil
}

func resolveNiriIncludes(base, value string) ([]string, error) {
	value = os.ExpandEnv(value)
	if value == "~" || strings.HasPrefix(value, "~/") {
		home, _ := os.UserHomeDir()
		value = filepath.Join(home, strings.TrimPrefix(value, "~/"))
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(base, value)
	}
	if strings.ContainsAny(value, "*?[") {
		matches, err := filepath.Glob(value)
		if err != nil {
			return nil, err
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("include %q matched no files", value)
		}
		sort.Strings(matches)
		return matches, nil
	}
	if st, err := os.Stat(value); err != nil || st.IsDir() {
		if err == nil {
			err = fmt.Errorf("is a directory")
		}
		return nil, fmt.Errorf("include %q: %w", value, err)
	}
	return []string{value}, nil
}
