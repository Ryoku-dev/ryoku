pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Io
import Lithic
import Ryoku.Ui as RyokuUi
import Ryoku.Ui.Singletons
import "../../Singletons"

// the system card: a poster of this computer drawn at 1280 by 800 and scaled
// by its host. A figure over the ground, the owner's name, the machine's parts
// and live readings pinned to the figure. It moves only while `live`.
Item {
    id: card

    // animations, the ticking clock and live readings run only while true
    property bool live: false
    // every part, the hidden ones faint, so the editor can bring them back
    property bool showHidden: false

    readonly property real designWidth: 1280
    readonly property real designHeight: 800
    readonly property string configDir: (Quickshell.env("XDG_CONFIG_HOME") || (Quickshell.env("HOME") + "/.config")) + "/ryoku"

    width: card.designWidth
    height: card.designHeight

    property string machineId: ""
    property string installDate: ""
    property string edition: ""
    property var now: new Date()

    function f(path, def) { return ProfileStore.get(path, def); }
    function blockOn(id) { return card.f("blocks." + id, true); }
    function blockShown(id) { return card.blockOn(id) || card.showHidden; }
    function blockOpacity(id) { return card.blockOn(id) ? 1 : 0.25; }
    readonly property bool heroLeft: card.f("heroSide", "right") === "left"
    readonly property int heroDir: card.heroLeft ? -1 : 1

    function fv(x) {
        return x && x !== "-" && String(x).length > 0 ? String(x) : I18n.tr("unknown");
    }

    function editionNo() {
        var basis = card.machineId + card.installDate;
        var h = 2166136261;
        for (var i = 0; i < basis.length; i++)
            h = ((h ^ basis.charCodeAt(i)) * 16777619) >>> 0;
        return ("000" + (h % 10000)).slice(-4);
    }

    function rate(b) {
        if (b < 1024)
            return b + " B/s";
        if (b < 1048576)
            return (b / 1024).toFixed(b < 10240 ? 1 : 0) + " KB/s";
        return (b / 1048576).toFixed(1) + " MB/s";
    }

    function statBig(k) {
        if (k === "core")
            return LiveStats.cpuTemp > 0 ? LiveStats.cpuTemp + "°C" : LiveStats.cpu + "%";
        if (k === "gpu")
            return LiveStats.gpuOk ? LiveStats.gpuTemp + "°C" : LiveStats.gpuUtil + "%";
        if (k === "mem")
            return LiveStats.ram + "%";
        if (k === "net")
            return "↓ " + card.rate(LiveStats.netDown);
        if (k === "frac")
            return Version.rawVersion.length > 0 ? Version.editionSummary : card.fv("");
        return "";
    }

    function statSub(k) {
        if (k === "core")
            return I18n.tr("Processor %1%").replace("%1", LiveStats.cpu);
        if (k === "gpu")
            return LiveStats.gpuOk ? I18n.tr("Graphics %1%").replace("%1", LiveStats.gpuUtil) : I18n.tr("Graphics");
        if (k === "mem")
            return card.fv(SysInfo.sysRam);
        if (k === "net")
            return "↑ " + card.rate(LiveStats.netUp);
        if (k === "frac")
            return card.edition;
        return "";
    }

    readonly property var palette: SysInfo.sysPalette.length > 0 ? SysInfo.sysPalette.split(",")
        : [Lithic.gold, Lithic.jade, Lithic.lapis, Lithic.amber, Lithic.jasper, Lithic.fg]
    readonly property string barcodeText: ("RYOKU-" + SysInfo.codename + "-" + (card.installDate.length > 0 ? card.installDate : "0")).toUpperCase()

    Binding {
        target: LiveStats
        property: "active"
        value: true
        when: card.live
    }

    Timer {
        interval: 1000
        repeat: true
        running: card.live
        triggeredOnStart: true
        onTriggered: card.now = new Date()
    }

    Process {
        command: ["sh", "-c", "cat /etc/machine-id 2>/dev/null; echo; b=$(stat -c %W / 2>/dev/null); if [ \"${b:-0}\" -gt 0 ]; then date -d @$b +%Y%m%d; else ryoku-host pkg installed-at ryoku-desktop 2>/dev/null; fi"]
        running: true
        stdout: StdioCollector {
            onStreamFinished: {
                var l = this.text.split("\n");
                card.machineId = (l[0] || "").trim();
                card.installDate = (l[1] || "").trim();
            }
        }
    }

    FileView {
        path: "/etc/os-release"
        onLoaded: {
            var m = /^PRETTY_NAME="?([^"\n]*)"?/m.exec(text());
            card.edition = m ? m[1] : "";
        }
    }

    Rectangle {
        anchors.fill: parent
        color: Lithic.ground
    }

    // the watermark glyph behind everything, a desktop-wide gloss
    Text {
        visible: UiPreferences.showJapaneseGloss && card.blockShown("watermark")
        opacity: card.blockOpacity("watermark")
        anchors.centerIn: parent
        anchors.horizontalCenterOffset: -170
        text: card.f("text.watermarkGlyph", "顔")
        color: Lithic.alpha(Lithic.fg, 0.055)
        font.pixelSize: 540
    }

    // the epithets: big faint words that dissolve one into the next while live
    Item {
        id: kinetic
        visible: card.blockShown("epithets")
        opacity: card.blockOpacity("epithets")
        x: 100
        y: 4
        width: 800
        height: 180
        readonly property var words: card.f("text.epithets", [I18n.tr("Power"), I18n.tr("Beauty"), I18n.tr("Grace"), I18n.tr("Ruin"), I18n.tr("Demise"), I18n.tr("Void")])
        property int idx: 0
        property real mix: 0
        readonly property real fit: Math.min(1, width / Math.max(1, epithet.implicitWidth))

        Text {
            id: epithet
            anchors.fill: parent
            verticalAlignment: Text.AlignVCenter
            text: String(kinetic.words[kinetic.idx % Math.max(1, kinetic.words.length)] || "").toUpperCase()
            color: Lithic.alpha(Lithic.fg, 0.2)
            opacity: 1 - kinetic.mix
            font.family: Lithic.font.sans
            font.pixelSize: 210
            font.weight: Font.Medium
            transform: Scale {
                origin.x: 0
                origin.y: epithet.height / 2
                xScale: kinetic.fit * (1 + 0.1 * Math.sin(kinetic.mix * Math.PI))
                yScale: 1 - 0.07 * Math.sin(kinetic.mix * Math.PI)
            }
        }

        // a 1-bit stipple covers the word at the swap
        Canvas {
            anchors.left: epithet.left
            anchors.verticalCenter: epithet.verticalCenter
            anchors.verticalCenterOffset: -epithet.font.pixelSize * 0.11
            width: Math.ceil(epithet.implicitWidth * kinetic.fit) + 6
            height: Math.ceil(epithet.font.pixelSize * 0.82)
            property real m: kinetic.mix
            readonly property color ink: Lithic.alpha(Lithic.fg, 0.24)
            onMChanged: requestPaint()
            onPaint: {
                var ctx = getContext("2d");
                ctx.reset();
                if (m < 0.02)
                    return;
                var cell = 10;
                var cols = Math.ceil(width / cell), rows = Math.ceil(height / cell);
                var bayer = [[0, 8, 2, 10], [12, 4, 14, 6], [3, 11, 1, 9], [15, 7, 13, 5]];
                ctx.fillStyle = ink;
                for (var gy = 0; gy < rows; gy++) {
                    var thr = m * (0.3 + 0.7 * Math.sin((gy + 0.5) / rows * Math.PI));
                    for (var gx = 0; gx < cols; gx++)
                        if ((bayer[gy % 4][gx % 4] + 0.5) / 16 < thr)
                            ctx.fillRect(gx * cell, gy * cell, cell - 1, cell - 1);
                }
            }
        }

        SequentialAnimation {
            running: card.live && kinetic.visible
            loops: Animation.Infinite
            PauseAnimation { duration: 1300 }
            NumberAnimation { target: kinetic; property: "mix"; from: 0; to: 1; duration: 240; easing.type: Easing.InQuad }
            ScriptAction { script: kinetic.idx = (kinetic.idx + 1) % Math.max(1, kinetic.words.length) }
            NumberAnimation { target: kinetic; property: "mix"; from: 1; to: 0; duration: 240; easing.type: Easing.OutQuad }
            onRunningChanged: if (!running) kinetic.mix = 0
        }
    }

    // the same epithets as a still verse
    Column {
        visible: card.blockShown("epithets")
        opacity: card.blockOpacity("epithets")
        x: card.heroLeft ? 210 : 1030
        y: 400
        spacing: 13
        Repeater {
            model: UiPreferences.showJapaneseGloss ? card.f("text.verse", ["力", "美", "滅", "虚"])
                                                  : [I18n.tr("Power"), I18n.tr("Beauty"), I18n.tr("Decay"), I18n.tr("Void")]
            delegate: Text {
                required property var modelData
                text: String(modelData).toUpperCase()
                color: Lithic.alpha(Lithic.fg, 0.2)
                font.family: Lithic.font.sans
                font.pixelSize: UiPreferences.showJapaneseGloss ? 44 : 22
            }
        }
    }

    // the figure, inked in the card's own foreground so it reads on either ground
    Item {
        id: hero
        readonly property var config: card.f("hero", ({}))
        readonly property string kind: hero.config.kind || "default"
        readonly property string src: hero.config.source || ""
        readonly property real zoom: hero.config.zoom !== undefined ? hero.config.zoom : 1
        readonly property real focalX: hero.config.focalX !== undefined ? hero.config.focalX : 0.5
        readonly property real focalY: hero.config.focalY !== undefined ? hero.config.focalY : 0.4
        height: parent.height + 96
        width: height * 900 / 1157
        anchors.verticalCenter: parent.verticalCenter
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.horizontalCenterOffset: 240 * card.heroDir
        clip: true

        // your own photo is dithered live; the stock figures are already one-bit and only take the ink
        RyokuUi.DitherImage {
            visible: hero.kind === "custom"
            width: hero.width * hero.zoom
            height: hero.height * hero.zoom
            x: (hero.width - width) * hero.focalX
            y: (hero.height - height) * hero.focalY
            bone: Lithic.fg
            source: hero.kind === "custom" && hero.src.length > 0 ? "file://" + card.configDir + "/profile/" + hero.src : ""
            dotScale: hero.config.dither !== undefined ? hero.config.dither : 1
            invert: hero.config.invert === true
            fillMode: Image.PreserveAspectCrop
        }
        Image {
            id: stock
            anchors.fill: parent
            visible: false
            source: hero.kind === "custom" ? "" : hero.kind === "gallery" && hero.src.length > 0 ? Ryodecors.dir + hero.src
                : Qt.resolvedUrl("../../art/profile-hero.png")
            fillMode: Image.PreserveAspectFit
            smooth: false
            asynchronous: true
        }
        MultiEffect {
            anchors.fill: parent
            visible: hero.kind !== "custom"
            source: stock
            colorization: 1
            colorizationColor: Lithic.fg
        }
    }

    // a slow scan down the figure
    Item {
        visible: card.live
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        x: hero.x + hero.width * 0.55
        width: hero.width * 0.32
        clip: true
        Rectangle {
            id: sweep
            width: parent.width
            height: 62
            gradient: Gradient {
                GradientStop { position: 0; color: Lithic.alpha(Lithic.fg, 0) }
                GradientStop { position: 0.5; color: Lithic.alpha(Lithic.fg, 0.22) }
                GradientStop { position: 1; color: Lithic.alpha(Lithic.fg, 0) }
            }
            SequentialAnimation on y {
                running: card.live
                loops: Animation.Infinite
                NumberAnimation { from: -80; to: card.height + 40; duration: 4600; easing.type: Easing.InOutSine }
                PauseAnimation { duration: 2600 }
            }
        }
    }

    // the spine along the outer edge
    Text {
        visible: card.blockShown("marginalia")
        opacity: card.blockOpacity("marginalia")
        x: card.heroLeft ? card.width - 21 - width / 2 - height / 2 : 21 - width / 2 + height / 2
        y: card.height / 2 - height / 2
        rotation: -90
        text: card.f("text.marginalia", "RYOKU · " + card.fv(SysInfo.codename).toUpperCase() + " · KERNEL " + card.fv(SysInfo.sysKernel))
        color: Lithic.fg3
        font.family: Lithic.font.mono
        font.pixelSize: 10
        font.letterSpacing: 2
    }

    // the identity and the small print
    Item {
        id: dossier
        x: card.heroLeft ? card.width - 55 - width : 55
        y: 68
        width: 330
        height: card.height - 136

        Column {
            id: head
            width: parent.width
            spacing: 13

            Text {
                text: I18n.tr("System card")
                color: Lithic.fg2
                font.family: Lithic.font.sans
                font.pixelSize: 12
                font.letterSpacing: 1
            }
            Text {
                // with the figure on the left the column sits at the right edge, so the name stays inside the card
                width: card.heroLeft ? dossier.width + 20 : 560
                height: 120
                text: card.f("text.name", card.fv(SysInfo.sysUser))
                color: Lithic.fg
                font.family: Lithic.font.sans
                font.weight: Font.Medium
                font.pixelSize: 104
                fontSizeMode: Text.HorizontalFit
                minimumPixelSize: 30
                verticalAlignment: Text.AlignVCenter
                elide: Text.ElideRight
            }
            Text {
                text: "@" + card.fv(SysInfo.sysHost) + "   ·   " + (card.live ? TimeFormat.format(card.now) : Qt.formatDate(card.now, Qt.locale(TimeFormat.formatLocale || I18n.lang).dateFormat(Locale.ShortFormat)))
                color: Lithic.fg2
                font.family: Lithic.font.mono
                font.pixelSize: 13
            }
            Text {
                width: parent.width
                text: card.f("text.tagline", I18n.tr("A live specimen, shot on black."))
                color: Lithic.fg2
                font.family: Lithic.font.sans
                font.italic: true
                font.pixelSize: 16
                wrapMode: Text.Wrap
            }
        }

        Column {
            id: foot
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            spacing: 13

            Column {
                width: parent.width
                visible: card.blockShown("specs")
                opacity: card.blockOpacity("specs")
                Repeater {
                    model: [
                        { k: I18n.tr("Resolution"), v: card.fv(SysInfo.sysResolution) + (SysInfo.sysRefresh.length > 0 ? " @ " + SysInfo.sysRefresh : "") },
                        { k: I18n.tr("Window manager"), v: SysInfo.sysWM + (SysInfo.sysWmVer && SysInfo.sysWmVer !== "-" ? " " + SysInfo.sysWmVer.replace(/^v+/, "v") : "") },
                        { k: I18n.tr("Up for"), v: card.fv(SysInfo.sysUptime) }
                    ]
                    delegate: Item {
                        required property var modelData
                        width: foot.width
                        height: 26
                        Text {
                            id: specKey
                            anchors.verticalCenter: parent.verticalCenter
                            text: modelData.k
                            color: Lithic.fg2
                            font.family: Lithic.font.sans
                            font.pixelSize: 11
                        }
                        Text {
                            id: specValue
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            width: Math.min(implicitWidth, parent.width * 0.66)
                            horizontalAlignment: Text.AlignRight
                            elide: Text.ElideRight
                            text: modelData.v
                            color: Lithic.fg
                            font.family: Lithic.font.mono
                            font.pixelSize: 12
                        }
                        Rectangle {
                            anchors.left: specKey.right
                            anchors.right: specValue.left
                            anchors.leftMargin: 13
                            anchors.rightMargin: 13
                            anchors.verticalCenter: parent.verticalCenter
                            height: 1
                            color: Lithic.vein
                        }
                    }
                }
            }

            Column {
                width: parent.width
                spacing: 8
                visible: card.blockShown("packages")
                opacity: card.blockOpacity("packages")
                readonly property real share: {
                    var total = Math.max(1, parseInt(SysInfo.sysPackages) || 1);
                    var mine = (parseInt(SysInfo.sysPkgExplicit) || 0) + (Host.supports("aur") ? (parseInt(SysInfo.sysPkgAur) || 0) : 0);
                    return Math.max(0, Math.min(1, mine / total));
                }
                Canvas {
                    width: parent.width
                    height: 21
                    property real share: parent.share
                    readonly property color track: Lithic.edge
                    readonly property color ink: Lithic.fg
                    onShareChanged: requestPaint()
                    onTrackChanged: requestPaint()
                    onWidthChanged: requestPaint()
                    onPaint: {
                        var ctx = getContext("2d");
                        ctx.reset();
                        var mid = height / 2;
                        ctx.lineWidth = 2;
                        var wave = function (to) {
                            ctx.beginPath();
                            for (var x = 0; x <= to; x += 1.5) {
                                var y = mid + 3 * Math.sin(x * 0.45);
                                if (x === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
                            }
                            ctx.stroke();
                        };
                        ctx.strokeStyle = track;
                        wave(width);
                        if (share * width > 1) {
                            ctx.strokeStyle = ink;
                            wave(share * width);
                        }
                    }
                }
                Text {
                    text: Host.supports("aur")
                        ? I18n.tr("%1 chosen · %2 from the AUR · %3 in all").replace("%1", SysInfo.sysPkgExplicit).replace("%2", SysInfo.sysPkgAur).replace("%3", SysInfo.sysPackages)
                        : I18n.tr("%1 chosen · %2 in all").replace("%1", SysInfo.sysPkgExplicit).replace("%2", SysInfo.sysPackages)
                    color: Lithic.fg2
                    font.family: Lithic.font.mono
                    font.pixelSize: 10
                }
            }

            Row {
                width: parent.width
                height: 18
                visible: card.blockShown("palette")
                opacity: card.blockOpacity("palette")
                Repeater {
                    model: card.palette
                    delegate: Rectangle {
                        required property var modelData
                        width: foot.width / Math.max(1, card.palette.length)
                        height: 18
                        color: modelData
                    }
                }
            }

            Item {
                width: parent.width
                height: barcode.height
                visible: card.blockShown("barcode")
                opacity: card.blockOpacity("barcode")
                CardBarcode {
                    id: barcode
                    text: card.barcodeText
                    ink: Lithic.fg
                    unit: Math.max(1, Math.min(2, (foot.width * 0.62) / (Math.max(1, card.barcodeText.length + 2) * 16)))
                }
                Text {
                    anchors.right: parent.right
                    anchors.top: parent.top
                    text: I18n.tr("No. %1").replace("%1", card.editionNo())
                    color: Lithic.fg2
                    font.family: Lithic.font.mono
                    font.pixelSize: 11
                }
            }
        }
    }

    // live readings pinned to the figure by thin leaders
    Item {
        id: telemetry
        anchors.fill: parent
        visible: card.blockShown("telemetry")
        opacity: card.blockOpacity("telemetry")
        readonly property real cx: card.heroLeft ? card.width - 55 - 172 : 55
        readonly property real leadFrom: card.heroLeft ? card.width - 55 - 172 : 55 + 172
        readonly property var shown: card.f("vitals", ["core", "gpu", "mem", "net", "frac"])
        // the readings share the room between the name and the small print
        readonly property real footTop: dossier.y + foot.y - 13
        readonly property real span: telemetry.footTop - 338
        readonly property bool full: telemetry.span >= 3 * 64 + 62
        readonly property bool micro: !telemetry.full && (telemetry.span - 49) / 3 < 50
        readonly property real gap: telemetry.full ? 64 : telemetry.micro ? Math.min(64, Math.max(40, Math.floor((telemetry.span - 40) / 3)))
                                                                            : Math.min(64, Math.max(50, Math.floor((telemetry.span - 49) / 3)))
        readonly property real rowH: telemetry.full ? 62 : telemetry.micro ? 40 : 49
        readonly property real firstY: Math.max(316, Math.min(338, telemetry.footTop - (3 * telemetry.gap + telemetry.rowH)))
        readonly property var pins: [
            { x: cx, sx: leadFrom, cy: firstY, fx: 0.36, fy: 0.26, k: "core", name: I18n.tr("Core"), big: true },
            { x: cx, sx: leadFrom, cy: firstY + gap, fx: 0.30, fy: 0.40, k: "gpu", name: I18n.tr("Graphics"), big: true },
            { x: cx, sx: leadFrom, cy: firstY + 2 * gap, fx: 0.28, fy: 0.54, k: "mem", name: I18n.tr("Memory"), big: true },
            { x: cx, sx: leadFrom, cy: firstY + 3 * gap, fx: 0.40, fy: 0.70, k: "net", name: I18n.tr("Network"), big: true },
            { x: card.heroLeft ? card.width - 372 - 120 : 372, sx: card.heroLeft ? card.width - 372 : 372, cy: firstY + Math.round(gap * 0.94), fx: 0.47, fy: 0.44, k: "frac", name: I18n.tr("Release"), big: false }
        ].filter(p => telemetry.shown.indexOf(p.k) >= 0)

        Text {
            x: telemetry.cx
            y: 296
            text: I18n.tr("Live readings")
            color: Lithic.fg2
            font.family: Lithic.font.sans
            font.pixelSize: 11
            font.letterSpacing: 1
        }

        Canvas {
            id: leads
            anchors.fill: parent
            readonly property color ink: Lithic.fg
            property var pins: telemetry.pins
            property real heroX: hero.x
            onPinsChanged: requestPaint()
            onHeroXChanged: requestPaint()
            onInkChanged: requestPaint()
            onPaint: {
                var ctx = getContext("2d");
                ctx.reset();
                for (var i = 0; i < pins.length; i++) {
                    var p = pins[i];
                    var dx = hero.x + p.fx * hero.width;
                    var dy = hero.y + p.fy * hero.height;
                    var sy = p.cy + (p.big ? (telemetry.micro ? 26 : 30) : 22);
                    ctx.strokeStyle = Qt.rgba(ink.r, ink.g, ink.b, 0.3);
                    ctx.lineWidth = 1;
                    ctx.beginPath();
                    ctx.moveTo(p.sx, sy);
                    ctx.lineTo(dx, dy);
                    ctx.stroke();
                    ctx.fillStyle = Qt.rgba(ink.r, ink.g, ink.b, 0.9);
                    ctx.beginPath();
                    ctx.arc(dx, dy, 2.6, 0, 6.2832);
                    ctx.fill();
                }
            }
        }

        Repeater {
            model: telemetry.pins
            delegate: Column {
                required property var modelData
                x: modelData.x
                y: modelData.cy
                spacing: -1
                Text {
                    text: modelData.name
                    color: Lithic.fg2
                    font.family: Lithic.font.mono
                    font.pixelSize: 10
                }
                Text {
                    text: card.statBig(modelData.k)
                    color: Lithic.fg
                    font.family: Lithic.font.sans
                    font.pixelSize: modelData.big ? (telemetry.micro ? 22 : 30) : 15
                    font.weight: Font.Medium
                    font.features: ({ "tnum": 1 })
                }
                Text {
                    visible: telemetry.full || !modelData.big
                    text: card.statSub(modelData.k)
                    color: Lithic.fg2
                    font.family: Lithic.font.mono
                    font.pixelSize: 10
                }
            }
        }
    }

    // the signal: load average over a moving wave
    Item {
        visible: card.blockShown("signal")
        opacity: card.blockOpacity("signal")
        x: card.heroLeft ? 55 : card.width - 55 - width
        y: card.height - 55 - height
        width: 240
        height: 58
        Text {
            text: I18n.tr("Signal")
            color: Lithic.fg2
            font.family: Lithic.font.sans
            font.pixelSize: 11
        }
        Text {
            anchors.right: parent.right
            text: I18n.tr("Load %1").replace("%1", LiveStats.load.toFixed(2))
            color: Lithic.fg2
            font.family: Lithic.font.mono
            font.pixelSize: 11
        }
        // a damped wave in the card's ink, travelling only while live
        Canvas {
            id: wave
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: 34
            property real phase: 0
            readonly property color ink: Lithic.fg
            onPhaseChanged: requestPaint()
            onInkChanged: requestPaint()
            onWidthChanged: requestPaint()
            onPaint: {
                var ctx = getContext("2d");
                ctx.reset();
                var mid = height / 2;
                ctx.strokeStyle = Qt.rgba(ink.r, ink.g, ink.b, 0.85);
                ctx.lineWidth = 1.5;
                ctx.beginPath();
                for (var x = 0; x <= width; x += 1) {
                    var t = x / Math.max(1, width);
                    var amp = (mid - 2) * Math.exp(-2.6 * t) * (0.55 + 0.45 * Math.sin(t * 9 + wave.phase * 0.5));
                    var y = mid + amp * Math.sin(x * 0.32 - wave.phase);
                    if (x === 0) ctx.moveTo(x, y); else ctx.lineTo(x, y);
                }
                ctx.stroke();
            }
            NumberAnimation on phase {
                running: card.live
                from: 0
                to: Math.PI * 2
                duration: 1400
                loops: Animation.Infinite
            }
        }
    }
}
