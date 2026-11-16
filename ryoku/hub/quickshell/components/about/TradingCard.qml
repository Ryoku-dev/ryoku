import QtQuick
import QtQuick.Effects
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import ".."
import "../badge"
import "design"
import "../../registry/rarity.js" as Rarity

// the system card as a collectible: a portrait trading card of this computer, drawn at its
// design size (63 x 88 mm, a playing card's) and scaled by whoever shows it. The figure stands
// in a holo window, the parts are its moves, the machine's power is its number, and the
// motto is its flavour text. The machine sets its rarity (registry/rarity.js): a Common
// card is plain paper, and each tier up shines more, to the gilded, rainbow-edged Secret
// rare. `foil` (-1..1 each way) slides the holo as the card tilts, and `live` lets its
// sparkles twinkle. Saved or copied, the picture carries the card's design
// (ryoku-hub profile card)
Item {
    id: card

    readonly property real designWidth: 630
    readonly property real designHeight: 880
    property vector2d foil: Qt.vector2d(0, 0)
    property bool live: false
    property var traded: null
    property string cardsStoreRoot: ""
    readonly property bool verifiedRecord: !!card.traded && card.traded.verified === true && card.traded.badgeCheck === "ok"
    property var badge: card.traded ? (card.verifiedRecord ? card.traded.badge || null : null) : Badge.ownBadge
    property var design: card.traded ? (card.verifiedRecord ? card.traded.design || null : null) : Badge.ownDesign
    readonly property var designSpec: card.design ? card.design.spec || card.design : null
    readonly property var cardSpec: card.designSpec && card.designSpec.card ? card.designSpec.card : null
    readonly property var palette: card.cardSpec ? card.cardSpec.palette : null
    readonly property color ink: card.palette ? card.palette.ink : Lithic.fg
    readonly property color mutedInk: card.palette ? Lithic.mix(card.palette.plate, card.palette.ink, 0.72) : Lithic.fg2
    readonly property color quietInk: card.palette ? Lithic.mix(card.palette.plate, card.palette.ink, 0.58) : Lithic.fg3
    readonly property color accent: card.palette ? card.palette.accent : Lithic.sel
    readonly property color plate: card.palette ? card.palette.plate : Lithic.stone1
    readonly property bool moving: card.live && card.visible && !Lithic.reduceMotion
    readonly property vector2d tilt: card.moving ? card.foil : Qt.vector2d(0, 0)
    readonly property bool imagesReady: background.ready && artLayers.ready && frameOverlay.ready && (card.cardSpec || cardFigure.ready) && (!card.badge || badgeSeal.ready)
    readonly property var designEffects: card.cardSpec ? card.cardSpec.effects || [] : []
    readonly property real glitchStrength: {
        for (var i = 0; i < card.designEffects.length; ++i)
            if (card.designEffects[i].id === "glitch") return card.designEffects[i].strength;
        return 0;
    }
    readonly property real glitchSpeed: {
        for (var i = 0; i < card.designEffects.length; ++i)
            if (card.designEffects[i].id === "glitch") return card.designEffects[i].speed;
        return 0;
    }
    readonly property var hw: card.traded ? card.traded.hw : null
    readonly property var code: card.traded ? { x: 40, ys: [550, 608, 666], pitch: 4, tick: 2, unit: 2.5, levels: card.traded.levels } : CardCode.code
    // a tier to show instead of the machine's own (About's rarity ladder previews them), or -1
    property int showLevel: -1
    // the figure, for whoever lets the reader drag it to frame it
    readonly property Item figure: cardFigure
    readonly property bool secret: card.level >= 5
    // One clock drives all live effects. Still renders always read phase zero.
    property real elapsed: 0
    readonly property real clock: card.moving ? card.elapsed : 0
    NumberAnimation on elapsed {
        from: 0
        to: 3600
        duration: 3600000
        loops: Animation.Infinite
        running: card.moving && (card.secret || card.designEffects.length > 0)
    }
    // the sweep's place across the card: crosses in 1.2 s, then rests 3 s off the card
    readonly property real sweepAt: {
        var t = card.clock % 4.2;
        return t < 1.2 ? -0.4 + 1.8 * (t / 1.2) : -1;
    }

    // the card prints the machine's whole read-out (cores, display, packages), which Settings
    // reads only when something asks for it
    Component.onCompleted: {
        if (!card.traded && !SysInfo.loaded && !SysInfo.loading)
            SysInfo.load();
    }

    width: card.designWidth
    height: card.designHeight

    function f(path, def) { return ProfileStore.get(path, def); }
    function known(v) { return v !== undefined && v !== null && String(v).length > 0 && String(v) !== "-"; }
    function num(re, s) { var m = re.exec(String(s || "")); return m ? parseInt(m[1]) : 0; }

    readonly property string name: card.traded ? card.traded.name : card.f("text.name", SysInfo.sysUser)
    readonly property string stockTagline: I18n.tr("For the sake of power and beauty.")
    readonly property bool designTagline: !!(card.cardSpec && card.cardSpec.words && card.cardSpec.words.tagline !== undefined)
    readonly property string tagline: card.designTagline ? card.cardSpec.words.tagline : card.traded ? card.traded.tagline : card.f("text.tagline", card.stockTagline)
    readonly property bool stockWords: !card.traded && !card.designTagline && card.tagline === card.stockTagline
    readonly property int threads: card.hw ? card.hw.threads : card.num(/(\d+)\s*T/, SysInfo.sysCpuCores) || card.num(/(\d+)/, SysInfo.sysCpuCores)
    readonly property int ramGb: card.hw ? card.hw.ramGb : card.num(/\/\s*(\d+)/, SysInfo.sysRam) || Math.round(SysInfo.memoryKb / 1048576)
    readonly property int refresh: card.hw ? card.hw.refresh : card.num(/(\d+)/, SysInfo.sysRefresh)
    readonly property bool discrete: card.hw ? card.hw.gpuClass > 0 : Math.max(Rarity.gpuClass(SysInfo.sysGpu), Rarity.gpuClass(SysInfo.sysGpu2)) > 0
    readonly property int screenWidth: card.hw ? card.hw.width : card.num(/^(\d+)/, SysInfo.sysResolution)
    readonly property var rarity: Rarity.of(card.hw || { threads: card.threads, ramGb: card.ramGb, gpus: [SysInfo.sysGpu, SysInfo.sysGpu2],
                                              refresh: card.refresh, width: card.screenWidth })
    readonly property int level: card.showLevel >= 0 ? card.showLevel : card.rarity.level
    readonly property var tier: Rarity.tiers[card.level]
    // each tier's shine: how strong the window's holo is, and the face's
    readonly property real windowFoil: [0, 0.2, 0.42, 0.62, 0.7, 0.8][card.level]
    readonly property real faceFoil: [0, 0, 0.06, 0.12, 0.16, 0.2][card.level]
    // the border: bone for the plain tiers, the sun for the rare ones, gold from Ultra rare
    readonly property color rim: card.palette ? card.palette.rim : card.level >= 4 ? Lithic.gold : card.level >= 2 ? Lithic.sel : Lithic.mix(Lithic.ground, Lithic.fg, 0.55)
    readonly property color shine: card.palette ? card.palette.accent : card.level >= 4 ? Lithic.gold : Lithic.fg
    readonly property int power: Rarity.power(card.threads, card.ramGb, card.discrete)
    readonly property string serial: {
        var id = String(card.traded ? card.traded.id : Badge.status.cardId || (CardCode.code ? CardCode.code.cardId : "") || "");
        return /^[0-9a-f]{16}$/.test(id) ? id.slice(0, 6) : "";
    }
    readonly property var hero: card.traded ? card.traded.hero : card.f("hero", ({}))
    readonly property string heroKind: card.hero.kind || "default"
    readonly property string illustrator: card.heroKind === "default" ? I18n.tr("Caracalla Farnese, Naples. Photo Marie-Lan Nguyen, CC BY 2.5")
        : card.heroKind === "custom" ? I18n.tr("Figure by %1").replace("%1", card.name)
        : I18n.tr("Figure from Ryoku's gallery")
    readonly property string typeLine: (card.hw ? [card.hw.distro, card.hw.wm, card.hw.gpu]
        : [SysInfo.sysDistro !== "Linux" ? SysInfo.sysDistro : "", SysInfo.sysWM, card.known(SysInfo.sysGpu) ? SysInfo.sysGpu : ""])
        .filter(s => card.known(s)).join("  ·  ")
    readonly property var moves: (card.hw ? [
        { glyph: "memory", name: I18n.tr("Processor"), detail: I18n.tr("%1 threads").replace("%1", card.threads), value: card.threads, unit: I18n.tr("threads") },
        { glyph: "monitor", name: I18n.tr("Display"), detail: I18n.tr("%1 px wide").replace("%1", card.screenWidth), value: card.refresh, unit: "Hz" },
        { glyph: "developer_board", name: I18n.tr("Memory"), detail: card.ramGb + " GB", value: card.ramGb, unit: "GB" }
    ] : [
        { glyph: "memory", name: I18n.tr("Processor"), detail: SysInfo.sysCpu, value: card.threads, unit: I18n.tr("threads") },
        { glyph: "monitor", name: I18n.tr("Display"), detail: SysInfo.sysResolution, value: card.refresh, unit: "Hz" },
        { glyph: "developer_board", name: I18n.tr("Memory"), detail: SysInfo.sysRam, value: card.ramGb, unit: "GB" }
    ]).filter(m => m.value > 0)
    readonly property var traits: card.traded ? [
        { name: I18n.tr("From"), value: card.name },
        { name: I18n.tr("Added"), value: Qt.formatDate(new Date(card.traded.added), "yyyy-MM-dd") },
        { name: I18n.tr("Rarity points"), value: String(card.rarity.points) }
    ] : [
        { name: I18n.tr("Up for"), value: SysInfo.sysUptime },
        { name: I18n.tr("Packages"), value: SysInfo.sysPackages },
        { name: I18n.tr("Kernel"), value: String(SysInfo.sysKernel).replace(/-.*/, "") }
    ]
    readonly property string release: card.traded ? "" : Updates.currentName || Updates.currentVersion || "Ryoku"

    // the card's border and its face. A Secret rare's border is cut like a diamond
    CutRect {
        anchors.fill: parent
        cut: 22
        fill: card.rim
    }
    ShaderEffect {
        visible: card.secret && !card.cardSpec
        anchors.fill: parent
        property color c1: Lithic.jasper
        property color c2: Lithic.gold
        property color c3: Lithic.jade
        property color c4: Lithic.lapis
        property color c5: Lithic.sel
        property vector2d size: Qt.vector2d(width, height)
        property vector2d shift: card.tilt
        property real time: card.clock
        property real cut: 22
        property real facet: 9
        fragmentShader: Qt.resolvedUrl("../../shaders/diamond.frag.qsb")
    }
    DesignEffects {
        anchors.fill: parent
        effects: card.designEffects
        borderOnly: true
        accent: card.accent
        ink: card.ink
        time: card.clock
        tilt: card.tilt
    }
    CutRect {
        id: face
        anchors.fill: parent
        // a Secret rare's diamond border is set wider, so the stone shows
        anchors.margins: card.secret ? 20 : 14
        cut: 14
        fill: card.plate
        // from Holo rare the face is edged in gold
        strokeWidth: card.level >= 3 ? 2 : 1
        strokeColor: card.palette ? card.accent : card.level >= 3 ? Lithic.gold : Lithic.alpha(Lithic.fg, 0.2)
        clip: true

        DesignImage {
            id: background
            anchors.fill: parent
            source: card.cardSpec && card.cardSpec.background ? "file://" + card.cardSpec.background : ""
            live: card.moving
        }
        // the face's own foil, from Rare up; a Secret rare's is every mineral, etched
        ShaderEffect {
            visible: card.faceFoil > 0 && !card.cardSpec
            anchors.fill: parent
            property color c1: card.secret ? Lithic.jasper : Lithic.sel
            property color c2: Lithic.gold
            property color c3: card.secret ? Lithic.jade : Lithic.lapis
            property color c4: card.secret ? Lithic.lapis : Lithic.jade
            property vector2d size: Qt.vector2d(width, height)
            property vector2d shift: card.tilt
            property real strength: card.secret ? 0.42 : card.faceFoil
            property real cut: 14
            property real etch: card.secret ? 1 : 0
            fragmentShader: Qt.resolvedUrl("../../shaders/foil.frag.qsb")
        }
    }
    // Frame art stays below Ryoku's locked lanes, never over the code or labels.
    DesignImage {
        id: frameOverlay
        anchors.fill: parent
        source: card.cardSpec && card.cardSpec.frame ? "file://" + card.cardSpec.frame : ""
        live: card.moving
    }
    CutRect {
        visible: !!card.cardSpec
        x: 28; y: 22; width: card.width - 56; height: 74
        cut: 8
        fill: card.plate
    }
    CutRect {
        visible: !!card.cardSpec
        x: 28; y: 530; width: card.width - 56; height: 332
        cut: 8
        fill: card.plate
    }

    // the head: its stage, its name, its power
    Label {
        // drawn as outlines, so it stays sharp at any scale the card is shown or saved at
        renderType: Text.QtRendering
        x: 34
        y: 26
        text: I18n.tr("Ryoku machine") + "  ·  " + I18n.tr(card.tier.name) + (UiPreferences.showJapaneseGloss ? "  ·  力の機械" : "")
        typeRole: "monoSm"
        font.pixelSize: 12
        font.letterSpacing: 1.6
        font.capitalization: Font.AllUppercase
        ink: card.mutedInk
    }
    Label {
        renderType: Text.QtRendering
        id: nameLabel
        x: 32
        y: 42
        width: 400
        text: card.name
        ink: card.ink
        typeRole: "name"
        font.pixelSize: 42
        lineHeight: 50
        elide: Text.ElideRight
    }
    Label {
        renderType: Text.QtRendering
        id: powerLabel
        anchors.right: parent.right
        anchors.rightMargin: 34
        y: 40
        text: String(card.power)
        typeRole: "name"
        font.pixelSize: 44
        font.weight: Font.Black
        ink: card.shine
    }
    Label {
        renderType: Text.QtRendering
        anchors.right: powerLabel.left
        anchors.rightMargin: 6
        anchors.baseline: powerLabel.baseline
        text: "力"
        typeRole: "title"
        font.pixelSize: 22
        ink: card.accent
    }
    // a Secret rare's name is gilded too
    Binding {
        target: nameLabel
        property: "ink"
        value: card.shine
        when: card.secret && !card.cardSpec
    }

    // the figure in its holo window
    CutRect {
        id: window
        x: 30
        y: 104
        width: card.width - 60
        height: 400
        cut: 8
        fill: card.palette ? card.plate : Lithic.ground
        strokeWidth: 3
        strokeColor: card.palette ? card.rim : card.level >= 4 ? Lithic.gold : Lithic.mix(card.rim, Lithic.fg, 0.35)
        clip: true

        ShaderEffect {
            visible: card.windowFoil > 0 && !card.cardSpec
            anchors.fill: parent
            anchors.margins: 3
            property color c1: card.secret ? Lithic.jasper : Lithic.sel
            property color c2: Lithic.gold
            property color c3: card.secret ? Lithic.jade : Lithic.lapis
            property color c4: card.secret ? Lithic.lapis : Lithic.jade
            property vector2d size: Qt.vector2d(width, height)
            property vector2d shift: card.tilt
            property real strength: card.windowFoil
            property real cut: 6
            property real etch: card.secret ? 0.6 : 0
            fragmentShader: Qt.resolvedUrl("../../shaders/foil.frag.qsb")
        }
        Text {
            textFormat: Text.PlainText
            visible: !card.cardSpec
            renderType: Text.QtRendering
            anchors.horizontalCenter: parent.horizontalCenter
            y: 24
            text: card.cardSpec && card.cardSpec.words && card.cardSpec.words.epithet !== undefined ? card.cardSpec.words.epithet : card.traded ? card.traded.epithet : String(card.f("text.epithets", []).length > 0 ? card.f("text.epithets", [])[0] : "Ryoku")
            color: Lithic.alpha(card.ink, 0.1)
            font.family: Lithic.font.sans
            font.weight: Font.Black
            font.capitalization: Font.AllUppercase
            font.pixelSize: 120
        }
        CardFigure {
            id: cardFigure
            override: card.traded ? card.traded.hero : null
            visible: !card.cardSpec
            cardsStoreRoot: card.cardsStoreRoot
            tradedId: card.traded ? String(card.traded.id || "") : ""
            verifiedRecord: card.verifiedRecord
            readonly property real aspect: card.heroKind === "default" ? stockAspect : 900 / 1157
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottom: parent.bottom
            // your own picture fills the window, as a photo card's art does; a figure stands
            // in it, and from Ultra rare fills it, a full art
            readonly property bool fills: card.heroKind === "custom"
            height: fills ? parent.height - 6 : card.level >= 4 ? parent.height + 20 : parent.height - 18
            width: fills ? parent.width - 6 : Math.round(height * aspect)
            soft: true
            ink: Lithic.dark ? Lithic.fg : Lithic.stone4
        }
        ArtLayers {
            id: artLayers
            anchors.fill: parent
            anchors.margins: 3
            layers: card.cardSpec ? card.cardSpec.art || [] : []
            epithet: card.cardSpec && card.cardSpec.words ? card.cardSpec.words.epithet || "" : ""
            ink: card.ink
            live: card.moving
            tilt: card.tilt
            time: card.clock * card.glitchSpeed
            glitch: card.glitchStrength
        }
        DesignEffects {
            anchors.fill: artLayers
            effects: card.designEffects
            glow: card.cardSpec ? card.cardSpec.glow || "" : ""
            accent: card.accent
            ink: card.ink
            time: card.clock
            tilt: card.tilt
        }
    }
    Emblem {
        id: badgeSeal
        visible: !!card.badge
        x: 502; y: 78
        size: 104
        role: card.badge ? card.badge.role : "tester"
        spec: card.designSpec ? card.designSpec.badge : null
        live: card.moving
        layer.enabled: visible
        layer.effect: MultiEffect {
            shadowEnabled: true
            shadowColor: Lithic.ground
            shadowOpacity: 0.8
            shadowBlur: 0.55
            shadowVerticalOffset: 5
        }
    }
    // the type line and who made the figure, under the window as a card prints them
    CutRect {
        x: 54
        y: window.y + window.height - 6
        width: card.width - 108
        height: 30
        cut: 6
        fill: card.palette ? card.plate : Lithic.mix(Lithic.ground, Lithic.gold, 0.22)
        strokeWidth: 1
        strokeColor: card.palette ? Lithic.alpha(card.accent, 0.5) : Lithic.alpha(Lithic.fg, 0.25)
        Label {
            renderType: Text.QtRendering
            anchors.centerIn: parent
            width: parent.width - 24
            horizontalAlignment: Text.AlignHCenter
            text: "No. " + card.serial + "    " + card.typeLine
            typeRole: "monoSm"
            font.pixelSize: 11
            ink: card.ink
            elide: Text.ElideRight
        }
    }
    Label {
        renderType: Text.QtRendering
        x: 40
        y: window.y + window.height + 30
        width: card.width - 80
        horizontalAlignment: Text.AlignRight
        text: card.cardSpec ? I18n.tr("Design · %1").replace("%1", card.designSpec.name) : "Illus. " + card.illustrator
        typeRole: "monoSm"
        font.pixelSize: 9
        ink: card.quietInk
        elide: Text.ElideRight
    }

    // the moves: each part, what it is, and its number
    // each move stands in the band under its ruler, clear of the tallest tick
    Item {
        id: moveList
        x: 40
        width: card.width - 80
        readonly property var bands: card.code ? card.code.ys : [550, 608, 666]

        Repeater {
            model: card.moves
            delegate: Item {
                required property var modelData
                required property int index
                y: moveList.bands[Math.min(index, moveList.bands.length - 1)] + 12
                width: moveList.width
                height: 44
                CutRect {
                    x: 0
                    anchors.verticalCenter: parent.verticalCenter
                    width: 30
                    height: 30
                    cut: 6
                    fill: Lithic.alpha(card.accent, 0.18)
                    Glyph {
                        renderType: Text.QtRendering
                        anchors.centerIn: parent
                        name: modelData.glyph
                        size: 18
                        ink: card.accent
                    }
                }
                Column {
                    x: 44
                    anchors.verticalCenter: parent.verticalCenter
                    width: parent.width - 44 - 120
                    Label {
                        renderType: Text.QtRendering
                        text: modelData.name
                        ink: card.ink
                        typeRole: "label"
                        font.pixelSize: 19
                        font.weight: Font.Bold
                    }
                    Label {
                        renderType: Text.QtRendering
                        width: parent.width
                        text: modelData.detail
                        typeRole: "detail"
                        font.pixelSize: 13
                        ink: card.mutedInk
                        elide: Text.ElideRight
                    }
                }
                Label {
                    renderType: Text.QtRendering
                    id: amount
                    anchors.right: unit.left
                    anchors.rightMargin: 4
                    anchors.verticalCenter: parent.verticalCenter
                    text: String(modelData.value)
                    typeRole: "name"
                    font.pixelSize: 30
                    font.weight: Font.Black
                    ink: card.shine
                }
                Label {
                    renderType: Text.QtRendering
                    id: unit
                    anchors.right: parent.right
                    anchors.baseline: amount.baseline
                    text: modelData.unit
                    typeRole: "monoSm"
                    font.pixelSize: 11
                    ink: card.mutedInk
                }
            }
        }
    }

    // the card's code: three tick rulers that rule off the parts of the card and carry its
    // design in their tick heights, so a picture of it that went through a chat app still
    // opens as a card (Singletons/CardCode.qml, backend/cardcode.go). Where they stand,
    // the card keeps clear. The window above stays where the code reads an own picture
    Repeater {
        model: card.code ? card.code.ys.length : 0
        delegate: Item {
            id: ruler
            required property int index
            readonly property var levels: card.code.levels[ruler.index]
            x: card.code.x
            y: card.code.ys[ruler.index]
            // The image code requires an uninterrupted, opaque contrast field.
            Rectangle {
                x: -4; y: -2
                width: card.width - 72; height: 14
                color: card.plate
            }
            Repeater {
                model: ruler.levels.length
                delegate: Rectangle {
                    required property int index
                    x: index * card.code.pitch
                    width: card.code.tick
                    height: (ruler.levels[index] + 1) * card.code.unit
                    color: card.palette ? card.ink : card.level >= 4 ? Lithic.gold : Lithic.fg2
                    antialiasing: false
                }
            }
        }
    }

    // its traits, as a card's weakness, resistance and retreat
    Row {
        x: 40
        y: 734
        width: card.width - 80
        Repeater {
            model: card.traits
            delegate: Column {
                required property var modelData
                width: (card.width - 80) / 3
                spacing: 2
                Label {
                    renderType: Text.QtRendering
                    text: modelData.name
                    typeRole: "monoSm"
                    font.pixelSize: 10
                    font.letterSpacing: 1.2
                    font.capitalization: Font.AllUppercase
                    ink: card.quietInk
                }
                Label {
                    renderType: Text.QtRendering
                    width: parent.width - 8
                    text: card.known(modelData.value) ? modelData.value : "-"
                    ink: card.ink
                    typeRole: "mono"
                    font.pixelSize: 13
                    elide: Text.ElideRight
                }
            }
        }
    }

    // the flavour text: the reader's line, and its gloss
    CutRect {
        x: 40
        y: 780
        width: card.width - 80
        height: 46
        cut: 6
        fill: card.palette ? card.plate : "transparent"
        strokeWidth: 1
        strokeColor: Lithic.alpha(card.palette ? card.accent : Lithic.gold, 0.6)
        Label {
            renderType: Text.QtRendering
            anchors.left: parent.left
            anchors.leftMargin: 14
            anchors.right: gloss.visible ? gloss.left : parent.right
            anchors.rightMargin: 10
            anchors.verticalCenter: parent.verticalCenter
            text: card.tagline
            typeRole: "proof"
            font.pixelSize: 14
            font.italic: true
            ink: card.mutedInk
            elide: Text.ElideRight
        }
        Label {
            renderType: Text.QtRendering
            id: gloss
            visible: card.stockWords && UiPreferences.showJapaneseGloss
            anchors.right: parent.right
            anchors.rightMargin: 14
            anchors.verticalCenter: parent.verticalCenter
            text: "力と美のために"
            typeRole: "caption"
            font.pixelSize: 12
            ink: card.quietInk
        }
    }

    // the set's mark, its release and the card's rarity
    Row {
        x: 40
        y: 838
        spacing: 8
        RyokuMark {
            anchors.verticalCenter: parent.verticalCenter
            width: 16
            height: 14
            ink: card.mutedInk
            accent: card.accent
        }
        Label {
            renderType: Text.QtRendering
            anchors.verticalCenter: parent.verticalCenter
            text: card.traded ? I18n.tr("TRADED · from %1").replace("%1", card.name) : card.release + "  ·  " + card.serial
            width: 255
            elide: Text.ElideRight
            typeRole: "monoSm"
            font.pixelSize: 10
            ink: card.mutedInk
        }
    }
    // its rarity, as a card prints it in its corner
    Row {
        anchors.right: parent.right
        anchors.rightMargin: 40
        y: 836
        spacing: 8
        Label {
            renderType: Text.QtRendering
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr(card.tier.name).toUpperCase() + "  ·  " + I18n.tr("%1 pts").replace("%1", card.rarity.points)
            typeRole: "monoSm"
            font.pixelSize: 10
            font.letterSpacing: 1.2
            ink: card.palette ? card.accent : card.level >= 3 ? Lithic.gold : Lithic.fg2
        }
        Label {
            renderType: Text.QtRendering
            anchors.verticalCenter: parent.verticalCenter
            text: card.tier.mark
            typeRole: "label"
            font.pixelSize: 15
            ink: card.palette ? card.accent : card.level >= 2 ? Lithic.gold : Lithic.fg2
            property real beat: 1
            scale: card.moving ? beat : 1
            // a Secret rare's stars beat while it is live
            SequentialAnimation on beat {
                running: card.secret && card.moving
                loops: Animation.Infinite
                NumberAnimation { to: 1.25; duration: 260; easing.type: Easing.OutCubic }
                NumberAnimation { to: 1; duration: 520; easing.type: Easing.InOutSine }
                PauseAnimation { duration: 1400 }
            }
        }
    }

    // from Ultra rare up, sparkles over the figure; they twinkle while the card is live. A
    // Secret rare has twice as many, and they drift up and turn
    Repeater {
        model: card.cardSpec || card.level < 4 ? []
            : [[0.12, 0.2, 18], [0.86, 0.14, 24], [0.78, 0.52, 14], [0.18, 0.62, 20], [0.52, 0.08, 12], [0.92, 0.78, 16]]
              .concat(card.secret ? [[0.3, 0.4, 10], [0.66, 0.3, 16], [0.08, 0.86, 12], [0.44, 0.7, 14], [0.62, 0.9, 10], [0.96, 0.4, 12]] : [])
        delegate: Text {
            id: spark
            required property var modelData
            required property int index
            readonly property real rise: card.secret ? ((card.clock * 0.05 + spark.index * 0.137) % 1) : 0
            x: window.x + window.width * modelData[0] - width / 2 + (card.secret ? Math.sin(card.clock * 0.9 + spark.index) * 6 : 0)
            y: window.y + window.height * (card.secret ? (modelData[1] + 1 - spark.rise * 0.35) % 1 : modelData[1]) - height / 2
            rotation: card.secret ? card.clock * (spark.index % 2 ? 40 : -30) : 0
            text: "✦"
            font.pixelSize: modelData[2]
            color: card.secret ? [Lithic.gold, Lithic.fg, Lithic.jade, Lithic.lapis][spark.index % 4] : spark.index % 2 ? Lithic.gold : Lithic.fg
            property real glimmer: 0.85
            opacity: card.moving ? glimmer : 0.85
            SequentialAnimation on glimmer {
                running: card.moving
                loops: Animation.Infinite
                PauseAnimation { duration: 400 * spark.index }
                NumberAnimation { to: 0.15; duration: 700; easing.type: Easing.InOutSine }
                NumberAnimation { to: 0.95; duration: 700; easing.type: Easing.InOutSine }
                PauseAnimation { duration: 1200 }
            }
        }
    }

    // the light that crosses a Secret rare now and then, over everything on it
    ShaderEffect {
        visible: card.secret && !card.cardSpec && card.sweepAt > -0.5
        anchors.fill: parent
        property color tint: Lithic.gold
        property vector2d size: Qt.vector2d(width, height)
        property real pos: card.sweepAt
        property real cut: 22
        fragmentShader: Qt.resolvedUrl("../../shaders/sweep.frag.qsb")
    }
}
