package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const (
	launcherTransactionKey     = "ryoStoreLauncherTransaction"
	dockTransactionKey         = "ryoStoreDockTransaction"
	osdTransactionKey          = "ryoStoreOsdTransaction"
	notificationTransactionKey = "ryoStoreNotificationTransaction"
	sidebarTransactionKey      = "ryoStoreSidebarTransaction"
	barStyleTransactionKey     = "ryoStoreBarStyleTransaction"
)

// productSelection is the user setting that wears a product: shell.json names
// the bar style the desktop draws and the dock design every bar style shares,
// launcher.json the launcher Super+Space opens. Removing the worn product moves
// that setting to its fallback inside the removal's own transaction, so the
// desktop never keeps naming what is gone and a later reinstall does not put it
// back on by itself.
type productSelection struct {
	path func() string
	// object names the section that holds key, such as shell.json's dock; ""
	// when the setting sits at the root of the file.
	object   string
	key      string
	fallback string
	// marker is a root key the fallback write leaves in the same file, so
	// recovery reverts only its own fallback and preserves a newer user choice.
	marker string
}

func selectionFor(category string) (productSelection, bool) {
	switch category {
	case "subshells", "barstyles":
		return productSelection{path: defaultShellConfigPath, key: "barStyle", fallback: "orpheus", marker: barStyleTransactionKey}, true
	case "launchers":
		// An empty variant opens the launcher the shell ships as its default.
		return productSelection{path: defaultLauncherConfigPath, key: "variant", fallback: "", marker: launcherTransactionKey}, true
	case "docks":
		return productSelection{path: defaultShellConfigPath, object: "dock", key: "design", fallback: "ryoku", marker: dockTransactionKey}, true
	case "osds":
		return productSelection{path: defaultShellConfigPath, object: "designs", key: "osd", fallback: "ryoku", marker: osdTransactionKey}, true
	case "notifications":
		return productSelection{path: defaultShellConfigPath, object: "designs", key: "notification", fallback: "ryoku", marker: notificationTransactionKey}, true
	case "sidebars":
		return productSelection{path: defaultShellConfigPath, object: "designs", key: "sidebar", fallback: "ryoku", marker: sidebarTransactionKey}, true
	}
	return productSelection{}, false
}

func defaultLauncherConfigPath() string {
	return filepath.Join(configHome(), "ryoku", "launcher.json")
}

func defaultShellConfigPath() string {
	return filepath.Join(configHome(), "ryoku", "shell.json")
}

func writeBarStyleSelection(path, id string) error {
	return mutateConfigState(path, func(state map[string]any) bool {
		if state["barStyle"] == id {
			return false
		}
		state["barStyle"] = id
		return true
	})
}

// holder is the object that carries the setting, or nil when its section is
// missing or not an object (then nothing wears a product through it).
func (s productSelection) holder(state map[string]any) map[string]any {
	if s.object == "" {
		return state
	}
	section, _ := state[s.object].(map[string]any)
	return section
}

// current is the id the setting wears now, "" when it names nothing.
func (s productSelection) current() string {
	state, err := readConfigState(s.path())
	if err != nil {
		return ""
	}
	value, _ := s.holder(state)[s.key].(string)
	return value
}

func (s productSelection) applyFallback(id, token string) error {
	return mutateConfigState(s.path(), func(state map[string]any) bool {
		holder := s.holder(state)
		if holder == nil || holder[s.key] != id {
			return false
		}
		holder[s.key] = s.fallback
		state[s.marker] = token
		return true
	})
}

func (s productSelection) finishFallback(id, token string, committed bool) error {
	return mutateConfigState(s.path(), func(state map[string]any) bool {
		marker, owned := state[s.marker].(string)
		owned = owned && marker == token
		changed := false
		if holder := s.holder(state); holder != nil {
			if committed {
				if holder[s.key] == id {
					holder[s.key] = s.fallback
					changed = true
				}
			} else if owned && holder[s.key] == s.fallback {
				holder[s.key] = id
				changed = true
			}
		}
		if owned {
			delete(state, s.marker)
			changed = true
		}
		return changed
	})
}

// readConfigString is one top-level string setting of a JSON config, or "" when
// the file, the key or a string value is absent.
func readConfigString(path, key string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var state map[string]any
	if decodeOneJSON(raw, &state) != nil {
		return ""
	}
	value, _ := state[key].(string)
	return value
}

func lockConfigFile(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}

func readConfigState(path string) (map[string]any, error) {
	state := map[string]any{}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return state, nil
	}
	if err != nil {
		return nil, err
	}
	if err := decodeOneJSON(raw, &state); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	if state == nil {
		return nil, fmt.Errorf("parse %s: root must be an object", filepath.Base(path))
	}
	return state, nil
}

func writeConfigState(path string, state map[string]any) error {
	raw, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, append(raw, '\n'), 0o644)
}

func mutateConfigState(path string, mutate func(map[string]any) bool) error {
	unlock, err := lockConfigFile(path)
	if err != nil {
		return err
	}
	defer unlock()
	state, err := readConfigState(path)
	if err != nil {
		return err
	}
	if !mutate(state) {
		return nil
	}
	return writeConfigState(path, state)
}
