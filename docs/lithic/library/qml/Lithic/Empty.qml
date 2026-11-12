import QtQuick

Item {
    id: root

    property string glyph: "inbox"
    property string title: Lithic.tr("Nothing here yet")
    property string body: Lithic.tr("Create the first item to continue.")
    property var actions: []

    signal actionTriggered(int index)

    implicitWidth: LithicTokens.size.emptyBodyMax
    implicitHeight: content.implicitHeight

    Column {
        id: content
        anchors.centerIn: parent
        width: Math.min(root.width, LithicTokens.size.emptyBodyMax)
        spacing: LithicTokens.space.s2

        CutRect {
            anchors.horizontalCenter: parent.horizontalCenter
            width: LithicTokens.size.emptyFrame
            height: LithicTokens.size.emptyFrame
            fill: "transparent"
            strokeColor: Lithic.vein
            strokeWidth: LithicTokens.size.hairline
            cut: LithicTokens.cut.large

            Glyph {
                anchors.centerIn: parent
                name: root.glyph
                size: LithicTokens.size.emptyIcon
                ink: Lithic.fg3
            }
        }

        Label {
            anchors.horizontalCenter: parent.horizontalCenter
            text: root.title
            typeRole: "title"
        }

        Label {
            width: parent.width
            text: root.body
            typeRole: "body"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            horizontalAlignment: Text.AlignHCenter
        }

        Row {
            anchors.horizontalCenter: parent.horizontalCenter
            spacing: LithicTokens.space.s2

            Repeater {
                model: root.actions
                delegate: Plaque {
                    required property var modelData
                    required property int index
                    text: modelData.text || ""
                    kind: modelData.kind || (index === 0 ? "gold" : "line")
                    glyph: modelData.glyph || ""
                    enabled: modelData.enabled === undefined || modelData.enabled
                    onClicked: root.actionTriggered(index)
                }
            }
        }
    }
}
