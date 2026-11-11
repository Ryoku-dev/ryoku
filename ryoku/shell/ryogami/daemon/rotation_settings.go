package main

import "errors"

// Rotation follows the settings rather than an explicit start/stop RPC.

var errNoRandomSource = errors.New("select at least one random source category in settings")

func init() {
	onStart(func(d *daemon) { d.syncRandomRotation() })
	watchSetting("general.random", func(d *daemon, _ string, _ interface{}) { d.syncRandomRotation() })
	registerAction("randomRotate", func(d *daemon, _ map[string]interface{}) (interface{}, error) {
		return d.toggleRandomRotation()
	})
}

// rotationActive is the single auto-rotate state the picker shows: the random
// loop or any playlist bound to an output.
func (d *daemon) rotationActive() bool {
	running, _ := d.random.status()["running"].(bool)
	return running || d.playlists.anyAssigned()
}

// toggleRandomRotation backs the picker's auto-rotate button and its keybind. It
// flips the saved switch, so rotation started either way resumes after a restart
// with the saved interval and pool. Off also stops assigned playlists, the way
// wall.rotation_stop does.
func (d *daemon) toggleRandomRotation() (interface{}, error) {
	if d.rotationActive() {
		if err := d.setSetting("general.randomRotate", false); err != nil {
			return nil, err
		}
		d.random.stop()
		d.playlists.stopAll()
		d.broadcast("ryogami.wall.random_stopped", map[string]interface{}{})
		return map[string]interface{}{"active": false}, nil
	}
	if types, _ := d.randomPoolConfig(); len(types) == 0 {
		return nil, errNoRandomSource
	}
	if err := d.setSetting("general.randomRotate", true); err != nil {
		return nil, err
	}
	return map[string]interface{}{"active": d.rotationActive()}, nil
}

// Shared with the schedule's random target so both draw from the pool the user configured.
func (d *daemon) randomPoolConfig() (types []string, favOnly bool) {
	if d.settingBool("general.randomIncludeStatic") {
		types = append(types, "static")
	}
	if d.settingBool("general.randomIncludeVideo") {
		types = append(types, "video")
	}
	if d.settingBool("general.randomIncludeWE") {
		types = append(types, "we")
	}
	favOnly = d.settingBool("general.randomIncludeFavourites")
	return types, favOnly
}

func (d *daemon) syncRandomRotation() {
	if !d.settingBool("general.randomRotate") {
		d.stopRandomRotation()
		return
	}
	types, favOnly := d.randomPoolConfig()
	if len(types) == 0 {
		d.stopRandomRotation()
		return
	}
	interval := int64(d.settingNumber("general.randomInterval"))
	if interval < 10 {
		interval = 10
	}
	d.random.start(interval, types, favOnly, func() { d.randomPick(types, favOnly) })
	d.broadcast("ryogami.wall.random_started", map[string]interface{}{
		"interval": interval, "types": types, "favourites_only": favOnly,
	})
}

func (d *daemon) stopRandomRotation() {
	if d.random.stop() {
		d.broadcast("ryogami.wall.random_stopped", map[string]interface{}{})
	}
}
