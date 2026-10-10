package main

import "fmt"

const daemonVersion = "0.3.0"

// dispatchRequest routes one JSON-RPC request. The method set and response
// shapes are the wire contract the picker parses.
func (d *daemon) dispatchRequest(req *request) response {
	p := req.params()
	switch req.Method {
	case "status":
		return ok(req.ID, map[string]interface{}{
			"version":           daemonVersion,
			"current_wallpaper": nullable(d.currentName()),
		})

	// wm.focusedOutput: the picker opens on the display in use, which no Wayland
	// protocol reports to a client.
	case "wm.focusedOutput":
		name := ""
		for _, o := range outputs.list() {
			if o.Focused {
				name = o.Name
				break
			}
		}
		return ok(req.ID, map[string]interface{}{"name": name})

	case "state.get":
		if v, okKey := d.store.stateGet(strParam(p, "key", "")); okKey {
			return ok(req.ID, map[string]interface{}{"value": v})
		}
		return ok(req.ID, map[string]interface{}{"value": nil})

	case "state.set":
		key := strParam(p, "key", "")
		if key == "" {
			return errResp(req.ID, 1, "missing 'key' parameter")
		}
		if v, has := p["value"].(string); has {
			d.store.stateSet(key, &v)
		} else {
			d.store.stateSet(key, nil)
		}
		return ok(req.ID, map[string]interface{}{"ok": true})

	case "wall.list":
		rows := d.store.list(boolParam(p, "favourites", false))
		return ok(req.ID, map[string]interface{}{"count": len(rows), "wallpapers": rows})

	case "wall.apply":
		if strParam(p, "target", "") == "workspace" || p["workspace"] != nil {
			wall, err := d.assignWorkspace(p)
			if err != nil {
				return errResp(req.ID, 4, err.Error())
			}
			return ok(req.ID, map[string]interface{}{"applied": workspaceTargetLabel(workspaceTarget{
				Provider: wall.Provider, ID: wall.ID, Name: wall.Name, Output: wall.Output,
			})})
		}
		// A caller that only has a file (a fresh download) leaves the type to its extension.
		path := strParam(p, "path", "")
		wpType := strParam(p, "type", "")
		if wpType == "" {
			wpType = typeOf(path)
		}
		switch wpType {
		case "static", "video":
			if err := d.applyWallpaper(wpType, path, "set", strsParam(p, "outputs"), muteParam(p), volumeParam(p)); err != nil {
				return errResp(req.ID, 4, err.Error())
			}
		case "we":
			if err := d.applyWE(weIDParam(p), strsParam(p, "outputs"), muteParam(p), volumeParam(p)); err != nil {
				return errResp(req.ID, 4, err.Error())
			}
		default:
			return errResp(req.ID, 1, fmt.Sprintf("unsupported type: %s", wpType))
		}
		return ok(req.ID, map[string]interface{}{"applied": d.currentName()})

	case "wall.assign":
		wall, err := d.assignWorkspace(p)
		if err != nil {
			return errResp(req.ID, 4, err.Error())
		}
		return ok(req.ID, map[string]interface{}{"assignment": wall})

	case "wall.unassign":
		removed, err := d.unassignWorkspace(p)
		if err != nil {
			return errResp(req.ID, 4, err.Error())
		}
		return ok(req.ID, map[string]interface{}{"removed": removed})

	case "wall.workspaces":
		return ok(req.ID, d.workspaces.snapshot())

	case "effects.list":
		return ok(req.ID, map[string]interface{}{"effects": EffectsList()})

	case "effects.preview":
		out, err := EffectsPreview(d.config().cacheDir(), strParam(p, "input", ""), strParam(p, "effect", ""), subParams(p))
		if err != nil {
			return errResp(req.ID, 3, err.Error())
		}
		return ok(req.ID, map[string]interface{}{"output": out})

	case "effects.commit":
		out, err := EffectsCommit(strParam(p, "preview", ""), strParam(p, "input", ""), strParam(p, "effect", ""), subParams(p))
		if err != nil {
			return errResp(req.ID, 3, err.Error())
		}
		go d.rescan(true)
		return ok(req.ID, map[string]interface{}{"output": out})

	case "effects.discard":
		if err := EffectsDiscard(strParam(p, "preview", "")); err != nil {
			return errResp(req.ID, 3, err.Error())
		}
		return ok(req.ID, map[string]interface{}{"ok": true})

	// Video conversion runs from the settings folio's ConvertVideos action.
	case "optimize.start":
		if err := d.optimizer.Start("optimize", strParam(p, "preset", "balanced"), strParam(p, "resolution", "4k"), nil); err != nil {
			return errResp(req.ID, 3, err.Error())
		}
		return ok(req.ID, map[string]interface{}{"started": true})

	case "optimize.cancel":
		d.optimizer.Cancel("optimize")
		return ok(req.ID, map[string]interface{}{"cancelled": true})

	case "optimize.status":
		return ok(req.ID, d.optimizer.Status("optimize"))

	case "wall.set_favourite":
		key := strParam(p, "key", "")
		fav := 0
		if boolParam(p, "favourite", false) {
			fav = 1
		}
		if !d.store.mutate(key, func(e *Entry) { e.Favourite = fav }) {
			return errResp(req.ID, 2, fmt.Sprintf("unknown wallpaper: %s", key))
		}
		// The picker's Library only learns of catalog edits through wall.cached,
		// so the heart would never flip without this echo.
		if updated, has := d.store.get(key); has {
			d.broadcast("ryogami.wall.cached", updated)
		}
		return ok(req.ID, map[string]interface{}{"ok": true})

	case "wall.delete":
		if err := d.deleteWallpaper(strParam(p, "key", "")); err != nil {
			return errResp(req.ID, 2, err.Error())
		}
		return ok(req.ID, map[string]interface{}{"ok": true})

	case "wall.outputs":
		return ok(req.ID, map[string]interface{}{"outputs": d.outputsState()})

	// Forces a rescan; the daemon's end-to-end tests drive the catalogue with it.
	case "wall.cache_rebuild":
		go d.rescan(true)
		return ok(req.ID, map[string]interface{}{"started": true})

	case "wall.pause":
		paused := true
		if v, has := p["paused"].(bool); has {
			paused = v
		}
		d.setManualPause(strsParam(p, "outputs"), paused)
		return ok(req.ID, map[string]interface{}{"paused": paused})

	case "wall.set_audio":
		var mute *bool
		if v, has := p["mute"].(bool); has {
			mute = &v
		}
		var volume *int
		if v, has := p["volume"].(float64); has {
			vol := clampVolume(int(v))
			volume = &vol
		}
		d.setAudio(mute, volume, strsParam(p, "outputs"))
		return ok(req.ID, map[string]interface{}{"ok": true})

	case "picker.hidden":
		go d.ui.refreshIfStale()
		return ok(req.ID, map[string]interface{}{"ok": true})

	case "wall.random_start":
		interval := intParam(p, "interval", 300)
		types := strsParam(p, "types")
		favOnly := boolParam(p, "favourites_only", false)
		d.random.start(interval, types, favOnly, func() { d.randomPick(types, favOnly) })
		d.broadcast("ryogami.wall.random_started", map[string]interface{}{
			"interval": interval, "types": types, "favourites_only": favOnly,
		})
		return ok(req.ID, map[string]interface{}{"started": true})

	// Auto-rotate covers the random loop and assigned playlists; the schedule has its own switch.
	case "wall.rotation_status":
		st := d.random.status()
		running, _ := st["running"].(bool)
		return ok(req.ID, map[string]interface{}{"active": running || d.playlists.anyAssigned()})

	case "wall.rotation_stop":
		d.random.stop()
		d.playlists.stopAll()
		d.broadcast("ryogami.wall.random_stopped", map[string]interface{}{})
		return ok(req.ID, map[string]interface{}{"stopped": true})

	case "playlist.list":
		return ok(req.ID, d.playlists.snapshot())

	case "playlist.create":
		name := strParam(p, "name", "")
		if name == "" {
			return errResp(req.ID, 1, "missing 'name' parameter")
		}
		id := d.playlists.create(name)
		d.broadcast("ryogami.playlist.changed", map[string]interface{}{})
		return ok(req.ID, map[string]interface{}{"id": id})

	case "playlist.update":
		if !d.playlists.update(intParam(p, "id", 0), strParam(p, "field", ""), strParam(p, "value", "")) {
			return errResp(req.ID, 2, "unknown playlist or field")
		}
		d.broadcast("ryogami.playlist.changed", map[string]interface{}{})
		return ok(req.ID, map[string]interface{}{"ok": true})

	case "playlist.delete":
		d.playlists.delete(intParam(p, "id", 0))
		d.broadcast("ryogami.playlist.changed", map[string]interface{}{})
		return ok(req.ID, map[string]interface{}{"ok": true})

	case "playlist.members":
		return ok(req.ID, map[string]interface{}{"members": d.playlists.members(intParam(p, "id", 0))})

	case "playlist.memberships":
		return ok(req.ID, d.playlists.snapshot())

	case "playlist.add":
		if !d.playlists.addMember(intParam(p, "id", 0), strParam(p, "key", "")) {
			return errResp(req.ID, 2, "unknown playlist")
		}
		d.broadcast("ryogami.playlist.changed", map[string]interface{}{})
		return ok(req.ID, map[string]interface{}{"ok": true})

	case "playlist.remove":
		d.playlists.removeMember(intParam(p, "id", 0), strParam(p, "key", ""))
		d.broadcast("ryogami.playlist.changed", map[string]interface{}{})
		return ok(req.ID, map[string]interface{}{"ok": true})

	case "playlist.move":
		d.playlists.moveMember(intParam(p, "id", 0), strParam(p, "key", ""), intParam(p, "delta", 0))
		d.broadcast("ryogami.playlist.changed", map[string]interface{}{})
		return ok(req.ID, map[string]interface{}{"ok": true})

	case "playlist.assign":
		if !d.playlists.assign(strParam(p, "output", ""), intParam(p, "id", 0)) {
			return errResp(req.ID, 2, "unknown playlist")
		}
		d.broadcast("ryogami.playlist.changed", map[string]interface{}{})
		return ok(req.ID, map[string]interface{}{"ok": true})

	case "playlist.toggle":
		d.playlists.toggle(strParam(p, "output", ""), intParam(p, "id", 0))
		d.broadcast("ryogami.playlist.changed", map[string]interface{}{})
		return ok(req.ID, map[string]interface{}{"ok": true})

	case "playlist.stop":
		d.playlists.stop(intParam(p, "id", 0))
		d.broadcast("ryogami.playlist.changed", map[string]interface{}{})
		return ok(req.ID, map[string]interface{}{"ok": true})

	case "playlist.play_now":
		d.playlists.playNow(intParam(p, "id", 0))
		return ok(req.ID, map[string]interface{}{"ok": true})

	case "grade.preview":
		out, err := d.grader.Preview(strParam(p, "input", ""), gradeParamsFrom(p))
		if err != nil {
			return errResp(req.ID, 3, err.Error())
		}
		return ok(req.ID, map[string]interface{}{"output": out})

	case "grade.commit":
		out, err := d.grader.Commit(strParam(p, "input", ""), strParam(p, "output", ""), gradeParamsFrom(p))
		if err != nil {
			return errResp(req.ID, 3, err.Error())
		}
		go d.rescan(true)
		return ok(req.ID, map[string]interface{}{"output": out})

	case "upscale.start":
		if err := d.upscaler.Start(strParam(p, "input", ""), strParam(p, "kind", ""), int(intParam(p, "scale", defaultUpscaleScale))); err != nil {
			return errResp(req.ID, 3, err.Error())
		}
		return ok(req.ID, map[string]interface{}{"started": true})

	case "upscale.status":
		return ok(req.ID, d.upscaler.Status())

	case "upscale.cancel":
		d.upscaler.Cancel()
		return ok(req.ID, map[string]interface{}{"cancelled": true})

	default:
		if resp, handled := d.dispatchExtended(req, p); handled {
			return resp
		}
		return errResp(req.ID, -32601, fmt.Sprintf("unknown method: %s", req.Method))
	}
}

func nullable(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// muteParam and volumeParam pull wall.apply's per-output audio maps.
func muteParam(p map[string]interface{}) map[string]bool {
	out := map[string]bool{}
	if m, has := p["outputs_audio"].(map[string]interface{}); has {
		for k, v := range m {
			if b, isBool := v.(bool); isBool {
				out[k] = b
			}
		}
	}
	return out
}

func volumeParam(p map[string]interface{}) map[string]int {
	out := map[string]int{}
	if m, has := p["outputs_volume"].(map[string]interface{}); has {
		for k, v := range m {
			if n, isNum := v.(float64); isNum {
				out[k] = clampVolume(int(n))
			}
		}
	}
	return out
}

// subParams pulls the nested params object an effects request carries.
func subParams(p map[string]interface{}) map[string]interface{} {
	if m, has := p["params"].(map[string]interface{}); has {
		return m
	}
	return map[string]interface{}{}
}
