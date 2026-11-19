pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

Scope {
    id: root

    property bool started: false
    property bool ready: false
    property string edition: ""
    property var libraries: []
    property var entries: []
    property var knownIds: []
    property int revision: 0

    property int _pending: 0
    property var _settled: ({})
    property var _sources: ({})

    function start() {
        if (root.started)
            return;
        root.started = true;
        root._pending = 5;
        root._settled = ({});
        indexFile.path = Qt.resolvedUrl("catalog/index.json");
        cliFile.path = Qt.resolvedUrl("catalog/ryoku-cli.json");
        toolsFile.path = Qt.resolvedUrl("catalog/ryoku-tools.json");
        linuxFile.path = Qt.resolvedUrl("catalog/linux.json");
        editionProbe.running = true;
    }

    function _finish(key) {
        if (root._settled[key])
            return;
        root._settled[key] = true;
        root._pending--;
        if (root._pending === 0) {
            root.rebuild();
            root.ready = true;
        }
    }

    function _warn(fileName, message) {
        console.warn("Commands catalogue: " + fileName + ": " + message);
    }

    function _parseIndex(text) {
        try {
            const parsed = JSON.parse(text);
            if (!parsed || parsed.schema !== 1 || !Array.isArray(parsed.libraries))
                throw new Error("expected schema 1 with a libraries array");
            const result = [];
            const seen = ({});
            for (const value of parsed.libraries) {
                if (!value || typeof value.id !== "string" || value.id.length === 0
                        || typeof value.label !== "string" || value.label.length === 0
                        || !Array.isArray(value.groups)) {
                    root._warn("index.json", "skipping an invalid library");
                    continue;
                }
                if (seen[value.id]) {
                    root._warn("index.json", "skipping duplicate library " + value.id);
                    continue;
                }
                const groups = [];
                const groupSeen = ({});
                for (const group of value.groups) {
                    if (!group || typeof group.id !== "string" || group.id.length === 0
                            || typeof group.label !== "string" || group.label.length === 0) {
                        root._warn("index.json", "skipping an invalid group in " + value.id);
                        continue;
                    }
                    if (groupSeen[group.id]) {
                        root._warn("index.json", "skipping duplicate group " + group.id);
                        continue;
                    }
                    groupSeen[group.id] = true;
                    groups.push({ id: group.id, label: group.label });
                }
                seen[value.id] = true;
                result.push({ id: value.id, label: value.label, groups: groups });
            }
            root.libraries = result;
        } catch (exception) {
            root.libraries = [];
            root._warn("index.json", String(exception));
        }
    }

    function _parseSource(text, expectedLibrary, fileName) {
        try {
            const parsed = JSON.parse(text);
            if (!parsed || parsed.schema !== 1 || parsed.library !== expectedLibrary
                    || !Array.isArray(parsed.commands))
                throw new Error("expected schema 1 for library " + expectedLibrary);
            root._sources[fileName] = {
                library: expectedLibrary,
                commands: parsed.commands
            };
        } catch (exception) {
            root._sources[fileName] = { library: expectedLibrary, commands: [] };
            root._warn(fileName, String(exception));
        }
    }

    function _groupsFor(libraryId) {
        for (const library of root.libraries) {
            if (library.id === libraryId)
                return library.groups;
        }
        return [];
    }

    function _validString(value) {
        return typeof value === "string" && value.trim().length > 0;
    }

    function _validStringArray(value) {
        if (!Array.isArray(value))
            return false;
        for (const item of value) {
            if (!root._validString(item))
                return false;
        }
        return true;
    }

    function _cleanEntry(value, libraryId, fileName, position) {
        if (!value || typeof value !== "object") {
            root._warn(fileName, "skipping invalid entry " + position);
            return null;
        }
        const required = ["id", "group", "title", "command", "description", "tool", "level"];
        for (const field of required) {
            if (!root._validString(value[field])) {
                root._warn(fileName, "skipping entry " + position + " with invalid " + field);
                return null;
            }
        }
        for (const field of ["featured", "privileged", "danger"]) {
            if (value[field] !== undefined && typeof value[field] !== "boolean") {
                root._warn(fileName, "skipping " + value.id + " with invalid " + field);
                return null;
            }
        }
        if (["basic", "advanced", "internal"].indexOf(value.level) < 0) {
            root._warn(fileName, "skipping " + value.id + " with invalid level");
            return null;
        }
        if (value.level === "internal" && value.group !== "ryoku-internal") {
            root._warn(fileName, "skipping " + value.id + " because internal entries need the internal group");
            return null;
        }
        const groups = root._groupsFor(libraryId);
        if (!groups.some(group => group.id === value.group)) {
            root._warn(fileName, "skipping " + value.id + " with unknown group " + value.group);
            return null;
        }
        if (value.editions !== undefined && !root._validStringArray(value.editions)) {
            root._warn(fileName, "skipping " + value.id + " with invalid editions");
            return null;
        }
        if (value.compositors !== undefined && !root._validStringArray(value.compositors)) {
            root._warn(fileName, "skipping " + value.id + " with invalid compositors");
            return null;
        }
        if (value.tags !== undefined && !root._validStringArray(value.tags)) {
            root._warn(fileName, "skipping " + value.id + " with invalid tags");
            return null;
        }
        if (value.requires !== undefined && value.requires !== "" && !root._validString(value.requires)) {
            root._warn(fileName, "skipping " + value.id + " with invalid requires");
            return null;
        }
        const editions = Array.isArray(value.editions) ? value.editions.map(item => String(item)) : [];
        const editionVisible = editions.length === 0 || editions.indexOf(root.edition) >= 0;
        const compositors = Array.isArray(value.compositors) ? value.compositors.map(item => String(item)) : [];
        return {
            id: value.id.trim(),
            library: libraryId,
            group: value.group.trim(),
            title: value.title.trim(),
            name: value.title.trim(),
            command: value.command.trim(),
            description: value.description.trim(),
            tool: value.tool.trim(),
            level: value.level,
            featured: value.featured === true,
            privileged: value.privileged === true,
            danger: value.danger === true,
            editions: editions,
            compositors: compositors,
            requires: typeof value.requires === "string" ? value.requires.trim() : "",
            tags: Array.isArray(value.tags) ? value.tags.map(item => String(item).trim().toLowerCase()) : [],
            source: "catalogue",
            _editionVisible: editionVisible,
            onlyOn: root._onlyOn(compositors)
        };
    }

    function _displayName(value) {
        const text = String(value || "");
        return text.length > 0 ? text.charAt(0).toUpperCase() + text.slice(1) : text;
    }

    function _onlyOn(compositors) {
        const provider = String(Wm.provider || "");
        if (provider.length === 0 || compositors.length === 0 || compositors.indexOf(provider) >= 0)
            return "";
        return compositors.map(root._displayName).join(", ");
    }


    function rebuild() {
        if (root._pending !== 0)
            return;
        const result = [];
        const seen = ({});
        const order = ["ryoku-cli.json", "ryoku-tools.json", "linux.json"];
        for (const fileName of order) {
            const source = root._sources[fileName] || { library: "", commands: [] };
            for (let i = 0; i < source.commands.length; i++) {
                const entry = root._cleanEntry(source.commands[i], source.library, fileName, i);
                if (!entry)
                    continue;
                if (seen[entry.id]) {
                    root._warn(fileName, "skipping duplicate id " + entry.id);
                    continue;
                }
                seen[entry.id] = true;
                if (entry._editionVisible)
                    result.push(entry);
            }
        }
        root.knownIds = Object.keys(seen);
        root.entries = result;
        root.revision++;
    }

    Connections {
        target: Wm
        function onProviderChanged() { root.rebuild(); }
    }

    FileView {
        id: indexFile
        path: ""
        printErrors: false
        onLoaded: {
            root._parseIndex(indexFile.text());
            root._finish("index");
        }
        onLoadFailed: error => {
            root.libraries = [];
            root._warn("index.json", "unavailable (" + error + ")");
            root._finish("index");
        }
    }

    FileView {
        id: cliFile
        path: ""
        printErrors: false
        onLoaded: {
            root._parseSource(cliFile.text(), "ryoku", "ryoku-cli.json");
            root._finish("cli");
        }
        onLoadFailed: error => {
            root._sources["ryoku-cli.json"] = { library: "ryoku", commands: [] };
            root._warn("ryoku-cli.json", "unavailable (" + error + ")");
            root._finish("cli");
        }
    }

    FileView {
        id: toolsFile
        path: ""
        printErrors: false
        onLoaded: {
            root._parseSource(toolsFile.text(), "ryoku", "ryoku-tools.json");
            root._finish("tools");
        }
        onLoadFailed: error => {
            root._sources["ryoku-tools.json"] = { library: "ryoku", commands: [] };
            root._warn("ryoku-tools.json", "unavailable (" + error + ")");
            root._finish("tools");
        }
    }

    FileView {
        id: linuxFile
        path: ""
        printErrors: false
        onLoaded: {
            root._parseSource(linuxFile.text(), "linux", "linux.json");
            root._finish("linux");
        }
        onLoadFailed: error => {
            root._sources["linux.json"] = { library: "linux", commands: [] };
            root._warn("linux.json", "unavailable (" + error + ")");
            root._finish("linux");
        }
    }

    Process {
        id: editionProbe
        command: ["ryoku-host", "capabilities"]
        environment: Spawn.env
        stdout: StdioCollector {
            onStreamFinished: {
                try {
                    const facts = JSON.parse(text);
                    root.edition = String(facts.edition || "");
                } catch (_) {
                    root.edition = "";
                }
            }
        }
        onExited: root._finish("edition")
    }
}
