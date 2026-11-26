import QtQuick
import Lithic

// A drawer that buds from a button on the action island
Item {
    id: body

    required property PickerState state
    required property string key
    required property real anchorX
    property bool shown: false
    property real maxWidth: 1320
    // a width change (a section that previews on the cards) flows the body, not snaps it
    Behavior on maxWidth { enabled: !Lithic.reduceMotion; NumberAnimation { duration: Theme.slow; easing.type: Easing.InOutCubic } }
    default property alias content: inner.data
    readonly property real s: Theme.scale

    anchors.fill: parent

    property real progress: body.shown ? 1 : 0
    Behavior on progress {
        enabled: !Lithic.reduceMotion
        NumberAnimation {
            duration: body.shown ? Theme.slow : Theme.standard
            easing.type: body.shown ? Easing.OutCubic : Easing.InCubic
        }
    }
    readonly property real targetW: Math.min(body.maxWidth * body.s, body.width - 48 * body.s)
    readonly property real topY: body.state.chromeBottom + body.state.drawerGap * body.s
    readonly property rect target: Qt.rect(
        Math.max(24 * body.s, Math.min(body.width - body.targetW - 24 * body.s, body.state.chromeRight - body.targetW)),
        body.topY, body.targetW, body.height - body.topY - 24 * body.s)
    readonly property real contentOpacity: Math.max(0, (body.progress - 0.55) / 0.45)

    readonly property bool live: body.progress > 0.001
    readonly property bool owns: body.state.drawerKey === body.key
    onLiveChanged: body._publish()
    onShownChanged: body._publish()
    function _publish() {
        if (body.live || body.shown)
            body.state.drawerKey = body.key
        else if (body.owns) {
            body.state.drawerKey = ""
            body.state.drawerProgress = 0
        }
    }
    Binding { when: body.owns; target: body.state; property: "drawerProgress"; value: body.progress }
    Binding { when: body.owns; target: body.state; property: "drawerTarget"; value: body.target }
    Binding { when: body.owns; target: body.state; property: "drawerAnchorX"; value: body.anchorX }

    visible: body.live && body.owns

    Item {
        id: clipper
        readonly property rect now: body.state.drawerNow
        x: now.x
        y: now.y
        width: now.width
        height: now.height
        clip: true
        // keeps clicks and the wheel inside the drawer off the scrim behind it
        MouseArea { anchors.fill: parent; hoverEnabled: true; onWheel: (w) => w.accepted = true }

        Item {
            id: inner
            x: body.target.x - clipper.x
            y: body.target.y - clipper.y
            width: body.target.width
            height: body.target.height
            opacity: body.contentOpacity
        }
    }
}
