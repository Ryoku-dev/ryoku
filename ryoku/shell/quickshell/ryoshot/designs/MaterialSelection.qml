import QtQuick
import "../look"

// Material's capture chrome, after Android's and ChromeOS's screen capture: no
// guides, a 2 dp primary frame, round corner handles and pill edge handles
// while editing, the size on a tonal chip that springs in under the frame, a
// primary outline around the window under the pointer and a round loupe. A drag
// that will record wears the error colour.
Item {
    id: look

    required property var chrome

    readonly property real u: look.chrome.u
    property color accent: look.chrome.record ? Look.alert : Look.sel
    Behavior on accent { ColorAnimation { duration: Look.dur(200) } }
    readonly property color chipFill: look.chrome.record ? Look.alert : Look.container
    readonly property color chipInk: look.chrome.record ? Look.onAlert : Look.containerInk

    RoundOutline {
        id: hover
        anchors.fill: parent
        chrome: look.chrome
        radius: 16
        strokeWidth: 3
        stroke: look.accent
        fillAlpha: 0.1

        ValueChip {
            readonly property real inset: 12 * look.u
            visible: look.chrome.hoverApp !== ""
            x: look.chrome.snap(Math.max(hover.rx, 0) + inset)
            y: look.chrome.snap(Math.max(hover.ry, 0) + inset)
            u: look.u
            text: look.chrome.hoverApp + "  \u00b7  " + look.chrome.hoverSizeText
            fill: look.chipFill
            ink: look.chipInk
            radius: 8
            chipHeight: 32
            padX: 14
            elevation: 1
        }
    }

    Item {
        id: region
        anchors.fill: parent
        visible: look.chrome.hasArea
        readonly property var s: look.chrome.sel
        readonly property real sw: look.chrome.px2(2)

        Rectangle {
            x: region.s ? look.chrome.snap(region.s.x) - region.sw : 0
            y: region.s ? look.chrome.snap(region.s.y) - region.sw : 0
            width: region.s ? look.chrome.snap(region.s.w) + region.sw * 2 : 0
            height: region.s ? look.chrome.snap(region.s.h) + region.sw * 2 : 0
            color: "transparent"
            border.width: region.sw
            border.color: look.accent
            antialiasing: false
        }

        RoundHandles {
            anchors.fill: parent
            visible: look.chrome.editing
            chrome: look.chrome
            rect: region.s
            cornerSize: 12
            edgeLong: 24
            edgeShort: 6
            fill: look.accent
            ring: Look.ground
            ringWidth: 2
            hotRole: look.chrome.hotGrip
            downRole: look.chrome.gripRole
        }

        // the size, centred under the frame; above it, or inside its foot, when the
        // edge or the annotation dock leaves no room below
        ValueChip {
            id: size
            readonly property real gap: 12 * look.u
            readonly property real below: region.s ? region.s.y + region.s.h + gap : 0
            readonly property real above: region.s ? region.s.y - gap - height : 0
            readonly property bool roomBelow: !look.chrome.editing && size.below + height <= look.chrome.height - 8 * look.u
            readonly property bool roomAbove: size.above >= 8 * look.u
            shown: look.chrome.live && look.chrome.corner !== null
            x: region.s ? look.chrome.snap(Math.max(8 * look.u, Math.min(region.s.x + region.s.w / 2 - width / 2,
                                                                     look.chrome.width - width - 8 * look.u))) : 0
            y: !region.s ? 0
                : look.chrome.snap(size.roomBelow ? size.below
                    : size.roomAbove ? size.above
                    : region.s.y + region.s.h - gap - height)
            u: look.u
            lead: look.chrome.record ? "\u25cf" : ""
            leadInk: look.chipInk
            text: look.chrome.selSizeText
            fill: look.chipFill
            ink: look.chipInk
            chipHeight: 28
            radius: 8
            padX: 12
        }
    }

    // the annotation picked by the select tool
    Item {
        readonly property var b: look.chrome.annBox
        anchors.fill: parent
        visible: b !== null
        Rectangle {
            x: parent.b ? look.chrome.snap(parent.b.x) : 0
            y: parent.b ? look.chrome.snap(parent.b.y) : 0
            width: parent.b ? look.chrome.snap(parent.b.w) : 0
            height: parent.b ? look.chrome.snap(parent.b.h) : 0
            radius: 8 * look.u
            color: "transparent"
            border.width: look.chrome.px2(2)
            border.color: Look.sel
            antialiasing: true
        }
        RoundHandles {
            anchors.fill: parent
            chrome: look.chrome
            rect: parent.b
            edges: false
            cornerSize: 8
            fill: Look.sel
            ring: Look.ground
            ringWidth: 1.5
        }
    }

    Item {
        id: corner
        anchors.fill: parent
        readonly property var p: look.chrome.loupePoint
        readonly property var place: corner.p
            ? look.chrome.placeAround(corner.p, 0, 0, lens.implicitWidth, lens.implicitHeight)
            : null

        RoundLoupe {
            id: lens
            chrome: look.chrome
            shown: corner.p !== null
            x: corner.place ? look.chrome.snap(corner.place.loupe.x) : 0
            y: corner.place ? look.chrome.snap(corner.place.loupe.y) : 0
            px: corner.p ? corner.p.x : 0
            py: corner.p ? corner.p.y : 0
            diameter: 104
            ringWidth: 3
            ring: look.accent
            mark: look.accent
            chipFill: look.chipFill
            chipInk: look.chipInk
            chipHeight: 28
            chipRadius: 8
            value: look.chrome.picking ? (look.chrome.pickHex || "#\u2026") : look.chrome.pointText
        }
    }
}
