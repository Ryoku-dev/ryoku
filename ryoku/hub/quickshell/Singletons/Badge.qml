pragma Singleton

import QtQuick
import Quickshell
import Ryoku.Ui.Singletons

Singleton {
    id: root

    property bool settingsOpen: false
    property var status: ({})
    readonly property string request: status.request || ""
    readonly property var badge: status.badge || null
    readonly property var ownBadge: root.badge
    readonly property bool owner: status.owner === true
    // the card's signed record is published for friends' binders; anyone may opt in, badge or not
    readonly property bool online: status.online === true
    readonly property var design: status.design || null
    // the built-in card styles this badge's login owns: a style is one holder's own look
    readonly property var ownStyles: root.badge && Array.isArray(status.styles) ? status.styles : []
    property var preview: null
    property var checked: null
    property var candidate: null
    readonly property var ownDesign: root.preview || root.design
    readonly property bool previewing: root.preview !== null
    property string job: ""
    property string error: ""
    property string message: ""
    property string issuedCode: ""
    property bool reread: false
    property bool reading: false
    property bool syncing: false
    property bool syncAgain: false
    // the error came from a background sync or status read, so their next success clears it
    property string errorFrom: ""
    // a record carries the facts the share still shows: published before the machine's full
    // read-out, it gave friends' binders a card with no threads, refresh or width
    readonly property bool factsReady: SysInfo.baseLanded && !SysInfo.loading && CardCode.ready

    function words(err) {
        var s = String(err || "").trim().replace(/^ryoku-hub:\s*/, "");
        if (/timeout|deadline|timed out/i.test(s))
            return I18n.tr("The team service took too long. Try again when you are online.");
        if (/network|connection|no such host|dial tcp|TLS/i.test(s))
            return I18n.tr("The team service could not be reached. Check your connection and try again.");
        if (/keyring|secret-tool|owner key/i.test(s))
            return I18n.tr("The owner's signing key is not available in the keyring.");
        var refused = /refused the card: (.*)$/i.exec(s);
        if (refused)
            return I18n.tr("The team service refused your card: %1").replace("%1", refused[1]);
        return s || I18n.tr("The badge tool could not finish. Update Ryoku and try again.");
    }
    function fail(from, err) {
        root.error = root.words(err);
        root.errorFrom = from;
    }
    function recovered(from) {
        if (root.errorFrom === from) {
            root.error = "";
            root.errorFrom = "";
        }
    }
    // fresh re-reads the team list so an approval made a moment ago shows; plain reads trust the
    // backend's ten-minute cache
    function refresh(fresh) {
        if (root.reading) {
            root.reread = true;
            return;
        }
        root.reading = true;
        Changes.run(fresh ? ["ryoku-hub", "badge", "status", "--refresh"] : ["ryoku-hub", "badge", "status"], function(ok, out, err) {
            root.reading = false;
            if (root.reread) {
                root.reread = false;
                Qt.callLater(root.refresh);
            }
            if (!ok) {
                root.fail("status", err);
                return;
            }
            try { root.status = JSON.parse(out); root.recovered("status"); }
            catch (e) { root.error = I18n.tr("The badge status could not be read. Try again."); root.errorFrom = "status"; }
        }, { timeout: 45000 });
    }
    function run(action, argv, done) {
        if (root.job !== "")
            return;
        root.job = action;
        root.error = "";
        root.errorFrom = "";
        root.message = "";
        Changes.run(argv, function(ok, out, err) {
            if (!ok) {
                if (action === "check") {
                    try { root.checked = JSON.parse(out); } catch (e) { root.checked = null; }
                }
                root.error = root.words(err);
                root.errorFrom = action;
            } else {
                try { done(out.trim() === "" ? ({}) : JSON.parse(out)); }
                catch (e) { root.error = I18n.tr("The badge tool returned an unreadable answer. Try again."); }
            }
            root.refresh();
        }, { timeout: 45000 });
    }
    function activate(code) {
        root.run("activate", ["ryoku-hub", "badge", "activate", String(code).trim()], function(result) {
            root.status = result;
            root.message = I18n.tr("Badge activated");
            root.scheduleSync();
        });
    }
    function setOnline(on) {
        root.run("online", ["ryoku-hub", "badge", "online", on ? "on" : "off"], function(result) {
            root.status = result;
        });
    }
    function issue(fields) {
        root.issuedCode = "";
        root.run("issue", ["ryoku-hub", "badge", "issue", "--issuer", "owner", "--request", String(fields.request).trim(),
            "--login", String(fields.login).trim(), "--role", String(fields.role)], function(result) {
            root.issuedCode = result.code || "";
            root.message = result.published ? I18n.tr("Code made and published. Send it to the member.")
                : I18n.tr("Code made. Send it to the member; activation will try publishing it again.");
        });
    }
    function revoke(codeOrId) {
        root.run("revoke", ["ryoku-hub", "badge", "revoke", "--issuer", "owner", String(codeOrId).trim()], function(result) {
            root.message = result.published ? I18n.tr("Badge revoked") : I18n.tr("The revocation was not published. Try again when you are online.");
        });
    }
    function previewFolder(dir) {
        root.clearPreview();
        root.checked = null;
        root.candidate = null;
        root.run("check", ["ryoku-hub", "design", "check", dir], function(result) {
            root.checked = result;
            if (result.ok && result.design) {
                root.candidate = { dir: dir, name: result.design.name, spec: result.design, approved: false };
                root.preview = root.candidate;
            }
        });
    }
    function previewPending(sha) {
        root.run("fetch", ["ryoku-hub", "design", "fetch", String(sha).trim(), "--pending"], function(result) {
            root.candidate = { dir: result.dir, name: result.design.name, spec: result.design, approved: false };
            root.preview = root.candidate;
            root.message = I18n.tr("Preview only. This design has not been approved.");
        });
    }
    function clearPreview() { root.preview = null; }
    function saveDesign(dir, out) {
        root.run("pack", ["ryoku-hub", "design", "pack", dir, out], function() {
            root.message = I18n.tr("Saved to %1. Use /card submit in the team channel.").replace("%1", out);
        });
    }
    function scheduleSync() {
        if (root.online)
            settle.restart();
    }
    function sync() {
        if (!root.online)
            return;
        if (!root.factsReady) {
            // the full read-out runs only for pages that show it; onFactsReadyChanged comes back
            if (!SysInfo.loaded && !SysInfo.loading)
                SysInfo.load();
            return;
        }
        if (root.syncing) {
            root.syncAgain = true;
            return;
        }
        root.syncing = true;
        root.syncAgain = false;
        var hero = ProfileStore.get("hero", ({})) || {};
        if (hero.kind === "custom")
            root.captureRequested();
        else
            root.publish("");
    }
    function publish(picture) {
        var argv = ["ryoku-hub", "badge", "sync"].concat(CardCode.argv.slice(3));
        if (picture !== "")
            argv = argv.concat(["--picture", picture]);
        Changes.run(argv, function(ok, out, err) {
            if (!ok)
                root.fail("sync", err);
            else {
                try { root.status = Object.assign({}, root.status, JSON.parse(out)); root.recovered("sync"); }
                catch (e) { root.error = I18n.tr("The card's publishing status could not be read."); root.errorFrom = "sync"; }
            }
            if (picture !== "")
                Changes.run(["rm", "-f", "--", picture], function() { root.finishSync(); });
            else
                root.finishSync();
        }, { timeout: 30000 });
    }
    function finishSync() {
        root.syncing = false;
        if (root.syncAgain)
            settle.restart();
    }
    // a custom picture needs a fresh still of the card; Settings renders it (a singleton can't hold
    // the card without importing the components that import it) and answers with publish() or this
    function captureFailed(reason) {
        root.error = reason;
        root.errorFrom = "sync";
        root.finishSync();
    }
    signal captureRequested()
    Timer { id: settle; interval: 3000; onTriggered: root.sync() }
    Timer { interval: 600000; repeat: true; running: root.settingsOpen; onTriggered: root.refresh(true) }
    onSettingsOpenChanged: { if (root.settingsOpen) root.refresh(true); }
    onBadgeChanged: root.scheduleSync()
    onOnlineChanged: root.scheduleSync()
    onFactsReadyChanged: { if (root.factsReady) root.scheduleSync(); }
    Connections { target: ProfileStore; function onDataChanged() { root.scheduleSync(); } }
    Component.onCompleted: root.refresh()
}
