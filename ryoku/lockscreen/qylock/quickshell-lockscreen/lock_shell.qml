// lock_shell.qml - Ryoku in-session lock screen
//
// This is the entry point for the lock screen launched by `ryoku-shell lock`.
// It creates a Wayland session lock (or X11 fullscreen window) and loads the
// selected qylock theme (default: clockwork/orbital).
//
// Fingerprint integration:
//   - The SddmShim provides PAM and fingerprint state
//   - When WlSessionLock.secure becomes true, armWhenReady is set
//   - The shim probes fprintd and arms the sensor automatically
//   - On loginSucceeded, loginctl unlock-session is called
//   - On unlock (secure flips false), resetAuth() stops the sensor

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
    readonly property bool isWayland: Quickshell.env("XDG_SESSION_TYPE") === "wayland"
    property bool authenticated: false
    property bool sessionLocked: true
    property bool isTesting: Quickshell.env("QS_TESTING") === "1"

    SddmShim {
        id: sddmShim
        themePath: shellRoot.themePath
    }

    // ── login success handler ───────────────────────────────────────────────
    // Called by the shim when PAM authentication succeeds (either via fingerprint
    // or typed password). Unlocks the session and quits the lock screen.
    Connections {
        target: sddmShim.sddm
        function onLoginSucceeded() {
            shellRoot.authenticated = true

            Quickshell.execDetached(["loginctl", "unlock-session"]);

            // Dynamic exit delay: clockwork themes with windup animation
            // need a longer delay so the reveal animation completes.
            let delay = 100;
            if (activeTheme.includes("clockwork") && sddmShim.config.enableWindup === "true") {
                delay = 720;
            }
            quitTimer.interval = delay;
            quitTimer.start()
        }
    }

    Timer {
        id: quitTimer
        interval: 3000
        onTriggered: {
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
        active: shellRoot.isWayland
        sourceComponent: Component {
            WlSessionLock {
                id: lock
                locked: shellRoot.sessionLocked

                // onSecureChanged fires when the compositor confirms the lock.
                // We use this to:
                //   1. Write the qylock.locked marker (so ryoku-shell blocks)
                //   2. Arm the fingerprint sensor (armWhenReady = true)
                // On unlock, we clean up the marker and abort any PAM conversation.
                onSecureChanged: {
                    if (lock.secure) {
                        Quickshell.execDetached(["sh", "-c", "umask 077; : > \"${XDG_RUNTIME_DIR:-/tmp}/qylock.locked\""])
                        sddmShim.armWhenReady = true
                        // surface is up: arm the lock-in reveal (onCompleted too early)
                        sddmShim.sddm.surfaceRevealed()
                    } else {
                        Quickshell.execDetached(["sh", "-c", "rm -f \"${XDG_RUNTIME_DIR:-/tmp}/qylock.locked\""])
                        sddmShim.armWhenReady = false
                        sddmShim.resetAuth()
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

    // ── X11 fallback ────────────────────────────────────────────────────────
    // On X11 sessions, use fullscreen windows instead of WlSessionLock.
    // Each screen gets its own lock window.
    Loader {
        id: x11Loader
        active: !shellRoot.isWayland
        sourceComponent: Component {
            Variants {
                model: Quickshell.screens
                delegate: Window {
                    id: window
                    required property var modelData
                    screen: modelData
                    width: isTesting ? 1280 : screen.width
                    height: isTesting ? 720 : screen.height
                    visible: shellRoot.sessionLocked
                    visibility: isTesting ? Window.Windowed : Window.FullScreen
                    onClosing: (close) => {
                        close.accepted = shellRoot.authenticated || shellRoot.isTesting;
                    }
                    flags: Qt.WindowStaysOnTopHint | Qt.FramelessWindowHint | Qt.MaximizeUsingFullscreenGeometryHint
                    color: "black"
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
