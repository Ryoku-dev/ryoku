pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Effects
import QtQuick.Layouts
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.components.controls
import caelestia.components.effects
import caelestia.services
import caelestia.modules.drawers

MouseArea {
    id: root

    enum Side {
        Top,
        Bottom,
        Left,
        Right
    }

    required property Item attachTo
    property int attachSideX: Menu.Right
    property int attachSideY: Menu.Bottom
    property int thisSideX: Menu.Right
    property int thisSideY: Menu.Top
    property real marginX
    property real marginY

    property list<MenuItem> items
    property var dynamicModel: items
    property MenuItem active: dynamicModel[0] ?? null
    property bool expanded
    property bool rightClickReposition: false
    property real maxHeight: 320
    readonly property alias backgroundItem: menu
    property bool transparentBackground: false
    // Unfold sideways from the anchor with a short, non-overshooting curve
    // instead of dropping down.
    property bool revealHorizontal: false
    readonly property int firstVisibleIndex: {
        const model = dynamicModel ?? [];
        for (let i = 0; i < model.length; i++)
            if (model[i]?.visible)
                return i;
        return -1;
    }
    readonly property int lastVisibleIndex: {
        const model = dynamicModel ?? [];
        for (let i = model.length - 1; i >= 0; i--)
            if (model[i]?.visible)
                return i;
        return -1;
    }

    signal itemSelected(item: MenuItem)
    signal rightClickedAt(real x, real y)

    // Plays the opening animation again, e.g. after the menu moved while open.
    function replayReveal(): void {
        if (!revealHorizontal || !expanded)
            return;
        revealOut.stop();
        revealIn.stop();
        menu.hScale = 0;
        revealIn.start();
    }

    onExpandedChanged: {
        if (!revealHorizontal)
            return;
        revealIn.stop();
        revealOut.stop();
        if (expanded)
            revealIn.start();
        else
            revealOut.start();
    }

    parent: {
        let node = root.attachTo;
        let interactionsNode = null;
        
        while (node && node.parent) {
            if (node.utilitiesShortcutActive !== undefined) {
                interactionsNode = node;
            }
            node = node.parent;
        }

        if (interactionsNode) {
            return interactionsNode;
        }

        return node || root.parent;
    }
    anchors.fill: parent

    enabled: expanded
    acceptedButtons: rightClickReposition ? Qt.LeftButton | Qt.RightButton : Qt.LeftButton
    onClicked: mouse => {
        if (rightClickReposition && mouse.button === Qt.RightButton) {
            rightClickedAt(mouse.x, mouse.y);
            return;
        }
        expanded = false;
    }

    opacity: expanded ? 1 : 0
    visible: opacity > 0

    Behavior on opacity {
        Anim {
            type: root.revealHorizontal ? Anim.FastEffects : Anim.DefaultEffects
        }
    }

    NumberAnimation {
        id: revealIn

        target: menu
        property: "hScale"
        to: 1
        duration: Tokens.anim.durations.small
        easing: Tokens.anim.emphasizedDecel
    }

    NumberAnimation {
        id: revealOut

        target: menu
        property: "hScale"
        to: 0
        duration: Math.round(Tokens.anim.durations.small * 0.75)
        easing: Tokens.anim.emphasizedAccel
    }

    TransformWatcher {
        id: watcher

        a: root.parent
        b: root.attachTo
    }

    Elevation {
        id: menu

        property string vAnchor: "none"
        property string hAnchor: "none"
        property real offsetScale: 1 - animScale
        property real vScale: root.expanded ? 1 : 0.0
        property real hScale: 0
        readonly property real animScale: root.revealHorizontal ? hScale : vScale

        x: {
            watcher.transform;
            const item = root.attachTo;
            if (!item || !root.parent)
                return 0;
            let off = root.attachSideX === Menu.Left ? 0 : item.width;
            if (root.thisSideX === Menu.Right)
                off -= width;
            const pt = item.mapToItem(root.parent, off, 0);
            return (pt ? pt.x : 0) + root.marginX;
        }
        y: {
            watcher.transform;
            const item = root.attachTo;
            if (!item || !root.parent)
                return 0;
            let off = root.attachSideY === Menu.Top ? 0 : item.height;
            if (root.thisSideY === Menu.Bottom)
                off -= height;
            const pt = item.mapToItem(root.parent, 0, off);
            return (pt ? pt.y : 0) + root.marginY;
        }

        radius: Tokens.rounding.large
        level: root.transparentBackground ? 0 : 2
        implicitWidth: Math.max(200, column.implicitWidth + Tokens.padding.extraSmall * 2)
        implicitHeight: Math.min(root.maxHeight, column.implicitHeight + Tokens.padding.extraSmall * 2)
        width: root.revealHorizontal ? implicitWidth * animScale : implicitWidth
        height: root.revealHorizontal ? implicitHeight : implicitHeight * animScale
        clip: root.revealHorizontal

        Behavior on vScale {
            enabled: !root.revealHorizontal

            Anim {}
        }

        MouseArea {
            anchors.fill: parent
            hoverEnabled: true
            onWheel: e => e.accepted = true
            onClicked: {}
        }

        StyledRect {
            // Sideways, the content stays pinned to the edge the menu grows
            // from and trails it by a few pixels while the panel unfolds.
            x: {
                if (!root.revealHorizontal)
                    return 0;
                const trail = (1 - menu.animScale) * Tokens.padding.extraLarge;
                return root.thisSideX === Menu.Right ? menu.width - menu.implicitWidth + trail : -trail;
            }
            y: root.thisSideY === Menu.Bottom ? menu.height - menu.implicitHeight : 0
            width: menu.implicitWidth
            height: menu.implicitHeight
            opacity: root.revealHorizontal ? Math.min(1, menu.animScale * 1.5) : 1

            transform: Scale {
                yScale: root.revealHorizontal ? 1 : menu.animScale
                origin.y: root.thisSideY === Menu.Bottom ? menu.implicitHeight : 0
            }
            
            radius: parent.radius
            color: root.transparentBackground
                ? Qt.alpha(Colours.palette.m3surfaceContainerLow, 0)
                : (GlobalConfig.appearance.pitchBlack
                    ? "#000000"
                    : Colours.palette.m3surfaceContainerLow)

            Flickable {
                id: flickable

                anchors.fill: parent
                anchors.margins: Tokens.padding.extraSmall
                contentWidth: width
                contentHeight: column.implicitHeight
                clip: true

                interactive: contentHeight > height

                ScrollBar.vertical: StyledScrollBar {
                    flickable: flickable
                }

                ColumnLayout {
                    id: column

                    width: parent.width
                    spacing: 0

                    Repeater {
                        id: repeater

                        model: root.dynamicModel

                    StyledRect {
                        id: item

                        required property int index
                        required property MenuItem modelData
                        readonly property bool active: modelData === root?.active

                        visible: modelData?.visible ?? false

                        Layout.fillWidth: true
                        implicitWidth: menuOptionRow.implicitWidth + Tokens.padding.medium * 2
                        implicitHeight: visible ? menuOptionRow.implicitHeight + Tokens.padding.medium * 2 : 0


                        radius: active ? Tokens.rounding.medium : Tokens.rounding.extraSmall
                        topLeftRadius: index === root.firstVisibleIndex ? Tokens.rounding.medium : radius
                        topRightRadius: index === root.firstVisibleIndex ? Tokens.rounding.medium : radius
                        bottomLeftRadius: index === root.lastVisibleIndex ? Tokens.rounding.medium : radius
                        bottomRightRadius: index === root.lastVisibleIndex ? Tokens.rounding.medium : radius

                        color: "transparent"

                        Behavior on radius {
                            Anim {}
                        }

                        StateLayer {
                            topLeftRadius: parent.topLeftRadius
                            topRightRadius: parent.topRightRadius
                            bottomLeftRadius: parent.bottomLeftRadius
                            bottomRightRadius: parent.bottomRightRadius

                            color: Colours.palette.m3onSurface
                            disabled: !root.expanded
                            onClicked: {
                                root.itemSelected(item.modelData);
                                root.active = item.modelData;
                                item.modelData.clicked();
                                root.expanded = false;
                            }
                        }

                        RowLayout {
                            id: menuOptionRow

                            anchors.fill: parent
                            anchors.margins: Tokens.padding.medium
                            spacing: Tokens.spacing.small

                            MaterialIcon {
                                Layout.alignment: Qt.AlignVCenter
                                text: item.modelData?.icon ?? ""
                                color: Colours.palette.m3onSurfaceVariant
                            }

                            StyledText {
                                Layout.alignment: Qt.AlignVCenter
                                Layout.fillWidth: true
                                text: item.modelData?.text ?? ""
                                color: Colours.palette.m3onSurface
                            }

                            Loader {
                                asynchronous: true
                                Layout.alignment: Qt.AlignVCenter
                                active: item.modelData?.trailingIcon.length > 0
                                visible: active

                                sourceComponent: MaterialIcon {
                                    text: item.modelData.trailingIcon
                                    color: Colours.palette.m3onSurfaceVariant
                                }
                            }
                        }
                    }
                }
            }
        }
        }
    }
}
