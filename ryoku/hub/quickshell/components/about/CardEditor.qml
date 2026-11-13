pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Io
import Lithic
import Ryoku.Ui as RyokuUi
import Ryoku.Ui.Singletons
import "../../Singletons"

// customize the system card beside a live copy of it; every change applies at
// once through Changes, with Undo
Dialog {
    id: root

    property string pageId: "about"
    // the live plate preview is Home itself; Home loads this editor by URL, so
    // hub arrives from the loader before the dialog opens
    property var hub: null
    // opened from Home: the same design, with Settings' own colours beside it
    property bool home: false
    readonly property string configDir: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku"
    readonly property bool wide: root.availableWidth >= 900
    readonly property real bodyHeight: Math.max(320, (root.parent ? root.parent.height : 800) - 2 * LithicTokens.size.dialogMargin
                                                     - root.topPadding - root.bottomPadding - 140)
    property string imageError: ""

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
    function setHero(patch, label) {
        root.put("hero", root.heroFallback, Object.assign({}, root.hero, patch), label);
    }

    readonly property var gallery: ["david.png", "aurelius.png", "laocoon.png", "moon.png", "needle.png",
                                    "katana.png", "hawk.png", "mic.png", "camera.png", "rashin-hero.png"]
    // only the blocks Home's plate draws; a toggle for a part the plate does not
    // show would be a dead switch
    readonly property var parts: [
        { id: "epithets", label: I18n.tr("Big words") },
        { id: "telemetry", label: I18n.tr("Live readings") },
        { id: "specs", label: I18n.tr("Display, window manager and uptime") }
    ]

    // which parts each "Show" choice turns on; the choice and the parts are one
    // change, so its Undo brings back parts switched by hand
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

    title: root.home ? I18n.tr("Customise Home") : I18n.tr("Customise the system card")
    dialogWidth: 1240
    actions: [{ text: I18n.tr("Done"), kind: "gold" }]

    // a picked image is copied beside profile.json so it stays and can travel
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
            if (code === 0)
                root.setHero({ kind: "custom", source: "hero." + copyProc.ext, zoom: 1, focalX: 0.5, focalY: 0.5 }, I18n.tr("Card figure"));
            else
                root.imageError = I18n.tr("That image could not be copied: %1. Choose another, or check that it can be read.").replace("%1", copyErr.text.trim());
        }
    }

    RyokuUi.FileDialogHost {
        id: imagePick
        // the body takes its size from its first child, so this one lives beside it
        parent: root.parent
        title: I18n.tr("Choose an image for the card")
        nameFilters: [I18n.tr("Images") + " (*.png *.jpg *.jpeg *.webp *.gif *.bmp)"]
        currentFolder: "file://" + Paths.picturesDir
        onAccepted: root.adopt(imagePick.selectedFile)
    }

    Item {
        width: parent.width
        implicitHeight: root.bodyHeight

        // the live card; drag frames your own image
        Item {
            id: previewBox
            width: root.wide ? parent.width * 0.58 : parent.width
            height: root.wide ? parent.height : Math.min(parent.height * 0.45, width * 10 / 16)

            CutRect {
                id: frame
                anchors.centerIn: parent
                width: Math.min(parent.width, parent.height * 16 / 10)
                height: width * 10 / 16
                cut: LithicTokens.cut.small
                fill: Lithic.ground
                strokeColor: Lithic.vein
                strokeWidth: LithicTokens.size.hairline

                Item {
                    anchors.fill: parent
                    clip: true
                    // the preview is the real plate: Home itself, loaded by URL and
                    // inert, so it cannot drift from what Settings actually opens on.
                    // Home hands this editor its hub only after the editor is built,
                    // so the plate waits for it
                    Loader {
                        id: liveCard
                        anchors.centerIn: parent
                        width: frame.width
                        height: frame.height
                        Component.onCompleted: if (root.hub) liveCard.loadPlate()
                        Connections {
                            target: root
                            function onHubChanged() {
                                if (root.hub && !liveCard.item)
                                    liveCard.loadPlate();
                            }
                        }
                        function loadPlate() {
                            setSource("../Home.qml", { hub: root.hub, preview: true });
                        }
                        onLoaded: {
                            item.width = item.designWidth;
                            item.height = item.designHeight;
                            item.transformOrigin = Item.TopLeft;
                            item.scale = frame.width / item.designWidth;
                        }
                    }
                }

                MouseArea {
                    anchors.fill: parent
                    enabled: root.hero.kind === "custom"
                    cursorShape: enabled ? Qt.OpenHandCursor : Qt.ArrowCursor
                    property real ox: 0
                    property real oy: 0
                    onPressed: m => { ox = m.x; oy = m.y; }
                    onPositionChanged: m => {
                        var fx = Math.max(0, Math.min(1, root.hero.focalX - (m.x - ox) / Math.max(1, width * 0.4)));
                        var fy = Math.max(0, Math.min(1, root.hero.focalY - (m.y - oy) / Math.max(1, height)));
                        ox = m.x;
                        oy = m.y;
                        root.setHero({ focalX: Math.round(fx * 1000) / 1000, focalY: Math.round(fy * 1000) / 1000 }, I18n.tr("Card framing"));
                    }
                }
            }
        }

        Flickable {
            id: flick
            x: root.wide ? previewBox.width + LithicTokens.space.s5 : 0
            y: root.wide ? 0 : previewBox.height + LithicTokens.space.s4
            width: root.wide ? parent.width - x : parent.width
            height: parent.height - y
            contentHeight: settings.implicitHeight
            boundsBehavior: Flickable.StopAtBounds
            clip: true

            // a control reached by Tab below the fold scrolls into view
            readonly property Item focusedItem: flick.Window.activeFocusItem
            onFocusedItemChanged: {
                var f = flick.focusedItem;
                var p = f;
                while (p && p !== settings)
                    p = p.parent;
                if (!p || !root.opened)
                    return;
                var margin = LithicTokens.space.s4;
                var top = f.mapToItem(settings, 0, 0).y - margin;
                var bottom = top + f.height + 2 * margin;
                if (top < flick.contentY)
                    flick.contentY = Math.max(0, top);
                else if (bottom > flick.contentY + flick.height)
                    flick.contentY = Math.max(0, Math.min(bottom - flick.height, flick.contentHeight - flick.height));
            }

            Column {
                id: settings
                width: flick.width - LithicTokens.space.s4
                spacing: LithicTokens.space.s5

                Section {
                    visible: root.home
                    width: parent.width
                    title: I18n.tr("Colours")
                    Slab {
                        width: parent.width
                        Column {
                            width: parent.width
                            spacing: LithicTokens.space.s4
                            Choice {
                                title: I18n.tr("Settings wears")
                                detail: I18n.tr("Ryoku's own black and bone, its light paper, or the colours of your wallpaper")
                                model: [I18n.tr("Dark"), I18n.tr("Light"), I18n.tr("Wallpaper")]
                                index: Math.max(0, Look.coloursChoices.indexOf(Changes.value(Look.colours)))
                                onPicked: at => Changes.set(Look.colours, Look.coloursChoices[at], I18n.tr("Settings colours"), root.pageId)
                            }
                            Choice {
                                title: I18n.tr("Contrast")
                                detail: I18n.tr("High sets fields and lines further from the paper and the text stronger")
                                model: [I18n.tr("Standard"), I18n.tr("High")]
                                index: Math.max(0, Look.contrastChoices.indexOf(Changes.value(Look.contrast)))
                                onPicked: at => Changes.set(Look.contrast, Look.contrastChoices[at], I18n.tr("Settings contrast"), root.pageId)
                            }
                            Choice {
                                visible: Changes.value(Look.colours) === "wallpaper"
                                title: I18n.tr("Tint")
                                detail: I18n.tr("How much of the wallpaper's colour the paper takes")
                                model: [I18n.tr("Off"), I18n.tr("Low"), I18n.tr("Medium")]
                                index: Math.max(0, Look.tintChoices.indexOf(Changes.value(Look.tint)))
                                onPicked: at => Changes.set(Look.tint, Look.tintChoices[at], I18n.tr("Settings tint"), root.pageId)
                            }
                        }
                    }
                }

                Section {
                    width: parent.width
                    title: I18n.tr("Figure")
                    Column {
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
                                label: I18n.tr("Your image")
                                photo: true
                                art: root.hero.kind === "custom" && root.hero.source ? "file://" + root.configDir + "/profile/" + root.hero.source : ""
                                chosen: root.hero.kind === "custom"
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
                        Slab {
                            width: parent.width
                            visible: root.hero.kind === "custom"
                            Column {
                                width: parent.width
                                SettingRow {
                                    width: parent.width
                                    first: true
                                    title: I18n.tr("Zoom")
                                    detail: I18n.tr("Drag the figure on the card to frame it")
                                    Slider {
                                        width: 160
                                        from: 1
                                        to: 3
                                        value: root.hero.zoom
                                        accessibleName: I18n.tr("Zoom")
                                        onMoved: root.setHero({ zoom: Math.round(value * 100) / 100 }, I18n.tr("Card zoom"))
                                    }
                                }
                                SettingRow {
                                    width: parent.width
                                    title: I18n.tr("Grain")
                                    detail: I18n.tr("How coarse the dots that draw your image are")
                                    Slider {
                                        width: 160
                                        from: 1
                                        to: 6
                                        value: root.hero.dither
                                        accessibleName: I18n.tr("Grain")
                                        onMoved: root.setHero({ dither: Math.round(value * 100) / 100 }, I18n.tr("Card grain"))
                                    }
                                }
                                SettingRow {
                                    width: parent.width
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
                            }
                        }
                    }
                }

                Section {
                    width: parent.width
                    title: I18n.tr("Words")
                    Slab {
                        width: parent.width
                        Column {
                            width: parent.width
                            Repeater {
                                model: [
                                    { key: "text.name", label: I18n.tr("Name"), fallback: SysInfo.sysUser },
                                    { key: "text.tagline", label: I18n.tr("Line under the name"), fallback: I18n.tr("For the sake of power and beauty.") },
                                    // the first is the word carved behind the figure on Home; the card turns through them all
                                    { key: "text.epithets", label: I18n.tr("Big words"), fallback: "Ryoku", list: true },
                                    { key: "text.marginalia", label: I18n.tr("Edge line"), fallback: "" }
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
                }

                Section {
                    width: parent.width
                    title: I18n.tr("Layout")
                    Slab {
                        width: parent.width
                        Column {
                            width: parent.width
                            SettingRow {
                                width: parent.width
                                first: true
                                title: I18n.tr("Show")
                                detail: I18n.tr("Everything, or just the name, figure and a few facts")
                                Segmented {
                                    model: [I18n.tr("Everything"), I18n.tr("A little")]
                                    // choosing assigns the control's own index, which would end a plain
                                    // binding: an Undo must still move it
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
                                model: root.parts.filter(p => !p.gloss || UiPreferences.showJapaneseGloss)
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

    // one choice among a few shown at once: its words, then the options under them at
    // full width, since the column beside the card is narrow
    component Choice: Column {
        id: choice
        property string title: ""
        property string detail: ""
        property var model: []
        property int index: 0
        signal picked(int at)
        width: parent ? parent.width : 0
        spacing: LithicTokens.space.s2
        Label {
            width: parent.width
            text: choice.title
            typeRole: "row"
        }
        Label {
            width: parent.width
            visible: choice.detail.length > 0
            text: choice.detail
            typeRole: "detail"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Segmented {
            width: Math.min(parent.width, implicitWidth)
            model: choice.model
            Binding on currentIndex {
                value: choice.index
            }
            accessibleName: choice.title
            onActivated: at => choice.picked(at)
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
