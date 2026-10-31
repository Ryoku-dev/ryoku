import QtQuick
import Ryoku.Ui.Singletons

// Anchored to the CardField's live currentRect, so it follows the selection as the scene moves.
Item {
    id: chrome

    required property PickerState state
    readonly property CardField field: state.field

    readonly property rect cardRect: chrome.field ? chrome.field.currentRect : Qt.rect(0, 0, 0, 0)
    // Half the card's skew and edge tilt: the same inner box and shear card.frag draws.
    readonly property point shear: chrome.field ? chrome.field.currentShear : Qt.point(0, 0)
    readonly property real sx: chrome.shear.x * 0.5
    readonly property real ty: chrome.shear.y * 0.5
    readonly property real innerW: Math.max(chrome.cardRect.width - 2 * Math.abs(chrome.sx), 2)
    readonly property real innerH: Math.max(chrome.cardRect.height - 2 * Math.abs(chrome.ty), 2)
    readonly property int row: chrome.field ? chrome.field.currentIndex : -1
    // view.get() is a call QML cannot track, so any model change re-runs the lookup.
    property int _rev: 0
    Connections {
        target: chrome.state.view
        function onModelReset() { chrome._rev++ }
        function onDataChanged() { chrome._rev++ }
        function onLayoutChanged() { chrome._rev++ }
        function onCountChanged() { chrome._rev++ }
        function onCollectionChanged() { chrome._rev++ }
    }
    readonly property var entry: {
        chrome._rev
        var v = chrome.state.view
        if (!v || chrome.row < 0 || chrome.row >= v.count || chrome.cardRect.width <= 0)
            return ({})
        return v.get(chrome.row)
    }
    readonly property int badges: chrome.entry.badges !== undefined ? chrome.entry.badges : 0

    readonly property string typeLabel: {
        switch (chrome.entry.type) {
        case "static": return I18n.tr("PIC")
        case "video": return I18n.tr("VID")
        case "we": return I18n.tr("WE")
        default: return ""
        }
    }

    visible: !!(chrome.field && chrome.field.flippedIndex < 0
        && chrome.cardRect.width > 4 && chrome.cardRect.height > 4
        && (chrome.entry.name || chrome.entry.key))

    Item {
        x: chrome.cardRect.x + (chrome.cardRect.width - chrome.innerW) * 0.5
        y: chrome.cardRect.y + (chrome.cardRect.height - chrome.innerH) * 0.5
        width: chrome.innerW
        height: chrome.innerH
        clip: true
        transform: Matrix4x4 {
            matrix: Qt.matrix4x4(1, -2 * chrome.sx / chrome.innerH, 0, chrome.sx,
                                 -2 * chrome.ty / chrome.innerW, 1, 0, chrome.ty,
                                 0, 0, 1, 0,
                                 0, 0, 0, 1)
        }

        Row {
            anchors.left: parent.left
            anchors.top: parent.top
            anchors.margins: 8 * Theme.scale
            spacing: 5 * Theme.scale

            Rectangle {
                visible: chrome.typeLabel.length > 0
                radius: Theme.radius
                color: Theme.withAlpha(Theme.background, 0.72)
                border.width: 1
                border.color: Theme.withAlpha(Theme.outline, 0.40)
                implicitWidth: typeText.implicitWidth + 12 * Theme.scale
                implicitHeight: typeText.implicitHeight + 6 * Theme.scale
                Text {
                    id: typeText
                    anchors.centerIn: parent
                    text: chrome.typeLabel
                    font.family: Theme.sans
                    font.weight: Font.DemiBold
                    font.pixelSize: Theme.fontFine
                    color: Theme.withAlpha(Theme.surfaceText, 0.92)
                    renderType: Text.NativeRendering
                }
            }

            Rectangle {
                visible: (chrome.badges & 1) !== 0
                radius: Theme.radius
                color: Theme.withAlpha(Theme.background, 0.72)
                border.width: 1
                border.color: Theme.withAlpha(Theme.outline, 0.40)
                implicitWidth: favGlyph.implicitWidth + 10 * Theme.scale
                implicitHeight: favGlyph.implicitHeight + 6 * Theme.scale
                Text {
                    id: favGlyph
                    anchors.centerIn: parent
                    text: "\u{f02d1}"
                    font.family: Theme.icon
                    font.pixelSize: Theme.fontFine
                    color: Theme.withAlpha(Theme.surfaceText, 0.92)
                    renderType: Text.NativeRendering
                }
            }
        }

        Rectangle {
            visible: chrome.state.collection !== "workshop"
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            implicitHeight: nameText.implicitHeight + 12 * Theme.scale
            color: Theme.withAlpha(Theme.background, 0.62)

            Rectangle {
                visible: (chrome.badges & 32) !== 0
                anchors.left: parent.left
                anchors.top: parent.top
                width: 3 * Theme.scale
                height: parent.height
                color: Theme.surfaceText
            }

            Text {
                id: nameText
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                anchors.leftMargin: 12 * Theme.scale
                anchors.rightMargin: 12 * Theme.scale
                text: chrome.entry.name || chrome.entry.key || ""
                elide: Text.ElideRight
                font.family: Theme.sans
                font.weight: Font.DemiBold
                font.pixelSize: Theme.fontBody
                color: Theme.surfaceText
                renderType: Text.NativeRendering
            }
        }
    }
}
