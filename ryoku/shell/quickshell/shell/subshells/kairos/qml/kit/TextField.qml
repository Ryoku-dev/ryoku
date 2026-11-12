import QtQuick

Item {
    id: box

    property alias text: input.text
    property alias field: input
    property string placeholder: ""
    property bool password: false
    property bool error: false
    signal accepted()
    signal escaped()

    implicitWidth: 280
    implicitHeight: 52

    Rectangle {
        anchors.fill: parent
        topLeftRadius: Md.rMd
        topRightRadius: Md.rMd
        color: Md.containerHigh
    }

    Rectangle {
        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
        height: input.activeFocus || box.error ? 2 : 1
        color: box.error ? Md.error : input.activeFocus ? Md.accent : Md.inkFaint
    }

    TextInput {
        id: input
        anchors {
            left: parent.left; right: parent.right
            leftMargin: 16; rightMargin: 16
            verticalCenter: parent.verticalCenter
        }
        clip: true
        color: Md.ink
        font.family: Md.font
        font.pixelSize: 15
        echoMode: box.password ? TextInput.Password : TextInput.Normal
        selectionColor: Md.accent
        selectedTextColor: Md.onAccent
        selectByMouse: true
        onAccepted: box.accepted()
        Keys.onEscapePressed: function (event) {
            box.escaped();
            event.accepted = true;
        }

        Label {
            anchors.fill: parent
            visible: input.text.length === 0
            role: "body"
            font.pixelSize: 15
            text: box.placeholder
            color: Md.inkFaint
        }
    }

    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.IBeamCursor
        acceptedButtons: Qt.NoButton
    }
}
