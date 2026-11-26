import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// Somewhere to start
Row {
    id: row

    property bool live: false
    property string query: ""
    signal picked(string text)

    readonly property real s: Theme.scale
    spacing: 6 * row.s

    readonly property var ideas: row.live
        ? [I18n.tr("rain"), I18n.tr("lofi"), I18n.tr("anime"), I18n.tr("ocean"), I18n.tr("neon city"),
           I18n.tr("space"), I18n.tr("pixel art"), I18n.tr("forest")]
        : [I18n.tr("minimal"), I18n.tr("mountains"), I18n.tr("city at night"), I18n.tr("space"),
           I18n.tr("abstract"), I18n.tr("anime"), I18n.tr("forest"), I18n.tr("cyberpunk")]

    Text {
        anchors.verticalCenter: parent.verticalCenter
        rightPadding: 4 * row.s
        text: I18n.tr("Try")
        font.family: Lithic.font.sans
        font.pixelSize: Theme.fs(12.5)
        color: Lithic.fg3
        renderType: Text.NativeRendering
    }
    Repeater {
        model: row.ideas
        CutRect {
            id: idea
            required property string modelData
            readonly property bool on: row.query.toLowerCase() === modelData.toLowerCase()
            readonly property bool hovered: area.containsMouse
            height: 30 * row.s
            width: label.implicitWidth + 22 * row.s
            cut: 4 * row.s
            fill: idea.on ? Lithic.container : Lithic.alpha(Lithic.fg, idea.hovered ? 0.1 : 0)
            strokeWidth: idea.on ? 0 : 1
            strokeColor: Lithic.alpha(Lithic.fg, idea.hovered ? 0.2 : 0.12)
            Behavior on fill { ColorAnimation { duration: Theme.fast } }
            Text {
                id: label
                anchors.centerIn: parent
                text: idea.modelData
                font.family: Lithic.font.sans
                font.pixelSize: Theme.fs(12.5)
                color: idea.on ? Lithic.containerInk : idea.hovered ? Lithic.fg : Lithic.fg2
                renderType: Text.NativeRendering
            }
            MouseArea {
                id: area
                anchors.fill: parent
                hoverEnabled: true
                cursorShape: Qt.PointingHandCursor
                onClicked: row.picked(idea.modelData)
            }
        }
    }
}
