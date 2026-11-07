import QtQuick

FocusScope {
    id: root

    property alias text: input.text
    property string placeholder: ""

    signal accepted(string text)
    signal cancelled()

    implicitWidth: Math.max(180,
        Math.max(input.contentWidth, placeholderLabel.implicitWidth) + Style.s4)
    implicitHeight: 32

    Rectangle {
        anchors.fill: parent
        radius: Style.radius
        color: "transparent"
        border.width: Style.border
        border.color: input.activeFocus ? Style.bone
            : fieldHover.hovered ? Style.lineStrong : Style.line

        Behavior on border.color {
            enabled: !Style.still
            ColorAnimation { duration: Style.snap }
        }
    }

    Label {
        id: placeholderLabel
        anchors.left: parent.left
        anchors.leftMargin: Style.s2
        anchors.right: parent.right
        anchors.rightMargin: Style.s2
        anchors.verticalCenter: parent.verticalCenter
        visible: input.text.length === 0
        text: root.placeholder
        role: "label"
        tone: "muted"
    }

    TextInput {
        id: input
        anchors.left: parent.left
        anchors.leftMargin: Style.s2
        anchors.right: parent.right
        anchors.rightMargin: Style.s2
        anchors.verticalCenter: parent.verticalCenter
        activeFocusOnTab: true
        color: Style.ink
        selectionColor: Style.bone
        selectedTextColor: Style.inkOnBone
        font.family: Style.font
        font.pixelSize: Style.label
        font.capitalization: Font.MixedCase
        font.letterSpacing: 0
        verticalAlignment: TextInput.AlignVCenter
        clip: true

        Keys.onReturnPressed: event => {
            root.accepted(input.text);
            event.accepted = true;
        }
        Keys.onEnterPressed: event => {
            root.accepted(input.text);
            event.accepted = true;
        }
        Keys.onEscapePressed: event => {
            root.cancelled();
            input.focus = false;
            event.accepted = true;
        }
    }

    HoverHandler { id: fieldHover; cursorShape: Qt.IBeamCursor }
    TapHandler {
        acceptedButtons: Qt.LeftButton
        onTapped: input.forceActiveFocus()
    }
}
