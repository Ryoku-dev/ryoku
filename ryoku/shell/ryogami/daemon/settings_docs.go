package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// settingsDoc is a flat JSON file the shell, the doctor and rices read. Ryogami
// is its one writer: every other tool sets these keys through `ryogami settings
// set`, so a merge here never races a second merge somewhere else.
type settingsDoc struct {
	path   func() string
	fields map[string]string // schema key -> document field
}

var settingsDocs = map[string]settingsDoc{
	// the palette knobs the shell reads before every repaint
	"matugen": {path: matugenStorePath, fields: map[string]string{
		"theme.mode":             "mode",
		"matugen.schemeType":     "schemeType",
		"matugen.contrast":       "contrast",
		"matugen.colorIndex":     "sourceColorIndex",
		"matugen.lightnessDark":  "lightnessDark",
		"matugen.lightnessLight": "lightnessLight",
		"matugen.prefer":         "prefer",
		"matugen.themeRyokuApps": "themeRyokuApps",
		"matugen.templates":      "templates",
	}},
	// the toolkit choices the shell resolves on each repaint
	"theme": {path: themeStorePath, fields: map[string]string{
		"theme.themeApps":   "themeApps",
		"theme.gtkTheme":    "gtkTheme",
		"theme.gnomeAccent": "gnomeAccent",
	}},
}

var settingsDocMu sync.Mutex

func matugenStorePath() string { return filepath.Join(ryokuConfigDir(), "matugen.json") }
func themeStorePath() string   { return filepath.Join(ryokuConfigDir(), "theme.json") }

func isDocStore(store string) bool {
	_, ok := settingsDocs[store]
	return ok
}

func docField(spec *settingSpec) string { return settingsDocs[spec.Store].fields[spec.Key] }

func loadDoc(store string) map[string]interface{} {
	doc, ok := settingsDocs[store]
	if !ok {
		return nil
	}
	b, err := os.ReadFile(doc.path())
	if err != nil {
		return nil
	}
	var m map[string]interface{}
	if json.Unmarshal(b, &m) != nil {
		return nil
	}
	return m
}

func loadDocs() map[string]map[string]interface{} {
	out := make(map[string]map[string]interface{}, len(settingsDocs))
	for store := range settingsDocs {
		out[store] = loadDoc(store)
	}
	return out
}

func docSettingValue(spec *settingSpec) interface{} {
	if v, ok := loadDoc(spec.Store)[docField(spec)]; ok {
		return normalizeSetting(spec.Key, v)
	}
	return spec.Default
}

// updateDoc rewrites the document under one lock, keeping every field mutate leaves alone.
// The hub rewrites its own theme.json fields, so a sibling flock keeps each read-modify-write whole.
func updateDoc(store string, mutate func(m map[string]interface{}) bool) error {
	doc, ok := settingsDocs[store]
	if !ok {
		return fmt.Errorf("unknown settings document %q", store)
	}
	settingsDocMu.Lock()
	defer settingsDocMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(doc.path()), 0o755); err != nil {
		return err
	}
	lock, err := os.OpenFile(doc.path()+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	m := loadDoc(store)
	if m == nil {
		if _, err := os.Stat(doc.path()); err == nil {
			return fmt.Errorf("%s is not a JSON object; fix or remove it first", doc.path())
		}
		m = map[string]interface{}{}
	}
	if !mutate(m) {
		return nil
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return settingsAtomicWrite(doc.path(), append(b, '\n'))
}

func writeDocSetting(spec *settingSpec, value interface{}) error {
	return writeDocSettings(map[string]interface{}{spec.Key: value})
}

// An absent field reads as its default everywhere, so a reset drops it.
func resetDocSetting(spec *settingSpec) error {
	return writeDocSettings(map[string]interface{}{spec.Key: nil})
}

// one write per document, so a whole rice repaints once; nil drops the field
func writeDocSettings(values map[string]interface{}) error {
	byStore := map[string]map[string]interface{}{}
	for key, value := range values {
		spec, ok := specFor(key)
		if !ok || !isDocStore(spec.Store) {
			return fmt.Errorf("%s is not a document setting", key)
		}
		field := docField(spec)
		if field == "" {
			return fmt.Errorf("no %s field for %s", spec.Store, spec.Key)
		}
		v := normalizeSetting(key, value)
		if n, ok := toNumber(v); ok && key == "matugen.colorIndex" {
			v = int(n)
		}
		if byStore[spec.Store] == nil {
			byStore[spec.Store] = map[string]interface{}{}
		}
		byStore[spec.Store][field] = v
	}
	for store, fields := range byStore {
		err := updateDoc(store, func(m map[string]interface{}) bool {
			changed := false
			for field, v := range fields {
				old, had := m[field]
				switch {
				case v == nil && had:
					delete(m, field)
					changed = true
				case v != nil && (!had || !sameDocValue(old, v)):
					m[field] = v
					changed = true
				}
			}
			return changed
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// a stored 4 decodes as float64 while a written one is int; both are the same value
func sameDocValue(a, b interface{}) bool {
	if x, ok := toNumber(a); ok {
		if y, ok := toNumber(b); ok {
			return x == y
		}
	}
	return sameSetting(a, b)
}

// one non-bool roster entry would make the shell drop the whole matugen.json
func validateDocValue(key string, value interface{}) error {
	if key != "matugen.templates" {
		return nil
	}
	roster, _ := value.(map[string]interface{})
	for id, v := range roster {
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("%s: %s must be true or false", key, id)
		}
	}
	return nil
}

// normalizeSetting maps values older tools wrote onto the ones the shell resolves:
// theme.mode auto was the same luminance rule as smart, and the shell's palette
// run rejects scheme-smart outright. ryoku doctor rewrites the stored files; a
// rice captured before still hands these to `set`.
func normalizeSetting(key string, value interface{}) interface{} {
	s, ok := value.(string)
	if !ok {
		return value
	}
	switch {
	case key == "theme.mode" && s == "auto":
		return "smart"
	case key == "matugen.schemeType" && s == "scheme-smart":
		return "scheme-tonal-spot"
	}
	return value
}

func init() {
	// the shell re-resolves gtk-theme, the accent and app theming on a repaint
	for key := range settingsDocs["theme"].fields {
		watchSetting(key, func(d *daemon, _ string, _ interface{}) {
			if d.surface != nil {
				d.surface.republish()
			}
		})
	}
}
