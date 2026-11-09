import QtQuick

Behavior {
    id: root
    property int duration: LithicTokens.motion.select
    NumberAnimation {
        duration: Lithic.duration(root.duration)
        easing.type: Easing.BezierSpline
        easing.bezierCurve: Lithic.curve
    }
}
