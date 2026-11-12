import QtQuick

// Labels keep the 140 column and values start s4 after it. Either side wraps,
// never elides, and a row grows past 44 to fit its taller side.
Item {
    id: root

    property var facts: []

    readonly property real _gap: LithicTokens.space.s4
    readonly property real _labelWidth: Math.min(LithicTokens.size.factsLabelWidth,
                                                 Math.max(0, width - _gap))

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
                height: Math.max(LithicTokens.size.factsRowHeight,
                                 label.implicitHeight, value.implicitHeight)

                Label {
                    id: label
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    width: root._labelWidth
                    text: modelData.label || ""
                    typeRole: "body"
                    inkRole: "fg3"
                    wrapMode: Text.WrapAtWordBoundaryOrAnywhere
                    elide: Text.ElideNone
                }

                Label {
                    id: value
                    anchors.left: parent.left
                    anchors.leftMargin: root._labelWidth + root._gap
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    text: modelData.value === undefined || modelData.value === ""
                        ? "-"
                        : modelData.value
                    typeRole: modelData.literal ? "mono" : "body"
                    inkRole: "fg"
                    wrapMode: Text.WrapAtWordBoundaryOrAnywhere
                    elide: Text.ElideNone
                }
            }
        }
    }
}
