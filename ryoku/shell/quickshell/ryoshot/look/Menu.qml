import QtQuick
import Lithic

// A menu or popover that opens from its button and flips to stay inside bounds:
// a cut plane in Lithic, an M3 menu (radius 16 here, items full bleed), Adwaita's
// popover (radius 12), Fluent's MenuFlyout (radius 8, acrylic). Put MenuItems
// inside; a triggered row closes it. Set open; place it with anchor and bounds,
// both items in the same window. preferAbove opens it over the button. A
// triggered row asks to close it through dismissed(); the owner clears open.
Item {
    id: root

    readonly property bool lookMenu: true
    property real u: 1
    property bool open: false
    property Item anchor: null
    property Item bounds: root.parent
    property bool preferAbove: true
    property real minWidth: 168
    property real margin: 8

    default property alias items: column.data

    signal dismissed()
    function dismiss() {
        root.dismissed();
    }

    readonly property real pad: (Look.material ? 0 : Look.lithic ? 4 : 6) * root.u
    readonly property real padY: (Look.material ? 8 : Look.lithic ? 4 : 6) * root.u
    readonly property real gap: (Look.lithic ? 6 : 8) * root.u

    implicitWidth: Math.ceil(Math.max(root.minWidth * root.u, root._widest()) + root.pad * 2)
    implicitHeight: Math.ceil(column.implicitHeight + root.padY * 2)
    width: implicitWidth
    height: implicitHeight
    z: 200

    function _widest() {
        let w = 0;
        for (let i = 0; i < column.children.length; i++)
            w = Math.max(w, column.children[i].implicitWidth || 0);
        return w;
    }

    // where it opens, in its parent: centred on the anchor, above or below it,
    // flipped to the other side and slid along when the bounds would cut it
    readonly property var spot: {
        // mapToItem is not a binding source: opening re-reads where the anchor is
        root.open;
        if (!root.anchor || !root.parent)
            return Qt.point(0, 0);
        const a = root.anchor.mapToItem(root.parent, 0, 0, root.anchor.width, root.anchor.height);
        const b = root.bounds ? root.bounds.mapToItem(root.parent, 0, 0, root.bounds.width, root.bounds.height)
            : Qt.rect(0, 0, root.parent.width, root.parent.height);
        const m = root.margin * root.u;
        const aboveY = a.y - root.gap - root.height;
        const belowY = a.y + a.height + root.gap;
        const fitsAbove = aboveY >= b.y + m;
        const fitsBelow = belowY + root.height <= b.y + b.height - m;
        let y = root.preferAbove ? (fitsAbove || !fitsBelow ? aboveY : belowY) : (fitsBelow || !fitsAbove ? belowY : aboveY);
        y = Math.max(b.y + m, Math.min(y, b.y + b.height - m - root.height));
        let x = a.x + a.width / 2 - root.width / 2;
        x = Math.max(b.x + m, Math.min(x, b.x + b.width - m - root.width));
        return Qt.point(Math.round(x), Math.round(y));
    }
    readonly property bool opensUp: root.anchor && root.parent
        ? root.spot.y + root.height / 2 < root.anchor.mapToItem(root.parent, 0, 0).y : root.preferAbove
    x: root.spot.x
    y: root.spot.y

    property real reveal: root.open ? 1 : 0
    Behavior on reveal {
        NumberAnimation {
            duration: Look.dur(root.open ? (Look.fluent ? 250 : Look.material ? 220 : 140) : 100)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: root.open ? (Look.fluent ? Look.fluentIn : Look.decelerate) : Look.accelerate
        }
    }
    visible: root.reveal > 0
    enabled: root.open
    opacity: Look.material ? Math.min(1, root.reveal * 2) : root.reveal

    // Material grows from the button's edge, Fluent slides in from it, Lithic
    // and Adwaita drop a few px
    readonly property real slide: (Look.fluent ? 12 : Look.material ? 0 : 6) * root.u * (1 - root.reveal)
    transform: [
        Translate { y: root.opensUp ? root.slide : -root.slide },
        Scale {
            origin.x: root.width / 2
            origin.y: root.opensUp ? root.height : 0
            yScale: Look.material ? 0.6 + 0.4 * root.reveal : Look.adwaita ? 0.96 + 0.04 * root.reveal : 1
            xScale: Look.adwaita ? 0.96 + 0.04 * root.reveal : 1
        }
    ]

    // clicks inside never reach what lies under it
    MouseArea {
        anchors.fill: parent
        hoverEnabled: true
        acceptedButtons: Qt.AllButtons
        onWheel: wheel => wheel.accepted = true
    }

    Surface {
        anchors.fill: parent
        u: root.u
        cut: Look.lithic ? Look.cutMd : Look.cutLg
        radius: (Look.material ? 16 : Look.adwaita ? 12 : 8) * root.u
        elevation: 2
    }

    Column {
        id: column
        x: root.pad
        y: root.padY
        width: root.width - root.pad * 2
    }
}
