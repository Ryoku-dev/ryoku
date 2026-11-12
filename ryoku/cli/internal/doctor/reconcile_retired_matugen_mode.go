package doctor

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// Ryogami's custom outputs follow theme.mode (matugen.json "mode") now, so the
// matugen.mode key in ryogami.json has no reader. A user who set it and never
// set theme.mode keeps their choice: it moves into matugen.json before the strip.
// matugen.json may also still hold "auto" and "scheme-smart", spellings the
// shell's palette run no longer resolves; they become smart and tonal spot.
func reconcileRetiredMatugenMode(checkOnly bool) recResult {
	dir := filepath.Join(sys.ConfigHome(), "ryoku")
	ryogamiPath := filepath.Join(dir, "ryogami.json")
	matugenPath := filepath.Join(dir, "matugen.json")

	ryogamiRaw, err := readOptional(ryogamiPath)
	if err != nil {
		return failRes(i18n.T("could not read %s: %v"), ryogamiPath, err)
	}
	matugenRaw, err := readOptional(matugenPath)
	if err != nil {
		return failRes(i18n.T("could not read %s: %v"), matugenPath, err)
	}

	ryogamiOut, carried, ryogamiChanged, err := takeRetiredMatugenMode(ryogamiRaw)
	if err != nil {
		return warnRes(i18n.T("%s does not parse (%v); Ryogami falls back to defaults"), ryogamiPath, err).
			withFix(i18n.T("delete %s to re-seed it"), ryogamiPath)
	}
	matugenOut, matugenChanged, err := reconcileMatugenDoc(matugenRaw, carried)
	if err != nil {
		return warnRes(i18n.T("%s does not parse (%v); the palette falls back to defaults"), matugenPath, err).
			withFix(i18n.T("delete %s to re-seed it"), matugenPath)
	}
	if !ryogamiChanged && !matugenChanged {
		return okRes(i18n.T("the palette settings carry no retired matugen mode or old values"))
	}
	if checkOnly {
		return wouldRes(i18n.T("the palette settings still carry the retired matugen.mode key or the old auto and scheme-smart values")).
			withFix(i18n.T("ryoku doctor carries the mode into theme.mode and rewrites them in place"))
	}
	// matugen.json first: a failure before the strip leaves the old key for a retry to carry
	if matugenChanged {
		if err := sys.WriteFileAtomic(matugenPath, string(matugenOut), 0o644); err != nil {
			return failRes(i18n.T("could not write %s: %v"), matugenPath, err)
		}
	}
	if ryogamiChanged {
		if err := sys.WriteFileAtomic(ryogamiPath, string(ryogamiOut), 0o644); err != nil {
			return failRes(i18n.T("could not write %s: %v"), ryogamiPath, err)
		}
	}
	return fixedRes(i18n.T("retired matugen.mode and rewrote the old auto and scheme-smart palette values"))
}

func readOptional(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return raw, err
}

// themeModeOf maps a stored mode onto a value theme.mode accepts, or "" when it names none.
func themeModeOf(mode string) string {
	switch mode {
	case "auto":
		return "smart"
	case "dark", "light", "smart", "sun":
		return mode
	}
	return ""
}

// takeRetiredMatugenMode drops matugen.mode from a Ryogami store and returns the
// theme mode it named, keeping every other key as its own raw bytes. A matugen
// object left empty is dropped; an absent store or key is a no-op.
func takeRetiredMatugenMode(raw []byte) (out []byte, carried string, changed bool, err error) {
	if raw == nil {
		return nil, "", false, nil
	}
	var top map[string]json.RawMessage
	if err = json.Unmarshal(raw, &top); err != nil {
		return nil, "", false, err
	}
	if top == nil {
		return nil, "", false, errors.New("not a JSON object")
	}
	matRaw, ok := top["matugen"]
	if !ok {
		return nil, "", false, nil
	}
	var mat map[string]json.RawMessage
	if err = json.Unmarshal(matRaw, &mat); err != nil {
		return nil, "", false, err
	}
	modeRaw, ok := mat["mode"]
	if !ok {
		return nil, "", false, nil
	}
	var mode string
	if json.Unmarshal(modeRaw, &mode) == nil {
		carried = themeModeOf(mode)
	}
	delete(mat, "mode")
	if len(mat) == 0 {
		delete(top, "matugen")
	} else {
		repacked, err := json.Marshal(mat)
		if err != nil {
			return nil, "", false, err
		}
		top["matugen"] = repacked
	}
	out, err = indentJSON(top)
	return out, carried, true, err
}

// reconcileMatugenDoc rewrites the old mode and scheme spellings in matugen.json
// and fills an unset mode with carried. raw nil means the file does not exist yet.
func reconcileMatugenDoc(raw []byte, carried string) (out []byte, changed bool, err error) {
	top := map[string]json.RawMessage{}
	if raw != nil {
		if err = json.Unmarshal(raw, &top); err != nil {
			return nil, false, err
		}
		if top == nil {
			return nil, false, errors.New("not a JSON object")
		}
	}
	set := func(field, value string) {
		top[field], _ = json.Marshal(value)
		changed = true
	}
	switch mode, isString := docString(top, "mode"); {
	case isString && mode == "auto":
		set("mode", "smart")
	case isString && mode == "" && carried != "":
		set("mode", carried)
	}
	if scheme, _ := docString(top, "schemeType"); scheme == "scheme-smart" {
		set("schemeType", "scheme-tonal-spot")
	}
	if !changed {
		return nil, false, nil
	}
	out, err = indentJSON(top)
	return out, true, err
}

// docString reads a string field; an absent or null field reads as "" and still counts as a string.
func docString(top map[string]json.RawMessage, field string) (string, bool) {
	raw, ok := top[field]
	if !ok {
		return "", true
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return "", false
	}
	return s, true
}

func indentJSON(top map[string]json.RawMessage) ([]byte, error) {
	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
