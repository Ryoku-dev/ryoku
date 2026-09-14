package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSavedSnapCurveAcrossPresets(t *testing.T) {
	lua, err := exec.LookPath("lua")
	if err != nil {
		t.Skip("lua is required to exercise the shipped preset loader")
	}
	root, err := filepath.Abs("../../hyprland")
	if err != nil {
		t.Fatal(err)
	}
	presets, err := filepath.Glob(filepath.Join(root, "modules/animations/*.lua"))
	if err != nil || len(presets) == 0 {
		t.Fatalf("find animation presets: %v", err)
	}
	for _, preset := range presets {
		name := strings.TrimSuffix(filepath.Base(preset), ".lua")
		t.Run(name, func(t *testing.T) {
			config := t.TempDir()
			if err := os.Symlink(root, filepath.Join(config, "hypr")); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(config, "ryoku"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(config, "ryoku/anim-preset"), []byte(name), 0600); err != nil {
				t.Fatal(err)
			}
			o := defaultOverrides()
			o.Anim.Items = []AnimItem{{Leaf: "windows", Enabled: true, Speed: 3, Bezier: "snap", Style: "popin 80%"}}
			cmd := exec.Command(lua, "-")
			cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+config, "LUA_PATH="+root+"/?.lua;;")
			cmd.Stdin = strings.NewReader(`
local curves = { default = true, linear = true }
hl = {
    config = function() end,
    curve = function(name, curve) curves[name] = curve end,
    animation = function(item)
        assert(item.bezier == nil or curves[item.bezier], "no such bezier: " .. tostring(item.bezier))
    end,
}
require("modules.animations")
` + genAnimBlock(o) + `
local custom = { type = "bezier", points = { { 0.2, 0.8 }, { 0.3, 1 } } }
hl.curve("snap", custom)
assert(curves.snap == custom, "user curve must win")
`)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("load preset with saved snap override: %v\n%s", err, out)
			}
		})
	}
}
