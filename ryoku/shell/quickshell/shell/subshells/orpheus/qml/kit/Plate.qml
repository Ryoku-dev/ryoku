import QtQuick

// A plate: the cut plaque Orpheus sets its controls and groups on. Design-aware
// (Shape): Lithic cuts it, the round styles round it; Material lifts it a tone
// with no outline. Colour stays on the shell palette.
Shape {
    id: root

    property bool active: false
    readonly property color inkColor: active ? Style.inkOnBone : Style.ink

    implicitWidth: childrenRect.width
    implicitHeight: childrenRect.height
    cutSize: Look.cutMd
    fill: active ? Style.bone : Style.plate
    strokeWidth: Look.material && !active ? 0 : Style.border
    strokeColor: active ? Style.bone : Style.lineSoft

    Behavior on fill {
        enabled: !Style.still
        ColorAnimation { duration: Style.snap }
    }
}