package main

import (
	"fmt"
	"sort"

	wm "ryoku-wm"
)

// Ryoku Settings owns the language; the folio's row hands off to its page.
func init() {
	registerAction("OpenTimeLanguage", func(d *daemon, _ map[string]interface{}) (interface{}, error) {
		runDetachedShell("ryoku-shell hub open time-language")
		return map[string]interface{}{"ok": true}, nil
	})
}

func (d *daemon) dispatchSettings(req *request, p map[string]interface{}) (response, bool) {
	switch req.Method {
	case "settings.schema":
		return d.settingsSchema(req), true
	case "settings.get":
		if boolParam(p, "effective", false) {
			return d.effectiveValues(req, strsParam(p, "keys")), true
		}
		return ok(req.ID, map[string]interface{}{"values": d.collectValues(strsParam(p, "keys"))}), true
	case "settings.set":
		return d.settingsSet(req, p), true
	case "settings.reset":
		return d.settingsReset(req, p), true
	case "settings.action":
		return d.settingsAction(req, p), true
	}
	return response{}, false
}

func (d *daemon) settingsSchema(req *request) response {
	caps, _ := wmClient.Caps()
	capMap := make(map[string]bool, len(wm.All()))
	for _, c := range wm.All() {
		capMap[string(c)] = caps.Has(c)
	}
	return ok(req.ID, map[string]interface{}{
		"revision":     settingsRevision,
		"keys":         settingsSchema,
		"availability": d.availabilityMap(),
		"caps":         capMap,
	})
}

// settings.set takes a batch; null resets a key, and every document key lands in one write per file.
func (d *daemon) settingsSet(req *request, p map[string]interface{}) response {
	raw, hasValues := p["values"].(map[string]interface{})
	if !hasValues {
		return errResp(req.ID, 1, "missing 'values' parameter")
	}
	keys := sortedKeys(raw)
	// Validate the whole batch first so a bad value never leaves a half-applied set.
	docs := map[string]interface{}{}
	resets := map[string]bool{}
	for _, k := range keys {
		spec, known := specFor(k)
		if raw[k] == nil {
			if !known {
				return errResp(req.ID, 5, k+": unknown setting")
			}
			resets[k] = true
			raw[k] = spec.Default
		} else {
			raw[k] = normalizeSetting(k, raw[k])
			if err := validateSetting(k, raw[k]); err != nil {
				return errResp(req.ID, 5, err.Error())
			}
		}
		if known && isDocStore(spec.Store) {
			if resets[k] {
				docs[k] = nil
			} else {
				docs[k] = raw[k]
			}
		}
	}
	if err := writeDocSettings(docs); err != nil {
		return errResp(req.ID, 4, err.Error())
	}
	for _, k := range keys {
		if _, isDoc := docs[k]; isDoc {
			continue
		}
		var err error
		if resets[k] {
			err = d.resetSetting(k)
		} else {
			err = d.persistSetting(k, raw[k])
		}
		if err != nil {
			return errResp(req.ID, 4, fmt.Sprintf("%s: %v", k, err))
		}
	}
	d.broadcast("ryogami.settings.changed", map[string]interface{}{"values": raw})
	for _, k := range keys {
		d.notifySetting(k, raw[k])
	}
	return ok(req.ID, map[string]interface{}{"ok": true})
}

func (d *daemon) settingsReset(req *request, p map[string]interface{}) response {
	keys := strsParam(p, "keys")
	if len(keys) == 0 {
		return errResp(req.ID, 1, "missing 'keys' parameter")
	}
	defaults := map[string]interface{}{}
	for _, k := range keys {
		if err := d.resetSetting(k); err != nil {
			return errResp(req.ID, 4, fmt.Sprintf("%s: %v", k, err))
		}
		if spec, ok := specFor(k); ok {
			defaults[k] = spec.Default
		} else {
			defaults[k] = nil
		}
	}
	d.broadcast("ryogami.settings.changed", map[string]interface{}{"values": defaults})
	for k, v := range defaults {
		d.notifySetting(k, v)
	}
	return ok(req.ID, map[string]interface{}{"ok": true})
}

func (d *daemon) settingsAction(req *request, p map[string]interface{}) response {
	id := strParam(p, "id", "")
	if id == "" {
		return errResp(req.ID, 1, "missing 'id' parameter")
	}
	args, _ := p["args"].(map[string]interface{})
	res, err := d.runAction(id, args)
	if err != nil {
		return errResp(req.ID, 4, err.Error())
	}
	return ok(req.ID, map[string]interface{}{"result": res})
}

// effectiveValues answers what a key reads as now, defaults included; an unknown key is an error.
func (d *daemon) effectiveValues(req *request, keys []string) response {
	out := map[string]interface{}{}
	for _, k := range keys {
		if _, ok := specFor(k); !ok {
			if _, ok := userRyogami(k); !ok {
				return errResp(req.ID, 5, k+": unknown setting")
			}
		}
		out[k] = settingValue(k)
	}
	return ok(req.ID, map[string]interface{}{"values": out})
}

// A key with no explicit value is omitted so the client falls back to the schema default.
func (d *daemon) collectValues(keys []string) map[string]interface{} {
	tree := currentRyogamiTree()
	shell := loadShellTree()
	docs := loadDocs()
	out := map[string]interface{}{}
	if len(keys) > 0 {
		for _, k := range keys {
			if v, ok := d.explicitValue(k, tree, shell, docs); ok {
				out[k] = v
			}
		}
		return out
	}
	for i := range settingsSchema {
		k := settingsSchema[i].Key
		if v, ok := d.explicitValue(k, tree, shell, docs); ok {
			out[k] = v
		}
	}
	for k, v := range flattenSettings(tree) {
		// Only literal schema keys were covered above; per-output keys come from the tree.
		if _, isSchema := schemaIndex[k]; !isSchema {
			out[k] = v
		}
	}
	return out
}

// A shell or document value equal to the schema default reports as absent, like an unset key.
func (d *daemon) explicitValue(key string, tree, shell map[string]interface{}, docs map[string]map[string]interface{}) (interface{}, bool) {
	spec, ok := specFor(key)
	if !ok {
		return lookupDotted(tree, key)
	}
	switch spec.Store {
	case "shell":
		v, ok := d.shellExplicit(spec, tree, shell)
		if !ok || sameSetting(v, spec.Default) {
			return nil, false
		}
		return v, true
	case "matugen", "theme":
		v, ok := docs[spec.Store][docField(spec)]
		v = normalizeSetting(key, v)
		if !ok || sameSetting(v, spec.Default) {
			return nil, false
		}
		return v, true
	default:
		return lookupDotted(tree, key)
	}
}

func (d *daemon) shellExplicit(spec *settingSpec, tree, shell map[string]interface{}) (interface{}, bool) {
	if spec.Key == "display.fillMode" {
		if v, ok := lookupDotted(tree, "display.fillMode"); ok {
			return v, true
		}
		if v, ok := lookupDotted(shell, "wallpaper.content_fit"); ok {
			if s, ok := v.(string); ok {
				return contentFitToMode(s), true
			}
		}
		return nil, false
	}
	return lookupDotted(shell, shellKeyOf(spec))
}

// Overlaid with writes the debounced flush has not persisted, so a read after a write is consistent.
func currentRyogamiTree() map[string]interface{} {
	tree := loadRyogamiTree()
	if tree == nil {
		tree = map[string]interface{}{}
	}
	settingsMu.Lock()
	for k, v := range settingsPending {
		if isTombstone(v) {
			deleteDotted(tree, k)
		} else {
			setDotted(tree, k, v)
		}
	}
	settingsMu.Unlock()
	return tree
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
