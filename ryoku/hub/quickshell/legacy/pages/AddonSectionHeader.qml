pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui
import Ryoku.Ui.Singletons

Item {
    id: header

    property string title: ""
    property string countText: ""
    property string actionText: ""
    property bool refreshEnabled: true

    signal actionRequested()
    signal refreshRequested()

    height: Tokens.ctlH

    Row {
        id: labelRow
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
            text: header.title
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: Tokens.fMicro
            font.weight: Font.Medium
            font.letterSpacing: Tokens.trackMark
        }
    }

    Row {
        id: actions
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        spacing: Tokens.s2
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: header.countText
            color: Tokens.inkFaint
            font.family: Tokens.mono
            font.pixelSize: Tokens.fTiny
        }
        Btn {
            visible: header.actionText !== ""
            compact: true
            text: header.actionText
            onAct: header.actionRequested()
        }
        IconBtn {
            visible: header.refreshEnabled
            glyph: "\u21bb"
            onAct: header.refreshRequested()
        }
    }

    Rectangle {
        anchors.left: labelRow.right
        anchors.right: actions.left
        anchors.leftMargin: Tokens.s3
        anchors.rightMargin: Tokens.s3
        anchors.verticalCenter: parent.verticalCenter
        height: 1
        color: Tokens.lineSoft
    }
}
