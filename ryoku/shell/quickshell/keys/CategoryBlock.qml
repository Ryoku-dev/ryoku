pragma ComponentBehavior: Bound
import QtQuick
import Ryoku.Ui.Singletons

// A category rendered as a spaced ledger: an optional tracked head (shown in
// search results, where several groups stack) and its rows with air between
// them.
Column {
    id: block

    property string name: ""
    property var binds: []
    property bool showHead: true
    // Kit search reveals every matching row's hint, not only the hovered one.
    property bool searching: false

    spacing: Tokens.s1

    Item {
        visible: block.showHead
        width: block.width
        height: block.showHead ? 30 : 0

        Row {
            id: label
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s2
            Rectangle {
                width: 4
                height: 4
                color: Tokens.ink
                anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr(block.name)
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fMicro
                font.weight: Font.Medium
                font.letterSpacing: Tokens.trackMark
                font.capitalization: Font.AllUppercase
            }
        }
        Rectangle {
            anchors.left: label.right
            anchors.leftMargin: Tokens.s3
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            height: 1
            color: Tokens.lineSoft
        }
    }

    Repeater {
        model: block.binds
        delegate: BindRow {
            id: bindRow
            required property var modelData
            required property int index
            property real entryProgress: Tokens.reduceMotion ? 1 : 0
            width: block.width
            keys: modelData.keys
            desc: modelData.desc
            hint: modelData.hint || ""
            taps: modelData.taps || 0
            whileHighlighting: modelData.whileHighlighting === true
            unhonored: modelData.unhonored || ""
            searching: block.searching
            opacity: entryProgress
            transform: Translate { x: (1 - bindRow.entryProgress) * 8 }

            SequentialAnimation {
                id: entryAnimation
                PauseAnimation { duration: Math.min(bindRow.index, 8) * 12 }
                NumberAnimation {
                    target: bindRow
                    property: "entryProgress"
                    from: 0
                    to: 1
                    duration: Tokens.durSmall
                    easing.type: Easing.OutCubic
                }
            }
            Component.onCompleted: if (!Tokens.reduceMotion) entryAnimation.start()
        }
    }
}
