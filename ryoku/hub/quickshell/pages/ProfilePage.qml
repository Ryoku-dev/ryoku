pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"

// Profile, the dossier (DESIGN.md section 9) -- the plate users screenshot for a
// rice showcase, so it is the best-looking page in the hub. A gothic system
// poster in the post-punk register: a cracked marble profile bust, baked to a
// high-contrast bone xerox, bleeds on black; the identity is monumental in
// Fraunces over a huge 顔 watermark; the machine's live vitals read as
// line-and-stat callouts pinned to the face (not boxes) -- a body scan of the
// operator; the dossier trails as brutalist small-print; the audio-wave signal
// sits over it. Full-bleed. Bone on black; the only colour
// is the palette strip (data). Driven by LiveStats at 1.5s -- read-only.
Item {
    id: pg

    property var hub
    readonly property bool fullBleed: true
    property bool editing: false
    property bool heroEditing: false
    property bool grabbing: false
    property int fieldsArmed: 0 // inline text fields currently being edited

    Component.onCompleted: LiveStats.active = true
    Component.onDestruction: LiveStats.active = false

    property var now: new Date()
    Timer {
        interval: 1000
        running: true
        repeat: true
        triggeredOnStart: true
        onTriggered: pg.now = new Date()
    }
    readonly property string clockTime: TimeFormat.format(pg.now)

    property string machineId: ""
    property string installDate: ""
    Process {
        id: idp
        running: true
        command: ["sh", "-c", "id=$(cat /etc/machine-id 2>/dev/null); echo $id; b=$(stat -c %W / 2>/dev/null); if [ ${b:-0} -gt 0 ]; then date -d @$b +%Y%m%d; else head -1 /var/log/pacman.log 2>/dev/null | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}' | head -1 | tr -d -; fi"]
        stdout: StdioCollector {
            onStreamFinished: {
                const l = text.split("\n");
                pg.machineId = (l[0] || "").trim();
                pg.installDate = (l[1] || "").trim();
            }
        }
    }

    function fv(x) {
        return (x && x !== "-" && String(x).length > 0) ? x : I18n.tr("unknown");
    }
    function editionNo() {
        const basis = pg.machineId + pg.installDate;
        let h = 2166136261;
        for (let i = 0; i < basis.length; i++)
            h = ((h ^ basis.charCodeAt(i)) * 16777619) >>> 0;
        return ("000" + (h % 10000)).slice(-4);
    }
    function fmtRate(b) {
        if (b < 1024)
            return b + " B/s";
        if (b < 1048576)
            return (b / 1024).toFixed(b < 10240 ? 1 : 0) + " KB/s";
        return (b / 1048576).toFixed(1) + " MB/s";
    }
    // The big figure and the sub for each pinned callout, live off LiveStats.
    function statBig(k) {
        if (k === "core")
            return LiveStats.cpuTemp > 0 ? LiveStats.cpuTemp + "°C" : LiveStats.cpu + "%";
        if (k === "gpu")
            return LiveStats.gpuOk ? LiveStats.gpuTemp + "°C" : LiveStats.gpuUtil + "%";
        if (k === "mem")
            return LiveStats.ram + "%";
        if (k === "net")
            return "\u2193 " + pg.fmtRate(LiveStats.netDown);
        if (k === "frac")
            return Version.editionSummary;
        return "";
    }
    function statSub(k) {
        if (k === "core")
            return "CPU · " + LiveStats.cpu + "%";
        if (k === "gpu")
            return "GPU · " + (LiveStats.gpuOk ? LiveStats.gpuUtil + "%" : "n/a");
        if (k === "mem")
            return pg.fv(SysInfo.sysRam);
        if (k === "net")
            return "\u2191 " + pg.fmtRate(LiveStats.netUp);
        if (k === "frac")
            return Version.editionIndex;
        return "";
    }

    readonly property var paletteModel: SysInfo.sysPalette.length > 0 ? SysInfo.sysPalette.split(",") : Scheme.ramp
    readonly property string barcodeText: I18n.tr("RYOKU-") + SysInfo.codename.toUpperCase() + "-" + (pg.installDate.length > 0 ? pg.installDate : I18n.tr("UNKNOWN"))

    // customization: read from ProfileStore with the plate's built-in default, so
    // an absent/empty profile.json renders exactly the stock marble plate.
    function f(path, def) { return ProfileStore.get(path, def); }

    // The version the seam reports already carries its v (Hyprland answers
    // "v0.56.2"); strip one so the dossier reads one v, not two.
    function wmVersion(v) {
        return v === "-" ? "" : v.replace(/^v+/, "");
    }
    // The dossier foot is bottom-anchored inside the dossier's own margins, so
    // on a short window it climbs into the live column. Its absolute top is the
    // line the telemetry must stop above.
    readonly property real footTop: dossier.y + foot.y - Tokens.s3

    // ── edit-mode helpers ────────────────────────────────────────────────────
    // A block reads on unless profile.json turns it off. In EDIT mode an off
    // block stays rendered but ghosted (a bring-it-back affordance); at rest it
    // vanishes.
    function blockOn(id) { return pg.f("blocks." + id, true); }
    function blockVisible(id) { return pg.blockOn(id) || pg.editing; }
    function blockOpacity(id) { return pg.blockOn(id) ? 1.0 : 0.28; }
    function toggleBlock(id) {
        var p = { "blocks": {} };
        p.blocks[id] = !pg.blockOn(id);
        ProfileStore.put(p);
    }
    readonly property bool heroLeft: pg.f("heroSide", "right") === "left"
    // the hero sits right by default (dossier left); heroSide flips both.
    readonly property int heroDir: pg.heroLeft ? -1 : 1

    // A small corner eye-toggle for a block, shown only in EDIT mode.
    component EyeChip: Rectangle {
        id: chip
        property string block: ""
        readonly property bool on: pg.blockOn(chip.block)
        visible: pg.editing
        z: 50
        width: crow.width + Tokens.s3 * 2
        height: 26
        radius: Tokens.radius
        color: chip.on ? Tokens.bone : Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.9)
        border.width: Tokens.border
        border.color: chip.on ? Tokens.bone : Tokens.lineStrong
        Row {
            id: crow
            anchors.centerIn: parent
            spacing: Tokens.s1
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: chip.on ? "\u25c9" : "\u25cb"
                color: chip.on ? Tokens.inkOnBone : Tokens.inkDim
                font.pixelSize: 11
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: chip.block.toUpperCase()
                color: chip.on ? Tokens.inkOnBone : Tokens.inkDim
                font.family: Tokens.ui
                font.pixelSize: 8
                font.letterSpacing: 0.5
            }
        }
        HoverHandler { cursorShape: Qt.PointingHandCursor }
        MouseArea {
            anchors.fill: parent
            anchors.margins: -9
            onClicked: pg.toggleBlock(chip.block)
        }
    }

    // Click-to-edit text: shows a styled label at rest, an outlined hit box in
    // EDIT mode, and a clean horizontal field (upright + readable, whatever the
    // label's transform) while editing. Commits to `field` on Enter / focus loss.
    component InlineText: Item {
        id: ie
        property string field: ""
        property string value: ""
        property alias font: disp.font
        property color color: Tokens.ink
        property int hAlign: Text.AlignLeft
        property int fontSizeMode: Text.FixedSize
        property int minimumPixelSize: 8
        property bool armed: false
        implicitWidth: disp.implicitWidth
        implicitHeight: disp.implicitHeight

        onArmedChanged: {
            pg.fieldsArmed += ie.armed ? 1 : -1;
            if (ie.armed) {
                inp.text = ie.value;
                inp.forceActiveFocus();
                inp.selectAll();
            }
        }
        Connections {
            target: pg
            function onEditingChanged() { if (!pg.editing && ie.armed) ie.commit(); }
        }

        function commit() {
            if (!ie.armed)
                return;
            ie.armed = false;
            if (inp.text === ie.value)
                return;
            var parts = ie.field.split(".");
            var patch = {};
            patch[parts[0]] = {};
            patch[parts[0]][parts[1]] = inp.text;
            ProfileStore.put(patch);
        }

        // the styled label, at rest and in EDIT mode until armed.
        Text {
            id: disp
            anchors.fill: parent
            text: ie.value
            color: ie.color
            horizontalAlignment: ie.hAlign
            verticalAlignment: Text.AlignVCenter
            elide: Text.ElideRight
            fontSizeMode: ie.fontSizeMode
            minimumPixelSize: ie.minimumPixelSize
            visible: !ie.armed
        }
        // an outlined hit box, so the label reads as editable in EDIT mode.
        Rectangle {
            anchors.fill: parent
            anchors.margins: -3
            visible: pg.editing && !ie.armed
            radius: 2
            color: click.containsMouse ? Qt.rgba(Tokens.ink.r, Tokens.ink.g, Tokens.ink.b, 0.06) : "transparent"
            border.width: Tokens.border
            border.color: click.containsMouse ? Tokens.lineStrong : Tokens.line
        }
        // the in-place editor: same font, size, and position as the label.
        TextInput {
            id: inp
            anchors.fill: parent
            visible: ie.armed
            enabled: ie.armed
            font: disp.font
            color: ie.color
            horizontalAlignment: ie.hAlign
            verticalAlignment: TextInput.AlignVCenter
            clip: true
            onAccepted: ie.commit()
            onActiveFocusChanged: if (!activeFocus) ie.commit()
            Keys.onEscapePressed: event => { inp.text = ie.value; ie.armed = false; event.accepted = true; }
        }
        MouseArea {
            id: click
            anchors.fill: parent
            enabled: pg.editing && !ie.armed
            hoverEnabled: pg.editing
            cursorShape: Qt.IBeamCursor
            onClicked: ie.armed = true
        }
    }

    // ── reusable pieces ─────────────────────────────────────────────────────

    component SpecRow: Item {
        id: sr
        property string k: ""
        property string v: ""
        width: parent ? parent.width : 0
        height: Tokens.s4 + Tokens.s1
        Text {
            id: srk
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            text: sr.k
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fMicro
            font.letterSpacing: Tokens.trackLabel
            font.capitalization: Font.AllUppercase
        }
        Text {
            id: srv
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, sr.width * 0.66)
            horizontalAlignment: Text.AlignRight
            elide: Text.ElideRight
            text: sr.v
            color: Tokens.ink
            font.family: Tokens.mono
            font.pixelSize: Tokens.fSmall
        }
        Rectangle {
            anchors.left: srk.right
            anchors.right: srv.left
            anchors.leftMargin: Tokens.s3
            anchors.rightMargin: Tokens.s3
            anchors.verticalCenter: parent.verticalCenter
            height: Tokens.border
            color: Tokens.lineSoft
        }
    }

    component Wave: Item {
        id: wv
        property real frac: 0
        implicitHeight: Tokens.s4
        Canvas {
            anchors.fill: parent
            property real frac: wv.frac
            onFracChanged: requestPaint()
            onWidthChanged: requestPaint()
            Component.onCompleted: requestPaint()
            onPaint: {
                const ctx = getContext("2d");
                ctx.reset();
                const wd = width;
                const mid = height / 2;
                const fill = Math.max(0, Math.min(1, frac)) * wd;
                ctx.lineWidth = 2;
                ctx.lineCap = "round";
                ctx.strokeStyle = Tokens.line;
                ctx.beginPath();
                for (let x = 0; x <= wd; x += 1.5) {
                    const y = mid + 3 * Math.sin(x * 0.45);
                    x === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y);
                }
                ctx.stroke();
                if (fill > 1) {
                    ctx.strokeStyle = Tokens.ink;
                    ctx.beginPath();
                    for (let x = 0; x <= fill; x += 1.5) {
                        const y = mid + 3 * Math.sin(x * 0.45);
                        x === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y);
                    }
                    ctx.stroke();
                }
            }
        }
    }

    // A pinned callout: label, a big live figure, a sub. Placed by the overlay.
    component Callout: Column {
        id: co
        property string name: ""
        property string k: ""
        property bool big: true
        property bool showSub: true
        property real figure: 30
        spacing: -1
        Text {
            text: co.name
            color: Tokens.inkMuted
            font.family: Tokens.mono
            font.pixelSize: Tokens.fTiny
            font.letterSpacing: Tokens.trackLabel
        }
        Text {
            text: pg.statBig(co.k)
            color: Tokens.ink
            font.family: Tokens.ui
            font.pixelSize: co.big ? co.figure : Tokens.fRow
            font.weight: Font.Medium
            font.features: ({ "tnum": 1 })
        }
        Text {
            text: pg.statSub(co.k)
            color: Tokens.inkDim
            font.family: Tokens.mono
            font.pixelSize: Tokens.fMicro
            visible: co.showSub
        }
    }

    // ── stage ───────────────────────────────────────────────────────────────

    Rectangle {
        anchors.fill: parent
        color: Tokens.paper
    }

    // A monumental face-kanji breathing behind the void (the Profile's own 横顔).
    Text {
        visible: pg.blockVisible("watermark")
        opacity: pg.blockOpacity("watermark")
        anchors.verticalCenter: parent.verticalCenter
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.horizontalCenterOffset: -170
        text: pg.f("text.watermarkGlyph", "顔")
        color: Qt.rgba(Tokens.ink.r, Tokens.ink.g, Tokens.ink.b, 0.055)
        font.family: Tokens.jp
        font.pixelSize: 540
    }
    EyeChip {
        block: "watermark"
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.horizontalCenterOffset: -170
        anchors.verticalCenter: parent.verticalCenter
        anchors.verticalCenterOffset: -300
    }

    // Kinetic epithets: big ghosted words -- power, beauty, demise, void -- warping
    // through a 1-bit dither dissolve, the bust eating their right edge.
    Item {
        id: kinetic
        visible: pg.blockVisible("epithets")
        opacity: pg.blockOpacity("epithets")
        anchors.left: parent.left
        anchors.leftMargin: 100
        anchors.top: parent.top
        anchors.topMargin: Tokens.s1
        width: 800
        height: 180

        EyeChip {
            block: "epithets"
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
        }

        // The arc: ascent to ruin -- power, beauty, grace, then the fall.
        readonly property var words: pg.f("text.epithets", [I18n.tr("POWER"), I18n.tr("BEAUTY"), I18n.tr("GRACE"), I18n.tr("RUIN"), I18n.tr("DEMISE"), I18n.tr("VOID")])
        property int idx: 0
        property real mix: 0
        // The box is wide enough to hold the longest word at full size, so all
        // six read one height; the fit guard only clamps an unexpected overrun.
        readonly property real fit: Math.min(1, width / Math.max(1, epithet.implicitWidth))

        // A uniform-height dissolve: the word reads left-to-right from beside the
        // name (it may start behind it); the bust eats the tail of the long ones.
        Text {
            id: epithet
            anchors.fill: parent
            horizontalAlignment: Text.AlignLeft
            verticalAlignment: Text.AlignVCenter
            text: kinetic.words[kinetic.idx]
            color: Qt.rgba(Tokens.ink.r, Tokens.ink.g, Tokens.ink.b, 0.2)
            opacity: 1 - kinetic.mix
            font.family: Tokens.display
            font.pixelSize: 210
            font.weight: Font.Medium
            transform: Scale {
                origin.x: 0
                origin.y: epithet.height / 2
                xScale: kinetic.fit * (1 + 0.1 * Math.sin(kinetic.mix * Math.PI))
                yScale: 1 - 0.07 * Math.sin(kinetic.mix * Math.PI)
            }
        }

        // The dither burst: chunky Bayer cells cover the glyphs at the swap, so
        // the word dissolves through 1-bit noise rather than a clean fade.
        Canvas {
            id: dither
            anchors.left: epithet.left
            anchors.verticalCenter: epithet.verticalCenter
            anchors.verticalCenterOffset: -epithet.font.pixelSize * 0.11
            width: Math.ceil(epithet.implicitWidth * kinetic.fit) + 6
            height: Math.ceil(epithet.font.pixelSize * 0.82)
            property real m: kinetic.mix
            onMChanged: requestPaint()
            onWidthChanged: requestPaint()
            onHeightChanged: requestPaint()
            onPaint: {
                const ctx = getContext("2d");
                ctx.reset();
                if (m < 0.02)
                    return;
                const cell = 10;
                const cols = Math.ceil(width / cell);
                const rows = Math.ceil(height / cell);
                const bayer = [[0, 8, 2, 10], [12, 4, 14, 6], [3, 11, 1, 9], [15, 7, 13, 5]];
                ctx.fillStyle = Qt.rgba(Tokens.ink.r, Tokens.ink.g, Tokens.ink.b, 0.24);
                for (let gy = 0; gy < rows; gy++) {
                    const fall = Math.sin((gy + 0.5) / rows * Math.PI);
                    const thr = m * (0.3 + 0.7 * fall);
                    for (let gx = 0; gx < cols; gx++)
                        if ((bayer[gy % 4][gx % 4] + 0.5) / 16 < thr)
                            ctx.fillRect(gx * cell, gy * cell, cell - 1, cell - 1);
                }
            }
        }

        SequentialAnimation {
            running: true
            loops: Animation.Infinite
            PauseAnimation { duration: 1300 }
            NumberAnimation { target: kinetic; property: "mix"; from: 0; to: 1; duration: 240; easing.type: Easing.InQuad }
            ScriptAction { script: kinetic.idx = (kinetic.idx + 1) % kinetic.words.length }
            NumberAnimation { target: kinetic; property: "mix"; from: 1; to: 0; duration: 240; easing.type: Easing.OutQuad }
        }
    }

    // The epithets in kanji -- a still vertical verse in the void, the arc the
    // marble already walked: power, beauty, ruin, emptiness.
    Column {
        id: verse
        visible: pg.blockVisible("epithets")
        opacity: pg.blockOpacity("epithets")
        x: 1030
        y: 400
        spacing: Tokens.s3
        Repeater {
            model: pg.f("text.verse", ["力", "美", "滅", "虚"])
            delegate: Text {
                required property var modelData
                text: modelData
                color: Qt.rgba(Tokens.ink.r, Tokens.ink.g, Tokens.ink.b, 0.2)
                font.family: Tokens.jp
                font.pixelSize: 44
            }
        }
    }

    // The hero: the cracked bust, bone xerox, bleeding on black. Faces left,
    // into the dossier; the curls bleed off the right.
    Item {
        id: hero
        readonly property string kind: pg.f("hero.kind", "default")
        readonly property string src: pg.f("hero.source", "")
        height: parent.height + 96
        width: height * 900 / 1157
        anchors.verticalCenter: parent.verticalCenter
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.horizontalCenterOffset: 240 * pg.heroDir
        // default marble + gallery art are already 1-bit bone, shown raw.
        Image {
            anchors.fill: parent
            visible: hero.kind !== "custom"
            source: (hero.kind === "gallery" && hero.src.length > 0) ? (Ryodecors.dir + hero.src) : Qt.resolvedUrl("../art/profile-hero.png")
            fillMode: Image.PreserveAspectFit
            smooth: false
            asynchronous: true
        }
        // a custom image runs the live dither, framed by focal point + zoom.
        Item {
            anchors.fill: parent
            clip: true
            visible: hero.kind === "custom"
            DitherImage {
                width: hero.width * pg.f("hero.zoom", 1.0)
                height: hero.height * pg.f("hero.zoom", 1.0)
                x: (hero.width - width) * pg.f("hero.focalX", 0.5)
                y: (hero.height - height) * pg.f("hero.focalY", 0.4)
                source: hero.src.length > 0 ? ("file://" + (Quickshell.env("HOME") || "") + "/.config/ryoku/profile/" + hero.src) : ""
                dotScale: pg.f("hero.dither", 1.0)
                invert: pg.f("hero.invert", false)
                fillMode: Image.PreserveAspectCrop
            }
        }
        // in EDIT mode the hero carries an affordance opening the hero editor.
        Rectangle {
            visible: pg.editing
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottom: parent.bottom
            anchors.bottomMargin: parent.height * 0.2
            width: heroLabel.width + Tokens.s4 * 2
            height: 34
            radius: Tokens.radius
            color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.92)
            border.width: Tokens.border
            border.color: Tokens.lineStrong
            z: 40
            Text {
                id: heroLabel
                anchors.centerIn: parent
                text: I18n.tr("EDIT HERO")
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: 10
                font.letterSpacing: 0.6
            }
            HoverHandler { cursorShape: Qt.PointingHandCursor }
            TapHandler { onTapped: pg.heroEditing = true }
        }
    }

    // A slow scan drifting down the bust -- the specimen under the lens.
    Item {
        id: scanSweep
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        x: hero.x + hero.width * 0.55
        width: hero.width * 0.32
        clip: true
        Rectangle {
            width: parent.width
            height: 62
            gradient: Gradient {
                GradientStop { position: 0.0; color: "transparent" }
                GradientStop { position: 0.5; color: Qt.rgba(Tokens.ink.r, Tokens.ink.g, Tokens.ink.b, 0.22) }
                GradientStop { position: 1.0; color: "transparent" }
            }
            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                anchors.left: parent.left
                anchors.right: parent.right
                height: 1
                color: Qt.rgba(Tokens.ink.r, Tokens.ink.g, Tokens.ink.b, 0.45)
            }
            SequentialAnimation on y {
                running: true
                loops: Animation.Infinite
                NumberAnimation { from: -80; to: scanSweep.height + 40; duration: 4600; easing.type: Easing.InOutSine }
                PauseAnimation { duration: 2600 }
            }
        }
    }

    // Marginalia spine.
    Item {
        visible: pg.blockVisible("marginalia")
        opacity: pg.blockOpacity("marginalia")
        anchors.left: pg.heroLeft ? undefined : parent.left
        anchors.right: pg.heroLeft ? parent.right : undefined
        anchors.leftMargin: Tokens.s4
        anchors.rightMargin: Tokens.s4
        anchors.verticalCenter: parent.verticalCenter
        width: 1
        height: 1
        Text {
            anchors.centerIn: parent
            rotation: -90
            text: pg.f("text.marginalia", I18n.tr("RYOKU · %1 · KERNEL %2 · SHOT ON BLACK").arg(pg.fv(SysInfo.codename)).arg(pg.fv(SysInfo.sysKernel)))
            color: Tokens.inkFaint
            font.family: Tokens.mono
            font.pixelSize: Tokens.fTiny
            font.letterSpacing: Tokens.trackMark
        }
    }
    EyeChip {
        block: "marginalia"
        anchors.left: pg.heroLeft ? undefined : parent.left
        anchors.right: pg.heroLeft ? parent.right : undefined
        anchors.leftMargin: Tokens.s2
        anchors.rightMargin: Tokens.s2
        anchors.bottom: parent.bottom
        anchors.bottomMargin: Tokens.s6 * 2
    }

    Btn {
        id: editBtn
        text: pg.editing ? I18n.tr("DONE") : I18n.tr("EDIT")
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.rightMargin: Tokens.s6
        // below the corner chrome strip (the FILES and UPDATES chips end at 46):
        // EDIT used to stack directly under UPDATES and read as a stray
        anchors.topMargin: Tokens.s6 * 2
        onAct: pg.editing = !pg.editing
    }

    // ── LEFT: the identity + dossier small-print ──────────────────────────────
    Item {
        id: dossier
        anchors.left: parent.left
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.leftMargin: Tokens.s6 + Tokens.s5
        anchors.topMargin: Tokens.s6 * 2
        anchors.bottomMargin: Tokens.s6 * 2
        width: 330

        Column {
            id: head
            anchors { left: parent.left; right: parent.right; top: parent.top }
            // the register row sits off the title: a rule over a 32px
            // title needs more than the gap between two lines of body text
            spacing: Tokens.s3

            Row {
                // the register row holds a fixed box, so the rule and the seal keep
                // their distance from the title on every page
                height: Tokens.s5
                spacing: Tokens.s2
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: "力"
                    color: Tokens.inkDim
                    font.family: Tokens.jp
                    font.pixelSize: Tokens.fMicro
                }
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("·  SYSTEM DOSSIER")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fTiny
                    font.letterSpacing: Tokens.trackMark
                    font.capitalization: Font.AllUppercase
                }
            }
            InlineText {
                // the headline runs wide, over the empty space left of the hero,
                // so long names shrink to fit on one line instead of truncating.
                width: 560
                field: "text.name"
                value: pg.f("text.name", pg.fv(SysInfo.sysUser))
                color: Tokens.ink
                font.family: Tokens.display
                font.pixelSize: 104
                fontSizeMode: Text.HorizontalFit
                minimumPixelSize: 30
            }
            Text {
                text: "@" + pg.fv(SysInfo.sysHost) + "   ·   " + pg.clockTime
                color: Tokens.inkMuted
                font.family: Tokens.mono
                font.pixelSize: Tokens.fSmall
            }
            InlineText {
                width: parent.width
                field: "text.tagline"
                value: pg.f("text.tagline", I18n.tr("A live specimen - cracked, shot on black."))
                color: Tokens.inkDim
                font.family: Tokens.display
                font.italic: true
                font.pixelSize: Tokens.fRow
            }
        }

        // Dossier small-print, anchored to the foot. Each block is wrapped so its
        // eye chip (in the gutter) can anchor to a real parent.
        Column {
            id: foot
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            spacing: Tokens.s3

            Item {
                width: parent.width
                height: specsBlk.height
                visible: pg.blockVisible("specs")
                Column {
                    id: specsBlk
                    width: parent.width
                    opacity: pg.blockOpacity("specs")
                    spacing: 0
                    SpecRow {
                        k: I18n.tr("Resolution")
                        v: pg.fv(SysInfo.sysResolution) + (SysInfo.sysRefresh && SysInfo.sysRefresh.length > 0 ? " @ " + SysInfo.sysRefresh : "")
                    }
                    SpecRow {
                        k: I18n.tr("Compositor")
                        v: SysInfo.sysWM + (SysInfo.sysWmVer && SysInfo.sysWmVer !== "-" ? " v" + pg.wmVersion(SysInfo.sysWmVer) : "")
                    }
                    SpecRow {
                        k: I18n.tr("Uptime")
                        v: pg.fv(SysInfo.sysUptime)
                    }
                }
                EyeChip { block: "specs"; anchors.left: parent.right; anchors.leftMargin: Tokens.s3; anchors.verticalCenter: parent.verticalCenter }
            }

            Item {
                width: parent.width
                height: pkgBlk.height
                visible: pg.blockVisible("packages")
                Column {
                    id: pkgBlk
                    width: parent.width
                    opacity: pg.blockOpacity("packages")
                    spacing: Tokens.s2
                    Wave {
                        width: parent.width
                        frac: {
                            const total = Math.max(1, parseInt(SysInfo.sysPackages) || 1);
                            const mine = (parseInt(SysInfo.sysPkgExplicit) || 0) + (parseInt(SysInfo.sysPkgAur) || 0);
                            return Math.max(0, Math.min(1, mine / total));
                        }
                    }
                    Text {
                        text: I18n.tr("%1 EXPLICIT · %2 AUR · %3 TOTAL").arg(SysInfo.sysPkgExplicit).arg(SysInfo.sysPkgAur).arg(SysInfo.sysPackages)
                        color: Tokens.inkMuted
                        font.family: Tokens.mono
                        font.pixelSize: 10
                    }
                }
                EyeChip { block: "packages"; anchors.left: parent.right; anchors.leftMargin: Tokens.s3; anchors.verticalCenter: parent.verticalCenter }
            }

            Item {
                width: parent.width
                height: palBlk.height
                visible: pg.blockVisible("palette")
                Rectangle {
                    id: palBlk
                    width: parent.width
                    opacity: pg.blockOpacity("palette")
                    height: 18
                    radius: Tokens.radius
                    color: "transparent"
                    border.width: Tokens.border
                    border.color: Tokens.line
                    clip: true
                    Row {
                        anchors.fill: parent
                        anchors.margins: Tokens.border
                        Repeater {
                            model: pg.paletteModel
                            Rectangle {
                                required property var modelData
                                width: parent.width / Math.max(1, pg.paletteModel.length)
                                height: parent.height
                                color: modelData
                            }
                        }
                    }
                }
                EyeChip { block: "palette"; anchors.left: parent.right; anchors.leftMargin: Tokens.s3; anchors.verticalCenter: parent.verticalCenter }
            }

            Item {
                width: parent.width
                height: barBlk.height
                visible: pg.blockVisible("barcode")
                Item {
                    id: barBlk
                    width: parent.width
                    opacity: pg.blockOpacity("barcode")
                    height: bc.implicitHeight
                    Barcode {
                        id: bc
                        anchors.left: parent.left
                        anchors.bottom: parent.bottom
                        text: pg.barcodeText
                        unit: Math.max(1, Math.min(2, (foot.width * 0.62) / (Math.max(1, bc.text.length + 2) * 16)))
                    }
                    Text {
                        anchors.right: parent.right
                        anchors.top: parent.top
                        text: I18n.tr("No. %1").arg(pg.editionNo())
                        color: Tokens.inkDim
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fMicro
                    }
                }
                EyeChip { block: "barcode"; anchors.left: parent.right; anchors.leftMargin: Tokens.s3; anchors.verticalCenter: parent.verticalCenter }
            }
        }
    }

    // ── the live scan: vitals as line-and-stat callouts pinned to the face ────
    Item {
        id: telem
        visible: pg.blockVisible("telemetry")
        opacity: pg.blockOpacity("telemetry")
        anchors.fill: parent
        EyeChip {
            block: "telemetry"
            anchors.left: parent.left
            anchors.leftMargin: Tokens.s6 + Tokens.s5
            anchors.top: parent.top
            anchors.topMargin: 296
        }
        readonly property real fL: hero.x
        readonly property real fT: hero.y
        readonly property real fW: hero.width
        readonly property real fH: hero.height
        readonly property real cx: Tokens.s6 + Tokens.s5
        readonly property real leadFrom: cx + 172
        // The column lives between the head and the bottom-anchored foot, so the
        // pins distribute over the real space instead of hard-coded pixels: the
        // stock 64px pitch until the foot climbs close, then compact (the sub
        // lines drop) and micro (the figures shrink) fallbacks.
        readonly property real span: pg.footTop - 338
        readonly property bool full: span >= 3 * 64 + 62
        readonly property bool micro: !full && (span - 49) / 3 < 50
        readonly property real gap: full ? 64
            : micro ? Math.min(64, Math.max(40, Math.floor((span - 40) / 3)))
            : Math.min(64, Math.max(50, Math.floor((span - 49) / 3)))
        readonly property real rowH: full ? 62 : (micro ? 40 : 49)
        readonly property real firstY: Math.max(316, Math.min(338, pg.footTop - (3 * gap + rowH)))
        // cy: the callout's y; (fx, fy): the face point it reads; sx: leader start x.
        readonly property var pins: [
            { x: cx, sx: leadFrom, cy: firstY, fx: 0.36, fy: 0.26, k: "core", name: I18n.tr("CORE"), big: true },
            { x: cx, sx: leadFrom, cy: firstY + gap, fx: 0.30, fy: 0.40, k: "gpu", name: "GPU", big: true },
            { x: cx, sx: leadFrom, cy: firstY + 2 * gap, fx: 0.28, fy: 0.54, k: "mem", name: I18n.tr("MEMORY"), big: true },
            { x: cx, sx: leadFrom, cy: firstY + 3 * gap, fx: 0.40, fy: 0.70, k: "net", name: I18n.tr("NETWORK"), big: true },
            { x: 372, sx: 372, cy: firstY + Math.round(gap * 0.94), fx: 0.47, fy: 0.44, k: "frac", name: "亀裂 · " + I18n.tr("FRACTURE"), big: false }
        ]

        // Eyebrow for the live column, with the never-still pulse.
        Row {
            x: telem.cx
            y: 296
            spacing: Tokens.s2
            Rectangle {
                anchors.verticalCenter: parent.verticalCenter
                width: 6
                height: 6
                radius: 3
                color: Tokens.ink
                SequentialAnimation on opacity {
                    running: true
                    loops: Animation.Infinite
                    NumberAnimation { to: 0.2; duration: 850; easing.type: Easing.InOutQuad }
                    NumberAnimation { to: 1; duration: 850; easing.type: Easing.InOutQuad }
                }
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: "脈 · " + I18n.tr("LIVE TELEMETRY")
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fTiny
                font.letterSpacing: Tokens.trackMark
                font.capitalization: Font.AllUppercase
            }
        }

        // Leaders: a thin bone line from each stat to the face point it reads.
        Canvas {
            id: leads
            anchors.fill: parent
            onWidthChanged: requestPaint()
            onHeightChanged: requestPaint()
            Component.onCompleted: requestPaint()
            Connections {
                target: telem
                function onPinsChanged() { leads.requestPaint(); }
            }
            Connections {
                target: hero
                function onXChanged() { leads.requestPaint(); }
                function onWidthChanged() { leads.requestPaint(); }
            }
            onPaint: {
                const ctx = getContext("2d");
                ctx.reset();
                if (telem.fW < 4)
                    return;
                for (let i = 0; i < telem.pins.length; i++) {
                    const p = telem.pins[i];
                    const dx = telem.fL + p.fx * telem.fW;
                    const dy = telem.fT + p.fy * telem.fH;
                    const sy = p.cy + (p.big ? (telem.micro ? 26 : 30) : 22);
                    ctx.strokeStyle = Qt.rgba(Tokens.ink.r, Tokens.ink.g, Tokens.ink.b, 0.3);
                    ctx.lineWidth = 1;
                    ctx.beginPath();
                    ctx.moveTo(p.sx, sy);
                    ctx.lineTo(dx, dy);
                    ctx.stroke();
                    ctx.fillStyle = Qt.rgba(Tokens.ink.r, Tokens.ink.g, Tokens.ink.b, 0.9);
                    ctx.beginPath();
                    ctx.arc(dx, dy, 2.6, 0, 6.2832);
                    ctx.fill();
                }
            }
        }

        Repeater {
            model: telem.pins.filter(function (p) { return pg.f("vitals", ["core", "gpu", "mem", "net", "frac"]).indexOf(p.k) >= 0; })
            delegate: Callout {
                required property var modelData
                x: modelData.x
                y: modelData.cy
                name: modelData.name
                k: modelData.k
                big: modelData.big
                showSub: telem.full
                figure: telem.micro ? 22 : 30
            }
        }
    }

    // The live signal: a damped-wave gif, proof the machine is transmitting.
    Item {
        id: signal
        visible: pg.blockVisible("signal")
        opacity: pg.blockOpacity("signal")
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.rightMargin: Tokens.s6
        anchors.bottomMargin: Tokens.s6
        width: 240
        height: 58
        EyeChip {
            block: "signal"
            anchors.right: parent.right
            anchors.bottom: parent.top
            anchors.bottomMargin: Tokens.s2
        }
        Text {
            anchors.left: parent.left
            anchors.top: parent.top
            text: I18n.tr("SIGNAL")
            color: Tokens.inkMuted
            font.family: Tokens.ui
            font.pixelSize: Tokens.fMicro
            font.letterSpacing: Tokens.trackLabel
            font.capitalization: Font.AllUppercase
        }
        Text {
            anchors.right: parent.right
            anchors.top: parent.top
            text: I18n.tr("LOAD %1").arg(LiveStats.load.toFixed(2))
            color: Tokens.inkDim
            font.family: Tokens.mono
            font.pixelSize: Tokens.fMicro
        }
        AnimatedImage {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            height: 34
            source: Ryodecors.dir + "wave.gif"
            fillMode: Image.PreserveAspectCrop
            playing: true
            speed: 0.7
            opacity: 0.85
        }
    }
    // ── EDIT overlays: the edit panel, the hero editor, share dialogs ─────────
    function exportImage() {
        pg.grabbing = true;
        grabTimer.start();
    }
    Timer {
        id: grabTimer
        interval: 60
        onTriggered: {
            const path = (Quickshell.env("HOME") || "") + "/Pictures/ryoku-profile-" + Qt.formatDate(new Date(), "yyyyMMdd") + ".png";
            pg.grabToImage(function (res) { res.saveToFile(path); pg.grabbing = false; }, Qt.size(pg.width * 2, pg.height * 2));
        }
    }
    Process { id: profileProc }
    PickFile {
        id: exportDlg
        title: I18n.tr("Export profile to a folder")
        foldersOnly: true
        startFolder: "file://" + (Quickshell.env("HOME") || "")
        onPicked: (p) => {
            profileProc.command = ["ryoku-hub", "profile", "export", ("" + p).replace("file://", "") + "/ryoku-" + pg.fv(SysInfo.sysUser) + ".ryoprofile"];
            profileProc.running = true;
            exportDlg.active = false;
        }
        onCanceled: exportDlg.active = false
    }
    PickFile {
        id: importDlg
        title: I18n.tr("Open a Ryoku profile")
        fileFilters: ["*.ryoprofile"]
        startFolder: "file://" + (Quickshell.env("HOME") || "")
        onPicked: (p) => {
            profileProc.command = ["ryoku-hub", "profile", "import", ("" + p).replace("file://", "")];
            profileProc.running = true;
            importDlg.active = false;
        }
        onCanceled: importDlg.active = false
    }
    // sibling overlays are loaded by URL (the robust form that resolves both in
    // the shell and standalone), lazily instantiated only while their mode is on.
    Loader {
        id: editLoader
        active: pg.editing
        visible: pg.editing && !pg.heroEditing && !pg.grabbing
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: parent.bottom
        anchors.bottomMargin: Tokens.s6
        source: Qt.resolvedUrl("ProfileToolbar.qml")
        Connections {
            target: editLoader.item
            function onExportImage() { pg.exportImage(); }
            function onExportProfile() { exportDlg.open(); }
            function onImportProfile() { importDlg.open(); }
            function onResetAll() { ProfileStore.reset(); }
            function onDone() { pg.editing = false; }
        }
    }
    Shortcut {
        sequence: "Escape"
        enabled: pg.editing && pg.fieldsArmed === 0
        onActivated: {
            if (pg.heroEditing)
                pg.heroEditing = false;
            else
                pg.editing = false;
        }
    }
    Loader {
        id: heroLoader
        active: pg.heroEditing
        visible: pg.heroEditing
        anchors.fill: parent
        anchors.margins: Tokens.s7
        source: Qt.resolvedUrl("HeroEditor.qml")
        Connections {
            target: heroLoader.item
            function onDone() { pg.heroEditing = false; }
        }
    }
}
