pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import Quickshell
import Quickshell.Widgets
import Caelestia.Config
import caelestia.components
import caelestia.services

StackView {
    id: root

    required property PopoutState popouts
    required property QsMenuHandle trayItem

    property real scaleOffset: 1.0
    property real fontScale: 1.0
    property bool _isSidebarOpen: false

    implicitWidth: (currentItem as SubMenu)?.hasChildren ? currentItem.implicitWidth : -Tokens.padding.extraLargeIncreased * root.scaleOffset
    implicitHeight: (currentItem as SubMenu)?.hasChildren ? currentItem.implicitHeight : -Tokens.padding.extraLargeIncreased * root.scaleOffset

    initialItem: SubMenu {
        handle: root.trayItem
        scaleOffset: root.scaleOffset
        fontScale: root.fontScale
    }

    pushEnter: NoAnim {}
    pushExit: NoAnim {}
    popEnter: NoAnim {}
    popExit: NoAnim {}

    Component {
        id: subMenuComp

        SubMenu {}
    }

    component NoAnim: Transition {
        NumberAnimation {
            duration: 0
        }
    }

    component SubMenu: Column {
        id: menu

        required property QsMenuHandle handle
        readonly property bool hasChildren: menuOpener.children.values.some(e => !e.isSeparator)
        property bool isSubMenu
        property bool shown
        property int groupRebuildCount: 0
        property real perfOpenStartedAt: 0

        required property real scaleOffset
        required property real fontScale

        padding: Tokens.padding.small * scaleOffset
        spacing: Tokens.spacing.small * scaleOffset

        opacity: shown ? 1 : 0
        scale: shown ? 1 : 0.8

        Component.onCompleted: shown = true
        StackView.onActivating: {
            shown = true;
            menu.perfOpenStartedAt = Date.now();
            menu.groupRebuildCount = 0;
        }
        StackView.onDeactivating: shown = false
        StackView.onRemoved: destroy()

        Behavior on opacity {
            Anim {
                type: Anim.DefaultEffects
            }
        }

        Behavior on scale {
            Anim {}
        }

        QsMenuOpener {
            id: menuOpener

            menu: menu.handle
        }

        property var itemGroups: []

        function updateGroups() {
            const rebuildStartedAt = Date.now();
            let groups = [];
            let currentGroup = [];
            for (let i = 0; i < groupInstantiator.count; ++i) {
                let obj = groupInstantiator.objectAt(i);
                if (obj && obj.isSeparator) {
                    if (currentGroup.length > 0) {
                        groups.push(currentGroup);
                        currentGroup = [];
                    }
                } else if (obj) {
                    currentGroup.push(obj.entry);
                }
            }
            if (currentGroup.length > 0) {
                groups.push(currentGroup);
            }
            itemGroups = groups;

            menu.groupRebuildCount += 1;
            const rebuildMs = Date.now() - rebuildStartedAt;
            console.log("[perf][TrayMenu] updateGroups ms=" + rebuildMs + " groups=" + groups.length + " rebuildCount=" + menu.groupRebuildCount);

            if (menu.perfOpenStartedAt > 0) {
                const openMs = Date.now() - menu.perfOpenStartedAt;
                console.log("[perf][TrayMenu] open/update latency ms=" + openMs + " rebuildCount=" + menu.groupRebuildCount);
                menu.perfOpenStartedAt = 0;
            }
        }

        function queueUpdateGroups() {
            groupUpdateDebounce.restart();
        }

        Timer {
            id: groupUpdateDebounce

            interval: 0
            repeat: false
            onTriggered: menu.updateGroups()
        }

        Instantiator {
            id: groupInstantiator

            model: menuOpener.children

            Item {
                required property QsMenuEntry modelData
                property bool isSeparator: modelData.isSeparator
                property var entry: modelData
            }

            onObjectAdded: menu.queueUpdateGroups()

            onObjectRemoved: menu.queueUpdateGroups()

            onModelChanged: menu.queueUpdateGroups()
        }

        Repeater {
            model: menu.itemGroups

            StyledRect {
                id: groupCard

                required property var modelData

                implicitWidth: Tokens.sizes.bar.trayMenuWidth * menu.scaleOffset + Tokens.padding.medium * 2 * menu.scaleOffset
                implicitHeight: groupLayout.implicitHeight + Tokens.padding.medium * 2 * menu.scaleOffset

                radius: Tokens.rounding.medium * menu.scaleOffset
                color: Colours.tPalette.m3surfaceContainer
                clip: true

                Column {
                    id: groupLayout

                    x: Tokens.padding.medium * menu.scaleOffset
                    y: Tokens.padding.medium * menu.scaleOffset
                    width: parent.width - Tokens.padding.medium * 2 * menu.scaleOffset
                    spacing: Tokens.spacing.small * menu.scaleOffset

                    Repeater {
                        model: groupCard.modelData

                        StyledRect {
                            id: item

                            required property var modelData

                            implicitWidth: parent.width
                            implicitHeight: childrenItem.implicitHeight

                            radius: Tokens.rounding.full * menu.scaleOffset
                            color: "transparent"

                            Loader {
                                id: childrenItem

                                asynchronous: false
                                anchors.left: parent.left
                                anchors.right: parent.right

                                sourceComponent: Item {
                                    implicitHeight: label.implicitHeight

                                    StateLayer {
                                        anchors.margins: -Tokens.padding.extraSmall / 2 * menu.scaleOffset
                                        anchors.leftMargin: -Tokens.padding.small * menu.scaleOffset
                                        anchors.rightMargin: -Tokens.padding.small * menu.scaleOffset

                                        radius: item.radius
                                        disabled: !item.modelData.enabled

                                        onClicked: {
                                            const entry = item.modelData;
                                            if (entry.hasChildren)
                                                root.push(subMenuComp.createObject(null, {
                                                    handle: entry,
                                                    isSubMenu: true,
                                                    scaleOffset: root.scaleOffset,
                                                    fontScale: root.fontScale
                                                }));
                                            else {
                                                item.modelData.triggered();
                                                root.popouts.hasCurrent = false;
                                            }
                                        }
                                    }

                                    Loader {
                                        id: icon

                                        asynchronous: false
                                        anchors.left: parent.left

                                        active: item.modelData.icon !== ""

                                        sourceComponent: IconImage {
                                            asynchronous: true
                                            implicitSize: label.implicitHeight

                                            source: item.modelData.icon
                                        }
                                    }

                                    StyledText {
                                        id: label

                                        anchors.left: icon.right
                                        anchors.leftMargin: icon.active ? Tokens.spacing.medium * menu.scaleOffset : 0

                                        text: labelMetrics.elidedText
                                        color: item.modelData.enabled ? Colours.palette.m3onSurface : Colours.palette.m3onSurfaceVariant
                                        font.pointSize: Tokens.font.body.medium.pointSize * menu.fontScale
                                    }

                                    property int trayMenuWidth: Tokens.sizes.bar.trayMenuWidth * menu.scaleOffset
                                    TextMetrics {
                                        id: labelMetrics

                                        text: item.modelData.text
                                        font: label.font

                                        elide: Text.ElideRight
                                        elideWidth: root.popouts.isHorizontal ? trayMenuWidth - (icon.active ? icon.implicitWidth + label.anchors.leftMargin : 0) - (expand.active ? expand.implicitWidth + Tokens.spacing.medium * menu.scaleOffset : 0) : 200 * menu.scaleOffset
                                    }

                                    Loader {
                                        id: expand

                                        asynchronous: false
                                        anchors.verticalCenter: parent.verticalCenter
                                        anchors.right: parent.right

                                        active: item.modelData.hasChildren

                                        sourceComponent: MaterialIcon {
                                            text: "chevron_right"
                                            color: item.modelData.enabled ? Colours.palette.m3onSurface : Colours.palette.m3onSurfaceVariant
                                            fontStyle.pointSize: Tokens.font.icon.medium.pointSize * menu.fontScale
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }

        Loader {
            asynchronous: true
            active: menu.isSubMenu

            sourceComponent: Item {
                implicitWidth: back.implicitWidth
                implicitHeight: back.implicitHeight + Tokens.spacing.extraSmall * menu.scaleOffset

                Item {
                    anchors.bottom: parent.bottom
                    implicitWidth: back.implicitWidth
                    implicitHeight: back.implicitHeight

                    StyledRect {
                        anchors.fill: parent
                        anchors.margins: -Tokens.padding.extraSmall / 2 * menu.scaleOffset
                        anchors.leftMargin: -Tokens.padding.small * menu.scaleOffset
                        anchors.rightMargin: -Tokens.padding.large * menu.scaleOffset

                        radius: Tokens.rounding.full * menu.scaleOffset
                        color: Colours.palette.m3secondaryContainer

                        StateLayer {
                            radius: parent.radius
                            color: Colours.palette.m3onSecondaryContainer
                            onClicked: root.pop()
                        }
                    }

                    Row {
                        id: back

                        anchors.verticalCenter: parent.verticalCenter

                        MaterialIcon {
                            anchors.verticalCenter: parent.verticalCenter
                            text: "chevron_left"
                            color: Colours.palette.m3onSecondaryContainer
                            fontStyle.pointSize: Tokens.font.icon.medium.pointSize * menu.fontScale
                        }

                        StyledText {
                            anchors.verticalCenter: parent.verticalCenter
                            text: qsTr("Back")
                            color: Colours.palette.m3onSecondaryContainer
                            font.pointSize: Tokens.font.body.medium.pointSize * menu.fontScale
                        }
                    }
                }
            }
        }
    }
}
