import QtQuick
import Ryoku.Ui.Singletons
import "../look"

// Lithic's drafting table: hairline guides with rulers on the output's top and
// left edges, a hairline frame with print crop marks, dimension lines once the
// region is set, a readout plaque at the corner being placed, and a cut loupe.
// A drag that will record draws all of it in jasper.
Item {
    id: look

    required property var chrome

    readonly property real u: look.chrome.u
    property color accent: look.chrome.record ? Look.alert : Look.sel
    readonly property color scrimInk: look.chrome.lightInk
    Behavior on accent { ColorAnimation { duration: Look.dur(160) } }

    // guides through the pointer, held 12 off it
    Item {
        id: guides
        anchors.fill: parent
        visible: look.chrome.crosshair
        readonly property real gx: look.chrome.snap(look.chrome.px)
        readonly property real gy: look.chrome.snap(look.chrome.py)
        readonly property real gap: 12 * look.u
        // while a region is drawn the frame's own edges are the guides
        readonly property color ink: Qt.rgba(look.scrimInk.r, look.scrimInk.g, look.scrimInk.b,
                                             look.chrome.hasArea ? 0 : 0.25)

        Rectangle {
            x: guides.gx
            width: look.chrome.hair
            height: Math.max(0, guides.gy - guides.gap)
            color: guides.ink
            antialiasing: false
        }
        Rectangle {
            x: guides.gx
            y: guides.gy + guides.gap
            width: look.chrome.hair
            height: Math.max(0, parent.height - y)
            color: guides.ink
            antialiasing: false
        }
        Rectangle {
            y: guides.gy
            width: Math.max(0, guides.gx - guides.gap)
            height: look.chrome.hair
            color: guides.ink
            antialiasing: false
        }
        Rectangle {
            x: guides.gx + guides.gap
            y: guides.gy
            width: Math.max(0, parent.width - x)
            height: look.chrome.hair
            color: guides.ink
            antialiasing: false
        }

        LithicRuler {
            anchors.fill: parent
            chrome: look.chrome
        }
        LithicRuler {
            anchors.fill: parent
            chrome: look.chrome
            vertical: true
        }
    }

    // the window or output a click would take; it glides from one to the next
    Item {
        id: hover
        readonly property var r: look.chrome.hover
        property real rx: 0
        property real ry: 0
        property real rw: 0
        property real rh: 0
        property bool gliding: false
        anchors.fill: parent
        opacity: hover.r ? 1 : 0
        visible: opacity > 0
        Behavior on opacity { NumberAnimation { duration: Look.dur(hover.r ? 120 : 90) } }
        Behavior on rx { enabled: hover.gliding; NumberAnimation { duration: Look.dur(140); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.decelerate } }
        Behavior on ry { enabled: hover.gliding; NumberAnimation { duration: Look.dur(140); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.decelerate } }
        Behavior on rw { enabled: hover.gliding; NumberAnimation { duration: Look.dur(140); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.decelerate } }
        Behavior on rh { enabled: hover.gliding; NumberAnimation { duration: Look.dur(140); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.decelerate } }
        onRChanged: {
            if (!hover.r) return;
            hover.gliding = hover.opacity > 0.5;
            hover.rx = hover.r.x;
            hover.ry = hover.r.y;
            hover.rw = hover.r.w;
            hover.rh = hover.r.h;
        }

        Rectangle {
            x: look.chrome.snap(hover.rx) - look.chrome.hair
            y: look.chrome.snap(hover.ry) - look.chrome.hair
            width: look.chrome.snap(hover.rw) + look.chrome.hair * 2
            height: look.chrome.snap(hover.rh) + look.chrome.hair * 2
            color: Qt.rgba(look.accent.r, look.accent.g, look.accent.b, 0.06)
            border.width: look.chrome.hair
            border.color: look.accent
            antialiasing: false
        }
        LithicMarks {
            anchors.fill: parent
            chrome: look.chrome
            rx: hover.rx
            ry: hover.ry
            rw: hover.rw
            rh: hover.rh
            ink: look.accent
        }
        Item {
            id: tag
            readonly property real h: look.chrome.snap(18 * look.u)
            readonly property real outside: look.chrome.snap(hover.ry) - look.chrome.snap(10 * look.u) - tag.h
            readonly property bool inside: tag.outside < look.chrome.snap(4 * look.u)
            x: look.chrome.snap(Math.max(4 * look.u, Math.min(hover.rx + (tag.inside ? 10 * look.u : -4 * look.u),
                                                              look.chrome.width - width - 4 * look.u)))
            y: tag.inside ? look.chrome.snap(Math.max(0, hover.ry) + 10 * look.u) : tag.outside
            width: Math.ceil(tagRow.implicitWidth + 14 * look.u)
            height: tag.h
            visible: look.chrome.hoverApp !== ""

            Surface {
                anchors.fill: parent
                u: look.u
                cut: Look.cutSm
                cutTopRight: 0
                cutBottomLeft: 0
                elevation: 1
                fill: Look.planes.pane
                stroke: Look.vein
            }
            Row {
                id: tagRow
                anchors.centerIn: parent
                spacing: look.chrome.snap(6 * look.u)
                Text {
                    text: look.chrome.hoverApp.toUpperCase()
                    color: Look.fg
                    font.family: Look.mono
                    font.pixelSize: Math.round(9.5 * look.u)
                    font.letterSpacing: 1 * look.u
                    renderType: Text.NativeRendering
                }
                Text {
                    text: "\u00b7"
                    color: Look.fg3
                    font.family: Look.mono
                    font.pixelSize: Math.round(9.5 * look.u)
                    renderType: Text.NativeRendering
                }
                Text {
                    text: look.chrome.hoverSizeText
                    color: Look.fg2
                    font.family: Look.mono
                    font.pixelSize: Math.round(9.5 * look.u)
                    font.letterSpacing: 0.4 * look.u
                    font.features: ({ "tnum": 1 })
                    renderType: Text.NativeRendering
                }
            }
        }
    }

    // the region: frame, crop marks (the grips while editing) and dimensions
    Item {
        id: region
        anchors.fill: parent
        visible: look.chrome.hasArea
        readonly property var s: look.chrome.sel

        Rectangle {
            x: region.s ? look.chrome.snap(region.s.x) - look.chrome.hair : 0
            y: region.s ? look.chrome.snap(region.s.y) - look.chrome.hair : 0
            width: region.s ? look.chrome.snap(region.s.x + region.s.w) - look.chrome.snap(region.s.x) + look.chrome.hair * 2 : 0
            height: region.s ? look.chrome.snap(region.s.y + region.s.h) - look.chrome.snap(region.s.y) + look.chrome.hair * 2 : 0
            color: "transparent"
            border.width: look.chrome.hair
            border.color: look.accent
            antialiasing: false
        }
        LithicMarks {
            anchors.fill: parent
            chrome: look.chrome
            rx: region.s ? region.s.x : 0
            ry: region.s ? region.s.y : 0
            rw: region.s ? region.s.w : 0
            rh: region.s ? region.s.h : 0
            ink: look.accent
            hotInk: look.scrimInk
            grips: look.chrome.editing
            hotRole: look.chrome.hotGrip
            downRole: look.chrome.gripRole
        }

        Item {
            id: dims
            anchors.fill: parent
            opacity: look.chrome.editing && !look.chrome.live ? 1 : 0
            visible: opacity > 0
            Behavior on opacity { NumberAnimation { duration: Look.dur(look.chrome.live ? 60 : 160) } }
            // clear of the edge-tick grips, which reach 10 out from the frame
            readonly property real lead: 20 * look.u
            readonly property real room: 32 * look.u

            LithicDimension {
                anchors.fill: parent
                chrome: look.chrome
                visible: region.s !== null && region.s.w >= 72 * look.u
                from: region.s ? region.s.x : 0
                to: region.s ? region.s.x + region.s.w : 0
                at: !region.s ? 0 : region.s.y >= dims.room ? region.s.y - dims.lead : region.s.y + dims.lead
                ink: look.accent
            }
            LithicDimension {
                anchors.fill: parent
                chrome: look.chrome
                vertical: true
                visible: region.s !== null && region.s.h >= 72 * look.u
                from: region.s ? region.s.y : 0
                to: region.s ? region.s.y + region.s.h : 0
                at: !region.s ? 0 : region.s.x >= dims.room ? region.s.x - dims.lead : region.s.x + dims.lead
                ink: look.accent
            }
        }
    }

    // the annotation picked by the select tool
    Item {
        readonly property var b: look.chrome.annBox
        visible: b !== null
        anchors.fill: parent
        Rectangle {
            x: parent.b ? look.chrome.snap(parent.b.x) : 0
            y: parent.b ? look.chrome.snap(parent.b.y) : 0
            width: parent.b ? look.chrome.snap(parent.b.w) : 0
            height: parent.b ? look.chrome.snap(parent.b.h) : 0
            color: "transparent"
            border.width: look.chrome.hair
            border.color: Qt.rgba(Look.sel.r, Look.sel.g, Look.sel.b, 0.7)
            antialiasing: false
        }
        LithicMarks {
            anchors.fill: parent
            chrome: look.chrome
            rx: parent.b ? parent.b.x : 0
            ry: parent.b ? parent.b.y : 0
            rw: parent.b ? parent.b.w : 0
            rh: parent.b ? parent.b.h : 0
            length: 7
            thickness: 1.5
            offset: 2
            ink: Look.sel
        }
    }

    // the readout and the loupe beside the corner being placed
    Item {
        id: corner
        anchors.fill: parent
        readonly property var p: look.chrome.loupePoint
        readonly property bool showReadout: look.chrome.corner !== null && look.chrome.hasArea
        readonly property var place: corner.p
            ? look.chrome.placeAround(corner.p,
                corner.showReadout ? readout.width : 0, corner.showReadout ? readout.height : 0,
                lens.implicitWidth, lens.implicitHeight)
            : null

        Item {
            id: readout
            readonly property bool shown: corner.showReadout && corner.place !== null
            visible: opacity > 0
            opacity: readout.shown ? 1 : 0
            Behavior on opacity { enabled: readout.shown; NumberAnimation { duration: Look.dur(120); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.decelerate } }
            x: corner.place && corner.place.readout ? look.chrome.snap(corner.place.readout.x) : 0
            y: corner.place && corner.place.readout ? look.chrome.snap(corner.place.readout.y) : 0
            width: Math.ceil(Math.max(sizeRow.implicitWidth, origin.implicitWidth) + 16 * look.u)
            height: look.chrome.snap(36 * look.u)

            Surface {
                anchors.fill: parent
                u: look.u
                cut: Look.cutSm
                cutTopRight: 0
                cutBottomLeft: 0
                elevation: 2
                fill: Look.planes.pane
                stroke: look.chrome.record ? Look.alert : Look.vein
            }
            Row {
                id: sizeRow
                x: look.chrome.snap(8 * look.u)
                y: look.chrome.snap(5 * look.u)
                spacing: look.chrome.snap(6 * look.u)
                Text {
                    visible: look.chrome.record
                    text: "\u25cf " + I18n.tr("REC")
                    color: Look.alert
                    font.family: Look.mono
                    font.pixelSize: Math.round(11 * look.u)
                    font.letterSpacing: 0.8 * look.u
                    renderType: Text.NativeRendering
                }
                Text {
                    text: look.chrome.selSizeText
                    color: Look.fg
                    font.family: Look.mono
                    font.pixelSize: Math.round(11 * look.u)
                    font.letterSpacing: 0.4 * look.u
                    font.features: ({ "tnum": 1 })
                    renderType: Text.NativeRendering
                }
            }
            Text {
                id: origin
                x: look.chrome.snap(8 * look.u)
                y: look.chrome.snap(20 * look.u)
                text: look.chrome.originText
                color: Look.fg3
                font.family: Look.mono
                font.pixelSize: Math.round(9 * look.u)
                font.letterSpacing: 0.4 * look.u
                font.features: ({ "tnum": 1 })
                renderType: Text.NativeRendering
            }
        }

        LithicLoupe {
            id: lens
            shown: corner.p !== null
            chrome: look.chrome
            x: corner.place ? look.chrome.snap(corner.place.loupe.x) : 0
            y: corner.place ? look.chrome.snap(corner.place.loupe.y) : 0
            px: corner.p ? corner.p.x : 0
            py: corner.p ? corner.p.y : 0
            mark: look.accent
            showSwatch: look.chrome.picking && look.chrome.pickHex !== ""
            swatch: look.chrome.pickHex !== "" ? look.chrome.pickHex : "transparent"
            value: look.chrome.picking ? (look.chrome.pickHex || "#\u2026") : look.chrome.pointText
        }
    }
}
