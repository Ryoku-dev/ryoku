pragma ComponentBehavior: Bound

import QtQuick
import Lithic

// a divided list of changes or packages: the name, and the version it moves from and to
Column {
    id: list

    // [{ name, old, new }]
    property var items: []

    Repeater {
        model: list.items
        delegate: Item {
            id: entry
            required property var modelData
            required property int index
            readonly property string from: String(entry.modelData.old || "")
            readonly property string to: String(entry.modelData["new"] || "")
            readonly property real gap: entry.index === 0 ? 0 : LithicTokens.size.settingsRowGap

            width: list.width
            height: entry.gap + Math.max(LithicTokens.size.factsRowHeight * 0.75, name.implicitHeight)

            Rectangle {
                visible: entry.index > 0
                y: Math.floor((entry.gap - LithicTokens.size.hairline) / 2)
                width: entry.width
                height: LithicTokens.size.hairline
                color: Lithic.vein
            }
            Label {
                id: name
                anchors.left: parent.left
                anchors.right: versions.left
                anchors.rightMargin: LithicTokens.space.s4
                y: entry.gap + (entry.height - entry.gap - height) / 2
                text: String(entry.modelData.name || "")
                typeRole: "body"
            }
            Label {
                id: versions
                anchors.right: parent.right
                y: entry.gap + (entry.height - entry.gap - height) / 2
                width: Math.min(implicitWidth, entry.width * 0.45)
                horizontalAlignment: Text.AlignRight
                text: entry.from !== "" && entry.to !== "" ? entry.from + "  \u2192  " + entry.to : entry.to
                typeRole: "monoSm"
                inkRole: "fg2"
                elide: Text.ElideLeft
            }
        }
    }
}
