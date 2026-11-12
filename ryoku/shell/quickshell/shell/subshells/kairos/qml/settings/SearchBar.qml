import QtQuick
import Ryoku.Ui.Singletons
import "../kit"

Item {
    id: bar

    property alias text: input.text
    property alias field: input
    signal escaped()

    implicitWidth: 320
    implicitHeight: 56

    Rectangle {
        anchors.fill: parent
        radius: height / 2
        color: Md.containerHigh
        border.width: input.activeFocus ? 2 : 0
        border.color: Md.accent
    }

    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.IBeamCursor
        onClicked: input.forceActiveFocus()
    }

    Icon {
        id: glyph
        anchors { left: parent.left; leftMargin: 18; verticalCenter: parent.verticalCenter }
        name: "search"
        size: 24
        color: Md.inkMuted
    }

    TextInput {
        id: input
        anchors {
            left: glyph.right; leftMargin: 14
            right: clear.visible ? clear.left : parent.right; rightMargin: clear.visible ? 4 : 20
            verticalCenter: parent.verticalCenter
        }
        clip: true
        color: Md.ink
        font.family: Md.font
        font.pixelSize: 16
        selectionColor: Md.accent
        selectedTextColor: Md.onAccent
        selectByMouse: true
        Accessible.role: Accessible.EditableText
        Accessible.name: I18n.tr("Search settings")
        // the first Escape clears the query, the next leaves the field so the window can take it
        Keys.onEscapePressed: function (event) {
            if (input.text.length > 0)
                input.text = "";
            else
                bar.escaped();
            event.accepted = true;
        }

        Label {
            anchors.fill: parent
            visible: input.text.length === 0
            role: "body"
            font.pixelSize: 16
            color: Md.inkMuted
            text: I18n.tr("Search settings")
        }
    }

    IconButton {
        id: clear
        anchors { right: parent.right; rightMargin: 8; verticalCenter: parent.verticalCenter }
        visible: input.text.length > 0
        icon: "close"
        tip: I18n.tr("Clear search")
        onClicked: {
            input.text = "";
            input.forceActiveFocus();
        }
    }
}
