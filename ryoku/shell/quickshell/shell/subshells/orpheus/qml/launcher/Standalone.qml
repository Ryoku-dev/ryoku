pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import Ryoku.Blobs
import shell.services
import "../kit" as K
import "../settings" as S
import "../Metrics.js" as M

// The launcher when Orpheus is not the bar on this screen: the same liquid
// sheet hangs from the top screen edge at the centre, its shoulders melting
// into the edge, in an overlay window of its own.
PanelWindow {
    id: win

    property string monitor: ""
    property bool open: false
    readonly property bool shown: open || stage.p > 0.001
    readonly property alias sheet: sheet

    signal dismissed()

    // the sheet fills as the open is asked for, before the pour starts
    onOpenChanged: if (open) sheet.prepare()
    Component.onCompleted: if (open) sheet.prepare()

    screen: {
        const screens = Quickshell.screens;
        for (let i = 0; i < screens.length; ++i) {
            if (screens[i] && screens[i].name === win.monitor)
                return screens[i];
        }
        return screens.length > 0 ? screens[0] : null;
    }
    visible: shown
    color: "transparent"
    // under another top bar the launcher pours from that bar's lower edge instead of covering it
    exclusionMode: ExclusionMode.Normal
    anchors { top: true; bottom: true; left: true; right: true }
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "launcher"
    WlrLayershell.keyboardFocus: win.open ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None

    Item {
        id: stage

        // derived, not latched: a pour started before the surface maps would
        // finish unseen, and a quick reopen must pour again
        readonly property bool armed: win.open && win.backingWindowVisible
        property real p: 0

        readonly property bool square: S.Prefs.form === "square"
        readonly property real smoothing: square ? K.Look.squareSmoothing : K.Look.liquidSmoothing
        readonly property real radius: square
            ? K.Look.cornerAt(K.Look.cutSm, 200, 40, K.Style.scale)
            : K.Look.cornerAt(K.Look.cutXl, 400, K.Style.barH, K.Style.scale)
        readonly property real maxHeight: Math.max(0, Math.min(
            M.launcherMaxH * K.Style.scale, height - K.Style.barH - 48))
        readonly property real sheetW: Math.max(0, Math.min(
            M.launcherMaxW * K.Style.scale, width, Number(sheet.implicitWidth) || 0))
        readonly property real sheetH: Math.max(0, Math.min(maxHeight,
            Number(sheet.level) || 0))
        // the pour starts as a narrow drip and widens as it fills
        readonly property real restW: Math.min(sheetW, 96 * K.Style.scale)
        readonly property bool active: win.open || p > 0.001
        readonly property real curW: active ? restW + (sheetW - restW) * p : 0
        readonly property real curH: active ? sheetH * p : 0
        readonly property real sheetX: Math.round((width - curW) / 2)
        // The body buries a neck above the top edge so the shoulder fillets
        // fuse it into the edge; on close the neck drains before it vanishes.
        readonly property real neck: smoothing * 2
        readonly property real burial: (1 - p) * smoothing
        readonly property real blobY: -neck + burial
        readonly property real blobH: active ? curH + neck - burial : 0
        readonly property real reveal: Math.max(0, Math.min(1, (p - 0.35) / 0.25))
        readonly property bool contentShown: win.open && p >= 0.35

        readonly property var edgeOwner: ({ owner: "orpheus-launcher-standalone-edge" })
        readonly property bool edgeRunning: win.open && sheet.wantsMusicEdge === true
            && S.Prefs.gain > 0 && K.Look.ambientMotion

        function syncEdgeClaim() {
            AudioBars.setActive(stage.edgeOwner, stage.edgeRunning);
        }

        anchors.fill: parent

        onArmedChanged: if (armed) Qt.callLater(sheet.focusSearch)
        onEdgeRunningChanged: syncEdgeClaim()
        Component.onCompleted: syncEdgeClaim()
        Component.onDestruction: AudioBars.setActive(stage.edgeOwner, false)

        states: State {
            name: "open"
            when: stage.armed
            PropertyChanges { stage.p: 1 }
        }

        transitions: [
            Transition {
                to: "open"
                enabled: !K.Style.still
                NumberAnimation {
                    property: "p"
                    duration: K.Style.enter
                    easing.type: Easing.BezierSpline
                    easing.bezierCurve: K.Style.enterCurve
                }
            },
            Transition {
                from: "open"
                enabled: !K.Style.still
                NumberAnimation {
                    property: "p"
                    duration: K.Style.exit
                    easing.type: Easing.BezierSpline
                    easing.bezierCurve: K.Style.exitCurve
                }
            }
        ]

        // The dismiss backdrop: a click anywhere off the sheet closes it.
        MouseArea {
            anchors.fill: parent
            enabled: win.open
            acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
            onPressed: mouse => {
                mouse.accepted = true;
                Qt.callLater(win.dismissed);
            }
        }

        // Fallback when focus has left the sheet; the sheet takes Esc itself
        // through a ShortcutOverride so its staged Esc runs first.
        Shortcut {
            sequences: [StandardKey.Cancel]
            enabled: win.open
            onActivated: win.dismissed()
        }

        BlobGroup {
            id: liquid
            color: K.Style.base
            smoothing: stage.smoothing
            cutCorners: K.Look.lithic
            borderWidth: S.Prefs.rim ? 1 : 0
            borderColor: K.Style.rim
            waveColor: K.Style.trace
            shadowStrength: 0
            shadowSize: 0
        }

        // Every shape in the group is unioned by the shader, visible or not, so
        // a hidden shape must also have zero size.
        BlobRect {
            group: liquid
            visible: stage.active
            x: stage.sheetX - M.shoulderReach
            y: -M.shoulder - 2
            width: stage.active ? stage.curW + M.shoulderReach * 2 : 0
            height: M.shoulder
            radius: 0
            sinks: false
            deformScale: 0
        }

        // The paper. Its lower edge ripples outward with the music, below the
        // sheet, so it never covers the content.
        BlobWave {
            group: liquid
            visible: stage.active
            x: stage.sheetX
            y: stage.blobY
            width: stage.curW
            height: stage.blobH
            topLeftRadius: 0
            topRightRadius: 0
            bottomLeftRadius: stage.radius
            bottomRightRadius: stage.radius
            sinks: false
            deformScale: 0
            levels: stage.edgeRunning ? AudioBars.levels : []
            gain: S.Prefs.gain
            amplitude: M.launcherWave * K.Style.scale
            points: Math.round(Math.min(64, Math.max(12,
                width / (12 * K.Style.scale))))
            stepped: stage.square
            blockPitch: Math.max(14 * K.Style.scale, Math.min(26 * K.Style.scale,
                (width - 2 * K.Style.barPad) / 10))
            mirror: S.Prefs.mirror
            edgeInset: stage.square ? K.Style.barPad
                : Math.max(K.Style.barPad, width * 0.16)
            waveStiffness: S.Prefs.waveStiffness
            waveDamping: S.Prefs.waveDamping
        }

        Item {
            id: host
            x: stage.sheetX
            y: 0
            width: stage.curW
            height: stage.curH
            // never hidden: a hidden ancestor would refuse the field's focus while it pours
            clip: true

            // swallows clicks on the paper so only a click off the sheet closes it
            MouseArea {
                anchors.fill: parent
                acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
                onPressed: mouse => mouse.accepted = true
            }

            Sheet {
                id: sheet
                x: Math.round((host.width - width) / 2)
                y: 0
                width: stage.sheetW
                height: stage.sheetH
                opacity: stage.reveal
                shown: stage.contentShown
                welded: false
                monitor: win.monitor
                maxHeight: stage.maxHeight
                // the music edge ripples outward below the paper, so no room is reserved
                waveRoom: 0
                onCloseRequested: win.dismissed()
            }
        }
    }
}
