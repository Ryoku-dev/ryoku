import QtQuick
import Lithic
import "../../ask/look"

// A setting that is on or off, drawn only: the row it sits in takes the press.
Shape {
    id: root

    property bool on: false

    width: Math.round(34 * root.s)
    height: Math.round(18 * root.s)
    cutSize: Look.lithic ? 4 : 9
    fill: root.on ? Lithic.sel : Lithic.alpha(Lithic.fg, Look.dark ? 0.16 : 0.18)
    Behavior on fill {
        ColorAnimation { duration: Look.dur(Look.stateMs) }
    }

    Shape {
        readonly property real inset: Math.round(3 * root.s)

        x: root.on ? parent.width - width - inset : inset
        y: inset
        width: parent.height - inset * 2
        height: width
        s: root.s
        cutSize: Look.lithic ? 2 : 6
        fill: root.on ? Lithic.onSel : Lithic.fg
        Behavior on x {
            NumberAnimation { duration: Look.dur(Look.plateMs); easing.type: Easing.BezierSpline; easing.bezierCurve: Look.arrive }
        }
    }
}
