package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
)

// The Caelestia shell style keeps its settings in one JSON file of nested objects
// that holds only what differs from the shell's own defaults, and it watches that
// file, so a save here reaches the running shell without a restart. The Hub shows
// every option it carries, so `get` lays the file over the defaults, and `save`
// writes back only the differences.

//go:embed caelestia_defaults.json
var caelestiaDefaults []byte

func caelestiaPath() string {
	return filepath.Join(ryokuConfigDir(), "caelestia", "shell.json")
}

// caelestiaPackage carries the Caelestia QML plugin (Caelestia.*, M3Shapes). It is
// optional: only the Caelestia bar style needs it, so Bar Studio installs it when
// that style is picked, the way the Rashin switch installs ryoku-rashin.
const caelestiaPackage = "ryoku-caelestia"

// caelestiaPluginInstalled reports whether the plugin is on the shell's QML import
// path: the package's system copy, or a source install's ~/.local/lib/qt6/qml.
func caelestiaPluginInstalled() bool {
	dirs := []string{"/usr/lib/qt6/qml"}
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".local", "lib", "qt6", "qml"))
	}
	for _, dir := range dirs {
		if _, err := os.Stat(filepath.Join(dir, "Caelestia", "qmldir")); err == nil {
			return true
		}
	}
	return false
}

func runCaelestia(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("caelestia needs get|defaults|save|status|install")
	}
	switch args[0] {
	case "status":
		return printJSON(map[string]bool{"installed": caelestiaPluginInstalled()})
	case "install":
		if caelestiaPluginInstalled() {
			return nil
		}
		if err := privileged("pkg", "install", caelestiaPackage); err != nil {
			return err
		}
		if !caelestiaPluginInstalled() {
			return fmt.Errorf("%s did not land", caelestiaPackage)
		}
		return nil
	}
	defaults, err := caelestiaDefaultsTree()
	if err != nil {
		return err
	}
	switch args[0] {
	case "defaults":
		return printJSON(map[string]any{"caelestia": defaults})
	case "get":
		file, err := readCaelestiaFile()
		if err != nil {
			return err
		}
		return printJSON(map[string]any{"caelestia": mergeTrees(defaults, file)})
	case "save":
		if len(args) < 2 {
			return fmt.Errorf("caelestia save needs a JSON argument")
		}
		var draft struct {
			Caelestia map[string]any `json:"caelestia"`
		}
		if err := json.Unmarshal([]byte(args[1]), &draft); err != nil {
			return fmt.Errorf("caelestia save: %w", err)
		}
		file, err := readCaelestiaFile()
		if err != nil {
			return err
		}
		if err := applyCaelestiaDraft(defaults, draft.Caelestia, file, ""); err != nil {
			return err
		}
		b, err := json.MarshalIndent(file, "", "  ")
		if err != nil {
			return err
		}
		return atomicWrite(caelestiaPath(), append(b, '\n'), 0o644)
	}
	return fmt.Errorf("caelestia needs get|defaults|save")
}

func caelestiaDefaultsTree() (map[string]any, error) {
	var tree map[string]any
	if err := json.Unmarshal(caelestiaDefaults, &tree); err != nil {
		return nil, fmt.Errorf("caelestia defaults: %w", err)
	}
	return tree, nil
}

// A missing file is an empty one; a file that does not parse is an error, so a save
// never overwrites settings it could not read.
func readCaelestiaFile() (map[string]any, error) {
	b, err := os.ReadFile(caelestiaPath())
	if os.IsNotExist(err) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	var tree map[string]any
	if err := json.Unmarshal(b, &tree); err != nil {
		return nil, fmt.Errorf("%s: %w", caelestiaPath(), err)
	}
	if tree == nil {
		tree = map[string]any{}
	}
	return tree, nil
}

func mergeTrees(base, over map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range over {
		bm, bok := out[k].(map[string]any)
		om, ook := v.(map[string]any)
		if bok && ook {
			out[k] = mergeTrees(bm, om)
		} else {
			out[k] = v
		}
	}
	return out
}

// applyCaelestiaDraft walks the defaults tree: each option the draft changes is set
// in file, each one back at its default is removed, and anything the defaults do not
// name (an option the Hub does not carry) is left as it was.
func applyCaelestiaDraft(defaults, draft, file map[string]any, path string) error {
	for key, def := range defaults {
		here := path + key
		got, present := draft[key]
		if !present {
			continue
		}
		if dm, ok := def.(map[string]any); ok {
			gm, ok := got.(map[string]any)
			if !ok {
				return fmt.Errorf("%s must be an object", here)
			}
			sub, ok := file[key].(map[string]any)
			if !ok {
				sub = map[string]any{}
			}
			if err := applyCaelestiaDraft(dm, gm, sub, here+"."); err != nil {
				return err
			}
			if len(sub) == 0 {
				delete(file, key)
			} else {
				file[key] = sub
			}
			continue
		}
		if reflect.TypeOf(got) != reflect.TypeOf(def) {
			return fmt.Errorf("%s must be %T, got %T", here, def, got)
		}
		if reflect.DeepEqual(got, def) {
			delete(file, key)
		} else {
			file[key] = got
		}
	}
	return nil
}
