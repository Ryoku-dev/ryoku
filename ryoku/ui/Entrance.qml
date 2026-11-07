import QtQuick
import Ryoku.Ui.Singletons

// A stagger-reveal wrapper: lifts its content a step and fades it in, delayed by
// `index`, so a column arrives one card after another instead of popping in.
//
// The rise is a Translate and the fade is opacity, so childrenRect (and with it
// implicitHeight) always reports the child's resting height: a column measuring
// these, and a plate sized from that measurement, never sees a mid-reveal size.
Item {
    id: root

    property int index: 0
    // a sixth of the step reads as a ripple rather than a queue
    property int stride: Math.round(Tokens.durFastSpatial / 6)
    // past the cap the tail arrives together, so a long column stays under a second
    property int maxDelay: Tokens.durDefaultSpatial
    property bool shown: true
    property real rise: Tokens.s3

    readonly property int _delay: Math.min(root.index * root.stride, root.maxDelay)

    default property alias content: holder.data

    // A filled Entrance must not read its size back from children that fill
    // it in turn; that cycle was the launcher's binding loop (#383).
    implicitWidth: root.anchors.fill ? 0 : holder.childrenRect.width
    implicitHeight: root.anchors.fill ? 0 : holder.childrenRect.height

    // bound to opacity and the lift, so an interrupted stagger still resolves to
    // shown instead of stranding a card invisible
    property bool _revealed: false

    opacity: _revealed ? 1 : 0
    Behavior on opacity {
        enabled: Tokens.durDefaultEffects > 0
        NumberAnimation {
            duration: Tokens.durDefaultEffects
            easing.type: Easing.Bezier
            easing.bezierCurve: Tokens.curveDefaultEffects
        }
    }

    transform: Translate {
        id: lift
        y: root._revealed ? 0 : root.rise
        Behavior on y {
            enabled: Tokens.durFastSpatial > 0
            NumberAnimation {
                duration: Tokens.durFastSpatial
                easing.type: Easing.Bezier
                easing.bezierCurve: Tokens.curveDefaultSpatial
            }
        }
    }

    Item {
        id: holder
        anchors.fill: parent
    }

    Timer {
        id: delayTimer
        interval: root._delay
        repeat: false
        onTriggered: root._revealed = true
    }

    // With no delay (reduce-motion zeroes the stride, or this is the first item)
    // flip immediately so nothing waits on a timer; otherwise arm the one-shot.
    // A false `shown` clears the reveal; true re-arms it.
    function _arm() {
        delayTimer.stop();
        if (!root.shown) {
            root._revealed = false;
            return;
        }
        if (root._delay <= 0)
            root._revealed = true;
        else
            delayTimer.restart();
    }

    onShownChanged: _arm()
    Component.onCompleted: _arm()
}
