pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import ".."
import "../bar"
import "../look"
import "../lib/root.js" as RootLogic
import "../lib/format.js" as Format
import "../../launcher/shared/providers/calc/calc.js" as Calc

// The bar's first list. Empty, it shows what is moving and what finished or failed since
// you last looked (downloads), what you asked today and where to go. Typed, it reads the
// line: a link leads with Download, a sum with its result, commands filter, and two
// fallbacks always close it (Ask Hyakki, Search the web).
Item {
    id: root

    required property real s
    property bool active: false
    property string query: ""
    property var args: ({})
    property real maximumHeight: 480 * s
    property HyakkiClient hyakki: null
    property HyakkiSession session: null
    property DownloadsClient downloads: null

    readonly property string title: ""
    readonly property string placeholder: I18n.tr("Ask, search or paste a link")
    readonly property real preferredWidth: 0
    readonly property string status: ""
    readonly property Component statusItem: root.session && root.session.forgetting.length > 0 ? undoLine : null
    // the engine's own words on a failed download, for the Action Panel
    readonly property string actionNote: root.current && root.current.kind === "task" && root.current.task.state === "error" && root.downloads
        ? root.downloads.detail(root.current.task.error) : ""

    readonly property string text: root.query.trim()
    // the list's clock, so what it shows ages while the bar stays open
    property real now: Date.now()
    readonly property int askWindow: 24 * 60 * 60 * 1000
    readonly property var homeTasks: root.downloads ? RootLogic.pickDownloads(root.downloads.tasks, root.now, 3) : []
    readonly property int homeSettled: root.homeTasks.filter(task => root.settled(task.state)).length
    readonly property var unseenIds: root.downloads ? RootLogic.unseen(root.downloads.tasks, root.now) : []
    // a string, so the progress ticks of a moving download leave the mark timer alone
    readonly property string unseenKey: root.unseenIds.join(",")
    property string markedFor: ""
    readonly property string hyakkiState: root.hyakki ? root.hyakki.state : "off"
    readonly property bool hyakkiReady: root.hyakkiState === "ready"
    readonly property bool hyakkiMissing: root.hyakkiState === "off" || root.hyakkiState === "not-installed"
    // several links in the field: the engine reads the text (links), and its
    // answer for exactly this text decides what the first row is
    readonly property bool manyMaybe: RootLogic.countLinks(root.text) >= 2
    property string readFor: ""
    property var read: null
    property var readError: null
    // Enter or Ctrl+Enter pressed before the answer came: "add" | "review"
    property string afterRead: ""
    property bool adding: false
    readonly property bool readCurrent: root.manyMaybe && root.readFor === root.text
    readonly property var many: !root.manyMaybe ? null
        : !root.readCurrent ? { state: "reading" }
        : root.readError ? { state: "failed", error: root.readError }
        : root.read && root.read.count >= 2 ? { state: "ready", batch: root.read } : null
    readonly property var link: root.readCurrent && !root.readError && root.read && root.read.count === 1
        ? root.read.links[0] : RootLogic.detectLink(root.text)
    property string calcExpr: ""
    property string calcResult: ""

    readonly property var commands: {
        const list = [
            {
                id: "ask", title: I18n.tr("Ask Hyakki"), glyph: "explore", hint: "\\",
                keywords: [I18n.tr("question"), I18n.tr("answer"), I18n.tr("assistant")],
                subtitle: root.hyakkiState === "off" ? I18n.tr("Hyakki is off")
                    : root.hyakkiState === "not-installed" ? I18n.tr("Hyakki is not installed") : ""
            },
            { id: "web", title: I18n.tr("Search the web"), glyph: "language", hint: "?", keywords: [I18n.tr("internet"), I18n.tr("find"), I18n.tr("look up")], subtitle: "" },
            { id: "downloads", title: I18n.tr("Downloads"), glyph: "download", hint: "/", keywords: [I18n.tr("torrent"), I18n.tr("video"), I18n.tr("queue"), I18n.tr("files")], subtitle: "" }
        ];
        if (root.hyakkiState !== "not-installed")
            list.push({ id: "hyakki", title: I18n.tr("Open Hyakki"), glyph: "open_in_new", hint: "", keywords: [I18n.tr("app"), I18n.tr("agents"), I18n.tr("chat")], subtitle: "" });
        return list;
    }

    readonly property var sections: {
        const built = RootLogic.sections({
            query: root.text,
            commands: root.commands,
            link: root.link,
            many: root.many,
            calc: root.calcExpr.length > 0 ? { expr: root.calcExpr, result: root.calcResult } : null,
            downloads: root.homeTasks.map(root.taskRow),
            asks: root.session ? RootLogic.pickAsks(root.session.sessions, root.now, root.askWindow, root.session.forgetting).map(root.askRow) : [],
            hyakki: root.hyakkiState,
            labels: {
                downloads: I18n.tr("Downloads"), asks: I18n.tr("Recent asks"), goTo: I18n.tr("Go to"),
                results: I18n.tr("Commands"), link: I18n.tr("Link"), links: I18n.tr("Links"), calc: I18n.tr("Calculator"),
                fallbacks: ""
            }
        });
        return built.map(section => ({ id: section.id, title: section.title, rows: section.rows.map(root.display) }));
    }

    readonly property var current: list.current
    readonly property var primaryAction: {
        const row = root.current;
        if (!row)
            return null;
        if (row.kind === "command")
            return { label: row.command.id === "ask" && root.hyakkiMissing ? I18n.tr("Open Hyakki settings") : row.title };
        if (row.kind === "link")
            return { label: I18n.tr("Download") };
        if (row.kind === "many")
            return row.many.state === "failed" ? null : { label: I18n.tr("Download all") };
        if (row.kind === "calc")
            return { label: I18n.tr("Copy result") };
        if (row.kind === "task")
            return { label: row.task.state === "done" ? I18n.tr("Open file") : I18n.tr("Show in Downloads") };
        if (row.kind === "ask")
            return { label: I18n.tr("Open answer") };
        if (row.kind === "fallback")
            return { label: row.target === "ask" ? (root.hyakkiMissing ? I18n.tr("Open Hyakki settings") : I18n.tr("Ask Hyakki")) : I18n.tr("Search the web") };
        return null;
    }
    readonly property var secondaryAction: !root.current ? null
        : root.current.kind === "link" ? { label: I18n.tr("Choose format and folder"), hint: "Ctrl+Enter" }
        : root.current.kind === "many" && root.current.many.state !== "failed" ? { label: I18n.tr("Review them in Downloads"), hint: "Ctrl+Enter" }
        : null
    readonly property var actions: {
        const row = root.current;
        const out = [];
        if (!row)
            return out;
        if (row.kind === "link") {
            out.push({ id: "link-open", label: I18n.tr("Open in the browser"), glyph: "open_in_browser", hint: "Ctrl+O", section: "" });
            out.push({ id: "link-copy", label: I18n.tr("Copy link"), glyph: "content_copy", hint: "Ctrl+Shift+C", section: "" });
        } else if (row.kind === "many" && row.many.state === "ready") {
            out.push({ id: "many-copy", label: I18n.tr("Copy clean links"), glyph: "content_copy", hint: "Ctrl+Shift+C", section: "" });
        } else if (row.kind === "calc") {
            out.push({ id: "calc-copy-line", label: I18n.tr("Copy with the sum"), glyph: "content_copy", hint: "Ctrl+Shift+C", section: "" });
        } else if (row.kind === "task") {
            const state = String(row.task.state);
            if (state === "done") {
                out.push({ id: "task-show", label: I18n.tr("Show in Downloads"), glyph: "download", hint: "", section: "" });
                out.push({ id: "task-reveal", label: I18n.tr("Show in folder"), glyph: "folder_open", hint: "Ctrl+O", section: "" });
            } else if (state === "paused") {
                out.push({ id: "task-resume", label: I18n.tr("Resume"), glyph: "play_arrow", hint: "Ctrl+P", section: "" });
            } else if (state === "error" || state === "stopped") {
                // a page only a browser gets past is finished in one
                if (state === "error" && row.task.error && String(row.task.error.code || "") === "browser_check")
                    out.push({ id: "task-browser", label: I18n.tr("Open in the browser"), glyph: "open_in_browser", hint: "Ctrl+O", section: "" });
                out.push({ id: "task-retry", label: I18n.tr("Try again"), glyph: "refresh", hint: "Ctrl+R", section: "" });
            } else {
                out.push({ id: "task-pause", label: I18n.tr("Pause"), glyph: "pause", hint: "Ctrl+P", section: "" });
            }
            out.push({ id: "task-copy", label: I18n.tr("Copy link"), glyph: "content_copy", hint: "Ctrl+Shift+C", section: "" });
            if (root.settled(state))
                out.push({ id: "task-dismiss", label: I18n.tr("Clear from this list"), glyph: "playlist_remove", hint: "Delete", section: I18n.tr("Remove") });
            if (state === "done" || state === "error" || state === "stopped")
                out.push({ id: "task-remove", label: I18n.tr("Remove from Downloads"), glyph: "close", hint: "", section: I18n.tr("Remove"), destructive: true });
            else
                out.push({ id: "task-cancel", label: I18n.tr("Cancel and delete the partial file"), glyph: "delete", hint: "", section: I18n.tr("Remove"), destructive: true });
            if (root.homeSettled > 1)
                out.push({ id: "news-dismiss", label: I18n.tr("Clear every finished download from this list"), glyph: "clear_all", hint: "Shift+Delete", section: I18n.tr("Remove") });
        } else if (row.kind === "ask") {
            out.push({ id: "ask-hyakki", label: I18n.tr("Open in Hyakki"), glyph: "open_in_new", hint: "Ctrl+O", section: "" });
            out.push({ id: "ask-delete", label: I18n.tr("Delete this ask"), glyph: "delete", hint: "Delete", section: I18n.tr("Remove"), destructive: true });
        }
        if (root.session && root.session.forgetting.length > 0)
            out.push({ id: "ask-undo", label: I18n.tr("Undo the delete"), glyph: "undo", hint: root.text.length === 0 ? "Ctrl+Z" : "", section: "" });
        return out;
    }

    signal pushRequested(string view, var args)
    signal closeRequested()
    signal hud(string text)
    signal alert(string text)
    signal queryRequested(string text)
    signal menuRequested()

    implicitHeight: list.implicitHeight

    function etaText(seconds) {
        const eta = Format.eta(seconds);
        if (!eta)
            return "";
        if (eta.unit === "s")
            return I18n.tr("%1 s left").arg(eta.a);
        if (eta.unit === "m")
            return eta.b > 0 ? I18n.tr("%1 min %2 s left").arg(eta.a).arg(eta.b) : I18n.tr("%1 min left").arg(eta.a);
        if (eta.unit === "h")
            return I18n.tr("%1 h %2 min left").arg(eta.a).arg(eta.b);
        return I18n.tr("%1 d %2 h left").arg(eta.a).arg(eta.b);
    }

    function agoText(stamp) {
        const value = Date.parse(String(stamp || ""));
        if (isNaN(value))
            return "";
        const ago = Format.ago(value, Date.now());
        if (!ago || ago.unit === "now")
            return I18n.tr("Just now");
        if (ago.unit === "m")
            return I18n.tr("%1m ago").arg(ago.n);
        if (ago.unit === "h")
            return I18n.tr("%1h ago").arg(ago.n);
        return I18n.tr("%1d ago").arg(ago.n);
    }

    function join(parts) {
        return parts.filter(part => String(part || "").length > 0).join(" \u00b7 ");
    }

    function taskRow(task) {
        const state = String(task.state || "");
        const fraction = Format.fraction(task.done, task.size);
        const percent = Format.percent(task.done, task.size);
        var glyph = task.kind === "torrent" ? "hub" : task.kind === "media" ? "movie" : "draft";
        var tone = "";
        var line = "";
        if (state === "active") {
            line = root.join([percent, task.size > 0 ? I18n.tr("%1 of %2").arg(Format.bytes(task.done)).arg(Format.bytes(task.size)) : Format.bytes(task.done), root.etaText(task.eta)]);
        } else if (state === "resolving") {
            line = I18n.tr("Reading the link");
        } else if (state === "merging") {
            line = I18n.tr("Putting video and sound together");
        } else if (state === "queued") {
            line = I18n.tr("Waiting its turn");
        } else if (state === "paused") {
            line = root.join([I18n.tr("Paused"), percent]);
            tone = "paused";
        } else if (state === "seeding") {
            line = root.join([I18n.tr("Seeding"), I18n.tr("ratio %1").arg(Number(task.ratio || 0).toFixed(2))]);
            tone = "done";
        } else if (state === "done") {
            line = root.join([Format.bytes(task.size > 0 ? task.size : task.done), String(task.source && task.source.name || ""), root.agoText(task.finished)]);
            tone = "done";
            glyph = "download_done";
        } else if (state === "error") {
            line = root.downloads.reason(task.error);
            tone = "error";
        } else {
            line = I18n.tr("Stopped");
            tone = "paused";
        }
        const moving = state === "active" || state === "merging" || state === "seeding";
        return {
            id: "task:" + task.id, kind: "task", task: task, size: "task",
            glyph: glyph,
            glyphTone: state === "error" ? "error" : state === "done" || state === "seeding" ? "ok"
                : state === "paused" || state === "stopped" ? "wait" : "busy",
            title: String(task.title || task.name || task.url || ""),
            subtitle: line, tone: state === "error" ? "error" : "",
            trail: state === "active" ? Format.speed(task.speed) : state === "seeding" ? Format.speed(task.upSpeed) : "",
            trailMono: true,
            // nothing landed, nothing to map
            transfer: state === "done" || (state === "error" && !(Number(task.done) > 0)) ? undefined : {
                fraction: fraction, pieces: Format.pieces(task.pieces),
                active: moving && Number(task.speed || task.upSpeed || 0) > 0, tone: tone
            }
        };
    }

    function askRow(info) {
        return {
            id: "ask:" + info.id, kind: "ask", session: String(info.id || ""), size: "one", glyph: "history",
            title: String(info.title || info.preview || info.id || "").replace(/\s+/g, " "),
            trail: root.agoText(info.updated)
        };
    }

    function display(row) {
        if (row.kind === "command") {
            const command = row.command;
            return { id: row.id, kind: "command", command: command, size: "one", glyph: command.glyph, title: command.title, subtitle: command.subtitle, hint: command.hint };
        }
        if (row.kind === "link") {
            const link = row.link;
            const what = link.kind === "magnet" ? (link.name.length > 0 ? link.name : I18n.tr("magnet link")) : link.host;
            return {
                id: row.id, kind: "link", link: link, size: "two", glyphTone: "accent",
                glyph: link.kind === "url" ? "download" : "hub",
                title: I18n.tr("Download %1").arg(what),
                subtitle: link.kind === "url" ? link.url : link.kind === "torrent" ? I18n.tr("Torrent file") : I18n.tr("Torrent")
            };
        }
        if (row.kind === "many")
            return root.manyRow(row);
        if (row.kind === "calc")
            return { id: row.id, kind: "calc", calc: row.calc, size: "calc", glyph: "calculate", title: row.calc.result, subtitle: row.calc.expr };
        if (row.kind === "fallback") {
            const ask = row.target === "ask";
            return {
                id: row.id, kind: "fallback", target: row.target, text: row.text, size: "one",
                glyph: ask ? "explore" : "language",
                title: ask ? I18n.tr("Ask Hyakki \u201c%1\u201d").arg(RootLogic.quote(row.text, 44))
                    : I18n.tr("Search the web for \u201c%1\u201d").arg(RootLogic.quote(row.text, 44)),
                subtitle: ask && root.hyakkiMissing ? (root.hyakkiState === "off" ? I18n.tr("Hyakki is off") : I18n.tr("Hyakki is not installed")) : "",
                hint: ask ? "Tab" : ""
            };
        }
        return row;
    }

    function copy(text) {
        Quickshell.clipboardText = String(text);
        root.hud(I18n.tr("Copied"));
    }

    function openHyakkiSettings() {
        Bus.run("hub open hyakki");
        root.closeRequested();
    }

    function startDownload(link) {
        if (!root.downloads)
            return;
        root.downloads.add({ url: link.url }, function(task, error) {
            if (error) {
                root.alert(root.downloads.reason(error));
                return;
            }
            root.hud(I18n.tr("Download started"));
            root.queryRequested("");
        });
    }

    function move(delta) { list.move(delta); }
    function jumpSection(delta) { list.jumpSection(delta); }
    function page(delta) { list.page(delta); }
    function handleEscape() { return false; }

    // what the engine adjusted, then where the links point
    function manyLine(batch) {
        const parts = [];
        if (batch.cleaned > 0)
            parts.push(I18n.tr("%1 cleaned").arg(batch.cleaned));
        if (batch.duplicates === 1)
            parts.push(I18n.tr("1 duplicate dropped"));
        else if (batch.duplicates > 1)
            parts.push(I18n.tr("%1 duplicates dropped").arg(batch.duplicates));
        const shown = batch.hosts.slice(0, 3).join(", ");
        const hosts = batch.hosts.length > 3 ? I18n.tr("%1 and %2 more").arg(shown).arg(batch.hosts.length - 3) : shown;
        return root.join([parts.join(", "), hosts]);
    }

    function manyRow(row) {
        const many = row.many;
        const base = { id: row.id, kind: "many", many: many, size: "two", glyph: "download", glyphTone: "accent" };
        if (many.state === "ready") {
            base.title = I18n.tr("Download %1 links").arg(many.batch.count);
            base.subtitle = root.manyLine(many.batch);
        } else if (many.state === "failed") {
            base.title = I18n.tr("Download links");
            base.subtitle = root.downloads ? root.downloads.reason(many.error) : "";
            base.tone = "error";
            base.glyphTone = "error";
        } else {
            base.title = I18n.tr("Download links");
            base.subtitle = I18n.tr("Reading what you pasted");
        }
        return base;
    }

    function readLinks() {
        const text = root.text;
        if (!root.manyMaybe || !root.downloads || root.readFor === text)
            return;
        root.downloads.links(text, function(result, error) {
            if (text !== root.text)
                return;
            root.readError = error || null;
            root.read = error ? null : RootLogic.batch(result);
            root.readFor = text;
            const next = root.afterRead;
            root.afterRead = "";
            if (next === "add")
                root.activate();
            else if (next === "review")
                root.activateSecondary();
        });
    }

    function startMany(batch) {
        if (!root.downloads || root.adding)
            return;
        root.adding = true;
        root.downloads.addMany(batch.urls.map(url => ({ url: url })), ({}), function(result, error) {
            root.adding = false;
            if (error) {
                root.alert(root.downloads.reason(error));
                return;
            }
            const started = (result.tasks || []).length;
            const failed = result.errors || [];
            if (started === 0) {
                root.alert(root.downloads.reason(failed.length > 0 ? failed[0] : null));
                return;
            }
            root.queryRequested("");
            if (failed.length > 0)
                root.alert(I18n.tr("%1 started, %2 could not start").arg(started).arg(failed.length));
            else
                root.hud(started === 1 ? I18n.tr("1 download started") : I18n.tr("%1 downloads started").arg(started));
        });
    }

    function activate() {
        const row = root.current;
        if (!row)
            return;
        if (row.kind === "command") {
            const id = row.command.id;
            if (id === "ask" && root.hyakkiMissing)
                root.openHyakkiSettings();
            else if (id === "hyakki") {
                Spawn.run(["hyakki", "open"]);
                root.closeRequested();
            } else {
                root.pushRequested(id, ({}));
            }
        } else if (row.kind === "link") {
            root.startDownload(row.link);
        } else if (row.kind === "many") {
            if (row.many.state === "ready")
                root.startMany(row.many.batch);
            else if (row.many.state === "reading")
                root.afterRead = "add";
        } else if (row.kind === "calc") {
            root.copy(row.calc.result);
        } else if (row.kind === "task") {
            if (row.task.state === "done" && root.downloads) {
                root.downloads.openFile(row.task.id);
                root.closeRequested();
            } else {
                root.pushRequested("downloads", { task: String(row.task.id) });
            }
        } else if (row.kind === "ask") {
            root.pushRequested("ask", { session: row.session });
        } else if (row.kind === "fallback") {
            if (row.target === "ask" && root.hyakkiMissing)
                root.openHyakkiSettings();
            else if (row.target === "ask")
                root.pushRequested("ask", { question: row.text });
            else
                root.pushRequested("web", { query: row.text });
        }
    }

    function activateSecondary() {
        const row = root.current;
        if (!row)
            return;
        if (row.kind === "link")
            root.pushRequested("downloads", { url: row.link.url, intake: true });
        else if (row.kind === "many" && row.many.state === "ready")
            root.pushRequested("downloads", { urls: row.many.batch.urls, text: root.text, intake: true });
        else if (row.kind === "many" && row.many.state === "reading")
            root.afterRead = "review";
    }

    function act(id) {
        const row = root.current;
        if (!row)
            return;
        if (id === "link-open") {
            Spawn.runApp(["xdg-open", row.link.url], { label: "browser" });
            root.closeRequested();
        } else if (id === "link-copy") {
            root.copy(row.link.url);
        } else if (id === "many-copy") {
            if (row.kind === "many" && row.many.state === "ready")
                root.copy(row.many.batch.urls.join("\n"));
        } else if (id === "calc-copy-line") {
            root.copy(row.calc.expr + " = " + row.calc.result);
        } else if (id === "ask-hyakki") {
            Spawn.run(["hyakki", "open", "--session", row.session]);
            root.closeRequested();
        } else if (id === "ask-delete" && root.session) {
            list.keepNear(row.id);
            root.session.forget(row.session);
        } else if (id === "ask-undo") {
            root.undoDelete();
        } else if (id === "news-dismiss" && root.downloads) {
            const settledIds = root.downloads.tasks.filter(task => root.settled(task.state)).map(task => String(task.id));
            root.downloads.seen(settledIds, RootLogic.dismissed(Date.now()));
        } else if (row.kind === "task" && root.downloads) {
            const task = row.task;
            if (id === "task-show")
                root.pushRequested("downloads", { task: String(task.id) });
            else if (id === "task-reveal")
                root.downloads.reveal(task.id);
            else if (id === "task-pause")
                root.downloads.pause(task.id);
            else if (id === "task-resume")
                root.downloads.resume(task.id);
            else if (id === "task-retry")
                root.downloads.retry(task.id);
            else if (id === "task-copy")
                root.copy(task.url);
            else if (id === "task-browser") {
                Spawn.runApp(["xdg-open", String(task.url)], { label: "browser" });
                root.closeRequested();
            } else if (id === "task-dismiss") {
                list.keepNear(row.id);
                root.downloads.seen([String(task.id)], RootLogic.dismissed(Date.now()));
            } else if (id === "task-remove")
                root.downloads.remove(task.id, false);
            else if (id === "task-cancel")
                root.downloads.cancel(task.id);
        }
    }

    function settled(state) {
        return state === "done" || state === "error" || state === "stopped" || state === "paused";
    }

    // the ask comes back where it was, and takes the selection with it
    function undoDelete() {
        if (!root.session || root.session.forgetting.length === 0)
            return;
        const back = root.session.forgetting;
        root.session.undoForget();
        list.selectId("ask:" + back);
    }

    // what stayed on screen a moment has been looked at, and leaves the list a while later
    function markSeen() {
        if (!root.active || root.text.length > 0 || !root.downloads || !root.downloads.ready)
            return;
        if (root.unseenKey.length === 0 || root.unseenKey === root.markedFor)
            return;
        root.markedFor = root.unseenKey;
        root.downloads.seen(root.unseenIds);
    }

    function refresh() {
        if (!root.active)
            return;
        if (root.downloads)
            root.downloads.open();
        if (root.hyakkiReady && root.session)
            root.session.list(20);
    }

    onActiveChanged: {
        if (root.active) {
            root.now = Date.now();
            root.markedFor = "";
            markDelay.restart();
        } else if (root.session) {
            root.session.commitForget();
        }
        root.refresh();
    }
    onHyakkiReadyChanged: root.refresh()
    onUnseenKeyChanged: markDelay.restart()
    onTextChanged: {
        if (root.text.length === 0)
            markDelay.restart();
        list.selectFirst();
        root.afterRead = "";
        // manyMaybe hangs on the same text and may not have caught up here
        if (RootLogic.countLinks(root.text) >= 2) {
            readDelay.restart();
        } else {
            readDelay.stop();
            root.readFor = "";
            root.read = null;
            root.readError = null;
        }
        if (RootLogic.looksLikeMath(root.text)) {
            calcDelay.restart();
        } else {
            calcDelay.stop();
            root.calcExpr = "";
            root.calcResult = "";
        }
    }

    // a paste lands at once, typing a second link settles first
    Timer { id: readDelay; interval: 110; onTriggered: root.readLinks() }
    Timer { id: markDelay; interval: 600; onTriggered: root.markSeen() }
    Timer {
        interval: 30000
        repeat: true
        running: root.active
        onTriggered: root.now = Date.now()
    }

    Component {
        id: undoLine
        UndoLine {
            s: root.s
            text: I18n.tr("Ask deleted")
            keyed: root.text.length === 0
            onUndo: root.undoDelete()
        }
    }

    Timer {
        id: calcDelay
        interval: 90
        onTriggered: {
            calc.running = false;
            calc.expr = root.text;
            calc.out = "";
            calc.running = true;
        }
    }

    Process {
        id: calc
        property string expr: ""
        property string out: ""
        command: ["/bin/sh", "-c", "exec ryoku-cmd-calc \"$1\"", "calc", calc.expr]
        environment: Spawn.env
        stdout: SplitParser { onRead: data => calc.out += data + "\n" }
        onExited: code => {
            if (calc.expr !== root.text)
                return;
            const result = code === 0 ? Calc.parseResult(calc.out) : null;
            // a line that only repeats itself is not a sum
            root.calcResult = result && result !== calc.expr ? result : "";
            root.calcExpr = calc.expr;
        }
    }

    ResultList {
        id: list
        width: parent.width
        height: implicitHeight
        s: root.s
        sections: root.sections
        maximumHeight: root.maximumHeight
        onActivated: root.activate()
        onMenuRequested: root.menuRequested()
    }
}
