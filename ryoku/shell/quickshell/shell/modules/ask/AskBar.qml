pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Wayland
import Lithic
import Ryoku.Ui.Singletons
import shell.services
import "bar"
import "look"

// The Alt+Space bar: a command bar over everything, in the desktop's design. This is the window, its arrival and its clients; the body is
// bar/BarSurface. mode is where the shell asked it to open: "ask" is the root
// list and "web" opens web search over it.
Scope {
    id: root

    required property var screen
    property bool active: false
    property string mode: "ask"

    signal requestClose()
    signal modeChangeRequested(string mode)

    readonly property real s: Tokens.uiScaleFor(screen ? screen.name : "")
    readonly property var clearances: SidebarState.railClearances(screen)
    readonly property real screenWidth: screen && screen.width > 0 ? screen.width : 1280
    readonly property real screenHeight: screen && screen.height > 0 ? screen.height : 800
    readonly property real gap: Math.round(13 * s)
    readonly property real availableWidth: Math.max(1, screenWidth - clearances.left - clearances.right - gap * 2)
    readonly property real panelY: Math.max(clearances.top + gap, Math.round(screenHeight * 0.18))
    readonly property real maximumHeight: Math.max(1, Math.min(620 * s, screenHeight - panelY - clearances.bottom - gap))

    // arrival: Lithic wipes down from its top edge, Material springs open from
    // 90%, Adwaita grows from 97%, Fluent slides down 10 as it fades in
    property real reveal: active ? 1 : 0
    readonly property real shown: Math.max(0, Math.min(1, reveal))
    Behavior on reveal {
        NumberAnimation {
            duration: Look.dur(root.active ? Look.openMs : Look.closeMs)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: root.active ? Look.arrive : Look.leave
        }
    }

    SearchClient { id: searchClient }
    DownloadsClient { id: downloadsClient }

    PanelWindow {
        id: panel
        screen: root.screen
        visible: root.active || root.reveal > 0.001
        color: "transparent"
        exclusionMode: ExclusionMode.Ignore
        exclusiveZone: 0
        anchors { top: true; bottom: true; left: true; right: true }
        WlrLayershell.layer: WlrLayer.Overlay
        WlrLayershell.namespace: "ryoku-ask"
        WlrLayershell.keyboardFocus: !root.active ? WlrKeyboardFocus.None
            : surface.yieldsKeyboard ? WlrKeyboardFocus.OnDemand : WlrKeyboardFocus.Exclusive
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
            width: Math.round(surface.presentedWidth)
            height: Math.round(surface.presentedHeight)
            focus: root.active
            opacity: Math.min(1, root.shown * (Look.lithic ? 3 : 1.5))
            scale: Look.material ? 0.9 + 0.1 * root.reveal : Look.adwaita ? 0.97 + 0.03 * root.shown : 1
            transformOrigin: Item.Top
            Keys.onEscapePressed: root.requestClose()
            onFocusChanged: if (focus) Qt.callLater(surface.focusField)

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

                BarSurface {
                    id: surface

                    readonly property bool yieldsKeyboard: surface.view !== null && surface.view !== undefined && surface.view.yieldsKeyboard === true

                    width: holder.width
                    height: holder.height
                    s: root.s
                    search: searchClient
                    downloads: downloadsClient
                    active: root.active
                    screenName: root.screen ? root.screen.name : ""
                    mode: root.mode
                    baseWidth: Math.round(720 * root.s)
                    maximumWidth: root.availableWidth
                    maximumHeight: root.maximumHeight
                    onRequestClose: root.requestClose()
                    onModeRequested: next => root.modeChangeRequested(next)
                }
            }
        }
    }
}
