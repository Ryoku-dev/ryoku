pragma ComponentBehavior: Bound

import QtQuick
import ".."
import shell.services
import "../../../components"
import "../framebars/menus" as Tips
import Qt.labs.folderlistmodel
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons

// Capture card: the Super+S surface, a frame-edge card on the shared PopoutCard
// skin so it opens, melts and dismisses exactly like the music / bluetooth cards.
// Screenshot is the quick path -- pick a delay, a save target and a mode; with
// "Beautify after" on the saved shot then opens in Ryoshot. Record starts a
// capture (the floating island takes over the live controls) with desktop / mic
// toggles. Every option is remembered (Capture / Recorder prefs). Compact by
// design; never grabs the keyboard.
Item {
    id: root

    property real s: 1
    property bool open: false
    signal requestClose()
    property bool skin: true                 // false: embed bare (quick-settings tab), no card
    // The card skin is the compact popup; embedded bare (the quick-settings tab)
    // the surface is roomier, so its padding, gaps and tiles open up.
    readonly property bool roomy: !root.skin

    readonly property real pad: (root.roomy ? 16 : 12) * root.s
    readonly property real gap: (root.roomy ? 10 : 7) * root.s
    readonly property color ink: Theme.ink(Theme.effectiveSurface)
    readonly property color inkDim: Theme.inkOn(Theme.effectiveSurface, Theme.onSurfaceVariant, 3.0)
    readonly property color line: Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.14)
    readonly property real innerW: root.width - root.pad * 2

    readonly property var delaySteps: [0, 1, 3, 5, 10]
    readonly property var saveSteps: ["both", "clipboard", "file"]
    function cycle(arr, cur) { const i = arr.indexOf(cur); return arr[(i + 1) % arr.length]; }

    readonly property string saveGlyph: Capture.save === "clipboard" ? "clipboard" : Capture.save === "file" ? "folder" : "image"
    readonly property string saveLabel: Capture.save === "clipboard" ? I18n.tr("Clip")
        : Capture.save === "file" ? I18n.tr("Folder") : I18n.tr("Both")

    // recording quality chips: value lists (daemon-validated) and the short labels
    // the chips show. The chip's own I18n.tr wraps the label, so these stay English.
    readonly property var codecSteps: ["h264", "hevc", "av1"]
    readonly property var qualitySteps: ["medium", "high", "very_high", "ultra"]
    readonly property var fpsSteps: [30, 60, 120]
    readonly property var containerSteps: ["mp4", "mkv", "webm"]
    function codecLabel(c) { return c === "hevc" ? "HEVC" : c === "av1" ? "AV1" : "H.264"; }
    function qualityLabel(q) { return q === "medium" ? "Medium" : q === "high" ? "High" : q === "ultra" ? "Ultra" : "Very high"; }
    function containerLabel(c) { return c === "mkv" ? "MKV" : c === "webm" ? "WEBM" : "MP4"; }

    function shoot(mode) { root.requestClose(); Capture.shoot(mode); }
    function record(mode) {
        root.requestClose();
        // Screen has no target to pick, so it arms the delay here; the other three
        // pick first, then apply the delay.
        if (mode === "screen")
            Recorder.startAfter(Recorder.recordArgs(), Capture.delay);
        else
            Capture.recordTarget(mode, Recorder.recordArgs());
    }

    // ── recent captures gallery (roomy tab) ─────────────────────────────────────
    // The latest screenshots and recordings, each in its own labelled group so the
    // two never blur together. A recording shows a poster frame with a play badge
    // and its duration; a click on any tile opens that group's folder.
    // The one recordings directory every writer and list agree on (Paths resolves
    // the env override, the Hub's setting, then the XDG Videos default).
    readonly property string recordingsDir: Paths.recordingsDir
    readonly property string thumbDir: Quickshell.env("HOME") + "/.cache/ryoku-capture-thumbs"
    property var clipDur: ({})                // recording path -> "M:SS"
    property var clipPoster: ({})             // recording path -> poster jpg path
    // FolderListModel's own Time sort proved unreliable here (it fell back to
    // name order, surfacing the oldest files), so read the folder and sort by
    // modified time in JS -- newest first -- then take the group's first few.
    function recentFrom(model, n) {
        const all = [];
        for (let i = 0; i < model.count; i++) {
            const u = model.get(i, "fileUrl");
            if (!u)
                continue;
            const d = model.get(i, "fileModified");
            const t = d ? (typeof d.getTime === "function" ? d.getTime() : Date.parse(d) || 0) : 0;
            all.push({ url: "" + u, path: "" + model.get(i, "filePath"), t: t });
        }
        all.sort((a, b) => b.t - a.t);
        return all.slice(0, n);
    }
    readonly property var recentShots: recentFrom(shotFiles, 3)
    readonly property var recentClips: recentFrom(clipFiles, 3)
    onRecentClipsChanged: if (root.roomy) clipTimer.restart()

    function shq(p) { return "'" + ("" + p).replace(/'/g, "'\\''") + "'"; }
    function fmtDur(sec) {
        if (!(sec > 0))
            return "";
        const m = Math.floor(sec / 60), r = Math.floor(sec % 60);
        return m + ":" + (r < 10 ? "0" : "") + r;
    }
    // For each recent recording, in one pass: cache a poster frame (keyed by a
    // path hash) if missing, then read the duration. Output: path\tseconds\tposter.
    function refreshClips() {
        const clips = root.recentClips;
        if (clips.length === 0)
            return;
        const dir = root.shq(root.thumbDir);
        const lines = clips.map(c => {
            const p = root.shq(c.path);
            return "K=$(printf %s " + p + " | md5sum | cut -d' ' -f1); PO=" + dir + "/$K.jpg; "
                + "[ -s \"$PO\" ] || ffmpeg -nostdin -loglevel error -y -ss 0.5 -i " + p
                + " -frames:v 1 -vf scale=480:-2 \"$PO\" 2>/dev/null; "
                + "D=$(ffprobe -v error -show_entries format=duration -of csv=p=0 " + p + " 2>/dev/null); "
                + "printf '%s\\t%s\\t%s\\n' " + p + " \"$D\" \"$PO\"";
        });
        clipProc.running = false;
        clipProc.command = ["bash", "-c", "mkdir -p " + dir + "\n" + lines.join("\n")];
        clipProc.running = true;
    }

    implicitWidth: 264 * root.s
    implicitHeight: content.implicitHeight + root.pad * 2

    // the shared card skin: framed surface tile + click-swallow, same as music/bt.
    PopoutCard { anchors.fill: parent; visible: root.skin }

    // Defer the recent gallery's heavy work (folder scans, thumbnail decodes, the
    // poster ffmpeg pass) until just after the open animation, so Super+Esc opens
    // without a hitch; the gallery fills in a beat later.
    property bool galleryReady: false
    Component.onCompleted: if (root.roomy) readyTimer.start()
    Timer { id: readyTimer; interval: 400; onTriggered: root.galleryReady = true }

    // Live views of the capture folders (newest first); FolderListModel watches
    // each dir, so a fresh shot or clip appears without polling.
    FolderListModel {
        id: shotFiles
        folder: root.galleryReady ? ("file://" + Capture.shotsDir) : ""
        showDirs: false
        nameFilters: ["*.png"]
    }
    FolderListModel {
        id: clipFiles
        folder: root.galleryReady ? ("file://" + root.recordingsDir) : ""
        showDirs: false
        nameFilters: ["*.mp4", "*.mkv", "*.webm", "*.mov", "*.avi", "*.m4v"]
    }
    Process {
        id: clipProc
        stdout: StdioCollector {
            id: clipOut
            onStreamFinished: {
                const dur = {}, post = {};
                const rows = clipOut.text.split("\n");
                for (let i = 0; i < rows.length; i++) {
                    const c = rows[i].split("\t");
                    if (c.length < 3)
                        continue;
                    const p = c[0], sec = parseFloat(c[1]);
                    if (p && sec > 0)
                        dur[p] = root.fmtDur(sec);
                    if (p && c[2])
                        post[p] = c[2];
                }
                root.clipDur = dur;
                root.clipPoster = post;
            }
        }
    }
    // Debounce: FolderListModel settles its count over several ticks on load, so
    // coalesce the bursts into one batch once the recent list stops changing.
    Timer {
        id: clipTimer
        interval: 200
        onTriggered: root.refreshClips()
    }

    // ── shared bits ───────────────────────────────────────────────────────────

    component Eyebrow: Text {
        color: root.inkDim
        font.family: Theme.mono
        font.pixelSize: 9 * root.s
        font.letterSpacing: 1.6
        font.weight: Font.Medium
    }

    // small tap-to-cycle pill (delay, save target): hairline box, glyph + label.
    component CycleChip: Item {
        id: chip
        property string glyph: ""
        property string label: ""
        signal tapped()
        property string tip: ""
        // A chip near the card's edge pins its bubble inward, or a long tip runs
        // off the card and clips: the bubble is one line and sized to its text.
        property string tipAlign: "center"
        implicitHeight: 19 * root.s
        implicitWidth: chipRow.implicitWidth + 12 * root.s
        Rectangle {
            anchors.fill: parent
            radius: 5 * root.s
            color: chTap.pressed ? Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.16)
                : (chHov.hovered ? Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.08) : "transparent")
            border.width: Theme.borderWidth
            border.color: root.line
            Behavior on color { ColorAnimation { duration: Motion.fast } }
        }
        Row {
            id: chipRow
            anchors.centerIn: parent
            spacing: 4 * root.s
            GlyphIcon {
                anchors.verticalCenter: parent.verticalCenter
                width: 11 * root.s
                height: width
                name: chip.glyph
                stroke: 1.7
                color: root.inkDim
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr(chip.label)
                color: root.ink
                font.family: Theme.mono
                font.pixelSize: 9.5 * root.s
                font.weight: Font.Medium
            }
        }
        HoverHandler { id: chHov; cursorShape: Qt.PointingHandCursor }
        MouseArea { id: chTap; anchors.fill: parent; onClicked: chip.tapped() }
        Tips.QsTip { text: chip.tip; below: true; hovered: chHov.hovered; align: chip.tipAlign }
    }
    // small icon-only toggle with a hover bubble (desktop / mic audio): bone-plate
    // when on, hairline when off.
    component IconToggle: Item {
        id: itg
        property string glyph: ""
        property string tip: ""
        property bool on: false
        signal toggled()
        implicitWidth: 22 * root.s
        implicitHeight: 18 * root.s
        Rectangle {
            anchors.fill: parent
            radius: 4 * root.s
            color: itg.on ? Theme.inverseSurface
                : (igTap.pressed ? Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.16)
                : (igHov.hovered ? Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.08) : "transparent"))
            border.width: itg.on ? 0 : Theme.borderWidth
            border.color: root.line
            Behavior on color { ColorAnimation { duration: Motion.fast } }
        }
        GlyphIcon {
            anchors.centerIn: parent
            width: 12.5 * root.s
            height: width
            name: itg.glyph
            stroke: 1.7
            color: itg.on ? Theme.inverseOnSurface : root.inkDim
        }
        HoverHandler { id: igHov; cursorShape: Qt.PointingHandCursor }
        MouseArea { id: igTap; anchors.fill: parent; onClicked: itg.toggled() }
        Tips.QsTip { text: itg.tip; below: true; hovered: igHov.hovered }
    }

    // a capture-mode tile: crisp glyph over a tiny label, hairline box, hover lift.
    // `accent` tints the glyph vermilion for the record tiles.
    component ModeTile: Item {
        id: tile
        property real w: 54 * root.s
        property string glyph: ""
        property string label: ""
        property bool accent: false
        property string tip: ""
        signal tapped()
        width: tile.w
        implicitHeight: (root.roomy ? 50 : 40) * root.s
        Rectangle {
            anchors.fill: parent
            radius: 6 * root.s
            color: tTap.pressed ? Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.16)
                : (tHov.hovered ? Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.08) : "transparent")
            border.width: Theme.borderWidth
            border.color: tHov.hovered ? Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.28) : root.line
            Behavior on border.color { ColorAnimation { duration: Motion.fast } }
            Behavior on color { ColorAnimation { duration: Motion.fast } }
        }
        Column {
            anchors.centerIn: parent
            spacing: (root.roomy ? 5 : 3) * root.s
            GlyphIcon {
                anchors.horizontalCenter: parent.horizontalCenter
                width: (root.roomy ? 19 : 16) * root.s
                height: width
                name: tile.glyph
                stroke: 1.7
                color: tile.accent ? Theme.vermLit : root.ink
            }
            Text {
                anchors.horizontalCenter: parent.horizontalCenter
                text: I18n.tr(tile.label)
                color: root.inkDim
                font.family: Theme.fontPrimary
                font.pixelSize: (root.roomy ? 10 : 8.5) * root.s
                font.weight: Font.Medium
            }
        }
        HoverHandler { id: tHov; cursorShape: Qt.PointingHandCursor }
        MouseArea { id: tTap; anchors.fill: parent; onClicked: tile.tapped() }
        Tips.QsTip { text: tile.tip; below: true; hovered: tHov.hovered }
    }

    // one-line switch row (beautify, edit-after): glyph + label + LinkToggle; the
    // whole row taps to flip.
    component InlineToggle: Item {
        id: it
        property string glyph: ""
        property string label: ""
        property bool on: false
        signal toggled()
        implicitHeight: (root.roomy ? 28 : 20) * root.s
        GlyphIcon {
            id: itIcon
            anchors.left: parent.left
            anchors.verticalCenter: parent.verticalCenter
            width: 14 * root.s
            height: width
            name: it.glyph
            stroke: 1.7
            color: it.on ? Theme.vermLit : root.inkDim
            Behavior on color { ColorAnimation { duration: Motion.fast } }
        }
        Text {
            anchors.left: itIcon.right
            anchors.leftMargin: 8 * root.s
            anchors.right: itSwitch.left
            anchors.rightMargin: 8 * root.s
            anchors.verticalCenter: parent.verticalCenter
            text: I18n.tr(it.label)
            color: root.ink
            font.family: Theme.fontPrimary
            font.pixelSize: 11.5 * root.s
            font.weight: Font.DemiBold
            elide: Text.ElideRight
        }
        LinkToggle {
            id: itSwitch
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            s: root.s
            on: it.on
            onToggled: it.toggled()
        }
        HoverHandler { cursorShape: Qt.PointingHandCursor }
        MouseArea { anchors.fill: parent; onClicked: it.toggled() }
    }

    // small square icon button for the live record controls (pause / stop).
    component MiniBtn: Rectangle {
        id: mb
        property string glyph: ""
        property color tint: root.ink
        signal tapped()
        width: 24 * root.s
        height: width
        radius: 6 * root.s
        color: mbTap.pressed ? Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.16)
            : (mbHov.hovered ? Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.10) : "transparent")
        Behavior on color { ColorAnimation { duration: Motion.fast } }
        GlyphIcon {
            anchors.centerIn: parent
            width: 12 * root.s
            height: width
            name: mb.glyph
            stroke: 1.7
            color: mb.tint
        }
        HoverHandler { id: mbHov; cursorShape: Qt.PointingHandCursor }
        TapHandler { id: mbTap; onTapped: mb.tapped() }
    }

    component Rule: Rectangle {
        implicitHeight: Theme.borderWidth
        color: root.line
    }

    // one recent-capture cell: a screenshot thumbnail, or a recording poster with a
    // play badge + duration, so the groups read differently at a glance. Hairline
    // framed, hover lifts, a click opens the capture folder.
    component RecentTile: Item {
        id: rt
        property var item: null
        property bool video: false
        property string dir: ""
        property real cw: 80 * root.s
        readonly property string src: !rt.item ? ""
            : rt.video ? (root.clipPoster[rt.item.path] ? "file://" + root.clipPoster[rt.item.path] : "")
            : rt.item.url
        width: rt.cw
        height: Math.round(rt.cw * 0.64)
        Rectangle {
            id: frame
            anchors.fill: parent
            radius: 7 * root.s
            clip: true
            color: Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.05)
            border.width: Theme.borderWidth
            border.color: rtHov.hovered ? Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.32) : root.line
            Behavior on border.color { ColorAnimation { duration: Motion.fast } }
            Image {
                id: thumb
                anchors.fill: parent
                source: rt.src
                visible: status === Image.Ready
                fillMode: Image.PreserveAspectCrop
                sourceSize.width: Math.round(rt.cw * 2)
                sourceSize.height: Math.round(rt.cw * 1.4)
                asynchronous: true
                cache: true
            }
            GlyphIcon {
                anchors.centerIn: parent
                visible: thumb.status !== Image.Ready
                width: 24 * root.s
                height: width
                name: rt.video ? "film" : "image"
                stroke: 1.7
                color: root.inkDim
            }
        }
        // play badge: the video signifier, centred over the poster
        Rectangle {
            visible: rt.video
            anchors.centerIn: frame
            width: 28 * root.s
            height: width
            radius: width / 2
            color: Qt.rgba(0, 0, 0, 0.5)
            border.width: Math.max(1, Theme.borderWidth)
            border.color: Qt.rgba(1, 1, 1, 0.75)
            GlyphIcon {
                anchors.centerIn: parent
                anchors.horizontalCenterOffset: 1 * root.s
                width: 14 * root.s
                height: width
                name: "play"
                stroke: 2
                color: "white"
            }
        }
        // duration badge, bottom-right on recordings
        Rectangle {
            visible: rt.video && !!rt.item && !!root.clipDur[rt.item.path]
            anchors.right: frame.right
            anchors.bottom: frame.bottom
            anchors.margins: 4 * root.s
            radius: 3 * root.s
            color: Qt.rgba(0, 0, 0, 0.66)
            width: durTxt.implicitWidth + 8 * root.s
            height: durTxt.implicitHeight + 4 * root.s
            Text {
                id: durTxt
                anchors.centerIn: parent
                text: (rt.item && root.clipDur[rt.item.path]) ? root.clipDur[rt.item.path] : ""
                color: "white"
                font.family: Theme.mono
                font.pixelSize: 10 * root.s
                font.weight: Font.Medium
            }
        }
        HoverHandler { id: rtHov; cursorShape: Qt.PointingHandCursor }
        TapHandler { id: rtTap; onTapped: if (rt.dir) Spawn.runApp(["nautilus", rt.dir], { label: "nautilus" }) }
        scale: rtTap.pressed ? 1.0 : (rtHov.hovered ? 1.03 : 1)
        Behavior on scale { NumberAnimation { duration: Motion.fast } }
    }

    // ── layout ────────────────────────────────────────────────────────────────
    Column {
        id: content
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        anchors.margins: root.pad
        spacing: (root.roomy ? 15 : 8) * root.s

        // SCREENSHOT eyebrow, with the save target + delay chips on the right.
        Item {
            z: 20
            width: parent.width
            height: (root.roomy ? 24 : 19) * root.s
            Eyebrow {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("SCREENSHOT")
            }
            Row {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: 5 * root.s
                CycleChip {
                    anchors.verticalCenter: parent.verticalCenter
                    glyph: root.saveGlyph
                    label: root.saveLabel
                    tip: Capture.save === "clipboard" ? I18n.tr("Clipboard only") : Capture.save === "file" ? I18n.tr("Screenshots folder") : I18n.tr("Folder + clipboard")
                    tipAlign: "right"
                    onTapped: Capture.save = root.cycle(root.saveSteps, Capture.save)
                }
                CycleChip {
                    anchors.verticalCenter: parent.verticalCenter
                    glyph: "watch"
                    label: Capture.delay + "s"
                    tip: Capture.delay === 0 ? I18n.tr("No delay before the shot") : I18n.tr("%1s delay before the shot").arg(Capture.delay)
                    tipAlign: "right"
                    onTapped: Capture.delay = root.cycle(root.delaySteps, Capture.delay)
                }
            }
        }

        // capture modes: All / Screen / Window / Region.
        Row {
            width: parent.width
            spacing: root.gap
            ModeTile { w: (root.innerW - root.gap * 3) / 4; glyph: "screens"; label: I18n.tr("All"); onTapped: root.shoot("all") }
            ModeTile { w: (root.innerW - root.gap * 3) / 4; glyph: "monitor"; label: I18n.tr("Screen"); onTapped: root.shoot("monitor") }
            ModeTile { w: (root.innerW - root.gap * 3) / 4; glyph: "window"; label: I18n.tr("Window"); onTapped: root.shoot("window") }
            ModeTile { w: (root.innerW - root.gap * 3) / 4; glyph: "region"; label: I18n.tr("Region"); onTapped: root.shoot("region") }
        }

        // beautify-after switch.
        InlineToggle {
            width: parent.width
            glyph: "sparkle"
            label: I18n.tr("Beautify after")
            on: Capture.beautify
            onToggled: Capture.beautify = !Capture.beautify
        }

        Rule { width: parent.width }

        // RECORD eyebrow, with capture companions and audio toggles on the right
        // (hidden while a recording is live).
        Item {
            z: 20
            width: parent.width
            height: (root.roomy ? 24 : 19) * root.s
            Eyebrow {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("RECORD")
            }
            Row {
                anchors.right: parent.right
                anchors.verticalCenter: parent.verticalCenter
                spacing: 5 * root.s
                visible: !Recorder.anyActive
                IconToggle {
                    glyph: "keyboard"
                    // A bubble is one line sized to its text, so these stay labels:
                    // a sentence overflows the card and clips at its edge.
                    tip: Keypresses.backendStatus === "error"
                        ? Keypresses.backendError
                        : I18n.tr("Show key presses")
                    on: Keypresses.active
                    onToggled: Keypresses.toggle()
                }
                IconToggle {
                    glyph: "webcam"
                    tip: I18n.tr("Webcam mirror")
                    on: Camera.active
                    onToggled: Camera.toggle()
                }
                IconToggle {
                    glyph: Recorder.optDesktopAudio ? "speaker" : "speaker-off"
                    tip: I18n.tr("Record desktop audio")
                    on: Recorder.optDesktopAudio
                    onToggled: Recorder.optDesktopAudio = !Recorder.optDesktopAudio
                }
                IconToggle {
                    glyph: Recorder.optMic ? "mic" : "mic-off"
                    tip: I18n.tr("Record microphone")
                    on: Recorder.optMic
                    onToggled: Recorder.optMic = !Recorder.optMic
                }
            }
        }

        // record starts: Screen / Monitor / Window / Region (swap for the live
        // indicator when active). Screen records the focused output; the other
        // three raise the shared selection overlay for that family.
        Row {
            width: parent.width
            visible: !Recorder.anyActive
            spacing: root.gap
            ModeTile { w: (root.innerW - root.gap * 3) / 4; glyph: "monitor"; label: I18n.tr("Screen"); accent: true; onTapped: root.record("screen") }
            ModeTile { w: (root.innerW - root.gap * 3) / 4; glyph: "screens"; label: I18n.tr("Monitor"); accent: true; onTapped: root.record("monitor") }
            // The tip is a one-line pill sized to its text, so it has to read as a
            // label, not a sentence: a longer string overflows the card and clips.
            ModeTile { w: (root.innerW - root.gap * 3) / 4; glyph: "window"; label: I18n.tr("Window"); accent: true; tip: I18n.tr("Records the area, not the window"); onTapped: root.record("window") }
            ModeTile { w: (root.innerW - root.gap * 3) / 4; glyph: "region"; label: I18n.tr("Region"); accent: true; onTapped: root.record("region") }
        }

        // live indicator: pulsing REC tag, elapsed clock, stop. The floating island
        // owns the full live controls; this is the card's compact mirror.
        Rectangle {
            width: parent.width
            visible: Recorder.anyActive
            height: 36 * root.s
            radius: 6 * root.s
            color: "transparent"
            border.width: Theme.borderWidth
            border.color: root.line
            Row {
                anchors.left: parent.left
                anchors.leftMargin: 9 * root.s
                anchors.verticalCenter: parent.verticalCenter
                spacing: 8 * root.s
                Rectangle {
                    anchors.verticalCenter: parent.verticalCenter
                    width: 8 * root.s
                    height: width
                    radius: width / 2
                    color: Theme.vermLit
                    SequentialAnimation on opacity {
                        running: Recorder.active
                        loops: Animation.Infinite
                        NumberAnimation { to: 0.25; duration: 620; easing.type: Easing.InOutSine }
                        NumberAnimation { to: 1.0; duration: 620; easing.type: Easing.InOutSine }
                    }
                }
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Recording")
                    color: root.ink
                    font.family: Theme.fontPrimary
                    font.pixelSize: 11.5 * root.s
                    font.weight: Font.DemiBold
                }
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: Recorder.elapsedText
                    color: root.inkDim
                    font.family: Theme.fontPrimary
                    font.pixelSize: 10.5 * root.s
                    font.features: { "tnum": 1 }
                }
            }
            MiniBtn {
                anchors.right: parent.right
                anchors.rightMargin: 7 * root.s
                anchors.verticalCenter: parent.verticalCenter
                glyph: "stop"
                tint: Theme.vermLit
                onTapped: Recorder.stop()
            }
        }

        // recording quality (pre-record only): each chip reads the daemon-owned
        // setting and cycles it through the daemon, which validates and rewrites
        // recording.json; the mirror then reloads and the chip updates.
        Flow {
            width: parent.width
            visible: !Recorder.anyActive
            spacing: 5 * root.s
            CycleChip {
                glyph: "film"
                label: root.codecLabel(Recorder.recCodec)
                tip: I18n.tr("Video codec")
                onTapped: Recorder.setSetting("codec", root.cycle(root.codecSteps, Recorder.recCodec))
            }
            CycleChip {
                glyph: "sparkle"
                label: root.qualityLabel(Recorder.recQuality)
                tip: I18n.tr("Recording quality")
                onTapped: Recorder.setSetting("quality", root.cycle(root.qualitySteps, Recorder.recQuality))
            }
            CycleChip {
                glyph: "watch"
                label: Recorder.recFps + " fps"
                tip: I18n.tr("Frames per second")
                onTapped: Recorder.setSetting("fps", root.cycle(root.fpsSteps, Recorder.recFps))
            }
            CycleChip {
                glyph: "archive"
                label: root.containerLabel(Recorder.recContainer)
                tip: I18n.tr("File format")
                onTapped: Recorder.setSetting("container", root.cycle(root.containerSteps, Recorder.recContainer))
            }
        }
        InlineToggle {
            width: parent.width
            visible: !Recorder.anyActive
            glyph: "mouse"
            label: I18n.tr("Record cursor")
            on: Recorder.recCursor
            onToggled: Recorder.setSetting("cursor", !Recorder.recCursor)
        }
        // Discord makes a size-capped copy of a finished clip.
        InlineToggle {
            width: parent.width
            visible: !Recorder.anyActive
            glyph: "discord"
            label: I18n.tr("Compact for Discord")
            on: Recorder.discordMode
            onToggled: Recorder.discordMode = !Recorder.discordMode
        }
        // the rest of the recording knobs live in the Hub.
        Item {
            width: parent.width
            visible: !Recorder.anyActive
            height: (root.roomy ? 24 : 18) * root.s
            Row {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                spacing: 6 * root.s
                GlyphIcon {
                    anchors.verticalCenter: parent.verticalCenter
                    width: 13 * root.s
                    height: width
                    name: "chevron-right"
                    stroke: 1.7
                    color: moreHov.hovered ? root.ink : root.inkDim
                }
                Text {
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("More recording settings")
                    color: moreHov.hovered ? root.ink : root.inkDim
                    font.family: Theme.fontPrimary
                    font.pixelSize: 11 * root.s
                    font.weight: Font.Medium
                }
            }
            HoverHandler { id: moreHov; cursorShape: Qt.PointingHandCursor }
            TapHandler { onTapped: Bus.run("hub open screen-capture") }
        }

        // RECENT - screenshots and recordings in separate labelled groups; a click
        // on any tile opens that group's folder.
        Rule { width: parent.width; visible: root.recentShots.length > 0 || root.recentClips.length > 0 }

        Item {
            width: parent.width
            height: (root.roomy ? 24 : 19) * root.s
            visible: root.recentShots.length > 0
            Eyebrow {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("SCREENSHOTS")
            }
        }
        Grid {
            id: shotGrid
            width: parent.width
            visible: root.recentShots.length > 0
            columns: 3
            columnSpacing: root.gap
            rowSpacing: root.gap
            readonly property real cw: (root.innerW - root.gap * 2) / 3
            Repeater {
                model: root.recentShots
                delegate: RecentTile {
                    required property var modelData
                    item: modelData
                    video: false
                    dir: Capture.shotsDir
                    cw: shotGrid.cw
                }
            }
        }

        Item {
            width: parent.width
            height: (root.roomy ? 24 : 19) * root.s
            visible: root.recentClips.length > 0
            Eyebrow {
                anchors.left: parent.left
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("RECORDINGS")
            }
        }
        Grid {
            id: clipGrid
            width: parent.width
            visible: root.recentClips.length > 0
            columns: 3
            columnSpacing: root.gap
            rowSpacing: root.gap
            readonly property real cw: (root.innerW - root.gap * 2) / 3
            Repeater {
                model: root.recentClips
                delegate: RecentTile {
                    required property var modelData
                    item: modelData
                    video: true
                    dir: root.recordingsDir
                    cw: clipGrid.cw
                }
            }
        }

        Rule { width: parent.width }

        // hint: the companion beautify/annotate app.
        Text {
            width: parent.width
            text: I18n.tr("Super+Shift+S opens Ryoshot to beautify & annotate.")
            color: root.inkDim
            font.family: Theme.fontPrimary
            font.pixelSize: (root.roomy ? 10 : 9) * root.s
            wrapMode: Text.WordWrap
            lineHeight: 1.1
        }
    }
}
