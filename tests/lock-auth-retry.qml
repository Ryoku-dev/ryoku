import QtQuick
import Quickshell
import "shim"
ShellRoot {
    id: root
    readonly property bool retryOn: Quickshell.env("LOCK_RETRY_RUN") === "on"
    readonly property string user: Quickshell.env("USER") || "traveler"
    property var pw: null
    property int prompts: 0
    property int shakes: 0
    property int unlocks: 0
    property int ends: 0
    property int phase: 0
    SddmShim { id: shim }
    Connections {
        target: shim.sddm
        function onLoginFailed() { root.shakes++; }
        function onLoginSucceeded() { root.unlocks++; }
    }
    function fail(msg) { console.error("FAIL: " + msg); Qt.quit(); }
    Component.onCompleted: {
        for (var i = 0; i < shim.data.length; i++) {
            if (shim.data[i] && shim.data[i].config === "ryoku-lock-pw")
                root.pw = shim.data[i];
        }
        if (!root.pw) { root.fail("no ryoku-lock-pw conversation in the shim"); return; }
        root.pw.completed.connect(() => root.ends++);
        root.pw.responseRequiredChanged.connect(() => { if (root.pw.responseRequired) root.prompts++; });
        shim.armWhenReady = true;
    }
    Timer {
        interval: 1000; running: true; repeat: true
        onTriggered: {
            if (root.phase === 0) {
                if (root.prompts !== 1) { root.fail("the password prompt never appeared"); return; }
                if (shim.authRetryEnabled !== root.retryOn) { root.fail("flag read as " + shim.authRetryEnabled); return; }
                shim.sddm.login(root.user, "", 0);
                interval = 600;
            } else if (root.phase === 1) {
                if (root.retryOn) {
                    if (root.prompts !== 2 || root.ends !== 0 || root.shakes !== 0 || !root.pw.active)
                        { root.fail("empty submit did not give one fresh quiet prompt: prompts=" + root.prompts + " ends=" + root.ends + " shakes=" + root.shakes); return; }
                } else if (root.shakes !== 1 || root.ends !== 1) {
                    root.fail("with retry off an empty submit must still answer PAM: shakes=" + root.shakes + " ends=" + root.ends); return;
                }
                shim.sddm.login(root.user, "wrong", 0);
                interval = 600;
            } else if (root.phase === 2) {
                var shakes = root.retryOn ? 1 : 2;
                if (root.shakes !== shakes || root.unlocks !== 0 || !root.pw.active || !root.pw.responseRequired)
                    { root.fail("a wrong key after the retry must shake once and re-prompt: shakes=" + root.shakes + " unlocks=" + root.unlocks); return; }
                console.log("PASS: retry " + (root.retryOn ? "on" : "off") + ", wrong key shook, never unlocked");
                Qt.quit();
            }
            root.phase++;
        }
    }
}
