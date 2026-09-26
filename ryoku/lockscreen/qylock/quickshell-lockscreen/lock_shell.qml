// lock_shell.qml - Ryoku in-session Wayland lock screen
//
// This is the entry point for the lock screen launched by `ryoku-shell lock`.
// It creates a secure Wayland session lock and loads the selected qylock theme
// (default: clockwork/orbital).
//
// Authentication integration:
//   - The SddmShim provides password and fingerprint PAM conversations
//   - A secure surface publishes session/generation proof before either arms
//   - Only the foreground login1 session may authenticate
//   - On loginSucceeded, the daemon restores its hard block and invalidates
//     proof before login1 unlocks
//   - Leaving the foreground cancels the reveal and republishes secure proof

import QtQuick
import Quickshell
import Quickshell.Wayland
import QtMultimedia
import Quickshell.Io
import "./shim"
import Ryoku.Ui.Singletons

ShellRoot {
    id: shellRoot

    // ── theme configuration ────────────────────────────────────────────────
    // QS_THEME and QS_THEME_PATH are set by lock.sh, which reads
    // ~/.config/qylock/theme and resolves the theme directory.
    property string activeTheme: Quickshell.env("QS_THEME") || "clockwork/orbital"
    property string themePath: Quickshell.env("QS_THEME_PATH") || (Quickshell.shellDir + "/themes_link/" + activeTheme)

    // ── shim interface ──────────────────────────────────────────────────────
    // Expose the SddmShim's properties to the theme via the sddm namespace.
    // Themes bind to sddm.login(), sddm.loginSucceeded, sddm.loginFailed,
    // sddm.hostName, sddm.fingerprintHint, etc.
    readonly property var sddm: sddmShim.sddm
    readonly property var config: sddmShim.config
    readonly property var userModel: sddmShim.userModel
    readonly property var sessionModel: sddmShim.sessionModel
    readonly property var keyboard: sddmShim.keyboard
    property bool authenticated: false
    property bool sessionLocked: true
    property bool lockSecure: false
    property bool proofPublished: false
    property bool surfaceAnnounced: false
    property string proofPendingAction: ""
    property bool unlockCancelled: false
    property bool relockPending: false
    readonly property string proofToken: Quickshell.env("QYLOCK_PROOF_TOKEN") || ""
    readonly property string sessionId: Quickshell.env("XDG_SESSION_ID") || ""

    function startProofOperation() {
        if (proofProcess.running || unlockProcess.running || relockProcess.running ||
                proofPendingAction === "")
            return
        proofProcess.action = proofPendingAction
        proofPendingAction = ""
        proofProcess.command = [
            Quickshell.shellDir + "/proof.sh",
            proofProcess.action,
            proofToken,
            sessionId
        ]
        proofProcess.running = true
    }

    function requestProof(action) {
        proofPendingAction = action
        startProofOperation()
    }

    function armAuthenticationIfSafe() {
        if (!lockSecure || !proofPublished || relockPending ||
                relockProcess.running || !sddmShim.loginSessionActive)
            return
        sddmShim.armWhenReady = true
        if (!surfaceAnnounced) {
            surfaceAnnounced = true
            sddmShim.sddm.surfaceRevealed()
        }
    }

    function recoverUnlock(showFailure) {
        quitTimer.stop()
        authenticated = false
        sddmShim.armWhenReady = false
        sddmShim.resetAuth()
        if (showFailure)
            sddmShim.sddm.loginFailed()
        proofPublished = false
        relockPending = lockSecure
        requestProof(lockSecure ? "publish" : "clear")
    }

    SddmShim {
        id: sddmShim
        themePath: shellRoot.themePath
    }

    // Publish and clear are serialized. Authentication is armed only after the
    // current generation has reached disk, so an older publish can never land
    // after an authenticated clear.
    Process {
        id: proofProcess
        property string action: ""
        onExited: (code) => {
            let completedAction = proofProcess.action
            proofProcess.action = ""
            if (code !== 0) {
                shellRoot.proofPublished = false
                shellRoot.proofPendingAction = shellRoot.lockSecure ? "publish" : "clear"
                proofRetry.restart()
                return
            }
            shellRoot.proofPublished = completedAction === "publish" && shellRoot.lockSecure
            Qt.callLater(shellRoot.startProofOperation)
            if (completedAction === "publish" && shellRoot.proofPublished &&
                    shellRoot.relockPending)
                relockProcess.running = true
            else if (completedAction === "publish")
                shellRoot.armAuthenticationIfSafe()
        }
    }

    Timer {
        id: proofRetry
        interval: 250
        onTriggered: shellRoot.startProofOperation()
    }

    // A cancelled reveal republishes proof, then reasserts login1's locked
    // state before authentication can arm again.
    Process {
        id: relockProcess
        command: ["loginctl", "lock-session", shellRoot.sessionId]
        onExited: (code) => {
            if (code !== 0 && shellRoot.lockSecure && shellRoot.relockPending) {
                relockRetry.restart()
                return
            }
            shellRoot.relockPending = false
            Qt.callLater(shellRoot.startProofOperation)
            shellRoot.armAuthenticationIfSafe()
        }
    }

    Timer {
        id: relockRetry
        interval: 1000
        onTriggered: {
            if (shellRoot.lockSecure && shellRoot.relockPending)
                relockProcess.running = true
        }
    }

    // The helper restores the hard sleep block, rechecks login1 activity and
    // invalidates proof inside one suspend transaction, then updates login1's
    // LockedHint. The Wayland lock stays raised through the reveal delay.
    Process {
        id: unlockProcess
        command: [Quickshell.shellDir + "/unlock.sh"]
        onExited: (code) => {
            if (code !== 0 || shellRoot.unlockCancelled ||
                    !sddmShim.loginSessionActive || !shellRoot.lockSecure) {
                shellRoot.recoverUnlock(code !== 0)
                proofRetry.restart()
                return
            }
            let delay = 100
            if (activeTheme.includes("clockwork") && sddmShim.config.enableWindup === "true") {
                // A password win answers after the wind-up already played, so
                // only the 560 ms reveal remains; a sensor win starts the full
                // wind-up at this instant (1600 ms + blast + reveal).
                delay = sddmShim.fingerprintUnlock ? 2300 : 720
            }
            quitTimer.interval = delay
            quitTimer.start()
        }
    }

    Connections {
        target: sddmShim.sddm
        function onLoginSucceeded() {
            if (unlockProcess.running)
                return
            if (!sddmShim.loginSessionActive || !shellRoot.lockSecure ||
                    !shellRoot.proofPublished || shellRoot.relockPending ||
                    relockProcess.running) {
                shellRoot.recoverUnlock(false)
                return
            }
            shellRoot.authenticated = true
            shellRoot.unlockCancelled = false
            shellRoot.proofPublished = false
            sddmShim.armWhenReady = false
            unlockProcess.running = true
        }
    }

    Connections {
        target: sddmShim
        function onLoginSessionActiveChanged() {
            if (sddmShim.loginSessionActive) {
                shellRoot.armAuthenticationIfSafe()
                return
            }
            let pendingUnlock = shellRoot.authenticated || unlockProcess.running || quitTimer.running
            if (pendingUnlock) {
                shellRoot.unlockCancelled = true
                shellRoot.recoverUnlock(false)
                return
            }
            sddmShim.armWhenReady = false
            sddmShim.resetAuth()
        }
    }

    Timer {
        id: quitTimer
        interval: 3000
        onTriggered: {
            if (!sddmShim.loginSessionActive || !shellRoot.lockSecure) {
                shellRoot.unlockCancelled = true
                shellRoot.recoverUnlock(false)
                return
            }
            shellRoot.sessionLocked = false
            Qt.quit()
        }
    }

    // ── theme loader ────────────────────────────────────────────────────────
    // Loads the selected theme's Main.qml into a fullscreen surface.
    Component {
        id: themeComponent
        Loader {
            anchors.fill: parent
            source: "file://" + shellRoot.themePath + "/Main.qml"
            onLoaded: { item.forceActiveFocus() }
            onStatusChanged: {
                if (status === Loader.Error) {
                    console.error("FAILED to load theme:", source)
                }
            }
        }
    }

    // ── fingerprint overlay (universal, above any skin) ─────────────────────
    // One reader rides above whatever theme the Loader above pulled in, in BOTH
    // surfaces below, so every skin shows the identical scan/unlock with zero
    // per-theme code. Bound only to the shim's fingerprint state -- it draws,
    // it never authenticates.
    property color fpAccent: "#ffb59b"
    FileView {
        id: paletteFile
        path: (Quickshell.env("HOME") || "") + "/.cache/ryoku/colors.json"
        blockLoading: true
        watchChanges: true
        printErrors: false
        onFileChanged: reload()
        onLoaded: {
            try {
                const o = JSON.parse(paletteFile.text() || "{}");
                if (o && typeof o.primary === "string" && o.primary.length)
                    shellRoot.fpAccent = o.primary;
            } catch (e) {}
        }
    }
    Component {
        id: fpOverlayComponent
        Item {
            id: ov
            anchors.fill: parent
            z: 10000
            readonly property var s: sddmShim.sddm
            readonly property string ph: !s.fingerprintReady ? "off"
                : (s.fingerprintState === "idle" ? "ready" : s.fingerprintState)
            readonly property bool unavailable: s.fingerprintState === "unavailable"

            // When the sensor is parked (claim held, #243) the scan glyph is
            // hidden and the message is neutral guidance, not the red "not
            // recognized" a real misread earns: the user did nothing wrong.
            FingerprintScan {
                id: fpScan
                anchors.horizontalCenter: parent.horizontalCenter
                y: parent.height * 0.60
                sizePx: Math.round(Math.min(parent.width, parent.height) * 0.10)
                accent: shellRoot.fpAccent
                phase: ov.unavailable ? "off" : ov.ph
            }
            Text {
                anchors.horizontalCenter: parent.horizontalCenter
                anchors.top: fpScan.bottom
                anchors.topMargin: Math.round(fpScan.sizePx * 0.18)
                font.pixelSize: Math.round(fpScan.sizePx * 0.18)
                color: ov.unavailable ? shellRoot.fpAccent : (ov.ph === "fail" ? "#e0806f" : shellRoot.fpAccent)
                opacity: (ov.unavailable || ov.ph === "scanning" || ov.ph === "success" || ov.ph === "fail") ? 0.92 : 0
                Behavior on opacity { NumberAnimation { duration: 180 } }
                text: ov.unavailable ? I18n.tr("Fingerprint unavailable — use your password")
                    : ov.ph === "success" ? I18n.tr("Unlocked")
                    : (ov.ph === "fail" ? I18n.tr("Not recognized") : I18n.tr("Reading\u2026"))
                visible: opacity > 0.01
            }
        }
    }

    // ── Wayland session lock ────────────────────────────────────────────────
    // Uses Quickshell's WlSessionLock to cover all outputs with a secure
    // surface. The lock is confirmed (secure=true) once the compositor
    // acknowledges every output is covered.
    Loader {
        id: waylandLoader
        active: true
        sourceComponent: Component {
            WlSessionLock {
                id: lock
                locked: shellRoot.sessionLocked

                // Proof publication completes before authentication is armed.
                // If login1 moves this session out of the foreground during an
                // authenticated reveal, the root state machine cancels it and
                // republishes proof while this secure surface stays raised.
                onSecureChanged: {
                    shellRoot.lockSecure = lock.secure
                    if (lock.secure) {
                        shellRoot.proofPublished = false
                        sddmShim.armWhenReady = false
                        shellRoot.requestProof("publish")
                    } else {
                        shellRoot.proofPublished = false
                        shellRoot.relockPending = false
                        relockRetry.stop()
                        sddmShim.armWhenReady = false
                        sddmShim.resetAuth()
                        shellRoot.requestProof("clear")
                    }
                }

                surface: Component {
                    WlSessionLockSurface {
                        color: "black"

                        // Absorb unhandled gestures (scroll, pinch) so they
                        // don't leak through to the desktop underneath.
                        PinchHandler { target: null }
                        WheelHandler { target: null }
                        MouseArea {
                            anchors.fill: parent
                            acceptedButtons: Qt.AllButtons
                            hoverEnabled: true
                            onWheel: (wheel) => { wheel.accepted = true }
                        }

                        Loader {
                            anchors.fill: parent
                            sourceComponent: themeComponent
                        }
                        Loader {
                            anchors.fill: parent
                            sourceComponent: fpOverlayComponent
                        }
                    }
                }
            }
        }
    }

}
