import QtQuick
import QtQuick.Effects
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"

// the profile's figure (profile.json hero). The stock one is a photograph, the Caracalla
// Farnese, drawn as it is; a gallery figure is inked in one colour so it reads on either
// ground. The reader's own picture is framed (zoom, position, fill or fit), adjusted
// (brightness, contrast, colour) and shown as a photo, in black and white, or as a 1-bit
// dither in the ink. Local GIF/WebP pictures may move; video retains its badge rule. Motion plays
// only while `live` (the card pointed at) and Settings' ambient motion runs, and rests on
// its first frame. The system card, Home and the card studio draw the same one
Item {
    id: hero

    property color ink: Lithic.fg
    property var override: null
    // The binder supplies its store context separately from the traded entry.
    property string cardsStoreRoot: ""
    property string tradedId: ""
    // the friend's signed record supplied this picture (live or kept), so its copy may be shown
    property bool recordPicture: false
    // a figure drawn at another size than its dots asks for soft sampling, or the dots beat
    property bool soft: false
    // the card is pointed at, so a moving picture may play
    property bool live: false
    readonly property string configDir: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku"
    readonly property var config: hero.override ? (hero.override.kind === "custom"
        ? { kind: "custom", file: hero.override.file, rev: hero.override.rev, style: "photo", focalX: 0.5, focalY: 0.5 } : hero.override)
        : ProfileStore.get("hero", ({}))
    readonly property string kind: hero.config.kind || "default"
    readonly property bool stock: hero.kind === "default"
    // the stock photograph is cut to its marble: whoever sizes the figure by it reads this
    readonly property real stockAspect: 665 / 874
    readonly property string src: hero.config.source || ""
    readonly property string galleryName: hero.src.replace(/\.(png|gif)$/, "")
    readonly property string gallerySource: /^[a-z0-9_-]+$/.test(hero.galleryName)
        ? Ryodecors.dir + hero.galleryName + (/\.gif$/.test(hero.src) ? ".gif" : ".png") : ""
    // a picture file is rewritten in place when it changes, and Qt would keep showing the one it
    // loaded first: the URL carries the picture's revision so a new one loads
    readonly property string ownFile: !hero.override && hero.kind === "custom" ? CardMedia.file : hero.tradedMotionFile
    readonly property string tradedMotionFile: {
        if (!hero.override || !hero.recordPicture || !/^[0-9a-f]{16}$/.test(hero.tradedId) || hero.cardsStoreRoot.charAt(0) !== "/")
            return "";
        var sha = String(hero.override.motionSha || "");
        var path = String(hero.override.motionFile || "");
        var base = hero.cardsStoreRoot + "/" + hero.tradedId + "-approved-" + sha;
        return /^[0-9a-f]{64}$/.test(sha) && (path === base + ".webp" || path === base + ".gif" || path === base + ".png") ? path : "";
    }
    readonly property bool video: /\.(mp4|webm|mkv)$/.test(hero.ownFile)
    readonly property string stillFile: !hero.override && hero.kind === "custom" ? CardMedia.poster : ""
    readonly property bool mayMove: /\.(gif|webp)$/.test(hero.ownFile) || (hero.video && !!Badge.ownBadge)
    readonly property bool playing: hero.mayMove && hero.live && hero.visible && Ambient.on
    readonly property string customSource: {
        if (hero.kind !== "custom")
            return "";
        if (!hero.override)
            return hero.stillFile !== "" ? hero.fileUrl(hero.stillFile) + "?v=" + CardMedia.digest : "";
        // Never treat a card's embedded file field as an image URL or a store root.
        if (!/^[0-9a-f]{16}$/.test(hero.tradedId) || hero.cardsStoreRoot.charAt(0) !== "/")
            return "";
        var base = hero.cardsStoreRoot + "/" + hero.tradedId;
        var path = String(hero.config.file || "");
        var rev = String(hero.config.rev || "");
        var url = path === base + ".png" || (hero.recordPicture && path === base + "-record.png") ? hero.fileUrl(path) : "";
        return url !== "" && /^[0-9a-f]{12}$/.test(rev) ? url + "?v=" + rev : url;
    }

    function fileUrl(path) {
        return "file://" + path.split("/").map(encodeURIComponent).join("/");
    }
    readonly property real zoom: hero.config.zoom !== undefined ? hero.config.zoom : 1
    readonly property real focalX: hero.config.focalX !== undefined ? hero.config.focalX : 0.5
    readonly property real focalY: hero.config.focalY !== undefined ? hero.config.focalY : 0.4
    // "photo", "mono" or "dither"; a picture chosen before there was a choice was dithered
    readonly property string style: hero.config.style || "dither"
    readonly property bool fit: hero.config.fit === true
    readonly property real brightness: hero.config.brightness || 0
    readonly property real contrast: hero.config.contrast || 0
    readonly property real saturation: hero.config.saturation || 0
    readonly property bool ready: hero.stock ? stockImage.status === Image.Ready
        : hero.kind === "custom" ? (hero.customSource !== "" && customImage.status === Image.Ready)
        : hero.kind === "gallery" ? (hero.gallerySource === "" || inked.status === Image.Ready) : true

    clip: true
    Text {
        anchors.fill: parent
        anchors.margins: 12
        visible: !hero.override && hero.kind === "custom" && CardMedia.error !== ""
        text: CardMedia.error
        textFormat: Text.PlainText
        wrapMode: Text.Wrap
        color: Lithic.fg
        font.pixelSize: 13
    }

    // "Caracalla MAN Napoli Inv6033 n01" by Marie-Lan Nguyen, CC BY 2.5: see the root NOTICE
    Image {
        id: stockImage
        anchors.fill: parent
        visible: hero.stock
        source: hero.stock ? Qt.resolvedUrl("../../art/caracalla.png") : ""
        fillMode: Image.PreserveAspectFit
        verticalAlignment: Image.AlignBottom
        smooth: true
        mipmap: true
        asynchronous: true
    }

    // your own picture, framed: zoomed about the focal point, filling the figure or fitting in it
    Item {
        id: framed
        anchors.fill: parent
        visible: false
        clip: true
        Image {
            id: customImage
            width: hero.width * hero.zoom
            height: hero.height * hero.zoom
            x: (hero.width - width) * hero.focalX
            y: (hero.height - height) * hero.focalY
            // a moving GIF's clear pixels would show the still through it
            opacity: motion.item && motion.item.ready ? 0 : 1
            source: hero.customSource
            fillMode: hero.fit ? Image.PreserveAspectFit : Image.PreserveAspectCrop
            sourceSize.width: 1600
            sourceSize.height: 1600
            smooth: true
            mipmap: true
            asynchronous: true
        }
        // loaded by URL, so a system without Qt Multimedia still draws the still
        Loader {
            id: motion
            x: customImage.x
            y: customImage.y
            width: customImage.width
            height: customImage.height
            active: hero.playing
            source: Qt.resolvedUrl("CardMotion.qml")
        }
        Binding {
            when: motion.status === Loader.Ready
            target: motion.item
            property: "url"
            value: hero.fileUrl(hero.ownFile) + "?v=" + (hero.override ? String(hero.override.motionSha || "") : CardMedia.digest)
        }
        Binding {
            when: motion.status === Loader.Ready
            target: motion.item
            property: "fit"
            value: hero.fit
        }
    }
    // and adjusted; shown as it is for a photo, or handed to the dither
    MultiEffect {
        id: adjusted
        anchors.fill: parent
        visible: hero.kind === "custom" && hero.style !== "dither"
        source: framed
        brightness: hero.brightness
        contrast: hero.contrast
        saturation: hero.style === "mono" ? -1 : hero.saturation
    }
    ShaderEffectSource {
        id: adjustedTexture
        sourceItem: hero.kind === "custom" && hero.style === "dither" ? adjusted : null
        hideSource: true
        live: true
    }
    ShaderEffect {
        anchors.fill: parent
        visible: hero.kind === "custom" && hero.style === "dither"
        property variant src: adjustedTexture
        property color ink: hero.ink
        property color accent: hero.ink
        property vector2d srcSize: Qt.vector2d(Math.max(1, width), Math.max(1, height))
        property real dotScale: hero.config.dither !== undefined ? hero.config.dither : 1
        property real develop: 1
        property real scan: 0
        property real band: 0
        property real invert: hero.config.invert === true ? 1 : 0
        fragmentShader: Qt.resolvedUrl("../../shaders/plate.frag.qsb")
    }

    // the gallery figures are already one-bit and only take the ink
    Image {
        id: inked
        anchors.fill: parent
        visible: false
        source: hero.kind === "gallery" ? hero.gallerySource : ""
        fillMode: Image.PreserveAspectFit
        sourceSize.width: 1600
        sourceSize.height: 1600
        smooth: hero.soft
        mipmap: hero.soft
        asynchronous: true
    }
    MultiEffect {
        anchors.fill: parent
        visible: hero.kind === "gallery"
        source: inked
        colorization: 1
        colorizationColor: hero.ink
    }
}
