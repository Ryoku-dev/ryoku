package main

import (
	"fmt"
	"os"
)

// nil means every apply under the wallpaper policy re-asserts follow.
var themeSourceHook func(d *daemon, ev applyEvent) (image string, retheme bool)

func init() {
	// The matugen knobs reach the shell through their own Hub write, so they are not watched here.
	watchSetting("theme.policy", func(d *daemon, key string, _ interface{}) {
		d.applyThemePolicy()
	})
	watchSetting("theme.staticTheme", func(d *daemon, key string, _ interface{}) {
		if d.settingString("theme.policy") == "fixed" {
			d.applyThemePolicy()
		}
	})
	watchSetting("theme.customColors", func(d *daemon, key string, _ interface{}) {
		if d.settingString("theme.policy") == "fixed" && d.settingString("theme.staticTheme") == "custom" {
			d.applyThemePolicy()
		}
	})
	afterApply(rethemeAfterApply)
}

func (d *daemon) applyThemePolicy() {
	switch d.settingString("theme.policy") {
	case "wallpaper":
		shellTheme("Wallpaper")
		shellColours("wallpaper")
	case "fixed":
		name := d.settingString("theme.staticTheme")
		if name == "custom" {
			id, err := d.buildCustomSeedTheme(d.settingString("theme.customColors"))
			if err != nil {
				fmt.Fprintf(os.Stderr, "ryogami: theme custom seed: %v\n", err)
				return
			}
			name = id
		}
		shellTheme(name)
		shellColours("wallpaper")
	case "off":
		shellColours("ryoku")
		setThemeFollow(false)
	}
}

// The shell re-tints on every switch; this only re-asserts follow after something moved it off.
func rethemeAfterApply(d *daemon, ev applyEvent) {
	if d.settingString("theme.policy") != "wallpaper" {
		return
	}
	if themeSourceHook != nil {
		if _, retheme := themeSourceHook(d, ev); !retheme {
			return
		}
	}
	name := shellThemeName()
	if (name == "" || name == "Wallpaper" || name == "Default") && followWallpaperOn() {
		return
	}
	shellTheme("Wallpaper")
}
