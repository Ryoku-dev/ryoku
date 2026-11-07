import QtQuick
import "../kit" as K

NumberAnimation {
    duration: K.Style.ms(600)
    easing.type: Easing.BezierSpline
    easing.bezierCurve: K.Style.spatialCurve
}
