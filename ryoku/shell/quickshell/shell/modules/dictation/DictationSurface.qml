import QtQuick
import Quickshell
import Quickshell.Wayland
import Lithic as L
import Ryoku.Ui.Singletons
import shell.services

PanelWindow {
    id: root

    readonly property real s: Tokens.uiScaleFor(screen ? screen.name : "")
    readonly property bool expanded: !Dictation.showingOff
    property real reveal: 0

    screen: ShellState.screens.find(scr => scr.name === Wm.focusedOutput)
        || ShellState.screens[0] || Quickshell.screens[0]
    color: "transparent"
    anchors { top: true; left: true }
    margins { top: Math.round(24 * root.s); left: Math.round(24 * root.s) }
    implicitWidth: Math.round(344 * s)
    implicitHeight: Math.round((expanded ? Math.max(136, card.implicitHeight / s + 16) : 64) * s)
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "ryoku-dictation"
    WlrLayershell.keyboardFocus: WlrKeyboardFocus.None
    mask: Region {}
    Binding {
        target: L.Lithic
        property: "reduceMotion"
        value: Motion.reduce
    }

    Component.onCompleted: reveal = Dictation.present ? 1 : 0
    Connections {
        target: Dictation
        function onPresentChanged() { root.reveal = Dictation.present ? 1 : 0; }
    }
    Behavior on reveal {
        NumberAnimation {
            duration: Motion.dur(root.reveal > 0 ? 240 : 160)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: L.Lithic.curve
        }
    }

    Item {
        id: plate
        x: Math.round(8 * root.s) - (1 - root.reveal) * 8 * root.s
        y: Math.round(8 * root.s) - (1 - root.reveal) * 6 * root.s
        width: parent.width - Math.round(16 * root.s)
        height: parent.height - Math.round(16 * root.s)
        opacity: root.reveal
        scale: 0.96 + 0.04 * root.reveal
        transformOrigin: Item.TopLeft

        L.CutRect {
            anchors.fill: parent
            fill: L.Lithic.stone1
            cutTopLeft: L.LithicTokens.cut.mid * root.s
            cutBottomRight: L.LithicTokens.cut.mid * root.s
            strokeWidth: root.s
            strokeColor: L.Lithic.vein
        }

        DictationCard {
            id: card
            anchors.fill: parent
            s: root.s
            state: Dictation.state
            live: Dictation.live
            busy: Dictation.busy
            loading: Dictation.loading
            failed: Dictation.failed
            offNote: Dictation.showingOff
            errorText: String(Dictation.frame.error || "")
            levels: Dictation.levels
            levelDb: Dictation.decibels
            modelName: Dictation.modelName
        }
    }
}
