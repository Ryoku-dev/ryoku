package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ryoku-cli/internal/sys"
	i18n "ryoku-i18n"
)

// plugin_new.go is `ryoku plugin new`: it scaffolds R1's plugin folder from the
// embedded template, generates a manifest for the chosen host, git-inits the
// folder and commits it as the author, then prints the next steps. The template
// is a complete, working demo (a ticking counter, a mark, a panel with a button)
// that passes `ryoku plugin validate` with zero findings.

//go:embed plugin_template
var pluginTemplate embed.FS

const templateRoot = "plugin_template"

func cmdPluginNew(args []string) error {
	id, name, author, to := "", "", "", ""
	host := ""
	hostFlags := 0
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--bar":
			host, hostFlags = "topbarGlyph", hostFlags+1
		case a == "--desktop":
			host, hostFlags = "desktopWidget", hostFlags+1
		case a == "--popout":
			host, hostFlags = "framePopout", hostFlags+1
		case a == "--name":
			i++
			if i >= len(args) {
				return fmt.Errorf(i18n.T("--name needs a value"))
			}
			name = args[i]
		case strings.HasPrefix(a, "--name="):
			name = strings.TrimPrefix(a, "--name=")
		case a == "--author":
			i++
			if i >= len(args) {
				return fmt.Errorf(i18n.T("--author needs a value"))
			}
			author = args[i]
		case strings.HasPrefix(a, "--author="):
			author = strings.TrimPrefix(a, "--author=")
		case a == "--to":
			i++
			if i >= len(args) {
				return fmt.Errorf(i18n.T("--to needs a value"))
			}
			to = args[i]
		case strings.HasPrefix(a, "--to="):
			to = strings.TrimPrefix(a, "--to=")
		case strings.HasPrefix(a, "-"):
			return fmt.Errorf(i18n.T("unknown flag %q"), a)
		default:
			if id != "" {
				return fmt.Errorf(i18n.T("give one plugin id"))
			}
			id = a
		}
	}
	if id == "" {
		return fmt.Errorf(i18n.T("usage: ryoku plugin new <id> [--bar|--desktop|--popout] [--name N] [--author \"N <m>\"] [--to <dir>]"))
	}
	if hostFlags > 1 {
		return fmt.Errorf(i18n.T("choose one of --bar, --desktop, --popout"))
	}
	if host == "" {
		host = "topbarGlyph" // default --bar
	}
	if !pluginIDRe.MatchString(id) {
		return fmt.Errorf(i18n.T("id %q must be lowercase letters, digits and dashes, not starting with a dash"), id)
	}
	if reservedIDs()[id] {
		return fmt.Errorf(i18n.T("%q is a reserved built-in widget id; choose another"), id)
	}
	if name == "" {
		name = titleFromID(id)
	}
	if author == "" {
		author = defaultAuthor()
	}
	dir := to
	if dir == "" {
		dir = filepath.Join(pluginAuthorRoot(), id)
	}
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return fmt.Errorf(i18n.T("%s already exists and is not empty"), dir)
	}

	bar := host == "topbarGlyph"
	if err := scaffoldPlugin(dir, id, name, author, host, bar); err != nil {
		return err
	}
	gitInitPlugin(dir, author)
	printNextSteps(id, dir, bar)
	return nil
}

// scaffoldPlugin lays the template into dir: the generated manifest, the copied
// template files (with placeholders filled), and an empty assets/ dir. The panel
// file is copied only for the bar host.
func scaffoldPlugin(dir, id, name, author, host string, bar bool) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	authorName, _ := splitAuthor(author)
	repl := strings.NewReplacer(
		"{{ID}}", id,
		"{{NAME}}", name,
		"{{AUTHOR}}", author,
		"{{AUTHORNAME}}", authorName,
		"{{YEAR}}", fmt.Sprint(time.Now().Year()),
		"{{HOST}}", host,
	)

	err := fs.WalkDir(pluginTemplate, templateRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == templateRoot {
			return nil
		}
		rel := strings.TrimPrefix(p, templateRoot+"/")
		if !bar && rel == "content/Panel.qml" {
			return nil
		}
		dst := filepath.Join(dir, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if rel == "gitignore" {
			dst = filepath.Join(dir, ".gitignore")
		}
		b, e := pluginTemplate.ReadFile(p)
		if e != nil {
			return e
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dst, []byte(repl.Replace(string(b))), 0o644)
	})
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		return err
	}
	return writeManifest(dir, id, name, author, host, bar)
}

// writeManifest generates manifest.json for the chosen host: entryPoints (with
// panel and a panel width only for the bar), the density set, an example
// setting, and honest metadata (official false, the author as Name <mail>).
func writeManifest(dir, id, name, author, host string, bar bool) error {
	type entryPoints struct {
		Main    string `json:"main"`
		Content string `json:"content"`
		Panel   string `json:"panel,omitempty"`
	}
	type panelSpec struct {
		Width int `json:"width"`
	}
	type capSpec struct {
		Densities []string `json:"densities"`
	}
	type depSpec struct {
		Commands []string `json:"commands"`
	}
	type settingSpec struct {
		Key     string `json:"key"`
		Type    string `json:"type"`
		Label   string `json:"label"`
		Group   string `json:"group"`
		Default any    `json:"default"`
	}
	type metaSpec struct {
		Settings []settingSpec `json:"settings"`
	}
	type manifestOut struct {
		ID           string         `json:"id"`
		Name         string         `json:"name"`
		Version      string         `json:"version"`
		Author       string         `json:"author"`
		Description  string         `json:"description"`
		License      string         `json:"license"`
		Official     bool           `json:"official"`
		Hosts        []string       `json:"hosts"`
		EntryPoints  entryPoints    `json:"entryPoints"`
		Panel        *panelSpec     `json:"panel,omitempty"`
		Files        []string       `json:"files"`
		Capabilities capSpec        `json:"capabilities"`
		Defaults     map[string]any `json:"defaults"`
		Dependencies depSpec        `json:"dependencies"`
		Metadata     metaSpec       `json:"metadata"`
	}

	m := manifestOut{
		ID:           id,
		Name:         name,
		Version:      "0.1.0",
		Author:       author,
		Description:  name + " is a Ryoku shell plugin.",
		License:      "MIT",
		Official:     false,
		Hosts:        []string{host},
		EntryPoints:  entryPoints{Main: "service/Main.qml", Content: "content/Widget.qml"},
		Files:        []string{},
		Capabilities: capSpec{Densities: densitiesFor(host)},
		Defaults:     map[string]any{"host": host, "label": name},
		Dependencies: depSpec{Commands: []string{}},
		Metadata: metaSpec{Settings: []settingSpec{
			{Key: "showCount", Type: "toggle", Label: "Show the tick count", Group: name, Default: true},
		}},
	}
	if bar {
		m.EntryPoints.Panel = "content/Panel.qml"
		m.Panel = &panelSpec{Width: 320}
	}

	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "manifest.json"), append(b, '\n'), 0o644)
}

func densitiesFor(host string) []string {
	switch host {
	case "topbarGlyph":
		return []string{"glyph", "compact"}
	case "desktopWidget":
		return []string{"compact", "full"}
	default:
		return []string{"compact"}
	}
}

// pluginAuthorRoot is R1's authoring root: the XDG DOCUMENTS dir's ryoku-plugins
// folder, falling back to ~/ryoku-plugins.
func pluginAuthorRoot() string {
	if sys.Has("xdg-user-dir") {
		if out, err := sys.RunOut("xdg-user-dir", "DOCUMENTS"); err == nil {
			d := strings.TrimSpace(out)
			if d != "" && d != sys.Home() {
				return filepath.Join(d, "ryoku-plugins")
			}
		}
	}
	return filepath.Join(sys.Home(), "ryoku-plugins")
}

// titleFromID turns a plugin id into a display name: demo-x -> "Demo X".
func titleFromID(id string) string {
	parts := strings.Split(id, "-")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

// defaultAuthor resolves the author when --author is absent: git's configured
// identity, then $USER, so a fresh plugin still carries an honest author (R11).
func defaultAuthor() string {
	name, _ := gitOut(".", "config", "user.name")
	mail, _ := gitOut(".", "config", "user.email")
	name, mail = strings.TrimSpace(name), strings.TrimSpace(mail)
	switch {
	case name != "" && mail != "":
		return fmt.Sprintf("%s <%s>", name, mail)
	case name != "":
		return name
	}
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "ryoku"
}

// gitInitPlugin makes the folder a git repo with one commit authored by the
// plugin author. Best-effort: no git, no repo, but the folder is still valid.
func gitInitPlugin(dir, author string) {
	if !sys.Has("git") {
		return
	}
	_ = gitIn(dir, "init", "-q")
	_ = gitIn(dir, "add", "-A")
	_ = gitCommitAs(dir, author, "Initial plugin scaffold")
}

func printNextSteps(id, dir string, bar bool) {
	fmt.Printf(i18n.T("%s %s at %s\n\n"), sys.Green(i18n.T("scaffolded")), id, dir)
	fmt.Println(i18n.T("Next steps:"))
	edit := i18n.T("  1. edit: service/Main.qml, content/Widget.qml")
	if bar {
		edit += ", content/Panel.qml"
	}
	fmt.Println(edit)
	fmt.Println(i18n.T("  2. capture assets/preview-widget.png (see README.md), then list it in manifest files"))
	fmt.Printf(i18n.T("  3. check:   ryoku plugin validate %s\n"), dir)
	if bar {
		fmt.Printf(i18n.T("  4. install: ryoku plugin add %s --bar --yes\n"), dir)
		fmt.Println(i18n.T("     then find it on the bar, and under QS Bar Settings > Community"))
	} else {
		fmt.Printf(i18n.T("  4. install: ryoku plugin add %s --yes\n"), dir)
	}
	fmt.Printf(i18n.T("  5. publish: ryoku plugin share %s   (only when you want to share it)\n"), id)
}
