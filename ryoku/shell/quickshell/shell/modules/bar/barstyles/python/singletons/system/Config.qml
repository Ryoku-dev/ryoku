pragma Singleton
import QtQuick
import Quickshell
import Quickshell.Io
import shell.services

// Python's settings live in the `python` key of ~/.config/ryoku/shell.json,
// the same namespaced per-style store every Ryoku bar style uses. The shell
// daemon is the one writer of shell.json, so every write here goes over its
// control socket as `settings.patch`; reads come from the shell's live Config
// singleton and retune on every frame it publishes. The serpantinum section
// vocabulary (general, bar, dock, theme, music, ...) is preserved verbatim
// under the namespace, so a ported widget reads exactly the keys it expects.
Item {
    id: config

    readonly property string settingsJsonPath: (Quickshell.env("XDG_CONFIG_HOME")
        || (Quickshell.env("HOME") + "/.config")) + "/ryoku/shell.json"

    readonly property var defaultSettings: ({})
    property bool dataReady: true
    property var rawSettings: ({})
    property int hostSettingsRevision: 0
    readonly property bool hostNotificationSoundEnabled: {
        const revision = hostSettingsRevision;
        return hostNotificationSoundEnabledFor(false);
    }
    readonly property string hostNotificationSoundFile: {
        const revision = hostSettingsRevision;
        return String(hostSetting("sounds.events.notification.file", "") || "");
    }


    signal settingsLoaded()

    // Ryoku's single global system font (shell.json top-level fontFamily),
    // passed through the daemon's settings.patch. The service Config is what
    // `Config` names inside this file (no module import here); ported
    // surfaces reach the global font through these two members because in
    // their scope the style's own singleton shadows the service name.
    property string globalFontFamily: ""

    function setGlobalFontFamily(name) {
        config.globalFontFamily = name;
        patchQueue("fontFamily", name);
    }

    function applyDefaults(settings) {
        let merged = JSON.parse(JSON.stringify(defaultSettings));
        // The daemon delivers Config.python as QVariant wrappers: a nested
        // array (a grouped bar-module row) stringifies and indexes fine but
        // fails Array.isArray, so consumers that branch on grouping silently
        // treat the whole group as one opaque entry. Normalise the incoming
        // tree to plain JS values before merging.
        let source = (settings && typeof settings === "object")
            ? JSON.parse(JSON.stringify(settings)) : ({});
        for (let key in source) {
            let value = source[key];
            if (value && typeof value === "object" && !Array.isArray(value)
                    && merged[key] && typeof merged[key] === "object" && !Array.isArray(merged[key])) {
                for (let nestedKey in value) {
                    merged[key][nestedKey] = value[nestedKey];
                }
            } else {
                merged[key] = value;
            }
        }
        return merged;
    }

    function pull() {
        const p = Config.python
        config.rawSettings = applyDefaults(p)
        config.globalFontFamily = Config.fontFamily
        config.settingsLoaded()
    }

    function areEqual(a, b) {
        if (a === b) return true;
        if (a === null || b === null || typeof a !== "object" || typeof b !== "object") return false;
        if (Array.isArray(a) !== Array.isArray(b)) return false;
        let keysA = Object.keys(a);
        let keysB = Object.keys(b);
        if (keysA.length !== keysB.length) return false;
        for (let i = 0; i < keysA.length; i++) {
            let k = keysA[i];
            if (!Object.prototype.hasOwnProperty.call(b, k)) return false;
            if (!areEqual(a[k], b[k])) return false;
        }
        return true;
    }

    function setNestedValue(obj, path, value) {
        let parts = typeof path === "string" ? path.split(".") : [path];
        let cur = obj;
        for (let i = 0; i < parts.length - 1; i++) {
            let p = parts[i];
            if (!cur[p] || typeof cur[p] !== "object") {
                cur[p] = {};
            }
            cur = cur[p];
        }
        cur[parts[parts.length - 1]] = value;
    }
    function nestedValue(obj, path, fallbackValue) {
        let parts = typeof path === "string" ? path.split(".") : [path];
        let cur = obj;
        for (let i = 0; i < parts.length; i++) {
            if (!cur || typeof cur !== "object" || !cur.hasOwnProperty(parts[i])) {
                return fallbackValue;
            }
            cur = cur[parts[i]];
        }
        return cur !== undefined && cur !== null ? cur : fallbackValue;
    }

    function hostSetting(path, fallbackValue) {
        const soundsPath = String(path || "").replace(/^sounds\./, "");
        return nestedValue(Config.sounds || ({}), soundsPath, fallbackValue);
    }
    function hostNotificationSoundEnabledFor(critical) {
        const event = critical ? "notification-critical" : "notification";
        return Boolean(hostSetting("sounds.enabled", true)
            && hostSetting("sounds.events." + event + ".enabled", true));
    }


    function setHostSetting(path, value) {
        patchQueue(path, value);
    }

    function setNotificationSoundEnabled(enabled) {
        if (enabled) setHostSetting("sounds.enabled", true);
        setHostSetting("sounds.events.notification.enabled", enabled);
        setHostSetting("sounds.events.notification-critical.enabled", enabled);
    }

    function setNotificationSoundFile(filePath) {
        const path = String(filePath || "").replace(/^file:\/\//, "");
        setHostSetting("sounds.events.notification.file", path);
        setHostSetting("sounds.events.notification-critical.file", path);
    }

    function previewNotificationSound(filePath) {
        callQueue("sounds.preview", {
            "event": "notification",
            "file": String(filePath || "").replace(/^file:\/\//, "")
        });
    }


    function sh(cmd) {
        Quickshell.execDetached(["bash", "-c", cmd]);
    }

    function getSetting(key, fallbackValue) {
        let raw = rawSettings;
        if (!raw || typeof raw !== "object") {
            if (fallbackValue !== null && typeof fallbackValue === "object") {
                try { return JSON.parse(JSON.stringify(fallbackValue)); } catch (e) { return fallbackValue; }
            }
            return fallbackValue;
        }

        let val = undefined;
        if (raw.hasOwnProperty(key) && raw[key] !== undefined && raw[key] !== null) {
            val = raw[key];
        } else if (typeof key === "string" && key.indexOf(".") !== -1) {
            let parts = key.split(".");
            let cur = raw;
            for (let i = 0; i < parts.length; i++) {
                if (cur && typeof cur === "object" && cur.hasOwnProperty(parts[i])) {
                    cur = cur[parts[i]];
                } else {
                    val = fallbackValue;
                    break;
                }
            }
            if (val === undefined) {
                val = (cur !== undefined && cur !== null) ? cur : fallbackValue;
            }
        } else {
            val = fallbackValue;
        }

        if (val !== null && typeof val === "object") {
            try {
                return JSON.parse(JSON.stringify(val));
            } catch (e) {
                return val;
            }
        }
        return val;
    }

    function setSetting(key, value) {
        let obj = {};
        obj[key] = value;
        updateJsonBulk(obj);
    }

    function updateJsonBulk(dataObj) {
        let next = JSON.parse(JSON.stringify(rawSettings || {}));
        for (let key in dataObj) {
            let val = dataObj[key];
            let clonedVal = (val !== null && typeof val === "object") ? JSON.parse(JSON.stringify(val)) : val;
            if (typeof key === "string" && key.indexOf(".") !== -1) {
                setNestedValue(next, key, clonedVal);
            } else {
                next[key] = clonedVal;
            }
        }
        if (areEqual(rawSettings, next)) return;
        // Optimistic: the in-memory tree updates now so every bound widget
        // retunes immediately, and the daemon echo refreshes it from disk.
        rawSettings = next;
        patchQueue("python", next);
    }

    readonly property string sockPath: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku-shell.sock"

    function callQueue(method, payload) {
        ctl.queued += "call " + method + " " + JSON.stringify(payload || {}) + "\n";
        if (ctl.connected) ctl.flushQueued();
        else ctl.connected = true;
    }

    function patchQueue(path, value) {
        ctl.queued += "call settings.patch "
            + JSON.stringify({ path: path, value: value }) + "\n";
        if (ctl.connected) ctl.flushQueued();
        else ctl.connected = true;
    }

    Socket {
        id: ctl
        path: config.sockPath
        property string queued: ""
        function flushQueued() {
            if (queued.length === 0) return;
            write(queued); flush(); queued = "";
        }
        onConnectionStateChanged: if (connected) flushQueued()
    }


    Connections {
        target: Config
        function onPythonChanged() { config.pull(); }
        function onFontFamilyChanged() { config.pull(); }
        function onSettingsLoaded() { config.hostSettingsRevision += 1; }
    }

    Component.onCompleted: {
        config.pull();
    }
}
