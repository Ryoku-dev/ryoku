pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import Ryoku.Ui.Singletons
import shell.services

Scope {
    id: root

    required property var screen
    required property string side
    property bool visible: true

    readonly property real s: Tokens.uiScaleFor(screen ? screen.name : "")
    readonly property real panelWidth: Config.sidebars.width * s
    readonly property real surfaceWidth: Math.max(1,
        Math.round(panelWidth * reveal * Config.sidebars.wallpaperSlide))
    readonly property bool open: SidebarState.isOpen(screen, side)
    readonly property real bleed: 48 * s
    property real reveal: open ? 1 : 0

    onRevealChanged: SidebarState.setProgress(screen, side, reveal)
    Component.onCompleted: SidebarState.setProgress(screen, side, reveal)
    Component.onDestruction: SidebarState.setProgress(screen, side, 0)

    Behavior on reveal {
        enabled: !Motion.reduce && !Tokens.reduceMotion
        NumberAnimation {
            duration: root.open ? SidebarState.enterDuration : SidebarState.exitDuration
            easing.type: Easing.Bezier
            easing.bezierCurve: root.open ? SidebarState.enterCurve : SidebarState.exitCurve
        }
    }

    PanelWindow {
        id: panel

        screen: root.screen
        visible: root.visible
        implicitWidth: root.surfaceWidth
        color: "transparent"
        exclusionMode: ExclusionMode.Normal
        exclusiveZone: Config.sidebars.push ? Math.round(root.panelWidth * root.reveal) : 0
        WlrLayershell.layer: WlrLayer.Background
        WlrLayershell.namespace: root.side === "left"
            ? "ryoku-sidebar-left" : "ryoku-sidebar-right"
        WlrLayershell.keyboardFocus: root.open
            ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None
        anchors {
            top: true
            bottom: true
            left: root.side === "left"
            right: root.side === "right"
        }

        Rectangle {
            anchors.fill: parent
            color: Tokens.paper
        }

        SidebarChrome {
            x: root.side === "left" ? 0 : panel.width - width
            width: root.panelWidth
            height: panel.height
            screen: root.screen
            side: root.side
            reveal: root.reveal
            open: root.open
        }

        Loader {
            id: depthEdge
            x: root.side === "left" ? panel.width - root.bleed : 0
            y: 0
            width: root.bleed + 1
            height: panel.height
            active: Config.sidebars.depth
            source: active ? "SidebarDepthEdge.qml" : ""
            onLoaded: {
                item.width = Qt.binding(function() { return depthEdge.width; });
                item.height = Qt.binding(function() { return depthEdge.height; });
                item.progress = Qt.binding(function() { return root.reveal; });
                item.side = Qt.binding(function() { return root.side === "left" ? 0 : 1; });
                item.intensity = 1;
                item.radius = Qt.binding(function() { return Tokens.radius * root.s; });
            }
        }

        Rectangle {
            x: root.side === "left" ? panel.width - root.bleed : 0
            y: 0
            width: root.bleed + 1
            height: panel.height
            visible: Config.sidebars.depth && depthEdge.status !== Loader.Ready
            opacity: root.reveal
            gradient: Gradient {
                orientation: Gradient.Horizontal
                GradientStop {
                    position: 0
                    color: root.side === "left" ? "transparent" : Qt.rgba(0, 0, 0, 0.34)
                }
                GradientStop {
                    position: 0.98
                    color: root.side === "left" ? Qt.rgba(0, 0, 0, 0.34) : "transparent"
                }
                GradientStop {
                    position: 1
                    color: root.side === "left" ? Tokens.lineStrong : "transparent"
                }
            }
        }

        Shortcut {
            sequence: "Escape"
            enabled: root.open
            onActivated: SidebarState.closeAll(root.screen)
        }
    }

    PanelWindow {
        id: dismiss

        screen: root.screen
        visible: root.visible && root.open
        color: "transparent"
        exclusionMode: ExclusionMode.Ignore
        exclusiveZone: 0
        WlrLayershell.layer: WlrLayer.Top
        WlrLayershell.namespace: "ryoku-sidebar-dismiss"
        WlrLayershell.keyboardFocus: WlrKeyboardFocus.None
        anchors { top: true; bottom: true; left: true; right: true }
        mask: Region {
            width: dismiss.width
            height: dismiss.height
            Region {
                intersection: Intersection.Subtract
                x: root.side === "left" ? 0 : dismiss.width - root.surfaceWidth
                y: 0
                width: root.surfaceWidth
                height: dismiss.height
            }
        }

        MouseArea {
            anchors.fill: parent
            acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
            onPressed: SidebarState.closeAll(root.screen)
        }
    }
}
