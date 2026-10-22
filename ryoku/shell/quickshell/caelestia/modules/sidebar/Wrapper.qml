pragma ComponentBehavior: Bound

import QtQuick
import Caelestia
import Caelestia.Config
import caelestia.components
import caelestia.services

Item {
    id: root

    required property DrawerVisibilities visibilities
    property var popouts
    property var utilities
    // Set while a popout pushes the sidebar; its height is already animated.
    property bool followsPopout: false
    readonly property Props props: Props {}
    readonly property bool shouldBeActive: visibilities.sidebar && Config.sidebar.enabled && !visibilities.overview
    property bool keepLoaded: false
    property real offsetScale: shouldBeActive ? 0 : 1
    // The sidebar sits against the right edge unless the bar is there.
    readonly property bool onRight: Config.bar.position !== "right"
    readonly property int defaultWidth: Tokens.sizes.sidebar.width
    readonly property int minWidth: Math.round(defaultWidth * 0.8)
    readonly property int maxWidth: Math.max(minWidth, Math.round((parent?.width ?? 0) * 0.5))

    visible: offsetScale < 1
    anchors.leftMargin: Config.bar.position === "right" ? (-implicitWidth - 5) * offsetScale : 0
    anchors.rightMargin: Config.bar.position !== "right" ? (-implicitWidth - 5) * offsetScale : 0
    implicitWidth: Visibilities.sidebarWidthFor(defaultWidth)
    opacity: 1 - offsetScale

    Connections {
        function onKeepLoadedChanged(): void {
            root.keepLoaded = content.item ? content.item.keepLoaded : false;
        }

        target: content.item
        ignoreUnknownSignals: true
    }
    Behavior on offsetScale {
        Anim {}
    }
    // Pinning or unpinning switches between the dragged and the default width;
    // while dragging, the edge follows the cursor directly.
    Behavior on implicitWidth {
        enabled: !resizeHandle.pressed

        Anim {}
    }
    // Animating on top of the popout's own height animation makes the edge lag
    // behind it, leaving a gap or an overlap while switching popouts.
    Behavior on anchors.bottomMargin {
        enabled: !root.followsPopout

        Anim {}
    }
    Loader {
        id: content

        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.left: parent.left
        anchors.leftMargin: Tokens.padding.large
        anchors.margins: CUtils.clamp(anchors.leftMargin - Config.border.thickness, 0, anchors.leftMargin)
        anchors.bottomMargin: 0
        active: root.shouldBeActive || root.visible || root.keepLoaded
        sourceComponent: Content {
            implicitWidth: root.implicitWidth - content.anchors.leftMargin - content.anchors.margins
            props: root.props
            visibilities: root.visibilities
            popouts: root.popouts
            utilities: root.utilities
        }
    }

    // Drag the inner edge of a pinned sidebar to resize it; double-click to go
    // back to the default width.
    MouseArea {
        id: resizeHandle

        property real pressX
        property real pressWidth

        function finish(): void {
            if (!Visibilities.sidebarResizing)
                return;
            Visibilities.setSidebarWidth(Visibilities.sidebarWidth);
            Visibilities.sidebarResizing = false;
        }

        anchors.top: parent.top
        anchors.bottom: parent.bottom
        x: root.onRight ? 0 : root.width - width
        z: 1
        width: root.onRight ? content.anchors.leftMargin : Math.max(content.anchors.margins, 6)
        enabled: Visibilities.sidebarPinned && root.shouldBeActive
        visible: enabled
        hoverEnabled: true
        preventStealing: true
        cursorShape: Qt.SizeHorCursor

        onPressed: mouse => {
            pressX = mapToItem(null, mouse.x, 0).x;
            pressWidth = root.implicitWidth;
            Visibilities.sidebarResizing = true;
        }
        onPositionChanged: mouse => {
            if (!pressed)
                return;
            const dx = mapToItem(null, mouse.x, 0).x - pressX;
            let w = CUtils.clamp(pressWidth + (root.onRight ? -dx : dx), root.minWidth, root.maxWidth);
            // Snap back onto the default width when passing near it.
            if (Math.abs(w - root.defaultWidth) < 12)
                w = root.defaultWidth;
            w = Math.round(w);
            Visibilities.sidebarWidth = w === root.defaultWidth ? 0 : w;
        }
        onReleased: finish()
        onCanceled: finish()
        onDoubleClicked: Visibilities.setSidebarWidth(0)

        StyledRect {
            anchors.centerIn: parent
            implicitWidth: resizeHandle.pressed ? 6 : 4
            implicitHeight: resizeHandle.pressed ? 72 : resizeHandle.containsMouse ? 56 : 40
            radius: Tokens.rounding.full
            color: resizeHandle.pressed ? Colours.palette.m3primary : Colours.palette.m3outline
            opacity: resizeHandle.pressed || resizeHandle.containsMouse ? 1 : 0

            Behavior on implicitWidth {
                Anim {
                    type: Anim.FastSpatial
                }
            }
            Behavior on implicitHeight {
                Anim {
                    type: Anim.FastSpatial
                }
            }
            Behavior on opacity {
                Anim {
                    type: Anim.DefaultEffects
                }
            }
            Behavior on color {
                CAnim {}
            }
        }
    }
}
