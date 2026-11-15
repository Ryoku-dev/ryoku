import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import ".."
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
    // a tier to show instead of the machine's own (About's rarity ladder previews them), or -1
    property int showLevel: -1

    width: card.designWidth
    height: card.designHeight

    function f(path, def) { return ProfileStore.get(path, def); }
    function known(v) { return v !== undefined && v !== null && String(v).length > 0 && String(v) !== "-"; }
    function num(re, s) { var m = re.exec(String(s || "")); return m ? parseInt(m[1]) : 0; }

    readonly property string name: card.f("text.name", SysInfo.sysUser)
    readonly property int threads: card.num(/(\d+)\s*T/, SysInfo.sysCpuCores) || card.num(/(\d+)/, SysInfo.sysCpuCores)
    readonly property int ramGb: card.num(/\/\s*(\d+)/, SysInfo.sysRam) || Math.round(SysInfo.memoryKb / 1048576)
    readonly property int refresh: card.num(/(\d+)/, SysInfo.sysRefresh)
    readonly property bool discrete: Math.max(Rarity.gpuClass(SysInfo.sysGpu), Rarity.gpuClass(SysInfo.sysGpu2)) > 0
    readonly property int screenWidth: card.num(/^(\d+)/, SysInfo.sysResolution)
    readonly property var rarity: Rarity.of({ threads: card.threads, ramGb: card.ramGb, gpus: [SysInfo.sysGpu, SysInfo.sysGpu2],
                                              refresh: card.refresh, width: card.screenWidth })
    readonly property int level: card.showLevel >= 0 ? card.showLevel : card.rarity.level
    readonly property var tier: Rarity.tiers[card.level]
    // each tier's shine: how strong the window's holo is, and the face's
    readonly property real windowFoil: [0, 0.2, 0.42, 0.62, 0.7, 0.8][card.level]
    readonly property real faceFoil: [0, 0, 0.06, 0.12, 0.16, 0.2][card.level]
    // the border: bone for the plain tiers, the sun for the rare ones, gold from Ultra rare
    readonly property color rim: card.level >= 4 ? Lithic.gold : card.level >= 2 ? Lithic.sel : Lithic.mix(Lithic.ground, Lithic.fg, 0.55)
    readonly property color shine: card.level >= 4 ? Lithic.gold : Lithic.fg
    // its power, as a card's HP: the machine's threads, memory and a discrete card, in tens
    readonly property int power: Math.max(30, Math.min(340, Math.round((card.threads * 8 + card.ramGb * 3 + (card.discrete ? 60 : 0)) / 10) * 10))
    // a card number from the host's name, so every machine has its own
    readonly property string serial: {
        var h = 7, s = String(SysInfo.sysHost || "ryoku");
        for (var i = 0; i < s.length; i++)
            h = (h * 31 + s.charCodeAt(i)) % 997;
        return ("00" + (h % 256 + 1)).slice(-3) + "/256";
    }
    readonly property var hero: card.f("hero", ({}))
    readonly property string heroKind: card.hero.kind || "default"
    readonly property string illustrator: card.heroKind === "default" ? I18n.tr("Caracalla Farnese, Naples. Photo Marie-Lan Nguyen, CC BY 2.5")
        : card.heroKind === "custom" ? I18n.tr("Figure by %1").replace("%1", card.name)
        : I18n.tr("Figure from Ryoku's gallery")
    readonly property string typeLine: [SysInfo.sysDistro !== "Linux" ? SysInfo.sysDistro : "", SysInfo.sysWM, card.known(SysInfo.sysGpu) ? SysInfo.sysGpu : ""]
        .filter(s => card.known(s)).join("  ·  ")
    readonly property var moves: [
        { glyph: "memory", name: I18n.tr("Processor"), detail: SysInfo.sysCpu, value: card.threads, unit: I18n.tr("threads") },
        { glyph: "monitor", name: I18n.tr("Display"), detail: SysInfo.sysResolution, value: card.refresh, unit: "Hz" },
        { glyph: "developer_board", name: I18n.tr("Memory"), detail: SysInfo.sysRam, value: card.ramGb, unit: "GB" }
    ].filter(m => m.value > 0)
    readonly property var traits: [
        { name: I18n.tr("Up for"), value: SysInfo.sysUptime },
        { name: I18n.tr("Packages"), value: SysInfo.sysPackages },
        { name: I18n.tr("Kernel"), value: String(SysInfo.sysKernel).replace(/-.*/, "") }
    ]
    readonly property string release: Updates.currentName || Updates.currentVersion || "Ryoku"

    // the card's border and its face; a Secret rare's border is all foil
    CutRect {
        anchors.fill: parent
        cut: 22
        fill: card.rim
    }
    ShaderEffect {
        visible: card.level >= 5
        anchors.fill: parent
        property color c1: Lithic.sel
        property color c2: Lithic.gold
        property color c3: Lithic.lapis
        property color c4: Lithic.jade
        property vector2d size: Qt.vector2d(width, height)
        property vector2d shift: card.foil
        property real strength: 1
        property real cell: 1
        property real cut: 22
        fragmentShader: Qt.resolvedUrl("../../shaders/foil.frag.qsb")
    }
    CutRect {
        id: face
        anchors.fill: parent
        anchors.margins: 14
        cut: 14
        fill: Lithic.stone1
        // from Holo rare the face is edged in gold
        strokeWidth: card.level >= 3 ? 2 : 1
        strokeColor: card.level >= 3 ? Lithic.gold : Lithic.alpha(Lithic.fg, 0.2)
        clip: true

        // the face's own foil, from Rare up
        ShaderEffect {
            visible: card.faceFoil > 0
            anchors.fill: parent
            property color c1: Lithic.sel
            property color c2: Lithic.gold
            property color c3: Lithic.lapis
            property color c4: Lithic.jade
            property vector2d size: Qt.vector2d(width, height)
            property vector2d shift: card.foil
            property real strength: card.faceFoil
            property real cell: 2
            property real cut: 14
            fragmentShader: Qt.resolvedUrl("../../shaders/foil.frag.qsb")
        }
    }

    // the head: its stage, its name, its power
    Label {
        x: 34
        y: 26
        text: I18n.tr("Ryoku machine") + "  ·  " + I18n.tr(card.tier.name) + (UiPreferences.showJapaneseGloss ? "  ·  力の機械" : "")
        typeRole: "monoSm"
        font.pixelSize: 12
        font.letterSpacing: 1.6
        font.capitalization: Font.AllUppercase
        inkRole: "fg2"
    }
    Label {
        x: 32
        y: 42
        width: 400
        text: card.name
        typeRole: "name"
        font.pixelSize: 42
        lineHeight: 50
        elide: Text.ElideRight
    }
    Label {
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
        anchors.right: powerLabel.left
        anchors.rightMargin: 6
        anchors.baseline: powerLabel.baseline
        text: "力"
        typeRole: "title"
        font.pixelSize: 22
        ink: card.level >= 4 ? Lithic.gold : Lithic.sel
    }

    // the figure in its holo window
    CutRect {
        id: window
        x: 30
        y: 104
        width: card.width - 60
        height: 400
        cut: 8
        fill: Lithic.ground
        strokeWidth: 3
        strokeColor: card.level >= 4 ? Lithic.gold : Lithic.mix(card.rim, Lithic.fg, 0.35)
        clip: true

        ShaderEffect {
            visible: card.windowFoil > 0
            anchors.fill: parent
            anchors.margins: 3
            property color c1: Lithic.sel
            property color c2: Lithic.gold
            property color c3: Lithic.lapis
            property color c4: Lithic.jade
            property vector2d size: Qt.vector2d(width, height)
            property vector2d shift: card.foil
            property real strength: card.windowFoil
            property real cell: 2
            property real cut: 6
            fragmentShader: Qt.resolvedUrl("../../shaders/foil.frag.qsb")
        }
        Text {
            anchors.horizontalCenter: parent.horizontalCenter
            y: 24
            text: String(card.f("text.epithets", []).length > 0 ? card.f("text.epithets", [])[0] : "Ryoku")
            color: Lithic.alpha(Lithic.fg, 0.1)
            font.family: Lithic.font.sans
            font.weight: Font.Black
            font.capitalization: Font.AllUppercase
            font.pixelSize: 120
        }
        CardFigure {
            readonly property real aspect: card.heroKind === "default" ? stockAspect : 900 / 1157
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottom: parent.bottom
            // from Ultra rare the figure fills its window, a full art
            height: card.level >= 4 ? parent.height + 20 : parent.height - 18
            width: Math.round(height * aspect)
            soft: true
            ink: Lithic.dark ? Lithic.fg : Lithic.stone4
        }
    }
    // the type line and who made the figure, under the window as a card prints them
    CutRect {
        x: 54
        y: window.y + window.height - 6
        width: card.width - 108
        height: 30
        cut: 6
        fill: Lithic.mix(Lithic.ground, Lithic.gold, 0.22)
        strokeWidth: 1
        strokeColor: Lithic.alpha(Lithic.fg, 0.25)
        Label {
            anchors.centerIn: parent
            width: parent.width - 24
            horizontalAlignment: Text.AlignHCenter
            text: "No. " + card.serial + "    " + card.typeLine
            typeRole: "monoSm"
            font.pixelSize: 11
            inkRole: "fg"
            elide: Text.ElideRight
        }
    }
    Label {
        x: 40
        y: window.y + window.height + 30
        width: card.width - 80
        horizontalAlignment: Text.AlignRight
        text: "Illus. " + card.illustrator
        typeRole: "monoSm"
        font.pixelSize: 9
        inkRole: "fg3"
        elide: Text.ElideRight
    }

    // the moves: each part, what it is, and its number
    Column {
        id: moveList
        x: 40
        y: 566
        width: card.width - 80
        spacing: 2

        Repeater {
            model: card.moves
            delegate: Item {
                required property var modelData
                required property int index
                width: moveList.width
                height: 54
                Rectangle {
                    visible: index > 0
                    width: parent.width
                    height: 1
                    color: Lithic.alpha(Lithic.fg, 0.14)
                }
                CutRect {
                    x: 0
                    anchors.verticalCenter: parent.verticalCenter
                    width: 30
                    height: 30
                    cut: 6
                    fill: Lithic.alpha(Lithic.sel, 0.18)
                    Glyph {
                        anchors.centerIn: parent
                        name: modelData.glyph
                        size: 18
                        ink: Lithic.sel
                    }
                }
                Column {
                    x: 44
                    anchors.verticalCenter: parent.verticalCenter
                    width: parent.width - 44 - 120
                    Label {
                        text: modelData.name
                        typeRole: "label"
                        font.pixelSize: 19
                        font.weight: Font.Bold
                    }
                    Label {
                        width: parent.width
                        text: modelData.detail
                        typeRole: "detail"
                        font.pixelSize: 13
                        inkRole: "fg2"
                        elide: Text.ElideRight
                    }
                }
                Label {
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
                    id: unit
                    anchors.right: parent.right
                    anchors.baseline: amount.baseline
                    text: modelData.unit
                    typeRole: "monoSm"
                    font.pixelSize: 11
                    inkRole: "fg2"
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
                    text: modelData.name
                    typeRole: "monoSm"
                    font.pixelSize: 10
                    font.letterSpacing: 1.2
                    font.capitalization: Font.AllUppercase
                    inkRole: "fg3"
                }
                Label {
                    width: parent.width - 8
                    text: card.known(modelData.value) ? modelData.value : "-"
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
        fill: "transparent"
        strokeWidth: 1
        strokeColor: Lithic.alpha(Lithic.gold, 0.6)
        Label {
            anchors.left: parent.left
            anchors.leftMargin: 14
            anchors.right: gloss.left
            anchors.rightMargin: 10
            anchors.verticalCenter: parent.verticalCenter
            text: card.f("text.tagline", I18n.tr("For the sake of power and beauty."))
            typeRole: "proof"
            font.pixelSize: 14
            font.italic: true
            inkRole: "fg2"
            elide: Text.ElideRight
        }
        Label {
            id: gloss
            visible: UiPreferences.showJapaneseGloss
            anchors.right: parent.right
            anchors.rightMargin: 14
            anchors.verticalCenter: parent.verticalCenter
            text: "力と美のために"
            typeRole: "caption"
            font.pixelSize: 12
            inkRole: "fg3"
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
            ink: Lithic.fg2
            accent: Lithic.sel
        }
        Label {
            anchors.verticalCenter: parent.verticalCenter
            text: card.release + "  ·  " + card.serial
            typeRole: "monoSm"
            font.pixelSize: 10
            inkRole: "fg2"
        }
    }
    // its rarity, as a card prints it in its corner
    Row {
        anchors.right: parent.right
        anchors.rightMargin: 40
        y: 836
        spacing: 8
        Label {
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr(card.tier.name).toUpperCase() + "  ·  " + I18n.tr("%1 pts").replace("%1", card.rarity.points)
            typeRole: "monoSm"
            font.pixelSize: 10
            font.letterSpacing: 1.2
            ink: card.level >= 3 ? Lithic.gold : Lithic.fg2
        }
        Label {
            anchors.verticalCenter: parent.verticalCenter
            text: card.tier.mark
            typeRole: "label"
            font.pixelSize: 15
            ink: card.level >= 2 ? Lithic.gold : Lithic.fg2
        }
    }

    // from Ultra rare up, sparkles over the figure; they twinkle while the card is live
    Repeater {
        model: card.level >= 4 ? [[0.12, 0.2, 18], [0.86, 0.14, 24], [0.78, 0.52, 14], [0.18, 0.62, 20], [0.52, 0.08, 12], [0.92, 0.78, 16]] : []
        delegate: Text {
            id: spark
            required property var modelData
            required property int index
            x: window.x + window.width * modelData[0] - width / 2
            y: window.y + window.height * modelData[1] - height / 2
            text: "✦"
            font.pixelSize: modelData[2]
            color: index % 2 ? Lithic.gold : Lithic.fg
            opacity: 0.85
            SequentialAnimation on opacity {
                running: card.live && !Lithic.reduceMotion
                loops: Animation.Infinite
                PauseAnimation { duration: 400 * spark.index }
                NumberAnimation { to: 0.15; duration: 700; easing.type: Easing.InOutSine }
                NumberAnimation { to: 0.95; duration: 700; easing.type: Easing.InOutSine }
                PauseAnimation { duration: 1200 }
            }
        }
    }
}
