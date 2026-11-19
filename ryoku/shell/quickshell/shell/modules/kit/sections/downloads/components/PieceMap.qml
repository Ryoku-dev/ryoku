pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Lithic
import "../../../../ask/look"
import "../../../../ask/lib/downloads.js" as Logic

// The daemon's 64-bucket map as one item: a shader draws every cell, so a row
// costs one quad however many buckets move. Cells sit on whole device pixels
// and ease to a new fill; Lithic and Fluent cut them square, Material rounds
// them into capsules, Adwaita softens the corners.
Item {
    id: root

    property real s: 1
    // what the map belongs to: a new owner shows its map at once, it does not ease from the last one
    property string owner: ""
    // 64 fills of 0..1
    property var pieces: []
    // "" moving | "paused" | "done" | "error" | "idle"
    property string tone: ""
    property int columns: 64
    property int rows: 1
    // the width it may take; it draws the widest whole-pixel grid that fits
    property real available: 0
    // a row's cell height in reference px; 0 makes the cells square
    property real cellHeight: 9
    // a landing bucket fills from its left edge instead of brightening
    property bool sweep: false

    readonly property real dpr: Math.max(1, (QsWindow.window as QsWindow)?.devicePixelRatio ?? 1)
    readonly property int gapPx: Math.max(1, Math.round((root.rows > 1 ? 2.2 : Look.material || Look.adwaita ? 1.5 : 0.75) * root.s * root.dpr))
    readonly property int pitchPx: Math.max(root.gapPx + 1, Math.floor((root.available * root.dpr + root.gapPx) / root.columns))
    readonly property int cellPx: root.pitchPx - root.gapPx
    readonly property int tallPx: root.cellHeight > 0 ? Math.max(2, Math.round(root.cellHeight * root.s * root.dpr)) : root.cellPx
    readonly property real corner: Look.material ? Math.min(root.cellPx, root.tallPx) * (root.rows > 1 ? 0.3 : 0.5)
        : Look.adwaita ? Math.min(Math.min(root.cellPx, root.tallPx) / 2, Math.round((root.rows > 1 ? 4 : 2) * root.s * root.dpr))
        : Look.fluent && root.rows > 1 ? Math.round(2 * root.s * root.dpr) : 0

    readonly property color ink: root.tone === "error" ? Lithic.jasper
        : root.tone === "paused" ? Lithic.alpha(Lithic.amber, Look.dark ? 0.62 : 0.7)
        : root.tone === "done" ? Lithic.alpha(Lithic.jade, 0.7)
        : root.tone === "idle" ? Lithic.fg3 : Lithic.lapis
    readonly property color track: Lithic.alpha(Lithic.fg, Look.dark ? 0.13 : 0.14)

    implicitWidth: (root.columns * root.pitchPx - root.gapPx) / root.dpr
    implicitHeight: (root.rows * (root.tallPx + root.gapPx) - root.gapPx) / root.dpr
    width: implicitWidth
    height: implicitHeight

    property var from: []
    property var to: []
    property real blend: 1
    property string shownOwner: "\u0000"

    function retarget() {
        const next = root.pieces && root.pieces.length === 64 ? root.pieces : Logic.blend([], [], 0);
        if (root.shownOwner !== root.owner || Look.still) {
            ease.stop();
            root.shownOwner = root.owner;
            root.from = next;
            root.to = next;
            root.blend = 1;
            return;
        }
        if (Logic.same(next, root.to))
            return;
        root.from = Logic.blend(root.from, root.to, root.blend);
        root.to = next;
        ease.restart();
    }
    onPiecesChanged: root.retarget()
    onOwnerChanged: root.retarget()
    Component.onCompleted: root.retarget()

    NumberAnimation {
        id: ease
        target: root
        property: "blend"
        from: 0
        to: 1
        duration: Look.dur(260)
        easing.type: Easing.OutCubic
    }

    function block(list, n) {
        const v = [];
        for (var i = 0; i < 16; i++)
            v.push(Number(list[n * 16 + i] || 0));
        return Qt.matrix4x4(v[0], v[1], v[2], v[3], v[4], v[5], v[6], v[7], v[8], v[9], v[10], v[11], v[12], v[13], v[14], v[15]);
    }

    ShaderEffect {
        anchors.fill: parent
        visible: root.available > 0

        readonly property matrix4x4 to0: root.block(root.to, 0)
        readonly property matrix4x4 to1: root.block(root.to, 1)
        readonly property matrix4x4 to2: root.block(root.to, 2)
        readonly property matrix4x4 to3: root.block(root.to, 3)
        readonly property matrix4x4 from0: root.block(root.from, 0)
        readonly property matrix4x4 from1: root.block(root.from, 1)
        readonly property matrix4x4 from2: root.block(root.from, 2)
        readonly property matrix4x4 from3: root.block(root.from, 3)
        readonly property color ink: root.ink
        readonly property color track: root.track
        readonly property vector2d extent: Qt.vector2d(root.width * root.dpr, root.height * root.dpr)
        readonly property vector2d grid: Qt.vector2d(root.columns, root.rows)
        readonly property vector2d cell: Qt.vector2d(root.cellPx, root.tallPx)
        readonly property vector2d gap: Qt.vector2d(root.gapPx, root.gapPx)
        readonly property real blend: root.blend
        readonly property real radius: root.corner
        readonly property real sweep: root.sweep ? 1 : 0

        fragmentShader: Qt.resolvedUrl("piecemap.frag.qsb")
    }
}
