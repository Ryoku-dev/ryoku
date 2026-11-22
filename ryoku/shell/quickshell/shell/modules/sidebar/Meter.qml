import QtQuick
import Lithic

Item {
    id: root

    property real s: 1
    property real value: 0
    property color ink: QsLook.lithic ? Lithic.fg2 : Lithic.sel
    property int settle: 0

    readonly property real fraction: Math.max(0, Math.min(1, isFinite(root.value) ? root.value : 0))
    // glide over settle ms so a sampled reading never steps
    property real shown: root.fraction
    Behavior on shown {
        NumberAnimation { duration: QsLook.dur(root.settle); easing.type: Easing.Linear }
    }

    implicitHeight: Math.max(2, Math.round((QsLook.lithic ? 4 : QsLook.material ? 4 : 3) * root.s))

    Blade {
        visible: QsLook.lithic
        anchors.fill: parent
        fraction: root.shown
        fill: root.ink
        track: Lithic.field
    }
    Rectangle {
        visible: !QsLook.lithic
        anchors.fill: parent
        radius: height / 2
        color: QsLook.material ? Lithic.mix(Lithic.field, Lithic.fg, 0.04) : Lithic.alpha(Lithic.fg, QsLook.dark ? 0.14 : 0.12)

        Rectangle {
            width: Math.max(root.shown > 0 ? height : 0, parent.width * root.shown)
            height: parent.height
            radius: height / 2
            color: root.ink
        }
    }
}
