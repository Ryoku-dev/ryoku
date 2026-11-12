import QtQuick

Item {
    id: button

    property string text: ""
    property string icon: ""
    property string variant: "tonal"
    property bool danger: false
    signal clicked()

    readonly property color body: button.variant === "filled"
        ? (button.danger ? Md.error : Md.accent)
        : button.variant === "tonal" ? Md.containerHighest : "transparent"
    readonly property color ink: button.variant === "filled"
        ? (button.danger ? Md.onError : Md.onAccent)
        : button.danger ? Md.error
        : (button.variant === "text" ? Md.accent : Md.ink)

    implicitWidth: row.implicitWidth + (button.icon.length > 0 ? 40 : 48)
    implicitHeight: 40
    opacity: button.enabled ? 1 : 0.38

    Accessible.role: Accessible.Button
    Accessible.name: button.text

    Rectangle {
        id: shape
        anchors.fill: parent
        radius: hit.pressed ? Md.rMd : height / 2
        color: button.body
        border.width: button.variant === "outlined" ? 1 : 0
        border.color: Md.outline
        Behavior on radius {
            enabled: !Md.still
            NumberAnimation { duration: Md.fast }
        }
    }

    Row {
        id: row
        anchors.centerIn: parent
        spacing: 8

        Icon {
            anchors.verticalCenter: parent.verticalCenter
            visible: button.icon.length > 0
            name: button.icon
            size: 18
            color: button.ink
        }
        Label {
            anchors.verticalCenter: parent.verticalCenter
            role: "label"
            text: button.text
            color: button.ink
        }
    }

    StateLayer {
        id: hit
        radius: shape.radius
        tint: button.ink
        onActivated: button.clicked()
    }
}
