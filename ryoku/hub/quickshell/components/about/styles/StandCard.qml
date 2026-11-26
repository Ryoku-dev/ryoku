import QtQuick
import QtQuick.Effects
import QtQuick.Shapes
import QtQuick.Window
import Quickshell
import Lithic
import Ryoku.Ui.Singletons
import "../../../Singletons"
import "../../badge"
import ".."
import "../../../registry/stand.js" as Stand

// the system card as a Stand card, a team badge holder's style: the machine is its owner's
// Stand and its parts are the Stand's parameters, graded A to E (registry/stand.js). The face is
// a manga page: heavy inks, white gutters, slanted panels, screentone, a speech balloon, the
// rush's SFX and the menacing ゴゴゴ. The holder's own picture fills the panel; with a cutout
// of its figure, the figure breaks out over the panel's border. Everything it shows comes from
// `host`, the TradingCard that draws it, which also draws the card code over this face.
// At rest it is one still page. Pointed at, THE WORLD stops time from the pointer, time flows
// again, the chart draws in grade by grade and the page stirs; let go, the frame freezes
// yellowed and TO BE CONTINUED slides in, then it settles
Item {
    id: stand

    required property Item host
    readonly property Item figure: figure
    readonly property bool ready: figure.ready && (!seal.visible || seal.ready)
        && (!stand.cutoutShown || cutoutImage.status === Image.Ready || cutoutImage.status === Image.Error)

    // a printed object, like a badge's seal: its inks are its own, whatever the theme
    readonly property color ink: "#0f0d0c"
    readonly property color paper: "#f7f2e8"
    readonly property color purple: "#7a3ce6"
    readonly property color deep: "#2b1159"
    readonly property color magenta: "#ec2a8e"
    readonly property color cyan: "#1fc6dc"
    readonly property color yellow: "#dcc63a"
    readonly property color orange: "#f25623"
    readonly property string cjk: "Noto Sans CJK JP"

    // the share still and the record's picture are drawn at twice the design size
    readonly property real res: Math.min(3, Math.max(1, stand.host.scale) * (Screen.devicePixelRatio || 1)) * 2
    // the record's picture is cut from this card's window, so that still keeps the window clean
    readonly property bool plain: !stand.host.sealed

    readonly property string configDir: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku"
    readonly property bool own: !stand.host.traded
    // the figure cut from the holder's own still picture, laid over the panel's border
    readonly property bool cutoutShown: stand.own && !stand.plain && stand.host.heroKind === "custom" && stand.host.hero.cutout === true
        && !figure.mayMove
    readonly property var grades: Stand.of({
        gpuClass: stand.host.hw ? stand.host.hw.gpuClass : CardCode.graphicsClass,
        threads: stand.host.threads, width: stand.host.screenWidth, ramGb: stand.host.ramGb,
        refresh: stand.host.refresh, points: stand.host.tier.min
    })
    // the balloon speaks the card's motto (a friend's own, from their badge-vouched record), or
    // Ryoku's when there is none
    readonly property string words: String(stand.host.tagline).trim() !== "" ? stand.host.tagline : stand.host.stockTagline
    // the holder's own name for the Stand: the Words field here, the record's on a friend's card;
    // without one the Stand is named after the motto
    readonly property string standName: {
        var named = String(stand.own ? stand.host.f("text.standName", "") : (stand.host.traded.standName || "")).trim();
        return named !== "" ? named : Stand.standName(stand.words, stand.host.stockTagline);
    }
    readonly property string partLine: stand.own
        ? "第" + Stand.part(Updates.currentVersion) + "部  PART " + Stand.part(Updates.currentVersion) + " · " + String(stand.host.release).toUpperCase()
        : I18n.tr("Traded card").toUpperCase()

    // ---------------------------------------------------------------- the clock
    // seconds since the pointer arrived (held when it leaves) and since it left; -1 at rest
    property real enterT: -1
    property real leaveT: -1
    property point pointer: Qt.point(315, 440)
    readonly property bool pointed: stand.enterT >= 0
    readonly property bool leaving: stand.leaveT >= 0
    function clamp01(x) { return Math.max(0, Math.min(1, x)); }
    function smooth(a, b, x) { var t = stand.clamp01((x - a) / (b - a)); return t * t * (3 - 2 * t); }
    function outCubic(t) { t = stand.clamp01(t); return 1 - Math.pow(1 - t, 3); }
    function inCubic(t) { t = stand.clamp01(t); return t * t * t; }
    function outBack(t) { t = stand.clamp01(t); return 1 + 2.9 * Math.pow(t - 1, 3) + 1.9 * Math.pow(t - 1, 2); }
    function hash(x) { var s = Math.sin(x * 127.1 + 311.7) * 43758.5453; return s - Math.floor(s); }

    // THE WORLD: the sphere opens from the pointer (0.35 s), holds, and closes as time moves again
    readonly property real sphere: {
        var e = stand.enterT;
        if (e < 0)
            return 0;
        var r = e < 0.35 ? stand.outCubic(e / 0.35) : e < 0.95 ? 1 : 1 - stand.inCubic((e - 0.95) / 0.3);
        if (stand.leaving)
            r *= 1 - stand.smooth(0, 0.2, stand.leaveT);
        return Math.max(0, r) * 1150;
    }
    // the page stirs once time moves again, and stops dead when the pointer leaves
    readonly property real idleT: Math.max(0, stand.enterT - 1.25)
    readonly property real motion: stand.pointed
        ? stand.smooth(1.25, 1.55, stand.enterT) * (stand.leaving ? 1 - stand.smooth(0.75, 1.1, stand.leaveT) : 1) : 0
    readonly property int beat: Math.floor(stand.idleT * 12)
    readonly property real jitter: stand.motion * (stand.hash(stand.beat * 1.7) > 0.55 ? 1 : 0.35)
    readonly property real sepia: stand.leaving ? stand.smooth(0, 0.1, stand.leaveT) * (1 - stand.smooth(0.8, 1.1, stand.leaveT)) : 0
    readonly property real arrowShift: !stand.pointed ? 0
        : stand.leaving ? 440 * (1 - stand.outBack((stand.leaveT - 0.08) / 0.36))
        : 440 * stand.inCubic(stand.enterT / 0.28)
    function grow(i) {
        if (!stand.pointed || stand.enterT < 0.95)
            return 1;
        var g = stand.outBack((stand.enterT - 1.0 - i * 0.12) / 0.14);
        return stand.leaving ? Math.max(g, stand.smooth(0.75, 1.1, stand.leaveT)) : g;
    }
    function slam(i) {
        if (!stand.pointed || stand.enterT < 0.95)
            return 1;
        var s = stand.clamp01((stand.enterT - 1.05 - i * 0.12) / 0.1);
        return stand.leaving ? Math.max(s, stand.smooth(0.75, 1.1, stand.leaveT)) : s;
    }

    // frames run only while pointed at and through the leave; at rest and with motion reduced
    // nothing runs and the still page shows
    readonly property bool wanted: stand.host.moving && !Lithic.reduceMotion
    FrameAnimation {
        running: !Lithic.reduceMotion && (stand.wanted || (stand.leaving && stand.leaveT < 1.1))
        onTriggered: {
            if (stand.wanted) {
                stand.enterT += frameTime;
                // the sphere opens where the pointer came in, read as it settles
                if (stand.enterT < 0.12)
                    stand.pointer = Qt.point((stand.host.foil.x + 1) / 2 * 630, (stand.host.foil.y + 1) / 2 * 880);
            } else {
                stand.leaveT += frameTime;
                if (stand.leaveT >= 1.1) {
                    stand.enterT = -1;
                    stand.leaveT = -1;
                }
            }
        }
    }
    onWantedChanged: {
        if (Lithic.reduceMotion) {
            stand.enterT = -1;
            stand.leaveT = -1;
        } else if (stand.wanted) {
            stand.enterT = 0;
            stand.leaveT = -1;
        } else if (stand.pointed) {
            stand.leaveT = 0;
        }
    }

    width: 630
    height: 880

    component Mono: Text {
        textFormat: Text.PlainText
        renderType: Text.QtRendering
        color: stand.ink
        font.family: Lithic.font.mono
        font.weight: Font.Bold
        font.pixelSize: 8
        font.letterSpacing: 1.2
    }
    component Kanji: Text {
        textFormat: Text.PlainText
        renderType: Text.QtRendering
        color: stand.ink
        font.family: stand.cjk
        font.weight: Font.Black
    }
    component Heavy: Text {
        textFormat: Text.PlainText
        renderType: Text.QtRendering
        color: stand.ink
        font.family: Lithic.font.sans
        font.weight: Font.Black
        font.italic: true
    }
    component Poly: Shape {
        id: poly
        property var points: []
        property color fill: stand.ink
        property color line: "transparent"
        property real lineWidth: 0
        preferredRendererType: Shape.CurveRenderer
        ShapePath {
            fillColor: poly.fill
            strokeColor: poly.line
            strokeWidth: poly.lineWidth
            joinStyle: ShapePath.MiterJoin
            PathPolyline { path: poly.points.length ? poly.points.concat([poly.points[0]]) : [] }
        }
    }
    // a parameter's tag: its name beside its grade, on a slanted chip
    component Grade: Item {
        id: chip
        property int stat: 0
        property color fill: stand.purple
        readonly property string ja: Stand.params[chip.stat].ja
        width: 80
        height: 32
        Poly {
            points: [Qt.point(6, 0), Qt.point(80, 0), Qt.point(74, 32), Qt.point(0, 32)]
            fill: chip.fill
            line: stand.ink
            lineWidth: 2
        }
        Kanji {
            x: 9
            y: 4
            text: chip.ja
            color: stand.paper
            font.pixelSize: chip.ja.length > 4 ? 7 : 9
            font.weight: Font.Bold
        }
        Mono {
            x: 7
            y: 20
            text: I18n.tr(Stand.params[chip.stat].en).toUpperCase().split(" ")[0]
            color: stand.paper
            font.pixelSize: 5
            font.letterSpacing: 0.6
        }
        Heavy {
            x: 70 - width
            y: 29 - baselineOffset
            text: Stand.letter(stand.grades[chip.stat])
            color: stand.paper
            font.pixelSize: 29
            style: Text.Outline
            styleColor: stand.ink
        }
    }
    component Star: Poly {
        property real reach: 17
        points: {
            var p = [];
            for (var i = 0; i < 10; i++) {
                var a = (-90 + i * 36) * Math.PI / 180;
                var r = i % 2 ? reach * 0.435 : reach;
                p.push(Qt.point(Math.cos(a) * r, Math.sin(a) * r));
            }
            return p;
        }
    }

    // ================================================================ the page
    Item {
        id: face
        width: 630
        height: 880
        layer.enabled: stand.sphere > 0 || stand.sepia > 0
        layer.textureSize: Qt.size(630 * stand.res / 2, 880 * stand.res / 2)
        layer.smooth: true
        layer.effect: ShaderEffect {
            property point size: Qt.point(630, 880)
            property point centre: stand.pointer
            property real radius: stand.sphere
            property real sepia: stand.sepia
            property real res: stand.res / 2
            fragmentShader: Qt.resolvedUrl("../../../shaders/stand-world.frag.qsb")
        }

        // an ink slab with Ryoku's cut corners, the page laid in it
        Poly {
            points: [Qt.point(22, 0), Qt.point(608, 0), Qt.point(630, 22), Qt.point(630, 858), Qt.point(608, 880),
                     Qt.point(22, 880), Qt.point(0, 858), Qt.point(0, 22)]
            fill: stand.ink
        }
        Poly {
            points: [Qt.point(24, 7), Qt.point(606, 7), Qt.point(623, 24), Qt.point(623, 856), Qt.point(606, 873),
                     Qt.point(24, 873), Qt.point(7, 856), Qt.point(7, 24)]
            fill: stand.paper
        }

        // ------------------------------------------------------------ the title panel
        StandTone {
            x: 16; y: 16; width: 598; height: 88
            ink: stand.purple
            ramp: Qt.vector4d(150, 0, 598, 0)
            cover: Qt.point(0, 0.62)
            pitch: 4.4
            res: stand.res
        }
        Poly {
            points: [Qt.point(14, 101), Qt.point(616, 92), Qt.point(616, 106), Qt.point(14, 106)]
            fill: stand.paper
        }
        Poly {
            points: [Qt.point(30, 16), Qt.point(600, 16), Qt.point(614, 30), Qt.point(614, 92), Qt.point(16, 101), Qt.point(16, 30)]
            fill: "transparent"
            line: stand.ink
            lineWidth: 3
        }

        // ------------------------------------------------------------ the picture panel
        Rectangle {
            x: 31.5; y: 108.5
            width: 567; height: 397
            color: stand.ink
        }
        Item {
            id: panel
            x: 35; y: 112
            width: 560; height: 390
            clip: true
            Rectangle {
                anchors.fill: parent
                color: stand.paper
            }
            // a figure that is not a photograph stands on a toned ground, as a panel's
            StandTone {
                visible: stand.host.heroKind !== "custom"
                anchors.fill: parent
                ink: stand.purple
                radial: 1
                ramp: Qt.vector4d(280, 250, 420, 0)
                cover: Qt.point(-0.15, 0.55)
                pitch: 4.4
                res: stand.res
            }
            Item {
                id: art
                anchors.fill: parent
                CardFigure {
                    id: figure
                    override: stand.host.traded ? stand.host.traded.hero : null
                    cardsStoreRoot: stand.host.cardsStoreRoot
                    tradedId: stand.host.traded ? String(stand.host.traded.id || "") : ""
                    recordPicture: stand.host.recordPicture
                    live: stand.host.moving
                    soft: true
                    ink: stand.ink
                    readonly property bool fills: stand.host.heroKind === "custom"
                    anchors.horizontalCenter: parent.horizontalCenter
                    anchors.bottom: parent.bottom
                    height: fills ? parent.height : parent.height - 10
                    width: fills ? parent.width : Math.round(height * (stand.host.heroKind === "default" ? stockAspect : 900 / 1157))
                }
            }
            // pulled apart into red and cyan while the page stirs; at rest the picture is as it is
            ShaderEffectSource {
                id: artTexture
                sourceItem: stand.jitter > 0 ? art : null
                hideSource: stand.jitter > 0
                live: true
            }
            ShaderEffect {
                anchors.fill: parent
                visible: stand.jitter > 0
                property var source: artTexture
                property point split: Qt.point(stand.jitter * (stand.hash(stand.beat * 3.1) - 0.3) * 0.006,
                                               stand.jitter * (stand.hash(stand.beat * 5.3) - 0.5) * 0.002)
                fragmentShader: Qt.resolvedUrl("../../../shaders/stand-split.frag.qsb")
            }
            // focus lines, behind a cut-out figure only: over a whole picture they would cover it
            Shape {
                visible: stand.cutoutShown
                opacity: 0.55 + 0.3 * stand.jitter
                x: panel.width * figure.focalX
                y: panel.height * figure.focalY
                rotation: stand.motion * (stand.hash(stand.beat * 0.9) - 0.5) * 1.6
                transformOrigin: Item.TopLeft
                preferredRendererType: Shape.CurveRenderer
                ShapePath {
                    fillColor: stand.ink
                    strokeColor: "transparent"
                    PathSvg {
                        path: {
                            var s = "";
                            var n = 96;
                            for (var i = 0; i < n; i++) {
                                var a = (i + stand.hash(i * 1.37) * 0.9) / n * 2 * Math.PI;
                                var r0 = 150 + stand.hash(i * 4.1) * 110;
                                var w = 0.003 + stand.hash(i * 7.9) * 0.011;
                                s += "M" + (Math.cos(a) * r0).toFixed(2) + " " + (Math.sin(a) * r0).toFixed(2)
                                    + "L" + (Math.cos(a - w) * 900).toFixed(2) + " " + (Math.sin(a - w) * 900).toFixed(2)
                                    + "L" + (Math.cos(a + w) * 900).toFixed(2) + " " + (Math.sin(a + w) * 900).toFixed(2) + "Z";
                            }
                            return s;
                        }
                    }
                }
            }
            // the Stand behind its user: the figure again, in Star Platinum's purple, a step back
            MultiEffect {
                visible: stand.cutoutShown
                readonly property real pulse: 0.5 + 0.5 * Math.sin(stand.idleT * 3.2 * 2 * Math.PI)
                x: cutout.x - panel.x - 18 - stand.motion * 10 * pulse
                y: cutout.y - panel.y - 14 - stand.motion * 8 * pulse
                width: cutout.width
                height: cutout.height
                source: cutoutImage
                colorization: 1
                colorizationColor: stand.purple
                brightness: -0.1
                opacity: 0.55 + 0.3 * stand.motion * pulse
            }
        }

        // ------------------------------------------------------------ the figure breaking out
        // above, left and right of the panel, never below it, where the card's code runs
        Item {
            id: breakout
            visible: stand.cutoutShown && cutoutImage.status === Image.Ready
            x: 9; y: 64
            width: 612; height: 502 - 64
            clip: true
            Image {
                id: cutoutImage
                visible: false
                source: stand.cutoutShown && CardMedia.cutout !== "" ? figure.fileUrl(CardMedia.cutout) + "?v=" + ProfileStore.pictureRevision : ""
                sourceSize.width: 1600
                smooth: true
                mipmap: true
                asynchronous: true
            }
            // the whole picture's place: framed as the figure frames it, cropped parts included
            Item {
                id: cutout
                readonly property real aspect: cutoutImage.implicitHeight > 0 ? cutoutImage.implicitWidth / cutoutImage.implicitHeight : 1
                readonly property real boxW: panel.width * figure.zoom
                readonly property real boxH: panel.height * figure.zoom
                readonly property bool wide: cutout.boxW / cutout.boxH > cutout.aspect
                readonly property real w: (cutout.wide !== figure.fit) ? cutout.boxW : cutout.boxH * cutout.aspect
                readonly property real h: (cutout.wide !== figure.fit) ? cutout.boxW / cutout.aspect : cutout.boxH
                x: panel.x + (panel.width - cutout.boxW) * figure.focalX + (cutout.boxW - cutout.w) / 2 - breakout.x
                y: panel.y + (panel.height - cutout.boxH) * figure.focalY + (cutout.boxH - cutout.h) / 2 - breakout.y
                width: cutout.w
                height: cutout.h
            }
            // a paper edge where the figure crosses the border, as a letterer leaves it, only
            // outside the panel
            Repeater {
                model: [Qt.rect(0, 0, 612, 112 - 64 - 4), Qt.rect(0, 0, 35 - 9 - 4, 438), Qt.rect(595 - 9 + 4, 0, 30, 438)]
                delegate: Item {
                    required property rect modelData
                    x: modelData.x; y: modelData.y
                    width: modelData.width; height: modelData.height
                    clip: true
                    MultiEffect {
                        x: cutout.x - parent.x
                        y: cutout.y - parent.y
                        width: cutout.width
                        height: cutout.height
                        source: cutoutImage
                        colorization: 1
                        colorizationColor: stand.paper
                        brightness: 1
                        blurEnabled: true
                        blur: 0.35
                        blurMax: 12
                        blurMultiplier: 1
                    }
                }
            }
            MultiEffect {
                x: cutout.x
                y: cutout.y
                width: cutout.width
                height: cutout.height
                source: cutoutImage
                brightness: figure.brightness
                contrast: figure.contrast
                saturation: figure.style === "photo" ? figure.saturation : -1
            }
        }

        // ------------------------------------------------------------ the title
        Item {
            id: title
            x: 22; y: 16
            transform: Matrix4x4 { matrix: Qt.matrix4x4(1, -0.16, 0, 14, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1) }
            StandInked {
                id: titleName
                y: -4
                text: stand.host.name.toUpperCase()
                size: 52
                stretch: 1.12
                spacing: -1
                fitWidth: 190
                fill: stand.paper
                line: stand.ink
                lineWidth: 3.2
                shade: stand.magenta
                depth: 7
            }
            StandInked {
                x: titleName.width - 6; y: 2
                kana: true
                text: "の奇妙な機械"
                size: 31
                spacing: -1
                fill: stand.yellow
                line: stand.ink
                lineWidth: 2.6
                shade: stand.ink
                depth: 4
            }
            StandInked {
                x: titleName.width - 4; y: 42
                text: Stand.title(stand.host.name)
                size: 19.5
                spacing: 0.4
                fitWidth: 430 - titleName.width
                fill: stand.paper
                line: stand.ink
                lineWidth: 2.2
                shade: stand.purple
                depth: 3.5
            }
        }
        Item {
            x: 24; y: 87
            width: partText.width + 18; height: 13
            Poly {
                points: [Qt.point(3, 0), Qt.point(parent.width, 0), Qt.point(parent.width - 3, 13), Qt.point(0, 13)]
                fill: stand.ink
            }
            Mono {
                id: partText
                x: 9; y: 2.5
                text: stand.partLine
                color: stand.paper
                font.pixelSize: 7
            }
        }

        // ------------------------------------------------------------ ゴゴゴ and ドドド
        Repeater {
            model: [
                // x, y, size, rotation, phase, glyph, over the window
                [572, 272, 46, -14, 0.35, "ゴ", true], [592, 336, 34, 10, 0.62, "ゴ", true], [566, 402, 52, -8, 0.48, "ゴ", true],
                [594, 470, 30, 14, 0.71, "ゴ", true], [476, 28, 30, -12, 0.55, "ド", false], [452, 64, 22, 8, 0.4, "ド", false],
                [596, 762, 30, 12, 0.5, "ド", false], [580, 800, 22, -10, 0.66, "ド", false]
            ]
            delegate: StandInked {
                id: menace
                required property var modelData
                readonly property real s: (stand.idleT / 1.6 + modelData[4]) % 1
                readonly property real throb: Math.pow(Math.max(0, Math.sin((stand.idleT * 1.9 + modelData[4]) * 2 * Math.PI)), 6)
                visible: !stand.plain || !modelData[6]
                kana: true
                text: modelData[5]
                size: modelData[2]
                fill: stand.purple
                line: stand.ink
                shade: stand.magenta
                lineWidth: modelData[2] * 0.06
                depth: modelData[2] * 0.14
                x: modelData[0] - width / 2
                y: modelData[1] - height / 2 - stand.motion * 22 * (menace.s - modelData[4])
                rotation: modelData[3]
                scale: 1 + stand.motion * 0.12 * menace.throb
                opacity: 1 - stand.motion * (1 - Math.pow(Math.sin(Math.PI * menace.s), 0.35))
                z: modelData[1] > 700 ? 1 : 0
            }
        }

        // ------------------------------------------------------------ the speech balloon
        Item {
            id: balloon
            visible: !stand.plain
            x: 424; y: 150
            width: 172; height: 88
            readonly property real ecx: 86
            readonly property real ecy: 44
            readonly property real rx: 85
            readonly property real ry: 43
            function rim(deg) {
                var a = deg * Math.PI / 180;
                return Qt.point(balloon.ecx + balloon.rx * Math.cos(a), balloon.ecy + balloon.ry * Math.sin(a));
            }
            Shape {
                anchors.fill: parent
                preferredRendererType: Shape.CurveRenderer
                ShapePath {
                    fillColor: stand.paper
                    strokeColor: stand.ink
                    strokeWidth: 2.4
                    joinStyle: ShapePath.MiterJoin
                    startX: balloon.rim(150).x; startY: balloon.rim(150).y
                    // the tail's tip, in the air beside the speaker
                    PathLine { x: -36; y: 86 }
                    PathLine { x: balloon.rim(166).x; y: balloon.rim(166).y }
                    PathArc {
                        x: balloon.rim(150).x; y: balloon.rim(150).y
                        radiusX: balloon.rx; radiusY: balloon.ry
                        useLargeArc: true
                        direction: PathArc.Clockwise
                    }
                }
            }
            Column {
                x: balloon.ecx - width / 2
                y: balloon.ecy - height / 2
                spacing: 1
                Heavy {
                    width: 120
                    horizontalAlignment: Text.AlignHCenter
                    wrapMode: Text.WordWrap
                    maximumLineCount: 4
                    elide: Text.ElideRight
                    text: stand.words.toUpperCase()
                    font.pixelSize: text.length > 44 ? 10 : 12
                    lineHeight: 0.9
                }
                Kanji {
                    visible: stand.words === stand.host.stockTagline
                    anchors.horizontalCenter: parent.horizontalCenter
                    text: "力と美のために"
                    font.pixelSize: 9
                    font.weight: Font.Bold
                }
            }
        }

        // ------------------------------------------------------------ the rush SFX, in its burst
        Poly {
            visible: !stand.plain
            x: 148; y: 446
            rotation: -11
            scale: 1 + stand.jitter * 0.05 * stand.hash(stand.beat * 6.1)
            transformOrigin: Item.TopLeft
            fill: stand.paper
            line: stand.ink
            lineWidth: 2.4
            points: {
                var p = [];
                for (var i = 0; i < 30; i++) {
                    var a = i / 30 * 2 * Math.PI;
                    var out = i % 2 === 0;
                    var j = out ? 1 + stand.hash(i * 2.9) * 0.22 : 0.9 + stand.hash(i * 1.3) * 0.06;
                    p.push(Qt.point(Math.cos(a) * (out ? 128 : 108) * j, Math.sin(a) * (out ? 50 : 34) * j));
                }
                return p;
            }
        }
        StandInked {
            visible: !stand.plain
            x: 26 + stand.jitter * (stand.hash(stand.beat * 2.3) - 0.5) * 5
            y: 404 + stand.jitter * (stand.hash(stand.beat * 4.7) - 0.5) * 5
            rotation: -11
            kana: true
            text: "オラオラオラ"
            size: 40
            spacing: -3
            stretch: 1.1
            fill: stand.cyan
            line: stand.ink
            shade: stand.ink
            lineWidth: 2.8
            depth: 5
        }

        // who made the picture, up the right margin
        Mono {
            x: 618; y: 498
            width: 380
            rotation: -90
            transformOrigin: Item.TopLeft
            text: ("Illus. " + stand.host.illustrator).toUpperCase()
            elide: Text.ElideRight
            font.pixelSize: 5
            font.letterSpacing: 1.1
            opacity: 0.7
        }

        // ------------------------------------------------------------ the Stand user
        Poly {
            points: [Qt.point(22, 512), Qt.point(614, 512), Qt.point(606, 543), Qt.point(16, 543)]
            fill: stand.ink
        }
        Kanji {
            x: 30; y: 516
            text: "「スタンド使い」"
            color: stand.paper
            font.pixelSize: 9
            font.weight: Font.Bold
        }
        Mono {
            x: 34; y: 530
            text: I18n.tr("Stand user").toUpperCase()
            color: stand.yellow
            font.pixelSize: 6
        }
        Heavy {
            x: 104
            y: 541 - baselineOffset
            width: 88
            text: stand.host.name
            color: stand.paper
            font.pixelSize: 31
            fontSizeMode: Text.HorizontalFit
            minimumPixelSize: 9
            elide: Text.ElideRight
        }
        Mono {
            x: 198; y: 518
            width: 300
            text: stand.host.typeLine.toUpperCase()
            elide: Text.ElideRight
            color: stand.paper
            font.pixelSize: 7
            font.letterSpacing: 0.9
        }
        Mono {
            x: 198; y: 531
            width: 300
            text: stand.host.traits.map(t => t.name + " " + (stand.host.known(t.value) ? t.value : "-")).join("  ·  ").toUpperCase()
            elide: Text.ElideRight
            color: stand.paper
            opacity: 0.7
            font.pixelSize: 5
            font.letterSpacing: 1
        }
        Heavy {
            id: powerNumber
            x: 600 - width
            y: 541 - baselineOffset
            text: String(stand.host.power)
            color: stand.paper
            font.pixelSize: 31
        }
        Kanji {
            anchors.right: powerNumber.left
            anchors.rightMargin: 3
            y: 518
            text: "力"
            color: stand.orange
            font.pixelSize: 21
        }

        // ------------------------------------------------------------ the machine, graded
        // each part in the band under its ruler (the host draws the rulers over this page)
        Repeater {
            model: stand.host.moves
            delegate: Item {
                id: row
                required property var modelData
                required property int index
                // a part's own parameter, and the display's resolution as its range
                readonly property int stat: ({ memory: 1, monitor: 4, developer_board: 3 })[row.modelData.glyph]
                x: 40
                y: (stand.host.code ? stand.host.code.ys : [550, 608, 666])[Math.min(row.index, 2)] + 15
                width: 552
                height: 38
                Mono {
                    y: 1
                    text: row.modelData.name.toUpperCase()
                    font.pixelSize: 7
                    font.letterSpacing: 2
                }
                Text {
                    id: detail
                    y: 11
                    width: Math.min(implicitWidth, 300)
                    textFormat: Text.PlainText
                    renderType: Text.QtRendering
                    text: row.modelData.detail
                    elide: Text.ElideRight
                    color: stand.ink
                    font.family: Lithic.font.sans
                    font.weight: Font.Medium
                    font.pixelSize: 17
                }
                Grade {
                    visible: row.modelData.glyph === "monitor"
                    x: detail.width + 12; y: 2
                    stat: 2
                    fill: stand.purple
                }
                Heavy {
                    id: count
                    anchors.right: unit.left
                    anchors.rightMargin: 5
                    y: 36 - baselineOffset
                    text: String(row.modelData.value)
                    font.pixelSize: 36
                }
                Mono {
                    id: unit
                    x: 552 - 90 - implicitWidth
                    anchors.baseline: count.baseline
                    text: String(row.modelData.unit).toUpperCase()
                    font.pixelSize: 8
                }
                Grade {
                    x: 552 - 80; y: 3
                    stat: row.stat
                    fill: [stand.purple, stand.cyan, stand.purple, stand.deep, stand.magenta, stand.purple][row.stat]
                }
            }
        }

        // ------------------------------------------------------------ the chart | the Stand's name
        StandTone {
            x: 16; y: 725; width: 236; height: 141
            ink: stand.cyan
            radial: 1
            ramp: Qt.vector4d(110, 72, 150, 0)
            cover: Qt.point(-1, 0.85)
            res: stand.res
        }
        StandTone {
            x: 236; y: 722; width: 378; height: 144
            ink: stand.purple
            ramp: Qt.vector4d(378, 0, 140, 120)
            cover: Qt.point(0.55, 0)
            res: stand.res
        }
        Poly {
            points: [Qt.point(246.5, 720), Qt.point(257.5, 720), Qt.point(242.5, 869), Qt.point(231.5, 869)]
            fill: stand.paper
        }
        Poly {
            points: [Qt.point(16, 727), Qt.point(245, 725), Qt.point(230, 866), Qt.point(16, 866)]
            fill: "transparent"
            line: stand.ink
            lineWidth: 3
        }
        Poly {
            points: [Qt.point(259, 725), Qt.point(614, 722), Qt.point(614, 852), Qt.point(600, 866), Qt.point(244, 866)]
            fill: "transparent"
            line: stand.ink
            lineWidth: 3
        }
        StandChart {
            x: 0; y: 0
            width: 630; height: 880
            cx: 126; cy: 797
            radius: 40
            grades: stand.grades
            grow: stand.grow
            slam: stand.slam
            ink: stand.ink
            paper: stand.paper
            fill: Qt.rgba(stand.magenta.r, stand.magenta.g, stand.magenta.b, 0.88)
            strong: stand.purple
            weak: stand.magenta
        }
        Rectangle { x: 268; y: 725; width: standLabel.width + 6; height: 12; color: stand.paper }
        Kanji {
            id: standLabel
            x: 271; y: 726
            text: "「スタンド名」 " + I18n.tr("Stand").toUpperCase()
            font.pixelSize: 8
            font.weight: Font.Bold
        }
        Row {
            x: 262; y: 738
            spacing: -6
            StandInked { kana: true; text: "『"; size: 30; fill: stand.ink; lineWidth: 0; depth: 0 }
            StandInked {
                y: 1
                text: stand.standName
                size: 28
                fitWidth: 290
                fill: stand.magenta
                line: stand.ink
                shade: stand.ink
                lineWidth: 2.4
                depth: 4
            }
            StandInked { kana: true; text: "』"; size: 30; fill: stand.ink; lineWidth: 0; depth: 0 }
        }
        Rectangle { x: 266; y: 780; width: legend.width + 8; height: legend.height + 4; color: stand.paper }
        Column {
            id: legend
            x: 270; y: 782
            spacing: 3
            Kanji {
                text: Stand.grades.map(g => g.key + " " + g.ja).join("  ")
                font.pixelSize: 7
                font.weight: Font.Bold
                font.letterSpacing: 0.4
            }
            Mono {
                lineHeight: 1.25
                text: "A·B·C·D " + I18n.tr("at least").toUpperCase() + "   "
                    + Stand.params.map((p, i) => p.ja + " " + p.from.toUpperCase() + " " + p.floors.slice().reverse().join("·")
                                       + (i % 2 === 1 ? "\n" : "   ")).join("").trim() + "   " + I18n.tr("else E").toUpperCase()
                font.pixelSize: 5
                font.letterSpacing: 0.5
            }
        }
        // the page's own copy of the card's number, for when the arrow is away
        Mono {
            x: 416; y: 846
            text: "No." + stand.host.serial + " · " + stand.host.tier.mark + " " + I18n.tr(stand.host.tier.name).toUpperCase()
                + " · " + I18n.tr("%1 pts").replace("%1", stand.host.rarity.points).toUpperCase()
            font.pixelSize: 7
            font.letterSpacing: 0.5
        }

        // ------------------------------------------------------------ the seal, a sticker
        Item {
            visible: seal.visible
            x: 506; y: 30
            width: 104; height: 104
            rotation: 9
            Rectangle {
                x: -2; y: -2
                width: 116; height: 116; radius: 58
                color: stand.ink
            }
            Rectangle {
                x: -6; y: -6
                width: 116; height: 116; radius: 58
                color: stand.paper
                border.color: stand.ink
                border.width: 2.2
            }
            Emblem {
                id: seal
                visible: stand.host.sealed && !!stand.host.badge
                size: 104
                role: stand.host.badge ? stand.host.badge.role : "tester"
                spec: stand.host.designSpec ? stand.host.designSpec.badge : null
                live: stand.host.moving
            }
        }
        // the Joestar star, stuck on over the seal's rim
        Item {
            visible: seal.visible
            x: 488; y: 98
            rotation: -14
            Star { reach: 23; x: 2.5; y: 2.5; fill: stand.ink }
            Star { reach: 23; fill: stand.paper; line: stand.ink; lineWidth: 2 }
            Star { reach: 17; fill: stand.purple }
        }
    }

    // ================================================================ above stopped time
    StandInked {
        readonly property real t: stand.pointed && !stand.leaving ? stand.enterT : -1
        visible: t >= 0.3 && t < 0.86
        x: 315 - width / 2; y: 236
        rotation: -7
        scale: 1 + 1.5 * (1 - stand.outCubic((t - 0.3) / 0.12))
        kana: true
        text: "「ザ・ワールド」"
        size: 46
        spacing: -2
        stretch: 1.15
        fill: stand.yellow
        line: stand.ink
        shade: stand.magenta
        lineWidth: 3.2
        depth: 7
    }
    StandInked {
        readonly property real t: stand.pointed && !stand.leaving ? stand.enterT : -1
        visible: t >= 0.86 && t < 1.3
        opacity: 1 - stand.smooth(1.1, 1.3, t)
        x: 315 - width / 2; y: 250
        rotation: -4
        scale: 1 + 0.08 * stand.smooth(0.86, 1.3, t)
        kana: true
        text: "時は動き出す"
        size: 36
        stretch: 1.1
        fill: stand.paper
        line: stand.ink
        shade: stand.cyan
        lineWidth: 3
        depth: 5
    }
    // TO BE CONTINUED rides above the freeze frame's tint, in its own yellow
    Item {
        id: arrow
        x: 226 + stand.arrowShift
        y: 830
        Poly {
            points: [Qt.point(0, 19), Qt.point(32, -2), Qt.point(32, 6), Qt.point(388, 6), Qt.point(388, 32),
                     Qt.point(32, 32), Qt.point(32, 40)]
            fill: stand.yellow
            line: stand.ink
            lineWidth: 2.6
        }
        Heavy {
            x: 38
            y: 20 - height / 2
            text: "TO BE CONTINUED"
            font.pixelSize: 15
            font.letterSpacing: 0.2
        }
    }
    Item {
        x: arrow.x + 33; y: 837
        width: 354; height: 24
        clip: true
        Mono {
            x: 416 - parent.x; y: 9
            text: "No." + stand.host.serial + " · " + stand.host.tier.mark + " " + I18n.tr(stand.host.tier.name).toUpperCase()
                + " · " + I18n.tr("%1 pts").replace("%1", stand.host.rarity.points).toUpperCase()
            font.pixelSize: 7
            font.letterSpacing: 0.5
        }
    }
}
