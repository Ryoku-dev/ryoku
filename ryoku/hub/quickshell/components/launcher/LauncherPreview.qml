import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

// the launcher in use drawn by its own preview, live with your settings; dragging its picture reframes it
Item {
    id: root

    required property var shelf
    // what the preview reads: the launcher's own file, plus the desktop's weather units
    required property var settings

    // the tile picked in the gallery, else the launcher in use
    property string cardId: ""
    readonly property var card: root.shelf ? (root.shelf.card(root.cardId) || root.shelf.activeCard) : null
    readonly property bool hasArt: !!root.card && String(root.card.art || "").length > 0
    readonly property url previewUrl: root.card && (root.card.kind !== "shipped" || !root.shelf.catalogMissing)
        ? root.card.previewUrl : ""

    signal edit(string key, var value)

    implicitHeight: 240

    CutRect {
        anchors.fill: parent
        cut: LithicTokens.cut.small
        fill: Lithic.field
        strokeColor: Lithic.vein
        strokeWidth: LithicTokens.size.hairline
    }

    Item {
        id: stage
        anchors.fill: parent
        anchors.margins: LithicTokens.space.s2
        clip: true

        Loader {
            id: preview
            anchors.centerIn: parent
            width: item ? item.implicitWidth : 0
            height: item ? item.implicitHeight : 0
            scale: item && item.implicitWidth > 0 && item.implicitHeight > 0
                ? Math.min(0.72, (stage.width - LithicTokens.space.s4) / item.implicitWidth, stage.height / item.implicitHeight)
                : 1
            source: root.previewUrl
            onLoaded: {
                if ("settings" in item)
                    item.settings = Qt.binding(() => root.settings);
                if (item.editRequested)
                    item.editRequested.connect((key, value) => root.edit(key, value));
            }
        }

        // a RyoStore launcher with no preview of its own shows its store art
        Image {
            anchors.fill: parent
            visible: preview.status !== Loader.Ready && root.hasArt
            source: visible ? root.card.art : ""
            asynchronous: true
            fillMode: Image.PreserveAspectFit
        }

        Label {
            anchors.centerIn: parent
            width: parent.width - LithicTokens.space.s6
            visible: preview.status !== Loader.Ready && !root.hasArt
            horizontalAlignment: Text.AlignHCenter
            text: root.card ? I18n.tr("%1 has no preview here.").arg(root.card.name) : ""
            typeRole: "detail"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }
}
