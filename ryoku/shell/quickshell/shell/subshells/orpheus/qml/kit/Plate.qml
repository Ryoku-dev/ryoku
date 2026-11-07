import QtQuick

Rectangle {
    id: root

    property bool active: false
    readonly property color inkColor: active ? Style.inkOnBone : Style.ink

    implicitWidth: childrenRect.width
    implicitHeight: childrenRect.height
    radius: Style.plateRadius
    color: active ? Style.bone : Style.plate
    border.width: Style.border
    border.color: active ? Style.bone : Style.lineSoft

    Behavior on color {
        enabled: !Style.still
        ColorAnimation { duration: Style.snap }
    }
}
