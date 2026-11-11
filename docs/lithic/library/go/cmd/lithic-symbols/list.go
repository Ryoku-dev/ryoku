package main

import (
	"fmt"
	"os"
	"strings"
)

// entry is one line of the list: a Material Symbols name and whether its filled variant is wanted too.
type entry struct {
	name string
	fill bool
}

// readList reads one symbol per line, "name" or "name fill"; "#" starts a comment.
func readList(path string) ([]entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var entries []entry
	seen := map[string]bool{}
	for number, line := range strings.Split(string(data), "\n") {
		if hash := strings.IndexByte(line, '#'); hash >= 0 {
			line = line[:hash]
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		item := entry{name: fields[0]}
		switch {
		case len(fields) == 2 && fields[1] == "fill":
			item.fill = true
		case len(fields) != 1:
			return nil, fmt.Errorf("%s:%d: want a symbol name and an optional fill", path, number+1)
		}
		if !validName(item.name) {
			return nil, fmt.Errorf("%s:%d: %q is not a symbol name", path, number+1, item.name)
		}
		if seen[item.name] {
			return nil, fmt.Errorf("%s:%d: %s is listed twice", path, number+1, item.name)
		}
		seen[item.name] = true
		entries = append(entries, item)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("%s lists no symbols", path)
	}
	return entries, nil
}

// validName accepts the snake_case names Material Symbols uses that start with a letter, so each one spells a Go identifier.
func validName(name string) bool {
	if name == "" || name[0] < 'a' || name[0] > 'z' || strings.HasSuffix(name, "_") || strings.Contains(name, "__") {
		return false
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}
