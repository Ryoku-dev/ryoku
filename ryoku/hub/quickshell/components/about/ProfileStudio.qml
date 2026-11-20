import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import ".."

Column {
    id: root
    spacing: LithicTokens.space.s4
    property var hub: null
    readonly property var available: Progression.stickers
    readonly property var ownedBackgrounds: Progression.cosmetics.filter(c => c.type === "background")
    readonly property var ownedFonts: Progression.cosmetics.filter(c => c.type === "font")
    readonly property var ownedStickers: Progression.cosmetics.filter(c => c.type === "sticker")
    readonly property var palettes: Progression.cosmetics.filter(c => c.type === "palette")
    readonly property var placed: Progression.canvas.stickers || []

    ImageChooser {
        id: backgroundPicker
        parent: root.hub ? root.hub.overlayItem : root
        overlayParent: root.hub ? root.hub.overlayItem : null
        title: I18n.tr("Choose a profile background")
        onAccepted: Progression.setCustomImage(backgroundPicker.selectedFile)
    }

    Label { text: I18n.tr("Points Shop"); typeRole: "title" }
    Row {
        spacing: LithicTokens.space.s3
        Label {
            anchors.verticalCenter: parent.verticalCenter
            text: Progression.balance + " ◆"
            typeRole: "title"
            ink: Lithic.gold
        }
        Plaque {
            small: true
            kind: Progression.editMode ? "gold" : "line"
            text: Progression.editMode ? I18n.tr("Stop decorating") : I18n.tr("Decorate Home")
            onClicked: {
                Progression.editMode = !Progression.editMode;
                if (Progression.editMode && root.hub)
                    root.hub.goHome();
            }
        }
    }

    Label { text: I18n.tr("Background collection"); typeRole: "section" }
    Label {
        width: parent.width
        text: I18n.tr("Backdrops cover the whole profile canvas. The original Lithic ground is always free.")
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
    Row {
        spacing: LithicTokens.space.s2
        Plaque {
            kind: Progression.canvas.background === "" && !Progression.canvas.image ? "gold" : "line"
            small: true
            text: I18n.tr("Original Lithic background")
            onClicked: Progression.setBackground("")
        }
        Plaque {
            kind: Progression.canvas.image ? "gold" : "line"
            small: true
            text: I18n.tr("Use my picture")
            onClicked: backgroundPicker.open()
        }
    }
    Label {
        width: parent.width
        visible: !!Progression.canvas.image
        text: I18n.tr("Personal picture selected. Keep the original image file in place so it remains available.")
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
    }
    Grid {
        id: backdropGallery
        width: parent.width
        columns: 3
        columnSpacing: LithicTokens.space.s3
        rowSpacing: LithicTokens.space.s3

        Repeater {
            model: root.ownedBackgrounds
            delegate: Item {
                id: backdrop
                required property var modelData
                required property int index
                readonly property bool owned: Progression.data.owned[modelData.id] === true
                readonly property bool active: Progression.canvas.background === modelData.id
                readonly property color tint: modelData.id === "bg-ember" ||
                    modelData.id === "bg-archive" ? Lithic.gold : Lithic.lapis
                readonly property color ground: Lithic.mix(Lithic.ground, tint, 0.07)
                width: (backdropGallery.width - backdropGallery.columnSpacing * 2) / 3
                height: 217

                CutRect {
                    anchors.fill: parent
                    cut: LithicTokens.cut.small
                    fill: backdrop.ground
                    strokeColor: backdrop.active ? Lithic.gold : backdrop.tint
                    strokeWidth: backdrop.active ? 2 : 1
                }
                Item {
                    x: 2
                    y: 2
                    width: parent.width - 4
                    height: 133
                    clip: true

                    Image {
                        anchors.fill: parent
                        source: Qt.resolvedUrl("../../assets/" + backdrop.modelData.art)
                        sourceSize.width: Math.max(1, Math.round(width * 2))
                        sourceSize.height: Math.max(1, Math.round(height * 2))
                        asynchronous: true
                        fillMode: Image.PreserveAspectCrop
                        smooth: true
                    }
                }
                Label {
                    x: 11
                    y: 140
                    width: parent.width - 22
                    height: 33
                    text: I18n.tr(backdrop.modelData.name)
                    typeRole: "detail"
                    wrapMode: Text.Wrap
                    elide: Text.ElideRight
                    maximumLineCount: 2
                }
                Plaque {
                    x: 11
                    y: 178
                    small: true
                    kind: backdrop.active ? "gold" : "line"
                    text: backdrop.active ? I18n.tr("Equipped") :
                          backdrop.owned ? I18n.tr("Use canvas") :
                          I18n.tr("Unlock") + " · " + backdrop.modelData.price + " ◆"
                    enabled: !backdrop.active &&
                        (backdrop.owned || Progression.balance >= backdrop.modelData.price)
                    onClicked: {
                        if (backdrop.owned) Progression.setBackground(backdrop.modelData.id);
                        else Progression.purchase(backdrop.modelData.id);
                    }
                }
            }
        }
    }

    Item {
        width: parent.width
        height: visible ? 195 : 0
        visible: Progression.completionSceneUnlocked

        CutRect {
            anchors.fill: parent
            cut: LithicTokens.cut.large
            fill: "#0b0b15"
            strokeColor: Progression.canvas.background === Progression.completionScene.id ?
                Lithic.gold : Lithic.lapis
            strokeWidth: 2
        }
        BlackPrism {
            x: 8
            y: 8
            width: parent.width * 0.45
            height: parent.height - 16
            live: false
        }
        Column {
            x: parent.width * 0.48
            y: 16
            width: parent.width * 0.49
            spacing: LithicTokens.space.s2
            Label { text: "MASTER COLLECTION / 032"; typeRole: "monoSm"; ink: Lithic.gold }
            Label { text: I18n.tr("BLACK PRISM"); typeRole: "section"; ink: Lithic.fg }
            Label {
                width: parent.width
                text: I18n.tr("Exclusive animated backdrop. Earned by collecting the complete shop.")
                typeRole: "detail"
                ink: Lithic.fg2
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                small: true
                kind: Progression.canvas.background === Progression.completionScene.id ? "gold" : "line"
                text: Progression.canvas.background === Progression.completionScene.id ?
                    I18n.tr("Equipped") : I18n.tr("Equip reward")
                enabled: Progression.canvas.background !== Progression.completionScene.id
                onClicked: Progression.setBackground(Progression.completionScene.id)
            }
        }
    }

    Label { text: I18n.tr("Typography"); typeRole: "section" }
    Label {
        text: I18n.tr("Apply fonts in Customise → Words")
        typeRole: "detail"
        inkRole: "fg2"
    }
    Grid {
        id: fontGallery
        width: parent.width
        columns: 3
        columnSpacing: LithicTokens.space.s3
        rowSpacing: LithicTokens.space.s3
        Repeater {
            model: root.ownedFonts
            delegate: Item {
                id: fontTile
                required property var modelData
                readonly property bool owned: Progression.data.owned[modelData.id] === true
                width: (fontGallery.width - fontGallery.columnSpacing * 2) / 3
                height: 162

                CutRect {
                    anchors.fill: parent
                    cut: LithicTokens.cut.small
                    fill: Lithic.mix(Lithic.ground, Lithic.fg, 0.055)
                    strokeColor: fontTile.owned ? Lithic.gold : Lithic.vein
                    strokeWidth: 1
                }
                Column {
                    anchors.fill: parent
                    anchors.margins: 11
                    spacing: 5
                    Label {
                        text: "Aa"
                        typeRole: "title"
                        font.family: fontTile.modelData.family
                        font.pixelSize: 44
                        ink: Lithic.gold
                    }
                    Label {
                        text: I18n.tr(fontTile.modelData.name)
                        typeRole: "sentence"
                    }
                    Label {
                        text: fontTile.owned ? I18n.tr("COLLECTED") : fontTile.modelData.price + " ◆"
                        typeRole: "monoSm"
                        inkRole: "fg2"
                    }
                    Plaque {
                        small: true
                        kind: fontTile.owned ? "gold" : "line"
                        text: fontTile.owned ? I18n.tr("Owned") : I18n.tr("Unlock")
                        enabled: !fontTile.owned && Progression.balance >= fontTile.modelData.price
                        onClicked: Progression.purchase(fontTile.modelData.id)
                    }
                }
            }
        }
    }
    Label { text: I18n.tr("Collectible stickers"); typeRole: "section" }
    Label {
        width: parent.width
        text: I18n.tr("Claimed badges become stickers. Decorative stickers can be purchased. Place up to 24 on your profile and drag them while decorating.")
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
    Grid {
        id: stickerGallery
        width: parent.width
        columns: 4
        columnSpacing: LithicTokens.space.s2
        rowSpacing: LithicTokens.space.s2

        Repeater {
            model: root.ownedStickers
            delegate: Item {
                id: stickerTile
                required property var modelData
                readonly property bool owned: Progression.data.owned[modelData.id] === true
                width: (stickerGallery.width - stickerGallery.columnSpacing * 3) / 4
                height: 195

                CutRect {
                    anchors.fill: parent
                    cut: LithicTokens.cut.small
                    fill: Lithic.mix(Lithic.ground, stickerTile.modelData.tint, 0.06)
                    strokeColor: stickerTile.owned ? Lithic.gold : Lithic.mix(Lithic.ground, stickerTile.modelData.tint, 0.5)
                    strokeWidth: 1
                }
                StickerArtwork {
                    anchors.horizontalCenter: parent.horizontalCenter
                    y: 4
                    width: 112
                    height: 112
                    artwork: stickerTile.modelData.art
                    fallbackGlyph: stickerTile.modelData.glyph
                    tint: stickerTile.modelData.tint
                }
                Column {
                    x: 10
                    y: 103
                    width: parent.width - 20
                    spacing: 6
                    Label {
                        width: parent.width
                        text: I18n.tr(stickerTile.modelData.name)
                        typeRole: "detail"
                        elide: Text.ElideRight
                    }
                    Label {
                        text: stickerTile.owned ? I18n.tr("COLLECTED") : stickerTile.modelData.price + " ◆"
                        typeRole: "monoSm"
                        ink: stickerTile.modelData.tint
                    }
                    Plaque {
                        small: true
                        kind: stickerTile.owned ? "gold" : "line"
                        text: stickerTile.owned ? I18n.tr("Place") : I18n.tr("Unlock")
                        enabled: stickerTile.owned ? Progression.canvas.stickers.length < 24 :
                            Progression.balance >= stickerTile.modelData.price
                        onClicked: {
                            if (stickerTile.owned) Progression.addSticker(stickerTile.modelData.id);
                            else Progression.purchase(stickerTile.modelData.id);
                        }
                    }
                }
            }
        }
    }
    Label { text: I18n.tr("Your earned badge stickers"); typeRole: "sentence" }
    Label {
        width: parent.width
        visible: root.available.filter(a => a.type === "badge").length === 0
        text: I18n.tr("The archive is waiting. Claim an achievement to add its badge here.")
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
    }
    Grid {
        id: badgeGallery
        width: parent.width
        columns: 4
        columnSpacing: LithicTokens.space.s2
        rowSpacing: LithicTokens.space.s2

        Repeater {
            model: root.available.filter(a => a.type === "badge")
            delegate: Item {
                id: earnedSticker
                required property var modelData
                width: (badgeGallery.width - badgeGallery.columnSpacing * 3) / 4
                height: 182

                CutRect {
                    anchors.fill: parent
                    cut: LithicTokens.cut.small
                    fill: Lithic.mix(Lithic.ground, Lithic.gold, 0.08)
                    strokeColor: Lithic.mix(Lithic.ground, Lithic.gold, 0.42)
                    strokeWidth: 1
                }
                ArchiveSeal {
                    badgeId: earnedSticker.modelData.id
                    anchors.horizontalCenter: parent.horizontalCenter
                    y: 4
                    width: 89
                    height: 89
                    title: earnedSticker.modelData.name
                    sigil: earnedSticker.modelData.sigil
                    rarity: earnedSticker.modelData.rarity
                    category: "ARCHIVE"
                }
                Column {
                    x: 9
                    y: 96
                    width: parent.width - 18
                    spacing: 6
                    Label {
                        width: parent.width
                        text: I18n.tr(earnedSticker.modelData.name)
                        typeRole: "detail"
                        elide: Text.ElideRight
                    }
                    Plaque {
                        small: true
                        kind: "line"
                        text: I18n.tr("Place seal")
                        enabled: Progression.canvas.stickers.length < 24
                        onClicked: Progression.addSticker(earnedSticker.modelData.id)
                    }
                }
            }
        }
    }

    Label { text: I18n.tr("Card palettes"); typeRole: "section" }
    Repeater {
        model: root.palettes
        delegate: SettingRow {
            id: paletteRow
            required property var modelData
            readonly property bool owned: Progression.data.owned[modelData.id] === true
            readonly property bool equipped: Progression.data.active === modelData.id
            width: root.width
            title: I18n.tr(modelData.name)
            detail: owned ? I18n.tr("Collected") : modelData.price + " ◆"
            Plaque {
                kind: paletteRow.equipped ? "gold" : "line"
                small: true
                text: paletteRow.equipped ? I18n.tr("Equipped") : paletteRow.owned ? I18n.tr("Equip") : I18n.tr("Buy")
                enabled: !paletteRow.equipped && (paletteRow.owned || Progression.balance >= paletteRow.modelData.price)
                onClicked: {
                    if (paletteRow.owned) Progression.equip(paletteRow.modelData.id);
                    else Progression.purchase(paletteRow.modelData.id);
                }
            }
        }
    }
    Plaque {
        kind: "quiet"
        small: true
        text: I18n.tr("Original card colours")
        enabled: Progression.data.active !== ""
        onClicked: Progression.equip("")
    }
}
