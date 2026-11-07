package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var shellEnvKeyPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// The process environment an item may never change.
var (
	shellEnvProtectedKeys     = map[string]bool{"PATH": true, "HOME": true, "USER": true, "SHELL": true, "LANG": true, "DISPLAY": true}
	shellEnvProtectedPrefixes = []string{"LC_", "LD_", "XDG_", "QT_", "QML", "WAYLAND_", "DBUS_", "RYOKU_", "MALLOC_"}
	// The only variables a value may name; the daemon expands them.
	shellEnvExpandable = []string{"HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "RYOKU_ITEM_DIR"}
)

// validateShellEnv checks an item's [shell.env] against contract rule 13.13.
func validateShellEnv(env map[string]string) error {
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !shellEnvKeyPattern.MatchString(key) {
			return fmt.Errorf("env key %q is not an upper-case name", key)
		}
		if shellEnvProtected(key) {
			return fmt.Errorf("env key %q is protected", key)
		}
		if err := validateShellEnvValue(env[key]); err != nil {
			return fmt.Errorf("env %s: %w", key, err)
		}
	}
	return nil
}

func shellEnvProtected(key string) bool {
	if shellEnvProtectedKeys[key] {
		return true
	}
	for _, prefix := range shellEnvProtectedPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func validateShellEnvValue(value string) error {
	if strings.ContainsRune(value, '\x00') {
		return fmt.Errorf("value holds a NUL byte")
	}
	for rest := value; ; {
		index := strings.IndexByte(rest, '$')
		if index < 0 {
			return nil
		}
		rest = rest[index+1:]
		name := shellEnvNamePrefix(rest)
		if !shellEnvExpandableName(name) {
			return fmt.Errorf("value names $%s, which is not expanded", name)
		}
		rest = rest[len(name):]
	}
}

// shellEnvNamePrefix is the variable name at the start of s, "" when none.
func shellEnvNamePrefix(s string) string {
	end := 0
	for end < len(s) {
		c := s[end]
		if c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || end > 0 && c >= '0' && c <= '9' {
			end++
			continue
		}
		break
	}
	return s[:end]
}

func shellEnvExpandableName(name string) bool {
	for _, allowed := range shellEnvExpandable {
		if name == allowed {
			return true
		}
	}
	return false
}
