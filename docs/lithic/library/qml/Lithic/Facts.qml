import QtQuick

Item {
    id: root

    property var facts: []

    implicitWidth: LithicTokens.size.draftFormMax
    implicitHeight: rows.implicitHeight

    Column {
        id: rows
        width: parent.width

        Repeater {
            model: root.facts
            delegate: Item {
                required property var modelData
                required property int index
                width: rows.width
                height: LithicTokens.size.factsRowHeight

                Label {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    width: LithicTokens.size.factsLabelWidth
                    text: modelData.label || ""
                    typeRole: "body"
                    inkRole: "fg3"
                }

                Label {
                    anchors.left: parent.left
                    anchors.leftMargin: LithicTokens.size.factsLabelWidth
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    text: modelData.value === undefined || modelData.value === ""
                        ? "-"
                        : modelData.value
                    typeRole: modelData.literal ? "mono" : "body"
                    inkRole: "fg"
                    maximumLineCount: 1
                }
            }
        }
    }
}
