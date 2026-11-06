package profile

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

func expandTokens(value string, v Vars) string {
	r := strings.NewReplacer("@HOME@", v.Home, "@USER@", v.User, "@ID@", v.ID, "@PROFILE_BIN@", ProfileBinDir(v.ID))
	return r.Replace(value)
}

func (m *Manifest) PathEntries(v Vars) []string {
	out := make([]string, 0, len(m.Path))
	for _, entry := range m.Path {
		out = append(out, expandTokens(entry, v))
	}
	return out
}

func ComposePATH(entries []string, home string) string {
	all := append(append([]string{}, entries...), filepath.Join(home, ".local", "bin"), "/usr/local/bin", "/usr/bin", "/bin")
	seen := map[string]bool{}
	out := make([]string, 0, len(all))
	for _, entry := range all {
		if entry != "" && !seen[entry] {
			seen[entry] = true
			out = append(out, entry)
		}
	}
	return strings.Join(out, ":")
}

func DesktopEnv(m *Manifest, v Vars) []EnvVar {
	out := []EnvVar{{Key: "PATH", Value: ComposePATH(m.PathEntries(v), v.Home)}}
	for _, env := range m.Env {
		out = append(out, EnvVar{Key: env.Key, Value: expandTokens(env.Value, v)})
	}
	return out
}

func EnvironmentD(m *Manifest, v Vars) []byte {
	var b strings.Builder
	entries := m.PathEntries(v)
	path := strings.Join(entries, ":")
	if path != "" {
		path += ":"
	}
	fmt.Fprintf(&b, "PATH=%s${PATH}\n", environmentValue(path))
	for _, env := range m.Env {
		fmt.Fprintf(&b, "%s=%s\n", env.Key, environmentValue(expandTokens(env.Value, v)))
	}
	return []byte(b.String())
}

func environmentValue(value string) string {
	if !strings.ContainsAny(value, " \t\"\\") {
		return value
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
}

func MergeDesktopEnv(desktopJSON []byte, owned []string, add []EnvVar) (out []byte, ownedNow []string, err error) {
	root := map[string]any{}
	if len(strings.TrimSpace(string(desktopJSON))) != 0 {
		if err := json.Unmarshal(desktopJSON, &root); err != nil {
			return nil, nil, fmt.Errorf("desktop.json: %w", err)
		}
	}
	if root == nil {
		root = map[string]any{}
	}
	desktop, _ := root["desktop"].(map[string]any)
	if desktop == nil {
		desktop = map[string]any{}
		root["desktop"] = desktop
	}
	current := []EnvVar{}
	if raw, ok := desktop["env"]; ok {
		body, marshalErr := json.Marshal(raw)
		if marshalErr != nil {
			return nil, nil, marshalErr
		}
		if err := json.Unmarshal(body, &current); err != nil {
			return nil, nil, fmt.Errorf("desktop.env: %w", err)
		}
	}
	ownedSet := map[string]bool{}
	for _, key := range owned {
		ownedSet[key] = true
	}
	merged := make([]EnvVar, 0, len(current)+len(add))
	for _, env := range current {
		if !ownedSet[env.Key] {
			merged = append(merged, env)
		}
	}
	addByKey := map[string]EnvVar{}
	for _, env := range add {
		addByKey[env.Key] = env
	}
	keys := make([]string, 0, len(addByKey))
	for key := range addByKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		merged = append(merged, addByKey[key])
		ownedNow = append(ownedNow, key)
	}
	desktop["env"] = merged
	out, err = json.MarshalIndent(root, "", "  ")
	if err == nil {
		out = append(out, '\n')
	}
	return out, ownedNow, err
}
