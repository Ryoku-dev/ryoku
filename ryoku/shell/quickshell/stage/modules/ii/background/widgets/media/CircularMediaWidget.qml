import Quickshell
import Quickshell.Widgets
import Quickshell.Services.Mpris
import QtQuick
import QtQuick.Effects
import QtQuick.Layouts
import Qt5Compat.GraphicalEffects
import QtQuick.Shapes as Shapes
import Ryoku.Ui.Singletons
import stage
import stage.services
import shell.services as ShellServices
import stage.modules.common
import stage.modules.common.models
import stage.modules.common.functions
import stage.modules.common.widgets
import stage.modules.ii.background.widgets
AbstractBackgroundWidget {
    id: root

    configEntryName: "circular_media"

    visibleWhenLocked: root.lockBehavior === "keep" || root.lockBehavior === "custom" || root.lockBehavior === "center" || root.lockBehavior === "lockOnly" || (Config.options.lock.centerWidget === "media")

    // Default size is 240x240 for 1:1 widgets as per AGENTS.md guidelines.
    // The size factor is folded into the implicit size rather than left to an
    // Item.scale on the root: growing the box redraws the widget, scaling the
    // root stretches a finished bitmap. Everything inside already derives from
    // root.width, so there is nothing else to convert.
    readonly property real contentScale: (Config.options.background.widgets.circular_media.widgetSize ?? 100) / 100.0
    implicitWidth: 240 * contentScale
    implicitHeight: 240 * contentScale

    readonly property bool useAlbumColors: Config.ready ? (Config.options.background.widgets.circular_media.useAlbumColors ?? true) : true
    readonly property bool showPrevButton: Config.options.background.widgets.circular_media.showPrevButton ?? true
    readonly property bool showNextButton: Config.options.background.widgets.circular_media.showNextButton ?? true
    readonly property bool showDevicePill: Config.options.background.widgets.circular_media.showDevicePill ?? true
    readonly property MprisPlayer player: MprisController.activePlayer
    readonly property bool playing: player ? player.playbackState === MprisPlaybackState.Playing : false
    // only the daemon's saved cover: an https Image can segfault qs (quickshell#1368)
    readonly property string artSource: ShellServices.Music.coverFor(root.player)
    readonly property string trackTitle: StringUtils.cleanMusicTitle(player?.trackTitle) || Translation.tr("No media")
    readonly property string trackArtist: player?.trackArtist || Translation.tr("Unknown Artist")

    ColorQuantizer {
        id: colorQuantizer
        // the quantizer reads files only; an inline data: cover keeps the theme colours
        source: root.artSource.startsWith("file://") ? root.artSource : ""
        depth: 2 // Extract 4 colors so we get a better color representation
        rescaleSize: 1
    }

    readonly property color rawExtractedColor: colorQuantizer?.colors[0] ?? WidgetColorScheme.colors.colPrimary

    // Elevate saturation and adjust lightness of the extracted color to get a highly vibrant palette
    property color artDominantColor: {
        if (!root.useDynamicColors)
            return WidgetColorScheme.colors.colPrimary;
        let h = rawExtractedColor.hslHue;
        let s = Math.max(0.45, rawExtractedColor.hslSaturation);
        let l = Math.max(0.58, Math.min(0.65, rawExtractedColor.hslLightness));
        return Qt.hsla(h, s, l, 1.0);
    }

    property QtObject blendedColors: AdaptedMaterialScheme {
        color: root.artDominantColor
    }

    readonly property bool useDynamicColors: root.useAlbumColors && colorQuantizer.source.toString() !== ""

    // Vibrant button coloring using only colPrimary, colOnPrimary, and colPrimaryContainer
    readonly property color activeAccentColor: root.useDynamicColors ? blendedColors.colPrimary : WidgetColorScheme.colors.colPrimary
    readonly property color activeAccentContainer: root.useDynamicColors ? blendedColors.colPrimaryContainer : WidgetColorScheme.colors.colPrimaryContainer
    readonly property color activeOnPrimary: root.useDynamicColors ? blendedColors.colOnPrimary : WidgetColorScheme.colors.colOnPrimary

    // Text colors are blended with white (neutral tinting) to prevent extremely vibrant/hard-to-read text, matching reference watch displays
    readonly property color activeTextColor: root.useDynamicColors ? ColorUtils.mix("#FFFFFF", root.artDominantColor, 0.90) : WidgetColorScheme.colors.colOnSurface
    readonly property color activeSubtextColor: root.useDynamicColors ? ColorUtils.mix("#FFFFFF", root.artDominantColor, 0.80) : WidgetColorScheme.colors.colOnSurfaceVariant

    // Trigger position updates for the progress bar
    Timer {
        running: root.playing && root.live
        interval: Config.options.resources.updateInterval ?? 1000
        repeat: true
        onTriggered: {
            if (root.player) {
                root.player.positionChanged();
            }
        }
    }

    readonly property real progressValue: MprisController.trackProgressOf(root.player)

    // Every effect is clipped to this widget's own texture, so it is safe on the
    // shared desktop surface.
    readonly property bool blurredEffectsSafe: true

    // Outer bezel shadow support
    StyledDropShadow {
        target: bezelRing
        visible: root.blurredEffectsSafe && (Config.options.background.widgets.circular_media.enableShadows ?? true)
    }

    // Outer Bezel Ring (Moldura) using opaque solid colBackgroundSurfaceContainer base
    Rectangle {
        id: bezelRing
        anchors.fill: parent
        radius: width / 2
        color: WidgetColorScheme.tintBackground(WidgetColorScheme.m3colors.m3shadow)

        // Inner Screen Container
        Rectangle {
            id: innerScreen
            anchors.fill: parent
            anchors.margins: parent.width * 0.08 // 8% bezel thickness
            radius: width / 2
            color: WidgetColorScheme.tintBackground(WidgetColorScheme.m3colors.m3shadow)

            // Opaque Background Artwork + Gradient Container (with circular masking)
            Item {
                id: artBackgroundContainer
                anchors.fill: parent
                z: 0

                layer.enabled: true
                layer.effect: OpacityMask {
                    maskSource: Rectangle {
                        width: artBackgroundContainer.width
                        height: artBackgroundContainer.height
                        radius: artBackgroundContainer.width / 2
                    }
                }

                // Opaque base behind album art
                Rectangle {
                    anchors.fill: parent
                    color: WidgetColorScheme.tintBackground(WidgetColorScheme.m3colors.m3shadow)
                }

                // Album Art with a light blur
                Image {
                    id: albumArtImage
                    anchors.fill: parent
                    source: root.artSource
                    fillMode: Image.PreserveAspectCrop
                    visible: root.artSource !== ""
                    asynchronous: true

                    layer.enabled: root.blurredEffectsSafe
                    layer.effect: FastBlur {
                        radius: 4 // light blur
                        transparentBorder: true
                    }
                }

                // Radial Gradient: smooth/wide fade region, starts closer to the center
                RadialGradient {
                    id: radialGrad
                    anchors.fill: parent
                    horizontalRadius: width / 2
                    verticalRadius: height / 2
                    gradient: Gradient {
                        GradientStop {
                            position: 0.0
                            color: "transparent"
                        }
                        GradientStop {
                            position: 0.25
                            color: "transparent"
                        }
                        GradientStop {
                            position: 0.62
                            color: ColorUtils.transparentize(WidgetColorScheme.m3colors.m3shadow, 0.4)
                        }
                        GradientStop {
                            position: 0.75
                            color: ColorUtils.transparentize(WidgetColorScheme.m3colors.m3shadow, 0.2)
                        }
                        GradientStop {
                            position: 1.0
                            color: WidgetColorScheme.m3colors.m3shadow
                        }
                    }
                }
            }

            // Main Content Layout (Completely outside the art layer, sitting on top of the gradient)
            ColumnLayout {
                anchors.fill: parent
                anchors.margins: parent.width * 0.02 // minimized margins
                spacing: 1 // minimized spacing
                z: 2

                // Spacer top
                Item {
                    Layout.fillHeight: true
                }

                // Row with App Icon and Song Title (Centered RowLayout)
                RowLayout {
                    Layout.alignment: Qt.AlignHCenter
                    Layout.preferredHeight: root.width * 0.08
                    spacing: 6

                    MaterialShape {
                        id: sourceIconBadge
                        Layout.preferredWidth: root.width * 0.075
                        Layout.preferredHeight: Layout.preferredWidth
                        shapeString: "Circle"
                        color: "transparent"

                        Loader {
                            id: appIconLoader
                            anchors.fill: parent
                            active: root.player && root.player.desktopEntry !== ""
                            sourceComponent: IconImage {
                                implicitSize: parent.width
                                anchors.centerIn: parent
                                source: Quickshell.iconPath(root.player ? root.player.desktopEntry : "audio-x-generic", "audio-x-generic")
                            }
                        }

                        Loader {
                            anchors.fill: parent
                            active: !appIconLoader.active
                            sourceComponent: MaterialSymbol {
                                anchors.centerIn: parent
                                text: "music_note"
                                iconSize: parent.width * 0.8
                                color: root.activeTextColor
                            }
                        }
                    }

                    StyledText {
                        text: root.trackTitle
                        color: root.activeTextColor
                        font.pixelSize: root.width * 0.07 // reduced font size
                        font.weight: Font.Bold
                        font.styleName: "Rounded"
                        Layout.maximumWidth: root.width * 0.46
                        elide: Text.ElideRight
                        maximumLineCount: 1
                    }
                }

                // Artist Name Row
                StyledText {
                    Layout.fillWidth: true
                    Layout.alignment: Qt.AlignHCenter
                    Layout.leftMargin: parent.width * 0.06
                    Layout.rightMargin: parent.width * 0.06
                    text: root.trackArtist
                    color: root.activeSubtextColor
                    font.pixelSize: root.width * 0.038 // reduced font size
                    font.weight: Font.Normal
                    horizontalAlignment: Text.AlignHCenter
                    elide: Text.ElideRight
                    maximumLineCount: 1
                }

                // Minimal Spacer between text and controls
                Item {
                    Layout.preferredHeight: 3
                }

                // Controls Area (Previous, Play/Pause + Squiggle progress, Next)
                Item {
                    Layout.alignment: Qt.AlignHCenter
                    Layout.preferredHeight: root.width * 0.28
                    Layout.preferredWidth: root.width * 0.76

                    Row {
                        anchors.centerIn: parent
                        spacing: root.width * 0.05

                        // Previous Button (Perfect Circle matching watch design guidelines)
                        RippleButton {
                            id: prevButton
                            visible: root.showPrevButton
                            width: root.width * 0.20
                            height: width // perfect circle
                            anchors.verticalCenter: parent.verticalCenter
                            buttonRadius: width / 2
                            colBackground: root.activeAccentColor
                            colBackgroundHover: ColorUtils.mix(root.activeAccentColor, root.activeAccentColor, 0.9)
                            colRipple: ColorUtils.mix(root.activeAccentColor, root.activeAccentColor, 0.8)

                            contentItem: MaterialSymbol {
                                text: "skip_previous"
                                iconSize: parent.width * 0.6
                                color: root.activeOnPrimary
                                horizontalAlignment: Text.AlignHCenter
                                verticalAlignment: Text.AlignVCenter
                            }
                            onClicked: {
                                if (root.player)
                                    root.player.previous();
                            }
                        }

                        // Play/Pause central button wrapper (includes thickness for progress border)
                        Item {
                            id: centralButtonWrapper
                            width: root.width * 0.26
                            height: width
                            anchors.verticalCenter: parent.verticalCenter

                            // Underlay progress bar in Cookie9Sided shape with thicker borders
                            MaterialShape {
                                id: progressBgOutline
                                anchors.fill: parent
                                shapeString: "Cookie9Sided"
                                color: "transparent"
                                borderColor: ColorUtils.mix("#000000", root.activeAccentColor, 0.70)
                                borderWidth: 0.055
                            }

                            MaterialShape {
                                id: progressActiveOutline
                                anchors.fill: parent
                                shapeString: "Cookie9Sided"
                                color: "transparent"
                                borderColor: root.activeAccentColor
                                borderWidth: 0.055
                                visible: false
                            }

                            ConicalGradient {
                                id: conicalMask
                                anchors.fill: progressActiveOutline
                                angle: -90
                                visible: false
                                gradient: Gradient {
                                    GradientStop {
                                        position: 0.0
                                        color: "white"
                                    }
                                    GradientStop {
                                        position: root.progressValue
                                        color: "white"
                                    }
                                    GradientStop {
                                        position: root.progressValue + 0.0001
                                        color: "transparent"
                                    }
                                    GradientStop {
                                        position: 1.0
                                        color: "transparent"
                                    }
                                }
                            }

                            OpacityMask {
                                anchors.fill: progressActiveOutline
                                source: progressActiveOutline
                                maskSource: conicalMask
                            }

                            // Play/Pause button shape inside Cookie9Sided
                            RippleButton {
                                id: playPauseButton
                                anchors.fill: parent
                                anchors.margins: parent.width * 0.08
                                buttonRadius: width / 2
                                colBackground: root.activeAccentColor
                                colBackgroundHover: ColorUtils.mix(root.activeAccentColor, root.activeAccentColor, 0.9)
                                colRipple: ColorUtils.mix(root.activeAccentContainer, root.activeAccentColor, 0.8)

                                layer.enabled: true
                                layer.effect: OpacityMask {
                                    maskSource: MaterialShape {
                                        width: playPauseButton.width
                                        height: playPauseButton.height
                                        shapeString: "Cookie9Sided"
                                    }
                                }

                                contentItem: MaterialSymbol {
                                    text: root.playing ? "pause" : "play_arrow"
                                    fill: 1
                                    iconSize: parent.width * 0.5
                                    color: root.activeOnPrimary
                                    horizontalAlignment: Text.AlignHCenter
                                    verticalAlignment: Text.AlignVCenter
                                }

                                onClicked: {
                                    if (root.player)
                                        root.player.togglePlaying();
                                }
                            }
                        }

                        // Next Button (Perfect Circle matching watch design guidelines)
                        RippleButton {
                            id: nextButton
                            visible: root.showNextButton
                            width: root.width * 0.20
                            height: width // perfect circle
                            anchors.verticalCenter: parent.verticalCenter
                            buttonRadius: width / 2
                            colBackground: root.activeAccentColor
                            colBackgroundHover: ColorUtils.mix(root.activeAccentColor, root.activeAccentColor, 0.9)
                            colRipple: ColorUtils.mix(root.activeAccentColor, root.activeAccentColor, 0.8)

                            contentItem: MaterialSymbol {
                                text: "skip_next"
                                iconSize: parent.width * 0.6
                                color: root.activeOnPrimary
                                horizontalAlignment: Text.AlignHCenter
                                verticalAlignment: Text.AlignVCenter
                            }
                            onClicked: {
                                if (root.player)
                                    root.player.next();
                            }
                        }
                    }
                }

                // Spacer
                Item {
                    Layout.fillHeight: true
                }

                // Audio Output Device Pill Shape (Centered at the Bottom)
                RowLayout {
                    visible: root.showDevicePill
                    Layout.alignment: Qt.AlignHCenter
                    Layout.preferredHeight: root.width * 0.13

                    RippleButton {
                        id: devicePill
                        implicitHeight: root.width * 0.09
                        leftPadding: root.width * 0.04
                        rightPadding: root.width * 0.04
                        colBackground: ColorUtils.transparentize(WidgetColorScheme.colors.colSurfaceContainerHigh, 0.75)
                        colBackgroundHover: ColorUtils.transparentize(WidgetColorScheme.colors.colSurfaceContainerHighestHover, 0.65)
                        colRipple: ColorUtils.transparentize(WidgetColorScheme.colors.colSurfaceContainerHighestActive, 0.55)
                        buttonRadius: Appearance.rounding.full

                        readonly property string activeAudioDeviceName: ShellServices.Audio.sink ? (ShellServices.Audio.sink.description || "") : ""
                        readonly property string audioDeviceIcon: {
                            let desc = activeAudioDeviceName.toLowerCase();
                            if (desc.includes("headphone") || desc.includes("headset") || desc.includes("wired")) {
                                return "headphones";
                            }
                            return "volume_up";
                        }

                        onClicked: Spawn.runApp(["sh", "-c", Config.options.apps.volumeMixer], { label: "volume-mixer" })

                        contentItem: Item {
                            implicitWidth: deviceRowLayout.implicitWidth
                            implicitHeight: devicePill.height

                            Row {
                                id: deviceRowLayout
                                anchors.centerIn: parent
                                spacing: 4

                                MaterialSymbol {
                                    text: devicePill.audioDeviceIcon
                                    iconSize: devicePill.height * 0.5
                                    color: root.activeTextColor
                                    anchors.verticalCenter: parent.verticalCenter
                                }

                                StyledText {
                                    text: devicePill.activeAudioDeviceName !== "" ? devicePill.activeAudioDeviceName : Translation.tr("Audio")
                                    font.pixelSize: Appearance.font.pixelSize.smallest
                                    font.weight: Font.Bold
                                    color: root.activeTextColor
                                    width: Math.min(root.width * 0.32, implicitWidth)
                                    elide: Text.ElideRight
                                    anchors.verticalCenter: parent.verticalCenter
                                }
                            }
                        }
                    }
                }

                // Spacer bottom
                Item {
                    Layout.fillHeight: true
                }
            }
        }

        // 3D Glass Dome Reflection Overlay
        Item {
            id: glassReflectionOverlay
            anchors.fill: parent
            z: 10
            enabled: false // Transparent to mouse events
            visible: root.blurredEffectsSafe && (Config.options.background.widgets.circular_media.enableGlassReflection ?? true)

            layer.enabled: root.blurredEffectsSafe
            layer.effect: OpacityMask {
                maskSource: Item {
                    width: glassReflectionOverlay.width
                    height: glassReflectionOverlay.height

                    Rectangle {
                        id: outerMaskBase
                        anchors.fill: parent
                        radius: width / 2
                        visible: false
                    }

                    FastBlur {
                        anchors.fill: parent
                        source: outerMaskBase
                        radius: 3 // soft feather on the bezel mask boundary
                        transparentBorder: true
                    }
                }
            }

            // Top-Right Crescent Reflection (14:00 / 70 degrees)
            Item {
                id: topReflectionContainer
                anchors.fill: parent
                layer.enabled: root.blurredEffectsSafe
                layer.effect: FastBlur {
                    radius: 28 // increased blur/dispersion for a softer, broader premium glass glow
                    transparentBorder: true
                }

                // Crescent Mask Shape
                Shapes.Shape {
                    id: topMaskShape
                    anchors.fill: parent
                    visible: false

                    Shapes.ShapePath {
                        strokeColor: "transparent"
                        fillColor: "white"
                        startX: parent.width * 0.40
                        startY: parent.height * 0.04
                        PathArc {
                            x: topMaskShape.width * 0.96
                            y: topMaskShape.height * 0.60
                            radiusX: topMaskShape.width * 0.48
                            radiusY: topMaskShape.height * 0.48
                            useLargeArc: false
                        }
                        PathArc {
                            x: topMaskShape.width * 0.40
                            y: topMaskShape.height * 0.04
                            radiusX: topMaskShape.width * 0.35
                            radiusY: topMaskShape.height * 0.35
                            useLargeArc: false
                            direction: PathArc.Counterclockwise
                        }
                    }
                }

                LinearGradient {
                    anchors.fill: parent
                    start: Qt.point(width * 0.40, height * 0.04)
                    end: Qt.point(width * 0.96, height * 0.60)
                    cached: true
                    gradient: Gradient {
                        GradientStop { position: 0.0; color: "transparent" }
                        GradientStop { position: 0.3; color: ColorUtils.applyAlpha("#FFFFFF", 0.42) }
                        GradientStop { position: 0.7; color: ColorUtils.applyAlpha("#FFFFFF", 0.42) }
                        GradientStop { position: 1.0; color: "transparent" }
                    }
                    layer.enabled: true
                    layer.effect: OpacityMask {
                        maskSource: topMaskShape
                    }
                }
            }

            // Bottom-Left Crescent Reflection (250 degrees / 8:00)
            Item {
                id: bottomReflectionContainer
                anchors.fill: parent
                layer.enabled: root.blurredEffectsSafe
                layer.effect: FastBlur {
                    radius: 28 // increased blur/dispersion for a softer, broader premium glass glow
                    transparentBorder: true
                }

                // Crescent Mask Shape
                Shapes.Shape {
                    id: bottomMaskShape
                    anchors.fill: parent
                    visible: false

                    Shapes.ShapePath {
                        strokeColor: "transparent"
                        fillColor: "white"
                        startX: parent.width * 0.60
                        startY: parent.height * 0.96
                        PathArc {
                            x: bottomMaskShape.width * 0.04
                            y: bottomMaskShape.height * 0.40
                            radiusX: bottomMaskShape.width * 0.48
                            radiusY: bottomMaskShape.height * 0.48
                            useLargeArc: false
                        }
                        PathArc {
                            x: bottomMaskShape.width * 0.60
                            y: bottomMaskShape.height * 0.96
                            radiusX: bottomMaskShape.width * 0.35
                            radiusY: bottomMaskShape.height * 0.35
                            useLargeArc: false
                            direction: PathArc.Counterclockwise
                        }
                    }
                }

                LinearGradient {
                    anchors.fill: parent
                    start: Qt.point(width * 0.60, height * 0.96)
                    end: Qt.point(width * 0.04, height * 0.40)
                    cached: true
                    gradient: Gradient {
                        GradientStop { position: 0.0; color: "transparent" }
                        GradientStop { position: 0.3; color: ColorUtils.applyAlpha("#FFFFFF", 0.28) }
                        GradientStop { position: 0.7; color: ColorUtils.applyAlpha("#FFFFFF", 0.28) }
                        GradientStop { position: 1.0; color: "transparent" }
                    }
                    layer.enabled: true
                    layer.effect: OpacityMask {
                        maskSource: bottomMaskShape
                    }
                }
            }
        }
    }
}
