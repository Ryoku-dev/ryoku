import QtQuick
import Ryoku.Ui.Singletons

Item {
    id: bar

    property string title: ""
    default property alias trailing: trailingRow.data
    signal back()

    implicitHeight: 48

    IconButton {
        id: backButton
        anchors { left: parent.left; verticalCenter: parent.verticalCenter }
        icon: "arrow_back"
        tip: I18n.tr("Back")
        onClicked: bar.back()
    }

    Label {
        anchors {
            left: backButton.right; leftMargin: 8
            right: trailingRow.left; rightMargin: 8
            verticalCenter: parent.verticalCenter
        }
        role: "headline"
        text: bar.title
    }

    Row {
        id: trailingRow
        anchors { right: parent.right; verticalCenter: parent.verticalCenter }
        spacing: 8
    }
}
