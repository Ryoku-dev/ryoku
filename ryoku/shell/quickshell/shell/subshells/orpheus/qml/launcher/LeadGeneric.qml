pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons
import "../kit" as K
import "lib/rows.js" as Rows

// The lead's common body: the icon plate, the title and subtitle, a
// description, then the actions. Richer sub-views build on it: `body` lands
// between the text and the actions, the default slot after the actions. With no
// row in Apps mode it is the empty state, offering the web instead.
Column {
    id: root

    property var lead: null
    readonly property var row: root.lead ? root.lead.row : null
    readonly property var engine: root.lead ? root.lead.engine : null
    readonly property bool empty: root.lead !== null && root.lead.kind === "empty"

    property string title: root.empty
        ? I18n.tr("No app called “%1”").arg(String(root.engine ? root.engine.query : ""))
        : root.row ? String(root.row.title || "") : ""
    property string subtitle: root.row ? String(root.row.subtitle || "") : ""
    property string description: root.row && root.row.description ? String(root.row.description) : ""
    property string iconSource: root.row ? String(root.row.icon || "") : ""
    property string glyph: root.empty ? "search_off" : root.row ? Rows.glyphFor(root.row) : "apps"
    property alias body: bodySlot.data
    default property alias extra: extraSlot.data

    spacing: K.Style.s3

    IconPlate {
        source: root.iconSource
        glyph: root.glyph
        numeral: root.empty || !root.engine || root.engine.selectedIndex < 0 ? ""
            : Rows.indexLabel(root.engine.selectedIndex, true,
                Rows.indexDigits(root.engine.resultCount, true))
        rise: root.lead ? root.lead.rise : 0
    }

    Column {
        width: root.width
        spacing: K.Style.s1

        K.Label {
            width: parent.width
            text: root.title
            role: "headline"
            wrapMode: Text.Wrap
            maximumLineCount: 2
            elide: Text.ElideRight
        }

        K.Label {
            width: parent.width
            visible: text.length > 0
            text: root.subtitle
            role: "body"
            tone: "dim"
        }

        K.Label {
            width: parent.width
            visible: text.length > 0
            text: root.description
            role: "body"
            tone: "muted"
            wrapMode: Text.Wrap
            maximumLineCount: 2
            elide: Text.ElideRight
        }
    }

    Column {
        id: bodySlot
        width: root.width
        spacing: K.Style.s3
        // a hidden column reports no height, so judging by height would keep it hidden for good
        visible: children.length > 0
    }

    ActionList {
        width: root.width
        visible: root.lead !== null && root.lead.actionItems.length > 0
        items: root.lead ? root.lead.actionItems : []
        focusIndex: root.lead ? root.lead.actionFocus : -1
        scroller: root.lead ? root.lead.scroller : null
        onActivated: index => root.lead.runAt(index)
    }

    Item {
        visible: root.empty
        width: root.width
        height: K.Style.rowH

        Row {
            anchors.verticalCenter: parent.verticalCenter
            spacing: K.Style.s2

            KeyCap {
                anchors.verticalCenter: parent.verticalCenter
                text: "Enter"
            }

            K.Label {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("Search the web for it")
                role: "body"
                tone: webPress.hovered ? "ink" : "dim"
            }
        }

        K.Press {
            id: webPress
            tint: false
            onClicked: {
                if (root.engine)
                    root.engine.searchWeb();
            }
        }
    }

    Column {
        id: extraSlot
        width: root.width
        spacing: K.Style.s3
        visible: children.length > 0
    }
}
