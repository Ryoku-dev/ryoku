package harness

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ryoku-i18n"
	"ryoku-rashin/internal/paths"
	"ryoku-rashin/internal/protocol"
)

// setup.go reads what a harness's own settings say it will use, so the app can
// show it before the harness ever runs. The files belong to the harness:
// Rashin reads them and never writes them. A harness with no reader here is
// described by what it reports once it has started.

func setupOf(id string) []protocol.HarnessFact {
	switch id {
	case "omp":
		return ompSetup(filepath.Join(paths.Home(), ".omp", "agent", "config.yml"))
	}
	return nil
}

// ompSetup reads the model behind each of omp's roles and its thinking level.
// The default role comes first, then smol, then the rest by name.
func ompSetup(path string) []protocol.HarnessFact {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	roles := make(map[string]string)
	thinking := ""
	inRoles := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		indented := line[0] == ' ' || line[0] == '\t'
		key, value, ok := yamlPair(line)
		if !ok {
			continue
		}
		switch {
		case !indented:
			inRoles = key == "modelRoles" && value == ""
			if key == "defaultThinkingLevel" {
				thinking = value
			}
		case inRoles && value != "":
			roles[key] = value
		}
	}
	names := make([]string, 0, len(roles))
	for name := range roles {
		names = append(names, name)
	}
	sort.Slice(names, func(a, b int) bool {
		rank := func(name string) int {
			switch name {
			case "default":
				return 0
			case "smol":
				return 1
			}
			return 2
		}
		if rank(names[a]) != rank(names[b]) {
			return rank(names[a]) < rank(names[b])
		}
		return names[a] < names[b]
	})
	facts := make([]protocol.HarnessFact, 0, len(names)+1)
	for _, name := range names {
		label := i18n.Tf("%s model", name)
		if name == "default" {
			label = i18n.T("Model")
		}
		facts = append(facts, protocol.HarnessFact{Label: label, Value: roles[name]})
	}
	if thinking != "" {
		facts = append(facts, protocol.HarnessFact{Label: i18n.T("Thinking"), Value: thinking})
	}
	return facts
}

// yamlPair reads one `key: value` line of a flat YAML map, which is all the
// settings read here are. A value loses its quotes and any trailing comment.
func yamlPair(line string) (key, value string, ok bool) {
	key, value, ok = strings.Cut(strings.TrimSpace(line), ":")
	if !ok || key == "" || strings.ContainsAny(key, " \t") {
		return "", "", false
	}
	value = strings.TrimSpace(value)
	if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
		return key, value[1 : len(value)-1], true
	}
	if comment := strings.Index(value, " #"); comment >= 0 {
		value = strings.TrimSpace(value[:comment])
	}
	return key, value, true
}
