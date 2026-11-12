package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"

	"ryoku-cli/internal/sys"
	i18n "ryoku-i18n"
)

// The Hub's language picker used to store a human-readable name ("Español",
// "Português (BR)") because the list was six long and written into the schema.
// It now stores the catalog code, read from the one language table
// (ryoku/i18n/langs.json), so a language is added in one place and the picker
// can show 34 of them under their own names.
//
// The runtimes still resolve an old name (I18n.qml maps every spelling in the
// table, and the Go runtime's match() accepts a name or a native name), so
// nothing breaks unmigrated. This rewrites it anyway: the value a user sees in
// their own shell.json should be the one the picker writes today, and a name
// that leaves the table later would silently fall back to the system locale.
func reconcileShellLanguage(checkOnly bool) recResult {
	path := filepath.Join(sys.ConfigHome(), "ryoku", "shell.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return okRes(i18n.T("no shell.json yet; the shell seeds language on first run"))
	}
	var store map[string]json.RawMessage
	if err := json.Unmarshal(raw, &store); err != nil {
		return warnRes(i18n.T("shell.json does not parse (%v); the shell falls back to defaults"), err).
			withFix(i18n.T("delete %s to re-seed it"), path)
	}
	cur, ok := store["language"]
	if !ok {
		return okRes(i18n.T("shell.json has no language key; Auto follows the system locale"))
	}
	var name string
	if json.Unmarshal(cur, &name) != nil || name == "" || name == "auto" || name == "Auto" {
		return okRes(i18n.T("interface language follows the system locale"))
	}
	// already a catalog code: nothing to do.
	if _, isCode := i18n.Find(name); isCode {
		return okRes(i18n.Tf("interface language is the catalog code %s", name))
	}
	code := i18n.Resolve(name)
	// Resolve falls through to the environment when it cannot place the value,
	// so only rewrite when the stored value itself named a language.
	if code == "en" && !namesEnglish(name) {
		return warnRes(i18n.Tf("shell.json language %q is not a language Ryoku ships", name)).
			withFix(i18n.T("pick one again in Ryoku Settings > System > Time & language"))
	}
	if checkOnly {
		return wouldRes(i18n.Tf("shell.json language is the old name %q, not the code %q", name, code))
	}
	next, err := json.Marshal(code)
	if err != nil {
		return failRes(i18n.T("could not encode the language code: %v"), err)
	}
	store["language"] = next
	out, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return failRes(i18n.T("could not re-encode shell.json: %v"), err)
	}
	tmp := path + ".ryoku-tmp"
	if err := os.WriteFile(tmp, append(out, '\n'), 0o644); err != nil {
		return failRes(i18n.T("could not write %s: %v"), tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return failRes(i18n.T("could not replace %s: %v"), path, err)
	}
	return fixedRes(i18n.Tf("rewrote shell.json language %q as the code %q", name, code))
}

// namesEnglish tells an intentional English choice from a value Resolve could
// not place, both of which come back as "en".
func namesEnglish(v string) bool {
	l, ok := i18n.Find("en")
	return ok && (v == l.Name || v == l.Native || v == l.Code)
}
