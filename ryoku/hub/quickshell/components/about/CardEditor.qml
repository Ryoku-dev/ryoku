pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import ".."
import "../../registry/rarity.js" as Rarity
import "../../registry/stand.js" as Stand

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
    // the tab to open on
    property string tab: "picture"
    readonly property var tabs: ["picture", "adjust", "words", "layout", "share", "badge"]
    // what the preview shows; Share always shows the card it copies
    property string view: "card"
    readonly property string shownView: root.tab === "share" || root.tab === "badge" ? "card" : root.view
    readonly property string configDir: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku"
    readonly property real bodyHeight: Math.max(380, (root.parent ? root.parent.height : 800) - 2 * LithicTokens.size.dialogMargin
                                                     - root.topPadding - root.bottomPadding - 140)
    property string imageError: ""
    // the line under the pictures is a note, not a refusal
    property bool imageNote: false
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
    // a video's tile shows the first frame ryoku-hub kept beside it
    readonly property string customPicture: root.custom && /^hero\.(png|jpg|jpeg|webp|gif|bmp|avif|mp4|webm|mkv)$/.test(String(root.hero.source || ""))
        ? imagePick.urlOf(root.configDir + "/profile/" + String(root.hero.source).replace(/^hero\.(mp4|webm|mkv)$/, "hero-$1.png")) : ""
    // moving pictures are a badge holder's, the rule the backend keeps too
    readonly property bool movingAllowed: !!Badge.ownBadge
    // a card style is one badge holder's own look, set up for them rather than picked here: the
    // studio offers no choice of style, only the controls of a style the card already wears and
    // the badge's login owns. Everyone else's look is a team design
    readonly property var styleNames: ({ stand: I18n.tr("Stand") })
    readonly property var ownStyles: Badge.ownStyles.filter(style => root.styleNames[style] !== undefined)
    readonly property bool standStyle: root.ownStyles.indexOf("stand") >= 0 && root.val("style", "classic") === "stand"
    property string cutoutError: ""
    property bool cutting: false
    // Ryogami's wallpaper when it is a video: { video, path, bytes, size, limit }
    property var wallpaper: ({})
    readonly property bool wallpaperFits: root.wallpaper.video === true && root.wallpaper.bytes <= root.wallpaper.limit
    property bool adopting: false
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
    onClosed: { root.previewTier = -1; Badge.clearPreview(); }

    // the picked file goes to ryoku-hub, which reads what it is from its bytes and keeps it
    // beside profile.json; a new picture starts as a photo, framed to fill, and the cutout of
    // the last one no longer fits it
    function adopt(url) {
        var path = imagePick.localPath(url);
        if (path === "" || root.adopting)
            return;
        root.imageError = "";
        root.imageNote = false;
        root.adopting = true;
        Changes.run(["ryoku-hub", "profile", "picture", path], function (ok, out, err) {
            root.adopting = false;
            var saved = null;
            try { saved = ok ? JSON.parse(out) : null; } catch (e) { saved = null; }
            if (!saved || !/^hero\.[a-z0-9]+$/.test(String(saved.source || ""))) {
                root.imageError = root.pictureWords(err);
                return;
            }
            ProfileStore.pictureReplaced();
            if (root.hero.cutout === true)
                Changes.run(["ryoku-hub", "profile", "cutout", "--clear"], function () {});
            root.setHero({ kind: "custom", source: saved.source, style: "photo", fit: false, zoom: 1, focalX: 0.5, focalY: 0.5,
                           brightness: 0, contrast: 0, saturation: 0, cutout: false }, I18n.tr("Card figure"));
            if (saved.still) {
                root.imageError = I18n.tr("Moving pictures come with a team badge, so the card keeps the first frame.");
                root.imageNote = true;
            } else {
                root.tab = "adjust";
            }
        }, { timeout: 60000 });
    }
    // a badge holder's cutout: the figure cut from the card picture on a clear ground, which the
    // Stand card lays over its panel's border
    function adoptCutout(url) {
        var path = cutPick.localPath(url);
        if (path === "" || root.cutting)
            return;
        root.cutoutError = "";
        root.cutting = true;
        Changes.run(["ryoku-hub", "profile", "cutout", path], function (ok, out, err) {
            root.cutting = false;
            var saved = null;
            try { saved = ok ? JSON.parse(out) : null; } catch (e) { saved = null; }
            if (!saved || saved.cutout !== "hero-cutout.png") {
                root.cutoutError = root.cutoutWords(err);
                return;
            }
            ProfileStore.pictureReplaced();
            root.setHero({ cutout: true }, I18n.tr("Card cutout"));
        }, { timeout: 30000 });
    }
    function clearCutout() {
        root.cutoutError = "";
        Changes.run(["ryoku-hub", "profile", "cutout", "--clear"], function () {});
        root.setHero({ cutout: false }, I18n.tr("Card cutout"));
    }
    function cutoutWords(err) {
        var s = String(err || "").trim().replace(/^ryoku-hub:\s*/, "");
        var m = /needs your picture's shape \((\d+x\d+)\); this one is (\d+x\d+)/.exec(s);
        if (m)
            return I18n.tr("The cutout needs your picture's shape (%1); this one is %2. Cut it from the same picture.").replace("%1", m[1]).replace("%2", m[2]);
        if (/no clear ground/.test(s))
            return I18n.tr("This picture has no clear ground. A cutout keeps only the figure, on transparency.");
        if (/PNG with a clear ground/.test(s))
            return I18n.tr("A cutout is a PNG with a clear ground.");
        if (/choose your own picture first/.test(s))
            return I18n.tr("Choose your own picture first: the cutout is cut from it.");
        if (/come with a team badge/.test(s))
            return I18n.tr("A cutout comes with a team badge.");
        if ((m = /file is ([\d.]+ MB); a cutout can be up to ([\d.]+ MB)/.exec(s)))
            return I18n.tr("This file is %1. A cutout can be up to %2.").replace("%1", m[1]).replace("%2", m[2]);
        return s !== "" ? I18n.tr("That cutout could not be used: %1.").replace("%1", s) : I18n.tr("That cutout could not be used.");
    }
    // ryoku-hub's refusals, in the reader's words; the sizes it names stay as it measured them
    function pictureWords(err) {
        var s = String(err || "").trim().replace(/^ryoku-hub:\s*/, "");
        var m = /moving picture is ([\d.]+ MB); moving pictures can be up to ([\d.]+ MB)/.exec(s);
        if (m)
            return I18n.tr("This moving picture is %1. Moving pictures can be up to %2.").replace("%1", m[1]).replace("%2", m[2]);
        if ((m = /file is ([\d.]+ MB); a picture can be up to ([\d.]+ MB)/.exec(s)))
            return I18n.tr("This file is %1. A picture can be up to %2.").replace("%1", m[1]).replace("%2", m[2]);
        if ((m = /video is (\d+) seconds long; a moving picture can be up to (\d+) seconds/.exec(s)))
            return I18n.tr("This video is %1 seconds long. A moving picture can be up to %2 seconds.").replace("%1", m[1]).replace("%2", m[2]);
        if (/come with a team badge/.test(s))
            return I18n.tr("Moving pictures come with a team badge.");
        if (/named as a video|named as a picture|not what its first bytes say/.test(s))
            return I18n.tr("This file is not what its name says. Choose another.");
        if (/need (ffprobe|ffmpeg)/.test(s))
            return I18n.tr("Moving videos need ffmpeg, which Ryoku installs. Reinstall it and try again.");
        return s !== "" ? I18n.tr("That picture could not be used: %1. Choose another.").replace("%1", s)
            : I18n.tr("That picture could not be used. Choose another.");
    }
    function readWallpaper() {
        Changes.run(["ryoku-hub", "profile", "wallpaper"], function (ok, out) {
            try { root.wallpaper = ok ? JSON.parse(out) || ({}) : ({}); } catch (e) { root.wallpaper = ({}); }
        });
    }
    onTabChanged: if (root.opened && root.tab === "picture" && root.movingAllowed) root.readWallpaper()
    onOpenedChanged: if (root.opened && root.tab === "picture" && root.movingAllowed) root.readWallpaper()

    // the body takes its size from its first child, so the choosers live beside it
    ImageChooser {
        id: imagePick
        parent: root.parent
        overlayParent: root.hub ? root.hub.overlayItem : null
        title: I18n.tr("Choose a picture for the card")
        currentFolder: "file://" + Paths.picturesDir
        videos: root.movingAllowed
        onAccepted: root.adopt(imagePick.selectedFile)
    }
    ImageChooser {
        id: cutPick
        parent: root.parent
        overlayParent: root.hub ? root.hub.overlayItem : null
        title: I18n.tr("Choose a cutout of your picture's figure")
        currentFolder: "file://" + Paths.picturesDir
        onAccepted: root.adoptCutout(cutPick.selectedFile)
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
                enabled: root.tab !== "share" && root.tab !== "badge"
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
                model: [I18n.tr("Picture"), I18n.tr("Adjust"), I18n.tr("Words"), I18n.tr("Layout"), I18n.tr("Share"), I18n.tr("Badge")]
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
                    BadgeTab {
                        visible: root.tab === "badge"
                        width: parent.width
                        hub: root.hub
                        live: root.opened && root.tab === "badge"
                    }

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
                                art: root.customPicture
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
                        // moving pictures: a badge holder's, with the live wallpaper offered when
                        // it is a video within the size; everyone else is told where they come from
                        Label {
                            width: parent.width
                            text: root.movingAllowed
                                ? I18n.tr("Your picture may move: a GIF, an animated WebP or a video up to %1 seconds and 10 MB. It plays while you point at the card.").replace("%1", 60)
                                : I18n.tr("Moving pictures come with a team badge.")
                            typeRole: "detail"
                            inkRole: "fg2"
                            wrapMode: Text.Wrap
                            elide: Text.ElideNone
                        }
                        Plaque {
                            visible: root.movingAllowed && root.wallpaperFits
                            small: true
                            kind: "line"
                            glyph: "wallpaper"
                            text: I18n.tr("Use my live wallpaper")
                            busy: root.adopting
                            enabled: !root.adopting
                            onClicked: root.adopt(imagePick.urlOf(root.wallpaper.path))
                        }
                        Label {
                            width: parent.width
                            visible: root.movingAllowed && root.wallpaper.video === true && !root.wallpaperFits
                            text: I18n.tr("Your live wallpaper is %1, over the 10 MB a moving picture can be.").replace("%1", root.wallpaper.size || "")
                            typeRole: "detail"
                            inkRole: "fg2"
                            wrapMode: Text.Wrap
                            elide: Text.ElideNone
                        }
                        Label {
                            width: parent.width
                            visible: root.imageError.length > 0
                            text: root.imageError
                            typeRole: "detail"
                            ink: root.imageNote ? Lithic.fg2 : Lithic.jasper
                            wrapMode: Text.Wrap
                            elide: Text.ElideNone
                        }
                        // a badge holder's cutout of their own picture's figure, for the Stand card
                        Slab {
                            visible: root.standStyle && root.custom
                            width: parent.width
                            SettingRow {
                                width: parent.width
                                first: true
                                title: I18n.tr("Cutout for the figure breaking the frame")
                                detail: root.hero.cutout === true
                                    ? (root.standStyle ? I18n.tr("The Stand card lays it over its panel's border") : I18n.tr("Shown by the Stand card style, in Layout"))
                                    : I18n.tr("A PNG of your picture's figure on a clear ground, the picture's own shape")
                                Row {
                                    spacing: LithicTokens.space.s2
                                    Plaque {
                                        small: true
                                        kind: "line"
                                        glyph: "content_cut"
                                        text: root.hero.cutout === true ? I18n.tr("Change") : I18n.tr("Choose")
                                        busy: root.cutting
                                        enabled: !root.cutting
                                        onClicked: cutPick.open()
                                    }
                                    Plaque {
                                        visible: root.hero.cutout === true
                                        small: true
                                        kind: "quiet"
                                        glyph: "close"
                                        text: I18n.tr("Remove")
                                        onClicked: root.clearCutout()
                                    }
                                }
                            }
                        }
                        Label {
                            width: parent.width
                            visible: root.standStyle && root.custom && root.cutoutError.length > 0
                            text: root.cutoutError
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
                                ].concat(root.standStyle ? [
                                    // the Stand card names its Stand after the motto unless named here
                                    { key: "text.standName", label: I18n.tr("Stand's name"), fallback: Stand.standName(liveCard.tagline, liveCard.stockTagline) }
                                ] : [])
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

                        Slab {
                            width: parent.width
                            CardOnlineSwitch {
                                width: parent.width
                                hub: root.hub
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
                sourceSize.width: 240
                sourceSize.height: 240
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
