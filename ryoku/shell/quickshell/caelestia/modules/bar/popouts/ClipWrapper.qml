pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Caelestia.Config
import caelestia.components
import caelestia.services
import caelestia.modules.bar.popouts

Item {
    id: root

    required property ShellScreen screen
    Config.screen: screen.name
    required property var bar
    required property real borderThickness
    required property DrawerVisibilities visibilities

    readonly property alias content: content
    readonly property bool isHorizontal: bar.isHorizontal
    property real offsetScale: content.isDetached || content.hasCurrent ? 0 : 1
    // Where x is heading; x itself animates towards it.
    readonly property real targetX: {
        if (content.isDetached)
            return (parent.width - content.nonAnimWidth) / 2;
        if (isHorizontal) {
            if (content.sidebarOpen && !content.isDockPopout)
                return parent.width - content.nonAnimWidth;

            // An item left of the open sidebar gets its popout kept clear of it, so
            // the sidebar doesn't have to be pushed out of the way.
            const right = root.visibilities.sidebar && !content.underSidebar ? parent.width - Visibilities.sidebarWidthFor(Tokens.sizes.sidebar.width) - Tokens.spacing.extraLarge : parent.width;
            const off = content.currentCenter - parent.leftMargin - content.nonAnimWidth / 2;
            const diff = right - Math.floor(off + content.nonAnimWidth);
            if (diff < 0)
                return Math.max(off + diff, 0);
            return Math.max(off, 0);
        }
        if (bar.position === "right")
            return parent.width - implicitWidth;
        return 0;
    }

    visible: width > 0 && height > 0
    clip: true

    implicitWidth: isHorizontal ? content.implicitWidth : content.implicitWidth * (1 - offsetScale)
    implicitHeight: isHorizontal ? content.implicitHeight * (1 - offsetScale) : content.implicitHeight

    x: targetX
    y: {
        if (content.isDetached)
            return (parent.height - content.nonAnimHeight) / 2;
        if (isHorizontal) {
            if (bar.position === "bottom")
                return parent.height - implicitHeight;
            return 0;
        }

        const off = content.currentCenter - parent.topMargin - content.nonAnimHeight / 2;
        const diff = parent.height - Math.floor(off + content.nonAnimHeight);
        if (diff < 0)
            return off + diff;
        return Math.max(off, 0);
    }

    Behavior on offsetScale {
        Anim {}
    }

    Behavior on x {
        enabled: content.isDetached || isHorizontal

        Anim {
            duration: content.animLength
            easing: content.animCurve
        }
    }

    Behavior on y {
        enabled: content.isDetached || (!isHorizontal && root.offsetScale < 1)

        Anim {
            duration: content.animLength
            easing: content.animCurve
        }
    }

    Wrapper {
        id: content

        screen: root.screen
        offsetScale: root.offsetScale
        visibilities: root.visibilities
        underSidebar: !root.isHorizontal || content.currentCenter - root.parent.leftMargin >= root.parent.width - Visibilities.sidebarWidthFor(Tokens.sizes.sidebar.width)

        anchors.leftMargin: bar.position === "left" ? (-implicitWidth - 5) * root.offsetScale : 0
        anchors.rightMargin: bar.position === "right" ? (-implicitWidth - 5) * root.offsetScale : 0
        anchors.topMargin: bar.position === "top" ? (-implicitHeight - 5) * root.offsetScale : 0
        anchors.bottomMargin: bar.position === "bottom" ? (-implicitHeight - 5) * root.offsetScale : 0

        states: [
            State {
                name: "left"
                when: bar.position === "left"

                AnchorChanges {
                    target: content
                    anchors.left: parent.left
                    anchors.right: undefined
                    anchors.top: undefined
                    anchors.bottom: undefined
                    anchors.verticalCenter: parent.verticalCenter
                    anchors.horizontalCenter: undefined
                }
            },
            State {
                name: "right"
                when: bar.position === "right"

                AnchorChanges {
                    target: content
                    anchors.left: undefined
                    anchors.right: parent.right
                    anchors.top: undefined
                    anchors.bottom: undefined
                    anchors.verticalCenter: parent.verticalCenter
                    anchors.horizontalCenter: undefined
                }
            },
            State {
                name: "top"
                when: bar.position === "top"

                AnchorChanges {
                    target: content
                    anchors.left: undefined
                    anchors.right: undefined
                    anchors.top: parent.top
                    anchors.bottom: undefined
                    anchors.verticalCenter: undefined
                    anchors.horizontalCenter: parent.horizontalCenter
                }
            },
            State {
                name: "bottom"
                when: bar.position === "bottom"

                AnchorChanges {
                    target: content
                    anchors.left: undefined
                    anchors.right: undefined
                    anchors.top: undefined
                    anchors.bottom: parent.bottom
                    anchors.verticalCenter: undefined
                    anchors.horizontalCenter: parent.horizontalCenter
                }
            }
        ]
    }
}
