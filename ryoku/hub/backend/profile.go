package main

// The Profile page's share backend. profile.json (user-owned, hand-editable, a
// preference like decor.json) stays the source of truth; a custom hero image
// lives beside it in profile/. "export" packs both into one self-contained,
// portable .ryoprofile envelope (the hero base64-embedded so there is no archive
// tooling), and "import" validates that envelope and unpacks it back, applying
// nothing when the file is not a valid .ryoprofile.

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func profileDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "ryoku")
}

func profileConfigPath() string { return filepath.Join(profileDir(), "profile.json") }

func profileHeroDir() string { return filepath.Join(profileDir(), "profile") }

// profileModel peeks at just the hero fields export needs; the rest of the
// document is carried verbatim (see exportProfile) so nothing is lost or
// reformatted on a round-trip.
type profileModel struct {
	Hero struct {
		Kind   string `json:"kind"`
		Source string `json:"source"`
	} `json:"hero"`
}

// ryoHero is the embedded custom hero image inside an envelope: the bare file
// name plus the base64 of its raw bytes.
type ryoHero struct {
	Name string `json:"name"`
	B64  string `json:"b64"`
}

// ryoEnvelope is the .ryoprofile file: a version tag, the profile.json object
// verbatim, and (only for a custom hero) the packed image, else null.
type ryoEnvelope struct {
	Ryoprofile int             `json:"ryoprofile"`
	Profile    json.RawMessage `json:"profile"`
	Hero       *ryoHero        `json:"hero"`
}

// exportProfile packs profile.json (and a custom hero image, if any) into a
// single .ryoprofile envelope at dstPath. A missing profile.json is treated as
// an empty object.
func exportProfile(dstPath string) error {
	raw, err := os.ReadFile(profileConfigPath())
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		raw = []byte("{}")
	}
	var m profileModel
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("profile.json is not valid JSON: %w", err)
	}
	env := ryoEnvelope{Ryoprofile: 1, Profile: json.RawMessage(raw)}
	if m.Hero.Kind == "custom" && m.Hero.Source != "" {
		name := filepath.Base(m.Hero.Source)
		if img, err := os.ReadFile(filepath.Join(profileHeroDir(), name)); err == nil {
			env.Hero = &ryoHero{Name: name, B64: base64.StdEncoding.EncodeToString(img)}
		}
	}
	out, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return atomicWrite(dstPath, out, 0o644)
}

// importProfile validates a .ryoprofile at srcPath and unpacks it: the embedded
// hero image into profile/, then the settings into profile.json. Everything is
// decoded and validated before any write, so a parse/validation failure leaves
// the existing profile untouched.
func importProfile(srcPath string) error {
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	var env ryoEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("not a valid .ryoprofile file: %w", err)
	}
	if env.Ryoprofile != 1 {
		return fmt.Errorf("not a .ryoprofile file (ryoprofile != 1)")
	}
	var heroBytes []byte
	if env.Hero != nil {
		if env.Hero.Name == "" {
			return fmt.Errorf(".ryoprofile hero is missing a name")
		}
		heroBytes, err = base64.StdEncoding.DecodeString(env.Hero.B64)
		if err != nil {
			return fmt.Errorf(".ryoprofile hero image is not valid base64: %w", err)
		}
	}
	profile := env.Profile
	if len(profile) == 0 {
		profile = []byte("{}")
	}
	paths := []string{profileConfigPath()}
	heroPath := ""
	if env.Hero != nil {
		heroPath = filepath.Join(profileHeroDir(), filepath.Base(env.Hero.Name))
		paths = append(paths, heroPath)
	}
	before, err := snapshotFiles(paths...)
	if err != nil {
		return err
	}
	if heroPath != "" {
		if err := atomicWrite(heroPath, heroBytes, 0o644); err != nil {
			return err
		}
	}
	if err := atomicWrite(profileConfigPath(), profile, 0o644); err != nil {
		if rerr := before.restore(); rerr != nil {
			return errors.Join(err, fmt.Errorf("restore profile: %w", rerr))
		}
		return err
	}
	return nil
}

func runProfile(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("profile needs export|import")
	}
	switch args[0] {
	case "export":
		if len(args) < 2 {
			return fmt.Errorf("profile export needs a path")
		}
		return exportProfile(args[1])
	case "import":
		if len(args) < 2 {
			return fmt.Errorf("profile import needs a path")
		}
		return importProfile(args[1])
	default:
		return fmt.Errorf("unknown profile subcommand: %s", args[0])
	}
}
