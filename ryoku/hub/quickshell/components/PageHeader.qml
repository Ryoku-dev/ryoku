import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../registry/registry.js" as Registry

// the head of a page: where it lives, its name, what it is for, and what it says about the
// machine now. The page's own actions (Reset page) stand on the name's line, clear of the
// window's close button; nothing else takes room above the first section
Column {
    id: root

    required property var hub
    required property string pageId
    property string title: ""
    property string lead: ""
    property string stateLine: ""
    property string note: ""
    default property alias actions: actionRow.data

    readonly property var category: {
        var p = Registry.page(root.pageId);
        return p ? Registry.category(p.category) : null;
    }

    spacing: 0
    bottomPadding: 26

    Row {
        height: 22
        spacing: 6
        // a page named as its category (Updates) does not say it twice
        visible: !!root.category && I18n.tr(root.category.label) !== root.title

        Glyph {
            anchors.verticalCenter: parent.verticalCenter
            name: root.category ? root.category.glyph : ""
            size: 14
            ink: Tone.accent
        }
        Label {
            anchors.verticalCenter: parent.verticalCenter
            text: root.category ? I18n.tr(root.category.label) : ""
            typeRole: "group"
            inkRole: "fg3"
            font.capitalization: Font.AllUppercase
            font.letterSpacing: 1.2
        }
    }

    Item {
        width: root.width
        height: Math.max(titleLabel.height, actionRow.height)

        Label {
            id: titleLabel
            width: Math.min(implicitWidth, parent.width - actionRow.width - 56)
            anchors.verticalCenter: parent.verticalCenter
            text: root.title
            typeRole: "sentence"
            font.pixelSize: 28
            lineHeight: 36
        }
        // the window's close button stands in the corner above this line's end
        Row {
            id: actionRow
            anchors.right: parent.right
            anchors.rightMargin: 44
            anchors.verticalCenter: parent.verticalCenter
            spacing: 6
        }
    }

    Label {
        width: Math.min(parent.width - 44, 720)
        visible: root.lead.length > 0
        topPadding: 4
        text: root.lead
        typeRole: "body"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
    Label {
        width: Math.min(parent.width - 44, 720)
        visible: root.stateLine.length > 0
        topPadding: 6
        text: root.stateLine
        typeRole: "detail"
        inkRole: "fg"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
    Row {
        visible: root.note.length > 0
        topPadding: 8
        spacing: 6

        Glyph {
            name: "info"
            size: 14
            ink: Lithic.fg3
            y: 2
        }
        Label {
            width: Math.min(implicitWidth, root.width - 64)
            text: root.note
            typeRole: "caption"
            inkRole: "fg3"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }
}
