import QtQuick
import Ryoku.Ui.Singletons

Rectangle {
    id: hero

    property string sourceLabel: ""
    property int count: 0
    property int page: 1
    property string artSource: ""
    property real reveal: 1

    implicitHeight: 132 * Theme.scale
    clip: true
    color: Theme.withAlpha(Theme.background, 0.72)
    border.width: 1
    border.color: Theme.withAlpha(Theme.outline, 0.42)

    readonly property real _bigScale: Math.max(0.9, Math.min(1.08, Theme.scale))
    readonly property bool artFailed: art.status === Image.Error

    // The current result's art washes faintly behind the readout.
    Image {
        id: art
        anchors.fill: parent
        visible: hero.artSource.length > 0
        source: hero.artSource
        fillMode: Image.PreserveAspectCrop
        sourceSize: Qt.size(Math.max(1, Math.ceil(width)), Math.max(1, Math.ceil(height)))
        asynchronous: true
        cache: false
        opacity: 0.1 * hero.reveal
    }

    Column {
        id: idCol
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.leftMargin: 24 * Theme.scale
        anchors.rightMargin: 24 * Theme.scale
        anchors.bottomMargin: 20 * Theme.scale
        spacing: 6 * Theme.scale

        Text {
            width: parent.width
            text: hero.sourceLabel
            elide: Text.ElideRight
            lineHeight: 0.94
            font.family: Theme.display
            font.pixelSize: Theme.fontStudio
            color: Theme.withAlpha(Theme.surfaceText, 0.99 * hero.reveal)
            renderType: Text.NativeRendering
        }
        Text {
            width: parent.width
            text: I18n.tr("Open a result to check its full resolution, source, and download options.")
            wrapMode: Text.WordWrap
            maximumLineCount: 2
            elide: Text.ElideRight
            font.family: Theme.sans
            font.weight: Font.Normal
            font.pixelSize: Theme.fontBase
            color: Theme.withAlpha(Theme.surfaceText, 0.64 * hero.reveal)
            renderType: Text.NativeRendering
        }
    }

    Column {
        id: countCol
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.rightMargin: 24 * Theme.scale
        anchors.topMargin: 20 * Theme.scale
        spacing: 4 * Theme.scale

        Text {
            anchors.right: parent.right
            text: hero.count
            lineHeight: 0.86
            font.family: Theme.display
            font.pixelSize: 34 * hero._bigScale
            color: Theme.withAlpha(Theme.surfaceText, 0.98 * hero.reveal)
            renderType: Text.NativeRendering
        }
        Text {
            anchors.right: parent.right
            text: I18n.tr("Results / page %1").arg(hero.page).toUpperCase()
            font.family: Theme.sans
            font.weight: Font.DemiBold
            font.pixelSize: Theme.fontFine
            font.letterSpacing: 1.4
            color: Theme.withAlpha(Theme.surfaceText, 0.6 * hero.reveal)
            renderType: Text.NativeRendering
        }
    }
}
