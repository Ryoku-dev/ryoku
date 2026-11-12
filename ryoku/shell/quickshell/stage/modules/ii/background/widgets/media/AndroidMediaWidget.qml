pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Layouts
import QtQuick.Effects
import Qt5Compat.GraphicalEffects
import Quickshell
import Quickshell.Widgets
import Ryoku.Ui.Singletons
import Quickshell.Services.Mpris
import stage
import stage.services
import shell.services as ShellServices
import stage.modules.common
import stage.modules.common.widgets
import stage.modules.common.functions
import stage.modules.common.models
import stage.modules.ii.background.widgets
import stage.modules.common.media

AbstractBackgroundWidget {
    id: root

    configEntryName: "media"

    implicitWidth: contentItem.implicitWidth
    implicitHeight: contentItem.implicitHeight

    readonly property MprisPlayer player: MprisController.activePlayer
    readonly property bool showShadow: Config.options.background.widgets.media.enableShadows ?? true
    readonly property bool showLyrics: Config.options.background.widgets.media.showLyrics ?? true
    readonly property bool showControls: Config.options.background.widgets.media.showControls ?? true
    readonly property bool showDevicePill: Config.options.background.widgets.media.showDevicePill ?? true
    property list<real> visualizerPoints: []

    readonly property bool playing: player ? player.playbackState === MprisPlaybackState.Playing : false
    // only the daemon's saved cover: an https Image can segfault qs (quickshell#1368)
    readonly property string artUrl: ShellServices.Music.coverFor(root.player)
    readonly property string trackTitle: StringUtils.cleanMusicTitle(player?.trackTitle) || Translation.tr("No media")
    readonly property string trackArtist: player?.trackArtist || Translation.tr("Unknown Artist")
    readonly property string identity: player ? (player.identity ?? "") : ""
    readonly property bool hasTrack: (player?.trackTitle ?? "").length > 0

    readonly property bool hasActiveWindows: {
        const output = root.monitorName || Wm.focusedOutput || "";
        const workspaces = Wm.workspaces ?? [];
        let active = null;
        for (let i = 0; i < workspaces.length; i++) {
            if (workspaces[i].active === true && (output === "" || workspaces[i].output === output)) {
                active = workspaces[i];
                break;
            }
        }
        if (!active)
            active = Wm.focusedWorkspace;
        return active ? Wm.windowsOnWorkspace(active.id).length > 0 : false;
    }

    property string activeLyricText: ""
    property real lyricOpacity: 1.0
    property real lyricYOffset: 0.0

    property string displayArtist: root.trackArtist
    property real titleOpacity: 1.0
    property real titleYOffset: 0.0


    readonly property bool useDynamicColors: (Config.options.background.widgets.media.dynamicAlbumColors ?? false) && colorQuantizer.source.toString() !== ""

    ColorQuantizer {
        id: colorQuantizer
        // the quantizer reads files only; an inline data: cover keeps the theme colours
        source: root.artUrl.startsWith("file://") ? root.artUrl : ""
        depth: 0
        rescaleSize: 1
    }

    property color artDominantColor: ColorUtils.mix(
        (colorQuantizer?.colors[0] ?? WidgetColorScheme.colors.colPrimary),
        WidgetColorScheme.colors.colPrimaryContainer, 0.8
    ) || WidgetColorScheme.m3colors.m3secondaryContainer

    property QtObject blendedColors: AdaptedMaterialScheme {
        color: root.artDominantColor
    }

    // ── Widget color scheme (used when album art colors are off or no media) ──
    readonly property color cardBgColor: useDynamicColors ? root.blendedColors.colPrimaryContainer : WidgetColorScheme.cardBgColor
    readonly property color textColorOnBg: useDynamicColors ? root.blendedColors.colOnPrimaryContainer : WidgetColorScheme.textColorOnBg
    readonly property color subtextColorOnBg: useDynamicColors ? ColorUtils.transparentize(root.blendedColors.colOnPrimaryContainer, 0.35) : WidgetColorScheme.subtextColorOnBg
    readonly property color accentColor: useDynamicColors ? root.blendedColors.colPrimary : WidgetColorScheme.accentColor
    readonly property color pillFillColor: useDynamicColors ? root.blendedColors.colPrimaryContainer : WidgetColorScheme.pillFillColor
    readonly property color pillFillColorHover: useDynamicColors ? root.blendedColors.colPrimaryContainerHover : ColorUtils.mix(WidgetColorScheme.pillFillColor, WidgetColorScheme.accentColor, 0.15)
    readonly property color pillFillColorActive: useDynamicColors ? root.blendedColors.colPrimaryContainerActive : ColorUtils.mix(WidgetColorScheme.pillFillColor, WidgetColorScheme.accentColor, 0.25)
    readonly property color onPillFillColor: useDynamicColors ? root.blendedColors.colOnPrimaryContainer : WidgetColorScheme.textColorOnPillFill

    readonly property color artTextColor: root.textColorOnBg
    readonly property color artSubtextColor: root.subtextColorOnBg


    readonly property string displaySongText: {
        if (!root.showLyrics)
            return root.trackTitle;
        if (LyricsService.hasSyncedLines && LyricsService.statusText !== "") {
            return LyricsService.statusText;
        }
        return root.trackTitle;
    }

    onDisplaySongTextChanged: {
        lyricTransitionAnimation.stop();
        lyricTransitionAnimation.start();
    }

    SequentialAnimation {
        id: lyricTransitionAnimation
        NumberAnimation {
            target: root
            property: "lyricOpacity"
            to: 0.0
            duration: 120
            easing.type: Easing.OutQuad
        }
        PropertyAction {
            target: root
            property: "activeLyricText"
            value: root.displaySongText
        }
        NumberAnimation {
            target: root
            property: "lyricYOffset"
            from: 15
            to: 0.0
            duration: 180
            easing.type: Easing.OutCubic
        }
        NumberAnimation {
            target: root
            property: "lyricOpacity"
            to: 1.0
            duration: 180
            easing.type: Easing.OutCubic
        }
    }

    onTrackTitleChanged: {
        if (displayArtist === "") {
            displayArtist = root.trackArtist;
        }
    }

    Timer {
        running: root.playing && root.live
        interval: 1000
        repeat: true
        onTriggered: if (root.player) root.player.positionChanged()
    }

    function getBarAmplitude(index) {
        if (!root.playing || root.visualizerPoints.length === 0)
            return 0.0;
        let indices = [2, 5, 9, 14, 20, 26, 32, 38];
        let idx = indices[index] || 0;
        if (idx >= root.visualizerPoints.length) {
            idx = root.visualizerPoints.length - 1;
        }
        let val = root.visualizerPoints[idx] / 1000.0;
        return Math.min(1.0, Math.max(0.0, val * 1.8));
    }

    Component.onCompleted: {
        LyricsService.initiliazeLyrics();
        root.activeLyricText = root.displaySongText;
    }

    Item {
        id: contentItem

        implicitWidth: 492
        implicitHeight: 240

        Loader {
            active: root.showShadow
            sourceComponent: StyledRectangularShadow {
                target: mainBg
            }
        }

        Rectangle {
            id: mainBg
            anchors.fill: parent
            color: WidgetColorScheme.tintBackground(root.cardBgColor)
            radius: Appearance.rounding.windowRounding + 16
            clip: true

            // Only the artwork and its shading are rounded-clipped. A layer on mainBg
            // would also rasterize the texts below, which come out soft.
            Item {
                id: artClip
                anchors.fill: parent
                layer.enabled: true
                layer.effect: OpacityMask {
                    maskSource: Rectangle {
                        width: mainBg.width
                        height: mainBg.height
                        radius: mainBg.radius
                    }
                }

                AndroidMediaArtwork {
                    id: artwork
                    anchors.fill: parent
                    visible: root.hasTrack
                    artSource: root.artUrl
                    trackKey: JSON.stringify([root.player?.uniqueId ?? "", root.player?.trackTitle ?? "",
                        root.player?.trackArtist ?? "", root.player?.trackAlbum ?? ""])
                    hasPlayer: !!root.player
                    playing: root.playing
                    wide: 1
                }

                Item {
                    anchors.fill: parent
                    visible: root.hasTrack
                    opacity: 0.3

                    RadialGradient {
                        anchors.fill: parent
                        gradient: Gradient {
                            GradientStop { position: 0.0; color: Qt.rgba(0, 0, 0, 0) }
                            GradientStop { position: 1.0; color: Qt.rgba(0, 0, 0, 0.9) }
                        }
                        horizontalRadius: width * 0.7
                        verticalRadius: height * 0.7
                    }
                }
            }

            Connections {
                target: root.player
                function onPostTrackChanged(): void { artwork.requestArt(true); }
            }

            ColumnLayout {
                anchors.fill: parent
                anchors.leftMargin: 20
                anchors.rightMargin: 20
                anchors.topMargin: 16
                anchors.bottomMargin: 16
                spacing: 10

                RowLayout {
                    Layout.fillWidth: true
                    Layout.preferredHeight: 36

                    MaterialShape {
                        implicitWidth: 36
                        implicitHeight: 36
                        shapeString: "Cookie12Sided"
                        color: "transparent"
                        Layout.alignment: Qt.AlignLeft | Qt.AlignTop

                        Loader {
                            id: appIconLoader
                            anchors.fill: parent
                            active: root.player && root.player.desktopEntry !== ""
                            sourceComponent: IconImage {
                                implicitSize: 26
                                source: Quickshell.iconPath(root.player ? root.player.desktopEntry : "audio-x-generic", "audio-x-generic")
                            }
                        }

                        Loader {
                            anchors.fill: parent
                            active: !appIconLoader.active
                            visible: root.hasTrack
                            sourceComponent: MaterialSymbol {
                                anchors.centerIn: parent
                                text: "music_note"
                                iconSize: 18
                                color: root.textColorOnBg
                            }
                        }
                    }

                    Item {
                        Layout.fillWidth: true
                    }

                    RippleButton {
                        id: audioPill
                        visible: root.showDevicePill
                        implicitHeight: 32
                        leftPadding: 12
                        rightPadding: 12
                        Layout.alignment: Qt.AlignTop
                        colBackground: root.pillFillColor
                        colBackgroundHover: root.pillFillColorHover
                        colRipple: root.pillFillColorActive
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

                        contentItem: RowLayout {
                            id: audioPillLayout
                            spacing: 6

                            MaterialSymbol {
                                text: audioPill.audioDeviceIcon
                                iconSize: 14
                                color: root.onPillFillColor
                            }

                            StyledText {
                                text: audioPill.activeAudioDeviceName !== "" ? audioPill.activeAudioDeviceName : Translation.tr("Audio")
                                font.pixelSize: 13
                                font.bold: true
                                color: root.onPillFillColor
                                Layout.maximumWidth: 160
                                elide: Text.ElideRight
                            }
                        }
                    }
                }

                RowLayout {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    spacing: 20

                    ColumnLayout {
                        Layout.fillWidth: true
                        Layout.alignment: Qt.AlignVCenter
                        spacing: 4

                        Item {
                            id: lyricsWrapper
                            Layout.fillWidth: true
                            Layout.fillHeight: true
                            visible: root.hasTrack
                            clip: true

                            Column {
                                id: lyricsContainer
                                width: parent.width
                                spacing: 0
                                y: lyricsContainer.baseY - lyricsContainer.rowHeight - lyricsContainer.scrollOffset
                                visible: root.showLyrics && LyricsService.hasSyncedLines

                                readonly property int rowHeight: Math.floor(parent.height / 2.5)
                                readonly property real baseY: (parent.height - rowHeight) / 2
                                readonly property int targetCurrentIndex: LyricsService.hasSyncedLines ? LyricsService.currentIndex : -1
                                property int lastIndex: -1
                                property bool isMovingForward: true
                                property real scrollOffset: 0
                                readonly property real animProgress: rowHeight > 0 ? Math.abs(scrollOffset) / rowHeight : 0

                                onTargetCurrentIndexChanged: {
                                    if (targetCurrentIndex !== lastIndex && LyricsService.hasSyncedLines) {
                                        isMovingForward = targetCurrentIndex > lastIndex;
                                        lastIndex = targetCurrentIndex;
                                        scrollAnim.stop();
                                        scrollOffset = isMovingForward ? -rowHeight : rowHeight;
                                        scrollAnim.start();
                                    }
                                }

                                NumberAnimation {
                                    id: scrollAnim
                                    target: lyricsContainer
                                    property: "scrollOffset"
                                    to: 0
                                    duration: 400
                                    easing.type: Easing.OutQuart
                                }

                                Repeater {
                                    model: 3

                                    Item {
                                        required property int index
                                        property int lineOffset: index - 1
                                        property int actualIndex: lyricsContainer.targetCurrentIndex + lineOffset
                                        property bool isValidLine: LyricsService.hasSyncedLines && actualIndex >= 0 && actualIndex < LyricsService.syncedLines.length

                                        width: parent.width
                                        height: lyricsContainer.rowHeight

                                        property int oldLineOffset: lyricsContainer.isMovingForward ? lineOffset + 1 : lineOffset - 1

                                        function getOpacityForOffset(offset) {
                                            let dist = Math.abs(offset);
                                            if (dist === 0) return 1.0;
                                            if (dist === 1) return 0.35;
                                            return 0.1;
                                        }
                                        property real targetOpacity: getOpacityForOffset(lineOffset)
                                        property real startOpacity: getOpacityForOffset(oldLineOffset)
                                        opacity: startOpacity + (targetOpacity - startOpacity) * (1.0 - lyricsContainer.animProgress)

                                        function getScaleForOffset(offset) {
                                            return Math.abs(offset) === 0 ? 1.0 : 0.9;
                                        }
                                        property real targetScale: getScaleForOffset(lineOffset)
                                        property real startScale: getScaleForOffset(oldLineOffset)
                                        scale: startScale + (targetScale - startScale) * (1.0 - lyricsContainer.animProgress)

                                        transformOrigin: Item.Center

                                        StyledText {
                                            anchors.fill: parent
                                            font.family: Appearance.font.family.main
                                            font.pixelSize: 22
                                            font.weight: Math.abs(lineOffset) === 0 ? Font.Black : Font.Medium
                                            font.styleName: Math.abs(lineOffset) === 0 ? "Rounded" : "Regular"
                                            font.hintingPreference: Font.PreferNoHinting
                                            color: root.artTextColor
                                            text: isValidLine ? LyricsService.syncedLines[actualIndex].text : ""
                                            horizontalAlignment: Text.AlignLeft
                                            verticalAlignment: Text.AlignVCenter
                                            elide: Text.ElideRight
                                            wrapMode: Text.WordWrap
                                            maximumLineCount: 2
                                        }
                                    }
                                }
                            }

                            StyledText {
                                visible: !root.showLyrics || !LyricsService.hasSyncedLines
                                anchors.fill: parent
                                font.family: Appearance.font.family.main
                                font.pixelSize: 24
                                font.weight: Font.Black
                                font.styleName: "Rounded"
                                color: root.artTextColor
                                text: root.displaySongText
                                maximumLineCount: 3
                                wrapMode: Text.WordWrap
                                elide: Text.ElideRight
                                verticalAlignment: Text.AlignVCenter
                            }
                        }

                        StyledText {
                            Layout.fillWidth: true
                            visible: root.hasTrack
                            font.family: Appearance.font.family.main
                            font.pixelSize: 16
                            color: root.artSubtextColor
                            text: root.displayArtist
                            maximumLineCount: 1
                            elide: Text.ElideRight
                        }

                        RowLayout {
                            visible: !root.hasTrack
                            Layout.fillWidth: true
                            Layout.fillHeight: true
                            spacing: 16

                                MaterialShape {
                                    Layout.preferredWidth: 80
                                    Layout.preferredHeight: 80
                                    Layout.alignment: Qt.AlignLeft | Qt.AlignVCenter
                                    shapeString: "Cookie9Sided"
                                    color: root.pillFillColor

                                    MaterialSymbol {
                                        anchors.centerIn: parent
                                        text: "graphic_eq"
                                        iconSize: 42
                                        color: root.onPillFillColor
                                    }
                                }

                                ColumnLayout {
                                    Layout.fillWidth: true
                                    Layout.alignment: Qt.AlignVCenter
                                    spacing: 4

                                    StyledText {
                                        Layout.fillWidth: true
                                        font.family: Appearance.font.family.main
                                        font.pixelSize: 22
                                        font.weight: Font.Bold
                                        font.styleName: "Rounded"
                                        color: root.textColorOnBg
                                        text: Translation.tr("No media playing")
                                        elide: Text.ElideRight
                                    }

                                    StyledText {
                                        Layout.fillWidth: true
                                        font.family: Appearance.font.family.main
                                        font.pixelSize: 14
                                        font.weight: Font.Normal
                                        color: root.subtextColorOnBg
                                        text: Translation.tr("Play music or video to start")
                                        elide: Text.ElideRight
                                        opacity: 0.85
                                    }
                                }
                        }
                    }

                    RippleButton {
                        id: playBtn
                        visible: root.hasTrack && root.showControls
                        implicitWidth: 64
                        implicitHeight: 64
                        buttonRadius: 22
                        colBackground: root.pillFillColor
                        colBackgroundHover: root.pillFillColorHover
                        colRipple: root.pillFillColorActive
                        Layout.alignment: Qt.AlignRight | Qt.AlignVCenter

                        onClicked: {
                            if (root.player) {
                                if (root.playing)
                                    root.player.pause();
                                else
                                    root.player.play();
                            }
                        }

                        contentItem: Item {
                            implicitWidth: 64
                            implicitHeight: 64
                            MaterialSymbol {
                                anchors.centerIn: parent
                                text: root.playing ? "pause" : "play_arrow"
                                iconSize: 36
                                color: root.onPillFillColor
                                fill: 1
                            }
                        }
                    }
                }

                RowLayout {
                    visible: root.showControls
                    Layout.fillWidth: true
                    Layout.preferredHeight: 32
                    spacing: 16
                    Layout.alignment: Qt.AlignBottom

                    RippleButton {
                        id: prevBtn
                        implicitWidth: 32
                        implicitHeight: 32
                        buttonRadius: 16
                        colBackground: "transparent"
                        colBackgroundHover: Qt.rgba(1, 1, 1, 0.1)
                        colRipple: root.pillFillColor

                        onClicked: {
                            if (root.player?.canGoPrevious) {
                                artwork.beginChange();
                                root.player.previous();
                            }
                        }

                        contentItem: Item {
                            implicitWidth: 32
                            implicitHeight: 32
                            MaterialSymbol {
                                anchors.centerIn: parent
                                text: "skip_previous"
                                iconSize: 20
                                fill: 1
                                color: {
                                    if (!root.player || !root.player.canGoPrevious) {
                                        return root.subtextColorOnBg;
                                    }
                                    return root.textColorOnBg;
                                }
                                opacity: root.player && root.player.canGoPrevious ? 1.0 : 0.4
                            }
                        }
                    }

                    Item {
                        id: progressArea
                        Layout.fillWidth: true
                        Layout.preferredHeight: 20
                        Layout.alignment: Qt.AlignVCenter

                        Loader {
                            id: sliderLoader
                            anchors.fill: parent
                            active: root.player?.canSeek ?? false
                            sourceComponent: StyledSlider {
                                configuration: StyledSlider.Configuration.Wavy
                                animateWave: root.playing && root.visible && (!root.hasActiveWindows || GlobalStates.mediaControlsOpen)
                                highlightColor: root.accentColor
                                trackColor: Qt.rgba(1, 1, 1, 0.2)
                                handleColor: root.accentColor
                                value: MprisController.trackProgressOf(root.player)
                                // Nothing to seek to while the player publishes no length.
                                enabled: MprisController.hasTrackLength(root.player)
                                onMoved: MprisController.seekFraction(root.player, value)
                                // QQuickSlider writes `value` itself while the user drags, which destroys
                                // the binding below it. Without this the bar froze where the drag left it
                                // and never followed the track again.
                                onPressedChanged: if (!pressed)
                                    value = Qt.binding(() => MprisController.trackProgressOf(root.player))
                            }
                        }

                        Loader {
                            id: progressBarLoader
                            anchors {
                                verticalCenter: parent.verticalCenter
                                left: parent.left
                                right: parent.right
                            }
                            active: !!root.player && !sliderLoader.active
                            sourceComponent: StyledProgressBar {
                                wavy: root.player ? root.playing : false
                                animateWave: root.playing && root.visible && (!root.hasActiveWindows || GlobalStates.mediaControlsOpen)
                                highlightColor: root.accentColor
                                trackColor: Qt.rgba(1, 1, 1, 0.2)
                                value: MprisController.trackProgressOf(root.player)
                            }
                        }
                    }

                    RippleButton {
                        id: nextBtn
                        implicitWidth: 32
                        implicitHeight: 32
                        buttonRadius: 16
                        colBackground: "transparent"
                        colBackgroundHover: Qt.rgba(1, 1, 1, 0.1)
                        colRipple: root.pillFillColor

                        onClicked: {
                            if (root.player?.canGoNext) {
                                artwork.beginChange();
                                root.player.next();
                            }
                        }

                        contentItem: Item {
                            implicitWidth: 32
                            implicitHeight: 32
                            MaterialSymbol {
                                anchors.centerIn: parent
                                text: "skip_next"
                                iconSize: 20
                                fill: 1
                                color: {
                                    if (!root.player || !root.player.canGoNext) {
                                        return root.subtextColorOnBg;
                                    }
                                    return root.textColorOnBg;
                                }
                                opacity: root.player && root.player.canGoNext ? 1.0 : 0.4
                            }
                        }
                    }
                }
            }
        }
    }
}
