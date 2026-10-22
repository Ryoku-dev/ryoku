pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Services.Pipewire
import Caelestia.Config
import Caelestia.Services
import caelestia.components
import caelestia.components.controls
import caelestia.services
import caelestia.utils

ColumnLayout {
    id: root

    required property PopoutState popouts

    property real scaleOffset: 1.0
    property real fontScale: 1.0
    property bool _isSidebarOpen: false
    property bool _outputsOpen: false
    property int _streamCount: Audio.appStreams.length

    readonly property bool hasOutputChoice: Audio.sinks.length > 1
    readonly property bool hasSources: Audio.sources.length > 0
    readonly property bool hasStreams: Audio.appStreams.length > 0
    readonly property bool isPlaying: (Players.active?.isPlaying ?? false) || Players.list.some(p => p.isPlaying)

    function outputIcon(node: PwNode): string {
        if (!node)
            return "speaker";
        const name = (node.description || node.name || "").toLowerCase();
        if (name.includes("headset") || name.includes("headphone") || name.includes("earbud") || name.includes("earphone"))
            return "headphones";
        return "speaker";
    }

    function sourceIcon(node: PwNode): string {
        if (!node)
            return "mic";
        const name = (node.description || node.name || "").toLowerCase();
        if (name.includes("headset"))
            return "headset_mic";
        return "mic";
    }

    function syncStreamsSection(): void {
        if (hasStreams && _streamCount === 0)
            appsSection.expanded = true;
        else if (!hasStreams)
            appsSection.expanded = false;
        _streamCount = Audio.appStreams.length;
    }

    // With the sidebar open, match its width so the popout joins it (like the
    // narrower popouts do) instead of sticking out past it.
    width: _isSidebarOpen ? Math.max(300 * scaleOffset, (Visibilities.sidebarWidthFor(Tokens.sizes.sidebar.width) * scaleOffset) - Tokens.padding.extraLargeIncreased) : 440 * scaleOffset
    implicitWidth: width
    spacing: Tokens.spacing.small * scaleOffset

    Connections {
        function onStreamsChanged(): void {
            root.syncStreamsSection();
        }

        target: Audio
    }

    ServiceRef {
        service: Audio.cava
    }

    RowLayout {
        Layout.topMargin: Tokens.padding.small * root.scaleOffset
        Layout.leftMargin: Tokens.padding.small * root.scaleOffset
        Layout.rightMargin: Tokens.padding.small * root.scaleOffset
        Layout.fillWidth: true
        spacing: Tokens.spacing.small * root.scaleOffset

        StyledText {
            Layout.fillWidth: true
            text: qsTr("Audio")
            font.weight: 500
            font.pointSize: Tokens.font.title.small.pointSize * root.fontScale
        }

        IconButton {
            icon: "settings"
            font: Tokens.font.icon.medium
            type: IconButton.Tonal
            isRound: true
            inactiveColour: Colours.tPalette.m3surfaceContainerHigh
            inactiveOnColour: Colours.palette.m3onSurfaceVariant
            onClicked: root.popouts.detachRequested("audio")
        }
    }

    StyledRect {
        id: hero

        Layout.fillWidth: true
        implicitHeight: heroLayout.implicitHeight + Tokens.padding.medium * 2 * root.scaleOffset
        radius: Tokens.rounding.large * root.scaleOffset
        color: Colours.tPalette.m3surfaceContainer
        clip: true

        ColumnLayout {
            id: heroLayout

            width: parent.width - Tokens.padding.medium * 2 * root.scaleOffset
            x: Tokens.padding.medium * root.scaleOffset
            y: Tokens.padding.medium * root.scaleOffset
            spacing: Tokens.spacing.medium * root.scaleOffset

            Item {
                id: outputSummary

                Layout.fillWidth: true
                Layout.preferredHeight: 52 * root.scaleOffset

                RowLayout {
                    id: summaryRow

                    anchors.fill: parent
                    anchors.leftMargin: Tokens.padding.extraSmall * root.scaleOffset
                    anchors.rightMargin: Tokens.padding.extraSmall * root.scaleOffset
                    spacing: Tokens.spacing.medium * root.scaleOffset

                    StyledRect {
                        implicitWidth: 40 * root.scaleOffset
                        implicitHeight: 40 * root.scaleOffset
                        radius: Tokens.rounding.full * root.scaleOffset
                        color: Colours.palette.m3primaryContainer

                        MaterialIcon {
                            anchors.centerIn: parent
                            text: root.outputIcon(Audio.sink)
                            color: Colours.palette.m3onPrimaryContainer
                            fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
                        }
                    }

                    ColumnLayout {
                        Layout.fillWidth: true
                        spacing: 0

                        StyledText {
                            text: qsTr("Output device")
                            color: Colours.palette.m3onSurfaceVariant
                            font.pointSize: Tokens.font.label.small.pointSize * root.fontScale
                        }

                        StyledText {
                            Layout.fillWidth: true
                            text: Audio.sink ? (Audio.sink.properties?.["node.nick"] || Audio.sink.description || Audio.sink.name) : qsTr("No output device")
                            elide: Text.ElideRight
                            font.pointSize: Tokens.font.title.small.pointSize * root.fontScale
                            font.weight: Font.Medium
                        }
                    }

                    MaterialIcon {
                        visible: root.hasOutputChoice
                        text: "expand_more"
                        rotation: root._outputsOpen ? 180 : 0
                        color: Colours.palette.m3onSurfaceVariant
                        fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale

                        Behavior on rotation {
                            Anim {
                                type: Anim.StandardSmall
                            }
                        }
                    }
                }

                StateLayer {
                    anchors.fill: parent
                    radius: Tokens.rounding.medium * root.scaleOffset
                    disabled: !root.hasOutputChoice
                    onClicked: root._outputsOpen = !root._outputsOpen
                }
            }

            Item {
                id: outputList

                Layout.fillWidth: true
                Layout.preferredHeight: root._outputsOpen ? (outputColumn.implicitHeight + Tokens.spacing.extraSmall * root.scaleOffset) : 0
                implicitHeight: Layout.preferredHeight
                clip: true

                Behavior on Layout.preferredHeight {
                    Anim {}
                }

                ColumnLayout {
                    id: outputColumn

                    width: parent.width
                    y: Tokens.spacing.extraSmall * root.scaleOffset
                    spacing: Tokens.spacing.extraSmall * root.scaleOffset
                    opacity: root._outputsOpen ? 1.0 : 0.0

                    Behavior on opacity {
                        Anim {
                            type: Anim.DefaultEffects
                        }
                    }

                    Repeater {
                        model: Audio.sinks

                        AudioDevice {
                            active: Audio.sink?.id === modelData.id
                            onSelected: {
                                Audio.setAudioSink(modelData);
                                root._outputsOpen = false;
                            }
                        }
                    }
                }
            }

            StyledRect {
                id: cavaStrip

                Layout.fillWidth: true
                Layout.preferredHeight: 28 * root.scaleOffset
                visible: !!Audio.cava && root.isPlaying
                radius: Tokens.rounding.small * root.scaleOffset
                color: Colours.tPalette.m3surfaceContainerHigh

                Row {
                    id: barsRow

                    anchors.fill: parent
                    anchors.leftMargin: Tokens.padding.small * root.scaleOffset
                    anchors.rightMargin: Tokens.padding.small * root.scaleOffset
                    anchors.topMargin: Tokens.padding.extraSmall * root.scaleOffset
                    anchors.bottomMargin: Tokens.padding.extraSmall * root.scaleOffset
                    spacing: Math.max(2, Tokens.spacing.extraSmall / 2 * root.scaleOffset)

                    Repeater {
                        model: 24

                        Item {
                            required property int index

                            width: (barsRow.width - 23 * barsRow.spacing) / 24
                            height: barsRow.height

                            StyledRect {
                                anchors.bottom: parent.bottom
                                width: parent.width
                                radius: Tokens.rounding.full * root.scaleOffset
                                color: Colours.palette.m3primary

                                height: Math.max(4 * root.scaleOffset, parent.height * Math.min(1, Math.max(0, Audio.cava?.values?.[Math.floor(index * GlobalConfig.services.visualiserBars / 24)] ?? 0)))

                                Behavior on height {
                                    Anim {
                                        type: Anim.FastSpatial
                                    }
                                }
                            }
                        }
                    }
                }
            }

            RowLayout {
                Layout.fillWidth: true
                spacing: Tokens.spacing.medium * root.scaleOffset

                IconButton {
                    id: muteButton

                    icon: Icons.getVolumeIcon(Audio.volume, Audio.muted)
                    type: IconButton.Tonal
                    isRound: true
                    onClicked: {
                        if (Audio.sink?.ready && Audio.sink?.audio)
                            Audio.sink.audio.muted = !Audio.sink.audio.muted;
                    }
                }

                CustomMouseArea {
                    function onWheel(event: WheelEvent): void {
                        if (event.angleDelta.y > 0)
                            Audio.incrementVolume();
                        else if (event.angleDelta.y < 0)
                            Audio.decrementVolume();
                    }

                    Layout.fillWidth: true
                    implicitHeight: 18 * root.scaleOffset

                    StyledSlider {
                        anchors.left: parent.left
                        anchors.right: parent.right
                        anchors.verticalCenter: parent.verticalCenter
                        implicitHeight: 18 * root.scaleOffset
                        radius: Tokens.rounding.full * root.scaleOffset
                        value: Audio.volume
                        fgColour: Colours.palette.m3primary
                        bgColour: Colours.tPalette.m3surfaceContainerHigh
                        onInteraction: v => Audio.setVolume(v)
                        onReleased: v => Audio.playEffectTick()
                    }
                }

                StyledText {
                    Layout.preferredWidth: 72 * root.scaleOffset
                    text: Audio.muted ? qsTr("Muted") : Math.round(Audio.volume * 100) + "%"
                    color: Audio.muted ? Colours.palette.m3onSurfaceVariant : Colours.palette.m3onSurface
                    font.pointSize: Tokens.font.title.medium.pointSize * root.fontScale
                    font.weight: Font.Medium
                    horizontalAlignment: Text.AlignRight
                }
            }
        }
    }

    PopoutSection {
        Layout.fillWidth: true
        scaleOffset: root.scaleOffset
        fontScale: root.fontScale
        minHeaderHeight: 40
        headerRightMargin: Tokens.padding.small * root.scaleOffset
        title: qsTr("Input")

        StyledText {
            Layout.fillWidth: true
            visible: !root.hasSources
            text: qsTr("No input device")
            color: Colours.palette.m3onSurfaceVariant
            font.pointSize: Tokens.font.body.small.pointSize * root.fontScale
        }

        Repeater {
            model: Audio.sources

            AudioDevice {
                isInput: true
                active: Audio.source?.id === modelData.id
                onSelected: Audio.setAudioSource(modelData)
            }
        }

        SliderRow {
            Layout.fillWidth: true
            first: true
            last: true
            iconClickable: true
            icon: Icons.getMicVolumeIcon(Audio.sourceVolume, Audio.sourceMuted)
            label: qsTr("Input volume")
            valueLabel: Audio.sourceMuted ? qsTr("Muted") : Math.round(Audio.sourceVolume * 100) + "%"
            value: Audio.sourceVolume
            onIconClicked: {
                if (Audio.source?.ready && Audio.source?.audio)
                    Audio.source.audio.muted = !Audio.source.audio.muted;
            }
            onMoved: v => Audio.setSourceVolume(v)
        }
    }

    PopoutSection {
        id: appsSection

        Layout.fillWidth: true
        scaleOffset: root.scaleOffset
        fontScale: root.fontScale
        minHeaderHeight: 40
        headerRightMargin: Tokens.padding.small * root.scaleOffset
        title: qsTr("Now playing")
        expanded: root.hasStreams

        StyledText {
            Layout.fillWidth: true
            visible: !root.hasStreams
            text: qsTr("No apps playing audio")
            color: Colours.palette.m3onSurfaceVariant
            font.pointSize: Tokens.font.body.small.pointSize * root.fontScale
        }

        // every app in one card, like the output device above
        StyledRect {
            Layout.fillWidth: true
            visible: root.hasStreams
            implicitHeight: streamList.implicitHeight + Tokens.padding.small * 2 * root.scaleOffset
            radius: Tokens.rounding.large * root.scaleOffset
            color: Colours.tPalette.m3surfaceContainer

            ColumnLayout {
                id: streamList

                width: parent.width - Tokens.padding.medium * 2 * root.scaleOffset
                x: Tokens.padding.medium * root.scaleOffset
                y: Tokens.padding.small * root.scaleOffset
                spacing: 0

                Repeater {
                    model: ScriptModel {
                        values: [...Audio.appStreams]
                    }

                    ColumnLayout {
                        id: entry

                        required property PwNode modelData
                        required property int index

                        Layout.fillWidth: true
                        spacing: 0

                        StyledRect {
                            visible: entry.index > 0
                            Layout.fillWidth: true
                            implicitHeight: 1
                            color: Colours.palette.m3outlineVariant
                        }

                        AppStreamRow {
                            node: entry.modelData
                            muteOnIconClick: true
                        }
                    }
                }
            }
        }
    }

    component AudioDevice: StyledRect {
        id: device

        required property PwNode modelData
        property bool isInput: false
        property bool active: false

        signal selected

        Layout.fillWidth: true
        Layout.preferredWidth: 0
        implicitHeight: row.implicitHeight + Tokens.padding.small * 2 * root.scaleOffset
        radius: Tokens.rounding.small * root.scaleOffset
        color: device.active ? Colours.tPalette.m3surfaceContainerHigh : "transparent"

        RowLayout {
            id: row

            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            anchors.leftMargin: Tokens.padding.medium * root.scaleOffset
            anchors.rightMargin: Tokens.padding.medium * root.scaleOffset
            spacing: Tokens.spacing.small * root.scaleOffset

            MaterialIcon {
                text: device.isInput ? root.sourceIcon(device.modelData) : root.outputIcon(device.modelData)
                color: device.active ? Colours.palette.m3primary : Colours.palette.m3onSurfaceVariant
                fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
            }

            StyledText {
                Layout.fillWidth: true
                Layout.preferredWidth: 0
                elide: Text.ElideRight
                text: device.modelData?.properties?.["node.nick"] || device.modelData?.description || device.modelData?.name || qsTr("Unknown")
                color: device.active ? Colours.palette.m3onSurface : Colours.palette.m3onSurfaceVariant
                font.pointSize: Tokens.font.body.small.pointSize * root.fontScale
            }

            MaterialIcon {
                text: device.active ? "check_circle" : "radio_button_unchecked"
                color: device.active ? Colours.palette.m3primary : Colours.palette.m3onSurfaceVariant
                fontStyle.pointSize: Tokens.font.icon.medium.pointSize * root.fontScale
            }
        }

        StateLayer {
            anchors.fill: parent
            radius: device.radius
            onClicked: device.selected()
        }
    }
}

