// Command ryoku-manifest generates the release control manifest from a Ryoku
// checkout. build-repo.sh and fedora's build-rpm-repo.sh run it at publish time
// and serve the result beside the channel's release.json; nothing else writes
// it, and no one edits it.
//
// Usage: ryoku-manifest -repo <checkout> -release <str> -version <pkgver>
//
//	-commit <sha> -channel <stable|testing|v<tag>> [-edition fedora]
//	[-choices <path>] -out <file>
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ryoku-cli/internal/host"
	"ryoku-cli/internal/ryokumanifest"
)

const installerLane = "desktop"

func main() {
	repo := flag.String("repo", ".", "ryoku checkout to read")
	release := flag.String("release", "unknown", "release string")
	version := flag.String("version", "", "shared RYOKU_PKGVER the first-party set is pinned to")
	commit := flag.String("commit", "unknown", "full commit SHA")
	channel := flag.String("channel", "stable", "channel this manifest is published to")
	edition := flag.String("edition", "", "translate for an edition's own repository (fedora); empty for Arch")
	choices := flag.String("choices", "", "installer choices TSV (default: <repo>/installation/installerplan/choices.tsv)")
	out := flag.String("out", "", "manifest.json to write (default: stdout)")
	flag.Parse()

	choicesPath := *choices
	if choicesPath == "" {
		choicesPath = filepath.Join(*repo, "installation/installerplan/choices.tsv")
	}
	choicePackages, err := ryokumanifest.ChoicePackages(choicesPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ryoku-manifest:", err)
		os.Exit(1)
	}
	in, err := readInputs(*repo, *release, *version, *commit, *channel)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ryoku-manifest:", err)
		os.Exit(1)
	}
	in.Choices = choicePackages
	m := ryokumanifest.Build(in)
	if *edition != "" {
		if m, err = translate(m, *repo, *edition); err != nil {
			fmt.Fprintln(os.Stderr, "ryoku-manifest:", err)
			os.Exit(1)
		}
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "ryoku-manifest:", err)
		os.Exit(1)
	}
	b = append(b, '\n')
	if *out == "" {
		os.Stdout.Write(b)
		return
	}
	if err := os.WriteFile(*out, b, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "ryoku-manifest:", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "ryoku-manifest: %d names (%d base, %d first-party, %d AUR, %d provisioned)\n",
		len(m.Names()), len(m.Base), len(m.FirstParty), len(m.AUR), len(m.Provisioned))
}

func readInputs(repo, release, version, commit, channel string) (ryokumanifest.Inputs, error) {
	read := func(rel string) (string, error) {
		b, err := os.ReadFile(filepath.Join(repo, rel))
		return string(b), err
	}
	in := ryokumanifest.Inputs{
		Release: release, Version: version, Commit: commit, Channel: channel,
		Date: time.Now().UTC().Format(time.RFC3339),
	}
	var err error
	if in.Base, err = read("system/packages/base.packages"); err != nil {
		return in, fmt.Errorf("base.packages: %w", err)
	}
	if in.Dev, err = read("system/packages/dev.packages"); err != nil {
		return in, fmt.Errorf("dev.packages: %w", err)
	}
	if in.Hardware, err = read("system/packages/hardware.packages"); err != nil {
		return in, fmt.Errorf("hardware.packages: %w", err)
	}
	if in.AUR, err = read("system/packages/aur.packages"); err != nil {
		return in, fmt.Errorf("aur.packages: %w", err)
	}
	if in.CachyOSReplaces, err = read("system/packages/cachyos.replaces"); err != nil {
		return in, fmt.Errorf("cachyos.replaces: %w", err)
	}
	if in.FirstParty, err = readDirBodies(filepath.Join(repo, "release/packages"), "PKGBUILD"); err != nil {
		return in, fmt.Errorf("release/packages: %w", err)
	}
	// Each compositor provider declares its own packages in its caps.go, so the
	// manifest carries the same list the switch and reclaim already trust.
	if in.Compositor, err = readDirBodies(filepath.Join(repo, "ryoku/wm"), "caps.go"); err != nil {
		return in, fmt.Errorf("ryoku/wm: %w", err)
	}
	return in, nil
}

func translate(m ryokumanifest.Manifest, repo, edition string) (ryokumanifest.Manifest, error) {
	path := filepath.Join(repo, edition, "packages", "translations.tsv")
	rows, err := host.ReadPackageTranslations(path, edition)
	if err != nil {
		return m, err
	}
	table := make(map[string]ryokumanifest.Translation, len(rows))
	for name, row := range rows {
		table[name] = ryokumanifest.Translation{Names: row.Names, Lanes: row.Lanes}
	}
	m, err = ryokumanifest.Translate(m, table, installerLane)
	if err != nil {
		return m, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}

// readDirBodies reads <dir>/<entry>/<name> for every subdirectory, skipping
// entries that lack the file (a README in the tree must not fail the run).
func readDirBodies(dir, name string) (map[string]string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name(), name))
		if err != nil {
			continue
		}
		body := string(b)
		if !strings.Contains(body, "pkgname=") && !strings.Contains(body, "compositorPackages") {
			continue
		}
		out[e.Name()] = body
	}
	return out, nil
}
