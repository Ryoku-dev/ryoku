import QtQuick
import QtQuick.Effects

// Callers parent content into mastheadArea, indexArea and readingArea.
Item {
    id: sheet

    property real reveal: 1
    property bool showMasthead: true
    // Drops toward 0 so the picker scene shows through a design section.
    property real pageOpacity: 0.965
    property real scrimAlpha: 0.68

    readonly property alias mastheadArea: mastheadHost
    readonly property alias indexArea: indexHost
    readonly property alias readingArea: readingHost

    signal dismissed()

    anchors.fill: parent

    readonly property real _pw: Math.max(Theme.folioSheetMinWidth,
        Math.min(Theme.folioSheetWidth * Theme.scale, width - Theme.folioSheetMargin * Theme.scale))
    readonly property real _ph: Math.max(Theme.folioSheetMinHeight,
        Math.min(Theme.folioSheetHeight * Theme.scale, height - Theme.folioSheetMargin * Theme.scale))
    readonly property real _mastheadH: sheet.showMasthead ? Theme.folioMastheadHeight * Math.max(1, Theme.scale) : 0
    readonly property real _indexW: Theme.folioIndexWidth * Math.max(0.9, Theme.scale)

    Scrim {
        anchors.fill: parent
        alpha: sheet.scrimAlpha
        reveal: sheet.reveal
        onDismissed: sheet.dismissed()
    }

    Rectangle {
        id: panel
        anchors.centerIn: parent
        width: sheet._pw
        height: sheet._ph
        radius: Theme.radius
        color: "transparent"
        border.width: 1
        border.color: Theme.withAlpha(Theme.outline, 0.58)
        opacity: sheet.reveal
        clip: true

        Rectangle {
            anchors.top: parent.top
            anchors.left: parent.left
            anchors.right: parent.right
            height: sheet._mastheadH
            visible: sheet.showMasthead
            topLeftRadius: Theme.radius
            topRightRadius: Theme.radius
            color: Theme.withAlpha(Theme.surface, 0.99)
        }
        Rectangle {
            anchors.top: mastheadHost.bottom
            anchors.left: parent.left
            anchors.bottom: parent.bottom
            width: sheet._indexW
            bottomLeftRadius: Theme.radius
            color: Theme.withAlpha(Theme.surface, 0.99)
        }
        Item {
            id: mastheadHost
            anchors.top: parent.top
            anchors.left: parent.left
            anchors.right: parent.right
            height: sheet._mastheadH
            visible: sheet.showMasthead
        }

        Item {
            id: indexHost
            anchors.top: mastheadHost.bottom
            anchors.left: parent.left
            anchors.bottom: parent.bottom
            width: sheet._indexW
        }

        Item {
            id: page
            anchors.top: mastheadHost.bottom
            anchors.left: indexHost.right
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            clip: true

            Rectangle {
                anchors.fill: parent
                bottomRightRadius: Theme.radius
                color: Theme.withAlpha(Theme.surface, sheet.pageOpacity)
            }
            Item {
                id: readingHost
                anchors.fill: parent
            }
        }
    }
}
