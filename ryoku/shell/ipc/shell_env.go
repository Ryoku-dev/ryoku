package main

import (
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var itemEnvKey = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

var itemEnvProtectedKeys = map[string]bool{
	"PATH": true, "HOME": true, "USER": true, "SHELL": true, "LANG": true, "DISPLAY": true,
}

var itemEnvProtectedPrefixes = []string{"LC_", "LD_", "XDG_", "QT_", "QML", "WAYLAND_", "DBUS_", "RYOKU_", "MALLOC_"}

// itemEnvDropped remembers rejected entries so a bad record logs once, not on
// every shell start.
var itemEnvDropped sync.Map

func itemEnvKeyAllowed(key string) bool {
	if !itemEnvKey.MatchString(key) || itemEnvProtectedKeys[key] {
		return false
	}
	for _, prefix := range itemEnvProtectedPrefixes {
		if strings.HasPrefix(key, prefix) {
			return false
		}
	}
	return true
}

func itemEnvBase(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

// itemEnvVars are the only names an item's env value may reference.
func itemEnvVars(itemDir string) map[string]string {
	home := os.Getenv("HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	return map[string]string{
		"HOME":            home,
		"XDG_DATA_HOME":   itemEnvBase("XDG_DATA_HOME", filepath.Join(home, ".local", "share")),
		"XDG_STATE_HOME":  itemEnvBase("XDG_STATE_HOME", filepath.Join(home, ".local", "state")),
		"XDG_CONFIG_HOME": itemEnvBase("XDG_CONFIG_HOME", filepath.Join(home, ".config")),
		"XDG_CACHE_HOME":  itemEnvBase("XDG_CACHE_HOME", filepath.Join(home, ".cache")),
		"RYOKU_ITEM_DIR":  itemDir,
	}
}

// expandItemEnvValue expands $NAME and ${NAME} for the allowed names and
// rejects any other use of $.
func expandItemEnvValue(value string, vars map[string]string) (string, bool) {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '$' {
			b.WriteByte(value[i])
			continue
		}
		rest := value[i+1:]
		name, width := "", 0
		if strings.HasPrefix(rest, "{") {
			end := strings.IndexByte(rest, '}')
			if end < 0 {
				return "", false
			}
			name, width = rest[1:end], end+1
		} else {
			for width < len(rest) && (rest[width] == '_' || rest[width] >= 'A' && rest[width] <= 'Z' || width > 0 && rest[width] >= '0' && rest[width] <= '9') {
				width++
			}
			name = rest[:width]
		}
		expanded, ok := vars[name]
		if !ok {
			return "", false
		}
		b.WriteString(expanded)
		i += width
	}
	return b.String(), true
}

// itemEnv returns the valid entries of an item's [shell.env] as sorted KEY=VALUE
// pairs, expanded against the item's own dir.
func itemEnv(item *shellItem) []string {
	if item == nil || len(item.Env) == 0 {
		return nil
	}
	vars := itemEnvVars(shellItemDir(item))
	keys := make([]string, 0, len(item.Env))
	for key := range item.Env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		raw := item.Env[key]
		value, ok := expandItemEnvValue(raw, vars)
		if !itemEnvKeyAllowed(key) || !ok {
			if _, seen := itemEnvDropped.LoadOrStore(item.ID+"\x00"+key+"\x00"+raw, true); !seen {
				log.Printf("ryoku-shell: shell item %s: dropped env %s=%q", item.ID, key, raw)
			}
			continue
		}
		out = append(out, key+"="+value)
	}
	return out
}

// withItemEnv replaces any inherited value of the item's env keys.
func withItemEnv(base []string, item *shellItem) []string {
	extra := itemEnv(item)
	if len(extra) == 0 {
		return base
	}
	keys := make(map[string]bool, len(extra))
	for _, entry := range extra {
		keys[entry[:strings.IndexByte(entry, '=')]] = true
	}
	env := make([]string, 0, len(base)+len(extra))
	for _, entry := range base {
		if eq := strings.IndexByte(entry, '='); eq > 0 && keys[entry[:eq]] {
			continue
		}
		env = append(env, entry)
	}
	return append(env, extra...)
}

// activeShellItem is the prepared active record whose env the shell runs with.
func (d *daemon) activeShellItem() *shellItem {
	index := d.currentShellIndex()
	if index == nil {
		return nil
	}
	item := index.item(index.Active)
	if item == nil || !item.Prepared {
		return nil
	}
	return item
}

// shellQsEnv is the resident shell's environment: qsEnv plus the active item's env.
func (d *daemon) shellQsEnv() []string {
	return withItemEnv(qsEnv(), d.activeShellItem())
}
