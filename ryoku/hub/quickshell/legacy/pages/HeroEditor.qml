pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons

// The Profile hero editor: pick the plate's hero from the baked art set or your
// own image, frame it (draggable focal point + zoom), and, for a custom image,
// tune the live bone dither (strength + invert). Everything writes to
// ProfileStore at once, so the plate behind the editor updates live. A custom
// pick is copied into ~/.config/ryoku/profile/ so it persists and can travel in
// a .ryoprofile. The baked marble default and the gallery art are already 1-bit
// bone, so they show raw; only a custom (raw-colour) image runs the dither shader.
Item {
    id: ed
    signal done()

    readonly property string profileDir: (Quickshell.env("HOME") || "") + "/.config/ryoku/profile"

    property string heroKind: ProfileStore.get("hero.kind", "default")
    property string heroSource: ProfileStore.get("hero.source", "")
    property real focalX: ProfileStore.get("hero.focalX", 0.5)
    property real focalY: ProfileStore.get("hero.focalY", 0.4)
    property real zoomV: ProfileStore.get("hero.zoom", 1.0)
    property real ditherV: ProfileStore.get("hero.dither", 1.0)
    property bool invertV: ProfileStore.get("hero.invert", false)

    function commit() {
        ProfileStore.put({ hero: {
            kind: ed.heroKind, source: ed.heroSource,
            focalX: ed.focalX, focalY: ed.focalY, zoom: ed.zoomV,
            dither: ed.ditherV, invert: ed.invertV } });
    }

    function heroUrl(kind, src) {
        if (kind === "custom")
            return src.length > 0 ? ("file://" + ed.profileDir + "/" + src) : "";
        if (kind === "gallery")
            return src.length > 0 ? (Ryodecors.dir + src) : "";
        return Qt.resolvedUrl("../art/profile-hero.png");
    }

    readonly property var galleryArt: [
        { name: "david.png" }, { name: "aurelius.png" }, { name: "laocoon.png" },
        { name: "moon.png" }, { name: "needle.png" }, { name: "katana.png" },
        { name: "hawk.png" }, { name: "mic.png" }, { name: "camera.png" },
        { name: "rashin-hero.png" }
    ]

    // copy a picked file into the profile dir as hero.<ext>, then adopt it.
    Process {
        id: copyProc
        property string ext: "png"
        onExited: function (code, status) {
            if (code === 0) {
                ed.heroKind = "custom";
                ed.heroSource = "hero." + copyProc.ext;
                ed.focalX = 0.5; ed.focalY = 0.5; ed.zoomV = 1.0;
                ed.commit();
            }
        }
    }
    function adoptCustom(path) {
        var p = String(path).replace(/^file:\/\//, "");
        var dot = p.lastIndexOf(".");
        copyProc.ext = (dot >= 0 ? p.slice(dot + 1) : "png").toLowerCase();
        copyProc.command = ["sh", "-c",
            "mkdir -p " + ed.profileDir + " && cp '" + p + "' " + ed.profileDir + "/hero." + copyProc.ext];
        copyProc.running = true;
    }

    PickFile {
        id: pick
        title: I18n.tr("Choose a hero image")
        onPicked: (p) => { ed.adoptCustom(p); pick.active = false; }
        onCanceled: pick.active = false
    }

    Rectangle {
        anchors.fill: parent
        color: Tokens.paperLift
        border.width: Tokens.border
        border.color: Tokens.lineStrong
        radius: Tokens.radius

        Row {
            anchors.fill: parent
            anchors.margins: Tokens.s5
            spacing: Tokens.s5

            // preview: the framed, dithered hero. Drag to move the focal point.
            Rectangle {
                id: frame
                width: parent.width * 0.42
                height: parent.height
                color: Tokens.paper
                border.width: Tokens.border
                border.color: Tokens.line
                clip: true

                Item {
                    id: framed
                    width: frame.width * ed.zoomV
                    height: frame.height * ed.zoomV
                    x: (frame.width - width) * ed.focalX
                    y: (frame.height - height) * ed.focalY

                    DitherImage {
                        anchors.fill: parent
                        visible: ed.heroKind === "custom"
                        source: ed.heroUrl("custom", ed.heroSource)
                        dotScale: ed.ditherV
                        invert: ed.invertV
                        fillMode: Image.PreserveAspectCrop
                    }
                    Image {
                        anchors.fill: parent
                        visible: ed.heroKind !== "custom"
                        source: ed.heroUrl(ed.heroKind, ed.heroSource)
                        fillMode: Image.PreserveAspectCrop
                        smooth: false
                        asynchronous: true
                    }
                }

                MouseArea {
                    anchors.fill: parent
                    cursorShape: Qt.OpenHandCursor
                    property real ox: 0
                    property real oy: 0
                    onPressed: function (m) { ox = m.x; oy = m.y; }
                    onPositionChanged: function (m) {
                        ed.focalX = Math.max(0, Math.min(1, ed.focalX - (m.x - ox) / Math.max(1, frame.width)));
                        ed.focalY = Math.max(0, Math.min(1, ed.focalY - (m.y - oy) / Math.max(1, frame.height)));
                        ox = m.x; oy = m.y;
                        ed.commit();
                    }
                }
                Text {
                    anchors.left: parent.left; anchors.bottom: parent.bottom; anchors.margins: Tokens.s2
                    text: I18n.tr("drag to frame")
                    color: Tokens.inkFaint; font.family: Tokens.ui; font.pixelSize: Tokens.fTiny
                }
            }

            // controls
            Flickable {
                width: parent.width - frame.width - Tokens.s5
                height: parent.height
                contentWidth: width
                contentHeight: ctl.height
                clip: true
                ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
                WheelScroll { }

                Column {
                    id: ctl
                    width: parent.width - Tokens.s3
                    spacing: Tokens.s4

                    Text {
                        text: I18n.tr("HERO")
                        color: Tokens.inkMuted; font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro; font.weight: Font.Medium
                        font.letterSpacing: Tokens.trackMark; font.capitalization: Font.AllUppercase
                    }

                    Flow {
                        width: parent.width
                        spacing: Tokens.s2
                        HeroTile {
                            label: I18n.tr("MARBLE"); selected: ed.heroKind === "default"
                            art: Qt.resolvedUrl("../art/profile-hero.png")
                            onPicked: { ed.heroKind = "default"; ed.heroSource = ""; ed.commit(); }
                        }
                        HeroTile {
                            label: I18n.tr("YOUR IMAGE"); plus: true; selected: ed.heroKind === "custom"
                            art: ed.heroKind === "custom" ? ed.heroUrl("custom", ed.heroSource) : ""
                            onPicked: pick.open()
                        }
                        Repeater {
                            model: ed.galleryArt
                            delegate: HeroTile {
                                required property var modelData
                                label: modelData.name.split(".")[0].toUpperCase()
                                art: Ryodecors.dir + modelData.name
                                selected: ed.heroKind === "gallery" && ed.heroSource === modelData.name
                                onPicked: { ed.heroKind = "gallery"; ed.heroSource = modelData.name; ed.commit(); }
                            }
                        }
                    }

                    Rectangle {
                        width: parent.width; height: 40; radius: Tokens.radius
                        color: drop.containsDrag ? Tokens.tint10 : "transparent"
                        border.width: Tokens.border; border.color: Tokens.line
                        Text {
                            anchors.centerIn: parent; text: I18n.tr("drop an image here")
                            color: Tokens.inkFaint; font.family: Tokens.ui; font.pixelSize: Tokens.fSmall
                        }
                        DropArea {
                            id: drop; anchors.fill: parent
                            onDropped: function (d) { if (d.hasUrls && d.urls.length > 0) ed.adoptCustom(d.urls[0]); }
                        }
                    }

                    LblSlider {
                        width: parent.width; label: I18n.tr("ZOOM"); from: 1.0; to: 3.0; value: ed.zoomV
                        onMoved: function (v) { ed.zoomV = v; ed.commit(); }
                    }
                    LblSlider {
                        width: parent.width; visible: ed.heroKind === "custom"
                        label: I18n.tr("DITHER"); from: 1.0; to: 6.0; value: ed.ditherV
                        onMoved: function (v) { ed.ditherV = v; ed.commit(); }
                    }
                    Row {
                        width: parent.width; visible: ed.heroKind === "custom"; spacing: Tokens.s3
                        Text {
                            anchors.verticalCenter: parent.verticalCenter; text: I18n.tr("INVERT")
                            color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: Tokens.fMicro
                            font.letterSpacing: Tokens.trackLabel; font.capitalization: Font.AllUppercase
                        }
                        Sw {
                            anchors.verticalCenter: parent.verticalCenter; on: ed.invertV
                            onToggled: function (v) { ed.invertV = v; ed.commit(); }
                        }
                    }

                    // Power-user hint: the GUI dithers live, but the same dither
                    // is a CLI, and everything is hand-editable. Kept self-contained
                    // (the command + path) so it helps even where docs aren't shipped.
                    Rectangle {
                        width: parent.width
                        height: hint.implicitHeight + Tokens.s4
                        color: "transparent"
                        radius: Tokens.radius
                        border.width: Tokens.border
                        border.color: Tokens.line
                        Text {
                            id: hint
                            anchors.centerIn: parent
                            width: parent.width - Tokens.s4
                            wrapMode: Text.WordWrap
                            lineHeight: 1.35
                            color: Tokens.inkFaint
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fTiny
                            text: I18n.tr("A custom image is dithered live. To bake your own art into this gallery:\n  ryodither <image> --out %1/ryodecors\n(needs Pillow; add --invert for a dark subject on a pale ground). Hand-tune any field in ~/.config/ryoku/profile.json. Full guide: docs/profile.md.").arg(Paths.picturesDir)
                        }
                    }

                    Btn { text: I18n.tr("DONE"); primary: true; onAct: ed.done() }
                }
            }
        }
    }

    component HeroTile: Rectangle {
        id: tile
        property string label: ""
        property url art: ""
        property bool selected: false
        property bool plus: false
        signal picked()
        width: 78; height: 78; radius: Tokens.radius
        color: Tokens.paper
        border.width: tile.selected ? Tokens.border * 2 : Tokens.border
        border.color: tile.selected ? Tokens.ink : Tokens.line
        Image {
            anchors.fill: parent; anchors.margins: Tokens.s2
            source: tile.art; fillMode: Image.PreserveAspectFit
            smooth: false; asynchronous: true
            visible: !tile.plus || tile.art.toString().length > 0
        }
        Text {
            anchors.centerIn: parent; visible: tile.plus && tile.art.toString().length === 0
            text: "+"; color: Tokens.inkMuted; font.family: Tokens.ui; font.pixelSize: 24
        }
        Text {
            anchors.bottom: parent.bottom; anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottomMargin: 2
            text: I18n.tr(tile.label); color: Tokens.inkFaint; font.family: Tokens.ui; font.pixelSize: 8
            font.letterSpacing: Tokens.trackLabel
        }
        MouseArea { anchors.fill: parent; cursorShape: Qt.PointingHandCursor; onClicked: tile.picked() }
    }

    component LblSlider: Column {
        id: ls
        property string label: ""
        property real from: 0
        property real to: 1
        property real value: 0
        signal moved(real v)
        spacing: Tokens.s1
        Text {
            text: I18n.tr(ls.label); color: Tokens.inkMuted; font.family: Tokens.ui
            font.pixelSize: Tokens.fMicro; font.letterSpacing: Tokens.trackLabel
            font.capitalization: Font.AllUppercase
        }
        Slid {
            width: ls.width; from: ls.from; to: ls.to; value: ls.value
            onModified: function (v) { ls.moved(v); }
        }
    }
}
