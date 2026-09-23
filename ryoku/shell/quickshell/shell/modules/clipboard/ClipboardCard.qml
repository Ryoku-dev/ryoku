pragma ComponentBehavior: Bound

import QtQuick
import shell.services
import "../../components"

/**
 * One clipboard entry drawn for a ClipboardSurface pane. The kind decides the
 * silhouette: an "image" entry is a tall preview card whose daemon thumbnail
 * fills the frame cropped to aspect, a "text" entry is a long horizontal card
 * with a multi-line preview, and anything else is a compact long card showing
 * its mime and human-readable size. The whole card copies on click; a star in
 * the top-right corner (shown on hover, or always while starred) toggles the
 * entry between the history and Starred panes without copying, and a faint trash
 * affordance removes it.
 */
Rectangle {
    id: card

    property var entry: null
    property real s: 1

    signal copyRequested()
    signal starRequested()
    signal deleteRequested()

    readonly property string kind: card.entry ? String(card.entry.kind || "") : ""
    readonly property bool isImage: card.kind === "image"
    readonly property bool isText: card.kind === "text"
    readonly property bool starred: card.entry ? card.entry.starred === true : false
    readonly property real pad: Theme.paddingMd * card.s

    // Ink for the corner controls: light over an image (they float on a scrim),
    // the muted on-surface tone over a text/binary tile.
    readonly property color controlInk: card.isImage ? Theme.onSurface : Theme.onSurfaceVariant

    // Reference format_size: bytes below 1 KiB, one decimal KB/MB above.
    function humanSize(bytes) {
        if (bytes < 1024)
            return bytes + " B";
        if (bytes < 1048576)
            return (bytes / 1024).toFixed(1) + " KB";
        return (bytes / 1048576).toFixed(1) + " MB";
    }

    implicitHeight: card.isImage
        ? Math.round(150 * card.s)
        : Math.round(textCol.implicitHeight + card.pad * 2)

    radius: Math.round(14 * card.s)
    // Rounded corners must clip the thumbnail too, so image cards render through
    // a layer; text tiles keep their content inside and need no mask.
    layer.enabled: card.isImage
    layer.smooth: true
    color: hover.hovered ? Theme.surfaceContainerHigh : Theme.surfaceContainer
    border.width: Theme.borderWidth
    border.color: card.starred ? Theme.primary : Theme.outlineVariant

    Behavior on color { ColorAnimation { duration: Motion.rowFade; easing.type: Motion.easeType; easing.bezierCurve: Motion.easeCurve } }
    Behavior on border.color { ColorAnimation { duration: Motion.rowFade; easing.type: Motion.easeType; easing.bezierCurve: Motion.easeCurve } }

    // Whole-card hover, passive so it stays true while the pointer is over the
    // corner controls (a child MouseArea would otherwise steal it and flicker).
    HoverHandler { id: hover }

    // --- image preview: the persisted thumbnail fills the frame, cropped ------
    Image {
        anchors.fill: parent
        anchors.margins: Theme.borderWidth
        visible: card.isImage
        source: card.isImage && card.entry && card.entry.thumb ? "file://" + card.entry.thumb : ""
        fillMode: Image.PreserveAspectCrop
        asynchronous: true
        cache: false
    }

    // Top scrim so the corner controls read over the picture.
    Rectangle {
        anchors.top: parent.top
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.margins: Theme.borderWidth
        height: Math.round(48 * card.s)
        visible: card.isImage
        opacity: (hover.hovered || card.starred) ? 1 : 0
        gradient: Gradient {
            GradientStop { position: 0.0; color: Qt.rgba(0, 0, 0, 0.5) }
            GradientStop { position: 1.0; color: Qt.rgba(0, 0, 0, 0.0) }
        }
        Behavior on opacity { NumberAnimation { duration: Motion.rowFade; easing.type: Motion.easeStandard } }
    }

    // --- text / binary body ---------------------------------------------------
    Column {
        id: textCol
        visible: !card.isImage
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.margins: card.pad
        // Keep the multi-line preview clear of the corner star.
        anchors.rightMargin: card.pad + Math.round(24 * card.s)
        spacing: Math.round(4 * card.s)

        // Meta line: "#id" for text, the mime for binary.
        Text {
            width: parent.width
            text: card.isText
                ? ("#" + (card.entry ? String(card.entry.id) : ""))
                : (card.entry ? String(card.entry.mime || "") : "")
            color: Qt.rgba(Theme.onSurfaceVariant.r, Theme.onSurfaceVariant.g, Theme.onSurfaceVariant.b, 0.75)
            font.family: Theme.fontPrimary
            font.pixelSize: Math.round((Theme.fontSm - 3) * card.s)
            font.weight: Font.DemiBold
            elide: Text.ElideRight
        }

        // Text preview: a legible, multi-line block wrapping mid-word.
        Text {
            width: parent.width
            visible: card.isText
            text: card.entry ? String(card.entry.preview || "") : ""
            color: Theme.onSurface
            font.family: Theme.fontPrimary
            font.pixelSize: Math.round(Theme.fontSm * card.s)
            lineHeight: 1.2
            wrapMode: Text.WrapAtWordBoundaryOrAnywhere
            maximumLineCount: 3
            elide: Text.ElideRight
        }

        // Binary: the human-readable size under the mime.
        Text {
            width: parent.width
            visible: !card.isText
            text: card.humanSize(card.entry ? (card.entry.size || 0) : 0)
            color: Theme.onSurface
            font.family: Theme.fontPrimary
            font.pixelSize: Math.round(Theme.fontSm * card.s)
            font.weight: Font.DemiBold
            elide: Text.ElideRight
        }
    }

    // Body click copies the entry and dismisses; sits under the corner controls.
    MouseArea {
        id: body
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        onClicked: card.copyRequested()
    }

    // --- corner controls (top-right): delete then star -----------------------
    Row {
        anchors.top: parent.top
        anchors.right: parent.right
        anchors.topMargin: Math.round(6 * card.s)
        anchors.rightMargin: Math.round(6 * card.s)
        spacing: Math.round(2 * card.s)
        z: 3

        // Delete: subtle, hover-only, never competes with the star.
        Rectangle {
            width: Math.round(24 * card.s)
            height: width
            radius: width / 2
            visible: hover.hovered
            color: delArea.containsMouse
                ? Qt.rgba(Theme.error.r, Theme.error.g, Theme.error.b, 0.16)
                : "transparent"

            MaterialIcon {
                anchors.centerIn: parent
                text: "delete"
                fill: 0
                font.pixelSize: Math.round(Theme.iconSm * card.s * 0.9)
                color: delArea.containsMouse
                    ? Theme.error
                    : Qt.rgba(card.controlInk.r, card.controlInk.g, card.controlInk.b, 0.6)
            }

            MouseArea {
                id: delArea
                anchors.fill: parent
                hoverEnabled: true
                cursorShape: Qt.PointingHandCursor
                onClicked: card.deleteRequested()
            }
        }

        // Star: shown on hover or whenever the entry is starred. Filled + accent
        // while starred; clicking it toggles state without copying.
        Rectangle {
            width: Math.round(26 * card.s)
            height: width
            radius: width / 2
            visible: hover.hovered || card.starred
            color: card.starred
                ? Qt.rgba(Theme.primary.r, Theme.primary.g, Theme.primary.b, 0.20)
                : (starArea.containsMouse
                    ? Qt.rgba(card.controlInk.r, card.controlInk.g, card.controlInk.b, 0.14)
                    : (card.isImage ? Qt.rgba(0, 0, 0, 0.3) : "transparent"))

            MaterialIcon {
                anchors.centerIn: parent
                text: "star"
                fill: card.starred ? 1 : 0
                font.pixelSize: Math.round(Theme.iconSm * card.s)
                color: card.starred ? Theme.primary : card.controlInk
            }

            MouseArea {
                id: starArea
                anchors.fill: parent
                hoverEnabled: true
                cursorShape: Qt.PointingHandCursor
                onClicked: card.starRequested()
            }
        }
    }
}
