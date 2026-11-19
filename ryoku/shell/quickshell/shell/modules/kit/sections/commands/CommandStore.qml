pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

Scope {
    id: root

    required property var catalogue

    readonly property string dataHome: Quickshell.env("XDG_DATA_HOME") || (Quickshell.env("HOME") + "/.local/share")
    readonly property string directoryPath: dataHome + "/ryoku/kit"
    readonly property string filePath: directoryPath + "/commands.json"

    property var commands: []
    property var favouriteIds: []
    property int revision: 0
    property bool ready: false
    property bool saving: false
    property string error: ""
    property string notice: ""

    property bool _directoryReady: false
    property bool _userLoaded: false
    property bool _favouritesPruned: false

    function cleanTags(value) {
        const seen = {};
        const result = [];
        const source = Array.isArray(value) ? value : [];
        for (const raw of source) {
            const tag = String(raw || "").trim();
            const key = tag.toLowerCase();
            if (tag.length > 0 && !seen[key]) {
                seen[key] = true;
                result.push(tag);
            }
        }
        return result;
    }

    function normalizeEntry(value, fallbackId) {
        const entry = value && typeof value === "object" ? value : {};
        const command = String(entry.command || "").trim();
        if (command.length === 0)
            return null;
        const name = String(entry.name || entry.title || entry.description || command).trim();
        const words = command.split(/\s+/);
        return {
            id: String(entry.id || fallbackId),
            library: "saved",
            group: "my-commands",
            title: name,
            name: name,
            command: command,
            description: String(entry.description || "").trim(),
            tool: words.length > 0 ? words[0] : "",
            level: "basic",
            featured: false,
            privileged: false,
            danger: false,
            editions: [],
            compositors: [],
            requires: "",
            tags: root.cleanTags(entry.tags),
            source: "user",
            onlyOn: ""
        };
    }

    function normalizeEntries(value) {
        if (!Array.isArray(value))
            return [];
        const result = [];
        const seen = {};
        for (let i = 0; i < value.length; i++) {
            const entry = root.normalizeEntry(value[i], "user." + i);
            if (!entry || seen[entry.id])
                continue;
            seen[entry.id] = true;
            result.push(entry);
        }
        return result;
    }

    function rebuild() {
        root.commands = root.commands.slice();
        root.revision++;
        root.ready = root._userLoaded;
        root.pruneFavourites();
    }

    function isFavourite(id) {
        const observed = root.revision;
        return root.favouriteIds.indexOf(String(id)) >= 0;
    }

    function pruneFavourites() {
        if (root._favouritesPruned || !root._userLoaded || !root.catalogue.ready)
            return;
        const valid = {};
        for (const entry of root.commands)
            valid[entry.id] = true;
        for (const id of root.catalogue.knownIds)
            valid[id] = true;
        const next = [];
        const seen = {};
        for (const value of root.favouriteIds) {
            const id = String(value);
            if (valid[id] && !seen[id]) {
                seen[id] = true;
                next.push(id);
            }
        }
        root._favouritesPruned = true;
        if (next.length === root.favouriteIds.length)
            return;
        root.favouriteIds = next;
        root.revision++;
        root.saveUser();
    }

    function nextId() {
        return "user." + Date.now().toString(36) + "." + Math.random().toString(36).slice(2, 8);
    }

    function addCommand(name, command, description, tags) {
        const id = root.nextId();
        const entry = root.normalizeEntry({
            id: id,
            name: name,
            command: command,
            description: description,
            tags: tags
        }, id);
        if (!entry)
            return false;
        const next = root.commands.slice();
        next.push(entry);
        root.commands = next;
        root.rebuild();
        root.saveUser();
        return true;
    }

    function updateCommand(id, name, command, description, tags) {
        const next = root.commands.slice();
        const index = next.findIndex(entry => entry.id === id);
        if (index < 0)
            return false;
        const replacement = root.normalizeEntry({
            id: id,
            name: name,
            command: command,
            description: description,
            tags: tags
        }, id);
        if (!replacement)
            return false;
        next[index] = replacement;
        root.commands = next;
        root.rebuild();
        root.saveUser();
        return true;
    }

    function deleteCommand(id) {
        const next = root.commands.filter(entry => entry.id !== id);
        if (next.length === root.commands.length)
            return false;
        root.commands = next;
        root.favouriteIds = root.favouriteIds.filter(value => value !== id);
        root.rebuild();
        root.saveUser();
        return true;
    }

    function toggleFavourite(id) {
        const key = String(id);
        const next = root.favouriteIds.slice();
        const index = next.indexOf(key);
        if (index >= 0)
            next.splice(index, 1);
        else
            next.push(key);
        root.favouriteIds = next;
        root.revision++;
        root.saveUser();
    }

    function importCommands(path) {
        const source = String(path || "");
        if (source.length === 0)
            return;
        importFile.path = source.startsWith("file:") ? source : Qt.resolvedUrl("file://" + source);
        importFile.reload();
    }

    function saveUser() {
        if (!root._directoryReady)
            return;
        const serialised = root.commands.map(entry => ({
            id: entry.id,
            name: entry.name,
            command: entry.command,
            description: entry.description,
            tags: entry.tags
        }));
        root.saving = true;
        userFile.setText(JSON.stringify({
            version: 1,
            favourites: root.favouriteIds,
            commands: serialised
        }, null, 2) + "\n");
        saveSettled.restart();
    }

    function loadUser(text) {
        try {
            const parsed = JSON.parse(text);
            const payload = Array.isArray(parsed) ? { commands: parsed, favourites: [] } : parsed;
            root.commands = root.normalizeEntries(payload.commands);
            root.favouriteIds = Array.isArray(payload.favourites)
                ? payload.favourites.map(value => String(value)) : [];
            root.error = "";
        } catch (exception) {
            root.commands = [];
            root.favouriteIds = [];
            root.error = I18n.tr("Commands could not be read: %1").arg(String(exception));
        }
        root._userLoaded = true;
        root.rebuild();
    }

    Connections {
        target: root.catalogue
        function onReadyChanged() { root.pruneFavourites(); }
        function onRevisionChanged() {
            if (!root._favouritesPruned)
                root.pruneFavourites();
        }
    }

    Timer {
        id: saveSettled
        interval: 800
        onTriggered: root.saving = false
    }

    FileView {
        id: importFile
        path: ""
        printErrors: false
        onLoaded: {
            try {
                const parsed = JSON.parse(importFile.text());
                const values = Array.isArray(parsed) ? parsed : parsed.commands;
                if (!Array.isArray(values))
                    throw new Error("expected an array of commands");
                const next = root.commands.slice();
                let imported = 0;
                for (const value of values) {
                    const id = root.nextId();
                    const entry = root.normalizeEntry({
                        id: id,
                        name: value.name,
                        command: value.command,
                        description: value.description,
                        tags: value.tags
                    }, id);
                    if (entry) {
                        next.push(entry);
                        imported++;
                    }
                }
                root.commands = next;
                root.rebuild();
                root.saveUser();
                root.error = "";
                root.notice = I18n.tr(imported === 1 ? "%1 command imported" : "%1 commands imported").arg(imported);
                noticeTimer.restart();
            } catch (exception) {
                root.error = I18n.tr("Commands could not be imported: %1").arg(String(exception));
            }
        }
        onLoadFailed: error => root.error = I18n.tr("The command file could not be read (%1)").arg(error)
    }

    Timer {
        id: noticeTimer
        interval: 2400
        onTriggered: root.notice = ""
    }

    FileView {
        id: userFile
        path: root._directoryReady ? Qt.resolvedUrl(root.filePath) : ""
        atomicWrites: true
        blockWrites: true
        printErrors: false
        onLoaded: root.loadUser(userFile.text())
        onSaved: {
            saveSettled.stop();
            root.saving = false;
        }
        onSaveFailed: error => {
            saveSettled.stop();
            root.saving = false;
            root.error = I18n.tr("Commands could not be saved (%1)").arg(error);
        }
        onLoadFailed: error => {
            if (!root._directoryReady)
                return;
            if (error === FileViewError.FileNotFound) {
                root.commands = [];
                root.favouriteIds = [];
                root._userLoaded = true;
                root.rebuild();
                root.saveUser();
                return;
            }
            root._userLoaded = true;
            root.error = I18n.tr("Commands could not be loaded (%1)").arg(error);
            root.rebuild();
        }
    }

    Process {
        id: prepareDirectory
        command: ["mkdir", "-p", root.directoryPath]
        environment: Spawn.env
        running: true
        onExited: exitCode => {
            if (exitCode !== 0) {
                root.error = I18n.tr("The command library folder could not be created");
                root._userLoaded = true;
                root.rebuild();
                return;
            }
            root._directoryReady = true;
        }
    }
}
