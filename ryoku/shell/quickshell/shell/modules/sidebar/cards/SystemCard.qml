pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
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

    readonly property real gap: Tokens.s2 * root.s
    readonly property real pad: Tokens.s4 * root.s
    readonly property var controlTiles: [
        {
            glyph: Network.kind === "ethernet" ? "lan" : "wifi",
            label: I18n.tr("Wi-Fi"),
            detail: !Network.wifiPresent ? I18n.tr("Unavailable") : !Toggles.wifiOn ? I18n.tr("Off") : Network.activeSsid !== "" ? Network.activeSsid : I18n.tr("On"),
            active: Toggles.wifiOn,
            armed: Network.wifiPresent,
            action: () => Toggles.toggleWifi()
        },
        {
            glyph: "bluetooth",
            label: I18n.tr("Bluetooth"),
            detail: Toggles.btAdapter ? (Toggles.btOn ? I18n.tr("On") : I18n.tr("Off")) : I18n.tr("Unavailable"),
            active: Toggles.btOn,
            armed: !!Toggles.btAdapter,
            action: () => Toggles.toggleBt()
        },
        {
            glyph: Audio.sink && Audio.sink.audio && Audio.sink.audio.muted ? "volume_off" : "volume_up",
            label: I18n.tr("Audio"),
            detail: !Audio.sink || !Audio.sink.audio ? I18n.tr("Unavailable") : Audio.sink.audio.muted ? I18n.tr("Muted") : Math.round(Audio.sink.audio.volume * 100) + "%",
            active: !!(Audio.sink && Audio.sink.audio && !Audio.sink.audio.muted),
            armed: !!(Audio.sink && Audio.sink.audio),
            action: () => { if (Audio.sink && Audio.sink.audio) Audio.sink.audio.muted = !Audio.sink.audio.muted; }
        },
        {
            glyph: Toggles.micMuted ? "mic_off" : "mic",
            label: I18n.tr("Microphone"),
            detail: !Audio.source || !Audio.source.audio ? I18n.tr("Unavailable") : Toggles.micMuted ? I18n.tr("Muted") : Math.round(Audio.source.audio.volume * 100) + "%",
            active: !!(Audio.source && Audio.source.audio && !Toggles.micMuted),
            armed: !!(Audio.source && Audio.source.audio),
            action: () => Toggles.toggleMic()
        },
        {
            glyph: "do_not_disturb_on",
            label: I18n.tr("Do not disturb"),
            detail: Toggles.dnd ? I18n.tr("On") : I18n.tr("Off"),
            active: Toggles.dnd,
            armed: true,
            action: () => Toggles.toggleDnd()
        },
        {
            glyph: "bedtime",
            label: I18n.tr("Night light"),
            detail: Toggles.nightOn ? I18n.tr("On") : I18n.tr("Off"),
            active: Toggles.nightOn,
            armed: Wm.caps.nightLight === true,
            action: () => Toggles.toggleNight()
        },
        {
            glyph: "coffee",
            label: I18n.tr("Keep awake"),
            detail: Toggles.keepAwake ? I18n.tr("On") + root.awakeFor() : I18n.tr("Off"),
            active: Toggles.keepAwake,
            armed: true,
            action: () => Toggles.toggleCaffeine()
        },
        {
            glyph: "sports_esports",
            label: I18n.tr("Game mode"),
            detail: !Toggles.gameMode && !Battery.onAc ? I18n.tr("Needs AC") : Toggles.gameMode ? I18n.tr("On") : I18n.tr("Off"),
            active: Toggles.gameMode,
            armed: Toggles.gameMode || (Battery.onAc && Wm.caps.liveConfigEval === true),
            action: () => Toggles.toggleGame()
        }
    ]

    function greeting(): string {
        const h = clock.date.getHours();
        if (h < 5)
            return I18n.tr("Good night");
        if (h < 12)
            return I18n.tr("Good morning");
        if (h < 18)
            return I18n.tr("Good afternoon");
        return I18n.tr("Good evening");
    }

    function awakeFor(): string {
        const since = Flags.keepAwakeSince;
        if (!since || since <= 0)
            return "";
        const mins = Math.floor((clock.date.getTime() - since) / 60000);
        if (mins < 1)
            return "";
        if (mins < 60)
            return " " + I18n.tr("for %1m").arg(mins);
        if (mins < 1440)
            return " " + I18n.tr("for %1h").arg(Math.floor(mins / 60));
        return " " + I18n.tr("for %1d").arg(Math.floor(mins / 1440));
    }

    function profileLabel(profile): string {
        if (profile === "power-saver")
            return I18n.tr("Saver");
        if (profile === "balanced")
            return I18n.tr("Balanced");
        if (profile === "performance")
            return I18n.tr("Performance");
        return profile;
    }

    function profileLabels(): var {
        const labels = {};
        for (let i = 0; i < PowerProfiles.profiles.length; i++)
            labels[PowerProfiles.profiles[i]] = root.profileLabel(PowerProfiles.profiles[i]);
        return labels;
    }

    function sessionAction(action): void {
        root.requestClose();
        ShellState.askSessionAction(action, "");
    }

    implicitHeight: shell.implicitHeight

    SystemClock {
        id: clock
        precision: SystemClock.Minutes
        enabled: root.open && root.tabActive
    }

    component Tile: Rectangle {
        id: tile
        required property string glyph
        required property string label
        required property string detail
        required property bool active
        required property bool armed
        required property var action

        implicitHeight: 78 * root.s
        radius: Tokens.radius * root.s
        color: tile.active ? Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, tileTap.pressed ? 0.25 : tileHover.hovered ? 0.19 : 0.14)
            : tileTap.pressed ? Tokens.tint16 : tileHover.hovered ? Tokens.tint10 : Tokens.tint5
        border.width: Tokens.border
        border.color: tile.active ? Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.56) : tileHover.hovered ? Tokens.lineStrong : Tokens.line
        opacity: tile.armed ? 1 : 0.46
        scale: tileTap.pressed ? 0.98 : tileHover.hovered && !Tokens.reduceMotion ? 1.012 : 1
        transformOrigin: Item.Center
        Behavior on color { ColorAnimation { duration: Tokens.snap } }
        Behavior on scale { NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap } }

        Text {
            anchors.left: parent.left
            anchors.top: parent.top
            anchors.margins: Tokens.s3 * root.s
            text: tile.glyph
            color: tile.active ? Tokens.sun : Tokens.inkDim
            font.family: "Material Symbols Rounded"
            font.pixelSize: 23 * root.s
        }
        Column {
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.margins: Tokens.s3 * root.s
            spacing: Tokens.s1 * root.s
            Text {
                width: parent.width
                text: tile.label
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fRow * root.s
                font.weight: Font.DemiBold
                elide: Text.ElideRight
            }
            Text {
                width: parent.width
                text: tile.detail
                color: Tokens.inkMuted
                font.family: Tokens.ui
                font.pixelSize: Tokens.fMicro * root.s
                elide: Text.ElideRight
            }
        }
        HoverHandler { id: tileHover; enabled: tile.armed; cursorShape: Qt.PointingHandCursor }
        TapHandler { id: tileTap; enabled: tile.armed; onTapped: tile.action() }
    }

    component SessionButton: Rectangle {
        id: sessionBtn
        required property string glyph
        required property string label
        required property var action

        implicitWidth: Math.max(62 * root.s, sessionLabel.implicitWidth + Tokens.s4 * root.s)
        implicitHeight: 42 * root.s
        radius: Tokens.radius * root.s
        color: sessionTap.pressed ? Tokens.tint16 : sessionHover.hovered ? Tokens.tint10 : "transparent"
        border.width: Tokens.border
        border.color: sessionHover.hovered ? Tokens.lineStrong : Tokens.line
        scale: sessionTap.pressed ? 0.98 : 1
        Behavior on scale { NumberAnimation { duration: Tokens.snap } }
        Row {
            anchors.centerIn: parent
            spacing: Tokens.s1 * root.s
            Text {
                text: sessionBtn.glyph
                color: Tokens.inkDim
                font.family: "Material Symbols Rounded"
                font.pixelSize: 17 * root.s
            }
            Text {
                id: sessionLabel
                text: sessionBtn.label
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fMicro * root.s
                font.weight: Font.Medium
            }
        }
        HoverHandler { id: sessionHover; cursorShape: Qt.PointingHandCursor }
        TapHandler { id: sessionTap; onTapped: sessionBtn.action() }
    }

    SidebarCardShell {
        id: shell
        width: root.width
        index: root.index
        open: root.open
        reveal: root.reveal
        tabActive: root.tabActive
        title: I18n.tr("System")
        glyph: "tune"
        eyebrow: I18n.tr("CONTROL DECK")

        Column {
            width: parent.width
            spacing: Tokens.s4 * root.s

            Rectangle {
                width: parent.width
                implicitHeight: heroContent.implicitHeight + Tokens.s5 * root.s * 2
                radius: Tokens.radius * root.s
                border.width: Tokens.border
                border.color: Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.38)
                gradient: Gradient {
                    orientation: Gradient.Horizontal
                    GradientStop { position: 0; color: Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.22) }
                    GradientStop { position: 0.55; color: Qt.rgba(Tokens.sunDeep.r, Tokens.sunDeep.g, Tokens.sunDeep.b, 0.10) }
                    GradientStop { position: 1; color: Tokens.paperLift }
                }

                Row {
                    id: heroContent
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    anchors.margins: root.pad
                    spacing: Tokens.s4 * root.s

                    Column {
                        width: parent.width - clockText.width - Tokens.s4 * root.s
                        spacing: Tokens.s1 * root.s
                        Text {
                            width: parent.width
                            text: root.greeting()
                            color: Tokens.ink
                            font.family: Tokens.display
                            font.pixelSize: Tokens.fHero * root.s
                            font.weight: Font.Medium
                            wrapMode: Text.Wrap
                        }
                        Text {
                            width: parent.width
                            text: Qt.locale().toString(clock.date, "dddd") + " · " + Qt.formatDate(clock.date, "MMM d")
                            color: Tokens.inkMuted
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fSmall * root.s
                            elide: Text.ElideRight
                        }
                    }
                    Text {
                        id: clockText
                        anchors.verticalCenter: parent.verticalCenter
                        text: Qt.formatTime(clock.date, "HH:mm")
                        color: Tokens.sun
                        font.family: Tokens.mono
                        font.pixelSize: Tokens.fValue * root.s
                        font.weight: Font.DemiBold
                        font.features: ({ "tnum": 1 })
                    }
                }
            }

            Rectangle {
                width: parent.width
                implicitHeight: powerColumn.implicitHeight + root.pad * 2
                radius: Tokens.radius * root.s
                color: Tokens.tint5
                border.width: Tokens.border
                border.color: Tokens.line

                Column {
                    id: powerColumn
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    anchors.margins: root.pad
                    spacing: Tokens.s3 * root.s

                    Row {
                        width: parent.width
                        spacing: Tokens.s3 * root.s
                        Text {
                            anchors.verticalCenter: parent.verticalCenter
                            text: Battery.present ? (Battery.charging ? "battery_charging_full" : "battery_full") : "power"
                            color: Battery.low ? Tokens.alert : Tokens.sun
                            font.family: "Material Symbols Rounded"
                            font.pixelSize: 30 * root.s
                        }
                        Column {
                            anchors.verticalCenter: parent.verticalCenter
                            width: parent.width - (42 * root.s)
                            spacing: Tokens.s1 * root.s
                            Text {
                                width: parent.width
                                text: Battery.present ? Battery.pct + "% · " + Battery.stateLabel : I18n.tr("Desktop power")
                                color: Tokens.ink
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fValue * root.s
                                font.weight: Font.Medium
                                elide: Text.ElideRight
                            }
                            Text {
                                width: parent.width
                                text: Battery.present && Battery.hasTime ? Battery.timeStr : PowerProfiles.available ? root.profileLabel(PowerProfiles.profile) : I18n.tr("Power profile unavailable")
                                color: Tokens.inkMuted
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fMicro * root.s
                                elide: Text.ElideRight
                            }
                        }
                    }
                    Seg {
                        visible: PowerProfiles.available
                        width: parent.width
                        options: PowerProfiles.profiles
                        labels: root.profileLabels()
                        current: PowerProfiles.profile
                        onChose: profile => PowerProfiles.setProfile(profile)
                    }
                }
            }

            Grid {
                id: tileGrid
                width: parent.width
                columns: 2
                columnSpacing: root.gap
                rowSpacing: root.gap
                Repeater {
                    model: root.controlTiles
                    delegate: Tile {
                        required property var modelData
                        width: (tileGrid.width - tileGrid.columnSpacing) / 2
                        glyph: modelData.glyph
                        label: modelData.label
                        detail: modelData.detail
                        active: modelData.active
                        armed: modelData.armed
                        action: modelData.action
                    }
                }
            }

            Flow {
                width: parent.width
                spacing: root.gap
                SessionButton {
                    glyph: "lock"
                    label: I18n.tr("Lock")
                    action: () => {
                        root.requestClose();
                        Spawn.run(["ryoku-shell", "lock"]);
                    }
                }
                SessionButton { glyph: "logout"; label: I18n.tr("Log out"); action: () => root.sessionAction("logout") }
                SessionButton { glyph: "restart_alt"; label: I18n.tr("Reboot"); action: () => root.sessionAction("reboot") }
                SessionButton { glyph: "power_settings_new"; label: I18n.tr("Shut down"); action: () => root.sessionAction("shutdown") }
            }
        }
    }
}
