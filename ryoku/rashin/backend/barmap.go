package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

// barmap.go renders desktop.md's fenced "Bar and dock" section: the shell items
// listed in shells.json, QS Bar's live widget catalogue while it is the active
// bar style, and the dock commands.

// shellsIndex is the part of ryostore's shells.json this section reads.
type shellsIndex struct {
	Active string            `json:"active"`
	Items  []shellsIndexItem `json:"items"`
}

type shellsIndexItem struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Source   string `json:"source"`
	Name     string `json:"name"`
	Settings struct {
		IPC  string   `json:"ipc"`
		Call string   `json:"call"`
		Args []string `json:"args"`
	} `json:"settings"`
}

func shellsIndexPath() string {
	return filepath.Join(stateHome(), "ryoku", "store", "shells.json")
}

// loadShellsIndex returns nil when shells.json is absent or unreadable.
func loadShellsIndex() *shellsIndex {
	var index shellsIndex
	if json.Unmarshal([]byte(readFileOrEmpty(shellsIndexPath())), &index) != nil {
		return nil
	}
	return &index
}

// itemSettingsCommand is the command the Hub runs to open an item's settings.
// It only answers while that item is the active bar style.
func itemSettingsCommand(item shellsIndexItem) string {
	if item.Settings.IPC == "" || item.Settings.Call == "" {
		return "-"
	}
	parts := []string{"ryoku-shell", "ipc", "call", item.Settings.IPC, item.Settings.Call}
	for _, a := range item.Settings.Args {
		if a == "" {
			a = `""`
		}
		parts = append(parts, a)
	}
	return "`" + strings.Join(parts, " ") + "`"
}

// barWidget mirrors one row of QS Bar's `catalog` IPC reply. Only id and label
// are guaranteed; a reader tolerates a row that omits the rest.
type barWidget struct {
	ID       string       `json:"id"`
	Label    string       `json:"label"`
	Kind     string       `json:"kind"`
	Shown    bool         `json:"shown"`
	Section  string       `json:"section"`
	Settings []barSetting `json:"settings"`
}

type barSetting struct {
	Key string `json:"key"`
}

// loadQsbarCatalog asks a running QS Bar for its widget catalogue. It returns
// nil when QS Bar is not the active bar style or the shell does not answer.
// The call is read-only.
func loadQsbarCatalog(index *shellsIndex) []barWidget {
	if index != nil && index.Active != "" && index.Active != "qsbar" {
		return nil
	}
	out, ok := probe(5, "ryoku-shell", "ipc", "call", "qsbar", "catalog")
	if !ok {
		return nil
	}
	var all []barWidget
	if json.Unmarshal([]byte(strings.TrimSpace(out)), &all) != nil {
		return nil
	}
	return all
}

// settingKeys joins a catalogue widget's setting keys for the id table.
func settingKeys(rows []barSetting) string {
	keys := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.Key != "" {
			keys = append(keys, r.Key)
		}
	}
	if len(keys) == 0 {
		return "(none)"
	}
	return strings.Join(keys, ", ")
}

func barSectionBody() string {
	var b strings.Builder
	b.WriteString("\n## Bar and dock\n\n")
	b.WriteString(vaultBarFenceBegin + "\n")
	b.WriteString("One shell item draws the bar at a time: `barStyle` in\n")
	b.WriteString("`~/.config/ryoku/shell.json` names it. Ryoku ships the Orpheus and Kairos\n")
	b.WriteString("subshells; every other subshell and bar style is a Ryostore item\n")
	b.WriteString("(`ryostore catalog --category subshells`, `ryostore catalog --category barstyles`,\n")
	b.WriteString("`ryostore install <category> <id>`). Switch with `ryoku-shell barstyle <id>` or\n")
	b.WriteString("Ryoku Hub > Bar Studio; the shell restarts under the reload cover. An installed\n")
	b.WriteString("item that is not active stays packed and never loads, so its IPC target only\n")
	b.WriteString("exists while it is active. The shell daemon is the only writer of\n")
	b.WriteString("`shell.json`; change it through these commands, never by editing the file.\n\n")

	b.WriteString("### Installed shells\n\n")
	index := loadShellsIndex()
	if index == nil || len(index.Items) == 0 {
		b.WriteString("(shells index not found; `ryoku-shell shells sync` writes it)\n\n")
	} else {
		b.WriteString("| id | name | category | source | active | settings (while active) |\n|---|---|---|---|---|---|\n")
		for _, item := range index.Items {
			active := "no"
			if item.ID == index.Active {
				active = "yes"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", item.ID, mdCell(item.Name), item.Category,
				item.Source, active, mdCell(itemSettingsCommand(item)))
		}
		fmt.Fprintf(&b, "\nSource: `%s`.\n\n", tildeAbbrev(shellsIndexPath()))
	}

	b.WriteString("### QS Bar\n\n")
	b.WriteString("QS Bar (`qsbar`) is a store bar style. While it is active its layout is data\n")
	b.WriteString("in `shell.json` under `qsbar.layout` (ordered id lists for left, center and\n")
	b.WriteString("right) and it answers on its own IPC target:\n\n")
	b.WriteString("```\n")
	b.WriteString("ryoku-shell ipc call qsbar list | catalog | layout\n")
	b.WriteString("ryoku-shell ipc call qsbar move <id> <left|center|right> <placement>\n")
	b.WriteString("ryoku-shell ipc call qsbar show <id> | hide <id> | set <id> <key> <value>\n")
	b.WriteString("ryoku-shell ipc call qsbar position top|bottom | form full|fit|dock|notch|islands | defaults\n")
	b.WriteString("ryoku-shell ipc call qsbar settings <route>\n")
	b.WriteString("```\n\n")
	b.WriteString("A placement is empty (the end), an index, `before:<id>` or `after:<id>`.\n")
	b.WriteString("`settings \"\"` opens QS Bar Settings and `settings close` closes it.\n\n")
	if widgets := loadQsbarCatalog(index); len(widgets) == 0 {
		b.WriteString("(QS Bar is not the active bar style, so its widget catalogue is not listed)\n\n")
	} else {
		b.WriteString("| id | label | kind | shown | section | settings keys |\n|---|---|---|---|---|---|\n")
		for _, w := range widgets {
			section := w.Section
			if section == "" {
				section = "-"
			}
			fmt.Fprintf(&b, "| %s | %s | %s | %t | %s | %s |\n", w.ID, mdCell(w.Label), mdCell(w.Kind), w.Shown,
				mdCell(section), mdCell(settingKeys(w.Settings)))
		}
		b.WriteString("\n")
	}

	b.WriteString("### Dock\n\n")
	b.WriteString("Ryoku's own dock is part of the platform and works under every bar style:\n\n")
	b.WriteString("```\n")
	b.WriteString("ryoku-shell dock show|hide | edge auto|top|bottom|left|right | autohide on|off | pin <app>|unpin <app> | list [--json]\n")
	b.WriteString("```\n\n")

	b.WriteString("### Adding a widget\n\n")
	b.WriteString("A widget that is not in QS Bar's catalogue is a plugin, never an edit to the\n")
	b.WriteString("shipped QML under `~/.config/quickshell/` or `/usr/share/ryoku/`. Read the\n")
	b.WriteString("`ryoku` skill's `plugins.md` for the contract (manifest.json, service/Main.qml,\n")
	b.WriteString("content/Widget.qml, host `topbarGlyph`), write it in its own directory, then:\n\n")
	b.WriteString("```\n")
	b.WriteString("ryoku plugin validate <dir>\n")
	b.WriteString("ryoku plugin add <dir-or-git-url> --bar --yes\n")
	b.WriteString("```\n\n")
	b.WriteString("Installed plugins live in `~/.local/share/ryoku/plugins/<id>/`; their bar\n")
	b.WriteString("placement and settings persist in `~/.config/ryoku/plugins.json`. While QS Bar\n")
	b.WriteString("is active a bar plugin shows on it and under QS Bar Settings > Community\n")
	b.WriteString("(`ryoku-shell ipc call qsbar settings community`); `ryoku plugin remove <id>`\n")
	b.WriteString("takes it off again, and `ryoku plugin share <id>` (only when the user asks to\n")
	b.WriteString("publish) opens the Ryostore pull request that lists it for everyone.\n")
	b.WriteString(vaultBarFenceEnd + "\n")
	return b.String()
}
