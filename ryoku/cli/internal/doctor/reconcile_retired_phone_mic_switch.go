package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"

	"ryoku-cli/internal/sys"

	i18n "ryoku-i18n"
)

// The phone microphone starts on demand from its card, so the shell.json switch
// phone.microphone.enabled never had a reader and is gone from the schema; the
// daemon now rejects writes to it. This strips a stored one. Surgical and
// idempotent: a store already free of it is untouched.
func reconcileRetiredPhoneMicSwitch(checkOnly bool) recResult {
	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return okRes(i18n.T("no shell.json yet (seeded on first shell run)"))
	}
	migrated, changed, err := stripRetiredPhoneMicSwitch(raw)
	if err != nil {
		return warnRes(i18n.T("shell.json does not parse (%v); the shell falls back to defaults"), err).
			withFix(i18n.T("delete %s to re-seed it"), path)
	}
	if !changed {
		return okRes(i18n.T("shell.json carries no retired phone microphone switch"))
	}
	if checkOnly {
		return wouldRes(i18n.T("shell.json still carries the retired phone microphone switch")).
			withFix(i18n.T("ryoku doctor strips it in place"))
	}
	tmp := path + ".ryoku-tmp"
	if err := os.WriteFile(tmp, migrated, 0o644); err != nil {
		return failRes(i18n.T("could not write %s: %v"), tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return failRes(i18n.T("could not replace %s: %v"), path, err)
	}
	return fixedRes(i18n.T("stripped the retired phone microphone switch from shell.json"))
}

// stripRetiredPhoneMicSwitch drops enabled from a shell store's phone.microphone
// object, keeping every other key as its own raw bytes. An absent phone or
// microphone object, or one already free of it, is a no-op; a microphone object
// left empty is dropped; a malformed one errors rather than being rewritten.
func stripRetiredPhoneMicSwitch(raw []byte) (out []byte, changed bool, err error) {
	var top map[string]json.RawMessage
	if err = json.Unmarshal(raw, &top); err != nil {
		return nil, false, err
	}
	phoneRaw, ok := top["phone"]
	if !ok {
		return nil, false, nil
	}
	var phone map[string]json.RawMessage
	if err = json.Unmarshal(phoneRaw, &phone); err != nil {
		return nil, false, err
	}
	micRaw, ok := phone["microphone"]
	if !ok {
		return nil, false, nil
	}
	var mic map[string]json.RawMessage
	if err = json.Unmarshal(micRaw, &mic); err != nil {
		return nil, false, err
	}
	if _, ok := mic["enabled"]; !ok {
		return nil, false, nil
	}
	delete(mic, "enabled")
	if len(mic) == 0 {
		delete(phone, "microphone")
	} else {
		if phone["microphone"], err = json.Marshal(mic); err != nil {
			return nil, false, err
		}
	}
	if top["phone"], err = json.Marshal(phone); err != nil {
		return nil, false, err
	}
	out, err = json.MarshalIndent(top, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), true, nil
}
