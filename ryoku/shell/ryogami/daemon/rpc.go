package main

import "fmt"

const daemonVersion = "0.2.0"

// dispatchRequest routes one JSON-RPC request. The method set and response
// shapes are the wire contract the wall-ui picker parses; unimplemented
// subsystems (effects, optimize, video_convert, analysis, steam) answer with
// the standard unknown-method error, which the picker's default feature set
// never triggers.
func (d *daemon) dispatchRequest(req *request) response {
	p := req.params()
	switch req.Method {
	case "status":
		return ok(req.ID, map[string]interface{}{
			"version":           daemonVersion,
			"current_wallpaper": nullable(d.currentName()),
		})

	// wm.caps lets the wall-ui gate compositor affordances on capability rather
	// than a compositor name.
	case "wm.caps":
		caps, _ := wmClient.Caps()
		return ok(req.ID, caps)

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
		wpType := strParam(p, "type", "static")
		if wpType != "static" && wpType != "video" {
			return errResp(req.ID, 1, fmt.Sprintf("unsupported type: %s", wpType))
		}
		if err := d.applyWallpaper(wpType, strParam(p, "path", ""), "set", strsParam(p, "outputs"), muteParam(p), volumeParam(p)); err != nil {
			return errResp(req.ID, 4, err.Error())
		}
		return ok(req.ID, map[string]interface{}{"applied": d.currentName()})

	case "wall.restore":
		d.restoreOutputs()
		return ok(req.ID, map[string]interface{}{"ok": true})

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

	case "optimize.start", "video_convert.start":
		kind := "optimize"
		if req.Method == "video_convert.start" {
			kind = "convert"
		}
		if err := d.optimizer.Start(kind, strParam(p, "preset", "balanced"), strParam(p, "resolution", "4k")); err != nil {
			return errResp(req.ID, 3, err.Error())
		}
		return ok(req.ID, map[string]interface{}{"started": true})

	case "optimize.cancel", "video_convert.cancel":
		kind := "optimize"
		if req.Method == "video_convert.cancel" {
			kind = "convert"
		}
		d.optimizer.Cancel(kind)
		return ok(req.ID, map[string]interface{}{"cancelled": true})

	case "optimize.status", "video_convert.status":
		kind := "optimize"
		if req.Method == "video_convert.status" {
			kind = "convert"
		}
		return ok(req.ID, d.optimizer.Status(kind))

	case "optimize.presets", "video_convert.presets":
		kind := "optimize"
		if req.Method == "video_convert.presets" {
			kind = "convert"
		}
		return ok(req.ID, map[string]interface{}{"presets": d.optimizer.Presets(kind)})

	case "wall.set_favourite":
		key := strParam(p, "key", "")
		fav := 0
		if boolParam(p, "favourite", false) {
			fav = 1
		}
		if !d.store.mutate(key, func(e *Entry) { e.Favourite = fav }) {
			return errResp(req.ID, 2, fmt.Sprintf("unknown wallpaper: %s", key))
		}
		return ok(req.ID, map[string]interface{}{"ok": true})

	case "wall.update_metadata":
		key := strParam(p, "key", "")
		d.store.mutate(key, func(e *Entry) {
			if v := intParam(p, "filesize", 0); v > 0 {
				e.Filesize = v
			}
			if v := intParam(p, "width", 0); v > 0 {
				e.Width = int(v)
			}
			if v := intParam(p, "height", 0); v > 0 {
				e.Height = int(v)
			}
		})
		return ok(req.ID, map[string]interface{}{"ok": true})

	case "wall.delete":
		if err := d.deleteWallpaper(strParam(p, "key", "")); err != nil {
			return errResp(req.ID, 2, err.Error())
		}
		return ok(req.ID, map[string]interface{}{"ok": true})

	case "wall.import":
		if err := d.importWallpaper(strParam(p, "path", "")); err != nil {
			return errResp(req.ID, 2, err.Error())
		}
		return ok(req.ID, map[string]interface{}{"ok": true})

	case "wall.outputs":
		return ok(req.ID, map[string]interface{}{"outputs": d.outputsState()})

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

	case "wall.cache_rebuild":
		go d.rescan(true)
		return ok(req.ID, map[string]interface{}{"started": true})

	case "wall.cache_reset":
		go d.resetCache()
		return ok(req.ID, map[string]interface{}{"started": true})

	case "wall.recompute_colors":
		go d.rescan(true)
		return ok(req.ID, map[string]interface{}{"started": true})

	case "wall.cache_status":
		return ok(req.ID, map[string]interface{}{"ready": true, "count": len(d.store.list(false))})

	case "wall.clear_video_cache":
		removed, freed := pruneLivewallCache(int(intParam(p, "days", 0)))
		return ok(req.ID, map[string]interface{}{"removed": removed, "freed": freed})

	case "wall.toggle":
		if d.ui.ensure() {
			d.broadcast("ryogami.wall.toggle", map[string]interface{}{})
		}
		return ok(req.ID, map[string]interface{}{"toggled": true})

	case "wall.show":
		if d.ui.ensure() {
			d.broadcast("ryogami.wall.show", map[string]interface{}{})
		}
		return ok(req.ID, map[string]interface{}{"ok": true})

	case "wall.hide":
		d.broadcast("ryogami.wall.hide", map[string]interface{}{})
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

	case "wall.random_stop":
		d.random.stop()
		d.broadcast("ryogami.wall.random_stopped", map[string]interface{}{})
		return ok(req.ID, map[string]interface{}{"stopped": true})

	case "wall.random_status":
		return ok(req.ID, d.random.status())

	// Day/night video rotation (#247): the picker configures the pools and the
	// interval; the daemon owns the timer and the day/night decision (read from
	// the shell's weather isDay). start reads the config fresh so a GUI Save is
	// the only writer; force pins a phase for manual testing.
	case "wall.daynight_start":
		cfg := dayNightFromWall()
		d.daynight.start(cfg, d.dayNightTick, nil)
		d.broadcast("ryogami.wall.daynight_started", map[string]interface{}{
			"interval": cfg.Interval, "day_dir": cfg.DayDir, "night_dir": cfg.NightDir,
		})
		return ok(req.ID, map[string]interface{}{"started": true})

	case "wall.daynight_stop":
		d.daynight.stop()
		d.broadcast("ryogami.wall.daynight_stopped", map[string]interface{}{})
		return ok(req.ID, map[string]interface{}{"stopped": true})

	case "wall.daynight_status":
		return ok(req.ID, d.daynight.status())

	case "wall.daynight_force":
		d.daynight.force(strParam(p, "phase", ""))
		return ok(req.ID, d.daynight.status())

	// Unified auto-rotate: active if the random loop is running, any playlist
	// is assigned, or day/night rotation is up (each rotates independently of
	// the others). rotation_stop halts all three, so "auto-rotate off" truly
	// stops every wallpaper change.
	case "wall.rotation_status":
		st := d.random.status()
		running, _ := st["running"].(bool)
		dn, _ := d.daynight.status()["running"].(bool)
		return ok(req.ID, map[string]interface{}{"active": running || dn || d.playlists.anyAssigned()})

	case "wall.rotation_stop":
		d.random.stop()
		d.daynight.stop()
		d.playlists.stopAll()
		d.broadcast("ryogami.wall.random_stopped", map[string]interface{}{})
		return ok(req.ID, map[string]interface{}{"stopped": true})

	// Palette frame: which second of a video clip the still (and so the matugen
	// palette the shell derives) is sampled from. With a "frame" param it
	// persists the second and re-applies the current wallpaper so the new still
	// is painted and republished; the shell's bridge re-runs matugen off it.
	// Without a param it reports the current second.
	case "wall.palette_frame":
		if _, has := p["frame"]; has {
			sec := floatParam(p, "frame", 1)
			if sec < 0 {
				sec = 0
			}
			persistVideoFrame(sec)
			d.reloadConfig()
			d.restoreOutputs()
			d.broadcast("ryogami.wall.palette_frame", map[string]interface{}{"frame": sec})
			return ok(req.ID, map[string]interface{}{"frame": sec})
		}
		return ok(req.ID, map[string]interface{}{"frame": d.config().videoFrame()})

	case "playlist.list", "pl.list":
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

	case "playlist.members", "pl.contents":
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
