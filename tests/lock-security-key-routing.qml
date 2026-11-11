import QtQuick
import Quickshell
import "shim"

ShellRoot {
    id: root
    property var passwordPam: null
    property var keyPam: null
    property int keyEnds: 0
    property int successes: 0
    property int phase: 0
    property int ticks: 0

    SddmShim { id: shim }

    Connections {
        target: shim.sddm
        function onLoginSucceeded() { root.successes++ }
    }

    function fail(message) {
        console.error("FAIL: " + message)
        Qt.quit()
    }

    Component.onCompleted: {
        for (var i = 0; i < shim.data.length; i++) {
            var item = shim.data[i]
            if (!item)
                continue
            if (item.config === "ryoku-lock-pw")
                root.passwordPam = item
            else if (item.config === "ryoku-lock-key")
                root.keyPam = item
        }
        if (!root.passwordPam || !root.keyPam) {
            fail("missing PAM conversation")
            return
        }
        root.keyPam.completed.connect(() => root.keyEnds++)
        shim.sddm.keyPrompts = true
        shim.armWhenReady = true
    }

    Timer {
        interval: 50
        running: true
        repeat: true
        onTriggered: {
            root.ticks++
            if (root.ticks > 160) {
                root.fail("timed out")
                return
            }
            if (root.phase === 0 && root.passwordPam.responseRequired && root.keyPam.responseRequired) {
                if (shim.sddm.keyState !== "pin") {
                    root.fail("key prompt did not enter PIN state")
                    return
                }
                shim.sddm.login(Quickshell.env("USER") || "traveler", "password-only", 0)
                root.phase = 1
                root.ticks = 0
                return
            }
            if (root.phase === 1 && root.ticks >= 4) {
                if (root.keyEnds !== 0 || !root.keyPam.responseRequired || shim.sddm.keyState !== "pin") {
                    root.fail("generic password login consumed the key prompt")
                    return
                }
                shim.sddm.loginKeyPin(Quickshell.env("USER") || "traveler", "key-pin")
                root.phase = 2
                root.ticks = 0
                return
            }
            if (root.phase === 2 && root.keyEnds === 1) {
                if (root.successes !== 0) {
                    root.fail("failed PAM conversation unlocked the session")
                    return
                }
                console.log("PASS: password stayed on pamPw, explicit PIN reached pamKey, and failure stayed locked")
                Qt.quit()
            }
        }
    }
}
