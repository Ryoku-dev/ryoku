import QtQuick
import Lithic
import "look"
import "lib/dockplace.js" as Place

// A popover that opens from a dock button and stays whole on the output: a cut
// plane in Lithic, a card in Material, Adwaita's popover, Fluent's flyout.
// anchorRect is the button's column over the whole dock, in the parent, so the
// flyout clears both rows; it opens on the preferred side and flips when that
// side has no room. Lithic sets its edge on the button's left, like the dock
// under the selection; the others centre on it.
Item {
    id: root

    property real u: 1
    property bool open: false
    property rect anchorRect: Qt.rect(0, 0, 0, 0)
    property bool preferAbove: true
    property string align: Look.lithic ? "start" : "center"
    property real gap: 6
    property real padding: Look.lithic ? 10 : Look.material ? 16 : 12
    // Material's cards round more than its menus
    property real materialRadius: 24

    default property alias content: holder.data
    signal closeRequested()

    implicitWidth: Math.ceil(holder.childrenRect.width + root.padding * 2 * root.u)
    implicitHeight: Math.ceil(holder.childrenRect.height + root.padding * 2 * root.u)
    width: implicitWidth
    height: implicitHeight
    z: 200

    readonly property var spot: root.parent
        ? Place.popover({ x: root.anchorRect.x, w: root.anchorRect.width },
            { top: root.anchorRect.y, bottom: root.anchorRect.y + root.anchorRect.height },
            { w: root.width, h: root.height }, { w: root.parent.width, h: root.parent.height },
            root.preferAbove ? "up" : "down",
            { align: root.align, gap: root.gap * root.u, margin: 8 * root.u })
        : { x: 0, y: 0, side: "up" }
    readonly property bool opensUp: root.spot.side === "up"
    x: Math.round(root.spot.x)
    y: Math.round(root.spot.y)

    focus: root.open
    Keys.onEscapePressed: root.closeRequested()

    property real reveal: root.open ? 1 : 0
    Behavior on reveal {
        NumberAnimation {
            duration: Look.dur(root.open ? (Look.fluent ? 250 : Look.material ? 260 : 160) : 100)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: root.open ? (Look.material ? Look.springDefault : Look.fluent ? Look.fluentIn : Look.decelerate) : Look.accelerate
        }
    }
    visible: root.reveal > 0
    enabled: root.open
    opacity: Math.min(1, root.reveal * (Look.material ? 2 : 1))

    readonly property real slide: (Look.fluent ? 12 : Look.material ? 0 : 6) * root.u * (1 - Math.min(1, root.reveal))
    transform: [
        Translate { y: root.opensUp ? root.slide : -root.slide },
        Scale {
            origin.x: Look.lithic ? 0 : root.width / 2
            origin.y: root.opensUp ? root.height : 0
            xScale: Look.material ? 0.92 + 0.08 * root.reveal : Look.adwaita ? 0.96 + 0.04 * root.reveal : 1
            yScale: Look.material ? 0.92 + 0.08 * root.reveal : Look.adwaita ? 0.96 + 0.04 * root.reveal : 1
        }
    ]

    // clicks inside never reach the shot under it
    MouseArea {
        anchors.fill: parent
        hoverEnabled: true
        acceptedButtons: Qt.AllButtons
        onWheel: wheel => wheel.accepted = true
    }

    Surface {
        anchors.fill: parent
        u: root.u
        cut: Look.cutLg
        radius: (Look.material ? root.materialRadius : Look.adwaita ? 12 : 8) * root.u
        elevation: 2
    }

    Item {
        id: holder
        x: Math.round(root.padding * root.u)
        y: Math.round(root.padding * root.u)
        width: childrenRect.width
        height: childrenRect.height
    }
}
