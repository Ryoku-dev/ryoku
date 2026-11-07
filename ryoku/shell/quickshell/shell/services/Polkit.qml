pragma Singleton

import QtQuick
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons

// QML view of the daemon `polkit` topic. The daemon is the PolicyKit1
// authentication agent: it takes BeginAuthentication from polkitd, runs the PAM
// conversation through polkit-agent-helper-1, and streams the live prompt here.
// This only renders and collects input, so the secret never becomes a process
// argument and the D-Bus and PAM plumbing stays in the daemon.
//
// `prompt` is PAM's own text ("Password: ", or a fingerprint nudge), `echo` says
// whether it may be shown as typed, and `error` carries a failed attempt so the
// island can retry in place rather than closing and reopening.
Singleton {
    id: root

    property bool active: false
    property string message: ""     // the action's description, from polkitd
    property string info: ""        // PAM_TEXT_INFO
    property string error: ""       // PAM_ERROR_MSG, e.g. a wrong password
    property string prompt: ""      // PAM_PROMPT_ECHO_OFF/ON text
    property bool echo: false       // true when the answer may be shown
    property bool busy: false       // an answer is with PAM, awaiting the verdict
    property string fingerprint: "" // "" none, "scanning", "fail" (pam_fprintd)

    // Fired whenever a fresh frame arrives, so the surface clears the field and
    // takes focus again on a retry without the island closing in between.
    signal refreshed()

    function apply(f) {
        if (!f || typeof f !== "object") {
            // A malformed frame must never wedge the island open over the desktop.
            root.active = false;
            root.busy = false;
            return;
        }
        const wasActive = root.active;
        const hadError = root.error;
        root.active = f.active === true;
        root.message = f.message || "";
        root.info = f.info || "";
        root.error = f.error || "";
        root.prompt = f.prompt || "";
        root.echo = f.echo === true;
        root.fingerprint = f.fingerprint || "";
        // A new prompt (or a new error on a retry) means PAM answered; drop
        // the busy state so the field is live again.
        if (!root.active || !wasActive || root.error !== hadError)
            root.busy = false;
        if (root.active)
            root.refreshed();
    }

    function submit(secret) {
        if (!root.active || root.busy)
            return;
        root.busy = true;
        root.send("polkit.submit", { password: secret });
    }

    function cancel() {
        if (!root.active)
            return;
        root.busy = false;
        root.send("polkit.cancel", {});
    }

    function send(method, args) {
        Bus.call(method, args);
    }

    // The bus resyncs the prompt after a reconnect, so an authentication
    // request still lands once a restarted daemon is back.
    BusTopic {
        id: topicFeed
        topic: "polkit"
        onUpdated: root.apply(topicFeed.data)
    }

    Connections {
        target: Bus
        function onConnectedChanged() {
            if (!Bus.connected) {
                root.active = false;
                root.busy = false;
            }
        }
    }
}
