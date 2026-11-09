import QtQuick

Behavior {
    id: root
    property int duration: LithicTokens.motion.hoverIn
    ColorAnimation {
        duration: Lithic.duration(root.duration)
        easing.type: Easing.BezierSpline
        easing.bezierCurve: Lithic.curve
    }
}
