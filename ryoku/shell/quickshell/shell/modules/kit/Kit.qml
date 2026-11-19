pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Wayland
import Lithic
import Ryoku.Ui.Singletons
import "../ask/look"

// One window follows the requested output; its frame rests during the linger.
Scope {
    id: root

    required property var screen
    property bool active: false
    // where a route asked it to open; the serial lets the same route land twice
    property string requestSection: ""
    property string requestPage: ""
    property var requestArgs: ({})
    property int requestSerial: 0

    signal requestClose()

    function flush() { frame.flush(); }

    readonly property real s: Tokens.uiScaleFor(screen ? screen.name : "")
    readonly property real screenWidth: screen && screen.width > 0 ? screen.width : 1280
    readonly property real screenHeight: screen && screen.height > 0 ? screen.height : 800
    readonly property real kitWidth: Math.round(Math.max(480, Math.min(1340, root.screenWidth / root.s - 48)) * root.s)
    readonly property real kitHeight: Math.round(Math.max(360, Math.min(820, root.screenHeight / root.s - 48)) * root.s)

    property real reveal: active ? 1 : 0
    readonly property real shown: Math.max(0, Math.min(1, reveal))
    Behavior on reveal {
        enabled: root.active
        NumberAnimation {
            duration: Look.dur(root.active ? Look.openMs : Look.closeMs)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: root.active ? Look.arrive : Look.leave
        }
    }

    PanelWindow {
        id: panel
        screen: root.screen
        visible: root.active
        color: "transparent"
        exclusionMode: ExclusionMode.Ignore
        exclusiveZone: 0
        anchors { top: true; bottom: true; left: true; right: true }
        WlrLayershell.layer: WlrLayer.Overlay
        WlrLayershell.namespace: "ryoku-kit"
        WlrLayershell.keyboardFocus: root.active ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None

        // the resting tabs float on the desktop; the scrim sets them off it
        Rectangle {
            anchors.fill: parent
            // the role carries an alpha of its own; the spec's is set here
            color: Lithic.alpha(Lithic.scrim, Look.dark ? 0.42 : 0.3)
            opacity: root.shown
        }
        MouseArea {
            anchors.fill: parent
            enabled: root.active
            acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
            onPressed: Qt.callLater(root.requestClose)
        }

        FocusScope {
            id: holder
            x: Math.round((root.screenWidth - width) / 2)
            y: Math.round((root.screenHeight - height) / 2) + (Look.fluent ? -Math.round(10 * root.s) * (1 - root.shown) : 0)
            width: root.kitWidth
            height: root.kitHeight
            focus: root.active
            opacity: Math.min(1, root.shown * (Look.lithic ? 3 : 1.5))
            scale: Look.material ? 0.94 + 0.06 * root.reveal : Look.adwaita ? 0.97 + 0.03 * root.shown : Look.lithic ? 0.985 + 0.015 * root.shown : 1
            onFocusChanged: if (focus) Qt.callLater(frame.focusKeyboard)

            RectangularShadow {
                x: frame.pageX
                width: holder.width - frame.pageX
                height: holder.height
                offset.y: Math.round((Look.lithic ? 6 : 12) * root.s)
                blur: Math.round((Look.lithic ? 26 : 42) * root.s)
                radius: Look.lithic ? 0 : Look.cornerAt(Look.cutFrame, width, height, root.s)
                color: Look.shadow
            }

            KitFrame {
                id: frame
                anchors.fill: parent
                s: root.s
                active: root.active
                requestSection: root.requestSection
                requestPage: root.requestPage
                requestArgs: root.requestArgs
                requestSerial: root.requestSerial
                onRequestClose: root.requestClose()
            }
        }
    }
}
