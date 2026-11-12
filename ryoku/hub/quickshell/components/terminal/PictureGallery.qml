import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui as RyokuUi
import Ryoku.Ui.Singletons

// the pictures the greeting can show: Ryoku's marks, the shipped art, what
// RyoStore installed, and your own image (chosen or dropped here)
Column {
    id: gallery

    required property var host
    readonly property var store: gallery.host.store
    readonly property string source: gallery.store && gallery.store.ffModel ? String(gallery.store.ffModel.logo.source || "") : ""
    // a picture you chose yourself shows on the Your image tile, marked as the one in use
    readonly property bool ownImage: gallery.source.length > 0 && !gallery.host.readyArt.some(a => a.path === gallery.source)

    spacing: LithicTokens.space.s2

    function urlOf(path) {
        var s = String(path || "");
        if (s.charAt(0) === "~")
            s = Paths.home + s.slice(1);
        return "file://" + s;
    }

    component Tile: T.AbstractButton {
        id: tile

        property string path: ""
        property string glyph: ""
        property bool removable: false
        readonly property bool current: tile.path.length > 0 && tile.path === gallery.source
        readonly property real plateSize: 56

        signal removeRequested()

        implicitWidth: 96
        implicitHeight: LithicTokens.space.s2 * 3 + plateSize + caption.implicitHeight
        hoverEnabled: true
        focusPolicy: Qt.StrongFocus
        Accessible.role: Accessible.RadioButton
        Accessible.checked: tile.current
        Accessible.name: tile.text
        Keys.onReturnPressed: clicked()
        Keys.onEnterPressed: clicked()

        background: CutRect {
            id: plane
            fill: tile.current ? Lithic.sel
                : tile.down ? Lithic.pressRow(Lithic.hoverPlane(Lithic.slab))
                : tile.hovered ? Lithic.hoverPlane(Lithic.slab) : Lithic.slab
            strokeColor: tile.current ? Lithic.alpha(Lithic.vein, 0) : Lithic.vein
            strokeWidth: LithicTokens.size.hairline
            cut: LithicTokens.cut.small
            ChiselColorBehavior on fill {
                duration: tile.hovered ? LithicTokens.motion.hoverIn : LithicTokens.motion.hoverOut
            }

            FocusRing {
                x: -outset
                y: -outset
                width: plane.width + outset * 2
                height: plane.height + outset * 2
                cut: plane.cut
                keyboardFocusVisible: tile.visualFocus
            }
        }

        contentItem: Item {
            CutRect {
                id: plate
                x: (parent.width - width) / 2
                y: LithicTokens.space.s2 - tile.topPadding
                width: tile.plateSize
                height: tile.plateSize
                fill: tile.glyph.length > 0 ? Lithic.field : (Lithic.dark ? Lithic.field : Lithic.fg)
                cut: LithicTokens.cut.token

                Image {
                    anchors.fill: parent
                    anchors.margins: LithicTokens.space.s1
                    visible: tile.path.length > 0
                    source: tile.path.length > 0 ? gallery.urlOf(tile.path) : ""
                    fillMode: Image.PreserveAspectFit
                    sourceSize.width: 104
                    sourceSize.height: 104
                    asynchronous: true
                    smooth: true
                }
                Glyph {
                    anchors.centerIn: parent
                    visible: tile.glyph.length > 0
                    name: tile.glyph
                    size: LithicTokens.size.plaqueIcon
                    ink: Lithic.fg2
                }
            }

            Label {
                id: caption
                anchors.horizontalCenter: parent.horizontalCenter
                y: plate.y + plate.height + LithicTokens.space.s2
                width: parent.width - LithicTokens.space.s2 * 2
                horizontalAlignment: Text.AlignHCenter
                text: tile.text
                typeRole: "caption"
                ink: tile.current ? Lithic.onSel : Lithic.fg2
                elide: Text.ElideRight
            }

            IconPlaque {
                anchors.right: parent.right
                anchors.top: parent.top
                anchors.topMargin: -tile.topPadding
                anchors.rightMargin: -tile.rightPadding
                visible: tile.removable && (tile.hovered || tile.activeFocus || activeFocus)
                small: true
                kind: "quiet"
                glyph: "delete"
                accessibleName: I18n.tr("Remove %1").arg(tile.text)
                onClicked: tile.removeRequested()
            }
        }
    }

    DropArea {
        width: parent.width
        height: tiles.height
        onDropped: drop => {
            if (drop.hasUrls && drop.urls.length > 0)
                gallery.host.importImage(decodeURIComponent(String(drop.urls[0]).replace(/^file:\/\//, "")));
        }

        Flow {
            id: tiles
            width: parent.width
            spacing: LithicTokens.space.s2

            Tile {
                text: I18n.tr("Your image")
                path: gallery.ownImage ? gallery.source : ""
                glyph: gallery.ownImage ? "" : "add_photo_alternate"
                onClicked: imageDialog.open()
            }

            Repeater {
                model: gallery.host.readyArt
                delegate: Tile {
                    required property var modelData
                    text: I18n.tr(modelData.name)
                    path: modelData.path
                    removable: modelData.store === true
                    onClicked: gallery.host.pickArt(modelData.path)
                    onRemoveRequested: gallery.host.removeStoreArt(modelData)
                }
            }
        }
    }

    Label {
        width: parent.width
        text: I18n.tr("Pick a picture, or choose or drop your own: PNG, JPEG or WebP, and SVG becomes PNG. Square art looks best.")
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    Row {
        visible: gallery.host.pictureError.length > 0
        width: parent.width
        spacing: LithicTokens.space.s3

        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, parent.width - retry.width - LithicTokens.space.s3)
            text: gallery.host.pictureError
            typeRole: "detail"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            id: retry
            kind: "quiet"
            small: true
            glyph: "refresh"
            text: I18n.tr("Retry")
            onClicked: gallery.host.retryPicture()
        }
    }

    RyokuUi.FileDialogHost {
        id: imageDialog
        title: I18n.tr("Choose a picture for the greeting")
        nameFilters: [I18n.tr("Images") + " (*.png *.jpg *.jpeg *.webp *.svg)"]
        currentFolder: "file://" + (gallery.host.pictureFolder.length > 0 ? gallery.host.pictureFolder : Paths.picturesDir)
        onAccepted: gallery.host.importImage(decodeURIComponent(String(imageDialog.selectedFile).replace(/^file:\/\//, "")))
    }
}
