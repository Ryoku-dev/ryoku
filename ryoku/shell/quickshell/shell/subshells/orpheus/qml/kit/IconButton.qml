import QtQuick

Item {
    id: root

    property string icon: ""
    property real size: 36
    property string variant: "standard"
    property bool toggled: false

    signal clicked()
    signal rightClicked()

    readonly property bool plain: variant === "plain" || variant === "text"
    readonly property bool inverted: toggled || variant === "filled"

    implicitWidth: size
    implicitHeight: size
    opacity: enabled ? 1 : 0.38
    scale: buttonPress.pressScale
    activeFocusOnTab: true

    Keys.onPressed: event => {
        if ((event.key === Qt.Key_Return || event.key === Qt.Key_Enter
                || event.key === Qt.Key_Space) && !event.isAutoRepeat) {
            root.clicked();
            event.accepted = true;
        }
    }

    Rectangle {
        anchors.fill: parent
        radius: Style.radius
        color: root.inverted ? Style.bone
            : buttonPress.pressed ? Style.tint16
            : buttonPress.hovered ? Style.tint10 : "transparent"
        border.width: root.plain && !root.inverted ? 0 : Style.border
        border.color: root.inverted ? Style.bone
            : root.activeFocus || buttonPress.hovered ? Style.lineStrong : Style.line

        Behavior on color {
            enabled: !Style.still
            ColorAnimation { duration: Style.snap }
        }
    }

    Glyph {
        anchors.centerIn: parent
        name: root.icon
        size: Style.icon
        tone: root.inverted ? "onBone" : buttonPress.hovered ? "ink" : "dim"
        filled: root.toggled
    }

    Press {
        id: buttonPress
        enabled: root.enabled
        tint: false
        onClicked: {
            root.forceActiveFocus();
            root.clicked();
        }
        onRightClicked: root.rightClicked()
    }
}
