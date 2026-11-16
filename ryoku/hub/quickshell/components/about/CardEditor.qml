pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Io
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import ".."
import "../../registry/rarity.js" as Rarity

// the card studio: everything about the system card in one place, beside a live copy of it.
// Picture chooses the figure; Adjust frames and tunes your own picture (drag it on the
// preview, scroll to zoom); Words and Layout set the rest and whether Home shows the plate
// or the card; Share holds the card's rarity, Copy, Save and the binder. Every change
// applies at once through Changes, with Undo
Dialog {
    id: root

    property string pageId: ""
    // Home loads this studio by URL, so hub arrives from the loader before it opens
    property var hub: null
    property var binderOpener: null
    // the tab to open on: "picture", "adjust", "words", "layout" or "share"
    property string tab: "picture"
    readonly property var tabs: ["picture", "adjust", "words", "layout", "share"]
    // what the preview shows; Share always shows the card it copies
    property string view: "card"
    readonly property string shownView: root.tab === "share" ? "card" : root.view
    readonly property string configDir: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku"
    readonly property real bodyHeight: Math.max(380, (root.parent ? root.parent.height : 800) - 2 * LithicTokens.size.dialogMargin
                                                     - root.topPadding - root.bottomPadding - 140)
    property string imageError: ""
    // a tier the Share tab's ladder previews on the card, or -1
    property int previewTier: -1

    function openOn(tab) {
        root.tab = root.tabs.indexOf(tab) >= 0 ? tab : "picture";
        root.open();
    }

    function bind(key, fallback) {
        return { store: "card", key: key, fallback: fallback };
    }
    function val(key, fallback) {
        return Changes.value(root.bind(key, fallback));
    }
    function put(key, fallback, value, label) {
        Changes.set(root.bind(key, fallback), value, label, root.pageId);
    }

    readonly property var heroFallback: ({ kind: "default", source: "", zoom: 1, focalX: 0.5, focalY: 0.4, dither: 1, invert: false })
    readonly property var hero: Object.assign({}, root.heroFallback, root.val("hero", root.heroFallback) || {})
    readonly property bool custom: root.hero.kind === "custom"
    readonly property string style: root.hero.style || "dither"
    function setHero(patch, label) {
        root.put("hero", root.heroFallback, Object.assign({}, root.hero, patch), label);
    }
    // the framing and tuning back to as chosen, the picture kept
    function resetPicture() {
        root.setHero({ zoom: 1, focalX: 0.5, focalY: 0.5, fit: false, brightness: 0, contrast: 0, saturation: 0, dither: 1, invert: false },
                     I18n.tr("Card picture"));
    }
    function round(v) {
        return Math.round(v * 1000) / 1000;
    }

    readonly property var gallery: ["david.png", "aurelius.png", "laocoon.png", "moon.png", "needle.png",
                                    "katana.png", "hawk.png", "mic.png", "camera.png", "rashin-hero.png"]
    // the parts Home's plate draws; a toggle for a part the plate does not show would be dead
    readonly property var parts: [
        { id: "epithets", label: I18n.tr("Big words") },
        { id: "telemetry", label: I18n.tr("Live readings") },
        { id: "specs", label: I18n.tr("Display, window manager and uptime") }
    ]
    // which parts each "Show" choice turns on; the choice and the parts are one change, so its
    // Undo brings back parts switched by hand
    readonly property var presets: ({
        full: { epithets: true, telemetry: true, specs: true },
        minimal: { epithets: false, telemetry: false, specs: true }
    })
    function setLayout(preset) {
        var now = root.val("layout", ({})) || {};
        if ((now.preset || "full") === preset)
            return;
        root.put("layout", ({}), { preset: preset, blocks: Object.assign({}, now.blocks || {}, root.presets[preset]) },
                 I18n.tr("Card layout"));
    }

    title: I18n.tr("Card studio")
    dialogWidth: 1280
    actions: [{ text: I18n.tr("Done"), kind: "gold" }]
    onClosed: root.previewTier = -1

    // a picked image is copied beside profile.json so it stays and can travel; a new picture
    // starts as a photo, framed to fill
    function adopt(url) {
        var path = decodeURIComponent(String(url).replace(/^file:\/\//, ""));
        var dot = path.lastIndexOf(".");
        var ext = (dot >= 0 ? path.slice(dot + 1) : "png").toLowerCase().replace(/[^a-z0-9]/g, "") || "png";
        copyProc.ext = ext;
        copyProc.command = ["install", "-D", "-m", "644", path, root.configDir + "/profile/hero." + ext];
        root.imageError = "";
        copyProc.running = true;
    }

    Process {
        id: copyProc
        property string ext: "png"
        stderr: StdioCollector { id: copyErr }
        onExited: (code, status) => {
            if (code === 0) {
                root.setHero({ kind: "custom", source: "hero." + copyProc.ext, style: "photo", fit: false, zoom: 1, focalX: 0.5, focalY: 0.5,
                               brightness: 0, contrast: 0, saturation: 0 }, I18n.tr("Card figure"));
                root.tab = "adjust";
            } else {
                root.imageError = I18n.tr("That image could not be copied: %1. Choose another, or check that it can be read.").replace("%1", copyErr.text.trim());
            }
        }
    }

    // the body takes its size from its first child, so the choosers live beside it
    ImageChooser {
        id: imagePick
        parent: root.parent
        overlayParent: root.hub ? root.hub.overlayItem : null
        title: I18n.tr("Choose a picture for the card")
        currentFolder: "file://" + Paths.picturesDir
        onAccepted: root.adopt(imagePick.selectedFile)
    }
    CardShare {
        id: share
        parent: root.parent
        hub: root.hub
        pageId: root.pageId
    }

    Item {
        width: parent.width
        implicitHeight: root.bodyHeight

        // the preview: the card, or Home's plate, as it will be
        Item {
            id: previewBox
            width: parent.width * 0.5
            height: parent.height

            Segmented {
                id: viewChoice
                anchors.horizontalCenter: parent.horizontalCenter
                enabled: root.tab !== "share"
                opacity: enabled ? 1 : 0.4
                model: [I18n.tr("The card"), I18n.tr("Home")]
                Binding on currentIndex {
                    value: root.shownView === "home" ? 1 : 0
                }
                accessibleName: I18n.tr("Preview")
                onActivated: index => root.view = index === 1 ? "home" : "card"
            }

            Item {
                id: stage
                anchors.top: viewChoice.bottom
                anchors.topMargin: LithicTokens.space.s4
                anchors.bottom: hint.top
                anchors.bottomMargin: LithicTokens.space.s3
                width: parent.width

                // the card, tilting toward the pointer as it does on Home
                Item {
                    id: cardHolder
                    visible: root.shownView === "card"
                    readonly property real scaleTo: Math.min(stage.width / liveCard.designWidth, stage.height / liveCard.designHeight)
                    width: liveCard.designWidth * scaleTo
                    height: liveCard.designHeight * scaleTo
                    anchors.centerIn: parent
                    readonly property bool pointed: drag.containsMouse && !drag.pressed && !Lithic.reduceMotion
                    Chase {
                        id: tiltX
                        target: cardHolder.pointed ? Math.max(-1, Math.min(1, (drag.mouseX / drag.width - 0.5) * 2)) : 0
                    }
                    Chase {
                        id: tiltY
                        target: cardHolder.pointed ? Math.max(-1, Math.min(1, (drag.mouseY / drag.height - 0.5) * 2)) : 0
                    }
                    transform: [
                        Rotation { origin.x: cardHolder.width / 2; origin.y: cardHolder.height / 2; axis { x: 0; y: 1; z: 0 } angle: tiltX.value * 8 },
                        Rotation { origin.x: cardHolder.width / 2; origin.y: cardHolder.height / 2; axis { x: 1; y: 0; z: 0 } angle: -tiltY.value * 6 }
                    ]
                    TradingCard {
                        id: liveCard
                        transformOrigin: Item.TopLeft
                        scale: cardHolder.scaleTo
                        foil: Qt.vector2d(tiltX.value, tiltY.value)
                        live: root.opened && (cardHolder.pointed || root.previewTier >= 0)
                        showLevel: root.previewTier
                    }
                }

                // Home's plate itself, loaded by URL and inert, so it cannot drift from what
                // Settings opens on. Home hands the studio its hub only after it is built
                CutRect {
                    id: homeFrame
                    visible: root.shownView === "home"
                    anchors.centerIn: parent
                    width: Math.min(parent.width, parent.height * 16 / 10)
                    height: width * 10 / 16
                    cut: LithicTokens.cut.small
                    fill: Lithic.ground
                    strokeColor: Lithic.vein
                    strokeWidth: LithicTokens.size.hairline
                    clip: true
                    Loader {
                        id: homePlate
                        active: root.shownView === "home" && !!root.hub
                        onActiveChanged: if (active && !item) setSource("../Home.qml", { hub: root.hub, preview: true })
                        onLoaded: {
                            item.width = item.designWidth;
                            item.height = item.designHeight;
                            item.transformOrigin = Item.TopLeft;
                            item.scale = Qt.binding(() => homeFrame.width / item.designWidth);
                        }
                    }
                }

                // your own picture is framed by hand: drag moves it, the wheel zooms it
                MouseArea {
                    id: drag
                    anchors.fill: parent
                    hoverEnabled: true
                    readonly property Item figure: root.shownView === "card" ? liveCard.figure : (homePlate.item ? homePlate.item.figureItem : null)
                    cursorShape: root.custom ? (pressed ? Qt.ClosedHandCursor : Qt.OpenHandCursor) : Qt.ArrowCursor
                    property real ox: 0
                    property real oy: 0
                    // the figure's size on screen, scales and all
                    function shown() {
                        if (!drag.figure)
                            return { w: 1, h: 1 };
                        var a = drag.figure.mapToItem(drag, 0, 0), b = drag.figure.mapToItem(drag, drag.figure.width, drag.figure.height);
                        return { w: Math.max(1, b.x - a.x), h: Math.max(1, b.y - a.y) };
                    }
                    onPressed: m => { ox = m.x; oy = m.y; }
                    onPositionChanged: m => {
                        if (!pressed || !root.custom || root.hero.zoom <= 1.001 && !root.hero.fit)
                            return;
                        var size = drag.shown(), spare = Math.max(0.05, root.hero.zoom - 1);
                        var fx = Math.max(0, Math.min(1, root.hero.focalX - (m.x - ox) / (size.w * spare)));
                        var fy = Math.max(0, Math.min(1, root.hero.focalY - (m.y - oy) / (size.h * spare)));
                        ox = m.x;
                        oy = m.y;
                        root.setHero({ focalX: root.round(fx), focalY: root.round(fy) }, I18n.tr("Card framing"));
                    }
                    onWheel: w => {
                        if (!root.custom) {
                            w.accepted = false;
                            return;
                        }
                        var z = Math.max(1, Math.min(4, root.hero.zoom * (w.angleDelta.y > 0 ? 1.08 : 1 / 1.08)));
                        root.setHero({ zoom: root.round(z) }, I18n.tr("Card zoom"));
                    }
                }
            }

            Label {
                id: hint
                anchors.bottom: parent.bottom
                width: parent.width
                horizontalAlignment: Text.AlignHCenter
                text: root.custom ? I18n.tr("Drag your picture to move it, scroll to zoom")
                    : I18n.tr("Choose your own picture to frame and adjust it")
                typeRole: "caption"
                inkRole: "fg3"
            }
        }

        // the studio's tools
        Item {
            id: tools
            x: previewBox.width + LithicTokens.space.s6
            width: parent.width - x
            height: parent.height

            Segmented {
                id: tabChoice
                width: parent.width
                model: [I18n.tr("Picture"), I18n.tr("Adjust"), I18n.tr("Words"), I18n.tr("Layout"), I18n.tr("Share")]
                Binding on currentIndex {
                    value: root.tabs.indexOf(root.tab)
                }
                accessibleName: I18n.tr("Studio")
                onActivated: index => {
                    root.tab = root.tabs[index];
                    root.previewTier = -1;
                }
            }

            Flickable {
                id: flick
                anchors.top: tabChoice.bottom
                anchors.topMargin: LithicTokens.space.s5
                anchors.bottom: parent.bottom
                width: parent.width
                contentHeight: page.implicitHeight
                boundsBehavior: Flickable.StopAtBounds
                clip: true

                Column {
                    id: page
                    width: flick.width - LithicTokens.space.s4
                    spacing: LithicTokens.space.s5

                    // Picture: the figure
                    Column {
                        visible: root.tab === "picture"
                        width: parent.width
                        spacing: LithicTokens.space.s3
                        Flow {
                            width: parent.width
                            spacing: LithicTokens.space.s2
                            Tile {
                                label: I18n.tr("Caracalla")
                                photo: true
                                art: Qt.resolvedUrl("../../art/caracalla.png")
                                chosen: root.hero.kind === "default"
                                onPicked: root.setHero({ kind: "default", source: "" }, I18n.tr("Card figure"))
                            }
                            Tile {
                                label: root.custom ? I18n.tr("Change") : I18n.tr("Your picture")
                                photo: true
                                art: root.custom && root.hero.source ? "file://" + root.configDir + "/profile/" + root.hero.source : ""
                                chosen: root.custom
                                onPicked: imagePick.open()
                            }
                            Repeater {
                                model: root.gallery
                                delegate: Tile {
                                    required property var modelData
                                    label: String(modelData).replace(/\.png$/, "").replace(/-/g, " ")
                                    art: Ryodecors.dir + modelData
                                    chosen: root.hero.kind === "gallery" && root.hero.source === modelData
                                    onPicked: root.setHero({ kind: "gallery", source: modelData }, I18n.tr("Card figure"))
                                }
                            }
                        }
                        Label {
                            width: parent.width
                            visible: root.imageError.length > 0
                            text: root.imageError
                            typeRole: "detail"
                            ink: Lithic.jasper
                            wrapMode: Text.Wrap
                            elide: Text.ElideNone
                        }
                    }

                    // Adjust: your own picture's look, framing and tone
                    Label {
                        visible: root.tab === "adjust" && !root.custom
                        width: parent.width
                        text: I18n.tr("Choose your own picture in Picture, then frame and tune it here.")
                        typeRole: "body"
                        inkRole: "fg2"
                        wrapMode: Text.Wrap
                        elide: Text.ElideNone
                    }
                    Slab {
                        visible: root.tab === "adjust" && root.custom
                        width: parent.width
                        Column {
                            width: parent.width
                            SettingRow {
                                width: parent.width
                                first: true
                                title: I18n.tr("Look")
                                detail: I18n.tr("How your picture is drawn")
                                Segmented {
                                    model: [I18n.tr("Photo"), I18n.tr("Black & white"), I18n.tr("Dots")]
                                    Binding on currentIndex {
                                        value: root.style === "photo" ? 0 : root.style === "mono" ? 1 : 2
                                    }
                                    accessibleName: I18n.tr("Look")
                                    onActivated: index => root.setHero({ style: ["photo", "mono", "dither"][index] }, I18n.tr("Card look"))
                                }
                            }
                            SettingRow {
                                width: parent.width
                                title: I18n.tr("Frame")
                                detail: I18n.tr("Fill the figure's space, or show the whole picture")
                                Segmented {
                                    model: [I18n.tr("Fill"), I18n.tr("Whole picture")]
                                    Binding on currentIndex {
                                        value: root.hero.fit === true ? 1 : 0
                                    }
                                    accessibleName: I18n.tr("Frame")
                                    onActivated: index => root.setHero({ fit: index === 1 }, I18n.tr("Card frame"))
                                }
                            }
                            Repeater {
                                model: [
                                    { key: "zoom", label: I18n.tr("Zoom"), from: 1, to: 4, fallback: 1 },
                                    { key: "focalX", label: I18n.tr("Left and right"), from: 0, to: 1, fallback: 0.5 },
                                    { key: "focalY", label: I18n.tr("Up and down"), from: 0, to: 1, fallback: 0.5 },
                                    { key: "brightness", label: I18n.tr("Brightness"), from: -0.6, to: 0.6, fallback: 0 },
                                    { key: "contrast", label: I18n.tr("Contrast"), from: -0.6, to: 0.6, fallback: 0 },
                                    { key: "saturation", label: I18n.tr("Colour"), from: -1, to: 1, fallback: 0, look: "photo" },
                                    { key: "dither", label: I18n.tr("Dot size"), from: 1, to: 6, fallback: 1, look: "dither" }
                                ]
                                delegate: SettingRow {
                                    id: tune
                                    required property var modelData
                                    visible: !modelData.look || modelData.look === root.style
                                    width: parent.width
                                    title: modelData.label
                                    readonly property real current: root.hero[modelData.key] !== undefined ? root.hero[modelData.key] : modelData.fallback
                                    changed: Math.abs(tune.current - modelData.fallback) > 0.001
                                    onResetRequested: {
                                        var p = {};
                                        p[tune.modelData.key] = tune.modelData.fallback;
                                        root.setHero(p, tune.modelData.label);
                                    }
                                    Slider {
                                        width: 200
                                        from: tune.modelData.from
                                        to: tune.modelData.to
                                        value: tune.current
                                        accessibleName: tune.modelData.label
                                        onMoved: {
                                            var p = {};
                                            p[tune.modelData.key] = root.round(value);
                                            root.setHero(p, tune.modelData.label);
                                        }
                                    }
                                }
                            }
                            SettingRow {
                                width: parent.width
                                visible: root.style === "dither"
                                title: I18n.tr("Ink the dark parts")
                                detail: I18n.tr("For a dark subject on a pale background")
                                activatable: true
                                onActivated: root.setHero({ invert: !root.hero.invert }, I18n.tr("Card ink"))
                                Toggle {
                                    checked: root.hero.invert === true
                                    Accessible.name: I18n.tr("Ink the dark parts")
                                    onToggled: root.setHero({ invert: checked }, I18n.tr("Card ink"))
                                }
                            }
                            SettingRow {
                                width: parent.width
                                title: I18n.tr("Start over")
                                detail: I18n.tr("Framing and tone back as they were, the picture kept")
                                Plaque {
                                    kind: "quiet"
                                    small: true
                                    glyph: "restart_alt"
                                    text: I18n.tr("Reset")
                                    onClicked: root.resetPicture()
                                }
                            }
                        }
                    }

                    // Words
                    Slab {
                        visible: root.tab === "words"
                        width: parent.width
                        Column {
                            width: parent.width
                            Repeater {
                                model: [
                                    { key: "text.name", label: I18n.tr("Name"), fallback: SysInfo.sysUser },
                                    { key: "text.tagline", label: I18n.tr("Line under the name"), fallback: I18n.tr("For the sake of power and beauty.") },
                                    // the first is the word carved behind the figure; the card prints it in its window
                                    { key: "text.epithets", label: I18n.tr("Big words"), fallback: "Ryoku", list: true }
                                ]
                                delegate: SettingRow {
                                    id: wordRow
                                    required property var modelData
                                    required property int index
                                    width: parent.width
                                    first: index === 0
                                    title: modelData.label
                                    TextField {
                                        width: Math.min(260, wordRow.width * 0.5)
                                        readonly property string stored: wordRow.modelData.list
                                            ? (ProfileStore.get(wordRow.modelData.key, []) || []).join(", ")
                                            : String(ProfileStore.get(wordRow.modelData.key, "") || "")
                                        text: stored
                                        placeholderText: wordRow.modelData.fallback
                                        Accessible.name: wordRow.modelData.label
                                        onEditingFinished: {
                                            if (text === stored)
                                                return;
                                            if (wordRow.modelData.list)
                                                root.put(wordRow.modelData.key, [], text.split(",").map(w => w.trim()).filter(w => w.length > 0), wordRow.modelData.label);
                                            else
                                                root.put(wordRow.modelData.key, "", text, wordRow.modelData.label);
                                        }
                                    }
                                }
                            }
                        }
                    }

                    // Layout: what Home opens on, and the plate's parts
                    Slab {
                        visible: root.tab === "layout"
                        width: parent.width
                        Column {
                            width: parent.width
                            SettingRow {
                                width: parent.width
                                first: true
                                title: I18n.tr("Home shows")
                                detail: I18n.tr("The plate with your figure, or your system card")
                                Segmented {
                                    model: [I18n.tr("The plate"), I18n.tr("The card")]
                                    Binding on currentIndex {
                                        value: root.val("homeView", "plate") === "card" ? 1 : 0
                                    }
                                    accessibleName: I18n.tr("Home shows")
                                    onActivated: index => root.put("homeView", "plate", index === 1 ? "card" : "plate", I18n.tr("Home view"))
                                }
                            }
                            SettingRow {
                                width: parent.width
                                title: I18n.tr("Show")
                                detail: I18n.tr("Everything, or just the name, figure and a few facts")
                                Segmented {
                                    model: [I18n.tr("Everything"), I18n.tr("A little")]
                                    Binding on currentIndex {
                                        value: root.val("preset", "full") === "minimal" ? 1 : 0
                                    }
                                    accessibleName: I18n.tr("Show")
                                    onActivated: index => root.setLayout(index === 1 ? "minimal" : "full")
                                }
                            }
                            SettingRow {
                                width: parent.width
                                title: I18n.tr("Figure on the")
                                Segmented {
                                    model: [I18n.tr("Left"), I18n.tr("Right")]
                                    Binding on currentIndex {
                                        value: root.val("heroSide", "right") === "left" ? 0 : 1
                                    }
                                    accessibleName: I18n.tr("Figure on the")
                                    onActivated: index => root.put("heroSide", "right", index === 0 ? "left" : "right", I18n.tr("Card figure side"))
                                }
                            }
                            Repeater {
                                model: root.parts
                                delegate: SettingRow {
                                    id: partRow
                                    required property var modelData
                                    readonly property bool on: root.val("blocks." + modelData.id, true) === true
                                    width: parent.width
                                    title: modelData.label
                                    activatable: true
                                    changed: !partRow.on
                                    onResetRequested: root.put("blocks." + modelData.id, true, true, modelData.label)
                                    onActivated: root.put("blocks." + modelData.id, true, !partRow.on, modelData.label)
                                    Toggle {
                                        checked: partRow.on
                                        Accessible.name: partRow.modelData.label
                                        onToggled: root.put("blocks." + partRow.modelData.id, true, checked, partRow.modelData.label)
                                    }
                                }
                            }
                        }
                    }

                    // Share: the card's rarity, and passing it around
                    Column {
                        visible: root.tab === "share"
                        width: parent.width
                        spacing: LithicTokens.space.s3

                        Label {
                            width: parent.width
                            text: I18n.tr("Your machine draws a %1 card").replace("%1", I18n.tr(liveCard.rarity.tier.name))
                            typeRole: "sentence"
                            font.pixelSize: 22
                            wrapMode: Text.Wrap
                            elide: Text.ElideNone
                        }
                        Label {
                            width: parent.width
                            text: liveCard.rarity.next
                                ? I18n.tr("%1 points. %2 more for %3.").replace("%1", liveCard.rarity.points).replace("%2", liveCard.rarity.toNext).replace("%3", I18n.tr(liveCard.rarity.next.name))
                                : I18n.tr("%1 points, the rarest card there is.").replace("%1", liveCard.rarity.points)
                            typeRole: "body"
                            inkRole: "fg2"
                            wrapMode: Text.Wrap
                            elide: Text.ElideNone
                        }
                        Row {
                            spacing: LithicTokens.space.s5
                            Repeater {
                                model: liveCard.rarity.parts
                                delegate: Column {
                                    required property var modelData
                                    Label {
                                        text: ({ cpu: I18n.tr("Processor"), ram: I18n.tr("Memory"), gpu: I18n.tr("Graphics"), display: I18n.tr("Display") })[modelData.id]
                                        typeRole: "caption"
                                        inkRole: "fg3"
                                    }
                                    Label {
                                        text: "+" + modelData.points
                                        typeRole: "mono"
                                        font.pixelSize: 18
                                        ink: modelData.points > 0 ? Lithic.fg : Lithic.fg3
                                    }
                                }
                            }
                        }

                        // every tier, this card's lit; resting on one shows its look
                        Column {
                            width: parent.width
                            Repeater {
                                model: Rarity.tiers
                                delegate: Item {
                                    id: rung
                                    required property var modelData
                                    required property int index
                                    readonly property bool mine: index === liveCard.rarity.level
                                    readonly property bool reached: index <= liveCard.rarity.level
                                    width: parent.width
                                    height: 28
                                    CutRect {
                                        anchors.fill: parent
                                        cut: LithicTokens.cut.small
                                        fill: rung.mine ? Tone.select : over.hovered ? Tone.hover : "transparent"
                                    }
                                    Label {
                                        x: 10
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: modelData.mark
                                        typeRole: "label"
                                        ink: rung.reached && rung.index >= 2 ? Lithic.gold : rung.reached ? Lithic.fg : Lithic.fg3
                                    }
                                    Label {
                                        x: 56
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: I18n.tr(modelData.name)
                                        typeRole: "label"
                                        font.weight: rung.mine ? Font.DemiBold : Font.Normal
                                        ink: rung.mine ? Tone.selectInk : rung.reached ? Lithic.fg : Lithic.fg3
                                    }
                                    Label {
                                        anchors.right: parent.right
                                        anchors.rightMargin: 10
                                        anchors.verticalCenter: parent.verticalCenter
                                        text: I18n.tr("%1 pts").replace("%1", modelData.min)
                                        typeRole: "monoSm"
                                        inkRole: "fg3"
                                    }
                                    HoverHandler {
                                        id: over
                                        onHoveredChanged: root.previewTier = hovered ? rung.index : (root.previewTier === rung.index ? -1 : root.previewTier)
                                    }
                                }
                            }
                        }

                        Flow {
                            width: parent.width
                            spacing: LithicTokens.space.s2
                            Plaque {
                                kind: "gold"
                                glyph: "content_copy"
                                text: I18n.tr("Copy")
                                busy: share.job === "copy"
                                onClicked: if (share.job === "") share.take("copy")
                            }
                            Plaque {
                                kind: "line"
                                glyph: "image"
                                text: I18n.tr("Save")
                                busy: share.job === "save"
                                onClicked: if (share.job === "") share.take("save")
                            }
                            Plaque {
                                kind: "quiet"
                                glyph: "collections_bookmark"
                                text: I18n.tr("Binder")
                                enabled: typeof root.binderOpener === "function"
                                onClicked: {
                                    root.close();
                                    root.binderOpener();
                                }
                            }
                            Plaque {
                                kind: "quiet"
                                glyph: "restart_alt"
                                text: I18n.tr("Back to stock")
                                onClicked: share.reset()
                            }
                        }
                        Label {
                            width: parent.width
                            visible: share.message.length > 0 || share.error.length > 0
                            text: share.error.length > 0 ? share.error : share.message
                            typeRole: "detail"
                            ink: share.error.length > 0 ? Lithic.jasper : Lithic.fg2
                            wrapMode: Text.Wrap
                            elide: Text.ElideNone
                        }
                        Plaque {
                            visible: share.error.length > 0
                            kind: "quiet"
                            small: true
                            glyph: "refresh"
                            text: I18n.tr("Retry")
                            onClicked: share.retry()
                        }
                        Label {
                            width: parent.width
                            text: I18n.tr("Send your card picture to a friend. Collect their pictures in your Binder, compare your machines, or use their picture without changing your words. Rest on a tier to see its look.")
                            typeRole: "caption"
                            inkRole: "fg3"
                            wrapMode: Text.Wrap
                            elide: Text.ElideNone
                        }
                    }
                }
            }

            ScrollRail {
                flickable: flick
                anchors.top: flick.top
                anchors.bottom: flick.bottom
                anchors.right: flick.right
            }
        }
    }

    component Tile: Item {
        id: tile
        property string label: ""
        property url art: ""
        property bool chosen: false
        property bool photo: false
        signal picked()
        width: 76
        height: 76 + caption.height + LithicTokens.space.s1
        Accessible.role: Accessible.Button
        Accessible.name: tile.label
        activeFocusOnTab: true
        Keys.onSpacePressed: tile.picked()
        Keys.onReturnPressed: tile.picked()
        Keys.onEnterPressed: tile.picked()

        FocusRing {
            x: -outset
            y: -outset
            width: plate.width + outset * 2
            height: plate.height + outset * 2
            cut: plate.cut
            keyboardFocusVisible: tile.activeFocus
        }
        CutRect {
            id: plate
            width: 76
            height: 76
            cut: LithicTokens.cut.small
            fill: tile.chosen ? Lithic.sel : hover.hovered ? Lithic.hoverPlane(Lithic.field) : Lithic.field
            strokeColor: Lithic.vein
            strokeWidth: tile.chosen ? 0 : LithicTokens.size.hairline
            Image {
                id: tileArt
                anchors.fill: parent
                anchors.margins: LithicTokens.space.s1
                visible: tile.photo && String(tile.art).length > 0
                source: tile.art
                fillMode: tile.photo ? Image.PreserveAspectCrop : Image.PreserveAspectFit
                asynchronous: true
            }
            MultiEffect {
                anchors.fill: tileArt
                visible: !tile.photo && String(tile.art).length > 0
                source: tileArt
                colorization: 1
                colorizationColor: tile.chosen ? Lithic.onSel : Lithic.fg
            }
            Glyph {
                anchors.centerIn: parent
                visible: String(tile.art).length === 0
                name: "add_photo_alternate"
                size: 24
                ink: tile.chosen ? Lithic.onSel : Lithic.fg2
            }
        }
        Label {
            id: caption
            y: plate.height + LithicTokens.space.s1
            width: tile.width
            horizontalAlignment: Text.AlignHCenter
            text: tile.label
            typeRole: "caption"
            inkRole: "fg2"
            elide: Text.ElideRight
        }
        HoverHandler {
            id: hover
            cursorShape: Qt.PointingHandCursor
        }
        TapHandler {
            onTapped: tile.picked()
        }
    }
}
