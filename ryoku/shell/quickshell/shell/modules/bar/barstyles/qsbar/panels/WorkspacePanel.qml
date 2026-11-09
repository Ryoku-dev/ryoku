import QtQuick
import "../modules"
import Quickshell
import Quickshell.Wayland
import Ryoku.Ui.Singletons

PanelWindow {
    id: wsPanel
    required property var root

    screen: root.activePopupScreen

    color: "transparent"
    anchors { top: true; bottom: true; left: true; right: true }
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "ryoku-workspace"

    readonly property int barBottom: root.v2BarHeight
    readonly property int gap: 6

    property real reveal: root.workspaceVisible ? 1 : 0
    Behavior on reveal {
        NumberAnimation {
            duration: root.workspaceVisible ? 160 : 120
            easing.type: root.workspaceVisible ? Easing.OutCubic : Easing.InCubic
        }
    }
    visible: reveal > 0.001
    WlrLayershell.keyboardFocus: root.workspaceVisible ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None

    MouseArea { anchors.fill: parent; onClicked: root.workspaceVisible = false }

    Rectangle {
        id: card
        property int navIndex: -1
        readonly property var filteredWorkspaces: {
            var res = []
            var vs = Wm.workspaces
            for (var i = 0; i < vs.length; i++) {
                if (vs[i].special) continue
                if (root.isolateMonitors && root.activePopupScreenName && vs[i].output && vs[i].output !== root.activePopupScreenName)
                    continue
                res.push(vs[i])
            }
            res.sort(function(x, y) { return Number(x.name) - Number(y.name) })
            return res
        }
        readonly property var wsIds: {
            var a = []
            var vs = card.filteredWorkspaces
            for (var i = 0; i < vs.length; i++) a.push(vs[i].name)
            return a
        }
        width: 240
        height: col.implicitHeight + 24
        radius: reveal > 0.001 ? root.panelRadius : 0
        color: "transparent"
        border.color: root.panelBorder
        border.width: 0
        PillShadow { theme: root }
        ConnectedPanelSurface {
            root: wsPanel.root
            ownerActive: wsPanel.root.workspaceVisible
            targetX: wsPanel.root.workspaceBarX
            reveal: wsPanel.reveal
        }

        x: Math.round(Math.max(6, Math.min(root.workspaceBarX - width / 2, parent.width - width - 6)))
        y: root.barPosition === "bottom"
            ? (parent.height - barBottom - gap - height) + 2 * (1 - wsPanel.reveal)
            : (barBottom + gap) - 2 * (1 - wsPanel.reveal)
        opacity: wsPanel.reveal
        focus: root.workspaceVisible

        Keys.onPressed: function(event) {
            if (event.key === Qt.Key_Escape) { root.workspaceVisible = false; event.accepted = true }
            else if (event.key === Qt.Key_Down) { card.navIndex = card.navIndex < 0 ? 0 : Math.min(card.navIndex + 1, card.wsIds.length - 1); event.accepted = true }
            else if (event.key === Qt.Key_Up) { card.navIndex = card.navIndex <= 0 ? 0 : card.navIndex - 1; event.accepted = true }
            else if ((event.key === Qt.Key_Return || event.key === Qt.Key_Enter) && card.navIndex >= 0 && card.navIndex < card.wsIds.length) {
                root.gotoWorkspace(card.wsIds[card.navIndex]); root.workspaceVisible = false; event.accepted = true
            }
        }

        MouseArea { anchors.fill: parent; onClicked: {} }

        Column {
            id: col
            anchors.fill: parent
            anchors.margins: 12
            spacing: 8

            Item {
                width: parent.width
                height: 24
                UiText {
                    anchors.left: parent.left; anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Workspaces")
                    color: root.ink; font.family: root.mono; font.pixelSize: 13
                    font.letterSpacing: 2; font.weight: Font.Medium
                }
                UiText {
                    anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter
                    text: "✕"; color: closeMa.containsMouse ? root.seal : root.sumi; font.pixelSize: 12
                    Behavior on color { ColorAnimation { duration: 120 } }
                    MouseArea { id: closeMa; anchors.fill: parent; hoverEnabled: true; cursorShape: Qt.PointingHandCursor; onClicked: root.workspaceVisible = false }
                }
            }

            Rectangle { width: parent.width; height: 1; color: root.sep }

            Column {
                width: parent.width
                spacing: 4
                Repeater {
                    model: card.filteredWorkspaces

                    delegate: Rectangle {
                        required property var modelData
                        readonly property bool isActive: {
                            if (root.isolateMonitors && root.activePopupScreenName) {
                                var ws = Wm.workspaces
                                for (var i = 0; i < ws.length; i++) {
                                    if (!ws[i].special && ws[i].active && ws[i].output === root.activePopupScreenName)
                                        return ws[i].name === modelData.name
                                }
                            }
                            return Wm.focusedWorkspace && Wm.focusedWorkspace.name === modelData.name
                        }
                        readonly property bool navOn: card.navIndex >= 0 && card.navIndex < card.wsIds.length && card.wsIds[card.navIndex] === modelData.name
                        width: col.width
                        height: 30; radius: root.panelButtonRadius
                        color: isActive ? root.fillActive
                                : (navOn || ma.containsMouse) ? root.fillHover : root.fillIdle
                        border.color: (ma.containsMouse || isActive || navOn) ? root.seal : root.sep
                        border.width: 1
                        Behavior on color { ColorAnimation { duration: 120 } }

                        UiText {
                            anchors.left: parent.left; anchors.leftMargin: 10
                            anchors.verticalCenter: parent.verticalCenter
                            text: I18n.tr("Workspace %1").arg(modelData.name)
                            color: (ma.containsMouse || isActive) ? root.seal : root.ink
                            font.family: root.mono; font.pixelSize: 12
                            font.weight: isActive ? Font.Medium : Font.Normal
                        }
                        UiText {
                            anchors.right: parent.right; anchors.rightMargin: 10
                            anchors.verticalCenter: parent.verticalCenter
                            text: modelData.windows
                            color: root.sumiHi; font.family: root.mono; font.pixelSize: 10
                        }

                        MouseArea {
                            id: ma
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: {
                                root.gotoWorkspace(modelData.name)
                                root.workspaceVisible = false
                            }
                        }
                    }
                }
            }
        }
    }

}
