pragma Singleton
pragma ComponentBehavior: Bound
import QtQuick
import Quickshell
import Quickshell.Io
import stage
import stage.services
import stage.modules.common.functions
import "quickToggles/androidStyle/QuickToggleCatalog.js" as QuickToggleCatalog

Singleton {
    id: root
    property string filePath: Directories.shellConfigPath
    property alias options: configOptionsJsonAdapter
    property bool ready: false
    property int readWriteDelay: 75 // milliseconds
    property bool blockWrites: false
    // Wall-clock time (ms) at singleton init. Used to detect legitimately missing
    // config.json (after the singleton has been alive long enough) vs. transient
    // inaccessibility (git pull, hot-reload timing, FS race) where we must NOT
    // overwrite the user's real config.json with the in-memory QML defaults.
    property real initTimestamp: Date.now()
    // Grace window during which a missing file is assumed transient and a
    // reload is retried before we ever dare to write defaults.
    // Increased from 2000 to 5000 to match writeGuardDelay — prevents
    // config.json from being clobbered with defaults during hot-reload.
    property int missingFileGracePeriod: 5000
    property int missingFileRetryInterval: 1500
    // Minimum time (ms) since singleton creation before ANY write is allowed,
    // even after `onLoaded` sets `ready = true`. This catches hot-reload races
    // where the file loads from page cache almost instantly but the
    // JsonAdapter hasn't fully merged values into all nested JsonObjects yet.
    // Increased from 3000 to 5000 to prevent config.json resets during
    // shell hot-reload while Phone services are initializing.
    property int writeGuardDelay: 5000

    /**
     * What kind of thing a config value is, in the vocabulary the checks below
     * and the assistant both use.
     */
    function valueKind(value): string {
        if (value === undefined)
            return "unset";
        if (value === null)
            return "null";
        if (Array.isArray(value))
            return "list";
        if (typeof value === "boolean")
            return "bool";
        if (typeof value === "number")
            return Number.isInteger(value) ? "int" : "real";
        if (typeof value === "string")
            return "string";
        if (typeof value === "object") {
            // A JsonObject is a group of options; an array-like one is a list.
            if (typeof value.length === "number" && Object.keys(value).every(key => !isNaN(key) || key === "length"))
                return "list";
            return "group";
        }
        return "unknown";
    }

    /**
     * Whether a value may be written to a key, and as what.
     *
     * The loose path below converts anything that merely looks numeric, which
     * is how a string option set to "007" became the number 7 and how an enum
     * could be handed a value it never accepts. This decides against the type
     * the key already holds instead of against the shape of the incoming
     * string, and returns the converted value so the caller writes exactly
     * what was checked.
     *
     * Returns {ok, reason, value, kind}.
     */
    function validateNestedValue(nestedKey, value): var {
        const keys = String(nestedKey ?? "").split(".").filter(part => part.length > 0);
        if (keys.length === 0)
            return {
                ok: false,
                reason: "No key given.",
                value: undefined,
                kind: "unset"
            };

        let node = root.options;
        for (let i = 0; i < keys.length - 1; ++i) {
            if (node === undefined || node === null || typeof node !== "object")
                return {
                    ok: false,
                    reason: `\`${keys.slice(0, i + 1).join(".")}\` is not a group of settings.`,
                    value: undefined,
                    kind: "unset"
                };
            node = node[keys[i]];
        }
        if (node === undefined || node === null || typeof node !== "object")
            return {
                ok: false,
                reason: `\`${nestedKey}\` does not exist.`,
                value: undefined,
                kind: "unset"
            };

        const leaf = keys[keys.length - 1];
        const current = node[leaf];
        const kind = root.valueKind(current);
        if (kind === "unset")
            return {
                ok: false,
                reason: `\`${nestedKey}\` does not exist.`,
                value: undefined,
                kind: kind
            };
        if (kind === "group")
            return {
                ok: false,
                reason: `\`${nestedKey}\` is a group of settings, not a single value. Set the options inside it.`,
                value: undefined,
                kind: kind
            };

        const raw = typeof value === "string" ? value.trim() : value;
        let converted = raw;

        if (kind === "bool") {
            if (typeof raw === "boolean")
                converted = raw;
            else if (raw === "true" || raw === "false")
                converted = raw === "true";
            else
                return {
                    ok: false,
                    reason: `\`${nestedKey}\` is a switch. It takes true or false.`,
                    value: undefined,
                    kind: kind
                };
        } else if (kind === "int" || kind === "real") {
            if (typeof raw === "number")
                converted = raw;
            else if (typeof raw === "string" && /^-?(?:\d+|\d*\.\d+)$/.test(raw))
                converted = Number(raw);
            else
                return {
                    ok: false,
                    reason: `\`${nestedKey}\` is a number.`,
                    value: undefined,
                    kind: kind
                };
            if (!isFinite(converted))
                return {
                    ok: false,
                    reason: `\`${nestedKey}\` is a number.`,
                    value: undefined,
                    kind: kind
                };
            // Whole against fractional is deliberately not enforced. The kind
            // comes from the value the option happens to hold, and a `real`
            // sitting at 1 is indistinguishable from an `int` — rejecting 1.5
            // there would refuse a change that is perfectly legal. QML rounds
            // a fraction assigned to an int property, which is the mild half
            // of the two failures.
        } else if (kind === "list") {
            if (Array.isArray(raw))
                converted = raw;
            else if (typeof raw === "string") {
                try {
                    const parsed = JSON.parse(raw);
                    if (!Array.isArray(parsed))
                        throw new Error("not a list");
                    converted = parsed;
                } catch (e) {
                    return {
                        ok: false,
                        reason: `\`${nestedKey}\` is a list. Give it a JSON array.`,
                        value: undefined,
                        kind: kind
                    };
                }
            } else
                return {
                    ok: false,
                    reason: `\`${nestedKey}\` is a list.`,
                    value: undefined,
                    kind: kind
                };
        } else if (kind === "string") {
            // Deliberately no conversion: a string option keeps what it was
            // given, leading zeroes and all.
            if (typeof raw === "object")
                return {
                    ok: false,
                    reason: `\`${nestedKey}\` is text.`,
                    value: undefined,
                    kind: kind
                };
            converted = String(raw);
        }

        const allowed = root.enumConstraints[keys.join(".")];
        if (allowed !== undefined && allowed.indexOf(converted) === -1)
            return {
                ok: false,
                reason: `\`${nestedKey}\` only takes ${allowed.map(entry => JSON.stringify(entry)).join(", ")}.`,
                value: undefined,
                kind: kind
            };

        return {
            ok: true,
            reason: "",
            value: converted,
            kind: kind
        };
    }

    /**
     * Every option path in the config, as dotted keys.
     *
     * Built once: the shape comes from the QML declarations, so it changes
     * with the shell version and not with what the user sets. It exists so a
     * key can be looked for by name instead of the whole file being read out
     * to find one — the assistant used to be handed all of config.json, some
     * forty-six kilobytes of it, to change a single switch.
     */
    property var cachedKeyPaths: null

    function keyPaths(): var {
        if (root.cachedKeyPaths !== null)
            return root.cachedKeyPaths;
        const paths = [];
        const walk = (node, prefix) => {
            if (paths.length > 4000)
                return;
            for (const name in node) {
                if (name.startsWith("object") || name.startsWith("parent") || name.startsWith("children") || name.startsWith("metaObject") || name.startsWith("destroyed") || name.startsWith("reloadableId"))
                    continue;
                const value = node[name];
                if (typeof value === "function")
                    continue;
                const path = prefix.length > 0 ? `${prefix}.${name}` : name;
                if (root.valueKind(value) === "group")
                    walk(value, path);
                else
                    paths.push(path);
            }
        };
        walk(root.options, "");
        root.cachedKeyPaths = paths;
        return paths;
    }

    /** A value short enough to hand to a reader, with a note when it was cut. */
    function summariseValue(value, maxLength = 120): var {
        const kind = root.valueKind(value);
        if (kind === "group")
            return {
                kind: kind,
                value: "…"
            };
        if (kind === "list") {
            const list = Array.from(value ?? []);
            const text = JSON.stringify(list);
            return text.length <= maxLength ? {
                kind: kind,
                value: list
            } : {
                kind: kind,
                value: `${list.length} entries`,
                truncated: true
            };
        }
        if (kind === "string") {
            const text = String(value);
            return text.length <= maxLength ? {
                kind: kind,
                value: text
            } : {
                kind: kind,
                value: `${text.slice(0, maxLength)}…`,
                truncated: true
            };
        }
        return {
            kind: kind,
            value: value
        };
    }

    /**
     * Options whose key path matches every word given.
     *
     * Matching is on the key path alone, which is what the shell knows without
     * the settings window open. Labels, pages and translations belong to the
     * settings index, which is a separate thing.
     */
    function findKeys(query, limit = 25): var {
        const words = String(query ?? "").toLowerCase().split(/[\s._-]+/).filter(word => word.length > 1);
        if (words.length === 0)
            return [];
        const paths = root.keyPaths();
        const scored = [];
        for (let i = 0; i < paths.length; i++) {
            const path = paths[i];
            const lower = path.toLowerCase();
            // A key path is camelCase, so the words in it need separating
            // before "automatic suspend" can match "battery.automaticSuspend".
            const spaced = lower.replace(/([a-z])([A-Z])/g, "$1 $2").toLowerCase() + " " + path.replace(/([a-z])([A-Z])/g, "$1 $2").toLowerCase().replace(/\./g, " ");
            let score = 0;
            let matchedAll = true;
            for (let w = 0; w < words.length; w++) {
                const word = words[w];
                if (lower.endsWith(word))
                    score += 300;
                else if (spaced.indexOf(` ${word} `) >= 0 || spaced.indexOf(` ${word}`) >= 0)
                    score += 200;
                else if (lower.indexOf(word) >= 0)
                    score += 100;
                else {
                    matchedAll = false;
                    break;
                }
            }
            if (!matchedAll)
                continue;
            // A short path that matched is more likely the option itself than
            // a long one that merely contains the word.
            score -= path.length;
            scored.push({
                path: path,
                score: score
            });
        }
        scored.sort((a, b) => b.score - a.score);
        return scored.slice(0, Math.max(1, limit)).map(entry => {
            const summary = root.summariseValue(root.getNestedValue(root.options, entry.path.split(".")));
            return {
                key: entry.path,
                type: summary.kind,
                value: summary.value
            };
        });
    }

    /** What sits directly under one group, one level deep. */
    function listGroup(prefix, limit = 60): var {
        const keys = String(prefix ?? "").split(".").filter(part => part.length > 0);
        const node = keys.length === 0 ? root.options : root.getNestedValue(root.options, keys);
        if (node === undefined || root.valueKind(node) !== "group")
            return null;
        const entries = [];
        for (const name in node) {
            if (name.startsWith("object") || name.startsWith("parent") || name.startsWith("children") || name.startsWith("metaObject") || name.startsWith("destroyed") || name.startsWith("reloadableId"))
                continue;
            const value = node[name];
            if (typeof value === "function")
                continue;
            const path = keys.length > 0 ? `${keys.join(".")}.${name}` : name;
            const summary = root.summariseValue(value, 60);
            const entry = {
                key: path,
                type: summary.kind
            };
            if (summary.kind !== "group")
                entry.value = summary.value;
            entries.push(entry);
            if (entries.length >= Math.max(1, limit))
                break;
        }
        return entries;
    }

    /**
     * Writes one option.
     *
     * `strict` is for callers that did not write the key themselves — the
     * assistant, above all. Loose writing creates whatever path it is handed
     * and converts by guessing, which is fine for a switch in the settings
     * window bound to a key that provably exists, and is not fine for a key a
     * model produced from memory. Strict refuses an unknown key, refuses a
     * value of the wrong kind, and refuses a value outside a declared enum.
     */
    function setNestedValue(nestedKey, value, strict = false) {
        let keys = nestedKey.split(".");

        if (strict) {
            const verdict = root.validateNestedValue(nestedKey, value);
            if (!verdict.ok)
                throw new Error(verdict.reason);
            let target = root.options;
            for (let i = 0; i < keys.length - 1; ++i) {
                target = target[keys[i]];
            }
            target[keys[keys.length - 1]] = verdict.value;
            return verdict.value;
        }

        let obj = root.options;
        let parents = [obj];

        // Traverse and collect parent objects
        for (let i = 0; i < keys.length - 1; ++i) {
            if (!obj[keys[i]] || typeof obj[keys[i]] !== "object") {
                obj[keys[i]] = {};
            }
            obj = obj[keys[i]];
            parents.push(obj);
        }

        // Convert value to correct type using JSON.parse when safe
        let convertedValue = value;
        if (typeof value === "string") {
            let trimmed = value.trim();
            if (trimmed === "true" || trimmed === "false" || !isNaN(Number(trimmed))) {
                try {
                    convertedValue = JSON.parse(trimmed);
                } catch (e) {
                    convertedValue = value;
                }
            }
        }

        obj[keys[keys.length - 1]] = convertedValue;
        return convertedValue;
    }

    // Persist options immediately (e.g. kill dialog "Always" in a short-lived process).
    function saveOptionsNow() {
        // Never let an unrelated forced save (e.g. the kill dialog's
        // "Always" button) defeat the malformed-config write block — that
        // block exists specifically to stop the in-memory defaults from
        // clobbering a broken-but-recoverable file.
        if (root.configMalformed)
            return;
        root.blockWrites = false;
        if (!root.ready)
            return;
        configFileView.writeAdapter();
        root._adapterMatchesDisk();
        LocalPreferences.syncFromConfig();
    }

    Timer {
        id: fileReloadTimer
        interval: root.readWriteDelay
        repeat: false
        onTriggered: {
            if (root.ready && !root.configMalformed)
                incomingFileView.reload();
            else
                configFileView.reload();
        }
    }

    // External rewrites of config.json (presets, switchwall, scripts) do not go
    // through the adapter's deserializer once the config is loaded. It rewrites
    // every list property whether or not it changed — each Repeater bound to
    // one rebuilds — and does all of it inside a single frame, which froze the
    // shell for up to a second on a preset switch. Instead the new file is
    // diffed against the adapter, only values that differ are assigned, and
    // the top-level sections are spread over consecutive frames (all at once
    // while a preset holds motion — see PresetTransition).
    property var _incomingRaw: null
    property var _incomingSections: []
    property bool _incomingNeedsSave: false
    // True only while a staged assignment runs, so the writes it causes are
    // not saved back — the file already holds them. Matches the adapter's
    // own changesBlocked during a native load.
    property bool _assigningIncoming: false
    // While a preset holds motion nothing on screen can stutter, so the
    // sections land in one go instead; spacing them only stretched the hold.
    readonly property int _incomingFrameBudgetMs: 4
    /// An external change is still being applied, section by section.
    readonly property bool applyingExternal: root._incomingRaw !== null
    /// Bumped each time an external change has been fully applied.
    property int externalApplySerial: 0

    FileView {
        id: incomingFileView
        path: root.filePath
        printErrors: false
        // It preloads at startup too; the adapter's own load covers that one.
        onLoaded: {
            if (root.ready)
                root._stageIncoming(incomingFileView.text());
        }
        // Missing or unreadable: the native path owns the retry/defaults logic.
        onLoadFailed: configFileView.reload()
    }

    // What the adapter last matched on disk. After a staged apply the
    // FileView's own text is stale (it never loaded that file), so the staged
    // text stands in until the next native load or write.
    property string _stagedText: ""
    property bool _stagedTextCurrent: false
    function _adapterMatchesDisk() {
        root._stagedText = "";
        root._stagedTextCurrent = false;
    }

    // A preset's bar is still sliding out; the change lands once it is gone.
    property string _deferredIncoming: ""
    Connections {
        target: GlobalStates
        function onPresetWorkDeferredChanged() {
            if (GlobalStates.presetWorkDeferred || root._deferredIncoming === "")
                return;
            const text = root._deferredIncoming;
            root._deferredIncoming = "";
            root._stageIncoming(text);
        }
    }

    function _stageIncoming(text) {
        if (GlobalStates.presetWorkDeferred) {
            root._deferredIncoming = text;
            return;
        }
        // Our own writes come back through the watcher; nothing to apply.
        if (root._stagedTextCurrent ? text === root._stagedText : text === configFileView.text())
            return;
        let raw;
        try {
            raw = JSON.parse(text);
        } catch (e) {
            configFileView.reload();
            return;
        }
        if (raw === null || typeof raw !== "object" || Array.isArray(raw)) {
            configFileView.reload();
            return;
        }
        if (root.defaultOptions !== null) {
            const repaired = [];
            const migrated = root.migrateRaw(raw);
            root.repairTypeConflicts(raw, root.options, root.defaultOptions, "", repaired);
            root.repairEnumViolations(raw, repaired);
            if (migrated || repaired.length > 0)
                root._incomingNeedsSave = true;
        }
        root._stagedText = text;
        root._stagedTextCurrent = true;
        // A newer file replaces whatever is still queued from an older one.
        root._incomingRaw = raw;
        root._incomingSections = Object.keys(raw);
        // Build panels once against the final config, not every stage of it.
        PanelSchedule.hold = true;
        incomingStepTimer.stop();
        root._applyIncomingStep();
    }

    function _applyIncomingStep() {
        const raw = root._incomingRaw;
        if (!raw)
            return;
        const sections = root._incomingSections;
        const start = Date.now();
        root._assigningIncoming = true;
        try {
            // At least one section per frame; more while they stay cheap.
            do {
                const key = sections.shift();
                root._assignIncoming(root.options, key, raw[key]);
            } while (sections.length > 0 && (GlobalStates.presetHoldMotion || Date.now() - start < root._incomingFrameBudgetMs));
        } finally {
            root._assigningIncoming = false;
        }
        if (sections.length > 0) {
            incomingStepTimer.restart();
            return;
        }
        root._incomingRaw = null;
        PanelSchedule.hold = false;
        if (root._incomingNeedsSave) {
            root._incomingNeedsSave = false;
            fileWriteTimer.restart();
        }
        // What onLoaded does after a native load.
        root.migrateRoundingConfig();
        root.syncAppLaunchAnimation();
        if (Persistent.ready)
            Persistent.tryMigrateAndSyncUserData();
        LocalPreferences.reconcile();
        root.externalApplySerial++;
    }

    function _assignIncoming(target, key, next) {
        const current = target[key];
        // Unknown keys are dropped, nulls skipped — as the adapter does.
        if (current === undefined || next === null || next === undefined)
            return;
        if (typeof next === "object" && !Array.isArray(next)) {
            if (current !== null && typeof current === "object" && !root.isArrayLike(current)) {
                for (const child in next)
                    root._assignIncoming(current, child, next[child]);
            }
            return;
        }
        if (Array.isArray(next) || root.isArrayLike(current)) {
            if (JSON.stringify(Array.from(current || [])) !== JSON.stringify(next)) {
                target[key] = next;
            }
            return;
        }
        if (current !== next)
            target[key] = next;
    }

    Timer {
        id: incomingStepTimer
        // One frame apart, so a heavy section never runs back to back with
        // the next one.
        interval: 16
        repeat: false
        onTriggered: root._applyIncomingStep()
    }

    Timer {
        id: fileWriteTimer
        interval: root.readWriteDelay
        repeat: false
        onTriggered: {
            // Never overwrite the user's config.json with the in-memory
            // JsonAdapter state (which is mostly QML defaults until the file
            // is loaded). If the file has not loaded yet, defer the write and
            // keep retrying — once `onLoaded` flips `root.ready` the pending
            // write will fire with the file's real values merged in.
            if (root.blockWrites) {
                return;
            }
            if (!root.ready) {
                fileWriteTimer.restart();
                return;
            }
            // Extra guard: even after `ready`, if the singleton was created
            // less than `writeGuardDelay` ms ago, defer. This catches the
            // hot-reload race where `onLoaded` fires very fast (file is in
            // page cache) but the JsonAdapter hasn't fully merged the file's
            // values into all nested JsonObjects yet.
            const elapsed = Date.now() - root.initTimestamp;
            if (elapsed < root.writeGuardDelay) {
                fileWriteTimer.restart();
                return;
            }
            configFileView.writeAdapter();
            root._adapterMatchesDisk();
            LocalPreferences.syncFromConfig();
        }
    }

    // If `onLoadFailed(FileNotFound)` fires, do NOT immediately call
    // `writeAdapter()` — during a git pull, hot-reload, or PC restart the file
    // can be briefly inaccessible and the QML defaults would clobber the user's
    // real config.json. Instead, retry `reload()` after the grace period; only
    // if the file is still genuinely missing after the singleton has been alive
    // for a while do we create defaults.
    Timer {
        id: missingFileRetryTimer
        interval: root.missingFileRetryInterval
        repeat: false
        onTriggered: {
            configFileView.reload();
        }
    }

    // Shared gate for every OSD implementation (default, minimal/material, Waffle).
    // Unknown indicator ids stay enabled so a new one is opt-out, not opt-in.
    function osdIndicatorEnabled(indicatorId): bool {
        if (!root.ready || !root.options.osd)
            return true;
        if (!root.options.osd.enable)
            return false;
        const indicators = root.options.osd.indicators;
        if (!indicators)
            return true;
        switch (indicatorId) {
        case "volume":
            return indicators.volume;
        case "brightness":
            return indicators.brightness;
        case "keyboardBrightness":
            return indicators.keyboardBrightness;
        case "playerVolume":
            return indicators.playerVolume;
        case "gamma":
            return indicators.gamma;
        case "microphone":
            return indicators.microphone;
        case "capsLock":
            return indicators.capsLock;
        case "numLock":
            return indicators.numLock;
        case "pills":
            return indicators.pills;
        }
        return true;
    }

    // ── Ryoku bridge ────────────────────────────────────────────────────────
    // When Ryoku mounts the Stage Editor it installs a provider here: the
    // desktop widget store becomes Ryoku's (widgets.json + the plugin place
    // tool) rather than this island's own `activeWidgets` list, so the chrome
    // edits the desktop the user actually has. Without a provider the island
    // is self-contained (the verbatim reference behaviour).
    property var widgetProvider: null

    // Catalogues the provider adds beside Widgets, Wallpaper and Style: the
    // editors Ryoku folds into this mode (the visualiser, the depth stage).
    // Each is {section, label, icon, tooltip, intro, page}, where `page` is
    // the Component the drawer shows for it. None without a provider.
    readonly property var extraSections: root.widgetProvider
        ? (root.widgetProvider.extraSections ?? []) : []

    // The placed widgets, in the island's entry shape, from whichever store is
    // live. Readers (the drawer's counts, the clear row, the reset) go through
    // this rather than the raw option.
    readonly property var activeWidgets: root.widgetProvider
        ? root.widgetProvider.activeWidgets
        : (root.options.background.activeWidgets ?? [])

    // The catalogue the drawer lists: the provider's roster in the island's
    // row shape, or the reference's own registry.
    readonly property var widgetCatalogue: root.widgetProvider
        ? root.widgetProvider.catalogue
        : null

    // The drop grid's snap flag, from whichever store is live.
    readonly property bool widgetSnapEnabled: root.widgetProvider
        ? root.widgetProvider.snapEnabled
        : (root.options.background.widgets.enableSnap ?? true)

    // The picture the desktop shows, from whichever store is live. The Ryoku
    // mount reads it through the provider (ryogami owns the plane); the island
    // keeps its own option. Read-only: picks still land in `background` via
    // Wallpapers.apply, so the style history works unchanged.
    readonly property string wallpaperPath: root.widgetProvider
        ? root.widgetProvider.wallpaperPath
        : String(root.options.background.wallpaperPath ?? "")

    function isWidgetActive(widgetId) {
        return root.countWidgetInstances(widgetId) > 0;
    }

    // How many of this widget the desktop holds. A widget can be placed more
    // than once - two clocks in two corners, one weather card per city - so
    // "is it there" is a special case of "how many", not the other way round.
    function countWidgetInstances(widgetId) {
        if (root.widgetProvider)
            return root.widgetProvider.count(widgetId);
        let list = root.options.background.activeWidgets || [];
        let count = 0;
        for (let i = 0; i < list.length; i++) {
            if (list[i].widgetId === widgetId)
                count++;
        }
        return count;
    }

    function getWidgetLockBehavior(widgetId) {
        let list = root.options.background.activeWidgets || [];
        for (let i = 0; i < list.length; i++) {
            if (list[i].widgetId === widgetId)
                return list[i].lockBehavior || "hide";
        }
        return "hide";
    }

    // ── Desktop widget writers ───────────────────────────────────────────────
    // Every writer below records an {undo, redo} pair with GlobalStates while
    // Edit Mode is on (a no-op otherwise), as whole-entry snapshots: replaying
    // one puts the entry back exactly as it was, which covers a position, a
    // scale, a lock rule, a pin and a per-monitor fork in one shape, and a
    // removed widget comes back at its old index with its old placement. The
    // replay helpers write the list directly, so an undo never records itself.
    function _widgetEntryClone(entry) {
        return entry ? JSON.parse(JSON.stringify(entry)) : null;
    }

    function _widgetEntryIndex(list, instanceId) {
        for (let i = 0; i < list.length; i++) {
            if (list[i].id === instanceId)
                return i;
        }
        return -1;
    }

    function _replaceWidgetEntry(instanceId, snapshot, index) {
        let cloned = JSON.parse(JSON.stringify(root.options.background.activeWidgets || []));
        const at = root._widgetEntryIndex(cloned, instanceId);
        if (at !== -1) {
            cloned[at] = root._widgetEntryClone(snapshot);
        } else {
            const insertAt = Math.max(0, Math.min(index ?? cloned.length, cloned.length));
            cloned.splice(insertAt, 0, root._widgetEntryClone(snapshot));
        }
        root.options.background.activeWidgets = cloned;
    }

    function _removeWidgetEntry(instanceId) {
        let cloned = JSON.parse(JSON.stringify(root.options.background.activeWidgets || []));
        const at = root._widgetEntryIndex(cloned, instanceId);
        if (at === -1)
            return;
        cloned.splice(at, 1);
        root.options.background.activeWidgets = cloned;
    }

    // `before`/`after` are entry snapshots, null on the side where the entry
    // does not exist (an add has no before, a removal no after).
    // One lock island's stored order, written as one history entry. The
    // closures reach only this singleton, never the preview overlay that asked -
    // the Lockscreen tab tears that down and the stack outlives it.
    // One lock island item hidden or shown again, as one history entry: the
    // remove badge over the Lockscreen tab's preview, and the way back from
    // the catalogue's own list of what is hidden.
    function setLockIslandHidden(id, hidden) {
        const current = EditModeLogic.listCopy(root.options.lock.islands.hidden ?? []);
        const has = current.indexOf(id) !== -1;
        if (has === hidden)
            return;
        const next = hidden ? current.concat([id]) : current.filter(entry => entry !== id);
        root.options.lock.islands.hidden = next;
        GlobalStates.editHistoryPush({
            "undo": () => {
                root.options.lock.islands.hidden = current;
            },
            "redo": () => {
                root.options.lock.islands.hidden = next;
            }
        });
    }

    function setLockIslandOrder(island, list) {
        const islands = root.options.lock.islands;
        const before = EditModeLogic.listCopy(island === "main" ? islands.main : island === "left" ? islands.left : islands.right);
        const after = EditModeLogic.listCopy(list);
        const write = l => {
            if (island === "main")
                islands.main = l;
            else if (island === "left")
                islands.left = l;
            else
                islands.right = l;
        };
        write(after);
        GlobalStates.editHistoryPush({
            "undo": () => write(before),
            "redo": () => write(after)
        });
    }

    function _recordWidgetEdit(instanceId, before, after, index) {
        if (!GlobalStates.editMode)
            return;
        if (JSON.stringify(before) === JSON.stringify(after))
            return;
        const restore = snapshot => snapshot === null ? (() => root._removeWidgetEntry(instanceId)) : (() => root._replaceWidgetEntry(instanceId, snapshot, index));
        GlobalStates.editHistoryPush({
            "undo": restore(before),
            "redo": restore(after)
        });
    }

    // Edit one entry in place, recording the change.
    function _editWidgetEntry(instanceId, mutate) {
        let cloned = JSON.parse(JSON.stringify(root.options.background.activeWidgets || []));
        const at = root._widgetEntryIndex(cloned, instanceId);
        if (at === -1)
            return;
        const before = root._widgetEntryClone(cloned[at]);
        mutate(cloned[at]);
        root.options.background.activeWidgets = cloned;
        root._recordWidgetEdit(instanceId, before, root._widgetEntryClone(cloned[at]), at);
    }

    // By KIND, so every copy follows - the callers are Settings' widget cards,
    // which ask the question about the widget rather than about one of its
    // copies. `getWidgetLockBehavior` answers with the first, because a card
    // has one control to show it in; the two are asymmetric on purpose, and
    // the per-copy answer lives in Edit Mode's widget menu.
    function setWidgetLockBehavior(widgetId, newLockBehavior) {
        const list = root.options.background.activeWidgets || [];
        const ids = [];
        for (let i = 0; i < list.length; i++) {
            if (list[i].widgetId === widgetId)
                ids.push(list[i].id);
        }
        if (ids.length === 0)
            return;
        GlobalStates.editHistoryBeginBatch();
        for (const id of ids)
            root.updateWidgetLockBehavior(id, newLockBehavior);
        GlobalStates.editHistoryEndBatch();
    }

    // A widget instance id that nothing else holds. `Date.now()` alone is not
    // one: two widgets added inside the same millisecond - a batch, or a
    // double click on a catalogue row, both of which are ordinary now that a
    // widget can be placed twice - would share an id, and every lookup in this
    // file finds an entry BY id.
    function _freeWidgetInstanceId(list, widgetId) {
        const base = "widget_" + widgetId + "_" + Date.now();
        let candidate = base;
        let suffix = 1;
        while (list.some(entry => entry && entry.id === candidate)) {
            candidate = base + "_" + suffix;
            suffix++;
        }
        return candidate;
    }

    // Collision placement reads what `monitorName` shows (see WidgetPlacement);
    // the new entry itself is written as legacy x/y so every monitor starts it
    // from the same spot.
    //
    // Adding one that is already on the desktop adds ANOTHER, and that is the
    // point: this used to return early on a matching `widgetId`, which made
    // "two clocks in two corners" impossible to express. Returns the new
    // instance id so a caller can act on the one it just made.
    function addWidgetToDesktop(widgetId, defaultX, defaultY, monitorName, lockBehavior = "hide") {
        // The provider owns the write and its history entry: for Ryoku a
        // re-add of a placed widget is a move, and only the store can say
        // what the walk-back of that is.
        if (root.widgetProvider)
            return root.widgetProvider.addWidget(widgetId, defaultX, defaultY, monitorName);
        let cloned = JSON.parse(JSON.stringify(root.options.background.activeWidgets || []));

        let startX = defaultX !== undefined ? defaultX : 200;
        let startY = defaultY !== undefined ? defaultY : 200;

        if (defaultX === undefined && defaultY === undefined) {
            let offset = 0;
            while (true) {
                let collision = false;
                for (let i = 0; i < cloned.length; i++) {
                    const placed = WidgetPlacement.resolve(cloned[i], monitorName);
                    if (Math.abs(placed.x - (startX + offset)) < 30 && Math.abs(placed.y - (startY + offset)) < 30) {
                        collision = true;
                        break;
                    }
                }
                if (!collision) {
                    startX += offset;
                    startY += offset;
                    break;
                }
                offset += 80;
            }
        }

        let instanceId = root._freeWidgetInstanceId(cloned, widgetId);
        const entry = {
            "id": instanceId,
            "widgetId": widgetId,
            "x": startX,
            "y": startY,
            "placementStrategy": "free",
            "lockBehavior": lockBehavior
        };
        cloned.push(entry);
        root.options.background.activeWidgets = cloned;
        root._recordWidgetEdit(instanceId, null, root._widgetEntryClone(entry), cloned.length - 1);
        return instanceId;
    }

    // ONE instance, by its own id. This is what every caller that already knows
    // which widget it is talking about wants - the menu on a widget, a widget
    // dragged back into the catalogue - and before a widget could be placed
    // twice they all went through the widgetId path below, which removes the
    // first match and would now remove the wrong copy.
    // One more of this exact widget, beside the original: its behaviour, its
    // pin and its size come along, its per-monitor positions do not - the
    // copy starts as a plain placement a step down and to the right of where
    // the original is drawn on `monitorName`, so it is visible next to it.
    function duplicateWidgetInstance(instanceId, monitorName) {
        let cloned = JSON.parse(JSON.stringify(root.options.background.activeWidgets || []));
        const at = root._widgetEntryIndex(cloned, instanceId);
        if (at === -1)
            return "";
        const source = cloned[at];
        const placed = WidgetPlacement.resolve(source, monitorName);
        const copy = root._widgetEntryClone(source);
        delete copy.positions;
        delete copy.lockPositions;
        copy.id = root._freeWidgetInstanceId(cloned, source.widgetId);
        copy.x = placed.x + 40;
        copy.y = placed.y + 40;
        if (placed.scale !== undefined && placed.scale !== null)
            copy.scale = placed.scale;
        cloned.splice(at + 1, 0, copy);
        root.options.background.activeWidgets = cloned;
        root._recordWidgetEdit(copy.id, null, root._widgetEntryClone(copy), at + 1);
        return copy.id;
    }

    function removeWidgetInstance(instanceId) {
        if (root.widgetProvider) {
            const before = root.widgetProvider.snapshot(instanceId);
            root.widgetProvider.removeWidget(instanceId);
            GlobalStates.editHistoryPush({
                "undo": () => root.widgetProvider.restore(instanceId, before),
                "redo": () => root.widgetProvider.restore(instanceId, null)
            });
            return;
        }
        let cloned = JSON.parse(JSON.stringify(root.options.background.activeWidgets || []));
        const at = root._widgetEntryIndex(cloned, instanceId);
        if (at === -1)
            return;
        const before = root._widgetEntryClone(cloned[at]);
        cloned.splice(at, 1);
        root.options.background.activeWidgets = cloned;
        root._recordWidgetEdit(instanceId, before, null, at);
    }

    // EVERY instance of a kind, which is what "remove this widget from the
    // desktop" means where the question is about the kind rather than about a
    // copy - Settings' widget cards, the extension list. One history entry for
    // the lot, so undoing brings them all back.
    function removeWidgetFromDesktop(widgetId) {
        const list = root.activeWidgets;
        const ids = [];
        for (let i = 0; i < list.length; i++) {
            if (list[i].widgetId === widgetId)
                ids.push(list[i].id);
        }
        if (ids.length === 0)
            return;
        GlobalStates.editHistoryBeginBatch();
        for (const id of ids)
            root.removeWidgetInstance(id);
        GlobalStates.editHistoryEndBatch();
    }

    // The catalogue's minus, beside its plus: one copy off rather than the
    // whole kind. The LAST one placed, which is the copy the plus next to it
    // just made - a row that adds at one end and removes at the other would
    // be two different widgets to anyone watching the desktop.
    function removeLastWidgetInstance(widgetId) {
        const list = root.activeWidgets;
        for (let i = list.length - 1; i >= 0; i--) {
            if (list[i].widgetId === widgetId) {
                root.removeWidgetInstance(list[i].id);
                return;
            }
        }
    }

    // With a monitor name the write lands in that monitor's fork, created from
    // the values it currently shows; without one it lands in the legacy x/y
    // that every monitor without a fork follows (see WidgetPlacement).
    function updateWidgetPosition(instanceId, newX, newY, monitorName, lock = false) {
        root._editWidgetEntry(instanceId, entry => {
            if (lock)
                WidgetPlacement.claimLockPlacement(entry, monitorName);
            WidgetPlacement.setPosition(entry, monitorName, newX, newY, lock);
        });
    }

    // "Use desktop layout": every entry's lock fork on `monitorName` goes, as
    // one history entry, so the lock screen follows the desktop there again.
    // A custom widget left with no lock fork anywhere goes back to "keep".
    function clearWidgetLockPositions(monitorName) {
        if (!monitorName)
            return;
        const list = root.options.background.activeWidgets || [];
        const ids = list.filter(e => WidgetPlacement.fork(e, monitorName, true) !== null
            || (e.lockBehavior === "custom" && !e.lockPositions)).map(e => e.id);
        if (ids.length === 0)
            return;
        GlobalStates.editHistoryBeginBatch();
        for (const id of ids)
            root._editWidgetEntry(id, entry => {
                WidgetPlacement.clearFork(entry, monitorName, true);
                if (entry.lockBehavior === "custom" && !entry.lockPositions)
                    entry.lockBehavior = "keep";
            });
        GlobalStates.editHistoryEndBatch();
    }

    function updateWidgetLockBehavior(instanceId, newLockBehavior) {
        root._editWidgetEntry(instanceId, entry => {
            entry.lockBehavior = newLockBehavior;
        });
    }

    function updateWidgetScale(instanceId, newScale, monitorName, lock = false) {
        root._editWidgetEntry(instanceId, entry => {
            if (lock)
                WidgetPlacement.claimLockPlacement(entry, monitorName);
            WidgetPlacement.setScale(entry, monitorName, newScale, lock);
        });
    }

    // A pinned widget ignores drags and the resize grip whatever the global
    // lock says. Stored only when set, so untouched entries stay as they were.
    function updateWidgetPinned(instanceId, pinned) {
        root._editWidgetEntry(instanceId, entry => {
            if (pinned)
                entry.pinned = true;
            else
                delete entry.pinned;
        });
    }

    function migrateRoundingConfig() {
        if (root.options.appearance.roundingValue >= 0)
            return;
        var oldMode = root.options.appearance.globalRounding || "large";
        if (oldMode === "sharp") {
            root.options.appearance.roundingValue = 0;
        } else if (oldMode === "normal") {
            root.options.appearance.roundingValue = 17;
        } else if (oldMode === "verylarge") {
            root.options.appearance.roundingValue = 32;
        } else {
            root.options.appearance.roundingValue = 24;
        }
        root.options.appearance.sharpMode = (root.options.appearance.roundingValue === 0);
    }

    function syncAppLaunchAnimation() {
        if (!root.options || !root.options.appearance || !root.options.appearance.appLaunchAnimation)
            return;
        HyprlandSettings.updateAppLaunchAnimation(root.options.appearance.appLaunchAnimation);
    }

    function migrateWidgetLockBehavior() {
        // Same trap as the widget migration: an unloaded Persistent reports "not migrated yet".
        if (!Persistent.ready || Persistent.states.background.lockBehaviorMigrated)
            return;
        let cloned = JSON.parse(JSON.stringify(root.options.background.activeWidgets || []));
        let changed = false;
        for (let i = 0; i < cloned.length; i++) {
            if (!cloned[i].lockBehavior) {
                cloned[i].lockBehavior = "hide";
                changed = true;
            }
        }
        if (changed) {
            root.options.background.activeWidgets = cloned;
        }
        Persistent.states.background.lockBehaviorMigrated = true;
    }

    // ── Schema versioning & type-drift repair ─────────────────────────────────
    // JsonAdapter coerces rather than rejects, and the damaging coercions are
    // silent: an int landing on a string property becomes "1" and matches no
    // branch; a string landing on a list property is split into one entry per
    // character ("left" -> ["l","e","f","t"]); a list of objects landing on
    // list<string> becomes ["", ""]. The next write persists the wreckage, so
    // the damage outlives the version that caused it, which is why the only fix
    // users find is deleting config.json outright.
    //
    // Both passes below work on the raw file rather than the adapter, because
    // by the time the adapter is readable the original values are gone.
    // Migration runs first so a retyped key can be converted; whatever is still
    // unrepresentable afterwards is replaced with its schema default.
    //
    // Bump `currentConfigVersion` and add a matching block to `migrateRaw()`
    // whenever an existing key changes type or meaning.
    readonly property int currentConfigVersion: 29
    // Defaults have to be captured before the file lands, because deserializing
    // is what destroys them. FileView loads asynchronously, so at component
    // completion the adapter still holds nothing but the QML defaults.
    property var defaultOptions: null
    property bool configRepaired: false
    // True while config.json fails to parse as JSON at all. Writes stay
    // blocked the whole time (see saveOptionsNow() and fileWriteTimer)
    // because the in-memory adapter holds nothing but QML defaults, and the
    // broken file on disk is the only remaining source of truth.
    property bool configMalformed: false
    // Drives ConfigHealthBanner: "ok" | "malformed" | "recovered" | "migrated" | "repaired" | "unknownKeys" | "reset"
    property string configHealthState: "ok"
    property list<string> configHealthKeys: []

    function snapshotDefaults(source) {
        if (source === null || typeof source !== "object")
            return source;
        if (root.isArrayLike(source)) {
            let copy = [];
            for (let i = 0; i < source.length; ++i) {
                copy.push(root.snapshotDefaults(source[i]));
            }
            return copy;
        }
        let copy = {};
        for (const key in source) {
            if (key === "objectName")
                continue;
            const value = source[key];
            if (typeof value === "function")
                continue;
            copy[key] = root.snapshotDefaults(value);
        }
        return copy;
    }

    // A config written before versioning existed has no `configVersion` key, so
    // those files read back as 0 and every migration runs. A fresh config is
    // stamped with the current version when defaults are seeded, so it
    // correctly skips all of them.
    function migrateRaw(raw) {
        const from = raw.configVersion ?? 0;
        if (from >= root.currentConfigVersion)
            return false;

        // v0 -> v1: sidebar.position went from int to string ("more
        // understandable config naming", Jan 2026). Old files still hold 0-3,
        // which JsonAdapter turns into the strings "0".."3" — matching nothing,
        // so every sidebar position silently behaved as "default".
        if (from < 1 && raw.sidebar !== undefined && typeof raw.sidebar.position === "number") {
            const legacyPositions = ["default", "inverted", "left", "right"];
            const previous = raw.sidebar.position;
            if (previous >= 0 && previous < legacyPositions.length) {
                raw.sidebar.position = legacyPositions[previous];
                console.log(`[Config] Migrated sidebar.position ${previous} -> ${raw.sidebar.position}`);
            }
        }

        // v1 -> v2: the tiling assistant stopped inferring drags from window
        // motion (Aug 2026), so tiling.detection.idleHz changed meaning from
        // "how fast the cursor is tracked" to "how often the active window is
        // sampled" - the far slower job the old idleFloorHz already described.
        // Carrying the old 30 over would poll six times harder than anything
        // now asks for. The two keys that only existed for the heuristic go.
        if (from < 2 && raw.tiling?.detection !== undefined) {
            const detection = raw.tiling.detection;
            if (typeof detection.idleFloorHz === "number")
                detection.idleHz = detection.idleFloorHz;
            else if (typeof detection.idleHz === "number")
                detection.idleHz = Math.min(detection.idleHz, 5);
            delete detection.idleFloorHz;
            delete detection.useMotionHeuristic;
            console.log(`[Config] Migrated tiling.detection to keybind-only drag detection (idleHz ${detection.idleHz})`);
        }

        // v2 -> v3: the dashboard quick toggles changed from one flat list
        // (`android.toggles`) to pages (`android.pages`). Keep old layouts
        // editable instead of silently dropping the legacy key on the next save.
        if (from < 3 && raw.sidebar?.quickToggles?.android !== undefined) {
            const android = raw.sidebar.quickToggles.android;
            if (android.pages === undefined && Array.isArray(android.toggles)) {
                android.pages = [android.toggles];
                console.log("[Config] Migrated sidebar.quickToggles.android.toggles to pages");
            }
            delete android.toggles;
        }

        // v3 -> v4: osk.layout keeps its type but changes meaning (Aug 2026). It used to be
        // overwritten by HyprlandXkb on every layout switch, so no value on disk was ever a
        // deliberate choice - and the shipped default "qwerty_full" named no layout at all,
        // which is why the keyboard always drew the same one. "auto" says out loud what the
        // overwriting was doing, and the keyboard settings now offer a real choice to anyone
        // who would rather pin a layout.
        if (from < 4 && typeof raw.osk?.layout === "string" && raw.osk.layout !== "auto") {
            console.log(`[Config] Migrated osk.layout "${raw.osk.layout}" -> "auto"`);
            raw.osk.layout = "auto";
        }

        // v4 -> v5: Settings now has one performance switch instead of two
        // independent rendering switches. Older presets predate the new
        // switch, so default them to the safe, low-overhead path. Users can
        // still opt back into scroll effects after the migration.
        if (from < 5 && raw.appearance !== undefined && raw.appearance !== null && typeof raw.appearance === "object" && !Array.isArray(raw.appearance) && raw.appearance.settingsPerformanceMode === undefined) {
            raw.appearance.settingsPerformanceMode = true;
            console.log(`[Config] Migrated Settings performance mode to ${raw.appearance.settingsPerformanceMode}`);
        }

        // v5 -> v6: quick-toggle pages become canonical records with stable
        // identity and explicit dimensions. The normalizer is shared with the
        // sidebar so migration and runtime cannot disagree about defaults,
        // duplicate handling, or allowed sizes.
        // The v7 branch of this PR used the same version number for the AI
        // schema, so a missing layoutVersion remains a reliable migration cue.
        if (raw.sidebar?.quickToggles?.android !== undefined && (from < 6 || raw.sidebar.quickToggles.android.layoutVersion !== 2)) {
            const android = raw.sidebar.quickToggles.android;
            android.pages = QuickToggleCatalog.normalizePages(android.pages, android.columns, {
                warn: function (message) {
                    console.warn(message);
                }
            });
            android.layoutVersion = 2;
            console.log("[Config] Migrated sidebar.quickToggles.android to canonical layout records");
        }

        // Originally v6 -> v7: the bar gained the Modes & Routines indicator.
        // It hides itself while no mode is active, so appending it to an
        // existing layout changes nothing visible until a mode starts.
        if (from < 8 && raw.bar?.layouts !== undefined && raw.bar.layouts !== null && typeof raw.bar.layouts === "object") {
            const layouts = raw.bar.layouts;
            const sections = ["left", "center", "right"];
            const present = sections.some(k => Array.isArray(layouts[k]) && layouts[k].some(e => e && e.id === "mode_indicator"));
            if (!present) {
                if (!Array.isArray(layouts.left))
                    layouts.left = [];
                const after = layouts.left.findIndex(e => e && e.id === "record_indicator");
                const entry = {
                    "centered": false,
                    "id": "mode_indicator",
                    "visible": false
                };
                layouts.left.splice(after === -1 ? layouts.left.length : after + 1, 0, entry);
                console.log("[Config] Migrated bar layout: added mode_indicator");
            }
        }

        // v7 -> v8: reconcile settings that were introduced independently by
        // the AI rebuild and the dev branch. The checks are intentionally
        // shape-based so users already at either former v7 receive only the
        // migration their file is still missing.
        if (from < 8) {
            const ai = raw.ai;
            if (ai !== undefined && ai !== null && typeof ai === "object" && !Array.isArray(ai)) {
                if (typeof ai.tool === "string" || typeof ai.localModelTools === "boolean") {
                    const tools = (ai.tools !== undefined && ai.tools !== null && typeof ai.tools === "object" && !Array.isArray(ai.tools)) ? ai.tools : {};
                    if (typeof ai.tool === "string" && tools.mode === undefined)
                        tools.mode = ai.tool;
                    if (typeof ai.localModelTools === "boolean" && tools.localModels === undefined)
                        tools.localModels = ai.localModelTools;
                    ai.tools = tools;
                    console.log(`[Config] Migrated ai.tool "${ai.tool}" -> ai.tools.mode`);
                }
                delete ai.tool;
                delete ai.localModelTools;

                if (ai.customModels === undefined && (Array.isArray(ai.models) || Array.isArray(ai.otherModels))) {
                    const merged = [];
                    for (const group of (ai.models ?? [])) {
                        if (group === null || typeof group !== "object")
                            continue;
                        for (const providerId in group) {
                            const entries = group[providerId];
                            if (!Array.isArray(entries))
                                continue;
                            for (const entry of entries) {
                                if (entry === null || typeof entry !== "object")
                                    continue;
                                const moved = Object.assign({}, entry);
                                moved.provider = providerId;
                                merged.push(moved);
                            }
                        }
                    }
                    for (const entry of (ai.otherModels ?? [])) {
                        if (entry === null || typeof entry !== "object")
                            continue;
                        merged.push(entry);
                    }
                    ai.customModels = merged;
                    console.log(`[Config] Merged ${merged.length} custom AI model(s) into ai.customModels`);
                }
                delete ai.models;
                delete ai.otherModels;
            }
            if (raw.sidebar?.ai !== undefined && raw.sidebar.ai !== null) {
                delete raw.sidebar.ai.showProviderAndModelButtons;
                if (raw.ai === undefined || raw.ai === null || typeof raw.ai !== "object" || Array.isArray(raw.ai))
                    raw.ai = {};
                if (raw.ai.indexAtStartup === undefined && typeof raw.sidebar.ai.enable === "boolean")
                    raw.ai.indexAtStartup = raw.sidebar.ai.enable;
                delete raw.sidebar.ai.enable;
            }
            console.log("[Config] Reconciled AI settings and sidebar startup policy");
        }

        // v8 -> v9: the bar gained the dictation indicator. Like the recording
        // one it takes no space until dictation is actually running, so adding
        // it to an existing layout is invisible to anyone who never turns
        // dictation on.
        if (from < 9 && raw.bar?.layouts !== undefined && raw.bar.layouts !== null && typeof raw.bar.layouts === "object") {
            const dictationLayouts = raw.bar.layouts;
            const dictationSections = ["left", "center", "right"];
            const dictationPresent = dictationSections.some(k => Array.isArray(dictationLayouts[k]) && dictationLayouts[k].some(e => e && e.id === "dictation_indicator"));
            if (!dictationPresent) {
                if (!Array.isArray(dictationLayouts.left))
                    dictationLayouts.left = [];
                const afterRecord = dictationLayouts.left.findIndex(e => e && e.id === "record_indicator");
                const entry = {
                    "centered": false,
                    "id": "dictation_indicator",
                    "visible": true
                };
                dictationLayouts.left.splice(afterRecord === -1 ? dictationLayouts.left.length : afterRecord + 1, 0, entry);
                console.log("[Config] Migrated bar layout: added dictation_indicator");
            }
        }

        // v9 -> v10: dictation defaults to pasting rather than typing. Typing
        // synthesises one keystroke per character, which several applications
        // drop under load — the words arrive a letter short. Only the old
        // default is moved; anyone who picked "clipboard" keeps it.
        if (from < 10 && raw.dictation !== undefined && raw.dictation !== null && typeof raw.dictation === "object" && raw.dictation.outputMode === "type") {
            raw.dictation.outputMode = "paste";
            console.log("[Config] Migrated dictation output mode: type -> paste");
        }

        // v10 -> v11: screen recording quality stops being a raw bitrate. Mbps
        // means nothing without knowing the resolution it is spent on, so the
        // recorder now derives it from the picture size, the frame rate and a
        // three-step quality choice. An existing bitrate is read as the intent
        // behind it and mapped onto that choice.
        if (from < 11 && raw.screenRecord !== undefined && raw.screenRecord !== null && typeof raw.screenRecord === "object" && typeof raw.screenRecord.bitrate === "number") {
            const oldBitrate = raw.screenRecord.bitrate;
            raw.screenRecord.quality = oldBitrate <= 6 ? "low" : (oldBitrate >= 16 ? "high" : "balanced");
            delete raw.screenRecord.bitrate;
            console.log(`[Config] Migrated screen recording bitrate ${oldBitrate} Mbps -> quality "${raw.screenRecord.quality}"`);
        }

        // v9 -> v10: Search v2 keeps its lightweight content lists in a
        // stable schema. Existing users get the new objects without changing
        // their enabled modules, aliases, or prefix choices.
        if (from < 10) {
            if (raw.search === undefined || raw.search === null || typeof raw.search !== "object")
                raw.search = {};
            if (raw.search.favorites === undefined)
                raw.search.favorites = {
                    enable: true
                };
            if (raw.search.fallbacks === undefined)
                raw.search.fallbacks = {
                    enable: true,
                    actions: ["ai", "web", "tasks", "calendar"]
                };
            if (raw.search.history === undefined)
                raw.search.history = {
                    enable: true,
                    maxItems: 50
                };
            if (raw.search.keybindings === undefined)
                raw.search.keybindings = [
                    {
                        actionId: "actions",
                        shortcut: "Ctrl+K"
                    },
                    {
                        actionId: "favorite",
                        shortcut: "Ctrl+P"
                    },
                    {
                        actionId: "historyPrevious",
                        shortcut: "Up"
                    },
                    {
                        actionId: "historyNext",
                        shortcut: "Down"
                    },
                    {
                        actionId: "secondary",
                        shortcut: "Ctrl+Enter"
                    },
                    {
                        actionId: "copy",
                        shortcut: "Ctrl+C"
                    },
                    {
                        actionId: "save",
                        shortcut: "Ctrl+S"
                    },
                    {
                        actionId: "edit",
                        shortcut: "Ctrl+E"
                    },
                    {
                        actionId: "ocr",
                        shortcut: "Ctrl+O"
                    },
                    {
                        actionId: "create",
                        shortcut: "Ctrl+N"
                    },
                    {
                        actionId: "copyDispatch",
                        shortcut: "Ctrl+Shift+K"
                    },
                    {
                        actionId: "delete",
                        shortcut: "Shift+Delete"
                    },
                    {
                        actionId: "section",
                        shortcut: "Tab"
                    },
                    {
                        actionId: "select",
                        shortcut: "Ctrl+Space"
                    },
                    {
                        actionId: "cut",
                        shortcut: "Ctrl+X"
                    },
                    {
                        actionId: "paste",
                        shortcut: "Ctrl+V"
                    },
                    {
                        actionId: "createFolder",
                        shortcut: "Ctrl+Shift+N"
                    },
                    {
                        actionId: "duplicate",
                        shortcut: "Ctrl+D"
                    },
                    {
                        actionId: "toggleHidden",
                        shortcut: "Ctrl+H"
                    },
                    {
                        actionId: "refresh",
                        shortcut: "Ctrl+R"
                    },
                    {
                        actionId: "stageCopy",
                        shortcut: "Ctrl+Shift+C"
                    },
                    {
                        actionId: "sortFiles",
                        shortcut: "Ctrl+Shift+S"
                    },
                    {
                        actionId: "goHome",
                        shortcut: "Ctrl+Home"
                    },
                    {
                        actionId: "forward",
                        shortcut: "Alt+Right"
                    }
                ];
            console.log("[Config] Added Search v2 content defaults");
        }

        // v10 -> v11: normal Search can index bookmarks and selected history
        // from the default Mozilla-compatible browser profile. Existing result
        // orders receive the new section exactly once; after this migration the
        // stored v11 order is authoritative, so removing Sites stays removed.
        if (from < 11) {
            if (raw.search === undefined || raw.search === null || typeof raw.search !== "object" || Array.isArray(raw.search))
                raw.search = {};
            if (raw.search.browserSites === undefined) {
                raw.search.browserSites = {
                    enable: true,
                    profilePath: "",
                    maxIndexedSites: 300,
                    maxResults: 6,
                    includeHistory: true,
                    useLocalFavicons: true,
                    allowRemoteFavicons: false,
                    refreshMinutes: 10
                };
            }
            const sectionOrder = raw.search.sectionOrder;
            if (Array.isArray(sectionOrder) && sectionOrder.length > 0) {
                const hasSites = sectionOrder.some(entry => String(entry?.id ?? entry) === "sites");
                if (!hasSites) {
                    const appsIndex = sectionOrder.findIndex(entry => String(entry?.id ?? entry) === "apps");
                    const settingsIndex = sectionOrder.findIndex(entry => String(entry?.id ?? entry) === "settings");
                    const insertAt = appsIndex >= 0 ? appsIndex + 1 : (settingsIndex >= 0 ? settingsIndex : sectionOrder.length);
                    sectionOrder.splice(insertAt, 0, {
                        "id": "sites"
                    });
                }
            }
            console.log("[Config] Added Browser Sites search provider");
        }

        // v11 -> v12: aliases became an explicit result class. Existing orders
        // predate the id and are otherwise authoritative, so without a one-time
        // insertion every upgraded user would keep filtering aliases out. Put
        // exact alias intent before every broader fuzzy result class.
        if (from < 12) {
            if (raw.search === undefined || raw.search === null || typeof raw.search !== "object" || Array.isArray(raw.search))
                raw.search = {};
            const sectionOrder = raw.search.sectionOrder;
            if (Array.isArray(sectionOrder) && sectionOrder.length > 0) {
                const hasAliases = sectionOrder.some(entry => String(entry?.id ?? entry) === "aliases");
                if (!hasAliases)
                    sectionOrder.unshift({
                        "id": "aliases"
                    });
            }
            console.log("[Config] Added Aliases search result group");
        }

        // v12 -> v13: the former Content result group mixed two independent
        // user-owned providers. Replace it in place so the surrounding priority
        // stays unchanged. If Content had been removed, both providers remain
        // disabled and are merely offered by the Settings add selector.
        if (from < 13) {
            if (raw.search === undefined || raw.search === null || typeof raw.search !== "object" || Array.isArray(raw.search))
                raw.search = {};
            const sectionOrder = raw.search.sectionOrder;
            if (Array.isArray(sectionOrder) && sectionOrder.length > 0) {
                const contentIndex = sectionOrder.findIndex(entry => String(entry?.id ?? entry) === "content");
                if (contentIndex >= 0) {
                    const hasQuicklinks = sectionOrder.some(entry => String(entry?.id ?? entry) === "quicklinks");
                    const hasTextSnippets = sectionOrder.some(entry => String(entry?.id ?? entry) === "textSnippets");
                    if (!hasQuicklinks && !hasTextSnippets)
                        sectionOrder.splice(contentIndex, 1, {
                            "id": "quicklinks"
                        }, {
                            "id": "textSnippets"
                        });
                    else
                        sectionOrder.splice(contentIndex, 1);
                }
            }
            console.log("[Config] Split Content search results into Quick links and Text snippets");
        }

        // v13 -> v14: idle Search (empty query) grew a dedicated frecency-ranked
        // "Suggestions" strip. It is idle-only — a typed query never populates
        // it — but it shares the same reorder/on-off list as every other
        // result class, so an upgraded order needs the id too.
        if (from < 14) {
            if (raw.search === undefined || raw.search === null || typeof raw.search !== "object" || Array.isArray(raw.search))
                raw.search = {};
            const sectionOrder = raw.search.sectionOrder;
            if (Array.isArray(sectionOrder) && sectionOrder.length > 0) {
                const hasSuggested = sectionOrder.some(entry => String(entry?.id ?? entry) === "suggested");
                if (!hasSuggested)
                    sectionOrder.unshift({
                        "id": "suggested"
                    });
            }
            console.log("[Config] Added idle Suggestions search result group");
        }

        // v14 -> v15: the sidebar avatar shape split off from the general
        // userProfile one. Existing configs kept a single shape for both, so
        // seed the new key from it to leave the sidebar looking untouched.
        if (from < 15 && typeof raw.userProfile?.avatarShape === "string") {
            if (raw.sidebar === undefined || raw.sidebar === null || typeof raw.sidebar !== "object" || Array.isArray(raw.sidebar))
                raw.sidebar = {};
            if (raw.sidebar.dashboardHeader === undefined || raw.sidebar.dashboardHeader === null || typeof raw.sidebar.dashboardHeader !== "object" || Array.isArray(raw.sidebar.dashboardHeader))
                raw.sidebar.dashboardHeader = {};
            if (typeof raw.sidebar.dashboardHeader.avatarShape !== "string") {
                raw.sidebar.dashboardHeader.avatarShape = raw.userProfile.avatarShape;
                console.log(`[Config] Seeded sidebar.dashboardHeader.avatarShape from userProfile.avatarShape (${raw.userProfile.avatarShape})`);
            }
        }

        // v15 -> v16: the idle now-playing row moved out of "More results" into
        // its own "Now playing" group. Every stored order predates that id, and
        // a section missing from the order is a section that never renders — so
        // without this the row silently disappeared the moment it was reclassed.
        if (from < 16) {
            if (raw.search === undefined || raw.search === null || typeof raw.search !== "object" || Array.isArray(raw.search))
                raw.search = {};
            const sectionOrder = raw.search.sectionOrder;
            if (Array.isArray(sectionOrder) && sectionOrder.length > 0) {
                const hasMedia = sectionOrder.some(entry => String(entry?.id ?? entry) === "media");
                if (!hasMedia) {
                    const aliasesIndex = sectionOrder.findIndex(entry => String(entry?.id ?? entry) === "aliases");
                    sectionOrder.splice(aliasesIndex >= 0 ? aliasesIndex + 1 : 0, 0, {
                        "id": "media"
                    });
                }
            }
            console.log("[Config] Added Now playing search result group");
        }

        // v16 -> v17: the tablet taskbar stopped hiding its app row on an occupied
        // workspace. The old default was `true`, so every file written before this
        // carries it explicitly — and a stored value always wins over a changed
        // default, which would have left the fix invisible to exactly the people who
        // reported the dock going blank. The reference product's taskbar is
        // persistent; anyone who wants the old behaviour turns it back on in Settings.
        if (from < 17 && raw.tablet?.dock?.autoHideOnOccupiedWorkspace === true) {
            raw.tablet.dock.autoHideOnOccupiedWorkspace = false;
            console.log("[Config] Migrated tablet.dock.autoHideOnOccupiedWorkspace -> false (persistent taskbar)");
        }

        // v17 -> v18: live draw joined the floating bubble's sheet. The action list is a
        // stored user ordering, so a new default reaches nobody who already has a config
        // — and the bubble is the only place this feature is discoverable from.
        if (from < 18 && Array.isArray(raw.tablet?.bubble?.actions)) {
            const actions = raw.tablet.bubble.actions;
            if (!actions.some(entry => String(entry ?? "") === "liveDraw")) {
                actions.unshift("liveDraw");
                console.log("[Config] Added liveDraw to the tablet bubble's actions");
            }
        }

        // v18 -> v19: cache follows the last cheatsheet tab, not only Keybinds.
        // Preserve an explicit opt-out and prefer an already configured new key.
        if (from < 19 && raw.cheatsheet && typeof raw.cheatsheet === "object" && !Array.isArray(raw.cheatsheet)) {
            const cheatsheet = raw.cheatsheet;
            if (cheatsheet.keepLastTabLoaded === undefined && typeof cheatsheet.keepKeybindsLoaded === "boolean")
                cheatsheet.keepLastTabLoaded = cheatsheet.keepKeybindsLoaded;
            delete cheatsheet.keepKeybindsLoaded;
        }

        // v19 -> v20: the updater's script path and flags were never read by
        // anything; stripping them here keeps the "unrecognized settings"
        // notice from greeting everyone after this update.
        if (from < 20 && raw.update && typeof raw.update === "object" && !Array.isArray(raw.update)) {
            delete raw.update.scriptPath;
            delete raw.update.scriptFlags;
        }

        // v20 -> v21: inline file results became the default. Every config
        // written before this carries the old `false` default explicitly, so
        // only a new default would never reach an existing install.
        if (from < 21 && raw.search?.fileSearch && typeof raw.search.fileSearch === "object" && !Array.isArray(raw.search.fileSearch))
            raw.search.fileSearch.inlineResults = true;

        // v21 -> v22: Settings now always unloads on close; remove the former
        // keep-warm preference rather than reporting it as an unknown key.
        if (from < 22)
            delete raw.settingsApp;

        // v22 -> v23: the Dynamic Island moves out of `bar.floatingNotch` and into its
        // own top-level block. The old schema was ~40 flat keys under the bar - one
        // `disable<Widget>` and one `height<Widget>` per activity, with the two island
        // styles sharing them - which is why "is this widget on?" was answered
        // differently in different places. The new shape is per-activity objects, so a
        // widget's settings live together and the settings page can be generated.
        //
        // Existing users keep the notch: `style` is only "pills" for a fresh install.
        if (from < 23 && raw.bar?.floatingNotch && typeof raw.bar.floatingNotch === "object"
                && !Array.isArray(raw.bar.floatingNotch)) {
            const old = raw.bar.floatingNotch;
            const island = (raw.dynamicIsland && typeof raw.dynamicIsland === "object"
                && !Array.isArray(raw.dynamicIsland)) ? raw.dynamicIsland : {};

            if (island.enable === undefined)
                island.enable = old.enable === true || old.centerInBar === true;
            if (island.style === undefined)
                island.style = "notch";

            island.monitor = island.monitor ?? {};
            if (island.monitor.followFocus === undefined)
                island.monitor.followFocus = old.onlyShowOnSingleMonitor !== true;
            if (island.monitor.name === undefined)
                island.monitor.name = old.singleMonitorName ?? "";

            island.behavior = island.behavior ?? {};
            if (island.behavior.autoHide === undefined)
                island.behavior.autoHide = old.autoHide === true;

            island.appearance = island.appearance ?? {};
            if (island.appearance.dropShadow === undefined)
                island.appearance.dropShadow = old.dropShadow === true;

            island.notch = island.notch ?? {};
            for (const [to, key] of [["centerInBar", "centerInBar"],
                                     ["clickToExpand", "clickToExpand"], ["singleWidgetExpanded", "singleWidgetExpanded"]]) {
                if (island.notch[to] === undefined && old[key] !== undefined)
                    island.notch[to] = old[key] === true;
            }

            // `disable<Widget>` inverts into `widgets.<id>.enable`. The per-widget
            // heights are not copied: v25 deletes them with the sliders that wrote them.
            const widgetKeys = {
                "workspaces": "Workspaces", "keyboard": "Keyboard", "wifi": "Wifi",
                "bluetooth": "Bluetooth", "media": "Media", "notification": "Notification",
                "osd": "Osd", "recording": "Recording", "dictation": "Dictation",
                "timer": "Timer", "clipboard": "Clipboard", "localSend": "LocalSend",
                "progress": "Progress", "battery": "Battery", "ai": "AiStatus",
                "earbuds": "Earbuds", "weather": "Weather", "batteryGlance": "BatteryGlance"
            };
            island.widgets = island.widgets ?? {};
            for (const id in widgetKeys) {
                const suffix = widgetKeys[id];
                const entry = island.widgets[id] ?? {};
                if (entry.enable === undefined && old["disable" + suffix] !== undefined)
                    entry.enable = old["disable" + suffix] !== true;
                island.widgets[id] = entry;
            }
            if (old.disableKdeConnectInLocalSend !== undefined) {
                const ls = island.widgets.localSend ?? {};
                if (ls.kdeConnectColumn === undefined)
                    ls.kdeConnectColumn = old.disableKdeConnectInLocalSend !== true;
                island.widgets.localSend = ls;
            }

            // Extra Compact was dropped with the panel it was written for: the engine
            // sizes the notch from its activities instead of scaling the whole surface.
            delete old.extraCompact;

            raw.dynamicIsland = island;
            // `bar.floatingNotch` is deliberately kept for now: the legacy notch surface
            // and the current settings page still read and write it, so deleting it here
            // would drop an existing user's island settings on the floor. The new block
            // is written first and becomes authoritative when those surfaces are ported
            // (IslandPolicy.useModernSchema); the old one is removed with them.
            console.log("[Config] Migrated bar.floatingNotch -> dynamicIsland");
        }

        // v23 -> v24: Extra Compact is gone. It scaled the whole notch to fake a
        // smaller island, which the engine has no need for - it sizes the surface from
        // the activity it is showing. A config that already reached v23 still carries
        // the key, and leaving it would have ConfigHealthBanner report it as unknown.
        if (from < 24) {
            if (raw.bar?.floatingNotch) {
                delete raw.bar.floatingNotch.extraCompact;
                // The transitional switch between the engine notch and the panel; the
                // panel is gone, so there is nothing left to switch.
                delete raw.bar.floatingNotch.useEngineNotch;
            }
            if (raw.dynamicIsland?.notch)
                delete raw.dynamicIsland.notch.extraCompact;
        }

        // v24 -> v25: the island's dead knobs are deleted, not just hidden.
        // The checklist, calendar and audio "notches" have no activity in the
        // registry and no source - their toggles wrote keys nothing read. The
        // per-widget contracted heights are gone too: the faces that genuinely
        // need more than a pill (Bluetooth, notification, media, dictation,
        // progress) now declare that height in their registry descriptor, and
        // every other slider sat below the pill height where it could do
        // nothing. blurTransitions was copied by v23 and read by no one.
        if (from < 25) {
            const notch = raw.bar?.floatingNotch;
            if (notch) {
                for (const k of ["disableChecklist", "checklistAlwaysVisible",
                                 "checklistOnlyExpanded", "disableCalendar",
                                 "disableAudio", "blurTransitions"])
                    delete notch[k];
                for (const k of Object.keys(notch))
                    if (/^height[A-Z]/.test(k)) delete notch[k];
            }
            const widgets = raw.dynamicIsland?.widgets;
            if (widgets) {
                delete widgets.checklist;
                delete widgets.calendar;
                delete widgets.audio;
            }
            if (raw.dynamicIsland?.appearance)
                delete raw.dynamicIsland.appearance.blurTransitions;
            console.log("[Config] Dropped dead dynamic island keys (v25)");
        }

        // v25 -> v26: the grace period only covers sources that flicker (apps,
        // players, devices) now, so the old 20 s default is far too long for them.
        // A value the user picked themselves is kept.
        if (from < 26) {
            if (raw.modes?.graceSec === 20)
                raw.modes.graceSec = 5;
        }

        // v26 -> v27: "keep" means "where it is on the desktop" again, and a
        // lock placement of its own is the explicit "custom" behavior. Widgets
        // that were already dragged on the Lockscreen tab carry a lock fork;
        // they become "custom" so their lock layout does not move.
        if (from < 27) {
            const widgets = raw.background?.activeWidgets;
            if (Array.isArray(widgets)) {
                for (const w of widgets) {
                    if (w && w.lockBehavior === "keep" && w.lockPositions
                            && typeof w.lockPositions === "object" && Object.keys(w.lockPositions).length > 0)
                        w.lockBehavior = "custom";
                }
            }
        }

        // v27 -> v28: the bar gained the EasyEffects preset indicator. Like the mode
        // indicator it takes no room while EasyEffects is not running, so it joins
        // existing layouts beside that one without changing what anybody sees.
        if (from < 28 && raw.bar?.layouts !== undefined && raw.bar.layouts !== null && typeof raw.bar.layouts === "object") {
            const layouts = raw.bar.layouts;
            const sections = ["left", "center", "right"];
            const present = sections.some(k => Array.isArray(layouts[k]) && layouts[k].some(e => e && e.id === "easyeffects_indicator"));
            if (!present) {
                if (!Array.isArray(layouts.left))
                    layouts.left = [];
                const after = layouts.left.findIndex(e => e && e.id === "mode_indicator");
                layouts.left.splice(after === -1 ? layouts.left.length : after + 1, 0, {
                    "centered": false,
                    "id": "easyeffects_indicator",
                    "visible": false
                });
                console.log("[Config] Migrated bar layout: added easyeffects_indicator");
            }
        }

        // v28 -> v29: the game overlay gained the performance HUD. Existing
        // taskbars get its button; nothing opens until the user picks it.
        if (from < 29 && Array.isArray(raw.overlay?.buttons) && !raw.overlay.buttons.includes("perfMonitor")) {
            raw.overlay.buttons.push("perfMonitor");
            console.log("[Config] Migrated overlay buttons: added perfMonitor");
        }

        raw.configVersion = root.currentConfigVersion;
        console.log(`[Config] Migrated config schema ${from} -> ${root.currentConfigVersion}`);
        return true;
    }

    // list<var>/list<string> are QML sequences: indexable with a numeric length,
    // but Array.isArray is false for them.
    function isArrayLike(value) {
        return typeof value === "object" && value !== null && typeof value.length === "number";
    }

    // The adapter is the type oracle: whatever a value was coerced into, its
    // type is by definition the type the schema declares.
    function typesConflict(rawValue, qmlValue) {
        if (rawValue === null || rawValue === undefined)
            return false;
        // Unknown keys have no counterpart to conflict with. JsonAdapter drops
        // them on the next write anyway.
        if (qmlValue === undefined || qmlValue === null)
            return false;

        const rawIsArray = Array.isArray(rawValue);
        const qmlIsArray = root.isArrayLike(qmlValue);
        if (rawIsArray !== qmlIsArray)
            return true;

        if (rawIsArray) {
            // Coercion is positional and length-preserving, so mismatched
            // lengths mean this is not a coerced copy and nothing can be
            // concluded. Equal lengths let each element be compared in place,
            // which is what catches [{...}] collapsing into [""].
            if (rawValue.length !== qmlValue.length)
                return false;
            for (let i = 0; i < rawValue.length; ++i) {
                if (typeof rawValue[i] !== typeof qmlValue[i])
                    return true;
            }
            return false;
        }

        const rawIsObject = typeof rawValue === "object";
        const qmlIsObject = typeof qmlValue === "object";
        if (rawIsObject !== qmlIsObject)
            return true;
        if (rawIsObject)
            return false;

        // Scalars. number<->bool is deliberately allowed: 0/1 for a bool is a
        // plausible hand-edit and coerces to the right thing.
        if (typeof rawValue === "string" && typeof qmlValue === "number")
            return true;
        if (typeof rawValue === "number" && typeof qmlValue === "string")
            return true;
        if (typeof rawValue === "string" && typeof qmlValue === "boolean")
            return true;
        return false;
    }

    function repairTypeConflicts(rawObject, qmlObject, defaultObject, prefix, repaired) {
        for (const key in rawObject) {
            const rawValue = rawObject[key];
            const qmlValue = qmlObject[key];
            const defaultValue = defaultObject ? defaultObject[key] : undefined;
            const path = prefix ? `${prefix}.${key}` : key;

            if (root.typesConflict(rawValue, qmlValue)) {
                // Writing the default back rather than dropping the key means
                // the adapter picks the right value up from this same write; a
                // missing key would leave the coerced garbage in place.
                if (defaultValue === undefined)
                    continue;
                console.warn(`[Config] Resetting ${path} to its default: the stored value no longer matches the schema`);
                rawObject[key] = defaultValue;
                repaired.push(path);
                continue;
            }
            // Recurse into plain objects only — array elements are free-form.
            if (rawValue && typeof rawValue === "object" && !Array.isArray(rawValue) && qmlValue && typeof qmlValue === "object" && !root.isArrayLike(qmlValue)) {
                root.repairTypeConflicts(rawValue, qmlValue, defaultValue, path, repaired);
            }
        }
    }

    // Semantically wrong but syntactically valid values (a typo'd enum
    // string, a stale value from a renamed option, an out-of-range mode
    // number) pass typesConflict() untouched, since it only compares JS
    // types. Covers both string and integer enums — plenty of options here
    // are int-coded modes, and an unlisted number silently falls through
    // every branch that consumes it. Options with a continuous range rather
    // than a fixed value set (speedScale, gains, sizes) are deliberately
    // absent: they need bounds checking, not membership.
    //
    // An entry that is *missing* a legal value is worse than no entry at all:
    // it resets a setting the user deliberately chose. Every list below is
    // copied from the option set the corresponding ConfigSelectionArray in
    // modules/settings declares, so adding a mode to the UI means adding it
    // here too. Anything whose full value set can't be read off one such list
    // (background widget placementStrategy, which also accepts undocumented
    // aliases) is left out on purpose.
    readonly property var enumConstraints: ({
            "panelFamily": ["ii", "tablet", "waffle"],
            "ai.tools.mode": ["functions", "search", "none"],
            "policies.ai": [0, 1, 2],
            "policies.weeb": [0, 1, 2],
            "policies.wallpapers": [0, 1],
            "policies.translator": [0, 1, 2],
            "policies.player": [0, 1],
            "policies.phone": [0, 1],
            "phone.webcam.cameraFacing": ["front", "back"],
            "phone.webcam.resolution": ["640x480", "1280x720", "1920x1080"],
            "phone.webcam.rotateDegrees": [0, 90, 180, 270],
            "phone.webcam.connection": ["wifi", "usb"],
            "appearance.fakeScreenRounding": [0, 1, 2, 3, 4],
            "background.zoomOutStyle": [0, 1, 2],
            "background.overviewBackgroundStyle": ["", "gnome", "soft-focus", "camera-push", "depth", "card-lift", "desaturate", "directional", "material-shape"],
            "background.mediaMode.visualizerMode": [0, 1, 2, 3],
            "background.mediaMode.syllable.textHighlightStyle": [0, 1],
            "bar.cornerStyle": [0, 1, 2, 3],
            "bar.barGroupStyle": [0, 1, 2],
            "bar.barBackgroundStyle": [0, 1, 2, 3],
            "dock.dockStyle": ["floating", "islands", "hug", "dynamic_island", "transparent"],
            "bar.mediaPlayer.popupStyle": ["default", "expressive", "android"],
            "cheatsheet.aminoAcidScheme": ["five", "seven", "four"],
            "userProfile.imageStyle": ["initial", "expressive", "custom"],
            "lock.centerAlignment": ["vertical", "horizontal"],
            "lock.touchKeyboard.show": ["auto", "always", "never"],
            "lock.zoomAnimation.style": ["default", "gnome", "material-shape"],
            "tablet.gestures.sideEdges": ["back", "policies", "none"],
            "tablet.gestures.bottomEdge": ["android", "drawer"],
            "tablet.dock.backgroundStyle": ["none", "translucent", "solid"],
            "tablet.dock.appTapAction": ["focus", "launch"],
            "tablet.recents.layout": ["list", "grid"],
            "tablet.navigation.backKey": ["alt_left", "escape", "backspace", "browser_back", "custom"],
            "tablet.windows.floatMode": ["off", "all", "keepDialogs"],
            "lock.touchKeyboard.mode": ["text", "pin"],
            "lock.notifications.position": ["top_left", "top_right", "bottom_left", "bottom_right"],
            "lock.notifications.privacy": ["full", "redacted", "countOnly"],
            "lock.notifications.defaultPolicy": ["show", "hide"],
            "lock.notifications.filters.criticalOverride": ["full", "none"],
            // osk.layout is deliberately absent: its values are layout names out of layouts.js,
            // so a fixed list here would reject a layout added later. An unknown one already
            // falls back to the keyboard's default.
            "osk.style": ["deck", "classic"],
            "sidebar.position": ["default", "inverted", "left", "right"],
            "sidebar.sidebarStyle": ["default", "connect"],
            "sidebar.dashboardHeader.profileImageType": ["user_profile", "distro", "none"],
            "sidebar.dashboardHeader.textMode": ["username", "uptime", "none", "custom"],
            "sidebar.dashboardHeader.buttonsDesign": ["default", "circles"],
            "sounds.notificationDefaultPolicy": ["play", "mute"],
            "search.typingTest.mode": ["time", "words", "zen"],
            "search.typingTest.wordlistSize": ["standard", "extended"],
            "search.typingTest.caretStyle": ["line", "block", "underline", "off"],
            "search.typingTest.keyboard.layout": ["qwerty", "qwertz", "azerty", "dvorak", "colemak", "vial"],
            "search.typingTest.sounds.theme": ["click1", "click2", "click3", "click4", "click5", "click6", "click7"],
            "search.typingTest.sounds.errorTheme": ["error1", "error2", "error3", "error4"],
            "search.speedTest.mode": ["both", "download", "upload"],
            "search.speedTest.unit": ["mbps", "mBps"],
            "time.firstDayOfWeek": [0, 1, 2, 3, 4, 5, 6]
        })

    function getNestedValue(obj, keys) {
        let node = obj;
        for (const key of keys) {
            if (node === undefined || node === null || typeof node !== "object")
                return undefined;
            node = node[key];
        }
        return node;
    }

    function repairEnumViolations(raw, repaired) {
        for (const path in root.enumConstraints) {
            const allowed = root.enumConstraints[path];
            const keys = path.split(".");
            let node = raw;
            let parent = null;
            let lastKey = null;
            let missing = false;
            for (const key of keys) {
                if (node === undefined || node === null || typeof node !== "object") {
                    missing = true;
                    break;
                }
                parent = node;
                lastKey = key;
                node = node[key];
            }
            // Key absent from this file (older config, or genuinely unset):
            // nothing to validate. Strings and numbers are both checked; a
            // value of any other type on this same path is a type conflict,
            // already caught by repairTypeConflicts(). Bools are excluded
            // deliberately — `true` would otherwise match a `1` entry.
            const checkable = typeof node === "string" || typeof node === "number";
            if (missing || !checkable || allowed.includes(node))
                continue;

            const defaultValue = root.getNestedValue(root.defaultOptions, keys);
            if (defaultValue === undefined)
                continue;
            console.warn(`[Config] Resetting ${path} to its default: ${JSON.stringify(node)} is not a recognized value`);
            parent[lastKey] = defaultValue;
            repaired.push(path);
        }
    }

    // JsonAdapter has no property to bind an unrecognized key to, so it
    // ignores the key on read and omits it on the next write (serializeRec
    // builds a fresh object from declared properties only). A typo'd key —
    // "sidebar.postion" — therefore does nothing, warns about nothing, and
    // then quietly disappears, which is the single most common "why is my
    // setting not applying" case. Nothing here deletes anything: the drop is
    // already the runtime's behaviour. This only finds them so they can be
    // reported while the file still contains them.
    function collectUnknownKeys(rawObject, defaultObject, prefix, unknown) {
        for (const key in rawObject) {
            const path = prefix ? `${prefix}.${key}` : key;
            if (!(key in defaultObject)) {
                unknown.push(path);
                continue;
            }
            const rawValue = rawObject[key];
            const defaultValue = defaultObject[key];
            // Descend into plain objects on both sides only. Array entries are
            // free-form, and an empty default object is indistinguishable from
            // a free-form map, so neither can be checked against a schema.
            if (!rawValue || typeof rawValue !== "object" || Array.isArray(rawValue))
                continue;
            if (!defaultValue || typeof defaultValue !== "object" || root.isArrayLike(defaultValue))
                continue;
            if (Object.keys(defaultValue).length === 0)
                continue;
            root.collectUnknownKeys(rawValue, defaultValue, path, unknown);
        }
    }

    // Returns true when the file needed rewriting. Repair runs at most once per
    // session: it is a one-time upgrade step, and capping it removes any chance
    // of a write/reload loop.
    function repairConfigFile() {
        // Malformed-JSON detection runs on every load, independent of the
        // configRepaired cap below — that cap only limits the one-time
        // migration/type-repair pass, and gating parse detection on it too
        // would mean a corruption introduced *after* the first repair goes
        // undetected for the rest of the session.
        let raw;
        try {
            raw = JSON.parse(configFileView.text());
        } catch (e) {
            // JsonAdapter's own C++ parser swallows this exact same error
            // with a qmlWarning and treats the load as successful, so
            // onLoaded still fires — with the adapter holding nothing but
            // QML defaults. Without handleMalformedConfig() blocking writes
            // here, the first user-triggered save clobbers the real (broken
            // but recoverable) file with those defaults.
            root.handleMalformedConfig(e);
            return false;
        }
        // Parsing is necessary but not sufficient: deserializeAdapter() also
        // bails on `!json.isObject()`, so a file holding a bare string,
        // number or array leaves the adapter on pure QML defaults exactly
        // like a syntax error does — and would otherwise reach
        // recoverFromMalformedConfig() below, unblocking the very write that
        // clobbers it. Array.isArray matters here because `typeof []` is
        // "object" and would sail through the plain type check.
        if (raw === null || typeof raw !== "object" || Array.isArray(raw)) {
            root.handleMalformedConfig({
                message: `top-level value is ${Array.isArray(raw) ? "an array" : typeof raw}, expected an object`
            });
            return false;
        }
        root.recoverFromMalformedConfig();

        if (root.configRepaired || root.defaultOptions === null)
            return false;

        const migrated = root.migrateRaw(raw);
        let repaired = [];
        root.repairTypeConflicts(raw, root.options, root.defaultOptions, "", repaired);
        root.repairEnumViolations(raw, repaired);

        let unknown = [];
        root.collectUnknownKeys(raw, root.defaultOptions, "", unknown);
        if (unknown.length > 0)
            console.warn(`[Config] Ignoring ${unknown.length} unrecognized key(s), which the next save will drop: ${unknown.join(", ")}`);

        if (!migrated && repaired.length === 0) {
            // Unknown keys alone never justify a rewrite — they're dropped by
            // the next write regardless. Back the file up so the discarded
            // lines survive somewhere, then just report them.
            if (unknown.length > 0) {
                root.configRepaired = true;
                Quickshell.execDetached(["cp", "--", root.filePath, `${root.filePath}.bak`]);
                root.notifyConfigHealth("unknownKeys", unknown);
            }
            return false;
        }

        root.configRepaired = true;
        // Keep a copy before healing — the reset values are the only record of
        // what the user had, and a wrong guess here should be recoverable.
        // Unknown keys ride along: this same write is what erases them.
        if (repaired.length > 0 || unknown.length > 0)
            Quickshell.execDetached(["cp", "--", root.filePath, `${root.filePath}.bak`]);

        // Deferred: a setText issued from inside onLoaded is treated as part of
        // the load still in flight and its save gets dropped.
        const payload = JSON.stringify(raw, null, 2);
        Qt.callLater(() => {
            configFileView.setText(payload);
            root._adapterMatchesDisk();
            // setText re-deserializes, so the adapter now holds the repaired
            // values and rounding can be migrated off them.
            root.migrateRoundingConfig();
        });
        root.notifyConfigHealth(migrated ? "migrated" : "repaired", repaired);
        return true;
    }

    // Called once per malformed load, from the JSON.parse catch above.
    function handleMalformedConfig(parseError) {
        if (root.configMalformed)
            return;
        root.configMalformed = true;
        console.warn(`[Config] config.json is not valid JSON (${parseError.message}); preserving it as-is and blocking writes until it's fixed`);
        // Preserve the broken file exactly as the user left it — it's the
        // only copy, and hand-editing it back to valid JSON is the intended
        // recovery path (the existing watchChanges watcher picks the fix up
        // automatically via fileReloadTimer -> onLoaded -> repairConfigFile()).
        Quickshell.execDetached(["cp", "--", root.filePath, `${root.filePath}.malformed-${Date.now()}`]);
        root.blockWrites = true;
        root.notifyConfigHealth("malformed", []);
    }

    // Called on every successful parse; no-ops unless a prior load left
    // configMalformed set, i.e. the user (or watcher-triggered reload) just
    // fixed the file.
    function recoverFromMalformedConfig() {
        if (!root.configMalformed)
            return;
        root.configMalformed = false;
        root.blockWrites = false;
        console.log("[Config] config.json is valid JSON again; resuming normal writes");
        root.notifyConfigHealth("recovered", []);
    }

    // Escape hatch for a user who can't or doesn't want to hand-fix a
    // malformed config.json — wired to ConfigHealthBanner's reset action.
    function resetConfigToDefaults() {
        if (root.defaultOptions === null)
            return;
        console.warn("[Config] Resetting config.json to defaults at user request");
        root.configMalformed = false;
        root.blockWrites = false;
        // Object spread ({...obj}) isn't supported by this JS engine
        // (only array spread is) — use Object.assign instead.
        // The protected machine-local settings survive the reset: a bare
        // defaults payload would otherwise be the one write that silently
        // defeats LocalPreferences.
        const payload = JSON.stringify(LocalPreferences.overlayOntoDefaults(Object.assign({}, root.defaultOptions, {
            configVersion: root.currentConfigVersion
        })), null, 2);
        Qt.callLater(() => {
            configFileView.setText(payload);
            root._adapterMatchesDisk();
        });
        root.notifyConfigHealth("reset", []);
    }

    // Single point where config health changes reach the user: updates the
    // state ConfigHealthBanner binds to and fires a desktop notification.
    function notifyConfigHealth(state, keys) {
        root.configHealthState = state;
        root.configHealthKeys = keys;
        const copy = {
            "malformed": ["Config file is broken", "config.json has invalid JSON syntax. Your settings are safe on disk, but changes won't save until it's fixed."],
            "recovered": ["Config file fixed", "config.json is valid again — settings will save normally."],
            "migrated": ["Config updated", "Some settings were migrated to a newer format."],
            "repaired": ["Config values reset", `${keys.length} setting${keys.length === 1 ? "" : "s"} had an invalid value and ${keys.length === 1 ? "was" : "were"} reset to default.`],
            "unknownKeys": ["Unrecognized settings", `${keys.length} entr${keys.length === 1 ? "y" : "ies"} in config.json ${keys.length === 1 ? "isn't" : "aren't"} recognized and will be removed on the next save.`],
            "reset": ["Config reset to defaults", "config.json was replaced with default settings."]
        }[state];
        if (!copy)
            return;
        Quickshell.execDetached(["notify-send", copy[0], copy[1], "-a", "Shell", "-i", "dialog-warning", `--urgency=${state === "malformed" ? "critical" : "normal"}`]);
    }

    // Runs before the async file load completes, so this captures the pure QML
    // defaults — the only chance to, since deserializing overwrites them.
    Component.onCompleted: {
        root.defaultOptions = root.snapshotDefaults(root.options);
    }

    Component.onDestruction: {
        root.blockWrites = true;
    }

    FileView {
        id: configFileView
        path: root.filePath
        watchChanges: true
        blockWrites: root.blockWrites
        // Atomic writes: write to a temp file then rename. Prevents mid-write
        // corruption if the process is killed mid-save (e.g. PC power loss,
        // SIGKILL during shell update).
        atomicWrites: true
        onFileChanged: fileReloadTimer.restart()
        onAdapterUpdated: {
            if (root.ready && !root.blockWrites && !root._assigningIncoming)
                fileWriteTimer.restart();
        }
        onLoaded: {
            root.ready = true;
            root._adapterMatchesDisk();
            // When a repair is queued, rounding is migrated from inside it —
            // the values in the adapter right now are the coerced ones.
            if (root.repairConfigFile())
                return;
            migrateRoundingConfig();
            syncAppLaunchAnimation();
            if (Persistent.ready) {
                Persistent.tryMigrateAndSyncUserData();
            }
            // config.json just (re)appeared: take back any protected setting
            // it lost, then refresh the durable mirror.
            LocalPreferences.reconcile();
        }
        onLoadFailed: error => {
            if (error != FileViewError.FileNotFound) {
                return;
            }
            const elapsed = Date.now() - root.initTimestamp;
            if (elapsed > root.missingFileGracePeriod && !root.ready) {
                // Singleton has been alive past the grace window and the file
                // is still gone — legitimately missing (first-run install or
                // user manually deleted it). Safe to seed defaults.
                // Stamp the schema version so a brand-new file is not mistaken
                // for a pre-versioning one and re-migrated on next start.
                root.options.configVersion = root.currentConfigVersion;
                writeAdapter();
                // Mark ready so subsequent user-triggered writes go through
                // (fileWriteTimer guards on `root.ready`).
                root.ready = true;
                // config.json was genuinely missing and defaults were just
                // seeded — reconcile (restore before pull) so the protected
                // machine-local settings come back from local-preferences.json.
                LocalPreferences.reconcile();
            } else {
                // Likely transient: schedule a reload. If it succeeds,
                // `onLoaded` flips `root.ready` and nothing is overwritten. If
                // it still fails past the grace window, defaults are written.
                missingFileRetryTimer.restart();
            }
        }

        Component.onDestruction: {
            configFileView.blockWrites = true;
        }

        JsonAdapter {
            id: configOptionsJsonAdapter

            // 0 means "written before schema versioning existed" — see
            // migrateRaw(). Never default this to currentConfigVersion.
            property int configVersion: 0

            property string panelFamily: "ii" // "ii", "tablet", "waffle"

            // Preferences for surfaces that exist only in the tablet family. Keeping these
            // apart from `dock` lets the ii dock retain its desktop defaults while the two
            // families still share the user's pinned apps and adaptive-icon treatment.
            property JsonObject tablet: JsonObject {
                /**
                 * The workspace the Home button lands on. 0 means the lowest ordinary
                 * workspace of whichever monitor Home was pressed on.
                 *
                 * Home has to be the same place every time: the home screen's icons are
                 * stored per workspace, so "any empty workspace" shows a blank screen and
                 * leaves the arrangement on the workspace it was made on.
                 */
                property int homeWorkspace: 0

                property JsonObject dock: JsonObject {
                    // The taskbar is a real layer-shell reservation by default. It can be
                    // released only when the user deliberately prefers overlay behaviour.
                    property bool reserveSpace: true
                    property int height: 96
                    property int iconSize: 48
                    property bool showAppRow: true
                    /**
                     * Whether the launcher row gets out of the way once the workspace
                     * has a window on it.
                     *
                     * Off by default, because the reference product's taskbar is
                     * persistent: on a Pixel Tablet the row of apps is there whether or
                     * not something is open, and it is how you switch apps without going
                     * through Recents. Hiding it made the dock look broken instead of
                     * calm — the icons and the drawer button simply stopped existing the
                     * moment anything was running, with nothing on screen saying why.
                     * It stays available for people who want the home screen bare.
                     */
                    property bool autoHideOnOccupiedWorkspace: false
                    property bool keepNavigationVisible: true
                    property bool showNavigation: true
                    property list<string> navigationOrder: ["back", "home", "recents"]
                    property bool showRunningApps: true
                    /// How many running apps the dock shows beside the pinned ones. 0 fits
                    /// as many as the free space between the search pill and the navigation
                    /// pill allows, which is what a dock on a wide screen should do.
                    property int maximumRecents: 0
                    property bool showAppDrawerButton: true
                    // A search pill on the left of the dock, styled like the desktop's
                    // Android search widget. It opens the app drawer with the field already
                    // focused rather than searching anything itself.
                    property bool showSearchBar: true
                    property int searchBarWidth: 320
                    // "extended" is the Android pill; "compact" collapses it to a single
                    // circular button the size of a dock icon, for people who want the row
                    // to be apps and nothing else.
                    property string searchBarStyle: "extended"
                    // Either end of the pill is a button the user picks. "none" hides it,
                    // "search" and "apps" open the drawer on its search and its grid, and
                    // "tool:<id>" opens the drawer straight into one of the search panels
                    // (see SearchPanelRegistry) — "tool:clipboard" being the obvious one.
                    property string searchLeadingAction: "search"
                    property string searchTrailingAction: "apps"
                    /// Empty means the translated default, "Search".
                    property string searchPlaceholder: ""
                    property bool showAppDividers: true
                    // Circular arrows at either end of the dock, for moving between home
                    // screens without a swipe. On a laptop with a touchscreen the swipe is
                    // the only way there, and it needs bare wallpaper to start on.
                    property bool showWorkspaceArrows: true
                    property bool showPageCounter: true
                    property bool hidePageCounterOnOccupiedWorkspace: true
                    property bool compactWhenPageCounterHidden: true

                    /**
                     * Whether the dock has a surface of its own.
                     *
                     * "none"        — Android's home screen: icons straight on the wallpaper,
                     *                 outlined so they read against whatever is behind them.
                     * "translucent" — a shelf, dimmed by `backgroundOpacity`.
                     * "solid"       — an opaque shelf, like a Windows taskbar. Opaque even
                     *                 when the theme's own layer 0 is translucent, since a
                     *                 solid bar is the whole point of choosing this.
                     *
                     * The glyph outlines go with the surface: on a colour we chose they are
                     * a halo, so the two are decided together rather than mixed.
                     */
                    property string backgroundStyle: "none"
                    /// An inset rounded slab rather than a bar reaching the screen edges.
                    property bool backgroundFloating: false
                    /// Percent. Only read by "translucent".
                    property int backgroundOpacity: 75

                    /**
                     * What tapping an app that is already running does.
                     *
                     * "focus"  — raise the window, and switch to its workspace if it is on
                     *            another one. This is what a taskbar does everywhere, and
                     *            what the running dot under the icon promises.
                     * "launch" — always start another copy, which is what this dock used to
                     *            do unconditionally.
                     *
                     * With "focus", a second app window is still one gesture away: a double
                     * tap launches, and so does the long-press menu's "Open".
                     */
                    property string appTapAction: "focus"
                    /// Milliseconds within which a second tap on a dock icon means "another
                    /// window", not "focus again". 0 turns the double tap off entirely.
                    property int doubleTapLaunchMs: 320
                }

                /**
                 * The Recents surface — what was I just doing.
                 *
                 * "list" is a continuous row of cards you scrub sideways, which is what
                 * Android does and what this surface is. "grid" pages through four windows
                 * at a time: a genuinely better answer on a very large screen, but the wrong
                 * default, because a page is a unit the user has to reason about and Recents
                 * has no units — it has an order.
                 */
                property JsonObject recents: JsonObject {
                    property string layout: "list"
                    property int gridColumns: 2
                    property int gridRows: 2
                    /// Screenshot and Split under the card in the middle of the view, as
                    /// Android shows them. Off reclaims the strip for taller cards; the same
                    /// actions stay in the card's own menu either way.
                    property bool showCardActions: true
                    /// How opaque the backdrop behind the cards is, in percent, over an
                    /// opaque base — so the number means what it says. The windows
                    /// underneath stay faintly part of the transition rather than being
                    /// replaced by a flat colour, which is why this is not simply 100.
                    property int backdropOpacity: 82
                }

                /**
                 * What the Back button and the back gesture do once no shell surface is left
                 * to close.
                 *
                 * Android's Back is a key the focused application interprets. There is no
                 * such key on a Linux desktop, so the closest equivalent is sent instead —
                 * Alt+Left is browser and file-manager back, and it is what most toolkits
                 * map their own back action to.
                 */
                property JsonObject navigation: JsonObject {
                    /// Send the key at all. Off keeps Back inert on a bare home screen.
                    property bool sendBackKeyToApps: true
                    /// "alt_left" | "escape" | "backspace" | "browser_back" | "custom".
                    property string backKey: "alt_left"
                    /// Only read by "custom": Hyprland mods and key, e.g. "CTRL" / "bracketleft".
                    property string customBackMods: ""
                    property string customBackKey: ""
                }

                /**
                 * Windows as a tablet expects them: floating by default, with touch handles
                 * to move and resize them, because Hyprland's own move and resize are
                 * pointer-drag bindings a finger cannot reach.
                 */
                property JsonObject windows: JsonObject {
                    /// "off" | "all" | "keepDialogs". "keepDialogs" leaves anything the
                    /// compositor already floated where it put itself — an application's own
                    /// dialog knows its size and position better than a placement rule does.
                    property string floatMode: "off"
                    /// Percent of the monitor's usable area a newly floated window takes.
                    property int floatWidthPercent: 62
                    property int floatHeightPercent: 68
                    /// Cascade each new window down and right of the last, so a second window
                    /// does not land exactly on the first.
                    property bool cascade: true
                    /// App IDs that keep the compositor's own behaviour. Matched
                    /// case-insensitively against the Hyprland class.
                    property list<string> exclusions: ["quickshell"]
                    /// A title bar over the focused floating window with drag, resize,
                    /// fullscreen and close targets sized for a finger.
                    property bool touchControls: true
                    property int touchControlsHeight: 40
                    /// Between two tiled windows, a handle that resizes them and opens quick
                    /// actions, like the divider in Android's split screen. Hyprland keeps the
                    /// gutter at least splitHandleWidth + 2 × splitHandleSpacing wide, so the
                    /// pill never touches a window.
                    property bool splitHandles: true
                    /// The pill's width, px.
                    property int splitHandleWidth: 12
                    /// Air between the pill and the windows on either side of it, px.
                    property int splitHandleSpacing: 4
                }

                /**
                 * A small always-present circle, draggable anywhere, that opens a sheet of
                 * large quick actions.
                 *
                 * The gap it fills: a tablet held in two hands can reach one thing reliably,
                 * and everything this shell can do is otherwise behind an edge gesture, a
                 * keybind or a surface that first has to be summoned. Several of those —
                 * float this window, make it fullscreen, bring up the keyboard — are one tap
                 * on Android and a keyboard shortcut here. The bubble is the one control
                 * that is always where the user left it, including over a fullscreen app,
                 * which is exactly when the edge gestures are least reachable.
                 */
                property JsonObject bubble: JsonObject {
                    property bool enable: true
                    /// Stay on top of fullscreen windows. Off puts it below them, which is
                    /// what a normal overlay does and what a video player would prefer.
                    property bool showOverFullscreen: true
                    /// Diameter, px.
                    property int size: 56
                    /// Percent, when the sheet is closed and nothing has touched it lately.
                    property int idleOpacity: 65
                    /// Seconds of no interaction before it fades to idleOpacity. 0 never fades.
                    property int idleAfterSeconds: 4
                    /// Snap to the nearest side edge when released, as a chat head does.
                    property bool snapToEdge: true
                    /**
                     * Which actions the sheet offers, in order.
                     *
                     * Ids come from `ShellActionRegistry` — the same catalogue the gesture
                     * bindings and the search use — so the bubble adds a way to reach the
                     * shell's actions rather than a second list of them to keep in step.
                     * "none" leaves the slot empty.
                     */
                    property list<string> actions: [
                        // Live draw leads: it is marked prominent in the registry, so it
                        // is the wide bar at the top of the sheet rather than one square
                        // among eight. A pen comes out mid-thought and the control for it
                        // has to be the one that cannot be missed.
                        "liveDraw", "osk", "toggleFloating", "toggleFullscreen", "regionScreenshot", "sidebarRight", "recents", "appDrawer"]
                }

                /**
                 * The shell behaving as though a pen, not a mouse, is the pointer.
                 *
                 * Off by default: it changes the system cursor for the whole session,
                 * which is not something to do to somebody who never asked.
                 *
                 * No OpenTabletDriver configuration is involved, and that is deliberate.
                 * OTD passes the barrel buttons through as ordinary `BTN_STYLUS` and
                 * `BTN_STYLUS2` on the tablet device, which the keyboard's helper daemon
                 * already watches — so the buttons are bound here, in one place, rather
                 * than by writing key combinations into OTD's settings and binding those
                 * combinations again in Hyprland. Three files that have to agree is three
                 * files that can disagree, and opening OTD's own UI would break it.
                 */
                property JsonObject pen: JsonObject {
                    property bool enable: false
                    /// Swap the pointer for the current cursor theme's own pencil.
                    property bool cursor: true
                    /// Blank means "whatever XCURSOR_THEME says".
                    property string cursorTheme: ""
                    /// Smaller than a normal pointer, because a nib is a point.
                    property int cursorSize: 20
                    /**
                     * What each barrel button does, in order, as action ids from
                     * ShellActionRegistry — plus "dragWindow", which is handled by pen
                     * mode itself because it means something for as long as the button is
                     * held rather than once when it is pressed.
                     */
                    property list<string> buttons: ["dragWindow", "back"]
                }

                /**
                 * Drawing on the screen with a pen, over whatever is on it.
                 *
                 * The palette is a list of colours rather than theme tokens on purpose,
                 * and it is the one place in this shell where a literal colour is the
                 * right answer: these are pigment, not chrome. Ink that recoloured itself
                 * when the wallpaper changed would be ink you could not rely on, and a
                 * drawing done in red is a drawing done in red.
                 */
                property JsonObject liveDraw: JsonObject {
                    property bool enable: true
                    /// Nominal stroke width in px, before pressure scales it.
                    property int width: 4
                    /// Whether a stylus's pressure varies the stroke width. Ignored by
                    /// devices that do not report it — a finger draws an even line.
                    property bool pressure: true
                    /// How much the input filter smooths, 0–95. Higher is steadier and
                    /// lags further behind the tip; 0 draws the raw samples, tremble and
                    /// all.
                    property int smoothing: 55
                    property list<string> palette: ["#ffffff", "#111111", "#e53935", "#fb8c00", "#fdd835", "#43a047", "#1e88e5", "#8e24aa"]
                    /**
                     * Whether the ink slides with the workspace it belongs to.
                     *
                     * The sheets are per workspace, so switching already swaps which one
                     * is painted — this is about *how*. With it on, the outgoing sheet
                     * leaves and the incoming one arrives alongside the windows, but
                     * travelling slightly less far, so the ink reads as sitting a little
                     * behind the glass rather than being stuck to it.
                     *
                     * Costs one extra canvas for the length of the transition, and only
                     * when at least one of the two sheets has something on it.
                     */
                    property bool workspaceParallax: true
                }

                /**
                 * What the tablet becomes when it is charging and nobody is using it: an
                 * ambient clock, weather and now-playing display, readable across a room.
                 *
                 * The Pixel Tablet does this when docked. "Docked" is not something this
                 * shell can know, so charging is the proxy — it is the state where the
                 * device is parked rather than in your hands.
                 */
                property JsonObject hubMode: JsonObject {
                    property bool enable: false
                    property bool requireCharging: true
                    /// Seconds of no input before it takes over.
                    property int idleSeconds: 120
                    /// Never interrupt something being watched.
                    property bool pauseWhilePlaying: true
                }

                property JsonObject gestures: JsonObject {
                    /**
                     * What swiping in from the left and right edges does.
                     *
                     * "back"     — both edges go back, as on Android, where it is the most
                     *              used gesture after Home.
                     * "policies" — the left edge opens the first policies app instead, which
                     *              is what it did before; the right edge still goes back.
                     * "none"     — neither edge is claimed, so both fall through to whatever
                     *              is bound under Edge and corner bindings.
                     */
                    property string sideEdges: "back"

                    /**
                     * What swiping up from the bottom edge does.
                     *
                     * "android" — up is Home; up from the home screen opens the app drawer,
                     *             following the finger; up and hold opens Recents.
                     * "drawer"  — the edge only ever opens the drawer, as it did before.
                     *             Keeps the old muscle memory, at the cost of Home and
                     *             Recents having no gesture at all.
                     */
                    property string bottomEdge: "android"
                }

                property JsonObject appDrawer: JsonObject {
                    // "name" | "nameDesc" | "category" | "usage". Only the unsearched grid
                    // is sorted: with a query, relevance is the order, and re-sorting it
                    // would discard the ranking the user is typing towards.
                    property string sortMode: "name"
                    /// How tall the bottom-edge strip that starts a pull-up drag is. Same
                    /// idea as the shade's pull-down edge, and the same default.
                    property int edgeDragHeight: 8
                    property bool showSortButton: true
                    property bool showCategoryFilter: true
                    /// Shows a GNOME-style workspace overview strip with live screencopies
                    /// between the search bar and the app grid.
                    property bool showWorkspacesOverview: false
                    /// Long-press opens an Android-style menu on the tile. Off restores the
                    /// old behaviour, where a long-press dropped the app on the home screen.
                    property bool longPressMenu: true
                    /// Hold an app and drag it out: the drawer steps aside and the app opens
                    /// where it is dropped, with a preview of the layout it will make.
                    property bool dragToLaunch: true
                    /// How long the finger rests at a side edge during that drag before the
                    /// neighbouring workspace comes in, in milliseconds.
                    property int edgeSwitchDelay: 600
                    /// A predicted row above the grid, from the same launch history the
                    /// "Most used" sort reads. Hidden automatically when that sort is on.
                    property bool showSuggestions: true
                    /// The shell's own panels — clipboard, emoji, translator, downloader —
                    /// offered as a row instead of only when their name is typed.
                    property bool showToolShelf: true
                    property bool showClipboardResults: true
                    property bool showFileResults: true
                    /// File matches come from the whole filesystem, not only the Search
                    /// directory: on the tablet the drawer is the only launcher there is.
                    property bool searchWholeSystem: true
                    /// Quick toggles (Wi-Fi, Bluetooth, dark mode…) matching the query, as
                    /// switches beside the results.
                    property bool showQuickToggleResults: true
                    property int sideResultLimit: 6
                    /// 0 derives the tile from the screen, which is what a tablet wants.
                    property int tileWidth: 0
                    property int iconSize: 0
                }
            }

            property JsonObject policies: JsonObject {
                property int ai: 1 // 0: No | 1: Yes | 2: Local
                property int weeb: 0 // 0: No | 1: Open | 2: Closet
                property int wallpapers: 0 // 0: No | 1: Yes
                property int translator: 1 // 0: No | 1: Default (illogical-impulse) | 2: Expressive (reworked)
                property int player: 0 // 0: No | 1: Yes
                property int phone: 1 // 0: No | 1: Yes — Phone tab (future KDE Connect + scrcpy external)
            }

            property JsonObject phone: JsonObject {
                property bool kdeconnectEnabled: true
                // Shell actions offered to the phone through KDE Connect's Run Command plugin
                // (mirror, recording, media, lock...). Written into kdeconnect's own config.
                property bool remoteCommands: true
                // Keep phone notifications in the desktop list/popups too, not only the Phone tab
                property bool mirrorNotificationsToDesktop: true
                property bool showPeripheralCards: true
                property JsonObject contacts: JsonObject {
                    property bool enabled: true
                    property list<string> favoriteIds: []
                    property bool showAvatars: true
                    property string sortBy: "first" // "first" | "last"
                    // Android exports every raw contact from every sync adapter, so
                    // spam lists and SIM imports show up as bare phone numbers
                    property bool hideUnnamed: true
                }
                property JsonObject scrcpy: JsonObject {
                    property bool stayAwake: false  // bare scrcpy default — don't add overhead
                    property bool turnScreenOff: false  // turning screen off causes input delay on Samsung (touch sampling rate drops)
                    property bool noPowerOn: false  // bare scrcpy default
                    property bool noAudio: false
                    property bool showTouches: false
                    property bool fullscreen: false
                    property bool alwaysOnTop: false
                    property int maxFps: 0  // 0 = use device's native frame rate (matches bare `scrcpy`)
                    property string bitRate: "8M"
                    property int maxSize: 0
                    property int videoBuffer: 0  // scrcpy 4.0 default is 0ms — 80ms adds visible latency
                    property bool useWireless: false
                    property bool autoWirelessIp: true  // resolve IP live from KDE Connect instead of the manual field
                    // Android 11+ wireless debugging re-rolls its port whenever adbd restarts (a screen
                    // unlock is enough), which kills any running scrcpy. `adb tcpip 5555` pins a classic
                    // TCP port that survives those restarts, until the phone reboots. Off by default:
                    // it reconfigures the phone's debugging daemon, which is the user's call to make.
                    property bool pinAdbPort: false
                    // adbd comes back a few seconds after each restart, but whatever was
                    // mirroring died with it — reopen the same window once the phone answers.
                    property bool autoResume: true
                    property string wirelessIp: ""
                    property string wirelessPort: "5555"
                    property bool showTerminal: false
                    // "uhid" makes the phone see a real hardware keyboard: accents, dead keys and the
                    // PC's layout work, and shortcuts reach apps. "sdk" is scrcpy's old injected-text
                    // mode, for the phones where UHID is refused.
                    property string keyboardMode: "uhid"
                    // Copy on either side, paste on the other, while a mirror is open.
                    property bool clipboardSync: true
                    // Phone screen recordings (scrcpy --record). Empty folder means ~/Videos.
                    property JsonObject recording: JsonObject {
                        property string folder: ""
                        property bool withAudio: true
                    }
                    // The mirror the Phone sidebar draws inside itself.
                    property JsonObject embed: JsonObject {
                        // Off sends the Mirror card back to opening a separate
                        // scrcpy window, which is what it did before the
                        // sidebar could draw the phone itself.
                        property bool enabled: true
                        // The sidebar unloads its content soon after closing, which
                        // took the mirror down with it and made every trip back pay
                        // for a fresh connection. The session outlives the page by
                        // this much; the phone is still encoding during it, so 0
                        // turns the behaviour off entirely.
                        property int keepWarmSeconds: 120
                    }
                    property JsonObject appMode: JsonObject {
                        property bool enabled: true
                        property bool showAppIcons: true // Pull each app's launcher icon off the phone over adb
                        property string iconShape: "oneui" // Launcher-style mask for those icons, keys in AndroidIconMask.shapes
                        property bool flexDisplay: true
                        // What to do with the phone when a session ends: "home" leaves it on its
                        // home screen and unlocked, "continue" moves the app back to the phone's
                        // own screen instead of destroying it, "lock" puts the phone to sleep.
                        property string onSessionEnd: "home"
                        // Wake the phone and dismiss its lockscreen before launching. Only gets
                        // through when the phone already trusts this situation (extended unlock);
                        // a phone that wants its PIN is left alone and gets the unlock mirror.
                        property bool autoUnlock: true
                        property int displayWidth: 1280
                        property int displayHeight: 960
                        property int density: 160
                        property bool keepActive: true
                        property bool systemDecorations: true
                        property list<string> favoritePackages: []
                    }
                }
                property JsonObject webcam: JsonObject {
                    property bool enabled: false
                    property string cameraFacing: "front" // "front" | "back"
                    property string resolution: "1280x720" // "640x480" | "1280x720" | "1920x1080"
                    property int fps: 30
                    property string bitrate: "4M"
                    property bool mirrorHorizontally: false
                    property int rotateDegrees: 0 // 0 | 90 | 180 | 270
                    property string connection: "wifi" // "wifi" | "usb"
                    property string wifiIp: ""
                    property int port: 4747
                }
                property JsonObject microphone: JsonObject {
                    property bool enabled: false
                    property string connection: "wifi"
                    property string wifiIp: ""
                    property int port: 4748
                    property bool noiseSuppression: false
                    property bool echoCancellation: false
                    property bool autoGainControl: false
                    property int micGain: 100
                    property bool setAsDefault: false
                }
            }

            property JsonObject localsend: JsonObject {
                property bool autoStart: true
                property string downloadPath: Directories.localSendDownloadPath.replace("file://", "")
                property bool showNotifications: true
                property bool preferPopupOverNotification: true
            }

            property JsonObject googleDrive: JsonObject {
                property bool enabled: false
                property string syncInterval: "3d" // "1h", "4h", "1d", "2d", "3d"
                property bool syncOnBoot: true
                property bool syncOnNetworkChange: false
                property int bandwidthLimitKbps: 0
                property bool pauseOnMeteredConnection: true
                property list<string> backupFolders: []
                property list<string> excludePatterns: ["*.tmp", "*.swp", "*.lock", "node_modules/", ".git/", "__pycache__/"]
                // Empty means the per-machine default: <username>_<distro>_backups.
                // GoogleDriveService also treats the old "ii-backup" default as
                // automatic so existing installs migrate without rewriting config.
                property string driveBasePath: ""
                property bool notifyOnComplete: true
                property bool notifyOnError: true
                property int keepVersions: 3
                property bool deleteRemoteOrphans: false
                property bool onlyModifiedSinceLastSync: false
                property string lastSyncTime: ""
                property string lastSyncStatus: ""
                property int lastSyncFileCount: 0
                property real lastSyncSizeMb: 0.0
                // Durable sync events used by the Drive activity charts. Keep
                // this explicitly typed: nested JsonObject arrays must not use
                // `var` in Quickshell's JSON adapter.
                property list<var> syncHistory: []
                property real totalDriveUsageMb: 0.0
                property real driveQuotaMb: 0.0
                property real driveBackupUsageMb: 0.0
            }

            property JsonObject todo: JsonObject {
                // Choices: "local", "ticktick", or "googleTasks".
                property string provider: "local"
                property int refreshIntervalMinutes: 5
                property JsonObject googleTasks: JsonObject {
                    property string taskListId: ""
                    property string taskListTitle: ""
                }
            }

            property JsonObject vpn: JsonObject {
                property bool enabled: false
                property bool autoConnect: false
                property string backend: "networkmanager"
                property string defaultProfile: ""
                property string defaultProvider: "networkmanager"
                property string recentProvider: "networkmanager"
                property string defaultLocation: ""
                property bool protonSecureCore: false // Adds --securecore; defaultLocation becomes the exit country
                property bool disconnectOnDisable: false
                property bool killSwitch: false
                property bool blockLan: false
                property bool enableDiagnostics: true
            }

            property JsonObject tailscale: JsonObject {
                property bool enabled: false
                property bool autoConnect: false
                property bool acceptDns: true
                property bool shieldsUp: false
                property bool ssh: false
                property string exitNode: ""
                property list<string> advertiseRoutes: []
                property bool advertiseExitNode: false
                property bool showPeers: true
                property bool stopDaemonWhenDisabled: false
                property bool enableDiagnostics: true
            }

            property JsonObject dnsOverTls: JsonObject {
                property bool enabled: false // Desired state, re-applied on startup and network changes
                property string preset: "adguard"
                property string serverName: "dns.adguard-dns.com" // TLS certificate name (SNI)
                property string serverAddress: "94.140.14.14"
                property string fallbackAddress: "94.140.15.15"
                property bool strict: true // "yes" when true, "opportunistic" when false
                property bool routeAllQueries: true // Route every domain through this server
                property bool reapplyOnNetworkChange: true
            }

            property JsonObject screenShader: JsonObject {
                // Name of the shader a left click on the quick toggle turns on.
                // Empty means "whatever the picker offers first".
                property string lastUsed: ""
                // Extra directories to scan for .glsl/.frag files, on top of the shell's
                // own assets/shaders and everything hyprshade already looks at.
                property list<string> extraShaderDirs: []
            }

            property JsonObject ai: JsonObject {
                // Controls only proactive model/prompt indexing at startup;
                // policy availability is Config.options.policies.ai.
                property bool indexAtStartup: true
                // Name chats automatically from their first completed turn.
                // Turn this off when session titles must remain manual.
                property bool autoTitle: true
                // Interface/status rows are useful while a chat is open, but
                // can be omitted from the saved transcript when wanted.
                property bool ephemeralInterfaceMessages: false
                property string systemPrompt: "## Style\n- Use casual tone, don't be formal!\n- Always be brief and to the point, unless asked otherwise\n- Don't repeat the user's question\n- Be approachable: Avoid using overly complicated, domain-specific terms and provide analogies when asked to explain a concept\n\n## Context (ignore when irrelevant)\n- You are a helpful and inspiring sidebar assistant on a {DISTRO} Linux system\n- Desktop environment: {DE}\n- Current date & time: {DATETIME}\n- Focused app: {WINDOWCLASS}\n\n## Presentation\n- Use Markdown features in your response: \n  - **Bold** text to **highlight keywords** in your response\n  - **Split long information into small sections** with h2 headers and a relevant emoji at the start of it (for example `## \ud83d\udc27 Linux`). Bullet points are preferred over long paragraphs, unless you're offering writing support or instructed otherwise by the user.\n- Asked to compare different options? You should firstly use a table to compare the main aspects, then elaborate or include relevant comments from online forums *after* the table. Make sure to provide a final recommendation for the user's use case!\n- Use LaTeX formatting for mathematical and scientific notations whenever appropriate. Enclose all LaTeX '$$' delimiters. NEVER generate LaTeX code in a latex block unless the user explicitly asks for it. DO NOT use LaTeX for regular documents (resumes, letters, essays, CVs, etc.).\n\nThanks!\n"
                property JsonObject tools: JsonObject {
                    // What the assistant may reach for: "functions" (settings,
                    // commands, a hop to search), "search" (the provider's own
                    // web search) or "none".
                    property string mode: "functions"
                    // Locally served models advertise no capabilities, so tools
                    // stay off for them until the user says their models can
                    // handle function calling.
                    property bool localModels: false
                    // Permission per tool, by tool id. A tool named in neither
                    // list asks before it runs, which is the default for
                    // anything that writes.
                    // Reading is allowed outright; anything that writes or runs
                    // still asks. Searching and fetching a page only read.
                    property list<string> alwaysAllow: ["settings_search", "settings_get", "switch_to_search_mode", "web_search", "fetch_url"]
                    property list<string> alwaysDeny: []
                    // Show every proposed settings change next to its current
                    // value before writing any of them.
                    property bool reviewConfigChanges: true
                    // When enabled, choices made in the tools panel stay with
                    // the open conversation instead of becoming a global
                    // standing permission for every future chat.
                    property bool scopePerConversation: false
                    // How many tool calls the log remembers. 0 keeps none.
                    property int logSize: 50
                    // A local-only policy is about reducing what the assistant
                    // can reach, not only about the network, so shell commands
                    // stay off under it. Set true to disagree.
                    property bool allowShellInLocalPolicy: false
                }
                // Longest answer to ask for, in tokens. 0 uses whatever the
                // model itself supports, which is what most people want;
                // a positive value caps it and is clamped to the model's own
                // limit.
                property int maxOutputTokens: 0
                // The compact chat toolbar normally shows accumulated usage.
                // When enabled it shows the latest completed answer's
                // generated tokens per second instead.
                property bool showTokensPerSecond: false
                // When the session contains OpenRouter responses with a
                // reported charge, show their exact accumulated cost instead
                // of either token metric in the compact chat toolbar.
                property bool showOpenRouterSessionCost: false
                // Seconds before giving up on reaching the endpoint, and
                // before abandoning a reply that is still being written.
                property int connectTimeout: 15
                property int requestTimeout: 300
                // Extra attempts after a rate limit or a server error.
                property int maxRetries: 2
                // Biggest file that may be sent, in MiB, and how many may go
                // with one message. Both are about what a request can carry:
                // providers refuse bodies past roughly 20 MiB, and every file
                // is sent again with every following turn.
                property int maxAttachmentMib: 8
                property int maxAttachments: 6
                // What happens when a conversation outgrows the model's
                // context window. Sending it whole is how a long chat starts
                // failing outright, so the oldest turns are left behind and,
                // if asked, folded into a summary that goes in their place.
                property JsonObject context: JsonObject {
                    property bool manage: true
                    property bool summarise: true
                    // Room kept for the answer itself.
                    property int reserveTokens: 4096
                }
                // Documents a model cannot read natively are turned into text
                // on this machine instead of being dropped on the way out.
                property bool extractDocuments: true
                // Where the assistant may look for a file by itself, without
                // one having been chosen by hand first. Empty by default:
                // reaching the filesystem is something the user opts into,
                // never a folder guessed on their behalf.
                property JsonObject files: JsonObject {
                    property list<string> roots: []
                }
                // Read text out of an image with the OCR engine already on
                // the machine, when one is. The tool itself only ever runs on
                // a path the model already has — from a search result or a
                // file the user attached — never on a screenshot taken for
                // it, so leaving this on does not mean anything is captured
                // automatically.
                property JsonObject vision: JsonObject {
                    property bool ocrEnabled: true
                }
                property JsonObject voice: JsonObject {
                    // Turning speech into a draft the user still has to send.
                    // Off by default: nothing installs a transcription
                    // backend automatically, so a fresh install would
                    // otherwise show a microphone button that can only ever
                    // land on "error" until the user follows the Settings
                    // guide and turns this on themselves.
                    property bool enabled: false
                }
                // Local retrieval over folders the user pointed at
                // explicitly through Settings. Off by default: nothing is
                // chunked, embedded, or indexed until a collection exists.
                property JsonObject rag: JsonObject {
                    property bool enabled: false
                    // An Ollama model id, chosen from the ones detected as
                    // embedding-capable. Empty until the user picks one.
                    property string embeddingModel: ""
                    // {id, name, path}. The index files themselves live
                    // outside config.json, under Directories.state.
                    property list<var> collections: []
                }
                // A desktop notification when an answer lands. Only while the
                // chat is not on screen: telling someone what they are already
                // reading is noise.
                property JsonObject notify: JsonObject {
                    property bool whenDone: true
                    property bool onlyWhenAway: true
                    // An answer that came back before this many seconds is one
                    // the user almost certainly waited for.
                    property int minimumSeconds: 4
                }
                // Facts the assistant carries between conversations. Each one
                // is a line the user can read and delete; the model can only
                // propose one through a tool that asks first.
                property JsonObject memory: JsonObject {
                    property bool enabled: true
                    property int limit: 40
                }
                // Chats are first moved to .trash so undo and manual recovery
                // remain possible. The session store prunes that folder by
                // this retention window on startup and whenever it changes.
                property JsonObject sessions: JsonObject {
                    property int retentionDays: 30
                }
                // Projects: a name, a prompt of its own and files that go with
                // every chat filed under it. {id, name, icon, prompt, files[]}
                property list<var> projects: []
                // Personas the user wrote: {id, name, icon, description,
                // systemPrompt, modelId, thinking, temperature, starters[]}.
                // The ones that ship with the shell are not in here.
                property list<var> personas: []
                // Models the user added, in one flat list. An entry with a
                // `provider` naming a built-in provider is added to it;
                // anything else stands on its own under "Others" and brings
                // its own endpoint, dialect and key id.
                property list<var> customModels: []
            }

            property JsonObject appearance: JsonObject {
                property bool extraBackgroundTint: true
                property int fakeScreenRounding: 1 // 0: None | 1: Always | 2: When not fullscreen | 3: Wrapped
                property int wrappedFrameThickness: 6
                property bool sharpMode: false
                property string globalRounding: "large" // Legacy — migrated to roundingValue via onLoaded
                property int roundingValue: 28 // -1 = not yet migrated, 0-48 = active slider value (default 24 after migration)
                property int defaultBorderRadius: 18
                property bool toggleWindowRounding: true // Changes Hyprland window rounding to 0 if sharpMode is true
                property real iconTintPercentage: 0.6
                property JsonObject fonts: JsonObject {
                    property bool enableCustom: false
                    property string main: "Google Sans Flex" // xmpl: right sidebar, settings, system monitor, default clock, expressive weather
                    property string numbers: "Google Sans Flex" // xmpl: styled slider, config
                    property string title: "Google Sans Flex" // settings list item, popup titles
                    property string iconNerd: "JetBrainsMono Nerd Font"
                    property string monospace: "JetBrainsMono Nerd Font" // clipboard metadata
                    property string reading: "Readex Pro" // cookie clock quote
                    property string expressive: "Space Grotesk" // desktop widgets font, overview workspace number, user profile config
                    property bool roundnessFull: true
                }
                property JsonObject transparency: JsonObject {
                    property bool enable: false
                    property bool automatic: false
                    property bool popups: false
                    property real backgroundTransparency: 0.48
                    property real contentTransparency: 0.38
                }
                property int blurSize: 10
                // Keep blurSize at its existing path for presets. These defaults
                // match the II Hyprland look; the compositor still owns enabled.
                property JsonObject blur: JsonObject {
                    property bool advancedOptions: false
                    property int passes: 3
                    property real noise: 0.05
                    property real contrast: 0.89
                    property real brightness: 1.0
                    property real vibrancy: 0.2
                    property real vibrancyDarkness: 0.2
                    property bool ignoreOpacity: true
                    property bool newOptimizations: true
                    property bool xray: false
                    property bool special: false
                    property bool popups: false
                    property real popupsIgnoreAlpha: 0.6
                    property bool inputMethods: true
                    property real inputMethodsIgnoreAlpha: 0.8
                }
                property int borderWidth: 1
                property int gapsIn: 4
                property int gapsOut: 5
                property real ignoreAlpha: 0.5
                property JsonObject wallpaperTheming: JsonObject {
                    property bool enableAppsAndShell: true
                    property bool enableQtApps: true
                    property bool enableTerminal: true
                    property bool autoRestartQuickshell: false
                    property JsonObject terminalGenerationProps: JsonObject {
                        property real harmony: 0.6
                        property real harmonizeThreshold: 100
                        property real termFgBoost: 0.35
                        property bool forceDarkMode: false
                    }
                }
                property JsonObject icons: JsonObject {
                    property bool enableThemed: false
                    property bool enableShapeMask: false
                    property string shapeMask: "Circle"
                }
                property string borderColorType: "primary" // Options: primary, secondary, tertiary, primaryContainer, surface
                property bool borderless: true
                property string iconTheme: "Papirus"
                property JsonObject palette: JsonObject {
                    property string type: "scheme-intense" // Allowed: auto, scheme-content, scheme-expressive, scheme-fidelity, scheme-fruit-salad, scheme-intense, scheme-monochrome, scheme-neutral, scheme-rainbow, scheme-tonal-spot, scheme-vibrant
                    property string accentColor: ""
                    // Hand-picked key colors per mode, laid over whatever scheme is
                    // generated; empty = use the generated one (scripts/colors/color_overrides.py).
                    property JsonObject overrides: JsonObject {
                        property JsonObject dark: JsonObject {
                            property string primary: ""
                            property string secondary: ""
                            property string tertiary: ""
                            property string surface: ""
                        }
                        property JsonObject light: JsonObject {
                            property string primary: ""
                            property string secondary: ""
                            property string tertiary: ""
                            property string surface: ""
                        }
                    }
                }
                property list<string> customColorSchemes: []
                property real animationMultiplier: 0.8000000000000001 // 0.25 = fast, 1.0 = default, 2.0 = slow
                property bool colorfulScrollbar: false
                property bool scrollAnimations: false
                property bool scrollFadeMask: false
                property bool settingsPerformanceMode: true
                property int settingsUnloadDelay: 0 // Seconds Settings stays loaded after closing; 0 = unload at once, -1 = never
                property JsonObject appLaunchAnimation: JsonObject {
                    property bool enable: true
                    property string style: "scale" // Allowed: scale, slide
                    property string slideDirection: "auto" // Allowed: auto (nearest edge), bottom, top, left, right
                    property int startPercent: 20 // 5 - 90%, unused by slide
                    property real speed: 4.0 // Duration in tenths of a second
                    property string curve: "iiAppOpen"
                }
                property JsonObject openrgb: JsonObject {
                    property bool enable: false
                    property bool applyOnStartup: true
                    property real fadeDuration: 0.5
                    property real interpolationSteps: 100
                    property list<var> devices: []
                }
            }

            property JsonObject audio: JsonObject {
                // Values in %
                property JsonObject protection: JsonObject {
                    // Prevent sudden bangs
                    property bool enable: false
                    property real maxAllowedIncrease: 10
                    property real maxAllowed: 99
                }
            }

            property JsonObject apps: JsonObject {
                property string bluetooth: "kcmshell6 kcm_bluetooth"
                property string changePassword: "kitty -1 --hold=yes fish -i -c 'passwd'"
                property string network: "kcmshell6 kcm_networkmanagement"
                property string manageUser: "kcmshell6 kcm_users"
                property string networkEthernet: "kcmshell6 kcm_networkmanagement"
                property string taskManager: "plasma-systemmonitor --page-name Processes"
                property string terminal: "kitty -1" // This is only for shell actions
                property string update: "kitty -1 --hold=yes fish -i -c 'if test (ryoku-host pkgmgr 2>/dev/null) = dnf; ryoku update --system; else; pkexec pacman -Syu; end'"
                property string volumeMixer: `~/.config/hypr/hyprland/scripts/launch_first_available.sh "pavucontrol-qt" "pavucontrol"`
            }

            // Per-app usage and energy history. The sampler is a separate process
            // that reads its flags once at startup, so everything above the display
            // options relaunches it rather than taking effect in place.
            property JsonObject appStats: JsonObject {
                property bool enable: true
                property int sampleIntervalMs: 10000
                property int flushIntervalMs: 60000
                // Days of history kept. Under "previousMonth" this is a floor rather
                // than the window itself: nothing the previous calendar month needs
                // is dropped, so the window slides between 31 and 62 days and this
                // month can always be compared with the one before it. "fixed" keeps
                // exactly this many days.
                property int retentionDays: 31
                property string retentionMode: "previousMonth"
                // "auto" prefers RAPL and falls back to battery drain; "rapl",
                // "battery" and "none" pin the choice.
                property string energySource: "auto"
                // Sample intervals between full /proc sweeps for new GPU clients.
                // Only a backstop; a new window forces one immediately.
                property int gpuFullEvery: 30
                // Seconds without input before foreground time stops accruing.
                // 0 turns the idle monitor off, and time keeps running.
                property int idleTimeoutSec: 300
                // Daemons with no window are recorded either way; this only decides
                // whether they are worth showing next to the apps.
                property bool trackHeadless: true
                property bool showHeadless: false
                // Loads the App usage app (named from when it was an overlay).
                property bool overlayEnabled: true

                // Remember the page, period and metric between openings.
                property string defaultGranularity: "day"
                property string defaultMetric: "fg"
                property bool rememberLastView: true
                property string lastGranularity: "day"
                property string lastMetric: "fg"
                // Stable page key; unavailable pages fall back to App usage.
                property string lastView: "apps"
                // Which day a week runs from. Weeks are calendar weeks so that the
                // one before is always the same seven days, whoever asks.
                property bool weekStartsMonday: true
                // Keep the picked app across openings instead of clearing it.
                property bool keepSelection: false
                // Off by default: it needs the period before the one on screen
                // parsed as well, which doubles the files read for a month.
                property bool showComparison: false
                // Apps under this many seconds for the chosen metric are left out of
                // the list. Duration metrics only; energy is never thresholded.
                property int minDurationSec: 0
            }

            // Daily limits (services/ScreenTimeLimits.qml), the Limits tab of the
            // App usage app. Everything counts focused time only, from AppStats.
            property JsonObject screenTime: JsonObject {
                property bool enable: true
                // { id, kind: "app" | "total", name, keys: [window classes], minutes,
                //   days: [7 bools, Sunday first], enabled, strict }
                property list<var> limits: []
                // Focus schedules: apps blocked between two times of day.
                // { id, name, start: "HH:mm", end: "HH:mm", days, keys, allApps,
                //   enabled, strict }
                property list<var> schedules: []
                // Never blocked by the total limit or an all-apps schedule.
                property list<string> alwaysAllowed: []
                property int warnMinutes: 5
                property bool warnLastMinute: true
                property bool notifyOnLimit: true
                // Minutes between reminders once a limit was ignored; 0 turns them off.
                property int overdueReminderMinutes: 15
                // Seconds the block screen waits before closing the app on its own;
                // 0 waits for a choice.
                property int autoCloseSeconds: 0
                // Salted md5 of the PIN. A speed bump, not a vault.
                property string pinHash: ""
                // Ask for the PIN before a limit is edited, removed or paused.
                property bool pinForEdits: false
            }

            // Modes & Routines (services/Modes.qml). Definitions are user data
            // but live here on purpose so one file carries the whole setup.
            property JsonObject modes: JsonObject {
                property bool enable: true
                // Loads the Modes & Routines app (named from when it was an overlay).
                property bool overlayEnabled: true
                // Presets are added once; deleting one afterwards sticks.
                property bool presetsSeeded: false
                // "auto" shows the start/end banner in the island or, without
                // a notch, as a top-centre popup; "off" shows nothing.
                property string flash: "auto"
                property bool lockPill: true
                // Last visited page; unknown keys fall back to Modes.
                property string lastTab: "modes"
                property string lastModeId: ""
                property string lastRoutineId: ""
                // Seconds an auto-started mode's triggers must stay false
                // before it ends, so a workspace switch does not flap it.
                property int graceSec: 5
                // Mode definitions, in priority order (first wins among
                // automatic starts). Shape: see services/modes/ModeSchema.js.
                property list<var> modes: []
                property list<var> routines: []
                // Game detection (services/GameDetector.qml). Signals 1–3 are
                // instant; the GPU heuristic needs `gpuThreshold` % for `holdSec`.
                property JsonObject game: JsonObject {
                    property bool useLauncherClasses: true
                    property bool useDesktopCategory: true
                    property bool useGpuHeuristic: true
                    property int gpuThreshold: 45
                    property int holdSec: 20
                    property list<string> extraClasses: []
                }
            }

            property list<var> bluetoothDeviceImages: []

            property JsonObject bluetooth: JsonObject {
                property JsonObject budsLink: JsonObject {
                    // Off by default: the BudsLink companion app is an optional,
                    // niche integration. With this off, the bridge process never
                    // spawns (see BudsLinkService.shouldBridgeRun) and every
                    // surface falls back to the generic BlueZ controls.
                    property bool enabled: false
                    property bool preferBudsLink: true
                    property bool showIntegrationNotices: true
                    property bool showBatteryBreakdown: true
                    property bool showControlsInBarPopup: true
                    property bool showControlsInConnectionPopup: true
                    property bool showEnhancedSettingsCard: true
                }
            }

            property JsonObject background: JsonObject {
                property bool enable: true // if someone wants to use an external wallpaper manager, note that its not fully tested but it should just disable background.qml from being loaded
                property bool blurGradientExperiment: false
                property JsonObject referenceResolution: JsonObject {
                    property int width: 1920
                    property int height: 1080
                }
                property JsonObject widgets: JsonObject {
                    // Standard snap step for the desktop wallpaper canvas is 10px.
                    property int gridStep: 10
                    property string colorScheme: "default"
                    property bool tintOpacityEnabled: false
                    property real tintOpacity: 0.55
                    property bool lightweightPreviews: true
                    property bool showOnlyOnSingleMonitor: false
                    property string targetMonitor: ""
                    property JsonObject clock_cookie: JsonObject {
                        property bool enable: false
                        property bool disableAnimationOnLock: false
                        property string placementStrategy: "free"
                        property real x: 1518.98
                        property real y: 168.8
                        property bool aiStyling: false
                        property string aiStylingModel: "gemini"
                        property int sides: 14
                        property string backgroundStyle: "cookie"
                        property string backgroundShape: "Arch"
                        property string dialNumberStyle: "full"
                        property string hourHandStyle: "fill"
                        property string minuteHandStyle: "medium"
                        property string secondHandStyle: "dot"
                        property string dateStyle: "bubble"
                        property bool timeIndicators: true
                        property bool hourMarks: false
                        property bool dateInClock: true
                        property bool constantlyRotate: false
                        property bool quoteEnable: false
                        property string quoteText: ""
                    }
                    property JsonObject clock_expressive_card: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property int widgetSize: 100
                    }

                    property JsonObject clock_flex: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property int widgetSize: 100
                        property bool useAltColors: true
                    }
                    property JsonObject clock_digital: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool adaptiveAlignment: true
                        property bool showDate: true
                        property bool animateChange: true
                        property bool vertical: false
                        property real lineSpacing: 0
                        property bool colorful: false
                        property bool showColon: true
                        property JsonObject font: JsonObject {
                            property real weight: 350
                            property real width: 100
                            property real size: 90
                            property real roundness: 0
                        }
                        property bool quoteEnable: false
                        property string quoteText: ""
                    }
                    property JsonObject clock_nagasaki: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool monochrome: false
                    }
                    property JsonObject clock_word: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property int size: 240
                        property string backgroundStyle: "shape"
                        property string backgroundShape: "Circle"
                    }
                    property JsonObject clock_dial: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool showTicks: true
                        property bool showMinuteHand: true
                        property bool enableShadows: false
                        property bool enableInnerShadow: false
                        property string hourHandStyle: "fill"
                        property string minuteHandStyle: "medium"
                        property bool showSecondHand: false
                        property string secondHandStyle: "dot"
                        property bool showNumberRing: false
                        property bool expressiveColors: false
                    }
                    property JsonObject nagasaki_text: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property int size: 200
                    }
                    property JsonObject clock_hori: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property int widgetSize: 100
                        property bool useAltColors: false
                    }
                    property JsonObject clock_ios: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool showDate: true
                        property real dateSpacing: -8
                        property string clockFontVariant: "bold"
                        property string dateFontVariant: "medium"
                    }
                    property JsonObject clock_nothing: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool use24h: true
                        property bool showAmPmChip: true
                        property bool showTopLabel: true
                        property bool showDate: true
                        property bool useAccentColor: false
                    }
                    property JsonObject nothing_wheel_clock: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject media: JsonObject {
                        property bool enable: true
                        property string style: "circular" // circular, expressive
                        property string placementStrategy: "free" // "free", "leastBusy", "mostBusy"
                        property real x: 249.21
                        property real y: 612.92
                        property bool useAlbumColors: true
                        property bool hideAllButtons: false
                        property bool showPreviousToggle: true
                        property bool tintArtCover: false
                        property string backgroundShape: "Cookie12Sided"  // Options: MaterialShape.Shape enum values as string
                        property bool rotateAlbumArt: true
                        property bool showTimeInfo: true
                        property bool showArtist: true
                        property bool showProgressSlider: true
                        property bool dynamicAlbumColors: false
                        property JsonObject glow: JsonObject {
                            property bool enable: true
                            property real brightness: 10
                        }
                        property JsonObject visualizer: JsonObject {
                            property bool enable: false
                            property real opacity: 0.15
                            property int smoothing: 2
                            property int blur: 1
                        }
                    }
                    property JsonObject circular_media: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 249.21
                        property real y: 612.92
                        property bool useAlbumColors: true
                        property bool enableGlassReflection: true
                        property bool enableShadows: false
                        property bool showPrevButton: true
                        property bool showNextButton: true
                        property bool showDevicePill: true
                        property string progressShape: "Cookie9Sided"
                        property int widgetSize: 100
                    }
                    property JsonObject wearos_clock: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 400
                        property real y: 100
                        property bool useAlbumColors: true
                        property bool enableGlassReflection: true
                        property bool showDistroLogo: true
                        property bool showSunsetComplication: true
                        property bool showDigitalTimePill: true
                        property bool showBatteryPill: true
                        property bool showHourSubDial: true
                        property bool showBedtimeIcon: true
                        property bool showKdeConnect: true
                        property bool showDateComplication: true
                        property bool showMinuteHand: true
                        property bool showOuterNumbers: true
                        property bool showInnerNumbers: true
                        property bool showBezelRing: true
                        property bool enableShadows: false
                        property int widgetSize: 100
                    }
                    property JsonObject wearos_arc_clock: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 400
                        property real y: 100
                        property int widgetSize: 100
                        property bool blackBackground: false
                        property bool enableGlassReflection: true
                        property bool enableBackgroundPattern: true
                        property string leftComplication: "weather"
                        property string rightComplication: "battery"
                        property string bottomComplication: "calendar"
                        property bool enableShadows: false
                    }
                    property JsonObject concentric_clock: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 400
                        property real y: 100
                        property int widgetSize: 100
                        property string dialStyle: "concentric"
                        property string frameStyle: "none"
                        property bool boldFont: false
                        property bool use24h: true
                        property bool showHourText: true
                        property string hourHandStyle: "hide"
                        property string minuteHandStyle: "hide"
                        property string secondHandStyle: "hide"
                        property bool showHourMarks: false
                        property string minuteStyle: "pill_horizontal"
                        property bool showArc24h: false
                        property bool showHourSubDial: false
                        property bool showSunsetDial: false
                        property string bottomSubDialContent: "weather_temp"
                        property bool showMinuteDot: false
                        property bool quoteEnable: false
                        property string quoteText: ""
                        property int minutePillLeftMargin: 67
                        property int subdialMarginOffset: 5
                        property int dialMarginOffset: 3
                        property int hourPixelSize: 30
                        property int hourFontWeight: 600
                        property int hourFontWidth: 85
                        property int hourFontRound: 100
                        property bool useBlackBg: false
                        property bool enableGlassReflection: false
                        property bool enableShadows: false
                    }
                    property JsonObject month_clock: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 400
                        property real y: 100
                        property int widgetSize: 100
                        property bool showMonthRing: true
                        property bool showDayRing: true
                        property bool showWeekRing: true
                        property bool showMonthPill: true
                        property bool showDayPill: true
                        property bool showWeekPill: true
                        property bool showTickMarks: true
                        property bool boldFont: true
                        property bool useBlackBg: true
                        property bool enableGlassReflection: false
                        property string hourHandStyle: "fill"
                        property string minuteHandStyle: "medium"
                        property string secondHandStyle: "line"
                    }
                    property JsonObject scallop_dot_clock: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 400
                        property real y: 100
                        property int widgetSize: 100
                        property bool boldFont: true
                        property bool useBlackBg: false
                        property bool enableGlassReflection: false
                        property bool showHourHand: true
                        property bool showMinuteBubble: true
                        property bool showDots: true
                    }
                    property JsonObject scallop_number_clock: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 400
                        property real y: 100
                        property int widgetSize: 100
                        property bool boldFont: true
                        property bool useBlackBg: false
                        property bool enableGlassReflection: false
                        property bool showHourHand: true
                        property bool showMinuteBubble: true
                        property bool showDots: true
                    }
                    property JsonObject circle_pointer_clock: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 400
                        property real y: 100
                        property int widgetSize: 100
                        property bool boldFont: true
                        property bool useBlackBg: false
                        property bool enableGlassReflection: false
                        property bool showDots: true
                    }
                    property JsonObject triple_ring_clock: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 400
                        property real y: 100
                        property int widgetSize: 100
                        property bool boldFont: true
                        property bool useBlackBg: false
                        property bool enableGlassReflection: false
                    }
                    property JsonObject photo_1x1: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 400
                        property real y: 100
                        property int widgetSize: 100
                        property string backgroundShape: "Cookie9Sided"
                        property string imagePath: ""
                    }
                    property JsonObject android_search_bar: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 400
                        property real y: 100
                        property int widgetSize: 100
                        property string aspectRatio: "0.5x2"
                        property string action1: "music_rec"
                        property string action2: "ai_chat"
                        property string action3: "search"
                    }
                    property JsonObject search_pill: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 400
                        property real y: 100
                        property int widgetSize: 100
                        property string aspectRatio: "0.5x2"
                        property string action1: "ai_chat"
                        property string action2: "music_rec"
                        property string action3: "search"
                        property string aiLogo: "gemini"
                        property string outerLeftIcon: "spark"
                        property bool useMaterialSymbolForOuterLeftIcon: false
                    }
                    property JsonObject resource_cpu_pill: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 400
                        property real y: 100
                        property int widgetSize: 100
                        property string aspectRatio: "2x0.5"
                        property bool showDetails: true
                    }
                    property JsonObject resource_ram_pill: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 400
                        property real y: 100
                        property int widgetSize: 100
                        property string aspectRatio: "2x0.5"
                        property bool showDetails: true
                    }
                    property JsonObject resource_disk_pill: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 400
                        property real y: 100
                        property int widgetSize: 100
                        property string aspectRatio: "2x0.5"
                        property bool showDetails: true
                    }
                    property JsonObject resource_fill_cards: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 400
                        property real y: 100
                        property int widgetSize: 100
                        property string orientation: "horizontal"
                        property bool enableCpu: true
                        property bool enableRam: true
                        property bool enableDisk: true
                    }
                    property JsonObject grid_card_clock: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 400
                        property real y: 100
                        property int widgetSize: 100
                    }
                    property JsonObject at_a_glance: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 400
                        property real y: 100
                        property int widgetSize: 100
                        property int widthCells: 3
                        property bool dualColumnMode: false
                        property list<string> servicePriority: ["media", "calendar", "sports", "todo", "email", "localsend", "kdeconnect", "fallback"]
                        property bool enableMedia: true
                        property bool enableCalendar: true
                        property bool enableSports: true
                        property bool enableTodo: true
                        property bool enableEmail: true
                        property bool enableLocalSend: true
                        property bool enableKdeConnect: true
                        property bool enableWeather: true
                        property int calendarWindowMinutes: 60
                        property int sportsWindowHours: 12
                        property bool showLocation: true
                        property bool showServiceLabel: false
                        property bool showSeparators: true
                        property bool animateContent: true
                    }
                    property JsonObject weather: JsonObject {
                        property bool enable: false
                        property string style: "default" // default, expressive
                        property string backgroundShape: "Cookie9Sided"
                        property string placementStrategy: "free" // "free", "leastBusy", "mostBusy"
                        property real x: 400
                        property real y: 100
                        property bool expressiveColors: false
                    }
                    property JsonObject weather_forecast: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: true
                    }
                    property JsonObject weather_card: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: true
                    }
                    property JsonObject weather_icon: JsonObject {
                        property bool enable: false
                        property string backgroundShape: "Cookie9Sided"
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: true
                    }
                    property JsonObject weather_pill: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: true
                    }
                    property JsonObject weather_circle: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: true
                    }
                    property JsonObject nothing_weather_circle: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    // The EasyEffects preset hero as a desktop widget, in two shapes. What
                    // each one shows is shared (Config.options.easyEffects.widget).
                    property JsonObject easyeffects_preset_portrait: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject easyeffects_preset_landscape: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject volume_mute_pill: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject wifi_pill: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject bluetooth_pill: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject mic_pill: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject dark_mode_pill: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject screen_record_pill: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject easy_effects_pill: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject nothing_ring_media: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject weather_typography: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: true
                    }
                    property JsonObject weather_hourly: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: true
                    }
                    property JsonObject date: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free" // "free", "leastBusy", "mostBusy"
                        property real x: 100
                        property real y: 100
                        property bool expressiveColors: false
                    }
                    property JsonObject calendar_minimal: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject calendar_grid: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject calendar_agenda: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject calendar_next_event: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject calendar_pill: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject calendar_upcoming_3days: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject photo: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property string imagePath: ""
                        property bool expressiveColors: false
                    }
                    property JsonObject photo_weather_2x1: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property string imagePath: ""
                        property bool showOverlay: true
                        property bool expressiveColors: false
                    }
                    property JsonObject photo_pill_2x1: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property string imagePath: ""
                        property bool showOverlay: true
                        property bool expressiveColors: false
                    }
                    property JsonObject photo_minimal_temp_2x1: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property string imagePath: ""
                        property bool showOverlay: true
                        property bool expressiveColors: false
                    }
                    property JsonObject bluetooth_battery: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject bluetooth_headphone: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool halfSize: true
                        property bool expressiveColors: false
                    }
                    property JsonObject mobile_battery: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject bluetooth_headphone_cookie: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property string materialShape: "Cookie12Sided"
                        property bool expressiveColors: false
                    }
                    property JsonObject bluetooth_fill_cards: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject pc_battery_bars: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject pc_battery_cable: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject devices_battery_list: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject devices_battery_list_1x1: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject bluetooth_earbuds_stem: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject email_inbox: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject email_inbox_2x1: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject quote: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                        property string quoteText: ""
                        property real fontSize: 16
                    }
                    property JsonObject quick_actions: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                        property string bottomButton1: "translator"
                        property string bottomButton2: "phone"
                    }
                    property JsonObject ai_chat: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject notes_widget: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject notes_widget_2x1: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                    }
                    property JsonObject media_cd: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool dynamicAlbumColors: false
                        property bool enableShadows: false
                        property bool enableInnerShadow: false
                        property int widgetSize: 100
                    }
                    property JsonObject compact_media: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool dynamicAlbumColors: false
                        property string backgroundShape: "Rectangle"
                        property bool enableShadows: false
                        property bool enableInnerShadow: false
                        property int widgetSize: 100
                    }
                    property JsonObject water_reminder: JsonObject {
                        property bool enable: false
                        property string placementStrategy: "free"
                        property real x: 200
                        property real y: 200
                        property bool expressiveColors: false
                        property int dailyGoal: 8
                        property int intervalHours: 2
                        property string reminderText: "Time to hydrate! 💧"
                    }
                    property bool enableInnerShadow: false
                    property bool enableShadows: false
                    property bool enableGrid: false
                    property bool enableSnap: true
                    property real widgetsScale: 1.0
                    property bool lockWidgetPositions: false
                }
                property list<var> activeWidgets: []
                property bool scaleLargeWallpapers: false
                property bool animateWallpaperChanges: true
                property string wallpaperAnimation: ""
                property bool zoomOutEnabled: true  // master toggle for zoom-out animations
                property bool useBackgroundOverviewAlways: false // keep overview background style permanently active on desktop
                // Desktop shortcut icon scale (apps, folders, files). Fixed
                // steps only — 1, 1.25, 1.5 — written from the Edit Mode
                // panel; the layer re-reads it, one binding, no rebuild.
                property real desktopIconScale: 1.0
                // Drag-lock for desktop shortcut icons. The desktop menu's
                // "Lock icons" row is the writer outside Settings; the layer
                // reads it in one binding and gates only drag START — click,
                // double-click, menu and selection keep working.
                property bool desktopIconsLocked: false
                // The desktop icons' grid, order and look. The cell is
                // `spacing` scaled by desktopIconScale, so a bigger icon
                // takes a bigger cell; everything that places an icon
                // (add, align, sort, a drop with autoArrange) walks the
                // same lattice from `origin` in `flow` order, inside the
                // work area the margins leave.
                property JsonObject desktopIcons: JsonObject {
                    property string spacing: "normal" // compact (80) | normal (100) | wide (120)
                    property bool autoArrange: false // a drop lands on the nearest free cell
                    property string origin: "topLeft" // topLeft | topRight | bottomLeft | bottomRight
                    property string flow: "columns" // columns: fill down first | rows: fill across first
                    property bool avoidPanels: true // keep the bar's and the dock's edges clear
                    property int margin: 12 // extra gap on every edge of the work area
                    property string sortBy: "name" // name | type | added | used
                    property bool sortDescending: false
                    property bool keepSorted: false // re-sort whenever an icon comes or goes
                    property bool stacks: false // one stack per kind: apps, folders, files
                    property bool hidden: false // a clean desktop; the icons stay stored
                    property string labels: "always" // always | hover | never
                    property int labelLines: 1 // 1 | 2
                    property string labelStyle: "auto" // auto | shadow | pill
                    property string iconBackground: "none" // none | translucent | circle | squircle
                    property bool runningBadges: true // the dock's "open" dot under running apps
                    property bool notificationBadges: true // unread count from the app's notifications
                }
                property bool windowZoomOnOverview: true // fake window scale-out during overview (GNOME-like)
                property bool windowZoomLiveCapture: true // keep screencopy live instead of freezing on overview open
                // Semantic style name. Empty keeps the legacy numeric setting
                // active until the user chooses a new preset in Settings.
                property string overviewBackgroundStyle: ""
                property bool materialShapeShadow: false
                property real materialShapeScale: 1.0
                property bool cheatsheetZoomOut: true
                property bool overviewZoomOut: true
                property bool workspaceBlur: false
                property string wallpaperPath: ""
                property string lockscreenWallpaperPath: ""
                property bool useSeparateLockscreenWallpaper: false
                property string lightModeWallpaperPath: ""
                property bool useSeparateLightModeWallpaper: false
                // Per screen, written by Edit Mode's Wallpaper catalogue
                // (services/WallpaperLayout.qml): a screen's own wallpaper and
                // how it frames each picture it has shown.
                //   [{ monitor, key, path, framings: [{ path, zoom, x, y, rotation, flipH, flipV }] }]
                // `key` = the monitor's "model/serial" ("" when it cannot be
                // told apart), so an entry follows the monitor across ports.
                // `path` "" = the shared wallpaper above, which is also the
                // one the colours come from. Machine-local (monitor names),
                // so presets leave it alone (presets_helper.py).
                property list<var> monitorWallpapers: []
                property string thumbnailPath: ""
                // Who plays a video wallpaper: "mpvpaper" (separate layer, the
                // shell only shows its poster frame and turns image effects off)
                // or "shell" (played inside the wallpaper plane, so blur,
                // parallax and the overview zoom apply to it).
                property string videoBackend: "mpvpaper"
                // Shell backend only: also pause while any window is open on the
                // active workspace. Fullscreen/maximized windows always pause it.
                property bool videoPauseWhenWindowsOpen: false
                // Shell backend only: freeze the frame while the lock screen is up.
                property bool videoPauseOnLock: true
                // Per video, the moment colors and the poster frame come from:
                // [{ path, seconds }]. Missing = first frame (switchwall.sh).
                property list<var> videoFrameTimes: []
                // Play a screen-sized copy of videos taller than the screen
                // (scripts/videos/video-proxy.sh, made once in the background).
                property bool videoDownscale: true
                // Written by switchwall.sh: the file to open for videoPlaybackSource
                // (its screen-sized copy once one exists).
                property string videoPlaybackSource: ""
                property string videoPlaybackPath: ""
                property bool hideWhenFullscreen: true
                property bool useWallpaperEngine: false
                property string wallpaperEngineId: ""
                property string wallpaperEngineAssetsPath: ""
                property bool wpeSilent: true
                property real wpeVolume: 50
                property bool wpeNoAutoMute: false
                property bool wpeNoAudioProcessing: false
                property int wpeFps: 30
                property string wpeScreenSpan: ""
                property string wpeScaling: "default"
                property bool wpeDisableMouse: false
                property bool wpeDisableParallax: false
                property bool wpeNoFullscreenPause: false
                property bool wpePauseWhenWindowsOpen: false
                // Deprecated compatibility mapping: 0 -> gnome, 1 -> soft-focus, 2 -> camera-push.
                property int zoomOutStyle: 2
                property bool blurWhenWindowsOpen: true
                property int blurWhenWindowsOpenRadius: 41
                property JsonObject gradientBlur: JsonObject {
                    property bool enable: false
                    property int radius: 50
                    property string direction: "top-to-bottom"
                }
                property JsonObject parallax: JsonObject {
                    property bool vertical: false
                    property bool autoVertical: false
                    property bool enableWorkspace: true
                    property real workspaceZoom: 1.07 // Relative to wallpaper size
                    property bool enableSidebar: true
                    property real widgetsFactor: 1.2
                    property bool loop: true
                    property bool invertHorizontal: false
                    property bool invertVertical: false
                    property int intensity: 4
                }
                property JsonObject mediaMode: JsonObject {
                    property bool immersive: false
                    property bool togglePerMonitor: true
                    property string backgroundShape: "Square"
                    property bool enableBackgroundAnimation: true // It **may** cause nausea for someone
                    property bool changeShellColor: true // Changes the shell color to the album color
                    property int backgroundOpacity: 50 // In percent
                    property int backgroundBlurRadius: 120
                    property int visualizerMode: 0 // 0: Off, 1: Waves, 2: Bars, 3: Radial
                    property bool showLyrics: true
                    property bool showPlayerSwitcher: true
                    property bool showSeekBar: true
                    property bool showVolumeSlider: true
                    property int lyricsOffsetMs: 0 // offset in milliseconds for lyrics sync adjustment
                    property JsonObject backgroundAnimation: JsonObject {
                        property bool enable: true
                        property int speedScale: 10 // 1: very slow, 10: default, 20: 2x speed etc.
                    }
                    property JsonObject syllable: JsonObject {
                        property int textHighlightStyle: 1 // 0: vertical, 1: horizontal (not perfect bc its not synced in a word level, but a cool animation to have)
                    }
                    property JsonObject musicVideo: JsonObject {
                        property bool enable: true
                        property int maxResolution: 1080
                        property bool dimBackground: true
                        property int dimOpacity: 60   // percent (0-100)
                        property string searchSuffix: "official music video"
                        property int videoSamplingInterval: 200 // ms between color sampling updates (100-5000)
                    }
                    property JsonObject crossfade: JsonObject {
                        property bool enable: false
                        property int durationSec: 3
                    }
                }
            }

            // ── Dynamic Island ───────────────────────────────────────────────────
            // Its own block rather than ~40 flat keys under `bar`: the island is a
            // module, not a bar style, and both of its styles read the same settings.
            // Per-activity objects keep a widget's settings together and let the
            // settings page be generated from IslandRegistry instead of being hand
            // written per activity. Migrated from `bar.floatingNotch` at v23.
            property JsonObject dynamicIsland: JsonObject {
                property bool enable: false
                // "notch" hangs from the top edge (the legacy look); "pills" is the
                // floating cluster of up to three islands.
                property string style: "pills"

                property JsonObject monitor: JsonObject {
                    property bool followFocus: true
                    property string name: ""
                }

                property JsonObject behavior: JsonObject {
                    property bool autoHide: false
                    property bool hideOnFullscreen: true
                    // Hover is an intent, not a hit: see IslandHoverIntent.
                    property int sideExpandDwellMs: 160
                    property int dashboardDwellMs: 320
                    // One grace for the island and for its bubbles; see
                    // IslandPolicy.collapseGraceMs. 450 is what the bubbles already
                    // used, and the island now matches them instead of lingering.
                    property int collapseGraceMs: 450
                    // How long state-derived triggers stay muted after a boot, a reload
                    // or an unlock, when the system restores state in bulk.
                    property int quietWindowMs: 1200
                    // Hold to reveal: the dashboard only opens once the pointer has
                    // rested on the island for this long, and the island swells while
                    // it waits so the hold is visible. Off: a hover opens it.
                    property bool holdToReveal: false
                    property int holdToRevealMs: 700
                    // Keep the dashboard's quick-toggle grid resident. Off: the grid
                    // is built on open and released after the closing fade. On: built
                    // once and kept, so openings are instant at the price of the
                    // tiles' RAM while idle.
                    property bool keepDashboardLoaded: true
                }

                property JsonObject appearance: JsonObject {
                    // The island's base height. Activities that need more ask for it in
                    // IslandRegistry; everything else is this tall.
                    property int height: 42
                    property bool dropShadow: true
                    // "layer" follows the shell's surfaces; "deep" is the darker,
                    // iOS-like ground.
                    property string surface: "layer"
                }

                property JsonObject notch: JsonObject {
                    property bool centerInBar: false
                    property bool clickToExpand: false
                    property bool singleWidgetExpanded: false
                }

                property JsonObject pills: JsonObject {
                    property int height: 38
                    property int gap: 8
                    property int maxIslands: 3
                    // The liquid neck between two islands as one detaches. Falls back to
                    // a plain slide where the shader is unavailable.
                    property bool gooeyMorph: true
                }

                property JsonObject dashboard: JsonObject {
                    property list<string> pages: ["controls", "notifications", "widgets", "wallpapers", "workspaces"]
                    property bool showActivityHero: true
                    property string photoWidgetPath: ""
                    property JsonObject photoWidgetImages: JsonObject {}
                    // The dashboard's grid: the sidebar's quick-toggle system on one page,
                    // sized by its columns and rows. `rows` is a hard limit - edits that
                    // would pack past it are refused.
                    property JsonObject quickToggles: JsonObject {
                        property int columns: 6
                        property int rows: 6
                        property int layoutVersion: 2
                        property list<var> pages: [[
                                { "id": "dashboardToolbar", "type": "dashboardToolbar", "sizeW": 2, "sizeH": 1 },
                                { "id": "network", "type": "network", "sizeW": 2, "sizeH": 1 },
                                { "id": "bluetooth", "type": "bluetooth", "sizeW": 2, "sizeH": 1 },
                                { "id": "volumeSlider", "type": "volumeSlider", "sizeW": 3, "sizeH": 1 },
                                { "id": "brightnessSlider", "type": "brightnessSlider", "sizeW": 3, "sizeH": 1 },
                                { "id": "darkMode", "type": "darkMode", "sizeW": 2, "sizeH": 1 },
                                { "id": "nightLight", "type": "nightLight", "sizeW": 2, "sizeH": 1 },
                                { "id": "idleInhibitor", "type": "idleInhibitor", "sizeW": 2, "sizeH": 1 },
                                { "id": "mediaWidget", "type": "mediaWidget", "sizeW": 4, "sizeH": 2 },
                                { "id": "notifications", "type": "notifications", "sizeW": 1, "sizeH": 1 },
                                { "id": "mic", "type": "mic", "sizeW": 1, "sizeH": 1 },
                                { "id": "screenSnip", "type": "screenSnip", "sizeW": 1, "sizeH": 1 },
                                { "id": "colorPicker", "type": "colorPicker", "sizeW": 1, "sizeH": 1 },
                                { "id": "powerProfile", "type": "powerProfile", "sizeW": 2, "sizeH": 1 },
                                { "id": "audio", "type": "audio", "sizeW": 2, "sizeH": 1 },
                                { "id": "screenRecord", "type": "screenRecord", "sizeW": 2, "sizeH": 1 }
                        ]]
                    }
                }

                // One object per activity in IslandRegistry. `notchHeight` is the
                // contracted height the notch style gives it.
                property JsonObject widgets: JsonObject {
                    property JsonObject clock: JsonObject {
                        property bool enable: true
                        property bool showDate: true
                        property int notchHeight: 36
                    }
                    property JsonObject media: JsonObject {
                        property bool enable: true
                        property string side: "right"
                        property int notchHeight: 52
                    }
                    property JsonObject workspaces: JsonObject {
                        property bool enable: true
                        property string side: "left"
                        property int ttlMs: 2000
                        property bool ignoreSpecial: true
                        property int notchHeight: 36
                    }
                    property JsonObject notification: JsonObject {
                        property bool enable: true
                        property int ttlMs: 4500
                        property int notchHeight: 60
                        // The contracted face as one slim line (title, then the body) instead
                        // of the title over the body.
                        property bool oneLine: false
                    }
                    property JsonObject osd: JsonObject {
                        property bool enable: true
                        property int notchHeight: 72
                    }
                    property JsonObject ai: JsonObject {
                        property bool enable: true
                        property string side: "right"
                        // CPU-based detection cannot see a turn that is purely
                        // "thinking"; hooks report the real state. Off by default.
                        property bool processFallback: false
                        property int notchHeight: 36
                    }
                    property JsonObject clipboard: JsonObject {
                        property bool enable: true
                        property int ttlMs: 2500
                        property int notchHeight: 36
                    }
                    property JsonObject timer: JsonObject {
                        property bool enable: true
                        property string side: "left"
                        property int notchHeight: 36
                    }
                    property JsonObject recording: JsonObject {
                        property bool enable: true
                        property string side: "left"
                        property int notchHeight: 36
                    }
                    property JsonObject dictation: JsonObject {
                        property bool enable: true
                        property string side: "left"
                        property int notchHeight: 44
                    }
                    property JsonObject battery: JsonObject {
                        property bool enable: true
                        property int ttlMs: 5000
                        property int notchHeight: 36
                    }
                    property JsonObject wifi: JsonObject {
                        property bool enable: true
                        property int ttlMs: 3000
                        property int notchHeight: 36
                    }
                    property JsonObject bluetooth: JsonObject {
                        property bool enable: true
                        property int ttlMs: 3000
                        property int notchHeight: 88
                    }
                    property JsonObject keyboard: JsonObject {
                        property bool enable: true
                        property string side: "left"
                        property int notchHeight: 36
                    }
                    property JsonObject localSend: JsonObject {
                        property bool enable: true
                        property bool kdeConnectColumn: true
                        property int notchHeight: 42
                    }
                    property JsonObject progress: JsonObject {
                        property bool enable: true
                        property string side: "left"
                        property int notchHeight: 48
                    }
                    property JsonObject mode: JsonObject {
                        property bool enable: true
                        property int ttlMs: 3000
                        property int notchHeight: 36
                    }
                    property JsonObject update: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject search: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject wallpaper: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject session: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject colorPicker: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject privacy: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject phoneCall: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject fingerprint: JsonObject {
                        property bool enable: true
                    }
                    // Only ever present when a prompt route is opted into (the
                    // `askpass*` switches under bar.floatingNotch); on by itself.
                    property JsonObject askpass: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject alarm: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject windowSwitcher: JsonObject {
                        property bool enable: true
                    }
                    // A Medium/Strong reminder alerting (off: the full-screen alert shows instead).
                    property JsonObject reminder: JsonObject {
                        property bool enable: true
                    }
                    // The next reminder within the hour, beside the clock.
                    property JsonObject reminderSoon: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject songRec: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject sports: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject vpn: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject discordVoice: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject phoneLink: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject phoneMirror: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject earbuds: JsonObject {
                        property bool enable: false
                        property string side: "right"
                        property int notchHeight: 36
                    }
                    // A phone connected over Bluetooth, as a bubble (only while bubbles are on).
                    property JsonObject btPhone: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject weather: JsonObject {
                        property bool enable: false
                        property string side: "right"
                        property int notchHeight: 36
                    }
                    property JsonObject batteryGlance: JsonObject {
                        property bool enable: false
                        property string side: "right"
                        property int notchHeight: 36
                    }
                    property JsonObject systemTray: JsonObject {
                        property bool enable: false
                        property string side: "right"
                    }
                    property JsonObject easyEffects: JsonObject {
                        property bool enable: true
                        property string side: "right"
                    }
                    // The teleprompter: a user script scrolling through the island.
                    // Every value here is read (and clamped) in one place, by
                    // services/Teleprompter.qml; the faces and the Settings page only
                    // ever talk to the service.
                    property JsonObject teleprompter: JsonObject {
                        property bool enable: false
                        // Lines shown in the contracted face (1..4); the island's
                        // height follows it live.
                        property int lines: 2
                        // Lines shown in the card the hover grows the island into.
                        property int expandedLines: 5
                        // Width of the island's face while prompting.
                        property int width: 520
                        property int fontSize: 22
                        property bool bold: true
                        // Scroll speed, in pixels per second.
                        property int speed: 45
                        // Restart from the top when the script ends.
                        property bool loop: false
                        // 3..2..1 overlay before the scroll starts; 0 disables it.
                        property int countdownSeconds: 3
                        // Flip the text horizontally, for teleprompter glass rigs.
                        property bool mirror: false
                        property bool showProgress: true
                        // Keep the island on screen while prompting: it overrides
                        // auto-hide and the fullscreen hide, because a prompter being
                        // read is the point of the screen.
                        property bool holdVisible: true
                        // The script itself, edited in Settings → Features → Teleprompter.
                        property string text: ""
                        // How long a finished script holds the island before it
                        // gives the surface back; 0 keeps it until stopped by hand.
                        property int finishHoldSeconds: 4
                    }
                }
            }

            property JsonObject bar: JsonObject {
                property bool borderless: false
                property bool expressiveGroupColor: false
                property JsonObject clock: JsonObject {
                    property string colorMode: "primary"
                    property bool showSeconds: false
                    property bool secondaryOpposite: false
                    property bool showPrimary: true
                    property bool showSecondary: true
                    property bool swapPrimaryWithSecondary: false
                }
                property JsonObject styles: JsonObject {
                    property string activeWindow: "default"
                    property string clock: "expressive" // default, material, expressive, neural, relief
                    property string media: "default" // default | expressive | neural | ring | tonal
                    property string notification: "default"
                    property string utilButtons: "expressive" // default | expressive | segments
                    property string workspaces: "default" // default | minimal | expressive | dock | index
                    property string weather: "expressive"
                    property string dashboard: "expressive" // default | expressive | orbs
                    property string resources: "expressive"
                    property string policies: "expressive" // default | expressive | outline
                    property string power: "expressive" // default | expressive | solid | dot
                    property string battery: "expressive"
                    property string systray: "expressive"
                    property string bluetooth: "expressive"
                    property string keyboard: "expressive"
                    property string sports: "expressive"
                    property string portWatcher: "expressive"
                    property string aiPlanUsage: "expressive"
                    property string search: "default"
                    property string date: "default"
                    property string timer: "expressive"
                    property string recordIndicator: "expressive" // default | expressive | neural
                }

                property JsonObject activeWindow: JsonObject {
                    property string colorMode: "primary"
                    property bool fixedSize: false
                    property int customSize: 225
                    property bool showOnAllMonitors: false
                    property bool animateTransition: true
                }

                property JsonObject weatherWidget: JsonObject {
                    property string horizonVariant: "balanced" // balanced | inverted | minimal
                    property string tesseraVariant: "paired" // paired | contrast | bare
                    property string colorMode: "tonal" // Material pairs: tonal | vibrant | neutral
                }

                property JsonObject autoHide: JsonObject {
                    property bool enable: false
                    property string mode: "instant" // "instant" | "dwell" | "wide" | "cautious"
                    property int hoverRegionWidth: 2
                    property int hoverDelay: 0
                    property bool pushWindows: false
                    property JsonObject showWhenPressingSuper: JsonObject {
                        property bool enable: true
                        property int delay: 140
                    }
                }

                property bool bottom: false // Instead of top
                property int cornerStyle: 0 // 0: Hug | 1: Float | 2: Plain rectangle
                property bool floatStyleShadow: true // Show shadow behind bar when cornerStyle == 1 (Float)
                property bool dropShadow: false
                property int dynamicIslandSpacingHorizontal: 48
                property int dynamicIslandSpacingVertical: 16
                property bool dynamicIslandLoadBalance: true

                property JsonObject dynamicIsland: JsonObject {
                    property JsonObject notchMode: JsonObject {
                        property bool enable: false
                        property int expandAnimDuration: 250
                        property int fadeDelay: 0
                        property list<string> visibleWidgets: []
                        property bool overlapApps: false
                    }
                }

                property JsonObject floatingNotch: JsonObject {
                    property bool enable: false
                    property bool autoHide: false
                    // How long the pointer has to rest on the island before it opens
                    // its expanded face. Hovering reveals the contracted face at once.
                    property int hoverExpandDelayMs: 600
                    // The island's outer shell. "notch" hangs from the top edge with
                    // concave shoulders; "island" is a free-floating pill that slides out
                    // of view when it hides. Both host the same widgets.
                    property string shape: "notch"
                    // A second, round surface beside the island. When media or a
                    // workspace change arrives while the island already shows something,
                    // it moves out into the bubble instead of taking the island over.
                    property bool auxiliaryBubble: true
                    // The wallpaper picker opens inside the island - one row of
                    // wallpapers with the path above and the toolbars below - instead of
                    // the full-screen selector. Off gives back the standalone selector.
                    property bool integratedWallpaperBrowser: true
                    // How the island's picker lays its wallpapers out: "carousel" (a
                    // cover-flow that loops) or "row" (the plain row of four).
                    property string wallpaperBrowserStyle: "row"
                    // The workspace overview under the island is a small fixed grid
                    // (2 x 3) that opens with the island instead of playing its own
                    // entrance. Off gives back the desktop overview's own layout,
                    // scale and animations, which the settings page then unlocks.
                    property bool integratedOverview: true
                    // The session menu opens inside the island - the same eight actions
                    // in the same four-by-two grid - instead of the full-screen session
                    // screen. Off gives back the session screen.
                    property bool integratedSessionMenu: true
                    // The colour picker's result and an incoming LocalSend transfer are
                    // shown in the island, in the popup's own vertical layout, instead
                    // of as floating cards by the screen edge.
                    property bool integratedPopups: true
                    // Password prompts on the island. Every route is opt-in: `sudo -A` (and
                    // SUDO_ASKPASS without a terminal), plain `sudo` typed in a terminal,
                    // polkit, and ssh/git passphrases. See AskpassService.
                    property bool askpassSudo: false
                    property bool askpassTerminal: false
                    property bool askpassPolkit: false
                    property bool askpassSsh: false
                    // "card" or "pill": prompts from sudo -A / GUI apps, and from terminals.
                    property string askpassStyle: "card"
                    property string askpassTerminalStyle: "pill"
                    property bool dropShadow: false
                    property bool onlyShowOnSingleMonitor: false
                    property string singleMonitorName: ""

                    // Disables
                    property bool disableWorkspaces: false
                    // The workspace change moves out into an auxiliary bubble beside
                    // the island. Off keeps the strip on the island itself.
                    property bool disableWorkspacesBubble: false
                    property bool disableKeyboard: false
                    property bool disableWifi: false
                    property bool disableBluetooth: false
                    property bool disableMedia: false
                    property bool disableNotification: false
                    property bool disableOsd: false
                    property bool disableRecording: false
                    property bool disableDictation: false
                    property bool disableTimer: false
                    property bool disableClipboard: false
                    property bool disableLocalSend: false
                    property bool disableKdeConnectInLocalSend: false
                    property bool disableProgress: false
                    property bool disableBattery: false
                    // Side glances: opt-in, unlike the announcement notches.
                    property bool disableEarbuds: true
                    property bool disableWeather: true
                    property bool disableBatteryGlance: true
                    property bool disableAiStatus: false
                    property bool disableMode: false
                    property bool disableUpdate: false
                    // Sensor-in-use dot beside the clock (microphone, camera, screen share).
                    property bool disablePrivacy: false
                    // A call ringing on the paired phone, then the call in progress.
                    property bool disablePhoneCall: false
                    // Any fingerprint request (sudo, polkit) asks for a touch here.
                    property bool disableFingerprint: false
                    // A ringing alarm; replaces the fullscreen alarm popup while on.
                    property bool disableAlarm: false
                    // Alt+Tab draws on the island (off: the floating switcher panel instead).
                    property bool disableWindowSwitcher: false
                    // A Medium/Strong reminder alerting; replaces the full-screen reminder alert.
                    property bool disableReminder: false
                    // The next reminder within the hour beside the clock. On by default,
                    // unlike the other glances: it is only there in the hour before one.
                    property bool disableReminderSoon: false
                    property bool disableSongRec: false
                    // Live games of the teams in the bar's sports team filter.
                    property bool disableSports: false
                    property bool disableVpn: false
                    // A Discord voice call: the join, then who is talking beside the clock.
                    property bool disableDiscordVoice: false
                    // The phone's camera or microphone streaming into this computer.
                    property bool disablePhoneLink: false
                    // The phone mirrored into a scrcpy window, full screen or one app.
                    property bool disablePhoneMirror: false
                    // The tray's programs as an auxiliary bubble beside the island: the
                    // icons contracted, the aligned grid of programs expanded. Opt-in,
                    // like the side glances — the bar's tray already carries the same set.
                    property bool disableSystemTray: true
                    // EasyEffects' preset as a bubble for as long as it runs: scroll to
                    // switch, rest on it for the presets, bypass and the app.
                    property bool disableEasyEffects: false
                    property bool clickToExpand: false
                    property bool centerInBar: false // "Dynamic Island in bar center" integration mode
                }

                property int barGroupStyle: 0 // 0: Pills | 1: Island (opaque) | 2: Transparent (or maybe line-separated in the future)
                property string topLeftIcon: "spark" // Options: "distro" or any icon name in ~/.config/quickshell/ii/assets/icons
                property bool useMaterialSymbolForTopLeftIcon: false
                property int barBackgroundStyle: 1 // 0: Transparent | 1: Visible | 2: Adaptive
                property bool transparentGlow: true
                property bool expressiveColors: false
                property string expressiveColorTheme: "content"
                property bool verbose: true
                property bool vertical: true
                property bool enableVolumeScroll: true
                property bool enableBrightnessScroll: true
                property bool sidebarClickOpen: true

                property JsonObject mediaPlayer: JsonObject {
                    property string colorMode: "primary"
                    property string popupStyle: "android" // "default" | "expressive" | "android"
                    property bool expressivePopup: false
                    property bool useFixedSize: false
                    property int customSize: 200
                    property int maxSize: 400
                    property bool enableVolumeScroll: false
                    property JsonObject artwork: JsonObject {
                        property bool enable: false
                    }
                    property JsonObject lyrics: JsonObject {
                        property bool enable: true
                        property int customSize: 300
                        property string style: "scroller" // Options: scroller, static
                        property bool useGradientMask: true
                    }
                }

                property JsonObject resources: JsonObject {
                    property bool showPercentageText: true
                    property bool alwaysShowRam: true
                    property bool alwaysShowCpu: true
                    property bool alwaysShowCpuTemp: false
                    property bool alwaysShowDisk: false
                    property bool alwaysShowSwap: false
                    property int memoryWarningThreshold: 95
                    property int swapWarningThreshold: 85
                    property int cpuWarningThreshold: 90
                    property bool expressivePopup: true
                    property bool showDocker: true
                }

                property JsonObject aiPlanUsage: JsonObject {
                    property string colorMode: "primary"
                    property bool enabled: true
                    property bool autoRefresh: true
                    property int refreshInterval: 300000
                    // Enabling a remote provider authorizes read-only quota
                    // requests with credentials discovered from its client.
                    // Tokens are never copied into config.json or the cache.
                    property bool claudeNetworkEnabled: true
                    property list<string> enabledProviders: ["chatgpt", "claude", "antigravity"]
                    property string visualization: "resource" // resource | semicircle | circle | shape | bar | text | capsule | mood | duo
                    property string percentMode: "remaining" // remaining | used
                    property bool showWindowLabel: false
                    property bool hideWhenUnavailable: false
                    property int lowRemainingThreshold: 20
                }

                property JsonObject portWatcher: JsonObject {
                    property string colorMode: "primary"
                    property bool enabled: true
                    property bool autoRefresh: true
                    property int refreshInterval: 5000
                    property bool showTcp: true
                    property bool showUdp: true
                    property bool showLoopback: true
                    // Ports without an owning user process (system daemons, other
                    // users) are noise for the widget's purpose, so they are off.
                    property bool showSystem: false
                    property bool exposedOnly: false
                    property bool notifyNewExposed: false
                    property bool hideWhenEmpty: false
                    property int minPort: 1
                    // Stop below the kernel's ephemeral range (32768+). Those are
                    // an app's transient IPC sockets churning, never a port
                    // anybody chose to serve on.
                    property int maxPort: 32767
                    // Comma separated ports or ranges, e.g. "3000, 5173, 8000-8999".
                    property string watchPorts: ""
                    property string ignorePorts: ""
                    property string ignoreProcesses: ""
                    property string sortMode: "port" // port | process | activity
                }

                property JsonObject privacyPill: JsonObject {
                    property bool enabled: true
                    property bool watchCamera: true
                    property bool watchMicrophone: true
                    property bool watchScreen: true
                    // GeoClue keeps its client latched while anything holds a
                    // position source — including this shell's own weather GPS —
                    // so watching it would pin the pill on. Opt-in.
                    property bool watchLocation: false
                    property int pollInterval: 1200
                    // How long the pill stays expanded before collapsing to the dot.
                    property int expandDuration: 4000
                    property bool collapseToDot: true
                    property bool showAppNames: true
                    property string ignoreApps: ""
                }

                property JsonObject searchWidget: JsonObject {
                    property string sizeMode: "compact" // compact | balanced | extended
                    property string colorMode: "tonal" // tonal | vibrant | neutral
                    property bool showShortcutHint: true
                }

                property JsonObject dateWidget: JsonObject {
                    property string expressiveVariant: "stack" // stack | badge | ribbon
                    property string neuralVariant: "orbit" // orbit | glyph | inlay
                    property string colorMode: "tonal" // tonal | vibrant | neutral
                    property bool uppercase: true
                    property bool showYear: false
                }

                property JsonObject clockWidget: JsonObject {
                    property string neuralVariant: "orbit" // orbit | bloom | dial
                    property string reliefVariant: "split" // split | seam | outline
                    property string colorMode: "tonal" // tonal | vibrant | neutral
                    property bool showMeridiem: true
                }

                property JsonObject sports: JsonObject {
                    property string colorMode: "primary"
                    property bool enable: true
                    property bool showBRA: true
                    property bool showBUND: false
                    property bool showCL: true
                    property bool showCLA: true
                    property bool showEPL: true
                    property bool showLIGA: true
                    property bool showLIG1: false
                    property bool showSERA: false
                    property bool showUECL: false
                    property bool showUEL: false
                    property bool showWC: true
                    property bool showWWC: false
                    property list<var> monitoredLeagues: [
                        {
                            "enabled": true,
                            "league": "bra.1",
                            "name": "Brasileir\u00e3o",
                            "sport": "soccer"
                        },
                        {
                            "enabled": true,
                            "league": "eng.1",
                            "name": "Premier League",
                            "sport": "soccer"
                        },
                        {
                            "enabled": true,
                            "league": "uefa.champions",
                            "name": "Champions League",
                            "sport": "soccer"
                        }
                    ]
                    property string teamFilter: ""
                    property int updateInterval: 60
                    property int maxCardsPopup: 4
                    property int showBeforeHours: 12
                    property int showAfterMinutes: 180
                    property string activeGameId: ""
                    property list<var> customOrder: []
                }
                property list<string> screenList: [] // List of output names, like "eDP-1"; the window-manager seam lists them
                property bool onlyShowOnSingleMonitor: false
                property string singleMonitorName: ""

                property JsonObject timers: JsonObject {
                    property string colorMode: "primary"
                    property bool showPomodoro: true
                    property bool showStopwatch: true
                    property bool showCountdowns: true
                }
                property JsonObject utilButtons: JsonObject {
                    property string colorMode: "primary"
                    property bool showScreenSnip: false
                    property bool showColorPicker: true
                    property bool showMicToggle: false
                    property bool showKeyboardToggle: false
                    property bool showDarkModeToggle: false
                    property bool showPerformanceProfileToggle: false
                    property bool showScreenRecord: true
                    property bool isRecording: false
                    property bool showWallpaperToggle: true
                    property bool showPhoneMirror: false
                }
                property JsonObject workspaces: JsonObject {
                    property string colorMode: "primary"
                    property bool monochromeIcons: false
                    property int shown: 5
                    property bool showAppIcons: false
                    property bool alwaysShowNumbers: true
                    property int showNumberDelay: 300 // milliseconds
                    property list<string> numberMap: [] // Characters to show instead of numbers on workspace indicator
                    property bool useWorkspaceMap: false
                    property list<var> workspaceMap: [0, 10]
                    property int maxWindowCount: 2 // Maximum windows to show in one workspace
                    property bool useNerdFont: false
                    property int activeIndicatorOpacity: 100 // 0-100
                    property bool dynamicWorkspaces: false
                    property bool useMaterialShapeForActiveIndicator: false
                    property bool useRandomShapeForActiveIndicator: true
                    // The active indicator becomes a triangle pointing the way
                    // you just moved, then settles back into a circle. Takes
                    // precedence over the random shape while it is on.
                    property bool useDirectionArrowForActiveIndicator: false
                    property string activeIndicatorShape: "Pentagon"
                    property bool dockShowActiveIndicator: true
                    property bool dockShowWindowDots: true
                    property bool dockHoverEffect: true
                    property bool dockShowAppIcons: true

                    property bool autoCompact: false // Run the workspace compactor automatically when a gap appears
                    property string autoCompactCurrentGap: "onswitch" // Gap on the current workspace: "onswitch" | "immediate" | "never"
                    property int autoCompactDelay: 600 // ms of quiet before an automatic compaction fires
                }
                property JsonObject weather: JsonObject {
                    property bool enable: false
                    property bool enableGPS: true // gps based location
                    property string city: "" // When 'enableGPS' is false
                    property bool useUSCS: false // Instead of metric (SI) units
                    property int fetchInterval: 10 // minutes
                }
                property JsonObject indicators: JsonObject {
                    property JsonObject notifications: JsonObject {
                        property bool showUnreadCount: true
                    }
                    property JsonObject record: JsonObject {
                        // Minimal drops the elapsed time and keeps the mark, in
                        // every design. See RecordIndicator.qml.
                        property bool minimal: false
                        property string expressiveVariant: "capsule" // capsule | badge | ribbon
                        property string neuralVariant: "duo" // duo | slab | meter
                        // `alert` is the error family — the default here, and the
                        // only widget entitled to it. See BarWidgetPalette.
                        property string colorMode: "alert" // alert | tonal | vibrant | neutral
                        property bool showLabel: true
                        property bool animateDigits: true
                    }
                }
                property JsonObject dashboardButton: JsonObject {
                    property string colorMode: "primary"
                    // Orbs style: a solid disc per indicator, or a ring with the
                    // bar showing through it.
                    property string orbVariant: "filled" // filled | outline
                    property bool showCaffeine: true
                    property bool showVolume: false
                    property bool showMic: true
                    property bool showNetwork: true
                    property bool showBluetooth: true
                    property bool showVpn: true
                    property bool showTailscale: true
                    property bool showNotifications: true
                    property bool showPomodoro: true
                    property bool showStopwatch: true
                    property bool showCountdowns: true
                    property bool showEasyEffects: true
                    property bool showDns: true
                    property bool showGameMode: true
                    property bool showPowerProfile: false
                    property bool showMusicRecognition: true
                    property bool showAlarms: true
                }
                property JsonObject layouts: JsonObject {
                    // Only storing id and layout-specific flags (visible, centered)
                    // Component display info (icon, title) comes from BarComponentRegistry
                    property list<var> left: [
                        {
                            "centered": false,
                            "id": "policies_panel_button",
                            "visible": true
                        },
                        {
                            "centered": false,
                            "id": "workspaces",
                            "visible": true
                        },
                        {
                            "centered": false,
                            "id": "record_indicator",
                            "visible": false
                        },
                        {
                            "centered": false,
                            "id": "mode_indicator",
                            "visible": false
                        }
                    ]
                    property list<var> center: [
                        {
                            "centered": false,
                            "id": "clock",
                            "visible": true
                        },
                        {
                            "centered": false,
                            "id": "weather",
                            "visible": true
                        }
                    ]
                    property list<var> right: [
                        {
                            "centered": false,
                            "id": "system_tray",
                            "visible": true
                        },
                        {
                            "centered": false,
                            "id": "dashboard_panel_button",
                            "visible": true
                        },
                        {
                            "centered": false,
                            "id": "power",
                            "visible": true
                        }
                    ]
                }
                property JsonObject tooltips: JsonObject {
                    property bool enableTooltips: true
                    property bool enablePopups: true
                    property bool clickToShow: false
                    property bool compactPopups: false
                    property real popupScaleMultiplier: 1.0
                    property int closeDelay: 50
                    property bool enableColorPickerPopup: true
                    property bool enableBluetoothConnectionPopup: true
                    property bool enableDisplayModesPopup: true
                    // Show the display modes popup when a new screen is plugged in
                    property bool displayModesOnHotplug: true
                    property bool enableKeyboardLayoutTransitionPopup: true
                }
                property JsonObject keyboardLayout: JsonObject {
                    property string colorMode: "primary"
                    property bool secondaryOpposite: false
                    property bool showSecondary: true
                    property bool showPrimary: true
                    property bool swapPrimaryWithSecondary: false
                    property bool uppercaseLayout: false
                }
                property JsonObject battery: JsonObject {
                    property string colorMode: "primary"
                    property bool secondaryOpposite: true
                    property bool showPrimary: true
                    property bool showSecondary: true
                    property bool swapPrimaryWithSecondary: false
                    property bool showPercentageInsideBattery: false
                    property string showPercentage: "off"
                    property bool colorByPowerProfile: true
                }
                property JsonObject systray: JsonObject {
                    property string colorMode: "neutral"
                }
                property JsonObject bluetooth: JsonObject {
                    property string colorMode: "primary"
                }
                property JsonObject power: JsonObject {
                    property string colorMode: "primary"
                }
                property JsonObject policies: JsonObject {
                    property string colorMode: "primary"
                }
                property string bluetoothDevicesLayout: "expressive" // Options: classic, expressive
                property JsonObject sizes: JsonObject {
                    property int height: 40 // horizontal mode
                    property int width: 44 // vertical mode
                }
            }

            property JsonObject battery: JsonObject {
                property string style: "android16"
                property string showPercentage: "off"
                property int low: 20
                property int critical: 5
                property int full: 101
                property bool automaticSuspend: true
                property int suspend: 3
                // Announce the end of charging: a firmware charge limit holding the pack counts,
                // and 101 above means the soft "nearly there" reminder is off.
                property bool notifyCharged: true
                // Grace period between the low-battery warning and the suspend itself; 0 suspends
                // immediately.
                property int suspendWarningSeconds: 30
            }

            property JsonObject calendar: JsonObject {
                property string locale: "en-GB"
                property JsonObject holidays: JsonObject {
                    property bool enable: true
                    // ISO 3166-1 alpha-2, or "auto" to derive it from the system locale
                    property string countryCode: "auto"
                    property bool showInMonthView: true
                }
                property JsonObject timetable: JsonObject {
                    // New events start in khal's configured default calendar
                    // until the user successfully creates one in another
                    // writable calendar. The selected khal collection then
                    // becomes the persistent Timetable default.
                    property string defaultCalendar: ""
                    property list<string> subscriptions: []
                    property JsonObject imports: JsonObject {
                        // Local ICS files and remote read-only subscriptions
                        // are inactive until the user enables calendar sources.
                        property bool enable: false
                        property JsonObject gmailIcs: JsonObject {
                            // Scans only calendar attachments from the active
                            // Gmail account and imports through the same
                            // deduplicating calendar bridge as local files.
                            property bool enable: false
                            property int maxMessages: 25
                            property int scanIntervalMinutes: 60
                        }
                        property JsonObject outlook: JsonObject {
                            // Direct Microsoft Graph calendar mirror. It is
                            // registered as a local read-only khal collection.
                            property bool enable: false
                            property int syncIntervalMinutes: 60
                            property JsonObject icsAttachments: JsonObject {
                                // Mail.Read is used only to locate bounded
                                // .ics/text-calendar files, never mail bodies.
                                property bool enable: false
                                property int maxMessages: 25
                                property int scanIntervalMinutes: 60
                            }
                        }
                    }
                    // ESPN games stay outside khal and are displayed only when
                    // explicitly requested in the Timetable.
                    property bool sportsEvents: false
                    // Number of dates summarized by the month view's agenda rail.
                    property int upcomingHorizonDays: 14
                    // Opt-in: timeline views may replace calendar colours with
                    // a gradient centred on the next event.
                    property bool proximityColorGradient: false
                    property JsonObject moonPhases: JsonObject {
                        // Moon phase badges in the month grid; computed locally,
                        // never fetched from the network.
                        property bool enable: false
                    }
                    property JsonObject birthdays: JsonObject {
                        // Contact birthdays are a read-only projection; they
                        // never create or mutate khal events.
                        property bool enable: false
                    }
                    property JsonObject googleColors: JsonObject {
                        // Per-event colour exists only in the Google Calendar API
                        // (colorId); the synced .ics files carry no COLOR at all.
                        // Opt-in because it needs the authorized account.
                        property bool enable: false
                        // Hours before the colour map is fetched again.
                        property int refreshHours: 6
                    }
                    property JsonObject notifications: JsonObject {
                        property bool enable: true
                        property list<string> offsets: ["-15m"]
                        property bool dailySummary: false
                        property string dailySummaryTime: "08:00"
                        property bool notifyAllDay: true
                        property bool sound: false
                    }
                }
            }

            property JsonObject cheatsheet: JsonObject {
                // Retain only the last selected tab for immediate reopening.
                property bool keepLastTabLoaded: false
                // Use a nerdfont to see the icons
                // 0: 󰖳  | 1: 󰌽 | 2: 󰘳 | 3:  | 4: 󰨡
                // 5:  | 6:  | 7: 󰣇 | 8:  | 9: 
                // 10:  | 11:  | 12:  | 13:  | 14: 󱄛
                property string superKey: ""
                property bool useMacSymbol: false
                property bool splitButtons: false
                property bool useMouseSymbol: false
                property bool useFnSymbol: false
                property bool filterUnbinds: true
                property bool enableGmail: true
                property bool enableTimetable: true
                property bool timetableTodayFirst: false
                property bool enablePeriodicTable: false
                property bool enableAminoAcids: false
                // "five" | "seven" | "four" — side chain classification scheme
                property string aminoAcidScheme: "seven"
                property bool enableCommands: true
                property bool commandsTagsSidebar: false
                property bool enableWorkspaceProfiles: false
                // The typing test also lives in the Overview search. Off by
                // default so the cheatsheet does not gain a tab nobody asked
                // for; the two hosts share one surface either way.
                property bool enableTypingTest: true
                // The Search Tools panel as a full-size page. Off by default:
                // a developer surface most people never open.
                property bool enableDevTools: false
                property JsonObject fontSize: JsonObject {
                    property int key: Appearance.font.pixelSize.smaller
                    property int comment: Appearance.font.pixelSize.smaller
                }
            }

            property JsonObject conflictKiller: JsonObject {
                property bool autoKillNotificationDaemons: false
                property bool autoKillTrays: false
            }

            property JsonObject crosshair: JsonObject {
                // Valorant crosshair format. Use https://www.vcrdb.net/builder
                property string code: "0;P;d;1;0l;10;0o;2;1b;0"
            }

            // Speech typed into the focused window, through voxtype. Off by
            // default: nothing installs voxtype or downloads a speech model on
            // its own, so a fresh install would otherwise ship a dictation key
            // that can only ever answer "not installed".
            property JsonObject dictation: JsonObject {
                property bool enabled: false
                // "fast" (base, 142 MB) or "accurate" (large-v3-turbo, 1.6 GB).
                // A size/latency choice; the language below picks the variant.
                property string quality: "fast"
                // "auto" to let Whisper detect it, a single code ("fr"), or
                // several comma-separated ("en,fr") to detect within that set.
                property string language: "en"
                property bool translateToEnglish: false
                // "paste" (clipboard + Ctrl+V), "type" (synthesised keystrokes),
                // or "clipboard" (copy only). Paste is the default because
                // typing races the receiving app and loses characters in
                // plenty of them; pasting arrives in one piece.
                property string outputMode: "paste"
                property bool pauseMedia: true
                property bool soundFeedback: false
                property int maxDurationSecs: 60
                // CPU threads for transcription. 0 leaves it to the shell,
                // which keeps one core free; Whisper's own default is four
                // regardless of how many the machine has.
                property int threads: 0
                // Primes Whisper with a punctuated sample so it punctuates in
                // kind. Empty picks one to match the language.
                property string punctuationHint: ""
                // Milliseconds between synthesised keystrokes. Voxtype types
                // with no gap by default, which silently loses characters in
                // plenty of apps — the words arrive one letter short.
                property int typeDelayMs: 5
                // Voxtype's own notification carrying the finished text. Worth
                // keeping on: with the text pasted into another window, this is
                // the only place it can be read back if it landed somewhere
                // unexpected.
                property bool notifyOnTranscription: true
                property bool showIndicator: true
                // Keeps the microphone in the bar while idle, as a click target
                // for anyone who would rather not reach for the keybind.
                property bool alwaysShowIndicator: false
                property bool showInIsland: true
            }

            property JsonObject dock: JsonObject {
                property bool enable: false
                property bool smartGrouping: false
                property bool isolateMonitors: false
                property bool showOnlyOnFocusedMonitor: false
                property bool monochromeIcons: false
                property bool dimInactiveIcons: false
                property real iconSpacing: -1
                property real dockRadius: -1
                property real widgetRadius: -1
                property bool enableMagnification: false
                property real magnificationScale: 1.5
                property real magnificationInfluenceRadius: 2.35
                property string magnificationCurve: "cosine"
                property string magnificationMotion: "balanced"
                property bool magnificationDynamicSpacing: true
                property string dockStyle: "floating"
                property bool islandsStyle: false
                property real islandSpacing: 8
                property bool enableAppGroups: true
                property bool enableShapeMask: true
                property string shapeMask: "Circle"
                property real height: 56
                property real hoverRegionHeight: 2
                property bool pinnedOnStartup: false
                property bool enablePreview: true
                property bool enableAppTooltip: false
                property bool hoverToReveal: true
                property bool blockHoverInFullscreen: true
                property bool enableMediaWidget: false
                property bool enableWeatherWidget: false
                property bool enableSportsWidget: false
                property bool enableLivePreviewWidget: false
                property bool enableTasksWidget: false
                // Several widgets sharing one slot, turned with the wheel.
                // Members are drawn only inside the stack while it is on.
                property bool enableWidgetStack: false
                property list<string> widgetStackItems: ["media", "weather", "tasks"]
                property string livePreviewAppId: ""
                property int livePreviewSlots: 2
                property bool livePreviewPaintCursor: false
                property string livePreviewCaptureMode: "visible"
                property bool livePreviewFollowActiveWindow: true
                property bool showPhoneButton: false
                property bool showDividers: true
                property bool showOverviewButton: true
                property bool showPinButton: true
                property bool showTrashButton: false
                property bool showNotificationBadges: true
                property bool switchPresetsOnScroll: true
                // Feedback on an icon: "none", "bounce", "hop", "pulse", "wiggle", "ripple"
                property string launchAnimation: "bounce"
                property string notificationAnimation: "bounce"
                property string position: "auto"
                property list<string> pinnedApps: ["org.kde.dolphin", "kitty"]
                property list<string> ignoredAppRegexes: []
                property list<string> pinnedFiles: []
                // Each entry is { id: string, apps: list<string> } and is
                // rendered as one dock item while app groups are enabled.
                property list<var> appGroups: []
                // Order keys the user has dragged into place by hand. Smart
                // grouping treats these as anchors and auto-arranges only what
                // is left, so turning the feature on no longer makes the dock
                // undo every reorder.
                property list<string> manualOrder: []
                property list<string> order: ["pin", "app:org.kde.dolphin", "app:kitty", "runningApps", "media", "weather", "sports", "livePreview", "phone", "trash", "overview"]
            }

            property JsonObject dockToPanel: JsonObject {
                property int iconSize: 25
                property int buttonSpacing: 2
                property bool enableWorkspaceScroll: false
                property bool alignToWorkspace: false
                property bool enableTooltip: false
                property bool enablePreview: false
                property bool enableMacOsMagnification: false
                property real macOsMagnificationScale: 1.6
                property bool isolateMonitors: false
            }

            property JsonObject hyprland: JsonObject {
                property string defaultHyprlandLayout: "default" // Options: dwindle, monocle, master // It's best to not use scrolling
                // Settings -> Hyprland shows only what a normal desktop needs until this is on.
                // Everything the compositor can be told, and the prose explaining why, is behind it.
                property bool advancedSettings: false
            }

            property JsonObject idle: JsonObject {
                // How long the "Keep awake" inhibitor should survive:
                // "never" (always off at startup), "session" (until logout/reboot), "always"
                property string persistInhibit: "session"
                // Durations offered in the "Keep awake" duration picker, in minutes
                property list<int> quickDurations: [15, 30, 60, 120]
                // Warn before a timed session ends, and let the notification extend it
                property bool notifyOnExpiry: true
                property int warnLeadSec: 60 // 0 disables the warning, keeping only the "ended" notice
                property int extendMinutes: 15 // How much the notification's "Extend" button adds
            }

            property JsonObject interactions: JsonObject {
                property JsonObject scrolling: JsonObject {
                    property bool fasterTouchpadScroll: false // Enable faster scrolling with touchpad
                    property int mouseScrollDeltaThreshold: 120 // delta >= this then it gets detected as mouse scroll rather than touchpad
                    property int mouseScrollFactor: 120
                    property int touchpadScrollFactor: 450
                    // Off: plain lists keep Qt's own mouse wheel unless fasterTouchpadScroll is on.
                    // On: every list moves mouseScrollFactor px per wheel notch.
                    property bool uniformMouseWheel: false
                }
                // Desktop icons: a single click selects, the second click of
                // the pair opens (Windows/KDE semantics). Off = the old
                // click-to-launch. One shared timer in the layer, never one
                // per icon.
                property bool desktopDoubleClick: true
                property JsonObject deadPixelWorkaround: JsonObject { // Hyprland leaves out 1 pixel on the right for interactions
                    property bool enable: false
                }
                property JsonObject touchGestures: JsonObject {
                    property bool enable: true

                    // Visual
                    property bool visualFeedback: true

                    // Device/output
                    property string deviceId: "auto"
                    property string targetMonitor: "auto"
                    property string transform: "auto"
                    // A stylus is also a pointer, so pen gestures drag/resize windows at the
                    // same time. Off unless the device is picked explicitly above.
                    property bool includeStylus: false

                    // Recognition geometry
                    property int edgeWidth: 24
                    property int cornerSize: 72

                    // Recognition thresholds
                    property int minDistance: 44
                    property int commitDistance: 110
                    property int velocityThreshold: 650
                    property int directionTolerance: 35
                    property int cooldownMs: 250

                    // Safety / context
                    property bool disableInFullscreen: false
                    property bool disableInMediaMode: true

                    /**
                     * Whole-hand swipes anywhere on the screen, the touchpad gestures'
                     * counterpart for a touchscreen.
                     *
                     * A tablet has no touchpad, so the compositor's three-finger bindings
                     * are unreachable there — and the two they ship with, scratchpad in and
                     * out, are a desktop window-management idea that a tablet has no use
                     * for. These defaults put the same fingers on the things a phone puts
                     * them on instead.
                     */
                    property JsonObject multiFinger: JsonObject {
                        property bool enable: true
                        property int fingers: 3
                        /// How far the hand travels before the swipe commits.
                        property int distance: 90
                        property string swipeLeft: "workspaceNext"
                        property string swipeRight: "workspacePrev"
                        property string swipeUp: "appDrawer"
                        property string swipeDown: "sidebarRight"
                    }

                    property JsonObject bindings: JsonObject {
                        property string leftEdge: "sidebarLeft"
                        property string rightEdge: "sidebarRight"
                        property string topEdge: "cheatsheet"
                        property string bottomEdge: "overview"
                        property string topLeftCorner: "none"
                        property string topRightCorner: "none"
                        property string bottomLeftCorner: "none"
                        property string bottomRightCorner: "osk"
                    }
                }

                /**
                 * What each touchpad swipe and pinch does (services/TouchpadGestures.qml).
                 *
                 * The compositor runs these, not the shell: the list is written out as a Lua
                 * snapshot that hypr/hyprland/gestures.lua registers, so Hyprland's own
                 * actions (workspace swipe, move, float...) never pass through the shell.
                 *
                 * Entry: { fingers, direction, mods, scale, kind, action, arg, mode }
                 *   direction  swipe | horizontal | vertical | left | right | up | down
                 *              | pinch | pinchin | pinchout
                 *   kind       hyprland  action is one of Hyprland's gesture actions
                 *              lua       action names a handler in gestures.lua
                 *              shell     action is a TouchGestureActionRegistry id, fired once
                 *              tracked   action is a surface that follows the fingers
                 *              command   arg is a shell command
                 *              dispatch  arg is a Lua dispatcher, e.g. hl.dsp.window.close()
                 *
                 * The default list mirrors ii_gestures.defaults in gestures.lua - change both.
                 */
                property JsonObject touchpadGestures: JsonObject {
                    property bool enable: true
                    property list<var> bindings: [
                        { "fingers": 4, "direction": "swipe", "kind": "hyprland", "action": "move" },
                        { "fingers": 4, "direction": "pinch", "kind": "hyprland", "action": "float" },
                        { "fingers": 3, "direction": "horizontal", "kind": "hyprland", "action": "workspace" },
                        { "fingers": 3, "direction": "up", "kind": "lua", "action": "scratchpadUp" },
                        { "fingers": 3, "direction": "down", "kind": "lua", "action": "scratchpadDown" }
                    ]
                    /// Finger travel, in touchpad pixels, that fully opens a tracked surface.
                    property int trackedDistance: 280
                }
            }

            property JsonObject language: JsonObject {
                property string ui: "en_US" // UI language. "auto" for system locale, or specific language code like "zh_CN", "en_US"
                property JsonObject translator: JsonObject {
                    property string engine: "auto" // Run `trans -list-engines` for available engines. auto should use google
                    property string targetLanguage: "auto" // Run `trans -list-all` for available languages
                    property string sourceLanguage: "auto"
                    property string defaultTargetLanguage: "auto"
                    property string defaultSourceLanguage: "auto"
                }
            }

            property JsonObject userProfile: JsonObject {
                property string imageStyle: "initial" // "initial", "expressive", "custom"
                property string imagePath: Directories.userProfileImagePath
                property string customName: ""
                property string customGreeting: ""
                property string customBio: ""
                property string avatarShape: "Cookie9Sided"
                property string avatarColor: "primary"
            }

            property JsonObject launcher: JsonObject {
                property list<string> pinnedApps: ["org.kde.dolphin", "kitty", "cmake-gui"]

                /**
                 * KRunner-style typing: with nothing on screen to type into, the first
                 * printable key opens the launcher and lands in its search field.
                 *
                 * Off by default. While it is armed the shell registers real Hyprland
                 * binds for those keys, so they stop reaching anything else — which is
                 * only acceptable because it arms exclusively when nothing has focus.
                 */
                property JsonObject typeToSearch: JsonObject {
                    property bool enable: false
                    // "emptyWorkspace": only when the focused workspace holds no windows at all.
                    // "noFocusedWindow": also after clicking the desktop next to an open window.
                    property string trigger: "emptyWorkspace"
                    // "letters" | "alphanumeric" | "all"
                    property string keys: "all"
                }
            }

            property JsonObject light: JsonObject {
                property JsonObject darkMode: JsonObject {
                    property bool automatic: false
                    property string from: "18:00" // Format: "HH:mm", 24-hour time
                    property string to: "06:00"   // Format: "HH:mm", 24-hour time
                }
                property JsonObject night: JsonObject {
                    property bool automatic: true
                    property string from: "19:00" // Format: "HH:mm", 24-hour time
                    property string to: "06:30"   // Format: "HH:mm", 24-hour time
                    property int colorTemperature: 5000
                    // How long a manual Night Light toggle (and the gamma level) should survive:
                    // "never" (always off at startup), "session" (until logout/reboot), "always".
                    // With automatic mode on, a restored toggle still expires at the next start/end time.
                    property string persistManual: "always"
                }
                property JsonObject gamma: JsonObject {
                    // Below the backlight minimum, brightness keys and the combined
                    // gamma/brightness slider keep dimming by lowering gamma.
                    // Turn off to drive the backlight only and leave gamma alone.
                    property bool dimBelowMinimum: true
                }
                property JsonObject antiFlashbang: JsonObject {
                    property bool enable: false
                }
                property JsonObject keyboardBacklight: JsonObject {
                    // Switch the keyboard backlight off after a period without keyboard
                    // or pointer input, then restore the previous level on the next input.
                    // Off by default: it changes hardware state without being asked to.
                    property bool autoOff: false
                    property int timeout: 15 // Seconds of no input
                }
            }

            property JsonObject lock: JsonObject {
                property bool useHyprlock: false
                property bool launchOnStartup: false
                property JsonObject blur: JsonObject {
                    property bool enable: true
                    property real radius: 45
                    property real extraZoom: 1
                }
                property JsonObject desaturate: JsonObject {
                    property bool enable: true
                    property real amount: 0.25
                }
                property JsonObject colorWash: JsonObject {
                    property bool enable: false
                    property real amount: 0.5
                }
                property JsonObject vignette: JsonObject {
                    property bool enable: true
                    property real amount: 0.3
                }

                property real centerSpacing: 20 // spacing between multiple centered widgets
                property string centerAlignment: "horizontal" // "vertical" | "horizontal"
                property bool showLockedText: true
                property JsonObject security: JsonObject {
                    property bool unlockKeyring: true
                    property bool requirePasswordToPower: false
                    property JsonObject fingerprint: JsonObject {
                        // Default on so an existing setup with enrolled prints keeps
                        // unlocking by finger exactly as it did before the toggle existed.
                        property bool enable: true
                        property bool showIndicator: true
                        // [{ finger: "right-index-finger", label: "Trigger finger" }]
                        property list<var> labels: []
                    }
                }
                /**
                 * A keyboard drawn inside the lock surface itself.
                 *
                 * The regular on-screen keyboard is a layer-shell surface and the session
                 * lock protocol covers every layer, so it cannot appear here. Without this,
                 * a device with no physical keyboard cannot be unlocked at all — which is
                 * why "auto" turns it on for touch-first families rather than leaving it to
                 * be discovered.
                 */
                property JsonObject touchKeyboard: JsonObject {
                    // "auto" | "always" | "never". Auto = on in a touch-first family.
                    property string show: "auto"
                    // "text" (qwerty) or "pin" (numeric pad), for a numeric password.
                    property string mode: "text"
                }
                property bool materialShapeChars: true
                property bool rippleEffect: true
                property bool nowPlaying: true
                property bool sports: true
                // The islands' draw order; see lock_islands.js for the defaults these must match.
                property JsonObject islands: JsonObject {
                    property list<string> main: ["fingerprint", "password", "confirm"]
                    property list<string> left: ["battery", "capsLock", "alarm", "weather", "keyboardLayout", "keepAwake", "mode"]
                    property list<string> right: ["sleep", "power", "reboot"]
                    // What the toolbars have been asked NOT to draw. A hide
                    // list rather than a per-item switch: the islands' draw
                    // order is already a list of the same ids, so one more
                    // list is the whole of "take that one off" - which is
                    // what Edit Mode's remove badge on the lock screen writes,
                    // the same gesture the bar and the dock already had.
                    property list<string> hidden: []
                }
                property bool showAlarm: true
                property bool showWeather: true
                property JsonObject zoomAnimation: JsonObject {
                    property bool enabled: true
                    // "default" | "gnome" | "material-shape". The two named designs
                    // reuse the overview background's own animations on lock; the
                    // default keeps the legacy centre-and-zoom.
                    property string style: "default"
                }
                property JsonObject notifications: JsonObject {
                    property bool enable: true // Off by default: showing notifications on the lock screen is a privacy trade-off
                    property string position: "top_right" // "top_left" | "top_right" | "bottom_left" | "bottom_right"
                    property string privacy: "redacted" // "full" | "redacted" | "countOnly"
                    property bool onlySinceLock: true // Only show notifications that arrived while locked
                    property int maxShown: 5
                    property int zoomPercent: 100 // 50-200, step 10
                    property string defaultPolicy: "show" // "show" | "hide" — apps without an explicit rule
                    property list<string> alwaysShowApps: [] // App names, case-insensitive match
                    property list<string> neverShowApps: []
                    property JsonObject filters: JsonObject {
                        property bool skipTransient: true
                        property bool skipLowUrgency: false
                        property string criticalOverride: "full" // "full" | "none" — critical notifications bypass privacy redaction
                    }
                }
            }

            property JsonObject media: JsonObject {
                property bool filterDuplicatePlayers: true
                property string priorityPlayer: ""
                property bool dynamicAlbumColors: true
                property bool albumArtBlurAnimations: true
            }

            property JsonObject networking: JsonObject {
                property string userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36"
            }

            property JsonObject notifications: JsonObject {
                property int timeout: 7000
                property string position: "top_right"
                property int zoomPercent: 100 // 50-200, step 10
                property bool autoDndFullscreen: true
                property JsonObject monitor: JsonObject {
                    property bool enable: false
                    property string name: "" // Output to show notifications on, like "eDP-1"; the window-manager seam lists them
                }
            }

            property JsonObject oledSaver: JsonObject {
                property bool enable: true
                property bool antiBurnIn: false // periodically shift widgets to prevent OLED burn-in
                property int cursorHideDelay: 5 // seconds of no mouse movement before the cursor hides again
                property int lockTimeout: 10 // minutes without input on the lock screen before it turns into the Always On Display; 0 = never
            }

            property JsonObject osd: JsonObject {
                property bool enable: true
                property string style: "default"
                property string position: "right"
                property int height: 500
                property int timeout: 3000
                property bool showValues: true
                property bool hideWhenFullscreen: true

                // Per-indicator popups. Turning one off keeps that OSD from showing
                // on its own; the master `enable` switch above still wins over all.
                property JsonObject indicators: JsonObject {
                    property bool volume: true
                    property bool brightness: true
                    property bool keyboardBrightness: true
                    property bool playerVolume: true
                    property bool gamma: true
                    // On/off pills. Drawn by the Dynamic Island and the connected OSD only.
                    property bool microphone: true
                    property bool capsLock: true
                    property bool numLock: true
                    // Pills sent by scripts: `qs -c ii ipc call osd pill <icon> <label> <on|off|"">`
                    property bool pills: true
                }

                // Backlight changes made outside the shell (keys bound straight to brightnessctl,
                // keyd, an ambient-light daemon) raise the brightness OSD only when a key press
                // was reported with them (`ipc call brightness keyPressed`). Turn on when
                // something adjusts the backlight on its own and the OSD should ignore it.
                // Also gates the keyboard backlight OSD (`ipc call keyboardBacklight keyPressed`;
                // keys reported by the driver through brightness_hw_changed always show).
                property bool brightnessKeysOnly: false

                property JsonObject material: JsonObject {
                    property bool rotateShape: false
                    property bool minimal: false
                    property bool shapedValues: true
                    property bool circledShapes: true
                }

                property JsonObject tuner: JsonObject {
                    // On/off OSDs (Caps Lock, the microphone, script pills) slide along the ruler
                    // like the sliders. Off: just OFF and ON, the active one in the middle.
                    property bool toggleRuler: false
                }
            }

            property JsonObject osk: JsonObject {
                // "deck" is the full-bleed keyboard, "classic" the original floating one.
                property string style: "deck"
                // "auto" follows whatever layout Hyprland reports; any other value pins a
                // layout by name ("French", "German", ...).
                property string layout: "auto"
                // Share of the screen height the deck takes. Key size falls out of it, so this
                // is the only size control the deck needs.
                property int heightPercent: 35
                // The small shift and AltGr glyphs in the corners of a deck key.
                property bool secondaryGlyphs: true
                property bool pinnedOnStartup: false
                /**
                 * Pushes the two halves apart with an empty middle, for typing with thumbs
                 * while holding the device. On a tablet held in two hands the middle columns
                 * of a full-width keyboard are out of reach of either thumb.
                 */
                property bool split: false

                // Raises the keyboard when a text field is focused by finger or pen.
                // Requires the osk_autoshow helper (see scripts/osk/README.md).
                //
                // On by default, and safe to be: the helper only reports focus that a touch
                // or pen caused, so a machine with neither never raises anything, and the
                // service does not even spawn the helper unless the binary is there. Off by
                // default meant a tablet — where tapping a text field is the whole way in —
                // did nothing until the user found a switch they had no reason to look for.
                property JsonObject autoShow: JsonObject {
                    property bool enable: true
                    property bool allowTouch: true
                    property bool allowPen: true
                    /**
                     * Whether a left click also raises the keyboard.
                     *
                     * Off, and it should stay off on any device that has a touchscreen:
                     * someone with a mouse has a keyboard, and a keyboard drawn over
                     * their text field is a nuisance rather than a feature.
                     *
                     * It exists because the alternative is a feature nobody can try. On a
                     * machine with no touch panel every other switch here is inert, and
                     * "I turned it on and nothing ever happened" is indistinguishable
                     * from a bug. This makes the pipeline demonstrable on the hardware
                     * people develop on.
                     */
                    property bool allowMouse: false
                    // How long after a touch a text field may claim focus and still count as touch-driven.
                    property int touchWindowMs: 1200
                    property bool hideOnPhysicalKey: true
                    property bool hideOnTouchOutside: true
                }
            }

            /**
             * The notes app.
             *
             * Deliberately small. Everything about how notes look and behave — paper,
             * typography, ink, autosave, cloud, AI — lives inside the app itself, where
             * the person changing it can see what they are changing. What belongs out
             * here is only what has to be answerable before the app exists: whether it
             * exists at all.
             */
            property JsonObject notes: JsonObject {
                property bool enable: true
                /**
                 * The defaults a new note starts from, and the one timing the editor has.
                 *
                 * These live here rather than inside the app's own sheet only because they
                 * are preferences that make sense in a preset. Every one of them replaces
                 * a number that used to be written into the code — a settings page whose
                 * switches configure nothing is worse than no settings page, which is what
                 * the first version of that sheet was.
                 */
                property string defaultPaper: "plain"
                property int paperStrength: 50
                property int autosaveDelay: 400
            }

            /**
             * EasyEffects: the app, and how the quick switchers (island bubble, quick
             * toggle, bar widget, keybinds) behave. Edited from the app's settings page.
             */
            property JsonObject easyEffects: JsonObject {
                property bool appEnable: true
                property string startTab: "last" // "last" | "presets" | "effects" | "devices"
                // What the quick switchers cycle: "device" keeps to the family of the output
                // device's default preset ("A50 · Music" -> every "A50 · …"), "all" is every preset.
                property string cycleScope: "device"
                property bool osdOnSwitch: true
                // Reapply the output device's default preset when EasyEffects starts (it
                // skips it on Pro Audio sinks: wwmm/easyeffects#5275).
                property bool applyDeviceDefaultOnStart: true
                // Load the new device's default preset whenever the default output or
                // input device changes.
                property bool applyDeviceDefaultOnSwitch: true
                // Knob changes in the editor are heard at once, before "Save to preset".
                property bool liveApply: true
                // The preset hero on the app's Presets page: the same parts as the widget's,
                // each one switchable (Settings > Preset card).
                property JsonObject hero: JsonObject {
                    property bool topography: true
                    property bool topographyReactive: true
                    property int topographyStrength: 100
                    property string art: "shape" // "shape" | "curve" | "none"
                    property bool showDevice: true
                    property bool showState: true
                    property bool showDefault: true
                    property bool showWave: true
                    property bool showCaption: true
                    property bool showName: true
                    property bool showEffects: true
                    property bool showButtons: true
                }
                // The desktop widget (modules/ii/background/widgets/system/EasyEffectsPresetWidget.qml),
                // both its shapes. Each part of the hero can be switched off.
                property JsonObject widget: JsonObject {
                    property string pipeline: "output" // "output" | "input"
                    property bool topography: true
                    // The music (cava) drives the texture; off, it only drifts.
                    property bool topographyReactive: true
                    // Hold the texture still while a program is open on the workspace shown.
                    property bool pauseOnWindows: true
                    // Opacity of the lines, as a percentage of the usual.
                    property int topographyStrength: 100
                    property string art: "shape" // "shape" | "curve" | "none"
                    property bool showDevice: true
                    property bool showState: true
                    property bool showDefault: true
                    property bool showWave: true
                    property bool showCaption: true
                    property bool showName: true
                    property bool showEffects: true
                    property bool showButtons: true
                }
            }

            /**
             * The clock app. Only whether it exists lives out here; every clock option is
             * edited from the app's own settings page.
             */
            property JsonObject clockApp: JsonObject {
                property bool enable: true
                property string startTab: "last" // "last" | "alarms" | "worldClock" | "timer" | "stopwatch" | "pomodoro" | "bedtime" | "reminders"
                property bool showTimetableEvents: true
                property int timetableLeadMinutes: 15
                property int timetableLookaheadDays: 2
                property bool analogWorldClock: true
                property bool showSecondsInApp: true
                property JsonObject bedtime: JsonObject {
                    property bool enable: false
                    property string time: "23:30"
                    // Nights it applies to, indexed like Date.getDay() of the evening.
                    property list<var> days: [true, true, true, true, true, true, true]
                    property int windDownMinutes: 30
                    // 0 turns the "still up" reminders off.
                    property int remindEveryMinutes: 15
                    // A night ends at this hour of the next morning.
                    property int nightEndsHour: 5
                }
                // Mirror the phone's next alarm over ADB (read-only).
                property JsonObject phoneAlarm: JsonObject {
                    property bool enable: true
                    property int pollMinutes: 10
                    // Notification app names that mean "the phone's alarm is ringing".
                    property list<string> clockApps: ["Clock", "Relógio", "Alarm", "Alarme", "com.google.android.deskclock", "com.sec.android.app.clockpackage"]
                    // Packages whose alarm-clock alarms aren't alarms (automations that borrow the API).
                    property list<string> ignorePackages: ["com.samsung.android.app.routines"]
                }
                // The Reminders tab (Samsung Reminder's model). The list itself lives in
                // $STATE/user/reminders.json; these are its preferences.
                property JsonObject reminders: JsonObject {
                    // "light": a notification · "medium": a full-screen alert and one
                    // chime · "strong": a full-screen alert that keeps ringing.
                    property string defaultAlert: "light"
                    // When an all-day reminder alerts on its day.
                    property string allDayTime: "09:00"
                    property int snoozeMinutes: 5
                    // A ringing "strong" reminder goes quiet after this long.
                    property int autoSilenceMinutes: 1
                    // An alert this late still fires as itself; older ones come in as missed.
                    property int catchUpMinutes: 10
                    property bool wakeFromSuspend: true
                    // The full-screen alert's backdrop: "" = the theme, else #rrggbb.
                    property string alertBackgroundColor: ""
                    property string alertBackgroundImage: ""
                    property bool alertSound: true
                    // Days a completed reminder is kept (0 = forever) and days in the bin.
                    property int autoDeleteCompletedDays: 0
                    property int trashDays: 30
                    // "alertTime" | "modified" | "created" | "name" | "category"
                    property string sortBy: "alertTime"
                    property bool pinImportant: true
                    property bool showCompleted: false
                    // "card" | "list"
                    property string view: "card"
                    property bool categoriesExpanded: true
                    property bool showTemplates: true
                    property string defaultCategory: "default"
                    property bool showInTimetable: true
                    // Two-way sync with Microsoft To Do — the bridge Samsung Reminder itself
                    // offers — through the Outlook sign-in.
                    property JsonObject todoSync: JsonObject {
                        property bool enable: false
                        property int intervalMinutes: 15
                    }
                }
            }

            property JsonObject overlay: JsonObject {
                property bool openingZoomAnimation: true
                property bool darkenScreen: true
                property real clickthroughOpacity: 0.8
                property list<string> buttons: ["crosshair", "recorder", "media", "volumeMixer", "resources", "discordVoice", "perfMonitor"]
                property JsonObject floatingImage: JsonObject {
                    property string imageSource: "https://media.tenor.com/H5U5bJzj3oAAAAAi/kukuru.gif" //media.tenor.com/H5U5bJzj3oAAAAAi/kukuru.gif"
                    property real scale: 0.5
                }
                property JsonObject notes: JsonObject {
                    property bool showTabs: true
                    property bool allowEditingIcon: true
                }
                property JsonObject media: JsonObject {
                    property int backgroundOpacityPercentage: 100
                    property bool useGradientMask: true
                    property bool showSlider: true
                    property int lyricSize: Appearance.font.pixelSize.larger
                }
                property JsonObject discordVoice: JsonObject {
                    property int maxAvatars: 8
                    property int avatarSize: 52
                    property string participantBackground: "name"
                    property real participantBackgroundOpacity: 0.72
                    property string layoutMode: "column"
                    property bool blurEnabled: true
                    property bool autoResize: true
                    property bool speakingPulseContinuous: true
                }
                // Performance HUD (RivaTuner style). Frame rate comes from MangoHud's logger.
                property JsonObject perfMonitor: JsonObject {
                    property string style: "bars" // bars | graph | text
                    property string palette: "accent" // accent | container | mono
                    property bool fpsOnly: false
                    property string title: ""
                    property string footerText: ""
                    // Names shown instead of the detected models; empty = detected
                    property string cpuName: ""
                    property string gpuName: ""
                    // PCI slot of the monitored GPU, or "auto" (the discrete one)
                    property string gpuDevice: "auto"
                    property bool uppercase: true
                    property bool showIcons: false
                    property real scale: 1.0
                    property int width: 280
                    property real backgroundOpacity: 0.55
                    // Compositor blur behind the card (Hyprland's overlay layer)
                    property bool blur: false
                    // Corner the HUD snaps to: topLeft | topRight | bottomLeft | bottomRight | none (free)
                    property string anchor: "topLeft"
                    property int snapMargin: 16
                    property int updateInterval: 1000
                    // Seconds of frames behind the average and the 1% / 0.1% lows
                    property int statsWindow: 30
                    property int fpsTarget: 60
                    property bool colorCodeFps: true
                    property int hotTemp: 85
                    property bool showFps: true
                    property bool showFpsAverage: true
                    property bool showFpsLow1: true
                    property bool showFpsLow01: false
                    property bool showFrametime: true
                    property bool showFrametimeGraph: true
                    property bool showCpu: true
                    property bool showCpuUsage: true
                    property bool showCpuTemp: true
                    property bool showCpuClock: true
                    property bool showCpuPower: true
                    property bool showRam: true
                    property bool showSwap: false
                    property bool showGpu: true
                    property bool showGpuUsage: true
                    property bool showGpuTemp: true
                    property bool showGpuClock: true
                    property bool showGpuMemClock: false
                    property bool showGpuPower: true
                    property bool showGpuFan: false
                    property bool showVram: true
                    property bool showBattery: true
                    property bool showBatteryEnergy: true
                    property bool showBatteryPower: false
                    property bool showBatteryTime: false
                    property bool showDetails: true
                    property bool showProcess: true
                    property bool showResolution: true
                    property bool showDriver: true
                    property bool showSessionTime: true
                    property bool showClock: false
                    property bool showStutter: true
                    property bool showFpsCap: true
                    property bool showPowerProfile: true
                    property bool showGameMode: true
                    // Network: rates of the default route and a ping
                    property bool showNetwork: false
                    property bool showNetJitter: false
                    property bool showNetLoss: false
                    property bool showNetDown: true
                    property bool showNetUp: true
                    // Host to ping; empty = the default gateway (nothing leaves the LAN)
                    property string pingTarget: ""
                    // Numbers in the monospace font so they do not jitter as digits change
                    property bool monoNumbers: true
                    // Written into MangoHud.conf by "Set up MangoHud logging"
                    property int mangohudLogInterval: 100
                    property bool mangohudHideHud: true
                }
            }

            // Alt+Tab. Drawn on the Dynamic Island when it is on, as a floating panel otherwise.
            property JsonObject windowSwitcher: JsonObject {
                property bool enable: true
                // Windows on every workspace (special ones included), or only those on screen.
                property bool includeOtherWorkspaces: true
                // Only the windows on the monitor Alt+Tab is pressed on.
                property bool currentMonitorOnly: false
                // Live window previews on the island's cover flow and the floating panel.
                property bool showThumbnails: true
                // Holding one selection this long peeks at the window where it is; 0 never peeks.
                property int peekDelayMs: 600
                // The peek shows the window's whole workspace (its other windows, the bar kept),
                // not the window alone over the wallpaper.
                property bool peekWholeWorkspace: false
                // A one-line key reminder under the switcher.
                property bool showKeyHints: true
                // Alt+letter on the desktop opens the switcher already searching. Off: typing
                // searches only once Alt+Tab has opened it.
                property bool searchAnywhere: false
            }

            property JsonObject overview: JsonObject {
                property bool enable: true
                // Reuse the Tablet Family's app drawer as the ii family's primary
                // Overview surface. Search panels still use their shared content host.
                property bool useAppDrawer: false
                // Shows a GNOME-style workspace overview strip with live screencopies
                // between the search bar and the app grid.
                property bool showWorkspacesOverview: false
                property bool showWindowPreviews: true
                property bool enableManualScale: false
                property real autoScaleFactor: 1.0 // Multiplier for automatic scaling (0.5 to 1.5)
                property real scale: 0.18 // Relative to screen size (used when enableManualScale is true)
                property string animationStyle: "zoom" // Options: "bounce", "smooth", "zoom", "none"
                property bool enableCascadeAnimation: true
                property real rows: 2
                property real columns: 5
                property bool orderRightLeft: false
                property bool orderBottomUp: false
                property bool showIcons: true
                property bool centerIcons: true
                property bool showOpeningAnimation: true
                property bool useWorkspaceMap: false

                property JsonObject scrollingStyle: JsonObject {
                    property string zoomStyle: "out"        // Options: in, out
                }
            }

            property JsonObject regionSelector: JsonObject {
                property bool showOnlyOnFocusedMonitor: false
                property bool enableOverlay: true
                // Shows a system notification when a screenshot is copied to the clipboard (default off).
                property bool copyNotification: false
                property JsonObject targetRegions: JsonObject {
                    property bool windows: true
                    property bool layers: true
                    property bool content: true
                    property bool showLabel: false
                    property real opacity: 0.3
                    property real contentRegionOpacity: 0.8
                    property int selectionPadding: 5
                }
                property JsonObject rect: JsonObject {
                    property bool showAimLines: false
                }
                property JsonObject circle: JsonObject {
                    property int strokeWidth: 6
                    property int padding: 10
                }
                property JsonObject annotation: JsonObject {
                    property bool useSatty: false
                    property bool enableInlineEditor: true
                    property real fillOpacity: 0.25
                    property real highlighterOpacity: 0.4
                    property int badgeStartNumber: 1
                    // Pixelation coarseness for the blur tool (source-px divisor):
                    // bigger = chunkier blocks. Independent of line thickness.
                    property int blurStrength: 24
                }
            }

            property JsonObject resources: JsonObject {
                property int updateInterval: 3000
                property int historyLength: 60
                // New keys (zero-cost on AMD; only NVIDIA/Intel invoke nvidia-smi one-shot)
                property int diskInterval: 30000
                property int gpuInterval: 3000
                // Which GPU to monitor on hybrid iGPU+dGPU systems.
                // "auto" (NVIDIA → AMD → Intel priority) | "nvidia" | "amd" | "intel"
                property string gpuPreference: "auto"
                // Mount point / path to monitor with df for disk usage.
                // Default: "/" (root filesystem). Change to e.g. "/home" or "/mnt/data".
                property string diskMount: "/"
                // Toggle for Docker section popup. When false, all Docker
                // polls (docker stats, docker ps) are suppressed and the
                // events stream is not subscribed.
                property bool enableDocker: true
            }

            property JsonObject lyricsService: JsonObject {
                property bool enable: true
                property bool enableGenius: true
                property bool enableLrclib: true
                property bool enableYtmusic: true // requires ytmusicapi in venv
                // "auto" | "lrclib" | "ytmusic" | "genius"
                // auto = lrclib synced → lrclib plain → ytmusic → genius
                property string lyricsProvider: "auto"
            }

            property JsonObject tray: JsonObject {
                property bool monochromeIcons: true
                property bool showItemId: false
                property bool invertPinnedItems: false // Makes the below a whitelist for the tray and blacklist for the pinned area
                property list<var> pinnedItems: ["Fcitx"]
                property bool filterPassive: false
            }


            property JsonObject update: JsonObject {
                // Whether the Settings "Update" button also overlays the fork's
                // dots/.config/hypr onto ~/.config/hypr (passes --hypr/--no-hypr
                // to setup-ii-p3drovfx.sh). See AboutConfig.qml.
                property bool replaceHyprConfig: true
                // How often ShellUpdates probes the fork's remote for new
                // commits: "disabled" | "10min" | "hourly" | "daily" | "weekly".
                property string autoCheckInterval: "daily"
                // Epoch ms of the last completed automatic check. Persisted so a
                // daily/weekly period is not restarted from zero on every shell
                // restart. Written by ShellUpdates, not by the settings UI.
                property real lastAutoCheck: 0
                // Ask the AI tab's current model for a plain-language summary of
                // the new commits after a check finds at least aiSummaryMinCommits
                // of them. Off by default: it spends a request on the user's key.
                // A manual "Summarize" button works regardless of this switch.
                property bool aiSummary: false
                property int aiSummaryMinCommits: 10
            }

            property JsonObject musicRecognition: JsonObject {
                property int timeout: 16
                property int interval: 4
            }

            property JsonObject search: JsonObject {
                property bool enableSystemControls: true
                property bool enableMathPreview: true
                property bool showSettings: false
                property bool alwaysListApps: false
                property int nonAppResultDelay: 30
                property string engineBaseUrl: "https://www.google.com/search?q=" //www.google.com/search?q="
                property list<string> excludedSites: ["quora.com", "facebook.com"]
                property bool sloppy: false
                property bool levenshtein: false
                property bool frecency: true
                // fuzzysort matches any subsequence, so with no floor "file"
                // reaches "OpenJDK 25 for x86_64 Monitoring & Management
                // Console". Measured on this machine, the genuine hits for
                // "file" score 0.82–0.94 and the noise 0.25–0.30, so 0.35 cuts
                // the tail without emptying short queries.
                property real fuzzyThreshold: 0.35
                // And anything below this fraction of the best hit is noise
                // *next to that hit*, whatever its absolute score.
                property real fuzzyRelativeCutoff: 0.35
                property JsonObject bestMatch: JsonObject {
                    // Render the top result as one prominent row with its own
                    // actions on it, and the rest as a single uniform list.
                    // A launcher's promise is that the first thing on screen is
                    // the thing you meant; showing it at the same weight as the
                    // nine rows below makes you verify that promise every time.
                    property bool enable: false
                    // Actions shown inline on the row. Four fit without turning
                    // it into a menu; the action panel still holds every one.
                    property int secondaryActions: 4
                    // Group captions are what the prominent row replaces: with
                    // one answer at the top, the rest reads better as one list.
                    property bool uniformList: true
                }
                property JsonObject typoTolerance: JsonObject {
                    // Myers bit-parallel edit distance as a last tier: it runs
                    // only when the exact and layout-corrected passes found
                    // nothing, so a typo returns the app instead of an empty
                    // list. Off by default — it changes what a miss looks like.
                    property bool enable: true
                    // 0.30 lands every case from the upstream PR's table on its
                    // target while still rejecting a query that is simply not a
                    // typo of anything. Lower widens the net.
                    property real threshold: 0.30
                    property int maxResults: 8
                    // Query typed with the wrong keyboard layout active, mapped
                    // back through the physical layout — and Cyrillic queries
                    // transliterated to Latin. Unlike the typo tier these run
                    // on every query, so they also fix a *partly* wrong query.
                    property bool keyboardLayouts: true
                }
                property JsonObject browserSites: JsonObject {
                    property bool enable: true
                    property string profilePath: ""
                    property int maxIndexedSites: 300
                    property int maxResults: 6
                    property bool includeHistory: true
                    property bool useLocalFavicons: true
                    property bool allowRemoteFavicons: false
                    property int refreshMinutes: 10
                }
                property list<var> aliases: []
                // Priority of the result groups, top to bottom. Removing an
                // entry hides that group's results, so this list is both the
                // order and the on/off switch. Reordered from Settings; the
                // catalogue of ids lives in SearchResultSectionRegistry.
                property list<var> sectionOrder: [
                    {
                        "id": "suggested"
                    },
                    {
                        "id": "aliases"
                    },
                    {
                        "id": "media"
                    },
                    {
                        "id": "best"
                    },
                    {
                        "id": "apps"
                    },
                    {
                        "id": "tools"
                    },
                    {
                        "id": "sites"
                    },
                    {
                        "id": "controls"
                    },
                    {
                        "id": "actions"
                    },
                    {
                        "id": "quicklinks"
                    },
                    {
                        "id": "textSnippets"
                    },
                    {
                        "id": "other"
                    },
                    {
                        "id": "settings"
                    },
                    {
                        "id": "files"
                    },
                    {
                        "id": "continue"
                    }
                ]
                property string fileSearchDirectory: "/home"
                // Image and vector hits draw themselves in the row's icon slot.
                // Turning this on drops the thumbnail and leaves the file kind's
                // symbol there instead — hiding the picture outright rather than
                // blurring 32 pixels of it.
                property bool blurFileSearchResultPreviews: false
                property JsonObject fileSearch: JsonObject {
                    // Show files and folders from the indexed directory for a
                    // plain query, no prefix. On by default: a name typed into
                    // Search that finds no file reads as a broken search. The
                    // walk stays debounced, threaded and off the keystroke path.
                    property bool inlineResults: true
                    // One or two letters match a large share of a home directory.
                    // The walk is only worth starting once the query narrows.
                    property int minimumQueryLength: 3
                    // This caps only Search's inline preview. Every matching
                    // path is still ranked and can be opened in File Browser.
                    property int maxResults: 8
                    // Depth limits the complete traversal; 0 leaves it
                    // unbounded inside the configured search directory.
                    property int maxDepth: 0
                    // fd saturates every core by default. On a 16-core machine
                    // that measured 1351% CPU and 0.70s of CPU time for one
                    // rare-query walk, against 388% and 0.28s at four threads —
                    // a third of the work for 50ms more wall time. A background
                    // helper should not take the machine hostage.
                    property int threads: 4
                    // fd skips dotfiles by default. Including them covers the
                    // whole directory, at the cost of walking every cache and
                    // state folder a home directory accumulates.
                    property bool includeHidden: false
                    property list<string> excludedDirectories: ["node_modules", ".git", ".cache", ".venv", "__pycache__", ".cargo", ".rustup", ".npm", ".local/share/Trash"]
                }
                property JsonObject fileContent: JsonObject {
                    // ripgrep stops once this many files matched; the walk
                    // of a whole home directory never has to finish.
                    property int maxResults: 40
                    property int maxFileSizeMb: 8
                    property int minimumQueryLength: 3
                }
                property JsonObject calculator: JsonObject {
                    // A currency conversion refreshes qalc's rates at most
                    // once a day; qalc otherwise answers from months-old ones.
                    property bool updateExchangeRates: true
                    property int historyMaxItems: 30
                }
                property JsonObject fileBrowser: JsonObject {
                    // The explorer needs more room than the result-oriented
                    // panels: its file list, preview and metadata are visible
                    // at the same time. These values remain user-adjustable.
                    property int panelWidth: 1120
                    property int panelBodyHeight: 620
                }
                property JsonObject prefix: JsonObject {
                    property bool showDefaultActionsWithoutPrefix: true
                    property string action: "/"
                    property string app: ">"
                    property string bluetooth: "<"
                    property string clipboard: ";"
                    property string fileSearch: ","
                    property string fileContent: "'"
                    property string emojis: ":"
                    property string math: "="
                    property string shellCommand: "$"
                    property string webSearch: "?"
                    property string windowSearch: "#"
                    property string fileBrowser: "~"
                    property string translator: "@"
                    property string mediaDownloader: "!"
                    property string materialSymbols: "*"
                    property string typingTest: "^"
                    property string speedTest: "%"
                    property string ai: "&"
                }
                property JsonObject typingTest: JsonObject {
                    property string language: "english_1k"
                    // "standard" uses the curated ~1k word lists; "extended" loads
                    // the full Monkeytype sets (5k–10k words) for variety.
                    property string wordlistSize: "standard"
                    property string mode: "time"
                    // Zen without a target is free typing; guided zen keeps the
                    // generated words but drops both limits, so the test only
                    // ends when the user says so.
                    property bool zenGuided: false
                    property int time: 30
                    property int words: 50
                    property bool punctuation: false
                    property bool numbers: false
                    property bool showLiveWpm: false
                    property bool showLiveAccuracy: false
                    property bool smoothCaret: true
                    // Typing surface. fontSize is the target text size in px:
                    // the test is the hero of the panel, so it does not follow
                    // the launcher's body scale.
                    property int fontSize: 26
                    property int visibleLines: 3
                    property string caretStyle: "line" // line, block, underline, off
                    // Highlight everything but the current word at reduced
                    // emphasis, the way Monkeytype's word highlight does.
                    property bool highlightCurrentWord: false
                    property bool blindMode: false
                    // Tab restarts the test, as on Monkeytype. Off by default
                    // because Tab also walks the launcher's controls.
                    property bool quickRestart: false
                    // Finish a words/quote test on the last word without
                    // needing a trailing space.
                    property bool finishOnLastWord: true
                    property JsonObject keyboard: JsonObject {
                        property bool enable: true
                        // qwerty, qwertz, azerty, dvorak, colemak, or "vial" for the
                        // keyboard actually plugged in, read from its own firmware.
                        property string layout: "qwerty"
                        property bool highlightNextKey: true
                        property bool fingerGuide: false
                        // Physical assignments by board UID + matrix position,
                        // shared across layers. Explicit lists keep JsonAdapter safe.
                        property list<string> fingerAssignments: []
                    }
                    property JsonObject sounds: JsonObject {
                        property bool enable: true
                        // Monkeytype pack ids, catalogued in
                        // assets/typing/sounds-manifest.json.
                        property string theme: "click1"
                        property string errorTheme: "error1"
                        property int volume: 55
                        property bool errorSound: true
                    }
                    property JsonObject history: JsonObject {
                        property bool enable: true
                        property int maxEntries: 100
                    }
                }
                property JsonObject ai: JsonObject {
                    // How the AI chat is triggered from the search:
                    // "prefix" — only via the configured prefix
                    // "suggest" — adds an "Ask AI" fallback row to the results
                    // "auto" — switches to the AI chat when the query matches nothing
                    property string trigger: "suggest"
                }
                property JsonObject speedTest: JsonObject {
                    property int duration: 10
                    property string mode: "both"
                    property string unit: "mbps"
                }
                // Search surfaces consume this module contract through
                // SearchPanelRegistry. A panel cannot accidentally remain in
                // aliases or prefix routing after its feature is disabled.
                property JsonObject modules: JsonObject {
                    property bool clipboard: true
                    property bool bluetooth: true
                    property bool translator: true
                    property bool mediaDownloader: true
                    property bool materialSymbols: true
                    property JsonObject typingTest: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject speedTest: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject emojis: JsonObject {
                        property bool enable: true
                        property string skinTone: "none"
                        property int gridColumns: 8
                        property bool showRecents: true
                        property string defaultCategory: "all"
                    }
                    property bool windowSearch: true
                    property bool fileBrowser: true
                    property bool fileSearch: true
                    // `'` searches inside files with ripgrep.
                    property bool fileContent: true
                    // KDE Connect / LocalSend actions on file rows.
                    property JsonObject phoneShare: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject gifs: JsonObject {
                        property bool enable: true
                        // KLIPY content rating: "off", "low", "medium" or "high".
                        property string contentFilter: "medium"
                        property int columns: 4
                        property int perPage: 30
                    }
                    property JsonObject grammar: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject fonts: JsonObject {
                        property bool enable: true
                        property string sampleText: "The quick brown fox jumps over the lazy dog 0123456789"
                    }
                    property bool math: true
                    property bool webSearch: true
                    property bool shellCommand: true
                    property bool systemControls: true
                    property bool shellActions: true
                    property JsonObject calendar: JsonObject {
                        property bool enable: false
                        property string source: "khal"
                        property int lookaheadDays: 14
                        property bool showDeclined: false
                        property bool allowCreate: true
                        property string defaultCalendarId: ""
                        property int defaultDurationMin: 30
                        property list<string> hiddenCalendars: []
                    }
                    property JsonObject tasks: JsonObject {
                        property bool enable: true
                        property bool showCompleted: false
                        property bool allowCreate: true
                        property string defaultList: ""
                        property int lookaheadDays: 7
                    }
                    property JsonObject timers: JsonObject {
                        property bool enable: true
                        property bool showPomodoro: true
                        property bool showStopwatch: true
                        property bool showAlarms: true
                        property list<int> quickPresets: [5, 10, 15, 25, 45, 60]
                    }
                    property JsonObject quicklinks: JsonObject {
                        property bool enable: true
                        property list<var> links: []
                        property bool copyOnEnter: false
                        property bool fetchFavicons: true
                    }
                    property JsonObject windowManagement: JsonObject {
                        property bool enable: true
                        property bool showTilingPresets: true
                        property bool showWorkspaceMoves: true
                        property bool showMonitorMoves: true
                        property int columns: 3
                    }
                    property JsonObject screenshots: JsonObject {
                        property bool enable: true
                        property int maxItems: 60
                        property bool blurPreviews: false
                    }
                    property JsonObject settingsToggles: JsonObject {
                        property bool enable: true
                        property bool showPages: true
                        property int maxInlineResults: 0
                    }
                    property JsonObject quickToggles: JsonObject {
                        property bool enable: true
                        property list<string> hidden: []
                    }
                    property JsonObject keybinds: JsonObject {
                        property bool enable: true
                        property bool includeUserBinds: true
                        property bool includeDefaultBinds: true
                    }
                    property JsonObject cheatsheet: JsonObject {
                        property bool enable: true
                        property bool commandsPanel: true
                        property bool gmailPanel: true
                    }
                    property JsonObject sports: JsonObject {
                        property bool enable: false
                        property int lookaheadHours: 72
                        property list<string> leagues: []
                    }
                    property JsonObject snippets: JsonObject {
                        property bool enable: true
                        property list<var> items: []
                    }
                    property JsonObject notes: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject processes: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject converter: JsonObject {
                        property bool enable: true
                        property string baseCurrency: "BRL"
                    }
                    property JsonObject tools: JsonObject {
                        property bool enable: true
                    }
                    property JsonObject generators: JsonObject {
                        property bool enable: true
                    }
                }
                property JsonObject frecencyData: JsonObject {
                    property bool trackApps: true
                    property bool trackPanels: true
                    property bool trackActions: true
                }
                property JsonObject favorites: JsonObject {
                    property bool enable: true
                }
                // Ctrl+letter shortcuts bound to results from More actions.
                // The bindings themselves live in Persistent.states.search.
                property JsonObject resultKeybinds: JsonObject {
                    property bool enable: true
                }
                property JsonObject fallbacks: JsonObject {
                    property bool enable: true
                    property list<string> actions: ["ai", "web", "tasks", "calendar"]
                }
                property JsonObject history: JsonObject {
                    property bool enable: true
                    property int maxItems: 50
                }
                // Search-only bindings. They remain local to the focused Search
                // field and therefore cannot collide with Hyprland global binds.
                property list<var> keybindings: [
                    {
                        actionId: "actions",
                        shortcut: "Ctrl+K"
                    },
                    {
                        actionId: "favorite",
                        shortcut: "Ctrl+P"
                    },
                    {
                        actionId: "historyPrevious",
                        shortcut: "Up"
                    },
                    {
                        actionId: "historyNext",
                        shortcut: "Down"
                    },
                    {
                        actionId: "secondary",
                        shortcut: "Ctrl+Enter"
                    },
                    {
                        actionId: "copy",
                        shortcut: "Ctrl+C"
                    },
                    {
                        actionId: "save",
                        shortcut: "Ctrl+S"
                    },
                    {
                        actionId: "edit",
                        shortcut: "Ctrl+E"
                    },
                    {
                        actionId: "ocr",
                        shortcut: "Ctrl+O"
                    },
                    {
                        actionId: "create",
                        shortcut: "Ctrl+N"
                    },
                    {
                        actionId: "copyDispatch",
                        shortcut: "Ctrl+Shift+K"
                    },
                    {
                        actionId: "delete",
                        shortcut: "Shift+Delete"
                    },
                    {
                        actionId: "section",
                        shortcut: "Tab"
                    },
                    {
                        actionId: "select",
                        shortcut: "Ctrl+Space"
                    },
                    {
                        actionId: "cut",
                        shortcut: "Ctrl+X"
                    },
                    {
                        actionId: "paste",
                        shortcut: "Ctrl+V"
                    },
                    {
                        actionId: "createFolder",
                        shortcut: "Ctrl+Shift+N"
                    },
                    {
                        actionId: "duplicate",
                        shortcut: "Ctrl+D"
                    },
                    {
                        actionId: "toggleHidden",
                        shortcut: "Ctrl+H"
                    },
                    {
                        actionId: "refresh",
                        shortcut: "Ctrl+R"
                    },
                    {
                        actionId: "stageCopy",
                        shortcut: "Ctrl+Shift+C"
                    },
                    {
                        actionId: "sortFiles",
                        shortcut: "Ctrl+Shift+S"
                    },
                    {
                        actionId: "goHome",
                        shortcut: "Ctrl+Home"
                    },
                    {
                        actionId: "forward",
                        shortcut: "Alt+Right"
                    }
                ]
                property JsonObject appearance: JsonObject {
                    property bool accentPanels: true
                    property real accentStrength: 0.12
                    property bool showKeyHints: true
                    property bool showKeyHintBar: true
                    property int panelWidth: 860
                    property int panelBodyHeight: 420
                }
                property JsonObject imageSearch: JsonObject {
                    property string imageSearchEngineBaseUrl: "https://lens.google.com/uploadbyurl?url=" //lens.google.com/uploadbyurl?url="
                    property bool useCircleSelection: true
                }
                property JsonObject clipboard: JsonObject {
                    property int panelWidth: 860
                    property real listColumnRatio: 0.40
                    property bool showMetadata: true
                    property int imageHeight: 200
                    property int previewFontSize: 12
                    property bool enableSloppySearch: false
                    property JsonObject autoDelete: JsonObject {
                        property bool enable: false
                        property int retentionDays: 30
                        property bool wipeOnShutdown: false
                    }
                    property JsonObject detectors: JsonObject {
                        property bool hexColor: true
                        property bool url: true
                        property bool email: true
                        property bool phone: true
                        property bool json: true
                        property bool filePath: true
                        property bool markdown: true
                        property bool number: true
                        property bool multiline: true
                    }
                }
                property JsonObject nowPlaying: JsonObject {
                    property bool enable: false
                    property bool showInlineControls: true
                    property bool tintFromArtwork: false
                    property bool showPlayerName: true
                }
                property bool showNowPlayingBubble: nowPlaying.enable
                property string connectStyle: "connect"  // Search rendered as embedded drop in Connect Mode
                property int baseWidth: 580
                property int baseHeight: 500
                property string positionStyle: "default"
                property real centerVerticalRatio: 0.3
                property JsonObject suggestions: JsonObject {
                    property bool enable: false
                    // Applies only to the "Suggestions" strip below — every
                    // other idle category shows in full, the same way its
                    // real-search counterpart does.
                    property int maxSuggestionsPerSection: 5
                    property bool showFrecency: true
                    property bool showCommands: false
                    property bool showApps: true
                    property bool showAliases: true
                    property bool showToggles: true
                    property bool showPanels: true
                    property bool showQuicklinks: true
                }
            }

            property JsonObject mediaDownloader: JsonObject {
                property bool enabled: true
                property string downloadPath: FileUtils.trimFileProtocol(`${Directories.home}/Downloads`)
                property int maxConcurrent: 2
                property string defaultFormat: "best"
                property bool embedMetadata: true
                property bool writeThumbnail: false
                property bool addChapters: true
                property string proxy: ""
                property int rateLimit: 0
                property bool throttleBypass: false
                property bool useAria2c: false
                property string extraArgs: ""
                property bool keepHistory: false
                property string lastUsedFormat: "best"
                property string videoResolution: "best"
                property string videoCodec: "any"
                property int audioBitrate: 0
                property string audioCodec: "any"
                property string lastUsedResolution: "best"
                property bool showAdvancedArgs: false
            }

            property JsonObject sidebar: JsonObject {
                property JsonObject dashboardHeader: JsonObject {
                    // "custom" was never a value SidebarDashboardContent.qml checks for —
                    // it only branches on "user_profile" | "distro" | "none" (falling through
                    // to nothing rendered otherwise). The real default renders the profile
                    // picture/uptime row, so this now matches "user_profile".
                    property string profileImageType: "user_profile" // "user_profile", "distro", "none"
                    property string profileImagePath: Directories.userProfileImagePath
                    // Independent from userProfile.avatarShape: the sidebar avatar is
                    // shaped on its own so the settings/general avatar can differ.
                    property string avatarShape: "Cookie9Sided"
                    property string textMode: "username" // "username", "uptime", "none", "custom"
                    property string customText: ""
                    property string buttonsDesign: "circles" // "default", "circles"
                }
                property bool enableBanner
                property bool useCustomBanner
                property string bannerImage: ""
                property JsonObject dashboardSubHeader: JsonObject {
                    property string greetingSubtextMode: "uptime"  // "uptime", "custom", "none"
                    property string customText: ""
                }
                property string position: "default"
                property string sidebarStyle: "default" // "default" | "connect"
                property JsonObject tabletShade: JsonObject {
                    // Height of the strip at the very top that starts the pull-down. It sits
                    // above the bar on the Overlay layer, so whatever it covers stops being
                    // clickable — raise it for an easier grab, lower it to keep the bar usable.
                    property int edgeDragHeight: 8
                    // The tablet shade blurs a screencopy of the desktop so the blur can ramp
                    // with the drag. Off = one frozen frame per open (cheap, no feedback);
                    // on = continuous capture, which also re-captures the shade's own output.
                    property bool liveBackdrop: false
                }
                property bool keepRightSidebarLoaded: true
                property bool keepLeftSidebarLoaded: true
                property bool dashboardEntranceAnimations: false
                property bool volumeDialogMediaWidget: true
                property JsonObject bottomGroup: JsonObject {
                    property bool notesTab: true
                }
                property JsonObject translator: JsonObject {
                    property bool enable: false
                    property int delay: 300 // Delay before sending request. Reduces (potential) rate limits and lag.
                }
                property JsonObject ai: JsonObject {
                    property bool textFadeIn: false
                    // Transcript presentation. These values deliberately live
                    // beside the sidebar because Search keeps its compact
                    // density regardless of this chat-specific preference.
                    property string density: "comfortable" // comfortable | compact
                    property bool showTimestamps: false
                    property bool showResponseTime: false
                    property bool showAnswerModel: true
                    // Empty follows the persisted per-model default; valid
                    // values are off, low, medium and high.
                    property string thinkingDefault: ""
                    property string activityDefault: "auto" // auto | expanded | collapsed
                    property bool autoScroll: true
                    // One source of truth for chat motion. Search forwards
                    // this same preference into its navigator.
                    property bool reducedMotion: false
                    // Ordered shortcuts for Ctrl+1 … Ctrl+9 in the sidebar.
                    property list<string> pinnedModels: []
                    property string sendKey: "enter" // enter | ctrlEnter
                    property bool renderMarkdown: true
                    property bool renderLatex: true
                    property bool codeWrap: false
                    property bool codeLineNumbers: true
                    property bool collapseLongAnswers: true
                    property list<string> barKeys: ["keys", "advanced", "sessions", "newChat"]
                    property string greeting: ""
                    property bool emptyStateKeys: true
                    property bool soundOnAnswer: false
                }
                property JsonObject booru: JsonObject {
                    property bool allowNsfw: false
                    property string defaultProvider: "yandere"
                    property int limit: 20
                    property JsonObject zerochan: JsonObject {
                        property string username: "[unset]"
                    }
                }
                property JsonObject cornerOpen: JsonObject {
                    property bool enable: false
                    property bool bottom: false
                    property bool valueScroll: true
                    property bool clickless: false
                    property int cornerRegionWidth: 100
                    property int cornerRegionHeight: 5
                    property bool visualize: false
                    property bool clicklessCornerEnd: true
                    property int clicklessCornerVerticalOffset: 1
                }

                property JsonObject quickToggles: JsonObject {
                    property string style: "android" // Options: classic, android
                    property bool useThreeWaySliders: true
                    property JsonObject classic: JsonObject {
                        // Order matters: it is the order the toggles appear in.
                        property list<var> toggles: ["network", "bluetooth", "vpn", "tailscale", "nightLight", "gameMode", "idleInhibitor", "modes", "easyEffects", "cloudflareWarp", "keyboardBacklight"]
                    }
                    /**
                     * The quick-toggle layout the *tablet* family draws and edits.
                     *
                     * A separate object, because the two families have different screens
                     * to fit the grid on: a four-column block tuned for a 460px sidebar
                     * is not the arrangement anyone wants on a tablet's full-width shade,
                     * and before this, adapting one silently rearranged the other.
                     *
                     * Empty `pages` means "never edited here". While it is empty the
                     * tablet draws the desktop's layout — see PanelFamily.quickTogglePages
                     * — so a new tablet user starts from something known-good rather than
                     * from a blank grid, and an existing one keeps the arrangement they
                     * already had. The first edit on the tablet writes a whole normalized
                     * set here, and from then on the two are independent.
                     *
                     * Empty rather than a copy of the list below on purpose: two copies of
                     * one default are two things to keep in step, and the fallback says
                     * "the same, until you say otherwise" without either drifting.
                     */
                    property JsonObject androidTablet: JsonObject {
                        property int columns: 4
                        property int layoutVersion: 2
                        property list<var> pages: []
                    }
                    property JsonObject android: JsonObject {
                        property int columns: 4
                        property int layoutVersion: 2
                        property list<var> pages: [[
                                {
                                    "id": "brightnessSlider",
                                    "sizeH": 1,
                                    "sizeW": 4,
                                    "type": "brightnessSlider"
                                },
                                {
                                    "id": "volumeSlider",
                                    "sizeH": 1,
                                    "sizeW": 4,
                                    "type": "volumeSlider"
                                },
                                {
                                    "id": "network",
                                    "sizeH": 1,
                                    "sizeW": 2,
                                    "type": "network"
                                },
                                {
                                    "id": "bluetooth",
                                    "sizeH": 1,
                                    "sizeW": 2,
                                    "type": "bluetooth"
                                },
                                {
                                    "id": "mic",
                                    "sizeH": 1,
                                    "sizeW": 2,
                                    "type": "mic"
                                },
                                {
                                    "id": "audio",
                                    "sizeH": 1,
                                    "sizeW": 2,
                                    "type": "audio"
                                },
                                {
                                    "id": "nightLight",
                                    "sizeH": 1,
                                    "sizeW": 2,
                                    "type": "nightLight"
                                },
                                {
                                    "id": "darkMode",
                                    "sizeH": 1,
                                    "sizeW": 2,
                                    "type": "darkMode"
                                }
                            ]]
                    }
                }

                property JsonObject quickSliders: JsonObject {
                    property bool enable: false
                    property bool vertical: false
                    property bool showMic: true
                    property bool showGamma: true
                    property bool showVolume: true
                    property bool showBrightness: false // gamma setting also works for brightness
                }
            }

            property JsonObject screenRecord: JsonObject {
                property string savePath: Directories.videos.replace("file://", "") // strip "file://"
                property string service: "wf-recorder"
                property bool useGpu: true
                property string codec: "auto"
                // Recorded size, as a target box the picture is fitted into
                // without distorting it: "native" | "2160p" | "1440p" | "1080p" | "720p" | "480p"
                property string resolution: "native"
                // Picks the bits-per-pixel the bitrate is derived from, so the
                // user never has to think in Mbps: "low" | "balanced" | "high"
                property string quality: "balanced"
                property int framerate: 60
                // "cfr" duplicates frames to hold the target rate, which is what
                // editors expect; "vfr" records only on screen updates, which is
                // smaller but harder to cut.
                property string frameSync: "cfr"
                // Records desktop audio on every recording, not only the
                // ones started with --sound (wf-recorder only).
                property bool recordAudio: false
                property bool useToolbar: false
                property string format: "mp4"
                property string mode: "fullscreen"
                property bool recordMic: false
                property string micDevice: ""
                property int countdown: 0
                property bool showNotifications: true
                property bool showEditPrompt: true
                property bool openInLosslessCut: false

                property JsonObject keypress: JsonObject {
                    // The persistent default applied to every new recording; the
                    // recording indicator can override it for one clip.
                    property bool showWhileRecording: false
                    // "top" | "topLeft" | "topRight" | "bottom" | "bottomLeft" | "bottomRight"
                    property string position: "bottom"
                    property int marginH: 32
                    property int marginV: 96
                    property int hideDelayMs: 2500
                    property int maxKeys: 5
                    property real scale: 1.0
                    property bool showMouseButtons: false
                    property bool onlyShortcuts: false
                    property bool mergeTyping: true
                }
            }

            property JsonObject screenSnip: JsonObject {
                property string savePath: "" // only copy to clipboard when empty
            }

            property JsonObject sounds: JsonObject {
                property bool enable: true
                property int volume: 100
                property string theme: "freedesktop"
                property bool monoAudio: false

                property bool notifications: true
                property bool volumeChange: true
                property bool battery: false
                property bool screenshot: true
                property bool pomodoro: false
                property bool timer: true
                property bool alarm: true
                property bool session: false
                property bool devices: true
                property bool lock: false

                property bool alarmFadeIn: false
                property int alarmFadeInSeconds: 30

                property string notificationDefaultPolicy: "play" // "play" | "mute"
                property list<string> alwaysPlayApps: []
                property list<string> neverPlayApps: []
                property JsonObject custom: JsonObject {
                    property string notifications: ""
                    property string volumeChange: ""
                    property string battery: ""
                    property string screenshot: ""
                    property string pomodoro: ""
                    property string timer: ""
                    property string alarm: ""
                    property string session: ""
                    property string devices: ""
                    property string lock: ""
                }
            }

            property JsonObject soundcore: JsonObject {
                property string macAddress: ""
                property string model: "SoundcoreA3028"
            }

            // Window tiling assistant: zone overlay while a window is dragged,
            // quick-tile on drop. Zones are stored as fractions of the usable
            // monitor area, so a layout survives resolution and scale changes.
            property JsonObject tiling: JsonObject {
                property bool enable: false
                // "quickTile": float the window and set exact geometry on drop
                // "preview": only draw the overlay, never touch the window
                // "hybrid": quick-tile floating windows, preview-only for
                //           windows currently in the Hyprland layout tree
                property string mode: "quickTile"
                // Off means the overlay only appears for keyboard quick-tile.
                property bool showOnDragStart: true
                // Restore pre-tile geometry and float state when a tiled window
                // is dragged back out of its zone.
                property bool restoreOnUntile: true
                // Super + Alt + arrow moves the focused window zone by zone, and
                // untiles it when it is already against that side of the screen.
                property bool keyboardQuickTile: true
                // Super + drag drops a window into the zone under the cursor.
                // Off leaves the keyboard the only way in, for people who would
                // rather a grabbed window went exactly where they put it.
                // Resizing a tiled window still pulls its neighbours along:
                // that adjusts a layout already there rather than tiling by
                // grab, and follows coResize below.
                property bool dragQuickTile: true

                property JsonObject detection: JsonObject {
                    // Companion Hyprland binds on the drag/resize mouse combos.
                    // The only way a drag is detected: client-side titlebar
                    // drags fire no bind, and inferring them from window motion
                    // was dropped because nothing reports the button coming up.
                    property bool useKeybinds: true
                    // How often the active window is sampled between gestures.
                    // Only used to keep a pre-drag geometry worth restoring to,
                    // so there is nothing to gain by doing it often.
                    property int idleHz: 5
                    property int activeHz: 90
                    // How far the window has to move before a drag counts as
                    // having left the place Hyprland would put it back to.
                    property int trackingTolerancePx: 2
                }

                property JsonObject coResize: JsonObject {
                    property bool enable: true
                    // How near a shared edge a resize has to land to pull its
                    // neighbours along.
                    property int edgeTolerancePx: 8
                    // Tiling one window takes the whole workspace with it:
                    // every other window on it is floated and given a zone, so
                    // they all share edges and one divider drag moves the lot.
                    // Off by default - it hands the workspace over wholesale
                    // and leaves Hyprland's own layout with nothing to tile.
                    property bool adoptWorkspace: false
                    // The reverse: untiling one window hands the whole
                    // workspace back, every window the assistant placed on it
                    // returning to where it was. Closing a window is not
                    // untiling it - what it leaves behind is still the
                    // arrangement that was asked for.
                    property bool releaseWorkspace: false
                }

                property JsonObject overlay: JsonObject {
                    property real zoneOpacity: 0.28
                    property real hoveredOpacity: 0.55
                    property int cornerRadius: 12
                    property bool showLabels: true
                    property int fadeDuration: 150
                    // How long the strip of layout names stays up after the
                    // layout is cycled.
                    property int layoutHintDuration: 1400
                    // How long the zones are shown after a keyboard
                    // quick-tile, which has no drag to show them during. Long
                    // enough to see where the window landed, short enough that
                    // arrowing a window across the screen is not one continuous
                    // flash.
                    property int quickTileDuration: 500
                    // Marks a zone holding more than one window while nothing is
                    // being dragged. Windows stacked in a zone are otherwise
                    // indistinguishable from one window sitting there.
                    property bool stackIndicator: true
                }

                property JsonObject gaps: JsonObject {
                    // Read gaps_in / gaps_out from Hyprland instead of the
                    // values below.
                    property bool followHyprland: true
                    property int outer: 8
                    property int inner: 4
                }

                // Preset id used by any monitor without an entry below.
                property string defaultPreset: "kde"
                // Per-monitor layouts. Each entry looks like:
                //   { name: "DP-1", preset: "custom", gapsOverride: null,
                //     zones: [{ x, y, w, h, label }] }
                // where x/y/w/h are 0..1 fractions of the usable area.
                property list<var> monitors: []
            }

            property JsonObject time: JsonObject {
                // https://doc.qt.io/qt-6/qtime.html#toString
                property string format: "hh:mm"
                property string secondsFormat: "ss"
                property string shortDateFormat: "dd/MM"
                property string longDateFormat: "dd/MM/yyyy"
                property string dateWithYearFormat: "dd/MM/yyyy"
                property string dateFormat: "MM/dd, ddd"
                property int firstDayOfWeek: 6 // 0: Monday, 1: Tuesday, 2: Wednesday, 3: Thursday, 4: Friday, 5: Saturday, 6: Sunday

                property JsonObject pomodoro: JsonObject {
                    property int breakTime: 300
                    property int cyclesBeforeLongBreak: 4
                    property int focus: 1500
                    property int longBreak: 900
                    property bool notify: true
                }
                property JsonObject timer: JsonObject {
                    property bool notify: true
                    property list<var> presets: [60, 300, 600, 900, 1800, 3600] // seconds
                }
                property list<var> worldClocks: []
                property bool secondPrecision: false

                property JsonObject alarms: JsonObject {
                    property bool useFullscreenPopup: false
                    property bool showAnalogClock: true
                    property bool showWorldClocks: true
                    property bool showAlarmsSection: true
                    property int snoozeMinutes: 9
                    property int autoSilenceMinutes: 5 // 0 keeps ringing until dismissed
                    // An alarm whose minute passed while the shell was not looking
                    // (suspend, crash, restart) still rings if it is at most this late;
                    // older ones are reported as missed instead.
                    property int catchUpMinutes: 10
                    // Program a systemd user timer with WakeSystem=true so a suspended
                    // machine wakes up shortly before the next alarm.
                    property bool wakeFromSuspend: true
                    property int wakeLeadSeconds: 90
                    // The bar and the island show the next alarm this many minutes ahead.
                    property int upcomingMinutes: 60
                    property bool showUpcoming: true
                }
            }

            property JsonObject updates: JsonObject {
                property bool enableCheck: true
                property int checkInterval: 120 // minutes
                property int adviseUpdateThreshold: 75 // packages
                property int stronglyAdviseUpdateThreshold: 200 // packages
            }

            property JsonObject wallpaperSelector: JsonObject {
                property bool useSystemFileDialog: false
                property list<var> directories: []
                property bool useCustomDefaultPath: false
                property string customDefaultPath: FileUtils.trimFileProtocol(`${Directories.pictures}/Wallpapers`)
                property string sortField: "modified"
                property bool sortReversed: false
            }

            property JsonObject windows: JsonObject {
                property bool showTitlebar: true // Client-side decoration for shell apps
                property bool centerTitle: true
            }

            property JsonObject hacks: JsonObject {
                property int arbitraryRaceConditionDelay: 20 // milliseconds
            }

            property JsonObject workSafety: JsonObject {
                property JsonObject enable: JsonObject {
                    property bool wallpaper: false
                    property bool clipboard: false
                }
                property JsonObject triggerCondition: JsonObject {
                    property list<string> networkNameKeywords: ["airport", "cafe", "college", "company", "eduroam", "free", "guest", "public", "school", "university"]
                    property list<string> fileKeywords: ["anime", "booru", "ecchi", "hentai", "yande.re", "konachan", "breast", "nipples", "pussy", "nsfw", "spoiler", "girl"]
                    property list<string> linkKeywords: ["hentai", "porn", "sukebei", "hitomi.la", "rule34", "gelbooru", "fanbox", "dlsite"]
                }
            }

            property JsonObject wallpapers: JsonObject {
                property string service: "wallhaven" // "unsplash" or "wallhaven"
                property string sort: "favourites"
                property bool showAnimeResults: false // only for wallhaven service
                property JsonObject paths: JsonObject {
                    property string download: FileUtils.trimFileProtocol(`${Directories.home}/Pictures/Wallpapers`)
                    property string nsfw: FileUtils.trimFileProtocol(`${Directories.home}/Pictures/Wallpapers/NSFW`)
                }
            }

            property JsonObject waffles: JsonObject {
                // Some spots are kinda janky/awkward. Setting the following to
                // false will make (some) stuff also be like that for accuracy.
                // Example: the right-click menu of the Start button
                property JsonObject tweaks: JsonObject {
                    property bool switchHandlePositionFix: true
                    property bool smootherMenuAnimations: true
                    property bool smootherSearchBar: true
                }
                property JsonObject bar: JsonObject {
                    property bool bottom: true
                    property bool leftAlignApps: false
                }
                property JsonObject actionCenter: JsonObject {
                    property list<string> toggles: ["network", "bluetooth", "easyEffects", "powerProfile", "idleInhibitor", "nightLight", "darkMode", "antiFlashbang", "cloudflareWarp", "mic", "musicRecognition", "notifications", "onScreenKeyboard", "gameMode", "screenSnip", "colorPicker", "videoEditor"]
                }
                property JsonObject calendar: JsonObject {
                    property bool force2CharDayOfWeek: true
                }
            }
        }
    }
}
