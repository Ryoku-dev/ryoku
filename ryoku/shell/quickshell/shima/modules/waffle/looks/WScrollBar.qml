import QtQuick
import QtQuick.Controls
import qs.modules.common
import qs.modules.common.widgets
import qs.modules.common.functions
import qs.modules.waffle.looks

ScrollBar {
    id: root

    policy: ScrollBar.AsNeeded
    active: hovered || pressed
    property color color: Looks.colors.controlBg

    contentItem: Rectangle {
        implicitWidth: root.active ? 4 : 2
        implicitHeight: root.visualSize
        radius: 9999
        color: root.color
        
        opacity: root.policy === ScrollBar.AlwaysOn || (root.active && root.size < 1.0) ? 0.5 : 0
        Behavior on opacity {
            animation: NumberAnimation { duration: Looks.transition.enabled ? Looks.transition.duration.normal : 0; easing.type: Easing.BezierSpline; easing.bezierCurve: Looks.transition.easing.bezierCurve.standard }
        }
    }
}
