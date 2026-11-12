import QtQuick

Item {
    id: button

    property string icon: ""
    property string variant: "standard"
    property bool checked: false
    property bool danger: false
    property int size: 40
    property int iconSize: 22
    property string tip: ""
    signal clicked()
    signal secondary()

    readonly property bool lit: button.variant === "filled" || button.checked
    readonly property color body: button.lit ? (button.danger ? Md.error : Md.accent)
        : button.variant === "tonal" ? Md.containerHighest : "transparent"
    readonly property color glyphColor: button.lit ? (button.danger ? Md.onError : Md.onAccent) : Md.ink

    implicitWidth: button.size
    implicitHeight: button.size
    opacity: button.enabled ? 1 : 0.38

    Accessible.role: Accessible.Button
    Accessible.name: button.tip

    Rectangle {
        id: shape
        anchors.fill: parent
        radius: hit.pressed ? Md.rMd : height / 2
        color: button.body
        Behavior on radius {
            enabled: !Md.still
            NumberAnimation { duration: Md.fast }
        }
        Behavior on color {
            enabled: !Md.still
            ColorAnimation { duration: Md.fast }
        }
    }

    Icon {
        anchors.centerIn: parent
        name: button.icon
        size: button.iconSize
        filled: button.lit
        color: button.glyphColor
    }

    StateLayer {
        id: hit
        radius: shape.radius
        tint: button.glyphColor
        onActivated: button.clicked()
        onSecondary: button.secondary()
    }
}
