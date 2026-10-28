package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config = Hub's persisted state. TOML at ~/.config/ryoku/hub.toml. sections
// add their own typed tables here as they graduate from "under construction";
// for now the UI table just remembers the last open section so the Hub
// reopens where you left it.
type Config struct {
	UI UIConfig `toml:"ui"`
}

type UIConfig struct {
	Section        string `toml:"section"`
	UpdateInterval string `toml:"update_interval"`
	UpdateSystem   string `toml:"update_system"`
	Advanced       string `toml:"advanced"`
}

func defaultConfig() Config {
	return Config{UI: UIConfig{Section: "displays", UpdateInterval: "daily", UpdateSystem: "off"}}
}

func configPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "ryoku", "hub.toml")
}

func normalizeSection(section string) string {
	if section == "bar" || section == "frame" {
		return "bar-studio"
	}
	return section
}

func loadConfig() Config {
	c := defaultConfig()
	if b, err := os.ReadFile(configPath()); err == nil {
		_ = toml.Unmarshal(b, &c)
	}
	if c.UI.Section != normalizeSection(c.UI.Section) {
		c.UI.Section = normalizeSection(c.UI.Section)
		_ = saveConfig(c)
	}
	return c
}

// saveConfig writes atomically (temp file + rename) so a crash mid-write can't
// leave a half TOML the next load would silently reset.
func saveConfig(c Config) error {
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(p), "hub-*.toml")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if err := toml.NewEncoder(f).Encode(c); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, p)
}

func configGet(key string) (string, bool) {
	c := loadConfig()
	switch key {
	case "section":
		return c.UI.Section, true
	case "update_interval":
		return c.UI.UpdateInterval, true
	case "update_system":
		return c.UI.UpdateSystem, true
	case "advanced":
		return c.UI.Advanced, true
	}
	return "", false
}

func configSet(key, value string) error {
	c := loadConfig()
	switch key {
	case "section":
		c.UI.Section = normalizeSection(value)
	case "update_interval":
		c.UI.UpdateInterval = value
	case "update_system":
		if value != "on" && value != "off" {
			return fmt.Errorf("update_system takes on or off, not %q", value)
		}
		c.UI.UpdateSystem = value
	case "advanced":
		c.UI.Advanced = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
	return saveConfig(c)
}
