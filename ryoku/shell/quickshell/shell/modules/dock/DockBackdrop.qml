pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import shell.services

// The rail and ledger forms draw ONE plate behind the whole app run instead of a
// plate per icon; every other form leaves this empty. Rail is a frosted sheet,
// square on the screen-edge side and rounded on the inner one, ruled by a single
// hairline along its inner edge. Ledger is a bordered sheet at the house radius;
// its per-cell dividers, index and active-cell inversion are drawn by the item.
Item {
    id: backdrop

    required property var band

    readonly property string style: backdrop.band.style
    readonly property bool horizontal: backdrop.band.horizontal
    readonly property string edge: backdrop.band.edge
    readonly property bool ledger: backdrop.style === "ledger"
    readonly property bool capsule: backdrop.style === "capsule"
    readonly property real r: backdrop.capsule ? backdrop.band.baseSize / 2 : backdrop.band.radius
    visible: backdrop.style === "rail" || backdrop.ledger || backdrop.capsule

    // Span the app run along the band, baseSize across; sits at the run origin.
    width: backdrop.horizontal ? backdrop.band.runSpan : backdrop.band.baseSize
    height: backdrop.horizontal ? backdrop.band.baseSize : backdrop.band.runSpan
    Behavior on width { enabled: backdrop.band.animate; NumberAnimation { duration: Motion.rowReveal; easing.type: Motion.rowRevealCurve } }
    Behavior on height { enabled: backdrop.band.animate; NumberAnimation { duration: Motion.rowReveal; easing.type: Motion.rowRevealCurve } }

    // outer = the screen-edge side; inner is the desktop-facing one it rounds.
    readonly property bool outerTop: backdrop.horizontal && backdrop.edge === "top"
    readonly property bool outerBottom: backdrop.horizontal && backdrop.edge === "bottom"
    readonly property bool outerLeft: !backdrop.horizontal && backdrop.edge === "left"
    readonly property bool outerRight: !backdrop.horizontal && backdrop.edge === "right"

    Rectangle {
        id: plate
        anchors.fill: parent
        color: Qt.rgba(Theme.surfaceContainer.r, Theme.surfaceContainer.g, Theme.surfaceContainer.b, backdrop.capsule ? 0.92 : (backdrop.band.frost ? 0.6 : 0.94))
        border.width: (backdrop.ledger || backdrop.capsule) ? 1 : 0
        border.color: backdrop.band.hairline
        topLeftRadius:     (backdrop.ledger || backdrop.capsule) ? backdrop.r : ((backdrop.outerBottom || backdrop.outerRight) ? backdrop.r : 0)
        topRightRadius:    (backdrop.ledger || backdrop.capsule) ? backdrop.r : ((backdrop.outerBottom || backdrop.outerLeft) ? backdrop.r : 0)
        bottomLeftRadius:  (backdrop.ledger || backdrop.capsule) ? backdrop.r : ((backdrop.outerTop || backdrop.outerRight) ? backdrop.r : 0)
        bottomRightRadius: (backdrop.ledger || backdrop.capsule) ? backdrop.r : ((backdrop.outerTop || backdrop.outerLeft) ? backdrop.r : 0)

        RectangularShadow {
            anchors.fill: parent
            radius: backdrop.r
            blur: 12
            spread: 0
            offset: backdrop.horizontal
                ? Qt.vector2d(0, backdrop.edge === "bottom" ? -2 : 2)
                : Qt.vector2d(backdrop.edge === "right" ? -2 : 2, 0)
            color: Qt.rgba(Theme.shadow.r, Theme.shadow.g, Theme.shadow.b, 0.5)
            visible: backdrop.band.shadow && !Perf.shadowsDisabled
            z: -1
        }
    }

    // Rail's single hairline rule along the inner edge (no box border).
    Rectangle {
        visible: backdrop.style === "rail"
        color: backdrop.band.hairline
        width: backdrop.horizontal ? parent.width : 1
        height: backdrop.horizontal ? 1 : parent.height
        x: backdrop.horizontal ? 0 : (backdrop.outerLeft ? parent.width - 1 : 0)
        y: backdrop.horizontal ? (backdrop.outerTop ? parent.height - 1 : 0) : 0
    }
}
