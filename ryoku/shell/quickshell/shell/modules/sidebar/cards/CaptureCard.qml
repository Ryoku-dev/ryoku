pragma ComponentBehavior: Bound

import QtQuick
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

    readonly property real gap: Tokens.s3 * root.s
    readonly property real pad: Tokens.s4 * root.s
    readonly property var delaySteps: [0, 1, 3, 5, 10]
    readonly property var codecSteps: ["h264", "hevc", "av1"]
    readonly property var qualitySteps: ["medium", "high", "very_high", "ultra"]
    readonly property var fpsSteps: [30, 60, 120]
    readonly property var containerSteps: ["mp4", "mkv", "webm"]

    implicitHeight: shell.implicitHeight

    function shoot(mode): void {
        root.requestClose();
        Capture.shoot(mode);
    }

    function record(mode): void {
        root.requestClose();
        if (mode === "screen")
            Recorder.startAfter(Recorder.recordArgs(), Capture.delay);
        else
            Capture.recordTarget(mode, Recorder.recordArgs());
    }

    function cycleDelay(): void {
        const at = root.delaySteps.indexOf(Capture.delay);
        Capture.delay = root.delaySteps[(at + 1) % root.delaySteps.length];
    }

    function codecLabel(codec): string {
        return codec === "hevc" ? "HEVC" : codec === "av1" ? "AV1" : "H.264";
    }

    function qualityLabel(quality): string {
        if (quality === "medium")
            return I18n.tr("Medium");
        if (quality === "high")
            return I18n.tr("High");
        if (quality === "ultra")
            return I18n.tr("Ultra");
        return I18n.tr("Very high");
    }

    component ModeTile: Rectangle {
        id: modeTile
        required property string glyph
        required property string label
        required property var action
        property bool accent: false

        implicitHeight: 76 * root.s
        radius: Tokens.radius * root.s
        color: modeTap.pressed ? Tokens.tint16 : modeHover.hovered
            ? (modeTile.accent ? Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.18) : Tokens.tint10)
            : modeTile.accent ? Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.11) : Tokens.tint5
        border.width: Tokens.border
        border.color: modeTile.accent ? Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.48) : modeHover.hovered ? Tokens.lineStrong : Tokens.line
        scale: modeTap.pressed ? 0.97 : modeHover.hovered && !Tokens.reduceMotion ? 1.02 : 1
        Behavior on color { ColorAnimation { duration: Tokens.snap } }
        Behavior on scale { NumberAnimation { duration: Tokens.snap; easing.type: Tokens.easeSnap } }
        Column {
            anchors.centerIn: parent
            spacing: Tokens.s2 * root.s
            Text {
                anchors.horizontalCenter: parent.horizontalCenter
                text: modeTile.glyph
                color: modeTile.accent ? Tokens.sun : Tokens.inkDim
                font.family: "Material Symbols Rounded"
                font.pixelSize: 25 * root.s
            }
            Text {
                anchors.horizontalCenter: parent.horizontalCenter
                text: modeTile.label
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fMicro * root.s
                font.weight: Font.DemiBold
            }
        }
        HoverHandler { id: modeHover; cursorShape: Qt.PointingHandCursor }
        TapHandler { id: modeTap; onTapped: modeTile.action() }
    }

    component ToggleRow: Rectangle {
        id: toggleRow
        required property string glyph
        required property string label
        required property bool on
        required property var action

        implicitHeight: 48 * root.s
        radius: Tokens.radius * root.s
        color: toggleRow.on ? Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.10) : Tokens.tint5
        border.width: Tokens.border
        border.color: toggleRow.on ? Qt.rgba(Tokens.sun.r, Tokens.sun.g, Tokens.sun.b, 0.40) : Tokens.line
        Row {
            anchors.left: parent.left
            anchors.right: toggleSwitch.left
            anchors.leftMargin: root.pad
            anchors.rightMargin: Tokens.s3 * root.s
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s2 * root.s
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: toggleRow.glyph
                color: toggleRow.on ? Tokens.sun : Tokens.inkDim
                font.family: "Material Symbols Rounded"
                font.pixelSize: 20 * root.s
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                width: parent.width - 30 * root.s
                text: toggleRow.label
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall * root.s
                font.weight: Font.Medium
                elide: Text.ElideRight
            }
        }
        Sw {
            id: toggleSwitch
            anchors.right: parent.right
            anchors.rightMargin: root.pad
            anchors.verticalCenter: parent.verticalCenter
            on: toggleRow.on
            onToggled: toggleRow.action()
        }
    }

    SidebarCardShell {
        id: shell
        width: root.width
        index: root.index
        open: root.open
        reveal: root.reveal
        tabActive: root.tabActive
        title: I18n.tr("Capture")
        glyph: "photo_camera"
        eyebrow: Recorder.anyActive ? I18n.tr("RECORDING") : Capture.selecting !== "" ? I18n.tr("SELECTING") : I18n.tr("SHOT + RECORD")

        Column {
            width: parent.width
            spacing: root.gap

            Rectangle {
                visible: Recorder.anyActive || Recorder.countingDown
                width: parent.width
                implicitHeight: liveRow.implicitHeight + root.pad * 2
                radius: Tokens.radius * root.s
                color: Qt.rgba(Tokens.alert.r, Tokens.alert.g, Tokens.alert.b, 0.10)
                border.width: Tokens.border
                border.color: Qt.rgba(Tokens.alert.r, Tokens.alert.g, Tokens.alert.b, 0.48)

                Row {
                    id: liveRow
                    anchors.left: parent.left
                    anchors.right: liveControls.left
                    anchors.leftMargin: root.pad
                    anchors.rightMargin: Tokens.s3 * root.s
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s3 * root.s
                    Rectangle {
                        anchors.verticalCenter: parent.verticalCenter
                        width: 10 * root.s
                        height: width
                        radius: width / 2
                        color: Tokens.alert
                        SequentialAnimation on opacity {
                            running: Recorder.active && !Recorder.paused
                            loops: Animation.Infinite
                            NumberAnimation { to: 0.22; duration: Tokens.dur(620) }
                            NumberAnimation { to: 1; duration: Tokens.dur(620) }
                        }
                    }
                    Column {
                        anchors.verticalCenter: parent.verticalCenter
                        width: parent.width - 20 * root.s
                        spacing: Tokens.s1 * root.s
                        Text {
                            width: parent.width
                            text: Recorder.countingDown ? I18n.tr("Recording in %1").arg(Recorder.countdownSec)
                                : Recorder.paused ? I18n.tr("Recording paused") : I18n.tr("Recording")
                            color: Tokens.ink
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fRow * root.s
                            font.weight: Font.DemiBold
                        }
                        Text {
                            width: parent.width
                            text: Recorder.countingDown ? I18n.tr("Get the desktop ready") : Recorder.elapsedText
                            color: Tokens.inkMuted
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fMicro * root.s
                            font.features: ({ "tnum": 1 })
                        }
                    }
                }
                Row {
                    id: liveControls
                    anchors.right: parent.right
                    anchors.rightMargin: root.pad
                    anchors.verticalCenter: parent.verticalCenter
                    spacing: Tokens.s2 * root.s
                    Btn {
                        visible: !Recorder.countingDown
                        text: Recorder.paused ? I18n.tr("Resume") : I18n.tr("Pause")
                        compact: true
                        onAct: Recorder.togglePause()
                    }
                    Btn {
                        text: Recorder.countingDown ? I18n.tr("Cancel") : I18n.tr("Stop")
                        compact: true
                        primary: true
                        onAct: Recorder.stop()
                    }
                }
            }

            Item {
                width: parent.width
                implicitHeight: Math.max(shotLabel.implicitHeight, shotOptions.implicitHeight)
                Text {
                    id: shotLabel
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("SCREENSHOT")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro * root.s
                    font.weight: Font.DemiBold
                    font.letterSpacing: Tokens.trackMark
                }
                Row {
                    id: shotOptions
                    anchors.right: parent.right
                    spacing: Tokens.s2 * root.s
                    Seg {
                        width: 168 * root.s
                        options: ["both", "clipboard", "file"]
                        labels: ({ both: I18n.tr("Both"), clipboard: I18n.tr("Clip"), file: I18n.tr("Folder") })
                        current: Capture.save
                        onChose: value => Capture.save = value
                    }
                    Btn {
                        text: I18n.tr("Delay %1s").arg(Capture.delay)
                        compact: true
                        onAct: root.cycleDelay()
                    }
                }
            }

            Grid {
                id: shotGrid
                width: parent.width
                columns: 4
                columnSpacing: Tokens.s2 * root.s
                rowSpacing: Tokens.s2 * root.s
                readonly property real tileWidth: (width - columnSpacing * 3) / 4
                ModeTile { width: shotGrid.tileWidth; glyph: "desktop_windows"; label: I18n.tr("All"); action: () => root.shoot("all") }
                ModeTile { width: shotGrid.tileWidth; glyph: "monitor"; label: I18n.tr("Screen"); action: () => root.shoot("monitor") }
                ModeTile { width: shotGrid.tileWidth; glyph: "window"; label: I18n.tr("Window"); action: () => root.shoot("window") }
                ModeTile { width: shotGrid.tileWidth; glyph: "screenshot_region"; label: I18n.tr("Region"); action: () => root.shoot("region") }
            }

            ToggleRow {
                width: parent.width
                glyph: "auto_fix_high"
                label: I18n.tr("Beautify after")
                on: Capture.beautify
                action: () => Capture.beautify = !Capture.beautify
            }

            Rectangle { width: parent.width; height: Tokens.border; color: Tokens.lineSoft }

            Item {
                width: parent.width
                implicitHeight: Math.max(recordLabel.implicitHeight, companionRow.implicitHeight)
                Text {
                    id: recordLabel
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("RECORD")
                    color: Tokens.inkMuted
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fMicro * root.s
                    font.weight: Font.DemiBold
                    font.letterSpacing: Tokens.trackMark
                }
                Row {
                    id: companionRow
                    anchors.right: parent.right
                    spacing: Tokens.s2 * root.s
                    Btn {
                        text: I18n.tr("Key presses")
                        compact: true
                        armed: !Recorder.anyActive
                        onAct: Keypresses.toggle()
                    }
                    Btn {
                        text: I18n.tr("Webcam")
                        compact: true
                        armed: !Recorder.anyActive
                        onAct: Camera.toggle()
                    }
                }
            }

            Grid {
                id: recordGrid
                visible: !Recorder.anyActive && !Recorder.countingDown
                width: parent.width
                columns: 4
                columnSpacing: Tokens.s2 * root.s
                rowSpacing: Tokens.s2 * root.s
                readonly property real tileWidth: (width - columnSpacing * 3) / 4
                ModeTile { width: recordGrid.tileWidth; glyph: "monitor"; label: I18n.tr("Screen"); accent: true; action: () => root.record("screen") }
                ModeTile { width: recordGrid.tileWidth; glyph: "desktop_windows"; label: I18n.tr("Monitor"); accent: true; action: () => root.record("monitor") }
                ModeTile { width: recordGrid.tileWidth; glyph: "window"; label: I18n.tr("Window"); accent: true; action: () => root.record("window") }
                ModeTile { width: recordGrid.tileWidth; glyph: "screenshot_region"; label: I18n.tr("Region"); accent: true; action: () => root.record("region") }
            }

            Grid {
                visible: !Recorder.anyActive && !Recorder.countingDown
                width: parent.width
                columns: 2
                columnSpacing: root.gap
                rowSpacing: root.gap
                ToggleRow {
                    width: (parent.width - root.gap) / 2
                    glyph: Recorder.optDesktopAudio ? "volume_up" : "volume_off"
                    label: I18n.tr("Desktop audio")
                    on: Recorder.optDesktopAudio
                    action: () => Recorder.optDesktopAudio = !Recorder.optDesktopAudio
                }
                ToggleRow {
                    width: (parent.width - root.gap) / 2
                    glyph: Recorder.optMic ? "mic" : "mic_off"
                    label: I18n.tr("Microphone")
                    on: Recorder.optMic
                    action: () => Recorder.optMic = !Recorder.optMic
                }
            }

            Column {
                visible: !Recorder.anyActive && !Recorder.countingDown
                width: parent.width
                spacing: Tokens.s3 * root.s

                Text {
                    text: I18n.tr("Recording quality")
                    color: Tokens.ink
                    font.family: Tokens.ui
                    font.pixelSize: Tokens.fRow * root.s
                    font.weight: Font.DemiBold
                }
                Seg {
                    width: parent.width
                    options: root.codecSteps
                    labels: ({ h264: "H.264", hevc: "HEVC", av1: "AV1" })
                    current: Recorder.recCodec
                    onChose: value => Recorder.setSetting("codec", value)
                }
                Seg {
                    width: parent.width
                    options: root.qualitySteps
                    labels: ({ medium: I18n.tr("Medium"), high: I18n.tr("High"), very_high: I18n.tr("Very high"), ultra: I18n.tr("Ultra") })
                    current: Recorder.recQuality
                    onChose: value => Recorder.setSetting("quality", value)
                }
                Row {
                    width: parent.width
                    spacing: Tokens.s3 * root.s
                    Seg {
                        options: ["30", "60", "120"]
                        labels: ({ "30": "30 fps", "60": "60 fps", "120": "120 fps" })
                        current: String(Recorder.recFps)
                        onChose: value => Recorder.setSetting("fps", Number(value))
                    }
                    Seg {
                        options: root.containerSteps
                        labels: ({ mp4: "MP4", mkv: "MKV", webm: "WEBM" })
                        current: Recorder.recContainer
                        onChose: value => Recorder.setSetting("container", value)
                    }
                }
                ToggleRow {
                    width: parent.width
                    glyph: "mouse"
                    label: I18n.tr("Record cursor")
                    on: Recorder.recCursor
                    action: () => Recorder.setSetting("cursor", !Recorder.recCursor)
                }
                ToggleRow {
                    width: parent.width
                    glyph: "compress"
                    label: I18n.tr("Compact for Discord")
                    on: Recorder.discordMode
                    action: () => Recorder.discordMode = !Recorder.discordMode
                }
            }
        }
    }
}
