pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../../../ask/bar"
import "../../../ask/look"
import "components"
import "../../parts"
import "../../../ask/lib/downloads.js" as Logic
import "../../../ask/lib/format.js" as Format
import "../../../ask/lib/root.js" as RootLogic

Item {
    id: root

    required property real s
    property bool active: false
    property string query: ""
    property var args: ({})
    property real maximumHeight: 480 * s
    property var downloads: null
    property var memory: null
    property var source: null
    property string pageId: "queue"
    readonly property var pages: [
        { id: "queue", label: I18n.tr("Queue"), count: root.tasks.length },
        { id: "settings", label: I18n.tr("Settings") }
    ]
    readonly property bool ownsText: false
    readonly property bool stacked: root.height < Math.round(610 * root.s)
    onPageIdChanged: {
        if (root.pageId !== "queue" && root.pageId !== "settings")
            root.pageId = "queue";
        root.editing = "";
    }

    readonly property string title: root.settingsOpen ? I18n.tr("Download settings") : I18n.tr("Downloads")
    readonly property string placeholder: root.editing === "dir" ? I18n.tr("The folder new downloads land in")
        : root.editing === "port" ? I18n.tr("A port number, 0 for a random one")
        : root.editing === "batch-dir" ? I18n.tr("The folder these links land in")
        : root.settingsOpen ? I18n.tr("Filter settings")
        : I18n.tr("Paste a link or filter downloads")
    readonly property real preferredWidth: root.paned || root.batch ? Math.round(860 * root.s) : 0
    readonly property Component statusItem: footComponent
    readonly property string status: root.footNote.length > 0 ? root.footNote : root.footCounts

    signal pushRequested(string view, var args)
    signal closeRequested()
    signal hud(string text)
    signal alert(string text)
    signal queryRequested(string text)
    signal menuRequested()

    // the engine
    readonly property string engine: root.downloads ? String(root.downloads.state) : "idle"
    readonly property bool ready: root.engine === "ready"
    readonly property bool offline: root.engine === "not-installed" || root.engine === "error"
    readonly property var tasks: root.downloads ? root.downloads.tasks : []
    readonly property var stats: root.downloads ? root.downloads.stats : ({})
    readonly property var settings: root.downloads ? root.downloads.settings : ({})
    readonly property string home: String(Quickshell.env("HOME") || "")

    // what the field holds: a link to read, or words to filter by
    // here a bare "name.iso" is a file to find in the queue, so only a link that
    // says it is one (a scheme, a magnet, www.) is read
    readonly property var typedLink: root.settingsOpen || !/^\s*(magnet:|[a-z][a-z0-9+.-]*:\/\/|www\.)/i.test(root.query) ? null : RootLogic.detectLink(root.query)
    property string handedLink: ""
    readonly property string link: root.typedLink ? String(root.typedLink.url) : root.handedLink
    readonly property bool intake: root.link.length > 0 && !root.offline && !root.batch
    readonly property string filter: root.typedLink || root.settingsOpen || root.batch ? "" : root.query.trim()

    // several links: typed together, or held (handed over, or lifted out of the
    // field while it is used to type the batch's folder)
    readonly property bool typedMany: !root.settingsOpen && root.editing !== "batch-dir" && RootLogic.countLinks(root.query) >= 2
    property string heldBatch: ""
    property string batchFolder: ""
    readonly property string batchText: root.typedMany ? root.query : root.heldBatch
    readonly property bool batch: root.batchText.length > 0 && !root.offline && !root.settingsOpen && root.typedLink === null
    readonly property BatchCard card: cardLoader.item as BatchCard

    readonly property bool settingsOpen: root.pageId === "settings"
    onSettingsOpenChanged: if (root.active) landing.restart()
    property string editing: ""

    property real landed: 1
    NumberAnimation {
        id: landing
        target: root
        property: "landed"
        from: 0
        to: 1
        duration: Look.dur(Look.viewMs)
        easing.type: Easing.BezierSpline
        easing.bezierCurve: Look.arrive
    }
    onIntakeChanged: if (root.active) landing.restart()
    onBatchChanged: if (root.active) landing.restart()
    readonly property SettingsSheet sheet: sheetLoader.item as SettingsSheet

    readonly property bool paned: !root.settingsOpen && !root.offline && !root.batch && (root.intake || root.tasks.length > 0)

    // ---- the queue ----
    // a map is decoded once per distinct string (finished rows all share one). The
    // cache is a plain object changed in place, so reading it binds nothing
    readonly property var fillCache: ({ maps: {}, count: 0 })
    function fillsOf(task) {
        const cache = root.fillCache;
        const key = String(task.pieces || "");
        var hit = cache.maps[key];
        if (hit === undefined) {
            if (cache.count >= 256) {
                cache.maps = {};
                cache.count = 0;
            }
            hit = Format.pieces(key);
            cache.maps[key] = hit;
            cache.count++;
        }
        return hit;
    }

    function timeLeft(task) {
        return Number(task.eta) >= 0 ? Format.duration(Math.max(1, Number(task.eta))) : "";
    }

    function join(parts) {
        return parts.filter(part => String(part || "").length > 0).join(" \u00b7 ");
    }

    function factsOf(task) {
        const state = String(task.state || "");
        const percent = Format.percent(task.done, task.size);
        if (state === "active")
            return root.join([percent.length > 0 ? percent : Format.bytes(task.done), Format.speed(task.speed), root.timeLeft(task)]);
        if (state === "resolving")
            return I18n.tr("Reading the link");
        if (state === "merging")
            return I18n.tr("Putting it together");
        if (state === "seeding")
            return root.join([I18n.tr("Seeding"), Format.speed(task.upSpeed)]);
        if (state === "queued")
            return root.join([I18n.tr("Queued"), Number(task.size) > 0 ? Format.bytes(task.size) : ""]);
        if (state === "paused")
            return root.join([I18n.tr("Paused"), percent]);
        if (state === "stopped")
            return root.join([I18n.tr("Stopped"), percent]);
        if (state === "done")
            return root.join([I18n.tr("Done"), Format.bytes(Number(task.size) > 0 ? task.size : task.done)]);
        return I18n.tr("Failed");
    }

    function rowOf(task) {
        const state = String(task.state || "");
        const failed = state === "error";
        return {
            id: String(task.id), size: "map", task: task, owner: String(task.id),
            glyph: Logic.glyph(task), title: Logic.nameOf(task),
            facts: root.factsOf(task),
            factTone: state === "active" || state === "merging" || state === "resolving" ? "" : state === "seeding" ? "done" : Logic.tone(state),
            fills: root.fillsOf(task), tone: Logic.tone(state),
            reason: failed ? root.join([task.source ? String(task.source.name || "") : "", root.reasonOf(task.error)]) : ""
        };
    }

    readonly property var grouped: Logic.group(root.tasks, root.filter)
    readonly property var sections: [
        { id: "active", title: I18n.tr("Active"), note: String(root.grouped.active.length), rows: root.grouped.active.map(root.rowOf) },
        { id: "waiting", title: I18n.tr("Waiting"), note: String(root.grouped.waiting.length), rows: root.grouped.waiting.map(root.rowOf) },
        { id: "finished", title: I18n.tr("Finished"), note: String(root.grouped.finished.length), rows: root.grouped.finished.map(root.rowOf) }
    ]

    // the selection, read fresh from the daemon's list
    readonly property var task: !root.intake && list.current ? list.current.task : null
    readonly property string taskId: root.task ? String(root.task.id) : ""
    readonly property var can: Logic.can(root.task)

    // a row asked for by id is taken once it is in the list
    property string wanted: ""
    function take() {
        if (root.wanted.length === 0)
            return;
        const rows = list.layout.rows;
        for (var i = 0; i < rows.length; i++) {
            if (String(rows[i].row.id) === root.wanted) {
                root.wanted = "";
                list.select(i);
                return;
            }
        }
    }
    onSectionsChanged: Qt.callLater(root.take)

    // ---- the selected transfer's last minute ----
    property var samples: []
    property string samplesFor: ""
    property bool daemonHistory: true

    function sample() {
        if (!root.active || !root.downloads || !root.task)
            return;
        const id = root.taskId;
        const state = String(root.task.state);
        if (state !== "active" && state !== "seeding" && state !== "merging")
            return;
        if (!root.daemonHistory) {
            const next = (root.samplesFor === id ? root.samples : []).concat([Number(state === "seeding" ? root.task.upSpeed : root.task.speed) || 0]);
            root.samplesFor = id;
            root.samples = next.slice(-60);
            return;
        }
        root.downloads.call("history", { id: id }, function(result, error) {
            if (error) {
                // an engine from before `history`: keep the samples seen while open
                if (error.code === "invalid")
                    root.daemonHistory = false;
                return;
            }
            if (id !== root.taskId)
                return;
            root.samplesFor = id;
            root.samples = (state === "seeding" ? result.up : result.down) || [];
        });
    }
    onTaskIdChanged: {
        if (root.samplesFor !== root.taskId)
            root.samples = [];
        root.sample();
    }
    Connections {
        target: root.downloads
        ignoreUnknownSignals: true
        function onEventReceived(name, data) {
            if (name === "stats")
                root.sample();
        }
    }

    // ---- a link being read ----
    property string resolvedLink: ""
    property var resolved: null
    property var failure: null
    property int chip: 0
    property var picked: []
    property int cursor: -1
    readonly property string phase: root.failure ? "error" : root.resolved ? "ready" : "resolving"
    readonly property var chipSet: Logic.chips(root.resolved, parseInt(String(root.settings.videoQuality || ""), 10) || 0)
    readonly property var entries: {
        const r = root.resolved;
        if (!r)
            return [];
        if ((r.files || []).length > 0)
            return r.files.map(file => ({ name: String(file.path || ""), note: Number(file.size) > 0 ? Format.bytes(file.size) : "", size: Number(file.size) || 0 }));
        if ((r.items || []).length > 1) {
            // a kind every entry shares says nothing
            const mixed = r.items.some(item => item.kind !== r.items[0].kind);
            return r.items.map(item => ({ name: String(item.title || item.url || ""), note: mixed ? root.kindWord(String(item.kind || "")) : "", size: 0, titled: true }));
        }
        return [];
    }
    readonly property bool listed: root.entries.length > 0

    function kindWord(kind) {
        return kind === "video" ? I18n.tr("video") : kind === "audio" ? I18n.tr("audio") : kind === "image" || kind === "photo" ? I18n.tr("picture") : "";
    }

    function read() {
        const link = root.link;
        if (link.length === 0 || !root.downloads || link === root.resolvedLink)
            return;
        root.resolvedLink = link;
        root.resolved = null;
        root.failure = null;
        root.chip = 0;
        root.picked = [];
        root.cursor = -1;
        root.downloads.resolve(link, function(result, error) {
            if (link !== root.link)
                return;
            if (error) {
                root.failure = error;
                return;
            }
            root.resolved = result;
            root.picked = Logic.everything(root.entries.length);
            root.cursor = root.entries.length > 0 ? 0 : -1;
        });
    }
    function unread() {
        root.resolvedLink = "";
        root.resolved = null;
        root.failure = null;
    }
    onLinkChanged: {
        if (root.link.length === 0)
            root.unread();
        else
            reading.restart();
    }
    // a link is typed a character at a time; read it once the hand rests
    Timer { id: reading; interval: 260; onTriggered: root.read() }

    // a failure in one plain sentence, and the engine's own words where there is room
    function reasonOf(error) {
        return error && root.downloads ? String(root.downloads.reason(error)) : "";
    }
    function detailOf(error) {
        const said = error && root.downloads ? String(root.downloads.detail(error)) : "";
        return said === root.reasonOf(error) ? "" : said;
    }

    function estimate() {
        const r = root.resolved;
        if (!r)
            return -1;
        if (root.listed && (r.files || []).length > 0)
            return Logic.selectedSize(r.files, root.picked);
        if (root.chipSet.length > 0)
            return root.chipSet[Math.min(root.chip, root.chipSet.length - 1)].size;
        return Number(r.size) > 0 ? Number(r.size) : -1;
    }

    function chipText(chip) {
        return chip.kind === "best" ? I18n.tr("Best") : chip.kind === "audio" ? I18n.tr("Audio") : chip.label;
    }

    function tilde(path) {
        const value = String(path || "");
        return root.home.length > 0 && value.indexOf(root.home) === 0 ? "~" + value.slice(root.home.length) : value;
    }

    function start(paused) {
        const r = root.resolved;
        if (!r || !root.downloads)
            return;
        const params = { url: String(r.url || root.link) };
        if (root.chipSet.length > 0) {
            const chip = root.chipSet[Math.min(root.chip, root.chipSet.length - 1)];
            if (chip.mode !== "auto")
                params.mode = chip.mode;
            if (chip.format.length > 0)
                params.format = chip.format;
        }
        if (root.listed && root.picked.length < root.entries.length)
            params.files = root.picked;
        if (paused)
            params.paused = true;
        // the name already read, so a task added paused is not called by its link
        const single = root.listed && root.picked.length === 1 && (r.items || []).length > 1 ? String(r.items[root.picked[0]].title || "") : "";
        const known = single.length > 0 ? single : String(r.title || r.name || "");
        if (known.length > 0)
            params.title = known;
        root.downloads.add(params, function(task, error) {
            if (error) {
                root.alert(root.reasonOf(error));
                return;
            }
            root.hud(paused ? I18n.tr("Added, paused") : I18n.tr("Download started"));
            root.wanted = String(task.id || "");
            root.leaveIntake();
            Qt.callLater(root.take);
        });
    }

    function leaveIntake() {
        root.handedLink = "";
        if (root.typedLink)
            root.queryRequested("");
        root.unread();
    }

    // ---- the contract ----
    readonly property var primaryAction: {
        if (root.offline || root.engine === "connecting" || root.engine === "idle")
            return root.offline ? { label: I18n.tr("Try again") } : null;
        if (root.settingsOpen) {
            const row = root.sheet ? root.sheet.current : null;
            if (root.editing.length > 0)
                return { label: I18n.tr("Set") };
            if (!row)
                return null;
            return { label: row.kind === "toggle" ? (row.on ? I18n.tr("Turn off") : I18n.tr("Turn on")) : row.kind === "text" ? I18n.tr("Change") : I18n.tr("Next value") };
        }
        if (root.batch) {
            if (root.editing === "batch-dir")
                return { label: I18n.tr("Set") };
            if (!root.card || root.card.count === 0 || root.card.adding)
                return null;
            return { label: root.card.ticked > 0 ? I18n.tr("Download %1").arg(root.card.ticked) : I18n.tr("Nothing ticked") };
        }
        if (root.intake)
            return root.phase === "ready" ? { label: root.listed && root.picked.length === 0 ? I18n.tr("Nothing ticked") : I18n.tr("Download") }
                : root.phase === "error" ? { label: I18n.tr("Try again") } : null;
        const verb = Logic.primary(root.task);
        if (verb === "pause")
            return { label: root.task.state === "seeding" ? I18n.tr("Stop seeding") : I18n.tr("Pause") };
        if (verb === "resume")
            return { label: I18n.tr("Resume") };
        if (verb === "retry")
            return { label: I18n.tr("Retry") };
        if (verb === "open")
            return { label: I18n.tr("Open") };
        return root.ready && root.tasks.length === 0 ? { label: I18n.tr("Paste a link") } : null;
    }
    readonly property var secondaryAction: {
        if (root.settingsOpen || root.offline)
            return null;
        if (root.batch)
            return root.card && root.card.rowListed && root.editing.length === 0
                ? { label: root.card.row.only < 0 ? I18n.tr("Only one of the list") : I18n.tr("All of the list"), glyph: "playlist_play", hint: "Ctrl+Enter" } : null;
        if (root.intake) {
            if (root.phase !== "ready")
                return null;
            if (root.listed)
                return { label: I18n.tr("Tick or untick"), glyph: "check_box", hint: "Ctrl+Enter" };
            return root.chipSet.length > 1 ? { label: I18n.tr("Next format"), glyph: "tune", hint: "Ctrl+Enter" } : null;
        }
        return root.task ? { label: I18n.tr("Show in folder"), glyph: "folder_open", hint: "Ctrl+Enter" } : null;
    }
    readonly property var actions: {
        const out = [];
        const own = I18n.tr("This download");
        const all = I18n.tr("All downloads");
        if (root.settingsOpen) {
            if (root.editing.length === 0 && root.sheet && root.sheet.current && root.sheet.current.kind !== "text") {
                out.push({ id: "step-up", label: root.sheet.current.kind === "toggle" ? I18n.tr("On") : I18n.tr("More"), glyph: "chevron_right", hint: "Right", section: "" });
                out.push({ id: "step-down", label: root.sheet.current.kind === "toggle" ? I18n.tr("Off") : I18n.tr("Less"), glyph: "chevron_left", hint: "Left", section: "" });
            }
            out.push({ id: "settings", label: I18n.tr("Back to downloads"), glyph: "arrow_back", hint: "Ctrl+,", section: "" });
            return out;
        }
        if (root.offline)
            return out;
        if (root.batch) {
            // while the folder is typed the field needs its own keys back
            if (!root.card || root.card.count === 0 || root.editing.length > 0)
                return out;
            const mine = I18n.tr("This link");
            const every = I18n.tr("All links");
            out.push({ id: "batch-tick", label: I18n.tr("Tick or untick"), glyph: "check_box", hint: "Space", section: mine });
            if (root.card.rowChoices.length > 1) {
                out.push({ id: "batch-row-next", label: I18n.tr("Next format for this link"), glyph: "chevron_right", hint: "Right", section: mine });
                out.push({ id: "batch-row-previous", label: I18n.tr("Previous format for this link"), glyph: "chevron_left", hint: "Left", section: mine });
            }
            out.push({ id: "batch-copy", label: I18n.tr("Copy pasted link"), glyph: "content_copy", hint: "Ctrl+Shift+C", section: mine });
            out.push({ id: "batch-all-next", label: I18n.tr("Next format for all"), glyph: "chevron_right", hint: "Ctrl+Right", section: every });
            out.push({ id: "batch-all-previous", label: I18n.tr("Previous format for all"), glyph: "chevron_left", hint: "Ctrl+Left", section: every });
            out.push({ id: "batch-paused", label: I18n.tr("Add without starting"), glyph: "pause", hint: "Ctrl+Shift+P", section: every });
            out.push({ id: "batch-all", label: I18n.tr("Tick everything"), glyph: "select_all", hint: "Ctrl+Shift+A", section: every });
            out.push({ id: "batch-none", label: I18n.tr("Untick everything"), glyph: "deselect", hint: "Ctrl+Shift+N", section: every });
            out.push({ id: "batch-dir", label: I18n.tr("Change the folder"), glyph: "folder", hint: "Ctrl+Shift+F", section: every });
            return out;
        }
        if (root.intake && root.phase === "ready") {
            if (root.chipSet.length > 1) {
                out.push({ id: "format-next", label: I18n.tr("Next format"), glyph: "chevron_right", hint: "Ctrl+Right", section: "" });
                out.push({ id: "format-previous", label: I18n.tr("Previous format"), glyph: "chevron_left", hint: "Ctrl+Left", section: "" });
            }
            if (root.listed) {
                out.push({ id: "pick-all", label: I18n.tr("Tick everything"), glyph: "select_all", hint: "Ctrl+Shift+A", section: "" });
                out.push({ id: "pick-none", label: I18n.tr("Untick everything"), glyph: "deselect", hint: "Ctrl+Shift+N", section: "" });
            }
            out.push({ id: "add-paused", label: I18n.tr("Add without starting"), glyph: "pause", hint: "Ctrl+Shift+P", section: "" });
            out.push({ id: "copy-typed", label: I18n.tr("Copy link"), glyph: "content_copy", hint: "Ctrl+Shift+C", section: "" });
        } else if (root.task) {
            if (root.can.pause && Logic.primary(root.task) !== "pause")
                out.push({ id: "pause", label: I18n.tr("Pause"), glyph: "pause", hint: "", section: own });
            if (root.can.resume && Logic.primary(root.task) !== "resume")
                out.push({ id: "resume", label: I18n.tr("Resume"), glyph: "play_arrow", hint: "", section: own });
            if (root.can.open && Logic.primary(root.task) !== "open")
                out.push({ id: "open", label: I18n.tr("Open"), glyph: "open_in_new", hint: "", section: own });
            if (root.can.link)
                out.push({ id: "copy", label: I18n.tr("Copy link"), glyph: "content_copy", hint: "Ctrl+Shift+C", section: own });
            if (root.can.cancel)
                out.push({ id: "cancel", label: I18n.tr("Cancel download"), glyph: "cancel", hint: "Ctrl+Delete", section: own, destructive: true });
            if (root.can.remove)
                out.push({ id: "remove", label: I18n.tr("Remove from the list"), glyph: "playlist_remove", hint: "Ctrl+Delete", section: own });
            if (root.can.removeFile)
                out.push({ id: "remove-file", label: I18n.tr("Remove and delete the file"), glyph: "delete", hint: "Ctrl+Shift+Delete", section: own, destructive: true });
        }
        if (root.ready) {
            if (root.grouped.active.length + root.grouped.waiting.length > 0) {
                out.push({ id: "pause-all", label: I18n.tr("Pause all"), glyph: "pause_circle", hint: "Ctrl+Shift+P", section: all });
                out.push({ id: "resume-all", label: I18n.tr("Resume all"), glyph: "play_circle", hint: "Ctrl+Shift+R", section: all });
            }
            if (root.grouped.finished.length > 0)
                out.push({ id: "clear", label: I18n.tr("Clear finished"), glyph: "clear_all", hint: "Ctrl+Shift+L", section: all });
            out.push({ id: "settings", label: I18n.tr("Download settings"), glyph: "settings", hint: "Ctrl+,", section: all });
        }
        // an intake's own Ctrl+Shift+P wins over pause all
        const seen = {};
        return out.filter(action => {
            if (action.hint.length > 0 && seen[action.hint])
                action.hint = "";
            seen[action.hint] = true;
            return true;
        });
    }

    // a failed row's own words lead the Action Panel
    readonly property string actionNote: !root.intake && !root.settingsOpen && root.task ? root.detailOf(root.task.error) : ""

    readonly property string footCounts: {
        const s = root.stats || ({});
        const parts = [];
        if (Number(s.active || 0) > 0)
            parts.push(I18n.tr("%1 active").arg(s.active));
        if (Number(s.queued || 0) > 0)
            parts.push(I18n.tr("%1 queued").arg(s.queued));
        if (Number(s.seeding || 0) > 0)
            parts.push(I18n.tr("%1 seeding").arg(s.seeding));
        if (parts.length === 0 && root.tasks.length > 0)
            parts.push(I18n.tr("Nothing moving"));
        return parts.join(", ");
    }
    readonly property string footNote: root.settingsOpen && root.ready ? I18n.tr("Changes apply at once")
        : root.batch && root.card ? root.card.progress : ""

    implicitHeight: root.batch ? (root.card ? root.card.implicitHeight : Math.round(96 * root.s))
        : root.settingsOpen ? (root.sheet ? root.sheet.implicitHeight : 0)
        : root.paned ? Math.min(root.maximumHeight, Math.max(list.count > 0 ? list.layout.height : 0, Math.round(464 * root.s)))
        : blank.implicitHeight

    function move(delta) {
        if (root.batch) {
            if (root.card && root.editing.length === 0)
                root.card.move(delta);
        } else if (root.settingsOpen) {
            if (root.sheet)
                root.sheet.move(delta);
        } else if (root.intake) {
            if (root.listed)
                root.cursor = Logic.clamp(root.cursor + delta, 0, root.entries.length - 1);
        } else {
            list.move(delta);
        }
    }
    function jumpSection(delta) {
        if (root.batch)
            { if (root.card && root.editing.length === 0) root.card.jumpSection(delta); }
        else if (root.settingsOpen)
            { if (root.sheet) root.sheet.jumpSection(delta); }
        else if (root.intake && root.listed)
            root.cursor = delta > 0 ? root.entries.length - 1 : 0;
        else if (!root.intake)
            list.jumpSection(delta);
    }
    function page(delta) {
        if (root.batch)
            { if (root.card && root.editing.length === 0) root.card.page(delta); }
        else if (root.settingsOpen)
            { if (root.sheet) root.sheet.page(delta); }
        else if (root.intake && root.listed)
            root.cursor = Logic.clamp(root.cursor + delta * 8, 0, root.entries.length - 1);
        else if (!root.intake)
            list.page(delta);
    }

    function leaveBatch() {
        root.editing = "";
        root.heldBatch = "";
        root.batchFolder = "";
        if (root.query.length > 0)
            root.queryRequested("");
    }

    function handleEscape() {
        if (root.editing === "batch-dir") {
            root.editing = "";
            root.queryRequested("");
            return true;
        }
        if (root.batch) {
            root.leaveBatch();
            return true;
        }
        if (root.editing.length > 0) {
            root.editing = "";
            root.queryRequested("");
            return true;
        }
        if (root.settingsOpen) {
            root.pageId = "queue";
            if (root.query.length > 0)
                root.queryRequested("");
            return true;
        }
        if (root.intake) {
            root.leaveIntake();
            return true;
        }
        return false;
    }

    function activate() {
        if (root.offline) {
            if (root.downloads) {
                // not-installed is final until asked again
                if (root.engine === "not-installed")
                    root.downloads.state = "idle";
                root.downloads.open();
            }
        } else if (root.editing === "batch-dir") {
            var path = root.query.trim();
            if (path === "~" || path.indexOf("~/") === 0)
                path = root.home + path.slice(1);
            if (path.charAt(0) !== "/") {
                root.alert(I18n.tr("Type the folder's whole path"));
                return;
            }
            root.batchFolder = path;
            root.editing = "";
            root.queryRequested("");
        } else if (root.batch) {
            if (root.card)
                root.card.add(false);
        } else if (root.settingsOpen) {
            if (root.sheet)
                root.sheet.activate();
        } else if (root.intake) {
            if (root.phase === "ready" && !(root.listed && root.picked.length === 0)) {
                root.start(false);
            } else if (root.phase === "error") {
                root.resolvedLink = "";
                root.read();
            }
        } else if (root.task) {
            root.act(Logic.primary(root.task));
        } else if (root.ready && root.tasks.length === 0) {
            const held = String(Quickshell.clipboardText || "");
            const clip = RootLogic.detectLink(held);
            if (clip)
                root.queryRequested(clip.url);
            else if (RootLogic.countLinks(held) >= 1)
                root.queryRequested(held.replace(/\s+/g, " ").trim());
            else
                root.alert(I18n.tr("The clipboard does not hold a link"));
        }
    }

    function activateSecondary() {
        if (root.settingsOpen || root.offline)
            return;
        if (root.batch) {
            if (root.card && root.editing.length === 0)
                root.card.spread();
        } else if (root.intake) {
            if (root.phase !== "ready")
                return;
            if (root.listed)
                root.tick(root.cursor);
            else
                root.act("format-next");
        } else if (root.task) {
            root.act("reveal");
        }
    }

    function tick(index) {
        if (index < 0 || index >= root.entries.length)
            return;
        root.picked = Logic.toggle(root.picked, index);
    }

    // a row that was acted on is the hand's choice: it keeps the selection when
    // the act moves it to another section
    function keep() {
        if (list.selectedIndex >= 0)
            list.select(list.selectedIndex, false);
        return true;
    }

    function act(id) {
        const d = root.downloads;
        if (!d)
            return;
        const task = root.task;
        if (id.indexOf("batch-") === 0) {
            const card = root.card;
            if (!card)
                return;
            if (id === "batch-tick") {
                card.tick();
            } else if (id === "batch-row-next" || id === "batch-row-previous") {
                card.stepRow(id === "batch-row-next" ? 1 : -1);
            } else if (id === "batch-all-next" || id === "batch-all-previous") {
                card.stepAll(id === "batch-all-next" ? 1 : -1);
            } else if (id === "batch-paused") {
                card.add(true);
            } else if (id === "batch-all" || id === "batch-none") {
                card.tickAll(id === "batch-all");
            } else if (id === "batch-copy") {
                if (card.row) {
                    Quickshell.clipboardText = String(card.row.raw);
                    root.hud(I18n.tr("Link copied"));
                }
            } else if (id === "batch-dir") {
                // the field is lent to the folder; the links wait in the card
                if (root.typedMany)
                    root.heldBatch = root.query;
                root.editing = "batch-dir";
                root.queryRequested(root.tilde(card.dir));
            }
        } else if (id === "settings") {
            root.editing = "";
            root.pageId = root.settingsOpen ? "queue" : "settings";
            if (root.query.length > 0)
                root.queryRequested("");
        } else if (id === "step-up" || id === "step-down") {
            if (root.sheet)
                root.sheet.step(id === "step-up" ? 1 : -1);
        } else if (id === "format-next" || id === "format-previous") {
            if (root.chipSet.length > 0)
                root.chip = (root.chip + (id === "format-next" ? 1 : -1) + root.chipSet.length) % root.chipSet.length;
        } else if (id === "pick-all") {
            root.picked = Logic.everything(root.entries.length);
        } else if (id === "pick-none") {
            root.picked = [];
        } else if (id === "add-paused") {
            root.start(true);
        } else if (id === "copy-typed") {
            Quickshell.clipboardText = root.link;
            root.hud(I18n.tr("Link copied"));
        } else if (id === "pause-all") {
            d.pause();
            root.hud(I18n.tr("All paused"));
        } else if (id === "resume-all") {
            d.resume();
            root.hud(I18n.tr("All resumed"));
        } else if (id === "clear") {
            d.clear();
            root.hud(I18n.tr("Finished downloads cleared"));
        } else if (!task) {
            return;
        } else if (root.keep() && id === "pause") {
            d.pause(task.id);
        } else if (id === "resume") {
            d.resume(task.id);
        } else if (id === "retry") {
            d.retry(task.id, function(result, error) {
                if (error)
                    root.alert(root.reasonOf(error));
            });
        } else if (id === "open") {
            d.call("open", { id: task.id }, function(result, error) {
                if (error)
                    root.alert(root.reasonOf(error));
                else
                    root.closeRequested();
            });
        } else if (id === "reveal") {
            d.reveal(task.id);
            root.closeRequested();
        } else if (id === "copy") {
            Quickshell.clipboardText = String(task.url || "");
            root.hud(I18n.tr("Link copied"));
        } else if (id === "cancel") {
            d.cancel(task.id);
            root.hud(I18n.tr("Cancelled"));
        } else if (id === "remove") {
            d.remove(task.id, false);
            root.hud(I18n.tr("Removed from the list"));
        } else if (id === "remove-file") {
            d.remove(task.id, true);
            root.hud(I18n.tr("Removed, file deleted"));
        }
    }

    function adopt() {
        const a = root.args || ({});
        if (a.query !== undefined)
            root.queryRequested(String(a.query));
        const many = a.intake === true && a.urls ? Array.from(a.urls).map(url => String(url || "")).filter(url => url.length > 0) : [];
        if (many.length > 1) {
            root.pageId = "queue";
            root.handedLink = "";
            root.batchFolder = "";
            // the paste itself, when it came along, keeps each row's original link
            root.heldBatch = String(a.text || "").length > 0 ? String(a.text) : many.join("\n");
        } else if (many.length === 1) {
            root.pageId = "queue";
            root.handedLink = many[0];
        } else if (a.intake === true && String(a.url || "").length > 0) {
            root.pageId = "queue";
            root.handedLink = String(a.url);
        } else if (String(a.task || "").length > 0) {
            root.pageId = "queue";
            root.handedLink = "";
            root.wanted = String(a.task);
            Qt.callLater(root.take);
        }
    }
    onArgsChanged: root.adopt()
    Component.onCompleted: root.adopt()
    onActiveChanged: {
        if (root.active && root.downloads)
            root.downloads.open();
        if (root.active)
            root.sample();
    }
    onDownloadsChanged: {
        if (root.active && root.downloads)
            root.downloads.open();
    }
    onFilterChanged: list.selectFirst()

    // ---- what the selection says in the pane ----
    function clock(stamp) {
        const value = Logic.stamp(stamp);
        return value > 0 ? Qt.formatTime(new Date(value), Qt.locale().timeFormat(Locale.ShortFormat)) : "";
    }

    // three figures, chosen by what the transfer is doing
    readonly property var paneFacts: {
        const t = root.task;
        if (!t)
            return [];
        const state = String(t.state);
        const sized = Number(t.size) > 0;
        const torrent = t.kind === "torrent";
        const pair = Format.bytePair(t.done, t.size);
        const landed = { label: I18n.tr("Landed"), value: sized ? I18n.tr("%1 of %2").arg(pair[0]).arg(pair[1]) : Format.bytes(t.done) };
        const started = { label: I18n.tr("Started"), value: root.clock(t.started) };
        const sent = { label: I18n.tr("Sent"), value: I18n.tr("%1, ratio %2").arg(Format.bytes(t.up)).arg(Number(t.ratio || 0).toFixed(2)) };
        var out;
        if (state === "done") {
            const took = (Logic.stamp(t.finished) - Logic.stamp(t.started)) / 1000;
            out = [{ label: I18n.tr("Size"), value: Format.bytes(sized ? t.size : t.done) },
                { label: I18n.tr("Finished"), value: root.clock(t.finished) },
                torrent ? sent : { label: I18n.tr("Took"), value: took >= 1 ? Format.duration(took) : "" }];
        } else if (state === "seeding") {
            out = [sent, { label: I18n.tr("Sending"), value: Format.speed(t.upSpeed) }, { label: I18n.tr("Finished"), value: root.clock(t.finished) }];
        } else if (state === "active") {
            out = [landed, { label: I18n.tr("Left"), value: root.timeLeft(t) }, torrent ? { label: I18n.tr("Ratio"), value: Number(t.ratio || 0).toFixed(2) } : started];
        } else if (state === "queued" || state === "resolving") {
            out = [{ label: I18n.tr("Size"), value: sized ? Format.bytes(t.size) : "" }, { label: I18n.tr("Added"), value: root.clock(t.created) }];
        } else if (state === "error" && !(Number(t.done) > 0)) {
            out = [started];
        } else {
            out = [landed, started];
        }
        return out.filter(fact => String(fact.value || "").length > 0);
    }
    readonly property var paneFiles: {
        const t = root.task;
        if (!t)
            return [];
        const files = t.files || [];
        if (t.kind === "torrent" && files.length > 0)
            return files.map(file => ({
                name: String(file.path || ""), selected: file.selected !== false,
                note: file.selected === false ? I18n.tr("Skipped") : root.join([Format.percent(file.done, file.size), Format.bytes(file.size)])
            }));
        if (String(t.name || "").length > 0)
            return [{ name: String(t.name), glyph: Logic.glyph(t), note: Number(t.size) > 0 ? Format.bytes(t.size) : "" }];
        return [];
    }
    readonly property string paneHeadline: {
        const t = root.task;
        if (!t)
            return "";
        if (t.kind === "torrent")
            return I18n.tr("%1 peers, %2 seeds").arg(t.peers || 0).arg(t.seeds || 0);
        const state = String(t.state);
        if (state === "merging")
            return I18n.tr("Putting the parts together");
        return state === "active" && Number(t.conns) > 0 ? I18n.tr("%1 connections").arg(t.conns) : "";
    }

    Component {
        id: footComponent

        StatusFoot {
            s: root.s
            stats: root.stats
            note: root.footNote
            counts: root.footCounts
        }
    }

    ResultList {
        id: list
        x: Math.round(10 * root.s)
        y: Math.round(10 * root.s)
        width: root.stacked ? parent.width - 20 * root.s : Math.round((parent.width - 30 * root.s) * 0.47)
        height: root.stacked ? Math.round((parent.height - 30 * root.s) * 0.42) : parent.height - 20 * root.s
        visible: root.paned
        s: root.s
        sections: root.sections
        maximumHeight: list.height
        rowDelegate: QueueRow {}
        onMenuRequested: root.menuRequested()
        // a link's card has the keys; the queue steps back
        opacity: root.intake ? 0.55 : 1
        Behavior on opacity {
            NumberAnimation { duration: Look.dur(Look.stateMs) }
        }
    }

    // a filter that leaves nothing, beside a card or a pane that still stands
    Line {
        x: Math.round(20 * root.s)
        y: Math.round(18 * root.s)
        width: list.width - x * 2
        visible: root.paned && list.count === 0
        s: root.s
        size: 13.5
        text: root.filter.length > 0 ? I18n.tr("Nothing in the queue matches \u201c%1\u201d").arg(root.filter) : I18n.tr("Nothing in the queue yet")
        ink: Lithic.fg2
        wrapMode: Text.WordWrap
        maximumLineCount: 3
    }

    // the pane and a link's card share one place; whichever takes it lands the
    // design's way: Lithic and Fluent settle at once, Material springs up a
    // little, Adwaita slides
    Item {
        id: paneSlot
        x: root.stacked ? Math.round(10 * root.s) : list.x + list.width + Math.round(10 * root.s)
        y: root.stacked ? list.y + list.height + Math.round(10 * root.s) : Math.round(10 * root.s)
        width: parent.width - x - Math.round(10 * root.s)
        height: parent.height - y - Math.round(10 * root.s)
        opacity: Math.min(1, 0.25 + root.landed)
        transform: Translate { y: Look.lithic || Look.fluent ? 0 : Math.round(10 * root.s) * (1 - root.landed) }
        scale: Look.fluent ? 0.98 + 0.02 * Math.min(1, root.landed) : 1

        DetailPane {
            width: parent.width
            height: Math.min(parent.height, implicitHeight)
            visible: root.paned && !root.intake && root.task !== null
            s: root.s
            compact: root.stacked
            owner: root.taskId
            title: root.task ? Logic.nameOf(root.task) : ""
            sub: root.task ? root.join([root.task.source ? String(root.task.source.name || "") : "", root.task.mode === "audio" ? I18n.tr("audio only") : root.task.mode === "mute" ? I18n.tr("no sound") : "", root.task.format ? String(root.task.format.label || "") : "", root.tilde(root.task.dir)]) : ""
            badge: {
                const t = root.task;
                if (!t)
                    return "";
                const state = String(t.state);
                if (state === "done")
                    return I18n.tr("Done");
                if (state === "error")
                    return I18n.tr("Failed");
                if (state === "queued")
                    return I18n.tr("Queued");
                if (state === "seeding")
                    return I18n.tr("Seeding");
                const percent = Format.percent(t.done, t.size);
                return percent.length > 0 ? percent.slice(0, -1) : state === "paused" ? I18n.tr("Paused") : "";
            }
            badgeUnit: root.task && ["done", "error", "queued", "seeding"].indexOf(String(root.task.state)) < 0 && Format.percent(root.task.done, root.task.size).length > 0 ? "%" : ""
            badgeTone: root.task ? (String(root.task.state) === "seeding" ? "done" : Logic.tone(String(root.task.state))) : ""
            tone: root.task ? Logic.tone(String(root.task.state)) : ""
            fills: root.task ? root.fillsOf(root.task) : []
            // a file lands in runs, so a bucket fills from its left; a torrent's pieces do not
            swept: root.task !== null && root.task.kind !== "torrent"
            mapped: root.task !== null && String(root.task.state) !== "error" && String(root.task.state) !== "resolving"
            headline: root.paneHeadline
            samples: root.samples
            traced: root.task !== null && ["active", "seeding", "merging"].indexOf(String(root.task.state)) >= 0
            rate: root.task ? Format.speed(String(root.task.state) === "seeding" ? root.task.upSpeed : root.task.speed) : ""
            facts: root.paneFacts
            reason: root.task ? root.reasonOf(root.task.error) : ""
            recovery: root.task ? root.detailOf(root.task.error) : ""
            files: root.paneFiles
            pickable: root.task !== null && root.task.kind === "torrent" && ["done", "error"].indexOf(String(root.task.state)) < 0 && (root.task.files || []).length > 1
            onFileToggled: index => {
                const files = root.task.files || [];
                const chosen = [];
                for (var i = 0; i < files.length; i++) {
                    if ((files[i].selected !== false) !== (i === index))
                        chosen.push(i);
                }
                if (chosen.length === 0) {
                    root.alert(I18n.tr("A torrent needs one file at least"));
                    return;
                }
                root.downloads.files(root.task.id, chosen, function(result, error) {
                    if (error)
                        root.alert(root.reasonOf(error));
                });
            }
        }

        IntakeCard {
            anchors.fill: parent
            visible: root.paned && root.intake
            s: root.s
            phase: root.phase
            link: root.link
            title: root.resolved ? String(root.resolved.title || root.resolved.name || root.link) : ""
            sub: root.resolved ? root.join([String(root.resolved.author || ""), root.resolved.source ? String(root.resolved.source.name || "") : ""]) : ""
            thumb: root.resolved ? String(root.resolved.thumb || "") : ""
            glyph: root.resolved ? Logic.glyph({ kind: root.resolved.kind, name: root.resolved.name, mime: root.resolved.mime }) : "link"
            duration: root.resolved && !root.listed ? Format.duration(root.resolved.duration) : ""
            chips: root.chipSet.map(chip => ({ id: chip.id, text: root.chipText(chip) }))
            chip: root.chip
            listTitle: root.resolved && (root.resolved.files || []).length > 0 ? I18n.tr("Files") : I18n.tr("In this link")
            listNote: root.listed ? I18n.tr("%1 of %2").arg(root.picked.length).arg(root.entries.length) : ""
            entries: root.entries
            picked: root.picked
            cursor: root.cursor
            facts: {
                const r = root.resolved;
                if (!r)
                    return [];
                const out = [];
                if (String(r.name || "").length > 0 && !root.listed)
                    out.push({ label: I18n.tr("File"), value: String(r.name) });
                out.push({ label: I18n.tr("Folder"), value: root.tilde(root.settings.dir) });
                const size = root.estimate();
                const chosen = root.chipSet.length > 0 ? root.chipSet[Math.min(root.chip, root.chipSet.length - 1)] : null;
                if (chosen && chosen.kind !== "video")
                    out.push({ label: I18n.tr("Takes"), value: chosen.kind === "audio" ? I18n.tr("the sound alone, %1").arg(chosen.detail) : I18n.tr("%1, the most your settings allow").arg(chosen.detail) });
                const perItem = root.listed && (r.files || []).length === 0;
                if (size > 0 && !perItem)
                    out.push({ label: I18n.tr("Size"), value: chosen ? I18n.tr("about %1").arg(Format.bytes(size)) : Format.bytes(size) });
                if (r.kind === "file")
                    out.push({ label: I18n.tr("Resume"), value: r.resumable ? I18n.tr("can pick up where it stopped") : I18n.tr("starts over if interrupted") });
                return out;
            }
            note: root.resolved ? String(root.resolved.note || "") : ""
            reason: root.reasonOf(root.failure)
            recovery: root.detailOf(root.failure)
            onChipPicked: index => root.chip = index
            onEntryToggled: index => {
                root.cursor = index;
                root.tick(index);
            }
        }
    }

    // several links take the whole stage
    Loader {
        id: cardLoader
        width: parent.width
        active: root.batch
        visible: root.batch
        opacity: Math.min(1, 0.25 + root.landed)
        sourceComponent: BatchCard {
            s: root.s
            downloads: root.downloads
            text: root.batchText
            live: root.active && root.batch
            settings: root.settings
            folder: root.batchFolder
            home: root.home
            maximumHeight: root.maximumHeight
            onHud: text => root.hud(text)
            onAlert: text => root.alert(text)
            onAdded: (tasks, paused) => {
                root.hud(paused ? I18n.tr("%1 added, paused").arg(tasks.length) : I18n.tr("%1 downloads started").arg(tasks.length));
                root.wanted = tasks.length > 0 ? String(tasks[0].id || "") : "";
                root.leaveBatch();
                Qt.callLater(root.take);
            }
        }
    }

    // built when asked for, gone when left
    Loader {
        id: sheetLoader
        width: parent.width
        active: root.settingsOpen
        visible: root.settingsOpen
        opacity: Math.min(1, 0.25 + root.landed)
        sourceComponent: SettingsSheet {
            s: root.s
            settings: root.settings
            query: root.query
            editing: root.editing
            maximumHeight: root.maximumHeight
            onPatched: patch => {
                if (!root.downloads)
                    return;
                root.downloads.setSettings(patch, function(result, error) {
                    if (error)
                        root.alert(root.reasonOf(error));
                });
            }
            onEditRequested: (id, text) => {
                root.editing = id;
                root.queryRequested(text);
            }
            onRefused: text => root.alert(text)
        }
    }

    // nothing to list: say why, and what one key does about it
    Item {
        id: blank
        width: parent.width
        visible: !root.paned && !root.settingsOpen && !root.batch
        implicitHeight: blankColumn.implicitHeight + Math.round(34 * root.s)
        height: implicitHeight

        Column {
            id: blankColumn
            x: Math.round(18 * root.s)
            y: Math.round(16 * root.s)
            width: parent.width - x * 2
            spacing: Math.round(6 * root.s)

            Line {
                width: parent.width
                s: root.s
                size: 15
                font.weight: Font.Medium
                text: root.engine === "not-installed" ? I18n.tr("The download engine is not installed")
                    : root.engine === "error" ? I18n.tr("The download engine did not start")
                    : !root.ready ? I18n.tr("Starting the download engine")
                    : I18n.tr("Paste a link to start")
                wrapMode: Text.WordWrap
            }
            Line {
                width: parent.width
                s: root.s
                size: 12.5
                ink: Lithic.fg2
                text: root.engine === "not-installed" ? I18n.tr("It ships with the Ryoku shell as ryoget. Update Ryoku, then try again.")
                    : root.engine === "error" ? I18n.tr("Its log is %1.").arg(root.tilde((String(Quickshell.env("XDG_STATE_HOME") || "") || root.home + "/.local/state") + "/ryoku/ryoget/ryoget.log"))
                    : !root.ready ? ""
                    : I18n.tr("A file, a video or post, a torrent or a magnet. It lands in %1.").arg(root.tilde(root.settings.dir))
                visible: text.length > 0
                wrapMode: Text.WordWrap
                maximumLineCount: 3
            }
            Busy {
                visible: !root.ready && !root.offline
                s: root.s
            }
            Row {
                visible: root.ready
                topPadding: Math.round(6 * root.s)
                spacing: Math.round(8 * root.s)

                KeyHint { anchors.verticalCenter: parent.verticalCenter; s: root.s; hint: "Ctrl+V" }
                Line { anchors.verticalCenter: parent.verticalCenter; s: root.s; size: 12.5; text: I18n.tr("paste"); ink: Lithic.fg2 }
                Item { width: Math.round(8 * root.s); height: 1 }
                KeyHint { anchors.verticalCenter: parent.verticalCenter; s: root.s; hint: "Enter" }
                Line { anchors.verticalCenter: parent.verticalCenter; s: root.s; size: 12.5; text: I18n.tr("take the link on the clipboard"); ink: Lithic.fg2 }
                Item { width: Math.round(8 * root.s); height: 1 }
                KeyHint { anchors.verticalCenter: parent.verticalCenter; s: root.s; hint: "Ctrl+," }
                Line { anchors.verticalCenter: parent.verticalCenter; s: root.s; size: 12.5; text: I18n.tr("settings"); ink: Lithic.fg2 }
            }
        }
    }
}
