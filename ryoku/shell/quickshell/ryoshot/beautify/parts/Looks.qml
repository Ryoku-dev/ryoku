pragma ComponentBehavior: Bound

import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../look"
import "../.."

// The looks, each drawn on the user's own shot. Lithic, Adwaita and Fluent lay
// them in a three-column grid: Lithic's cut tiles with a mono caption, framed in
// sel with tiny crop marks when chosen; Adwaita's flowbox of radius-8 thumbnails
// with a 2 px accent outline; Fluent's GridView with WinUI's accent border and
// check badge. Material browses them in an M3 multi-browse carousel: one large,
// one medium, one small, the chosen look settling into the large slot.
Item {
    id: lk

    required property var studio
    property real u: 1

    readonly property var looks: lk.studio ? lk.studio.looks : []
    readonly property int count: lk.looks.length
    // the chosen look, or -1 once a value no longer matches any
    readonly property int chosen: {
        if (!lk.studio)
            return -1;
        // lookMatches reads the studio's values, so this follows every edit
        for (let i = 0; i < lk.looks.length; i++) {
            if (lk.studio.lookMatches(lk.looks[i].cfg))
                return i;
        }
        return -1;
    }
    function pick(i) {
        lk.studio.applyLook(lk.looks[i].cfg);
    }

    implicitHeight: Look.material ? carousel.height : grid.height
    height: implicitHeight

    // ---------------------------------------------------------------- grid
    Grid {
        id: grid
        visible: !Look.material
        width: lk.width
        columns: 3
        readonly property real gap: Math.round((Look.lithic ? 8 : Look.adwaita ? 10 : 8) * lk.u)
        columnSpacing: grid.gap
        rowSpacing: Math.round((Look.lithic ? 10 : 8) * lk.u)
        readonly property real tileW: Math.floor((lk.width - 2 * grid.gap) / 3)
        readonly property real tileH: Math.round(grid.tileW * 0.66)

        Repeater {
            model: Look.material ? [] : lk.looks
            delegate: Item {
                id: tile
                required property var modelData
                required property int index
                readonly property bool chosen: lk.chosen === tile.index
                readonly property bool hovered: tileHover.hovered
                width: grid.tileW
                height: grid.tileH + caption.height + Math.round((Look.lithic ? 6 : 6) * lk.u)

                Item {
                    id: frame
                    width: grid.tileW
                    height: grid.tileH
                    scale: tileTap.pressed ? 0.97 : 1
                    Behavior on scale {
                        NumberAnimation { duration: Look.dur(120); easing.type: Easing.OutCubic }
                    }

                    // the chosen tile draws back from its outline in the round designs
                    property real inset: tile.chosen && !Look.lithic ? Math.round(4 * lk.u) : 0
                    Behavior on inset {
                        NumberAnimation { duration: Look.dur(160); easing.type: Easing.OutCubic }
                    }

                    Thumb {
                        anchors.fill: parent
                        anchors.margins: frame.inset
                        studio: lk.studio
                        cfg: tile.modelData.cfg
                        frameW: grid.tileW
                        frameH: grid.tileH
                        cut: Look.lithic
                        corner: (Look.lithic ? 4 : Look.adwaita ? 8 : 4) * lk.u
                    }

                    // hover: Lithic's vein hairline, the others a faint wash
                    CutRect {
                        visible: Look.lithic
                        anchors.fill: parent
                        design: "lithic"
                        cut: 4 * lk.u
                        strokeColor: tile.chosen ? Look.sel : tile.hovered ? Look.fg3 : Look.vein
                        strokeWidth: Math.max(1, Math.round(lk.u))
                    }
                    Rectangle {
                        visible: !Look.lithic
                        anchors.fill: parent
                        radius: (Look.adwaita ? 12 : 6) * lk.u
                        color: "transparent"
                        border.width: Math.max(1, Math.round(2 * lk.u))
                        border.color: tile.chosen ? Look.sel : tile.hovered ? Lithic.alpha(Look.fg, 0.25) : "transparent"
                        antialiasing: true
                        Behavior on border.color {
                            ColorAnimation { duration: Look.dur(Look.stateMs) }
                        }
                    }

                    // Lithic's tiny crop marks just off the chosen tile's corners
                    Repeater {
                        model: Look.lithic && tile.chosen ? [[0, 0, 1, 1], [1, 0, -1, 1], [1, 1, -1, -1], [0, 1, 1, -1]] : []
                        delegate: Item {
                            id: mark
                            required property var modelData
                            readonly property real off: Math.round(3 * lk.u)
                            readonly property real len: Math.round(6 * lk.u)
                            readonly property real t: Math.max(1, Math.round(lk.u))
                            x: mark.modelData[0] * frame.width - mark.modelData[2] * mark.off
                            y: mark.modelData[1] * frame.height - mark.modelData[3] * mark.off
                            Rectangle {
                                x: mark.modelData[2] > 0 ? 0 : -mark.len
                                y: mark.modelData[3] > 0 ? 0 : -mark.t
                                width: mark.len
                                height: mark.t
                                color: Look.sel
                            }
                            Rectangle {
                                x: mark.modelData[2] > 0 ? 0 : -mark.t
                                y: mark.modelData[3] > 0 ? 0 : -mark.len
                                width: mark.t
                                height: mark.len
                                color: Look.sel
                            }
                        }
                    }

                    // WinUI's check badge in the chosen tile's corner
                    Rectangle {
                        visible: Look.fluent
                        anchors.top: parent.top
                        anchors.right: parent.right
                        width: Math.round(18 * lk.u)
                        height: width
                        topRightRadius: 4 * lk.u
                        bottomLeftRadius: 4 * lk.u
                        color: Look.sel
                        opacity: tile.chosen ? 1 : 0
                        Behavior on opacity {
                            NumberAnimation { duration: Look.dur(120) }
                        }
                        Icon {
                            anchors.centerIn: parent
                            name: "check"
                            size: Math.round(12 * lk.u)
                            tint: Look.onSel
                        }
                    }
                }

                Text {
                    id: caption
                    x: Look.lithic ? 0 : Math.round((parent.width - width) / 2)
                    y: frame.height + Math.round(6 * lk.u)
                    width: Look.lithic ? parent.width : implicitWidth
                    text: Look.lithic ? tile.modelData.name.toUpperCase() : tile.modelData.name
                    color: tile.chosen ? (Look.lithic ? Look.sel : Look.fg) : tile.hovered ? Look.fg : Look.fg2
                    font.family: Look.lithic ? Look.mono : Look.sans
                    font.pixelSize: Math.round((Look.lithic ? 9 : 12) * lk.u)
                    font.weight: Look.lithic ? Font.Bold : tile.chosen ? Font.DemiBold : Font.Normal
                    font.letterSpacing: Look.lithic ? Look.tracking * lk.u : 0
                    renderType: Text.CurveRendering
                    Behavior on color {
                        ColorAnimation { duration: Look.dur(Look.stateMs) }
                    }
                }
                // Lithic's index beside the caption: the key that picks it
                Text {
                    visible: Look.lithic
                    anchors.right: parent.right
                    anchors.baseline: caption.baseline
                    text: String(tile.index + 1)
                    color: Look.fg3
                    font.family: Look.mono
                    font.pixelSize: Math.round(8 * lk.u)
                    renderType: Text.CurveRendering
                }

                HoverHandler {
                    id: tileHover
                    cursorShape: Qt.PointingHandCursor
                }
                TapHandler {
                    id: tileTap
                    onTapped: lk.pick(tile.index)
                }
            }
        }
    }

    // ------------------------------------------------------------ carousel
    Item {
        id: carousel
        visible: Look.material
        width: lk.width
        height: Math.round(112 * lk.u)

        readonly property real gap: Math.round(8 * lk.u)
        readonly property real small: Math.round(40 * lk.u)
        readonly property real medium: Math.round(84 * lk.u)
        readonly property real large: Math.max(0, carousel.width - carousel.small - carousel.medium - 2 * carousel.gap)

        // the look in the large slot; browsing moves it without choosing
        property int focusIndex: 0
        function settle() {
            if (lk.chosen >= 0)
                carousel.focusIndex = lk.chosen;
        }
        Connections {
            target: lk
            function onChosenChanged() {
                carousel.settle();
            }
            function onCountChanged() {
                carousel.focusIndex = Math.max(0, Math.min(carousel.focusIndex, lk.count - 1));
                carousel.settle();
            }
        }
        Component.onCompleted: carousel.settle()
        function step(d) {
            carousel.focusIndex = Math.max(0, Math.min(lk.count - 1, carousel.focusIndex + d));
        }

        // the slot an item takes for the large one at f: L M S, and at the end
        // S L M then S M L, so three items always fill the width
        function slot(i, f) {
            const n = lk.count;
            const start = Math.max(0, Math.min(f, n - 3));
            const order = f <= n - 3 ? ["L", "M", "S"] : f === n - 2 ? ["S", "L", "M"] : ["S", "M", "L"];
            const k = i - start;
            if (k < 0)
                return "before";
            if (k > 2)
                return "after";
            return order[k];
        }
        function widthOf(s) {
            return s === "L" ? carousel.large : s === "M" ? carousel.medium : s === "S" ? carousel.small : 0;
        }

        Repeater {
            id: items
            model: Look.material ? lk.looks : []
            delegate: Item {
                id: item
                required property var modelData
                required property int index
                readonly property string slot: carousel.slot(item.index, carousel.focusIndex)
                readonly property bool chosen: lk.chosen === item.index
                // the left edge of a slot: everything before it, plus the gaps
                readonly property real restX: {
                    const n = lk.count;
                    if (item.slot === "after")
                        return carousel.width;
                    let x = 0;
                    for (let i = 0; i < item.index; i++) {
                        const w = carousel.widthOf(carousel.slot(i, carousel.focusIndex));
                        if (w > 0)
                            x += w + carousel.gap;
                    }
                    return x;
                }
                property real w: carousel.widthOf(item.slot)
                x: item.restX
                width: item.w
                height: carousel.height
                visible: item.width > 0.5
                Behavior on w {
                    NumberAnimation { duration: Look.dur(420); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.springDefault }
                }
                Behavior on x {
                    NumberAnimation { duration: Look.dur(420); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.springDefault }
                }

                Thumb {
                    anchors.fill: parent
                    studio: lk.studio
                    cfg: item.modelData.cfg
                    frameW: carousel.large
                    frameH: carousel.height
                    corner: 20 * lk.u
                }
                // a state layer under the pointer
                Rectangle {
                    anchors.fill: parent
                    radius: 20 * lk.u
                    color: carouselTap.pressed ? Lithic.alpha(Look.fg, 0.12) : carouselHover.hovered ? Lithic.alpha(Look.fg, 0.08) : "transparent"
                }
                // the name on the large item, over a soft protection
                Rectangle {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.bottom: parent.bottom
                    height: Math.round(56 * lk.u)
                    bottomLeftRadius: 20 * lk.u
                    bottomRightRadius: 20 * lk.u
                    opacity: Math.max(0, Math.min(1, (item.width - carousel.medium) / (carousel.large - carousel.medium)))
                    gradient: Gradient {
                        GradientStop { position: 0; color: "transparent" }
                        GradientStop { position: 1; color: Qt.rgba(0, 0, 0, 0.6) }
                    }
                    Text {
                        anchors.left: parent.left
                        anchors.leftMargin: Math.round(16 * lk.u)
                        anchors.bottom: parent.bottom
                        anchors.bottomMargin: Math.round(12 * lk.u)
                        text: item.modelData.name
                        color: "white"
                        font.family: Look.sans
                        font.pixelSize: Math.round(14 * lk.u)
                        font.weight: Font.Medium
                        renderType: Text.CurveRendering
                    }
                }
                // the chosen look's badge
                Rectangle {
                    anchors.top: parent.top
                    anchors.right: parent.right
                    anchors.margins: Math.round(10 * lk.u)
                    width: Math.round(24 * lk.u)
                    height: width
                    radius: width / 2
                    color: Look.sel
                    scale: item.chosen && item.width > carousel.small * 1.5 ? 1 : 0
                    Behavior on scale {
                        NumberAnimation { duration: Look.dur(300); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.springFast }
                    }
                    Icon {
                        anchors.centerIn: parent
                        name: "check"
                        size: Math.round(16 * lk.u)
                        tint: Look.onSel
                    }
                }

                HoverHandler {
                    id: carouselHover
                    cursorShape: Qt.PointingHandCursor
                }
                TapHandler {
                    id: carouselTap
                    onTapped: {
                        carousel.focusIndex = item.index;
                        lk.pick(item.index);
                    }
                }
            }
        }

        // browse with a sideways scroll or drag; a vertical wheel still scrolls the panel
        WheelHandler {
            orientation: Qt.Horizontal
            property real acc: 0
            onWheel: (e) => {
                acc += e.angleDelta.x;
                if (Math.abs(acc) >= 120) {
                    carousel.step(acc > 0 ? -1 : 1);
                    acc = 0;
                }
            }
        }
        DragHandler {
            target: null
            xAxis.enabled: true
            yAxis.enabled: false
            onActiveChanged: {
                if (!active && Math.abs(translation.x) > 24 * lk.u)
                    carousel.step(translation.x < 0 ? 1 : -1);
            }
        }
    }
}
