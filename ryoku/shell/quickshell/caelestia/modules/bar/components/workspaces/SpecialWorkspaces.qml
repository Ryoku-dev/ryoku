pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.components.effects
import caelestia.services
import caelestia.utils

Item {
    id: root

    required property ShellScreen screen
    // Loosely typed: Kwin.monitorFor() returns a plain object, not a compositor monitor.
    readonly property var monitor: Kwin.monitorFor(screen)
    readonly property string activeSpecial: (Config.bar.workspaces.perMonitor ? monitor : Kwin.focusedMonitor)?.lastIpcObject.specialWorkspace?.name ?? ""

    readonly property bool isHorizontal: Config.bar.position === "top" || Config.bar.position === "bottom"


    layer.enabled: true
    layer.effect: Mask {
        maskSource: mask
    }

    Item {
        id: mask

        anchors.fill: parent
        layer.enabled: true
        visible: false

        Rectangle {
            anchors.fill: parent
            radius: Tokens.rounding.full

            gradient: Gradient {
                orientation: isHorizontal ? Gradient.Horizontal : Gradient.Vertical

                GradientStop {
                    position: 0
                    color: Qt.rgba(0, 0, 0, 0)
                }
                GradientStop {
                    position: 0.3
                    color: Qt.rgba(0, 0, 0, 1)
                }
                GradientStop {
                    position: 0.7
                    color: Qt.rgba(0, 0, 0, 1)
                }
                GradientStop {
                    position: 1
                    color: Qt.rgba(0, 0, 0, 0)
                }
            }
        }

        Rectangle {
            anchors.top: parent.top
            anchors.bottom: isHorizontal ? parent.bottom : undefined
            anchors.left: parent.left
            anchors.right: isHorizontal ? undefined : parent.right

            radius: Tokens.rounding.full
            implicitWidth: isHorizontal ? parent.width / 2 : 0
            implicitHeight: isHorizontal ? 0 : parent.height / 2
            opacity: isHorizontal ? (view.contentX > 0 ? 0 : 1) : (view.contentY > 0 ? 0 : 1)

            Behavior on opacity {
                Anim {
                    type: Anim.DefaultEffects
                }
            }
        }

        Rectangle {
            anchors.bottom: parent.bottom
            anchors.top: isHorizontal ? parent.top : undefined
            anchors.right: parent.right
            anchors.left: isHorizontal ? undefined : parent.left

            radius: Tokens.rounding.full
            implicitWidth: isHorizontal ? parent.width / 2 : 0
            implicitHeight: isHorizontal ? 0 : parent.height / 2
            opacity: isHorizontal ? (view.contentX < view.contentWidth - parent.width + Tokens.padding.extraSmall ? 0 : 1) : (view.contentY < view.contentHeight - parent.height + Tokens.padding.extraSmall ? 0 : 1)

            Behavior on opacity {
                Anim {
                    type: Anim.DefaultEffects
                }
            }
        }
    }

    ListView {
        id: view

        anchors.fill: parent
        spacing: Tokens.spacing.medium
        interactive: false

        orientation: isHorizontal ? ListView.Horizontal : ListView.Vertical

        currentIndex: model.values.findIndex(w => w.name === root.activeSpecial)
        onCurrentIndexChanged: currentIndex = Qt.binding(() => model.values.findIndex(w => w.name === root.activeSpecial))

        model: ScriptModel {
            // Kwin.specialWorkspaces is a plain array, so filter it directly.
            values: Kwin.specialWorkspaces.filter(w => !Config.bar.workspaces.perMonitor || w.monitor === root.monitor.name)
        }

        preferredHighlightBegin: 0
        preferredHighlightEnd: isHorizontal ? width : height
        highlightRangeMode: ListView.StrictlyEnforceRange

        highlightFollowsCurrentItem: false

        highlight: Item {
            x: isHorizontal ? (view.currentItem?.x ?? 0) : 0
            y: isHorizontal ? 0 : (view.currentItem?.y ?? 0)
            implicitWidth: isHorizontal ? ((view.currentItem as SpecialWsDelegate)?.size ?? 0) : 0
            implicitHeight: isHorizontal ? 0 : ((view.currentItem as SpecialWsDelegate)?.size ?? 0)

            Behavior on x {
                enabled: isHorizontal

                Anim {}
            }

            Behavior on y {
                enabled: !isHorizontal

                Anim {}
            }
        }

        delegate: SpecialWsDelegate {}

        add: Transition {
            Anim {
                properties: "scale"
                from: 0
                to: 1
                easing: Tokens.anim.standardDecel
            }
        }

        remove: Transition {
            Anim {
                property: "scale"
                to: 0.5
                type: Anim.StandardSmall
            }
            Anim {
                property: "opacity"
                to: 0
                type: Anim.StandardSmall
            }
        }

        move: Transition {
            Anim {
                properties: "scale"
                to: 1
                easing: Tokens.anim.standardDecel
            }
            Anim {
                properties: "x,y"
            }
        }

        displaced: Transition {
            Anim {
                properties: "scale"
                to: 1
                easing: Tokens.anim.standardDecel
            }
            Anim {
                properties: "x,y"
            }
        }
    }

    Loader {
        asynchronous: true
        active: Config.bar.workspaces.activeIndicator
        anchors.fill: parent

        sourceComponent: Item {
            StyledClippingRect {
                id: indicator

                anchors.left: isHorizontal ? undefined : parent.left
                anchors.right: isHorizontal ? undefined : parent.right
                anchors.top: isHorizontal ? parent.top : undefined
                anchors.bottom: isHorizontal ? parent.bottom : undefined

                x: isHorizontal ? ((view.currentItem?.x ?? 0) - view.contentX) : 0
                y: isHorizontal ? 0 : ((view.currentItem?.y ?? 0) - view.contentY)
                implicitWidth: isHorizontal ? ((view.currentItem as SpecialWsDelegate)?.size ?? 0) : view.width
                implicitHeight: isHorizontal ? view.height : ((view.currentItem as SpecialWsDelegate)?.size ?? 0)

                color: Colours.palette.m3tertiary
                radius: Tokens.rounding.full

                Colouriser {
                    source: view
                    sourceColor: Colours.palette.m3onSurface
                    colorizationColor: Colours.palette.m3onTertiary

                    anchors.horizontalCenter: isHorizontal ? undefined : parent.horizontalCenter
                    anchors.verticalCenter: isHorizontal ? parent.verticalCenter : undefined

                    x: isHorizontal ? -indicator.x : 0
                    y: isHorizontal ? 0 : -indicator.y
                    implicitWidth: view.width
                    implicitHeight: view.height
                }

                Behavior on x {
                    enabled: isHorizontal

                    Anim {
                        type: Anim.Emphasized
                    }
                }

                Behavior on y {
                    enabled: !isHorizontal

                    Anim {
                        type: Anim.Emphasized
                    }
                }

                Behavior on implicitWidth {
                    enabled: isHorizontal

                    Anim {
                        type: Anim.Emphasized
                    }
                }

                Behavior on implicitHeight {
                    enabled: !isHorizontal

                    Anim {
                        type: Anim.Emphasized
                    }
                }
            }
        }
    }

    MouseArea {
        property real startPos

        anchors.fill: view

        drag.target: view.contentItem

        drag.axis: isHorizontal ? Drag.XAxis : Drag.YAxis
        drag.maximumX: 0
        drag.minimumX: isHorizontal ? Math.min(0, view.width - view.contentWidth - Tokens.padding.small) : 0
        drag.maximumY: 0
        drag.minimumY: isHorizontal ? 0 : Math.min(0, view.height - view.contentHeight - Tokens.padding.extraSmall)

        onPressed: event => startPos = isHorizontal ? event.x : event.y

        onClicked: event => {
            const currentPos = isHorizontal ? event.x : event.y;
            if (Math.abs(currentPos - startPos) > drag.threshold)
                return;

                            return;

            const ws = view.itemAt(event.x, event.y) as SpecialWsDelegate;
            if (ws?.modelData)
                Kwin.dispatch(Kwin.usingLua ? `hl.dsp.workspace.toggle_special("${ws.modelData.name.slice(8)}")` : `togglespecialworkspace ${ws.modelData.name.slice(8)}`);
            else
                Kwin.dispatch(Kwin.usingLua ? 'hl.dsp.workspace.toggle_special("special")' : "togglespecialworkspace special");
        }
    }

    component SpecialWsDelegate: GridLayout {
        id: ws

        required property var modelData
        readonly property int size: isHorizontal ? (label.Layout.preferredWidth + (hasWindows ? windows.implicitWidth + Tokens.padding.extraSmall : 0)) : (label.Layout.preferredHeight + (hasWindows ? windows.implicitHeight + Tokens.padding.extraSmall : 0))
        property int wsId
        property string icon
        property bool hasWindows

        columns: isHorizontal ? -1 : 1
        rows: isHorizontal ? 1 : -1
        flow: isHorizontal ? GridLayout.LeftToRight : GridLayout.TopToBottom

        anchors.left: isHorizontal ? undefined : view.contentItem.left
        anchors.right: isHorizontal ? undefined : view.contentItem.right
        anchors.top: isHorizontal ? view.contentItem.top : undefined
        anchors.bottom: isHorizontal ? view.contentItem.bottom : undefined

        columnSpacing: 0
        rowSpacing: 0

        Component.onCompleted: {
            wsId = modelData.id;
            icon = Icons.getSpecialWsIcon(modelData.name);
            hasWindows = Config.bar.workspaces.showWindowsOnSpecialWorkspaces && modelData.lastIpcObject.windows > 0;
        }

        Connections {
            function onIdChanged(): void {
                if (ws.modelData)
                    ws.wsId = ws.modelData.id;
            }

            function onNameChanged(): void {
                if (ws.modelData)
                    ws.icon = Icons.getSpecialWsIcon(ws.modelData.name);
            }

            function onLastIpcObjectChanged(): void {
                if (ws.modelData) {
                    ws.hasWindows = root.Config.bar.workspaces.showWindowsOnSpecialWorkspaces && ws.modelData.lastIpcObject.windows > 0;
                    ws.wsId = ws.modelData.id;
                }
            }

            target: ws.modelData
        }

        Connections {
            function onShowWindowsOnSpecialWorkspacesChanged(): void {
                if (ws.modelData)
                    ws.hasWindows = root.Config.bar.workspaces.showWindowsOnSpecialWorkspaces && ws.modelData.lastIpcObject.windows > 0;
            }

            target: root.Config.bar.workspaces
        }

        Loader {
            id: label

            asynchronous: true

            Layout.alignment: isHorizontal ? (Qt.AlignVCenter | Qt.AlignLeft) : (Qt.AlignHCenter | Qt.AlignTop)
            Layout.preferredWidth: isHorizontal ? Math.round(Tokens.sizes.bar.innerWidth * Math.max(0.6, !isNaN(Config.bar.scale) ? Config.bar.scale : 1.0)) : -1
            Layout.preferredHeight: isHorizontal ? -1 : Math.round(Tokens.sizes.bar.innerWidth * Math.max(0.6, !isNaN(Config.bar.scale) ? Config.bar.scale : 1.0))

            sourceComponent: ws.icon.length === 1 ? letterComp : iconComp

            Component {
                id: iconComp

                MaterialIcon {
                    anchors.fill: parent
                    fill: 1
                    text: ws.icon
                    verticalAlignment: Qt.AlignVCenter
                    horizontalAlignment: Qt.AlignHCenter
                }
            }

            Component {
                id: letterComp

                StyledText {
                    anchors.fill: parent
                    text: ws.icon
                    verticalAlignment: Qt.AlignVCenter
                    horizontalAlignment: Qt.AlignHCenter
                }
            }
        }

        Loader {
            id: windows

            asynchronous: true

            Layout.alignment: isHorizontal ? Qt.AlignVCenter : Qt.AlignHCenter
            Layout.fillWidth: isHorizontal && enabled
            Layout.fillHeight: !isHorizontal && enabled

            visible: active
            active: ws.hasWindows

            sourceComponent: isHorizontal ? rowComponent : columnComponent

            Behavior on Layout.preferredHeight {
                enabled: !isHorizontal

                Anim {}
            }
        }

        // MOVED COMPONENTS INSIDE DELEGATE: This fixes the "ws is not defined" error
        Component {
            id: columnComponent

            Column {
                spacing: 0

                add: Transition {
                    Anim {
                        properties: "scale"
                        from: 0
                        to: 1
                        easing: Tokens.anim.standardDecel
                    }
                }

                move: Transition {
                    Anim {
                        properties: "scale"
                        to: 1
                        easing: Tokens.anim.standardDecel
                    }
                    Anim {
                        properties: "x,y"
                    }
                }

                Repeater {
                    model: ScriptModel {
                        values: {
                            const windows = Kwin.toplevels.values.filter(c => c.workspace?.id === ws.wsId);
                            const maxIcons = root.Config.bar.workspaces.maxWindowIcons;
                            return maxIcons > 0 ? windows.slice(0, maxIcons) : windows;
                        }
                    }

                    MaterialIcon {
                        required property var modelData

                        grade: 0
                        text: Icons.getAppCategoryIcon(modelData.lastIpcObject.class, "terminal")
                        color: Colours.palette.m3onSurfaceVariant
                    }
                }
            }
        }

        Component {
            id: rowComponent

            Row {
                spacing: 0
                add: Transition {
                    Anim {
                        properties: "scale"
                        from: 0
                        to: 1
                        easing: Tokens.anim.standardDecel
                    }
                }
                move: Transition {
                    Anim {
                        properties: "scale"
                        to: 1
                        easing: Tokens.anim.standardDecel
                    }
                    Anim {
                        properties: "x,y"
                    }
                }

                Repeater {
                    model: ScriptModel {
                        values: {
                            const windows = Kwin.toplevels.values.filter(c => c.workspace?.id === ws.wsId);
                            const maxIcons = root.Config.bar.workspaces.maxWindowIcons;
                            return maxIcons > 0 ? windows.slice(0, maxIcons) : windows;
                        }
                    }

                    MaterialIcon {
                        required property var modelData

                        grade: 0
                        text: Icons.getAppCategoryIcon(modelData.lastIpcObject.class, "terminal")
                        color: Colours.palette.m3onSurfaceVariant
                    }
                }
            }
        }
    }
}
