import QtQuick
import "Singletons"
import Ryoku.Ui.Singletons as Ui

// The rule yields all available width to the section name before collapsing,
// so translated labels stay readable in a narrow docked inspector.
Item {
    id: sec

    readonly property bool ryoSection: true

    property string label: ""
    property string gloss: ""

    width: parent ? parent.width : 0
    implicitHeight: sec.label.length > 0 ? 26 : 13

    Rectangle {
        id: tick
        visible: sec.label.length > 0
        anchors.verticalCenter: parent.verticalCenter
        width: 2
        height: 8
        color: Theme.ink
    }

    TextMetrics {
        id: glossMetrics
        text: Ui.UiPreferences.text(sec.gloss, "")
        font.family: Theme.fontJp
        font.pixelSize: 9
    }

    TextMetrics {
        id: labelMetrics
        text: sec.label.toUpperCase()
        font.family: Theme.font
        font.pixelSize: 9
        font.weight: Font.DemiBold
        font.letterSpacing: 1.6
    }

    Text {
        id: gl
        visible: Ui.UiPreferences.showJapaneseGloss && sec.gloss.length > 0 && sec.label.length > 0
        x: tick.x + tick.width + 7
        width: visible ? glossMetrics.advanceWidth : 0
        anchors.verticalCenter: parent.verticalCenter
        text: sec.gloss
        color: Theme.faint
        elide: Text.ElideRight
        maximumLineCount: 1
        font.family: Theme.fontJp
        font.pixelSize: 9
    }

    Text {
        id: cap
        visible: sec.label.length > 0
        x: gl.visible ? gl.x + gl.width + 7 : tick.x + tick.width + 7
        width: visible ? Math.min(labelMetrics.advanceWidth,
            Math.max(0, sec.width - x - 7)) : 0
        anchors.verticalCenter: parent.verticalCenter
        text: sec.label.toUpperCase()
        color: Qt.rgba(Theme.ink.r, Theme.ink.g, Theme.ink.b, 0.78)
        elide: Text.ElideRight
        maximumLineCount: 1
        font.family: Theme.font
        font.pixelSize: 9
        font.weight: Font.DemiBold
        font.letterSpacing: 1.6
    }

    Rectangle {
        x: sec.label.length > 0 ? cap.x + cap.width + 7 : 0
        width: Math.max(0, sec.width - x)
        anchors.verticalCenter: parent.verticalCenter
        height: 1
        color: Qt.rgba(Theme.ink.r, Theme.ink.g, Theme.ink.b, 0.4)
    }
}
