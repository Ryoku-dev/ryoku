import QtQuick
import Ryoku.Ui.Singletons

// One AI usage limit window drawn as a labeled meter.
Column {
    id: limitBar

    property var root
    property string label: ""
    property int pct: 0
    property int resetTs: 0
    property bool behind: false

    width: parent ? parent.width : 0
    spacing: 4

    Item {
        width: parent.width
        height: 14
        UiText {
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr(limitBar.label)
            color: limitBar.root.sumiHi
            font.family: limitBar.root.mono; font.pixelSize: 11
        }
        UiText {
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr("%1% used").arg(limitBar.pct)
                  + (limitBar.resetTs > 0 ? " · " + limitBar.root.aiFmtReset(limitBar.resetTs) : "")
            color: limitBar.root.sumi
            font.family: limitBar.root.mono; font.pixelSize: 11
        }
    }

    Rectangle {
        width: parent.width; height: 6
        radius: Math.min(Tokens.radius, height / 2)
        color: Qt.rgba(limitBar.root.seal.r, limitBar.root.seal.g, limitBar.root.seal.b, 0.15)
        Rectangle {
            width: parent.width * Math.max(0, Math.min(100, limitBar.pct)) / 100
            height: parent.height
            radius: Math.min(Tokens.radius, height / 2)
            color: limitBar.behind ? limitBar.root.sealRaw : limitBar.root.seal
            Behavior on width { NumberAnimation { duration: 300 } }
        }
    }
}
