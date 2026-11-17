pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import shell.services
import Ryoku.Ui.Singletons

// One passive overlay per output and kind (volume, mic, brightness, keyboard).
// Ryoku's own look (OsdLook) follows the desktop's design and names its edge,
// its gap from that edge and its reveal; a store design names its edge and
// reveal the same way. The window is built on a trigger and kept mapped until
// the look's exit has finished, so an idle desktop holds none of them.
// a store design keeps its window since it loads async and owns its reveal
Scope {
    id: host

    required property var modelData
    required property string kind

    readonly property bool storeChosen: Designs.osd !== Designs.shipped && Designs.osdUrl !== ""
    // Kairos and Orpheus draw their own volume and brightness feedback; the
    // keyboard layout is still the shell's to show
    readonly property bool styleOwns: host.kind !== "keyboard" && Config.styleOwnsFeedback
    // mirrored here so the loader never reads its own item
    property bool lingering: false

    OsdSource {
        id: feed
        kind: host.kind
    }

    LazyLoader {
        id: windowLoader
        active: !host.styleOwns && (host.storeChosen || feed.flashing || host.lingering)

        PanelWindow {
            id: win

            // Logical sizes follow this output's UI scale; the active item's OSD
            // scale (Shells.osdScale) sizes only Ryoku's own look.
            readonly property real us: Tokens.uiScaleFor(host.modelData ? host.modelData.name : "")
            readonly property bool storeRuns: designLoader.storeRuns
            readonly property bool stockRuns: Designs.osdReady && !win.storeRuns
            readonly property bool lingering: (win.storeRuns ? designLoader.reveal : look.reveal) > 0.01
            onLingeringChanged: host.lingering = win.lingering
            Component.onDestruction: host.lingering = false

            // A single edge anchor lets layer shell centre the other axis, and none
            // centres both. The gap is kept from that edge, outside any bar's or
            // dock's exclusive zone, so a look anchored with no gap sits right
            // against the system's chrome.
            readonly property string placement: win.storeRuns ? designLoader.placement : look.placement
            readonly property real gap: win.storeRuns ? 24 * win.us : look.gap

            screen: host.modelData
            visible: Designs.osdReady && (feed.flashing || win.lingering)
            color: "transparent"
            exclusionMode: ExclusionMode.Normal
            exclusiveZone: 0
            WlrLayershell.layer: WlrLayer.Overlay
            WlrLayershell.keyboardFocus: WlrKeyboardFocus.None
            WlrLayershell.namespace: host.kind === "keyboard" ? "ryoku-keyboard-osd" : "ryoku-osd"

            anchors.top: win.placement === "top"
            anchors.bottom: win.placement === "bottom"
            anchors.left: win.placement === "left"
            anchors.right: win.placement === "right"
            margins.top: win.placement === "top" ? win.gap : 0
            margins.bottom: win.placement === "bottom" ? win.gap : 0
            margins.left: win.placement === "left" ? win.gap : 0
            margins.right: win.placement === "right" ? win.gap : 0

            implicitWidth: win.storeRuns
                ? (designLoader.item ? designLoader.item.implicitWidth : 0) : look.implicitWidth
            implicitHeight: win.storeRuns
                ? (designLoader.item ? designLoader.item.implicitHeight : 0) : look.implicitHeight

            OsdLook {
                id: look
                anchors.fill: parent
                active: win.stockRuns
                kind: host.kind
                u: win.us * Shells.osdScale
                shown: feed.flashing
                value: feed.value
                maxValue: feed.maxValue
                muted: feed.muted
                iconName: feed.iconName
                label: feed.label
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
