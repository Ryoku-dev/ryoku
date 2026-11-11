import QtQuick

Slab {
    id: root

    property string title: "Preview"
    property var facts: []
    property string preview: ""

    implicitWidth: LithicTokens.size.draftAsideMin

    Column {
        width: root.width - root.inset * 2
        spacing: LithicTokens.space.s3

        Label {
            width: parent.width
            text: root.title
            typeRole: "title"
            wrapMode: Text.Wrap
        }

        Facts {
            visible: root.facts.length > 0
            width: parent.width
            facts: root.facts
        }

        Slab {
            visible: root.preview !== ""
            width: parent.width
            fill: Lithic.field
            cut: LithicTokens.cut.small
            inset: LithicTokens.space.s3

            Label {
                width: parent.width
                text: root.preview
                typeRole: "monoSm"
                wrapMode: Text.WrapAnywhere
            }
        }
    }
}
