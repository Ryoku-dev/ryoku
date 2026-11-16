import QtQuick
import Ryoku.Ui.Singletons

// The display row on the window-manager page: a link toggle, then one tab per
// output. Linked, only the first output's tab is live and every setting applies
// to all screens. Unlinked, every output tab is live and edits its own scope.
// The link is a button with its own state, not a tab.
Row {
    id: row

    property var outputs: []
    property bool linked: true
    property string selected: ""
    signal linkToggled(bool linked)
    signal chose(string output)

    spacing: Tokens.s2
    visible: row.outputs.length >= 2

    Rectangle {
        id: linkBtn
        width: 34
        height: 34
        radius: Tokens.radius
        anchors.verticalCenter: parent.verticalCenter
        color: linkTap.pressed ? Tokens.tint16 : (linkHover.hovered ? Tokens.tint5 : "transparent")
        border.width: Tokens.border
        border.color: row.linked ? Tokens.bone : Tokens.line

        Text {
            anchors.centerIn: parent
            text: row.linked ? "link" : "link_off"
            color: row.linked ? Tokens.ink : Tokens.inkMuted
            font.family: "Material Symbols Rounded"
            font.pixelSize: 18
        }
        HoverHandler { id: linkHover; cursorShape: Qt.PointingHandCursor }
        TapHandler {
            id: linkTap
            onTapped: row.linkToggled(!row.linked)
        }
    }

    Repeater {
        model: row.outputs
        Rectangle {
            id: plate
            required property string modelData
            required property int index
            readonly property bool live: !row.linked || index === 0
            readonly property bool on: row.linked ? index === 0 : row.selected === modelData

            width: lab.implicitWidth + Tokens.s3 * 2
            height: 34
            radius: Tokens.radius
            anchors.verticalCenter: parent.verticalCenter
            color: on ? Tokens.bone : (plateTap.pressed ? Tokens.tint16 : (plateHover.hovered ? Tokens.tint5 : "transparent"))
            border.width: Tokens.border
            border.color: on ? Tokens.bone : Tokens.line
            opacity: live ? 1 : 0.4

            Text {
                id: lab
                anchors.centerIn: parent
                // the raw output name is the identity; Tabs-style uppercasing
                // is display only, so the chose() payload stays the real name
                text: plate.modelData.toUpperCase()
                color: plate.on ? Tokens.inkOnBone : Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: 11
                font.weight: Font.Medium
                font.letterSpacing: Tokens.trackLabel
            }
            HoverHandler { id: plateHover; cursorShape: plate.live ? Qt.PointingHandCursor : Qt.ArrowCursor }
            TapHandler {
                id: plateTap
                enabled: plate.live
                onTapped: row.chose(plate.modelData)
            }
        }
    }
}
