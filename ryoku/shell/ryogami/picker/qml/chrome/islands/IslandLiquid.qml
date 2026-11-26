import QtQuick
import Lithic
import Ryoku.Blobs

// The picker's chrome as one liquid body
Item {
    id: liquid

    required property PickerState state
    property rect leftRect
    property rect midRect
    property rect rightRect
    property real reveal: 0
    property real weld: 0
    property rect sheetRect
    property real sheetAnchorX: 0
    property real sheetProgress: 0

    readonly property real s: Theme.scale
    readonly property real smoothing: Theme.smoothing
    readonly property real lift: (1 - liquid.reveal) * -(liquid.leftRect.height + 40 * s)
    readonly property real bridgeH: 20 * s
    readonly property real neckW: 64 * s

    function corner(cut, w, h) { return Theme.corner(cut * liquid.s, w, h) }

    readonly property BlobGroup body: blobs
    BlobGroup {
        id: blobs
        color: Lithic.stone2
        smoothing: liquid.smoothing
        cutCorners: Theme.cut
        borderWidth: Theme.lithic || Theme.fluent ? 1 : 0
        borderColor: Lithic.vein
        shadowStrength: 0
        shadowSize: 0
    }

    component Island: BlobRect {
        property rect r
        group: liquid.body
        x: r.x
        y: r.y + liquid.lift
        width: liquid.reveal > 0.001 ? r.width : 0
        height: liquid.reveal > 0.001 ? r.height : 0
        radius: liquid.corner(9, r.width, r.height)
        sinks: false
    }

    Island { id: leftShape; r: liquid.leftRect }
    Island { id: midShape; r: liquid.midRect }
    Island { id: rightShape; r: liquid.rightRect }

    component Bridge: BlobRect {
        property rect a
        property rect b
        readonly property bool live: liquid.weld > 0.01 && liquid.reveal > 0.001 && b.x > a.x
        group: liquid.body
        x: a.x + a.width - liquid.smoothing
        y: a.y + a.height / 2 - height / 2 + liquid.lift
        width: live ? Math.max(0, b.x - (a.x + a.width) + liquid.smoothing * 2) : 0
        height: live ? liquid.bridgeH * liquid.weld : 0
        radius: 0
        sinks: false
        deformScale: 0
    }
    Bridge { a: liquid.leftRect; b: liquid.midRect }
    Bridge { a: liquid.midRect; b: liquid.rightRect }

    component Neck: BlobRect {
        property real anchorX: 0
        // the neck stays clear of both shapes' corners, so it never bulges past an edge
        property real minX: -1e9
        property real maxX: 1e9
        property real fromY: 0
        property real toY: 0
        property real progress: 0
        readonly property bool live: progress > 0.001 && liquid.reveal > 0.001
        group: liquid.body
        width: live ? liquid.neckW * Math.min(1, progress * 2.5) : 0
        x: Math.max(minX, Math.min(maxX - width, anchorX - width / 2))
        y: fromY - liquid.smoothing
        height: live ? Math.max(0, toY - fromY) + liquid.smoothing * 2 : 0
        radius: 0
        sinks: false
        deformScale: 0
    }

    Neck {
        anchorX: liquid.sheetAnchorX
        minX: Math.max(liquid.midRect.x, liquid.sheetRect.x) + liquid.smoothing * 2
        maxX: Math.min(liquid.midRect.x + liquid.midRect.width, liquid.sheetRect.x + liquid.sheetRect.width) - liquid.smoothing * 2
        fromY: liquid.midRect.y + liquid.midRect.height
        toY: liquid.sheetRect.y
        progress: liquid.sheetProgress
    }
    BlobRect {
        group: liquid.body
        readonly property bool live: liquid.sheetProgress > 0.001
        x: liquid.sheetRect.x
        y: liquid.sheetRect.y
        width: live ? liquid.sheetRect.width : 0
        height: live ? liquid.sheetRect.height : 0
        radius: liquid.corner(16, width, height)
        sinks: false
        deformScale: 0
    }

    Neck {
        anchorX: liquid.state.drawerAnchorX
        minX: Math.max(liquid.rightRect.x, liquid.state.drawerNow.x) + liquid.smoothing * 2
        maxX: Math.min(liquid.rightRect.x + liquid.rightRect.width, liquid.state.drawerNow.x + liquid.state.drawerNow.width) - liquid.smoothing * 2
        fromY: liquid.rightRect.y + liquid.rightRect.height
        toY: liquid.state.drawerNow.y
        progress: liquid.state.drawerKey !== "" ? liquid.state.drawerProgress : 0
    }
    BlobRect {
        group: liquid.body
        readonly property rect r: liquid.state.drawerNow
        readonly property bool live: liquid.state.drawerKey !== "" && liquid.state.drawerProgress > 0.001
        x: r.x
        y: r.y
        width: live ? r.width : 0
        height: live ? r.height : 0
        radius: liquid.corner(16, r.width, r.height)
        sinks: false
        deformScale: 0
    }
}
