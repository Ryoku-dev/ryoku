import QtQuick
import "../look"

// Adwaita's capture chrome, after GNOME Shell's screenshot UI: a light frame
// with four round handles, the size on an osd pill inside the selection's foot
// while it is drawn, an accent outline with the app's name on a pill around the
// window under the pointer, and a round loupe. No springs: fades and short
// eases. A drag that will record turns the frame red.
Item {
    id: look

    required property var chrome

    readonly property real u: look.chrome.u
    property color frameInk: look.chrome.record ? Look.alert : look.chrome.lightInk
    Behavior on frameInk { ColorAnimation { duration: Look.dur(200) } }
    readonly property color accent: look.chrome.record ? Look.alert : Look.sel

    RoundOutline {
        id: hover
        anchors.fill: parent
        chrome: look.chrome
        radius: 12
        strokeWidth: 2
        stroke: look.accent
        fillAlpha: 0.15

        ValueChip {
            visible: look.chrome.hoverApp !== ""
            readonly property real visTop: Math.max(hover.ry, 0)
            readonly property real visBottom: Math.min(hover.ry + hover.rh, look.chrome.height)
            x: look.chrome.snap(Math.max(hover.rx, 0) + (Math.min(hover.rx + hover.rw, look.chrome.width) - Math.max(hover.rx, 0)) / 2 - width / 2)
            y: look.chrome.snap(Math.max(visTop + 12 * look.u, visBottom - 20 * look.u - height))
            u: look.u
            text: look.chrome.hoverApp + "  \u00b7  " + look.chrome.hoverSizeText
            fill: Look.plane
            ink: Look.fg
            chipHeight: 34
            padX: 16
            fontSize: 13
            fontWeight: Font.Bold
            elevation: 2
        }
    }

    Item {
        id: region
        anchors.fill: parent
        visible: look.chrome.hasArea
        readonly property var s: look.chrome.sel
        readonly property real sw: look.chrome.px2(1.5)

        Rectangle {
            x: region.s ? look.chrome.snap(region.s.x) - region.sw : 0
            y: region.s ? look.chrome.snap(region.s.y) - region.sw : 0
            width: region.s ? look.chrome.snap(region.s.w) + region.sw * 2 : 0
            height: region.s ? look.chrome.snap(region.s.h) + region.sw * 2 : 0
            color: "transparent"
            border.width: region.sw
            border.color: look.frameInk
            antialiasing: false
        }

        RoundHandles {
            anchors.fill: parent
            chrome: look.chrome
            rect: region.s
            edges: false
            cornerSize: 16
            fill: look.frameInk
            ring: Qt.rgba(0, 0, 0, 0)
            ringWidth: 0
            shadow: true
            hotRole: look.chrome.hotGrip
            downRole: look.chrome.gripRole
        }

        // the size at the selection's foot, inside it; below the frame when the
        // selection is too short to hold it
        ValueChip {
            id: size
            readonly property real inset: 16 * look.u
            readonly property bool fitsInside: region.s !== null && region.s.h >= height + inset * 2 && region.s.w >= width + inset
            shown: look.chrome.live && look.chrome.corner !== null
            x: region.s ? look.chrome.snap(Math.max(8 * look.u, Math.min(region.s.x + region.s.w / 2 - width / 2,
                                                                     look.chrome.width - width - 8 * look.u))) : 0
            y: !region.s ? 0
                : look.chrome.snap(size.fitsInside ? region.s.y + region.s.h - size.inset - height
                    : Math.min(region.s.y + region.s.h + 12 * look.u, look.chrome.height - height - 8 * look.u))
            u: look.u
            lead: look.chrome.record ? "\u25cf" : ""
            leadInk: Look.alert
            text: look.chrome.selSizeText
            fill: Look.plane
            ink: Look.fg
            chipHeight: 34
            padX: 16
            fontSize: 13
            fontWeight: Font.Bold
            elevation: 2
        }
    }

    Item {
        readonly property var b: look.chrome.annBox
        anchors.fill: parent
        visible: b !== null
        Rectangle {
            x: parent.b ? look.chrome.snap(parent.b.x) : 0
            y: parent.b ? look.chrome.snap(parent.b.y) : 0
            width: parent.b ? look.chrome.snap(parent.b.w) : 0
            height: parent.b ? look.chrome.snap(parent.b.h) : 0
            radius: 6 * look.u
            color: "transparent"
            border.width: look.chrome.px2(1.5)
            border.color: Look.sel
            antialiasing: true
        }
        RoundHandles {
            anchors.fill: parent
            chrome: look.chrome
            rect: parent.b
            edges: false
            cornerSize: 10
            fill: look.chrome.lightInk
            ring: Qt.rgba(0, 0, 0, 0)
            ringWidth: 0
            shadow: true
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
            diameter: 100
            ringWidth: 2
            ring: look.chrome.lightInk
            mark: look.accent
            chipFill: Look.plane
            chipInk: Look.fg
            chipHeight: 30
            value: look.chrome.picking ? (look.chrome.pickHex || "#\u2026") : look.chrome.pointText
        }
    }
}
