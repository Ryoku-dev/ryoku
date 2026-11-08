pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import shell.services
import Ryoku.Ui.Singletons

// One passive overlay per output and kind. The shipped window spans the
// desktop hole, centres its panel and rests it just above the bottom edge.
// Its eased progress keeps the surface mapped until the slide-out has finished.
// a store design keeps its window since it loads async and owns its reveal
Scope {
    id: host

    required property var modelData
    required property string kind

    readonly property bool storeChosen: Designs.osd !== Designs.shipped && Designs.osdUrl !== ""
    // mirrored here so the loader never reads its own item
    property bool lingering: false

    OsdSource {
        id: feed
        kind: host.kind
    }

    LazyLoader {
        id: windowLoader
        active: !Config.styleOwnsFeedback
            && (host.storeChosen || feed.flashing || host.lingering)

        PanelWindow {
            id: win

            // Logical sizes follow this output's UI scale. The active item's OSD scale
            // (Shells.osdScale) applies only to the shipped panel and its slide headroom.
            readonly property real us: Tokens.uiScaleFor(host.modelData ? host.modelData.name : "")
            readonly property real pad: 16 * win.us
            readonly property real osdScale: Shells.osdScale
            readonly property real slide: 18 * win.us
            // Store designs own their size and edge choice while the shell keeps the
            // window, input policy and timing.
            readonly property bool storeRuns: designLoader.storeRuns
            readonly property bool stockRuns: Designs.osdReady && !win.storeRuns
            readonly property bool lingering: (win.storeRuns ? designLoader.reveal : win.prog) > 0.01
            onLingeringChanged: host.lingering = win.lingering
            Component.onDestruction: host.lingering = false

            // starts at 0 and binds once built, so the reveal animates in even though the trigger is already on
            property real prog: 0
            Behavior on prog {
                NumberAnimation {
                    duration: Motion.effects
                    easing.type: Easing.OutCubic
                }
            }
            Component.onCompleted: win.prog = Qt.binding(() => feed.flashing ? 1 : 0)

            // A store design's reveal keeps its window mapped long enough to finish
            // an exit animation; the shipped design uses the host's eased progress.
            screen: host.modelData
            visible: Designs.osdReady && (feed.flashing || win.lingering)
            color: "transparent"
            exclusionMode: ExclusionMode.Normal
            exclusiveZone: 0
            WlrLayershell.layer: WlrLayer.Overlay
            WlrLayershell.keyboardFocus: WlrKeyboardFocus.None
            WlrLayershell.namespace: "ryoku-osd"

            // Single-edge anchors let layer shell centre the other axis. Center leaves
            // every edge free; the shipped panel keeps its original bottom span.
            anchors.bottom: !win.storeRuns || designLoader.placement === "bottom"
            anchors.top: win.storeRuns && designLoader.placement === "top"
            anchors.left: !win.storeRuns || designLoader.placement === "left"
            anchors.right: !win.storeRuns || designLoader.placement === "right"
            margins.bottom: win.storeRuns && designLoader.placement === "bottom"
                ? 24 * win.us : (!win.storeRuns ? 24 * win.osdScale * win.us : 0)
            margins.top: win.storeRuns && designLoader.placement === "top" ? 24 * win.us : 0
            margins.left: win.storeRuns && designLoader.placement === "left" ? 24 * win.us : 0
            margins.right: win.storeRuns && designLoader.placement === "right" ? 24 * win.us : 0

            implicitWidth: win.storeRuns && designLoader.item
                ? designLoader.item.implicitWidth : 0
            implicitHeight: win.storeRuns && designLoader.item
                ? designLoader.item.implicitHeight
                : box.height * win.osdScale + win.slide * win.osdScale

            Rectangle {
                id: box
                visible: win.stockRuns
                anchors.horizontalCenter: parent.horizontalCenter
                anchors.bottom: parent.bottom
                width: osd.implicitWidth + win.pad * 2
                height: osd.implicitHeight + win.pad * 2
                radius: Theme.radiusWindow * win.us
                color: Theme.surface
                opacity: Theme.windowOpacity * win.prog
                border.width: Theme.borderWidth
                border.color: Theme.outline
                antialiasing: true
                scale: win.osdScale
                transformOrigin: Item.Bottom
                transform: Translate { y: -win.prog * win.slide * win.osdScale }

                Osd {
                    id: osd
                    anchors.fill: parent
                    anchors.margins: win.pad
                    kind: host.kind
                    us: win.us
                    value: feed.value
                    maxValue: feed.maxValue
                    muted: feed.muted
                    iconName: feed.iconName
                }
            }

            OsdDesignLoader {
                id: designLoader
                anchors.fill: parent
                kind: host.kind
                screen: host.modelData
                us: win.us
                shown: feed.flashing
                value: feed.value
                maxValue: feed.maxValue
                muted: feed.muted
                icon: feed.iconName
                iconSource: Qt.resolvedUrl("../../components/icons/"
                    + feed.iconName + "-symbolic.svg")
                label: feed.label
            }

            // This readout is passive and must never consume a pointer press.
            mask: Region {}
        }
    }
}
