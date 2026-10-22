pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Widgets
import Caelestia
import Caelestia.Config
import Caelestia.Models
import caelestia.components
import caelestia.components.controls
import caelestia.components.images
import caelestia.services
import caelestia.utils
import caelestia.modules.launcher.services

Item {
    id: root

    required property var modelData
    required property var list

    function clicked(): void {
        Kwin.focusWindow(root.modelData.address);
        root.list.visibilities.launcher = false;
    }

    // Unlike clicked(), stays in the switcher rather than closing the launcher —
    // the point is closing several windows in a row without re-opening it each time.
    function closeWindow(): void {
        Windows.closeWindow(root.modelData.address);
    }

    width: list.itemWidth
    implicitWidth: previewBox.maxW + Tokens.padding.largeIncreased * 2
    implicitHeight: previewBox.maxH + label.height + Tokens.spacing.small / 2 + Tokens.padding.large + Tokens.padding.medium
    scale: ListView.isCurrentItem ? 1 : 0.8
    opacity: 1
    z: ListView.isCurrentItem ? 1 : 0

    Component.onCompleted: {
        if (root.modelData) {
            WinIcons.request(root.modelData.class, root.modelData.title, root.modelData.pid ?? 0, root.modelData.address ? String(root.modelData.address) : "");
        }
    }

    HoverHandler {
        id: tileHover
    }

    StateLayer {
        anchors.leftMargin: -Tokens.padding.large
        anchors.rightMargin: -Tokens.padding.large
        radius: Tokens.rounding.medium
        onClicked: root.clicked()
    }

    StyledRect {
        id: shadowRect

        anchors.fill: previewBox
        radius: previewBox.radius
        color: "transparent"
        opacity: root.ListView.isCurrentItem ? 1 : 0

        Behavior on opacity {
            Anim { type: Anim.FastEffects }
        }
    }

    StyledClippingRect {
        id: previewBox

        readonly property real windowAspect: {
            const size = root.modelData?.size;
            if (size && size.length >= 2) {
                const w = size[0];
                const h = size[1];
                if (w > 0 && h > 0) return w / h;
            }
            return 16.0 / 9.0;
        }
        readonly property real maxW: Tokens.sizes.launcher.windowSwitcherWidth
        readonly property real maxH: maxW / 16 * 9

        anchors.horizontalCenter: parent.horizontalCenter
        y: Tokens.padding.large
        implicitWidth: {
            const h = maxW / windowAspect;
            if (h > maxH) return maxH * windowAspect;
            return maxW;
        }
        implicitHeight: {
            const w = maxH * windowAspect;
            if (w > maxW) return maxW / windowAspect;
            return maxH;
        }
        color: "transparent"
        radius: Tokens.rounding.medium

        WindowPreview {
            anchors.fill: parent
            address: root.modelData?.address ?? ""
            fallbackIcon: root.modelData ? WinIcons.sourceForClient(root.modelData) : ""
            sourceAspect: previewBox.windowAspect
        }

        StyledRect {
            anchors.top: parent.top
            anchors.right: parent.right
            anchors.margins: Tokens.padding.small
            implicitWidth: closeIcon.implicitHeight + Tokens.padding.small * 2
            implicitHeight: closeIcon.implicitHeight + Tokens.padding.small * 2
            radius: Tokens.rounding.small
            color: Colours.tPalette.m3surfaceVariant
            opacity: tileHover.hovered ? 1 : 0
            visible: opacity > 0.01

            Behavior on opacity {
                Anim {}
            }

            StateLayer {
                anchors.fill: parent
                radius: Tokens.rounding.small
                onClicked: root.closeWindow()
            }

            MaterialIcon {
                id: closeIcon

                anchors.centerIn: parent
                text: "close"
            }
        }
    }

    StyledText {
        id: label

        anchors.top: previewBox.bottom
        anchors.topMargin: Tokens.spacing.small / 2
        anchors.horizontalCenter: parent.horizontalCenter
        width: previewBox.maxW - Tokens.padding.medium * 2
        horizontalAlignment: Text.AlignHCenter
        elide: Text.ElideRight
        renderType: Text.QtRendering
        text: {
            const title = root.modelData?.title || "";
            if (root.modelData?.minimized) return `(${title})`;
            return title;
        }
        font: Tokens.font.body.medium
    }

    Behavior on scale {
        Anim { type: Anim.FastSpatial }
    }
}

