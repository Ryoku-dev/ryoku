import QtQuick
import Quickshell.Io
import Ryoku.Ui.Singletons
import "../../Singletons"

// the lock screen design as a Changes store: `ryoku-hub lock list` is the truth,
// `ryoku-hub lock set` the write; Changes owns it so undo works from any page
QtObject {
    id: store

    property var skins: []
    property string active: ""
    property bool loading: true
    property bool loadFailed: false
    property bool listed: false
    // the design being applied, and whether the admin prompt is up for it
    property string pending: ""
    property bool adminPrompt: false
    // the last finished switch: { slug, ok }
    property var result: null

    property var doneCallback: null

    // the lock falls back to Orbital when nothing was ever picked, so undo lands there too
    readonly property string fallbackSlug: "clockwork/orbital"
    readonly property string inUse: store.active !== "" ? store.active
        : store.skins.some(s => s.slug === store.fallbackSlug) ? store.fallbackSlug : ""

    function read(key) {
        if (key === "active")
            return store.inUse;
        return undefined;
    }

    function skin(slug) {
        for (var i = 0; i < store.skins.length; i++)
            if (store.skins[i].slug === slug)
                return store.skins[i];
        return null;
    }

    function nameOf(slug) {
        var s = store.skin(slug);
        return s ? s.name : slug;
    }

    function reload() {
        if (store.listProc.running)
            return;
        store.loading = !store.listed;
        store.listProc.running = true;
    }

    function write(key, v, done) {
        if (key !== "active") {
            done(true, "");
            return;
        }
        var slug = String(v || "");
        if (store.pending !== "") {
            done(false, I18n.tr("Another design is still being applied. Try again when it finishes."));
            return;
        }
        store.pending = slug;
        store.adminPrompt = false;
        store.result = null;
        store.doneCallback = done;
        store.setProc.slug = slug;
        store.setProc.promptShown = false;
        store.setProc.command = ["ryoku-hub", "lock", "set", slug];
        store.setProc.launched = false;
        store.setProc.late = false;
        store.setProc.running = true;
        store.limit.restart();
        Changes.touch();
    }

    // runs once per switch: the stopped process of a hang may still report its exit
    function finished(code) {
        if (store.doneCallback === null)
            return;
        store.limit.stop();
        var slug = store.setProc.slug;
        var late = store.setProc.late;
        var why = setErr.text.trim().replace(/^ryoku-hub:\s*/, "");
        var done = store.doneCallback;
        store.doneCallback = null;
        store.pending = "";
        store.adminPrompt = false;
        store.result = { slug: slug, ok: code === 0 };
        // the design shows as in use only once the privileged install took; every failure leaves
        // the one that was in use before
        if (code === 0) {
            store.active = slug;
            if (done)
                done(true, "");
        } else if (done) {
            var kept = store.inUse !== "" ? store.nameOf(store.inUse) : "";
            var tail = kept !== "" ? " " + I18n.tr("%1 is still your lock screen and sign-in screen.").arg(kept) : "";
            done(false, code === -1 ? Changes.missingText("ryoku-hub")
                : late ? I18n.tr("%1 did not finish in time. Try again.").replace("%1", "ryoku-hub")
                : I18n.tr("Nothing changed: %1.").arg(why || I18n.tr("the settings backend did not answer")) + tail);
        }
        Changes.touch();
        store.reload();
    }

    property Process listProc: Process {
        command: ["ryoku-hub", "lock", "list"]
        stdout: StdioCollector { id: listOut }
        onExited: code => {
            var failed = code !== 0;
            if (!failed) {
                try {
                    var o = JSON.parse(listOut.text || "{}");
                    if (o.error) {
                        failed = true;
                    } else {
                        store.skins = (o.skins || []).map(s => ({
                            slug: s.slug,
                            name: s.name || s.slug,
                            theme: s.theme || "",
                            preview: s.preview || "",
                            summary: s.summary || "",
                            blurb: s.blurb || "",
                            tags: s.tags || []
                        }));
                        // a switch in flight owns the active mark until it finishes
                        if (store.pending === "")
                            store.active = o.active || "";
                    }
                } catch (e) {
                    failed = true;
                }
            }
            store.loadFailed = failed;
            store.loading = false;
            store.listed = store.listed || !failed;
            Changes.touch();
        }
    }

    // until the admin prompt is up a switch is the backend's own work, so a hang ends
    // after 10 s; once the prompt is up it waits for the person, as long as they need
    property Timer limit: Timer {
        interval: 10000
        onTriggered: {
            if (!store.setProc.running || store.setProc.promptShown)
                return;
            store.setProc.late = true;
            store.setProc.running = false;
            store.finished(1);
        }
    }

    property Process setProc: Process {
        property string slug: ""
        property bool promptShown: false
        property bool launched: false
        property bool late: false
        onStarted: store.setProc.launched = true
        // a program that cannot start reports no exit, only that it stopped running
        onRunningChanged: {
            if (!store.setProc.running && !store.setProc.launched && store.doneCallback)
                store.finished(-1);
        }
        stdout: SplitParser {
            onRead: line => {
                if (line.trim() !== "admin-prompt")
                    return;
                store.setProc.promptShown = true;
                store.adminPrompt = true;
                Changes.touch();
            }
        }
        stderr: StdioCollector { id: setErr }
        onExited: code => store.finished(code)
    }

    Component.onCompleted: {
        Changes.registerStore("lockdesign", store);
        store.reload();
    }
}
