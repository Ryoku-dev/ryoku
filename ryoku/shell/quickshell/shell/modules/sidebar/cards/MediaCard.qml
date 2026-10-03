pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell.Services.Mpris
import shell.services
import Ryoku.Ui
import Ryoku.Ui.Singletons
import ".."

Item {
    id: root

    required property real s
    required property bool open
    required property real reveal
    required property bool tabActive
    property int index: 0
    signal requestClose()

    readonly property var players: Mpris.players.values.filter(player => player && !Media.isWallpaper(player))
    property var picked: null
    readonly property var player: root.picked && root.players.indexOf(root.picked) >= 0 ? root.picked : Media.player
    readonly property int playerIndex: root.player ? root.players.indexOf(root.player) : -1
    property real pendingSeek: -1
    readonly property real length: root.player && root.player.length > 0 ? root.player.length : 0
    readonly property real position: root.pendingSeek >= 0 ? root.pendingSeek : root.player ? root.player.position : 0
    readonly property real gap: Tokens.s3 * root.s
    readonly property real pad: Tokens.s4 * root.s

    implicitHeight: shell.implicitHeight

    function artist(player): string {
        if (!player)
            return "";
        if (player.trackArtists && player.trackArtists.length > 0)
            return player.trackArtists.join(", ");
        return player.trackArtist || "";
    }

    function fmt(seconds): string {
        const sec = Math.max(0, Math.floor(seconds || 0));
        const h = Math.floor(sec / 3600);
        const m = Math.floor((sec % 3600) / 60);
        const s = sec % 60;
        const ss = (s < 10 ? "0" : "") + s;
        return h > 0 ? h + ":" + (m < 10 ? "0" : "") + m + ":" + ss : m + ":" + ss;
    }

    function pickOffset(offset): void {
        if (root.players.length === 0)
            return;
        const index = root.playerIndex < 0 ? 0 : root.playerIndex;
        root.picked = root.players[(index + offset + root.players.length) % root.players.length];
        root.pendingSeek = -1;
    }

    function loopGlyph(): string {
        if (!root.player)
            return "repeat";
        if (root.player.loopState === MprisLoopState.Track)
            return "repeat_one_on";
        if (root.player.loopState === MprisLoopState.Playlist)
            return "repeat_on";
        return "repeat";
    }

    function cycleLoop(): void {
        if (!root.player || !root.player.loopSupported)
            return;
        if (root.player.loopState === MprisLoopState.None)
            root.player.loopState = MprisLoopState.Playlist;
        else if (root.player.loopState === MprisLoopState.Playlist)
            root.player.loopState = MprisLoopState.Track;
        else
            root.player.loopState = MprisLoopState.None;
    }

    onPlayerChanged: root.pendingSeek = -1

    Timer {
        interval: 500
        repeat: true
        running: root.open && root.tabActive && root.player && root.player.isPlaying
        onTriggered: if (root.player) root.player.positionChanged()
    }
    Timer {
        id: seekDebounce
        interval: 300
        onTriggered: {
            if (root.player && root.player.canSeek && root.pendingSeek >= 0)
                root.player.position = root.pendingSeek;
            root.pendingSeek = -1;
        }
    }

    component TransportButton: Rectangle {
        id: transport
        required property string glyph
        required property bool armed
        required property var action
        property bool hero: false

        width: (transport.hero ? 52 : 40) * root.s
        height: width
        radius: width / 2
        opacity: transport.armed ? 1 : 0.32
        color: transport.hero ? Tokens.bone : transportTap.pressed ? Tokens.tint16 : transportHover.hovered ? Tokens.tint10 : Tokens.tint5
        border.width: Tokens.border
        border.color: transport.hero ? Tokens.bone : transportHover.hovered ? Tokens.lineStrong : Tokens.line
        scale: transportTap.pressed ? 0.96 : transportHover.hovered && !Tokens.reduceMotion ? 1.04 : 1
        Behavior on color { ColorAnimation { duration: Tokens.snap } }
        Behavior on scale { NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap } }
        Text {
            anchors.centerIn: parent
            text: transport.glyph
            color: transport.hero ? Tokens.inkOnBone : Tokens.ink
            font.family: "Material Symbols Rounded"
            font.pixelSize: (transport.hero ? 28 : 21) * root.s
        }
        HoverHandler { id: transportHover; enabled: transport.armed; cursorShape: Qt.PointingHandCursor }
        TapHandler { id: transportTap; enabled: transport.armed; onTapped: transport.action() }
    }

    SidebarCardShell {
        id: shell
        width: root.width
        index: root.index
        open: root.open
        reveal: root.reveal
        tabActive: root.tabActive
        title: I18n.tr("Media")
        glyph: "play_circle"
        eyebrow: root.players.length > 1 ? I18n.tr("%1 PLAYERS").arg(root.players.length) : I18n.tr("NOW PLAYING")

        Column {
            width: parent.width
            spacing: root.gap

            Rectangle {
                visible: root.player === null
                width: parent.width
                implicitHeight: emptyColumn.implicitHeight + Tokens.s7 * root.s
                radius: Tokens.radius * root.s
                color: Tokens.tint5
                border.width: Tokens.border
                border.color: Tokens.line
                Column {
                    id: emptyColumn
                    anchors.centerIn: parent
                    spacing: Tokens.s2 * root.s
                    Text {
                        anchors.horizontalCenter: parent.horizontalCenter
                        text: "music_off"
                        color: Tokens.inkMuted
                        font.family: "Material Symbols Rounded"
                        font.pixelSize: 40 * root.s
                    }
                    Text {
                        anchors.horizontalCenter: parent.horizontalCenter
                        text: I18n.tr("Nothing is playing")
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall * root.s
                    }
                }
            }

            Rectangle {
                visible: root.player !== null
                width: parent.width
                implicitHeight: 236 * root.s
                radius: Tokens.radius * root.s
                clip: true
                color: Tokens.paperLift
                border.width: Tokens.border
                border.color: Tokens.line

                Image {
                    id: artwork
                    anchors.fill: parent
                    source: root.player ? root.player.trackArtUrl || "" : ""
                    fillMode: Image.PreserveAspectCrop
                    asynchronous: true
                    cache: true
                    visible: status === Image.Ready
                    opacity: 0.52
                }
                MultiEffect {
                    anchors.fill: artwork
                    source: artwork
                    visible: artwork.visible
                    blurEnabled: true
                    blur: 0.38
                    saturation: 0.55
                }
                Rectangle {
                    anchors.fill: parent
                    gradient: Gradient {
                        orientation: Gradient.Vertical
                        GradientStop { position: 0; color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.18) }
                        GradientStop { position: 0.5; color: Qt.rgba(Tokens.paper.r, Tokens.paper.g, Tokens.paper.b, 0.56) }
                        GradientStop { position: 1; color: Tokens.paper }
                    }
                }
                Text {
                    visible: !artwork.visible
                    anchors.right: parent.right
                    anchors.top: parent.top
                    anchors.rightMargin: -15 * root.s
                    anchors.topMargin: -25 * root.s
                    text: "graphic_eq"
                    color: Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.12)
                    font.family: "Material Symbols Rounded"
                    font.pixelSize: 170 * root.s
                }

                Column {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.bottom: parent.bottom
                    anchors.margins: root.pad
                    spacing: Tokens.s1 * root.s
                    Text {
                        width: parent.width
                        text: root.player ? root.player.trackTitle || I18n.tr("Untitled") : ""
                        color: Tokens.ink
                        font.family: Tokens.display
                        font.pixelSize: Tokens.fHero * root.s
                        font.weight: Font.Medium
                        elide: Text.ElideRight
                    }
                    Text {
                        width: parent.width
                        text: root.artist(root.player)
                        color: Tokens.inkDim
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fSmall * root.s
                        font.weight: Font.Medium
                        elide: Text.ElideRight
                    }
                    Text {
                        width: parent.width
                        text: root.player ? (root.player.identity || root.player.dbusName || "") : ""
                        color: Tokens.inkMuted
                        font.family: Tokens.ui
                        font.pixelSize: Tokens.fMicro * root.s
                        font.letterSpacing: Tokens.trackLabel
                        elide: Text.ElideRight
                    }
                }
            }

            Item {
                visible: root.players.length > 1
                width: parent.width
                implicitHeight: Math.max(playerName.implicitHeight, playerSwitch.implicitHeight)
                Text {
                    id: playerName
                    anchors.left: parent.left
                    anchors.right: playerSwitch.left
                    anchors.rightMargin: Tokens.s3 * root.s
                    anchors.verticalCenter: parent.verticalCenter
                    text: root.player ? root.player.identity || root.player.dbusName : ""
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro * root.s
                    font.weight: Font.DemiBold
                    elide: Text.ElideRight
                }
                Row {
                    id: playerSwitch
                    anchors.right: parent.right
                    spacing: Tokens.s1 * root.s
                    TransportButton { glyph: "chevron_left"; armed: true; action: () => root.pickOffset(-1) }
                    TransportButton { glyph: "chevron_right"; armed: true; action: () => root.pickOffset(1) }
                }
            }

            Column {
                visible: root.player !== null
                width: parent.width
                spacing: Tokens.s2 * root.s
                Slid {
                    width: parent.width
                    from: 0
                    to: Math.max(1, root.length)
                    value: Math.min(root.length, root.position)
                    onModified: value => {
                        root.pendingSeek = value;
                        seekDebounce.restart();
                    }
                }
                Item {
                    width: parent.width
                    implicitHeight: Math.max(elapsed.implicitHeight, total.implicitHeight)
                    Text {
                        id: elapsed
                        anchors.left: parent.left
                        text: root.fmt(root.position)
                        color: Tokens.inkMuted
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fTiny * root.s
                        font.features: ({ "tnum": 1 })
                    }
                    Text {
                        id: total
                        anchors.right: parent.right
                        text: root.fmt(root.length)
                        color: Tokens.inkMuted
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fTiny * root.s
                        font.features: ({ "tnum": 1 })
                    }
                }
            }

            Row {
                visible: root.player !== null
                anchors.horizontalCenter: parent.horizontalCenter
                spacing: Tokens.s3 * root.s
                TransportButton {
                    glyph: root.player && root.player.shuffle ? "shuffle_on" : "shuffle"
                    armed: !!(root.player && root.player.shuffleSupported)
                    action: () => { if (root.player) root.player.shuffle = !root.player.shuffle; }
                }
                TransportButton {
                    glyph: "skip_previous"
                    armed: !!(root.player && root.player.canGoPrevious)
                    action: () => { if (root.player) root.player.previous(); }
                }
                TransportButton {
                    glyph: root.player && root.player.isPlaying ? "pause" : "play_arrow"
                    hero: true
                    armed: !!(root.player && root.player.canTogglePlaying)
                    action: () => { if (root.player) root.player.togglePlaying(); }
                }
                TransportButton {
                    glyph: "skip_next"
                    armed: !!(root.player && root.player.canGoNext)
                    action: () => { if (root.player) root.player.next(); }
                }
                TransportButton {
                    glyph: root.loopGlyph()
                    armed: !!(root.player && root.player.loopSupported)
                    action: () => root.cycleLoop()
                }
            }
        }
    }
}
