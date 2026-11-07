import QtQuick

Item {
    id: root

    property string text: ""
    property string icon: ""
    property string variant: "tonal"

    signal clicked()

    readonly property bool filled: variant === "filled" || variant === "danger"

    implicitWidth: buttonLabel.implicitWidth + 30
    implicitHeight: 32
    opacity: enabled ? 1 : 0.5
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
        color: root.filled ? Style.bone
            : buttonPress.pressed ? Style.tint16
            : buttonPress.hovered ? Style.tint10 : "transparent"
        border.width: Style.border
        border.color: root.filled ? Style.bone
            : root.activeFocus || buttonPress.hovered ? Style.lineStrong : Style.line

        Behavior on color {
            enabled: !Style.still
            ColorAnimation { duration: Style.snap }
        }
    }

    Label {
        id: buttonLabel
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.margins: Style.s3
        anchors.verticalCenter: parent.verticalCenter
        text: root.text
        role: "micro"
        tone: root.filled ? "onBone" : "ink"
        strong: true
        horizontalAlignment: Text.AlignHCenter
    }

    Press {
        id: buttonPress
        enabled: root.enabled
        tint: false
        onClicked: {
            root.forceActiveFocus();
            root.clicked();
        }
    }
}
