pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Wayland
import Quickshell.Widgets
import Ryoku.Ui.Singletons
import shell.services

Scope {
    id: root

    required property var screen
    property bool active: false
    property string mode: "ask"
    property string tool: ""

    signal requestClose()
    signal modeChangeRequested(string mode)
    signal bubblePinned(string question, string answer)

    readonly property real s: Tokens.uiScaleFor(screen ? screen.name : "")
    readonly property var clearances: SidebarState.railClearances(screen)
    readonly property real screenWidth: screen && screen.width > 0 ? screen.width : 1280
    readonly property real screenHeight: screen && screen.height > 0 ? screen.height : 800
    readonly property real gap: Tokens.s3 * s
    readonly property real availableWidth: Math.max(1, screenWidth - clearances.left - clearances.right - gap * 2)
    readonly property real panelWidth: Math.min(availableWidth, 720 * s)
    readonly property real panelX: (screenWidth - panelWidth) / 2
    readonly property real panelY: Math.max(clearances.top + gap, screenHeight * 0.18)
    readonly property real maximumHeight: Math.max(1, screenHeight - panelY - clearances.bottom - gap)
    readonly property real desiredHeight: Math.min(maximumHeight, surface.implicitHeight)
    property real presentedHeight: desiredHeight
    property real reveal: active ? 1 : 0
    property alias query: surface.query
    readonly property bool motionAllowed: !Tokens.reduceMotion && !Motion.reduce

    Behavior on reveal {
        enabled: root.motionAllowed
        NumberAnimation { duration: Tokens.swap; easing.type: Tokens.ease }
    }

    function switchMode(next) {
        if (["ask", "chat", "tools", "web"].indexOf(next) < 0 || mode === next)
            return;
        modeChangeRequested(next);
        Qt.callLater(surface.focusField);
    }

    onActiveChanged: {
        if (active) {
            if (tool === "compress" || tool === "install")
                switchMode("tools");
            Qt.callLater(surface.focusField);
        }
    }

    RashinClient { id: rashin }

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
            id: focusSurface
            x: root.panelX
            y: root.panelY
            width: root.panelWidth
            height: root.presentedHeight
            focus: root.active
            opacity: root.reveal
            scale: 0.96 + root.reveal * 0.04
            transformOrigin: Item.Top
            Keys.onEscapePressed: root.requestClose()
            onFocusChanged: if (focus) Qt.callLater(function() { focusSurface.forceActiveFocus(); })

            MouseArea {
                anchors.fill: parent
                acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
                onPressed: event => event.accepted = true
            }

            RectangularShadow {
                anchors.fill: surface
                radius: surface.radius
                blur: 28 * root.s
                spread: 4 * root.s
                color: Qt.rgba(Tokens.keycapDark.r, Tokens.keycapDark.g, Tokens.keycapDark.b, 0.42)
                visible: root.active
            }

            AskSurface {
                id: surface
                anchors.fill: parent
                s: root.s
                screen: root.screen
                client: rashin
                active: root.active
                mode: root.mode
                tool: root.tool
                maximumHeight: root.maximumHeight
                onRequestClose: root.requestClose()
                onModeRequested: next => root.switchMode(next)
                onBubblePinned: (question, answer) => root.bubblePinned(question, answer)
            }
        }
    }
}
