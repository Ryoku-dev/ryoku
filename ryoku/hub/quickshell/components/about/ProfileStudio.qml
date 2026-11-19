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

    Label { text: I18n.tr("PROFILE WORKSHOP"); typeRole: "mono"; ink: Lithic.gold }
    Label { text: I18n.tr("Make this space yours."); typeRole: "title" }
    Label {
        width: parent.width
        text: I18n.tr("Everything in your collection belongs to your Ryoku profile. Buy cosmetics with earned credits, then equip them or place them on Home.")
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
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
            onClicked: Progression.setCustomImage("")
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
                readonly property bool owned: Progression.data.owned[modelData.id] === true
                readonly property bool active: Progression.canvas.background === modelData.id
                readonly property color tint: modelData.id === "bg-blueprint" ? "#7395bc" :
                    modelData.id === "bg-ember" ? "#f1a17e" :
                    modelData.id === "bg-archive" ? "#cfba96" :
                    modelData.id === "bg-obsidian" ? "#a1bbc4" :
                    modelData.id === "bg-sutra" ? "#acd2c1" :
                    modelData.id === "bg-signal" ? "#76b8df" : "#a3aabc"
                readonly property color ground: modelData.id === "bg-blueprint" ? "#122437" :
                    modelData.id === "bg-ember" ? "#2c1c1c" :
                    modelData.id === "bg-archive" ? "#29241e" :
                    modelData.id === "bg-obsidian" ? "#11191e" :
                    modelData.id === "bg-sutra" ? "#1c2524" :
                    modelData.id === "bg-signal" ? "#13202b" : "#22242b"
                width: (backdropGallery.width - backdropGallery.columnSpacing * 2) / 3
                height: 208

                CutRect {
                    anchors.fill: parent
                    cut: LithicTokens.cut.small
                    fill: backdrop.ground
                    strokeColor: backdrop.active ? Lithic.gold : backdrop.tint
                    strokeWidth: backdrop.active ? 2 : 1
                }
                Item {
                    anchors.fill: parent
                    anchors.margins: 1
                    clip: true
                    Repeater {
                        model: 7
                        delegate: Rectangle {
                            x: index * (backdrop.width / 6)
                            y: 0
                            height: 126
                            width: 1
                            color: backdrop.tint
                            opacity: 0.13
                        }
                    }
                    Repeater {
                        model: 4
                        delegate: Rectangle {
                            y: index * 40 + 3
                            x: 0
                            width: backdrop.width
                            height: 1
                            color: backdrop.tint
                            opacity: 0.13
                        }
                    }
                    Glyph {
                        x: backdrop.width - 112
                        y: 12
                        name: backdrop.modelData.id === "bg-blueprint" ? "architecture" :
                            backdrop.modelData.id === "bg-ember" ? "flare" : "grid_4x4"
                        size: 92
                        ink: backdrop.tint
                        opacity: 0.25
                    }
                }
                Column {
                    anchors.fill: parent
                    anchors.margins: 12
                    spacing: 5
                    Label {
                        text: "CANVAS / 0" + (root.ownedBackgrounds.indexOf(backdrop.modelData) + 1)
                        typeRole: "monoSm"
                        ink: backdrop.tint
                    }
                    Item { width: 1; height: 72 }
                    Label {
                        width: parent.width
                        text: I18n.tr(backdrop.modelData.name)
                        typeRole: "sentence"
                        ink: "#f1f2f5"
                        elide: Text.ElideRight
                    }
                    Label {
                        text: backdrop.owned ? I18n.tr("COLLECTED") :
                              backdrop.modelData.price + " ◆"
                        typeRole: "monoSm"
                        ink: backdrop.tint
                    }
                    Plaque {
                        small: true
                        kind: backdrop.active ? "gold" : "line"
                        text: backdrop.active ? I18n.tr("Equipped") :
                              backdrop.owned ? I18n.tr("Use canvas") : I18n.tr("Unlock")
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
    }

    Label { text: I18n.tr("Typography"); typeRole: "section" }
    Plaque {
        small: true
        kind: "quiet"
        text: I18n.tr("Edit display name and tagline")
        onClicked: if (root.hub) root.hub.customiseHome("words")
    }
    Label {
        width: parent.width
        text: I18n.tr("Assign a font individually to the display name, hostname and tagline. The default Ryoku typography remains free.")
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
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
    Repeater {
        model: [
            { id: "name", title: "Display name" },
            { id: "hostname", title: "Hostname" },
            { id: "tagline", title: "Tagline" }
        ]
        delegate: Column {
            id: fontRow
            required property var modelData
            width: root.width
            spacing: LithicTokens.space.s2
            Label { text: I18n.tr(fontRow.modelData.title); typeRole: "sentence" }
            Row {
                spacing: LithicTokens.space.s2
                Plaque {
                    small: true
                    kind: !Progression.canvas.fonts[fontRow.modelData.id] ? "gold" : "line"
                    text: I18n.tr("Default")
                    onClicked: Progression.setFont(fontRow.modelData.id, "")
                }
                Repeater {
                    model: root.ownedFonts
                    delegate: Plaque {
                        id: fontChoice
                        required property var modelData
                        readonly property bool owned: Progression.data.owned[modelData.id] === true
                        small: true
                        kind: Progression.canvas.fonts[fontRow.modelData.id] === modelData.id ? "gold" : "line"
                        text: I18n.tr(modelData.name) + (owned ? "" : " · " + modelData.price + " ◆")
                        enabled: owned || Progression.balance >= modelData.price
                        onClicked: {
                            if (fontChoice.owned)
                                Progression.setFont(fontRow.modelData.id, modelData.id);
                            else
                                Progression.purchase(modelData.id);
                        }
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
                ArchiveSeal {
                    anchors.horizontalCenter: parent.horizontalCenter
                    y: 6
                    width: 93
                    height: 93
                    title: stickerTile.modelData.name
                    sigil: stickerTile.modelData.sigil || "力"
                    rarity: stickerTile.modelData.rarity || "common"
                    category: "RELIC"
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
                        enabled: Progression.canvas.stickers.length < 24 &&
                            (stickerTile.owned || Progression.balance >= stickerTile.modelData.price)
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

    Label { text: I18n.tr("Placed decorations"); typeRole: "section" }
    Label {
        width: parent.width
        visible: root.placed.length === 0
        text: I18n.tr("No stickers placed yet. Choose one above to start your layout.")
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }
    Repeater {
        model: root.placed
        delegate: SettingRow {
            id: placedRow
            required property var modelData
            readonly property var art: Progression.stickerArt(modelData.item)
            width: root.width
            title: art ? I18n.tr(art.name) : ""
            detail: I18n.tr("Size %1 · rotation %2°").replace("%1", Math.round(modelData.size)).replace("%2", Math.round(modelData.rotation))
            Row {
                spacing: 6
                Plaque {
                    small: true
                    kind: "line"
                    text: "−"
                    onClicked: Progression.moveSticker(placedRow.modelData.slot,
                        placedRow.modelData.x, placedRow.modelData.y, placedRow.modelData.size - 12,
                        placedRow.modelData.rotation)
                }
                Plaque {
                    small: true
                    kind: "line"
                    text: "+"
                    onClicked: Progression.moveSticker(placedRow.modelData.slot,
                        placedRow.modelData.x, placedRow.modelData.y, placedRow.modelData.size + 12,
                        placedRow.modelData.rotation)
                }
                Plaque {
                    small: true
                    kind: "line"
                    text: "↻"
                    onClicked: Progression.moveSticker(placedRow.modelData.slot,
                        placedRow.modelData.x, placedRow.modelData.y, placedRow.modelData.size,
                        placedRow.modelData.rotation + 15)
                }
                Plaque {
                    small: true
                    kind: "line"
                    text: I18n.tr("Remove")
                    onClicked: Progression.removeSticker(placedRow.modelData.slot)
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
