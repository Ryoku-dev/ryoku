pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Ryoku.Ui.Singletons

Singleton {
    id: root
    property var status: ({ state: "unread" })
    property var link: null
    property var draft: null
    property var submission: null
    property string job: ""
    property string error: ""
    property string message: ""
    property bool active: false
    readonly property bool signedIn: status.state === "signed_in"
    readonly property bool pending: status.state === "pending"
    readonly property bool busy: job !== ""

    function run(args, done) {
        if (root.busy)
            return;
        root.job = args.join(" ");
        root.error = "";
        Changes.run(["ryoku-hub", "account"].concat(args), function(ok, out, err) {
            root.job = "";
            var value = null;
            if (ok) {
                try { value = JSON.parse(out); }
                catch (e) { ok = false; err = I18n.tr("The Account tool returned unreadable data."); }
            }
            if (!ok) {
                root.error = String(err || I18n.tr("The Account request failed.")).trim().replace(/^ryoku-hub:\s*/, "");
                if (args[0] === "submission") {
                    Changes.run(["ryoku-hub", "account", "submission", "local"], function(readOK, localOut) {
                        if (readOK) {
                            try { root.submission = JSON.parse(localOut); }
                            catch (e) {}
                        }
                    });
                }
                return;
            }
            if (done)
                done(value);
        }, { timeout: 60000 });
    }
    function refresh() {
        root.run(["status"], function(value) {
            root.status = value;
            if (value.job && value.job.id)
                root.submission = value.job;
        });
    }
    function signIn() { root.run(["signin"], function(value) { root.status = value; root.link = null; }); }
    function poll() {
        root.run(["poll"], function(value) {
            root.status = value;
            if (value.state === "signed_in")
                root.refresh();
        });
    }
    function cancel() { root.run(["cancel"], function(value) { root.status = value; }); }
    function signOut() {
        root.run(["signout"], function(value) {
            root.status = value;
            root.link = null;
            root.draft = null;
            root.submission = null;
        });
    }
    function website() { root.run(["website"], function() { root.message = I18n.tr("Continue in your system browser, then refresh here."); }); }
    function prepareLink() { root.run(["link"], function(value) { root.link = value; }); }
    function confirmLink(text) {
        if (root.link)
            root.run(["link", root.link.challengeID, text], function() { root.link = null; root.refresh(); });
    }
    function unlink(accountID, cardID) { root.run(["unlink", accountID, cardID], function() { root.link = null; root.refresh(); }); }
    function prepare(kind, path) {
        root.draft = null;
        root.run(["submission", "prepare", kind, path], function(value) { root.draft = value; });
    }
    function submit() {
        if (!root.draft)
            return;
        root.run(["submission", "submit", root.draft.nonce, root.draft.originalSha, "rights", "policy", "review"], function(value) {
            root.submission = value;
            root.draft = null;
            root.message = I18n.tr("Submitted for independent inspection and review. Queued is not approved.");
        });
    }
    function submissionAction(action) {
        if (action !== "clear" && !root.submission)
            return;
        var args = action === "clear" ? ["submission", action] : ["submission", action, root.submission.id];
        root.run(args, function(value) {
            root.submission = value;
            if (action === "select" || action === "clear") {
                Badge.scheduleSync();
                Badge.refresh(false);
                root.refresh();
            }
        });
    }
    property Timer pollTimer: Timer {
        interval: 1000
        repeat: true
        running: root.active && root.pending && !root.busy && root.error === ""
        onTriggered: {
            if (Date.now() / 1000 >= Number(root.status.nextPollAt || 0))
                root.poll();
        }
    }
}
