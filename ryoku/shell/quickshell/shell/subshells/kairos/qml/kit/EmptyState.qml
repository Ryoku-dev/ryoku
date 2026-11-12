import QtQuick

Column {
    id: empty

    property string icon: ""
    property string title: ""
    property string body: ""
    property string action: ""
    signal acted()

    width: parent ? parent.width : 0
    topPadding: 28
    bottomPadding: 20
    spacing: 10

    Icon {
        anchors.horizontalCenter: parent.horizontalCenter
        name: empty.icon
        size: 36
        color: Md.inkMuted
    }
    Label {
        width: parent.width
        horizontalAlignment: Text.AlignHCenter
        role: "title"
        text: empty.title
    }
    Label {
        width: parent.width - 48
        anchors.horizontalCenter: parent.horizontalCenter
        horizontalAlignment: Text.AlignHCenter
        role: "body"
        color: Md.inkMuted
        wrapMode: Text.Wrap
        elide: Text.ElideNone
        text: empty.body
    }
    Item {
        width: 1
        height: 4
        visible: actionButton.visible
    }
    Button {
        id: actionButton
        anchors.horizontalCenter: parent.horizontalCenter
        visible: empty.action.length > 0
        variant: "filled"
        text: empty.action
        onClicked: empty.acted()
    }
}
