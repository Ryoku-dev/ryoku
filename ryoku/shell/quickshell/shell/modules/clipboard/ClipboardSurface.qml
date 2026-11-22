pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Wayland
import Lithic
import Ryoku.Ui.Singletons
import shell.services
import "../ask/look"

// the shell binds active to clipboardOpen and resets it on requestClose, so active is never assigned here
Scope {
    id: root

    property var screen: null
    property bool active: false

    signal requestClose()

    readonly property real s: Tokens.uiScaleFor(screen ? screen.name : "")
    readonly property var clearances: SidebarState.railClearances(screen)
    readonly property real screenWidth: screen && screen.width > 0 ? screen.width : 1280
    readonly property real screenHeight: screen && screen.height > 0 ? screen.height : 800
    readonly property real gap: Math.round(13 * s)
    readonly property real availableWidth: Math.max(1, screenWidth - clearances.left - clearances.right - gap * 2)
    readonly property real panelY: Math.max(clearances.top + gap, Math.round(screenHeight * 0.18))
    readonly property real availableHeight: Math.max(1, screenHeight - panelY - clearances.bottom - gap)

    property real reveal: active ? 1 : 0
    readonly property real shown: Math.max(0, Math.min(1, reveal))
    Behavior on reveal {
        NumberAnimation {
            duration: Look.dur(root.active ? Look.openMs : Look.closeMs)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: root.active ? Look.arrive : Look.leave
        }
    }

    PanelWindow {
        id: panel
        screen: root.screen
        visible: root.active || root.reveal > 0.001
        color: "transparent"
        exclusionMode: ExclusionMode.Ignore
        exclusiveZone: 0
        anchors { top: true; bottom: true; left: true; right: true }
        WlrLayershell.layer: WlrLayer.Overlay
        WlrLayershell.namespace: "clipboard"
        WlrLayershell.keyboardFocus: root.active ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None
        mask: Region {
            width: panel.width
            height: panel.height
            Region { intersection: Intersection.Subtract; width: panel.width; height: root.clearances.top }
            Region { intersection: Intersection.Subtract; y: panel.height - root.clearances.bottom; width: panel.width; height: root.clearances.bottom }
            Region { intersection: Intersection.Subtract; width: root.clearances.left; height: panel.height }
            Region { intersection: Intersection.Subtract; x: panel.width - root.clearances.right; width: root.clearances.right; height: panel.height }
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
            y: root.panelY + (Look.fluent ? -Math.round(10 * root.s) * (1 - root.shown) : 0)
            width: Math.round(body.implicitWidth)
            height: Math.round(body.implicitHeight)
            focus: root.active
            opacity: Math.min(1, root.shown * (Look.lithic ? 3 : 1.5))
            scale: Look.material ? 0.9 + 0.1 * root.reveal : Look.adwaita ? 0.97 + 0.03 * root.shown : 1
            transformOrigin: Item.Top
            onFocusChanged: if (focus) Qt.callLater(body.focusField)

            MouseArea {
                anchors.fill: parent
                acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
                onPressed: event => event.accepted = true
            }

            RectangularShadow {
                anchors.fill: slip
                offset.y: Math.round((Look.lithic ? 5 : 10) * root.s)
                blur: Math.round((Look.lithic ? 20 : 36) * root.s)
                radius: Look.lithic ? 0 : Look.cornerAt(Look.cutFrame, slip.width, slip.height, root.s)
                color: Look.shadow
            }

            Item {
                id: slip
                width: holder.width
                height: Look.lithic ? Math.ceil(holder.height * root.shown) : holder.height
                clip: Look.lithic && root.shown < 0.999

                Shape {
                    id: frame
                    width: holder.width
                    height: holder.height
                    s: root.s
                    cutSize: Look.cutFrame
                    cutTopRight: Look.cutFrameQuiet * root.s
                    cutBottomLeft: Look.cutFrameQuiet * root.s
                    fill: Look.frame
                    strokeColor: Look.frameStroke
                    strokeWidth: Qt.colorEqual(Look.frameStroke, "transparent") ? 0 : 1
                }
                Shape {
                    anchors.fill: frame
                    anchors.margins: 1
                    visible: Look.fluent
                    s: root.s
                    cutSize: Look.cutFrame
                    strokeColor: Look.frameInner
                    strokeWidth: 1
                }

                ClipPalette {
                    id: body
                    width: holder.width
                    height: holder.height
                    s: root.s
                    active: root.active
                    maximumWidth: root.availableWidth
                    maximumHeight: root.availableHeight
                    onRequestClose: root.requestClose()
                }
            }
        }
    }
}
