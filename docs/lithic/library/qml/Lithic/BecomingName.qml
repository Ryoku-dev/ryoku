import QtQuick

Item {
    id: root

    property string name: ""
    property string stage: ""
    property real progress: 0
    property var lines: []
    property bool active: true

    implicitWidth: LithicTokens.size.draftFormMax
    implicitHeight: content.implicitHeight

    Column {
        id: content
        width: parent.width
        spacing: LithicTokens.space.s2

        Item {
            width: parent.width
            height: outlineName.implicitHeight

            Label {
                id: outlineName
                width: parent.width
                text: root.name
                typeRole: "sentence"
                color: "transparent"
                style: Text.Outline
                styleColor: Lithic.fg
                maximumLineCount: 1
            }

            Item {
                width: Math.max(0, Math.min(1, root.progress)) * parent.width
                height: parent.height
                clip: true

                Label {
                    width: parent.parent.width
                    text: root.name
                    typeRole: "sentence"
                    inkRole: "fg"
                    maximumLineCount: 1
                }
            }
        }

        Label {
            visible: root.stage !== ""
            width: parent.width
            text: root.stage
            typeRole: "label"
            inkRole: "fg2"
        }

        BladeProgress {
            width: parent.width
            value: root.progress
            active: root.active
        }

        Repeater {
            model: root.lines
            delegate: Label {
                required property string modelData
                width: content.width
                text: modelData
                typeRole: "mono"
                inkRole: "fg2"
                maximumLineCount: 1
            }
        }
    }
}
