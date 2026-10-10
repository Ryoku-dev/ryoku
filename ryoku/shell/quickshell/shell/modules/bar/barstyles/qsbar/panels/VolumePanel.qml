import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import shell.services
import "../modules"
import Ryoku.Ui.Singletons

PanelWindow {
    id: volPanel
    required property var root

    screen: root.activePopupScreen

    color: "transparent"
    anchors { top: true; bottom: true; left: true; right: true }
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.namespace: "ryoku-volume"

    readonly property int barBottom: root.v2BarHeight
    readonly property int gap: 6

    AudioData { id: audio }
    readonly property int    volume:   audio.volume
    // The boost flag has to reach shell.json, because the media keys read it from
    // there. Config.qsbar assignments only ever moved the in-memory adapter, so
    // the toggle looked on while XF86AudioRaiseVolume still clamped at 100%.
    readonly property bool   boostOn:  root.audioBoost === true
    // PipeWire hands out one node per stream and a browser opens several, so the
    // raw list repeats the same application. Group by the name the row shows and
    // drive every node in the group together, so one row means one app and muting
    // it really silences the app rather than one of its tabs.
    readonly property var appGroups: {
        var out = []
        var index = ({})
        var list = Audio.streams || []
        for (var i = 0; i < list.length; i++) {
            var s = list[i]
            if (!s || !s.audio) continue
            var n = Audio.streamName(s) || "?"
            if (index[n] === undefined) {
                index[n] = out.length
                out.push({ name: n, nodes: [s] })
            } else {
                out[index[n]].nodes.push(s)
            }
        }
        return out
    }
    readonly property bool   muted:    audio.muted
    readonly property bool   micMuted: Audio.source && Audio.source.audio ? Audio.source.audio.muted : false
    property real   micLevel: 0
    readonly property real micNoiseFloorDb: -55
    readonly property bool micMeterAvailable: micPeakLoader.status === Loader.Ready
        && micPeakLoader.item !== null
    readonly property real micPeakValue: micMeterAvailable ? micPeakLoader.item.peak : 0

    Loader {
        id: micPeakLoader
        active: true
        source: "MicrophonePeakMonitor.qml"
        onLoaded: {
            item.panelOpen = Qt.binding(function() { return root.volVisible })
            item.muted = Qt.binding(function() { return volPanel.micMuted })
        }
    }

    function peakToMeter(peak) {
        if (!isFinite(peak) || peak <= 0) return 0
        // PwNodePeakMonitor exposes the cube root of linear amplitude.
        var amplitude = peak * peak * peak
        var db = 20 * Math.log(amplitude) / Math.LN10
        if (db <= micNoiseFloorDb) return 0
        return Math.max(0, Math.min(1, (db - micNoiseFloorDb) / -micNoiseFloorDb))
    }

    Timer {
        interval: 45 * Perf.pollFactor
        repeat: true
        running: root.volVisible && volPanel.micMeterAvailable
        onTriggered: {
            var sample = volPanel.micMuted ? 0 : volPanel.peakToMeter(volPanel.micPeakValue)
            volPanel.micLevel = sample >= volPanel.micLevel
                ? sample : Math.max(sample, volPanel.micLevel * 0.78)
        }
        onRunningChanged: if (!running) volPanel.micLevel = 0
    }

    readonly property real reveal: root.volReveal
    visible: reveal > 0.001
    WlrLayershell.keyboardFocus: root.volVisible ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None

    MouseArea {
        anchors.fill: parent
        onClicked: root.volVisible = false
    }

    Rectangle {
        id: card
        width: 280
        height: col.implicitHeight + 24
        radius: reveal > 0.001 ? root.panelRadius : 0
        color: "transparent"
        border.color: root.panelBorder
        border.width: 0
        PillShadow { theme: root }
        ConnectedPanelSurface {
            root: volPanel.root
            ownerActive: volPanel.root.volVisible
            targetX: volPanel.root.volumeBarX
            reveal: volPanel.reveal
        }

        x: Math.round(Math.max(6, Math.min(root.volumeBarX - width / 2, parent.width - width - 6)))
        y: root.barPosition === "bottom"
            ? (parent.height - barBottom - gap - height) + 2 * (1 - volPanel.reveal)
            : (barBottom + gap) - 2 * (1 - volPanel.reveal)
        opacity: volPanel.reveal
        focus: root.volVisible

        Keys.onPressed: function(event) {
            if (event.key === Qt.Key_Escape) {
                root.volVisible = false;
                event.accepted = true;
            }
        }

        MouseArea { anchors.fill: parent; onClicked: {} }

        Column {
            id: col
            anchors.fill: parent
            anchors.margins: 12
            spacing: 8

            // ── header ──
            Item {
                width: parent.width
                height: 24
                UiText {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("Volume")
                    color: root.ink
                    font.family: root.mono
                    font.pixelSize: 13
                    font.letterSpacing: 2
                    font.weight: Font.Medium
                }
                UiText {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    text: "✕"
                    color: closeMa.containsMouse ? root.seal : root.sumi
                    font.pixelSize: 12
                    Behavior on color { ColorAnimation { duration: 120 } }
                    MouseArea {
                        id: closeMa
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: root.volVisible = false
                    }
                }
            }

            Rectangle { width: parent.width; height: 1; color: root.sep }

            // ── volume bar ──
            UiText {
                text: I18n.tr("OUTPUT")
                color: root.sumiHi
                font.family: root.mono; font.pixelSize: 10; font.letterSpacing: 1
            }

            // Drag anywhere on the track to set the volume. With BOOST on the track
            // keeps its width but spans 0..150, so the 100% mark slides left and the
            // stretch past it reads red. Animating the span is what makes enabling
            // boost look like the slider growing rather than jumping.
            Item {
                id: volSlider
                width: parent.width
                height: 30

                property real maxPct: volPanel.boostOn ? 150 : 100
                Behavior on maxPct { NumberAnimation { duration: 280; easing.type: Easing.OutCubic } }

                readonly property real unitX:  volSlider.width / Math.max(1, maxPct)
                readonly property real markX:  Math.round(100 * unitX)
                readonly property real shownPct: volPanel.muted
                    ? 0 : Math.max(0, Math.min(volPanel.volume, maxPct))
                readonly property bool over: shownPct > 100.5
                readonly property color overColor: root.danger

                function applyAt(mx) {
                    var s = audio.sink
                    if (!s || !s.audio) return
                    var pct = Math.max(0, Math.min(volSlider.maxPct, mx / volSlider.width * volSlider.maxPct))
                    if (s.audio.muted && pct > 0) s.audio.muted = false
                    s.audio.volume = pct / 100
                }

                UiText {
                    anchors.horizontalCenter: parent.horizontalCenter
                    anchors.top: parent.top
                    text: volPanel.muted ? I18n.tr("Muted") : volPanel.volume + "%"
                    color: volPanel.muted
                        ? Qt.rgba(root.seal.r, root.seal.g, root.seal.b, 0.4)
                        : (volSlider.over ? volSlider.overColor : root.seal)
                    font.family: root.mono; font.pixelSize: 11; font.weight: Font.Medium
                    Behavior on color { ColorAnimation { duration: 160 } }
                }

                Rectangle {
                    id: volTrack
                    anchors.bottom: parent.bottom
                    width: parent.width; height: 8; radius: 4
                    color: root.fillActive

                    // up to 100%
                    Rectangle {
                        width: Math.min(volSlider.shownPct, 100) * volSlider.unitX
                        height: parent.height; radius: parent.radius
                        color: root.seal
                        Behavior on width { NumberAnimation { duration: 90 } }
                    }
                    // past 100%, in red, growing out of the mark
                    Rectangle {
                        x: volSlider.markX
                        width: Math.max(0, volSlider.shownPct - 100) * volSlider.unitX
                        height: parent.height; radius: parent.radius
                        color: volSlider.overColor
                        visible: width > 0.5
                        Behavior on width { NumberAnimation { duration: 90 } }
                    }
                    // the 100% line, only meaningful once there is room past it
                    Rectangle {
                        x: volSlider.markX - 1
                        anchors.verticalCenter: parent.verticalCenter
                        width: 2; height: parent.height + 6; radius: 1
                        color: Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.55)
                        visible: volSlider.maxPct > 100.5
                        opacity: Math.max(0, Math.min(1, (volSlider.maxPct - 100) / 50))
                    }
                }

                MouseArea {
                    anchors.fill: parent
                    acceptedButtons: Qt.LeftButton
                    cursorShape: Qt.PointingHandCursor
                    onPressed: (m) => volSlider.applyAt(m.x)
                    onPositionChanged: (m) => { if (pressed) volSlider.applyAt(m.x) }
                    onWheel: (w) => {
                        var s = audio.sink
                        if (!s || !s.audio) return
                        var step = w.angleDelta.y > 0 ? 0.05 : -0.05
                        s.audio.volume = Math.max(0, Math.min(volSlider.maxPct / 100, s.audio.volume + step))
                    }
                }
            }

            // ── boost toggle: lift the safe 100% cap to 150% for quiet hardware ──
            Rectangle {
                id: boostTile
                width: parent.width
                height: 26; radius: root.panelButtonRadius
                readonly property bool on: volPanel.boostOn
                color: on ? root.fillActive : (boostMa.containsMouse ? root.fillHover : root.fillIdle)
                border.color: (on || boostMa.containsMouse) ? root.seal : root.sep
                border.width: 1
                Behavior on color { ColorAnimation { duration: 120 } }
                UiText {
                    anchors.left: parent.left; anchors.leftMargin: 8
                    anchors.verticalCenter: parent.verticalCenter
                    text: I18n.tr("BOOST ABOVE 100%")
                    color: root.seal
                    font.family: root.mono; font.pixelSize: 10; font.letterSpacing: 1
                }
                UiText {
                    anchors.right: parent.right; anchors.rightMargin: 8
                    anchors.verticalCenter: parent.verticalCenter
                    text: boostTile.on ? I18n.tr("ON") : I18n.tr("OFF")
                    color: boostTile.on ? root.seal : Qt.rgba(root.seal.r, root.seal.g, root.seal.b, 0.4)
                    font.family: root.mono; font.pixelSize: 10; font.weight: Font.Medium
                }
                MouseArea {
                    id: boostMa
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: {
                        root.audioBoost = !volPanel.boostOn
                        // dropping the cap while boosted pulls the sink back to 0 dB
                        if (!root.audioBoost && audio.sink && audio.sink.audio
                                && audio.sink.audio.volume > 1)
                            audio.sink.audio.volume = 1
                    }
                }
            }

            // ── output device switcher ──
            UiText {
                text: I18n.tr("OUTPUT DEVICE")
                color: root.sumiHi
                font.family: root.mono; font.pixelSize: 10; font.letterSpacing: 1
            }
            Column {
                width: parent.width
                spacing: 4
                Repeater {
                    model: Audio.outputs.filter(Audio.alive)
                    delegate: Rectangle {
                        id: devTile
                        required property var modelData
                        readonly property bool isDef:   Audio.sink && devTile.modelData.name === Audio.sink.name
                        readonly property bool hovered: devMa.containsMouse
                        width: parent.width
                        height: 26; radius: root.panelButtonRadius
                        color: isDef     ? root.fillActive
                             : hovered ? root.fillHover : root.fillIdle
                        border.color: (isDef || hovered) ? root.seal : root.sep
                        border.width: 1
                        Behavior on color { ColorAnimation { duration: 120 } }
                        Row {
                            anchors.fill: parent
                            anchors.leftMargin: 8; anchors.rightMargin: 8
                            spacing: 6
                            UiText {
                                anchors.verticalCenter: parent.verticalCenter
                                text: devTile.isDef ? "●" : "○"
                                color: devTile.isDef ? root.seal : root.sumi
                                font.family: root.mono; font.pixelSize: 10
                            }
                            UiText {
                                anchors.verticalCenter: parent.verticalCenter
                                width: parent.width - 22
                                text: Audio.nodeLabel(devTile.modelData)
                                color: (devTile.isDef || devTile.hovered) ? root.seal : root.ink
                                font.family: root.mono; font.pixelSize: 11
                                elide: Text.ElideRight
                            }
                        }
                        MouseArea {
                            id: devMa
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: {
                                Audio.setOutput(devTile.modelData)
                            }
                        }
                    }
                }
            }

            Rectangle { width: parent.width; height: 1; color: root.sep }

            // ── mute toggle ──
            Rectangle {
                width: parent.width
                height: 28; radius: root.panelButtonRadius
                color: volPanel.muted ? root.fillActive
                    : muteMa.containsMouse ? root.fillHover
                    : root.fillIdle
                border.color: (muteMa.containsMouse || volPanel.muted) ? root.seal : root.sep
                border.width: 1
                Behavior on color { ColorAnimation { duration: 120 } }
                UiText {
                    anchors.centerIn: parent
                    text: volPanel.muted ? I18n.tr("Unmute volume") : I18n.tr("Mute volume")
                    color: (muteMa.containsMouse || volPanel.muted) ? root.seal : root.sumi
                    font.family: root.mono; font.pixelSize: 11
                }
                MouseArea {
                    id: muteMa
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: {
                        if (Audio.sink && Audio.sink.audio) Audio.sink.audio.muted = !Audio.sink.audio.muted
                    }
                }
            }

            // ── per-app mixer ──
            Rectangle { width: parent.width; height: 1; color: root.sep; visible: Audio.streams.length > 0 }
            UiText {
                visible: Audio.streams.length > 0
                text: I18n.tr("APPS")
                color: root.sumiHi
                font.family: root.mono; font.pixelSize: 10; font.letterSpacing: 1
            }
            Column {
                width: parent.width
                spacing: 8
                Repeater {
                    model: volPanel.appGroups
                    delegate: Item {
                        id: appRow
                        required property var modelData
                        width: parent.width
                        height: 32

                        readonly property var nodes: modelData.nodes || []
                        // one row per app, so it reads muted only when the whole app is
                        readonly property bool appMuted: {
                            if (appRow.nodes.length === 0) return false
                            for (var i = 0; i < appRow.nodes.length; i++)
                                if (!appRow.nodes[i].audio || !appRow.nodes[i].audio.muted) return false
                            return true
                        }
                        readonly property int nodeVol: {
                            var top = 0
                            for (var i = 0; i < appRow.nodes.length; i++) {
                                var a = appRow.nodes[i].audio
                                if (a && a.volume > top) top = a.volume
                            }
                            return Math.round(top * 100)
                        }
                        property int liveVol: appRow.nodeVol
                        onNodeVolChanged: liveVol = appRow.nodeVol

                        readonly property real maxPct: volSlider.maxPct
                        readonly property bool over: liveVol > 100.5

                        function applyVol(pct) {
                            var v = Math.max(0, Math.min(appRow.maxPct, pct)) / 100
                            for (var i = 0; i < appRow.nodes.length; i++)
                                if (appRow.nodes[i].audio) appRow.nodes[i].audio.volume = v
                        }
                        function toggleMuted() {
                            var next = !appRow.appMuted
                            for (var i = 0; i < appRow.nodes.length; i++)
                                if (appRow.nodes[i].audio) appRow.nodes[i].audio.muted = next
                        }

                        // mute glyph
                        IconText {
                            id: appMute
                            anchors.left: parent.left
                            anchors.top: parent.top
                            text: appRow.appMuted ? String.fromCodePoint(0xE04F) : String.fromCodePoint(0xE050)
                            font.pixelSize: 15
                            color: appRow.appMuted ? Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.4) : root.seal
                            MouseArea {
                                anchors.fill: parent; anchors.margins: -3
                                cursorShape: Qt.PointingHandCursor
                                onClicked: appRow.toggleMuted()
                            }
                        }
                        UiText {
                            anchors.left: appMute.right; anchors.leftMargin: 6
                            anchors.verticalCenter: appMute.verticalCenter
                            anchors.verticalCenterOffset: 1
                            anchors.right: appPct.left; anchors.rightMargin: 6
                            text: appRow.nodes.length > 1
                                ? appRow.modelData.name + "  ×" + appRow.nodes.length
                                : appRow.modelData.name
                            color: appRow.appMuted ? root.sumi : root.ink
                            font.family: root.mono; font.pixelSize: 11
                            elide: Text.ElideRight
                        }
                        UiText {
                            id: appPct
                            anchors.right: parent.right
                            anchors.verticalCenter: appMute.verticalCenter
                            anchors.verticalCenterOffset: 1
                            text: appRow.liveVol + "%"
                            color: appRow.over ? root.danger : root.seal
                            font.family: root.mono; font.pixelSize: 11; font.weight: Font.Medium
                            Behavior on color { ColorAnimation { duration: 160 } }
                        }

                        // draggable, on the same scale as the output slider so a
                        // boosted app reads red past the 100% mark too
                        Rectangle {
                            id: appTrack
                            anchors.left: parent.left; anchors.right: parent.right
                            anchors.bottom: parent.bottom
                            height: 8; radius: 4
                            color: root.fillActive

                            readonly property real unitX: appTrack.width / Math.max(1, appRow.maxPct)
                            readonly property real markX: Math.round(100 * appTrack.unitX)

                            Rectangle {
                                width: Math.min(appRow.liveVol, 100) * appTrack.unitX
                                height: parent.height; radius: parent.radius
                                color: appRow.appMuted ? Qt.rgba(root.seal.r, root.seal.g, root.seal.b, 0.4) : root.seal
                            }
                            Rectangle {
                                x: appTrack.markX
                                width: Math.max(0, appRow.liveVol - 100) * appTrack.unitX
                                height: parent.height; radius: parent.radius
                                color: appRow.appMuted ? Qt.rgba(root.danger.r, root.danger.g, root.danger.b, 0.4) : root.danger
                                visible: width > 0.5
                            }
                            Rectangle {
                                x: appTrack.markX - 1
                                anchors.verticalCenter: parent.verticalCenter
                                width: 2; height: parent.height + 4; radius: 1
                                color: Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.55)
                                visible: appRow.maxPct > 100.5
                            }

                            MouseArea {
                                anchors.fill: parent; anchors.topMargin: -8; anchors.bottomMargin: -4
                                cursorShape: Qt.PointingHandCursor
                                function setFromX(x) {
                                    appRow.liveVol = Math.max(0, Math.min(appRow.maxPct,
                                        Math.round(x / appTrack.width * appRow.maxPct)))
                                }
                                onPressed:          function(m) { setFromX(m.x) }
                                onPositionChanged:  function(m) { if (pressed) setFromX(m.x) }
                                onReleased:         function(m) { appRow.applyVol(appRow.liveVol) }
                            }
                        }
                    }
                }
            }

            Rectangle { width: parent.width; height: 1; color: root.sep }

            // ── mic section ──
            UiText {
                text: I18n.tr("INPUT")
                color: root.sumiHi
                font.family: root.mono; font.pixelSize: 10; font.letterSpacing: 1
            }

            Column {
                width: parent.width
                spacing: 4
                Repeater {
                    model: Audio.inputs.filter(Audio.alive)
                    delegate: Rectangle {
                        id: inTile
                        required property var modelData
                        readonly property bool isDef:   Audio.source && inTile.modelData.name === Audio.source.name
                        readonly property bool hovered: inMa.containsMouse
                        width: parent.width
                        height: 26; radius: root.panelButtonRadius
                        color: isDef     ? root.fillActive
                             : hovered ? root.fillHover : root.fillIdle
                        border.color: (isDef || hovered) ? root.seal : root.sep
                        border.width: 1
                        Behavior on color { ColorAnimation { duration: 120 } }
                        Row {
                            anchors.fill: parent
                            anchors.leftMargin: 8; anchors.rightMargin: 8
                            spacing: 6
                            UiText {
                                anchors.verticalCenter: parent.verticalCenter
                                text: inTile.isDef ? "●" : "○"
                                color: inTile.isDef ? root.seal : root.sumi
                                font.family: root.mono; font.pixelSize: 10
                            }
                            UiText {
                                anchors.verticalCenter: parent.verticalCenter
                                width: parent.width - 22
                                text: Audio.nodeLabel(inTile.modelData)
                                color: (inTile.isDef || inTile.hovered) ? root.seal : root.ink
                                font.family: root.mono; font.pixelSize: 11
                                elide: Text.ElideRight
                            }
                        }
                        MouseArea {
                            id: inMa
                            anchors.fill: parent
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: Audio.setInput(inTile.modelData)
                        }
                    }
                }
            }

            Row {
                width: parent.width
                UiText {
                    text: I18n.tr("Microphone")
                    color: root.sumiHi
                    font.family: root.mono; font.pixelSize: 11
                    width: parent.width * 0.5
                }
                UiText {
                    text: volPanel.micMuted ? I18n.tr("Muted") : I18n.tr("Active")
                    color: volPanel.micMuted
                        ? Qt.rgba(root.seal.r, root.seal.g, root.seal.b, 0.5)
                        : Qt.rgba(root.ink.r, root.ink.g, root.ink.b, 0.7)
                    font.family: root.mono; font.pixelSize: 11
                    width: parent.width * 0.5
                    horizontalAlignment: Text.AlignRight
                }
            }

            Item {
                width: parent.width
                height: visible ? 8 : 0
                visible: volPanel.micMeterAvailable

                Rectangle {
                    anchors.left: parent.left
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    height: 4
                    radius: 2
                    color: root.fillIdle
                    border.color: root.sep
                    border.width: 1

                    Rectangle {
                        anchors.left: parent.left
                        anchors.top: parent.top
                        anchors.bottom: parent.bottom
                        width: parent.width * volPanel.micLevel
                        radius: parent.radius
                        color: volPanel.micMuted
                            ? Qt.rgba(root.seal.r, root.seal.g, root.seal.b, 0.25)
                            : root.seal
                    }
                }
            }

            Rectangle {
                width: parent.width
                height: 28; radius: root.panelButtonRadius
                color: volPanel.micMuted ? root.fillActive
                    : micMuteMa.containsMouse ? root.fillHover
                    : root.fillIdle
                border.color: (micMuteMa.containsMouse || volPanel.micMuted) ? root.seal : root.sep
                border.width: 1
                Behavior on color { ColorAnimation { duration: 120 } }
                UiText {
                    anchors.centerIn: parent
                    text: volPanel.micMuted ? I18n.tr("Unmute mic") : I18n.tr("Mute mic")
                    color: (micMuteMa.containsMouse || volPanel.micMuted) ? root.seal : root.sumi
                    font.family: root.mono; font.pixelSize: 11
                }
                MouseArea {
                    id: micMuteMa
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: {
                        if (Audio.source && Audio.source.audio) Audio.source.audio.muted = !Audio.source.audio.muted
                    }
                }
            }

            Rectangle { width: parent.width; height: 1; color: root.sep }

            // ── open audio ──
            Rectangle {
                width: parent.width
                height: 28; radius: root.panelButtonRadius
                color: audioBtnMa.containsMouse ? root.fillPrimaryHover : root.seal
                Behavior on color { ColorAnimation { duration: 120 } }
                UiText {
                    anchors.centerIn: parent
                    text: I18n.tr("Open audio")
                    color: root.paper
                    font.family: root.mono; font.pixelSize: 11
                }
                MouseArea {
                    id: audioBtnMa
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: {
                        root.volVisible = false
                        audioRunner.running = false
                        audioRunner.running = true
                    }
                }
            }
        }
    }

    Process { id: audioRunner; command: ["bash", "-c", "command -v pavucontrol >/dev/null 2>&1 && exec pavucontrol || notify-send -a Ryoku 'Audio settings' 'pavucontrol is not installed'"] }
}
