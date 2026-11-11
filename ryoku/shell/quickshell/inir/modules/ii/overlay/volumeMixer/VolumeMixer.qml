pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import inir.modules.common
import inir.modules.iris.stage
import inir.modules.ii.overlay
import inir.services

StyledOverlayWidget {
    id: root
    title: Translation.tr("Volume mixer")
    minimumWidth: 390
    minimumHeight: 520
    property int page: 0

    contentItem: OverlayBackground {
        radius: root.contentRadius
        ColumnLayout {
            anchors.fill: parent
            anchors.margins: 8
            spacing: 8
            OverlaySegments {
                Layout.fillWidth: true
                segments: [
                    { label: Translation.tr("Output"), glyph: "media_output" },
                    { label: Translation.tr("Input"), glyph: "mic" },
                    { label: Translation.tr("Music"), glyph: "music_note" }
                ]
                currentIndex: root.page
                onActivated: index => root.page = index
            }
            Flickable {
                Layout.fillWidth: true
                Layout.fillHeight: true
                contentHeight: card.implicitHeight
                boundsBehavior: Flickable.StopAtBounds
                clip: true
                IrisCardContent {
                    id: card
                    width: parent.width
                    kind: root.page === 0 ? "sound" : root.page === 1 ? "mic" : "media"
                    contentActive: root.visible
                }
            }
        }
    }
}
