pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import shell.services
import Ryoku.Ui.Singletons

// One passive overlay per output and kind. The shipped window spans the
// desktop hole, centres its panel and rests it just above the bottom edge.
// Its eased progress keeps the surface mapped until the slide-out has finished.
PanelWindow {
    id: win

    required property var modelData
    required property string kind

    // Logical sizes follow this output's UI scale. The active item's OSD scale
    // (Shells.osdScale) applies only to the shipped panel and its slide headroom.
    readonly property real us: Tokens.uiScaleFor(modelData ? modelData.name : "")
    readonly property real pad: 16 * win.us
    readonly property real osdScale: Shells.osdScale
    readonly property real slide: 18 * win.us
    // Store designs own their size and edge choice while the shell keeps the
    // window, input policy and timing.
    readonly property bool storeRuns: designLoader.storeRuns
    readonly property bool stockRuns: Designs.osdReady && !win.storeRuns

    property real prog: feed.flashing ? 1 : 0
    Behavior on prog {
        NumberAnimation {
            duration: Motion.effects
            easing.type: Easing.OutCubic
        }
    }

    // A store design's reveal keeps its window mapped long enough to finish
    // an exit animation; the shipped design uses the host's eased progress.
    screen: modelData
    visible: Designs.osdReady
        && (feed.flashing || (win.storeRuns ? designLoader.reveal : win.prog) > 0.01)
        && !Config.styleOwnsFeedback
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

    OsdSource {
        id: feed
        kind: win.kind
    }

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
            kind: win.kind
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
        kind: win.kind
        screen: win.modelData
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
