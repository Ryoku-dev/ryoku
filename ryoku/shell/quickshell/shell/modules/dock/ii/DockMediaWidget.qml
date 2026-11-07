pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Layouts
import Qt5Compat.GraphicalEffects
import Quickshell
import Quickshell.Widgets
import Quickshell.Io
import Quickshell.Services.Mpris
import stage.modules.common.functions
import stage.modules.common.widgets
import stage.services
import stage.modules.common.models
import shell.services as Ryoku
import "./widgets"
import "."

Item {
    id: root

    property bool isVertical: false
    property var dockContent: null
    property int delegateIndex: -1

    readonly property real buttonSize: Appearance.sizes.dockButtonSize
    readonly property real dotMargin: root.dockContent?.dotMargin ?? Math.max(1, Math.round((Config.options?.dock.height ?? 60) * 0.2) - 2)
    readonly property real dotMarginV: root.dockContent?.dotMarginV ?? root.dotMargin
    readonly property real slotSize: root.dockContent?.buttonSlotSize ?? (buttonSize + dotMargin * 2)
    readonly property real slotHeight: root.dockContent
        ? (root.isVertical ? root.dockContent.buttonSlotSize : root.dockContent.buttonSlotHeight)
        : (buttonSize + dotMarginV * 2)
    readonly property real fixedSlots: isVertical ? 2.5 : 3
    readonly property real fixedLength: fixedSlots * slotSize

    implicitWidth: root.isVertical ? root.slotSize : root.fixedLength
    implicitHeight: root.isVertical ? root.slotSize : root.slotHeight
    function toggleMediaControls() {
        if (GlobalStates.mediaControlsOpen) {
            GlobalStates.mediaControlsOpen = false;
            return;
        }
        const topLeft = root.mapToGlobal(0, 0);
        const bottomRight = root.mapToGlobal(root.width, root.height);
        GlobalStates.openMediaControlsFromDock({
            "x": Math.min(topLeft.x, bottomRight.x),
            "y": Math.min(topLeft.y, bottomRight.y),
            "width": Math.abs(bottomRight.x - topLeft.x),
            "height": Math.abs(bottomRight.y - topLeft.y)
        }, String(root.dockContent?.dockPos ?? "bottom"),
            String(root.dockContent?.currentScreen?.name ?? ""));
    }

    // Magnified with the icons, at the muted share the dock keeps for a widget
    // body: the delegate wrapper grows the slot by exactly the room this scale
    // needs, so the neighbours slide instead of being drawn over.
    readonly property real contentMagnification: root.dockContent ? root.dockContent._getSlotMagScale(root) : 1.0
    scale: root.contentMagnification
    transformOrigin: root.dockContent?.magnificationTransformOrigin ?? Item.Bottom

    readonly property real widgetRadius: (Config.options?.dock?.widgetRadius ?? -1) >= 0
        ? Config.options.dock.widgetRadius
        : (Appearance.rounding.windowRounding + 12)

    // ── Media Player State ────────────────────────────────────────────────
    readonly property MprisPlayer currentPlayer: Ryoku.Media.player
    readonly property bool hasPlayer: !!currentPlayer
    readonly property bool isPlaying: currentPlayer?.isPlaying ?? false
    readonly property string title: StringUtils.cleanMusicTitle(
        String(root.currentPlayer?.trackTitle ?? ""))
        || Translation.tr("No media")
    readonly property string artist: String(root.currentPlayer?.trackArtist ?? "")
        || Translation.tr("Unknown Artist")
    readonly property string artUrl: String(root.currentPlayer?.trackArtUrl ?? "")
    readonly property string identity: currentPlayer ? (currentPlayer.identity ?? "") : ""
    readonly property bool hasTrack: hasPlayer
        && String(root.currentPlayer?.trackTitle ?? "").length > 0

    property string displayTitle: ""
    property string displayArtist: ""
    property real titleOpacity: 1.0
    property real titleYOffset: 0.0

    property bool mediaHovered: false
    property bool _initialized: false

    readonly property bool anyMouseContained: dragOverlay.containsMouse || playBtnMouseArea.containsMouse || nextBtnMouseArea.containsMouse || albumArtMouseArea.containsMouse

    onAnyMouseContainedChanged: {
        if (anyMouseContained) {
            hoverExitTimer.stop();
            if (!root.mediaHovered) {
                root.mediaHovered = true;
                if (root.dockContent) root.dockContent.onButtonEntered(root);
            }
        } else {
            hoverExitTimer.restart();
        }
    }

    Timer {
        id: hoverExitTimer
        interval: 80
        repeat: false
        onTriggered: {
            if (!root.anyMouseContained) {
                root.mediaHovered = false;
                if (root.dockContent) root.dockContent.onButtonExited(root);
            }
        }
    }

    // ── Cover Art Resolution & Caching ────────────────────────────────────
    property string artDownloadLocation: Directories.coverArt
    property bool isLocalArt: root.artUrl.startsWith("file://") || root.artUrl.startsWith("/")
    property string artFileName: root.artUrl !== "" ? Qt.md5(root.artUrl) : ""
    property string artFilePath: root.artFileName !== "" ? `${artDownloadLocation}/${root.artFileName}` : ""
    property bool artDownloaded: false

    readonly property string effectiveArtSource: {
        if (!root.artUrl || root.artUrl === "") return "";
        if (root.isLocalArt) {
            return root.artUrl.startsWith("/") ? ("file://" + root.artUrl) : root.artUrl;
        }
        if (root.artDownloaded && root.artFilePath !== "") {
            return "file://" + root.artFilePath;
        }
        return root.artUrl;
    }

    Process {
        id: artDownloader
        property string targetFile: ""
        property string filePath: ""
        property string tempPath: ""
        command: ["bash", "-c", `mkdir -p '${root.artDownloadLocation}' && ( [ -f '${filePath}' ] || (curl -4 -sSL '${StringUtils.shellSingleQuoteEscape(targetFile)}' -o '${tempPath}' && mv '${tempPath}' '${filePath}') )`]
        onExited: (exitCode, exitStatus) => {
            root.artDownloaded = (exitCode === 0);
        }
    }

    function checkAndDownloadArt() {
        if (!root.artUrl || root.artUrl === "") {
            root.artDownloaded = false;
            return;
        }
        if (root.isLocalArt) {
            root.artDownloaded = true;
            return;
        }
        artDownloader.targetFile = root.artUrl;
        artDownloader.filePath = root.artFilePath;
        artDownloader.tempPath = root.artFilePath + ".tmp";
        artDownloader.running = true;
    }

    onArtUrlChanged: {
        root.checkAndDownloadArt();
    }

    // ── Dynamic Color Scheme & M3 Tokens ──────────────────────────────────
    readonly property bool useDynamicColors: (Config.options?.media?.dynamicAlbumColors ?? false) && (root.artDownloaded || root.isLocalArt)

    ColorQuantizer {
        id: colorQuantizer
        source: root.artDownloaded ? ("file://" + root.artFilePath) : (root.isLocalArt ? (root.artUrl.startsWith("/") ? ("file://" + root.artUrl) : root.artUrl) : "")
        depth: 0
        rescaleSize: 1
    }

    property color artDominantColor: ColorUtils.mix(
        (colorQuantizer?.colors[0] ?? Appearance.colors.colPrimary),
        Appearance.colors.colPrimaryContainer, 0.8
    ) || Appearance.m3colors.m3secondaryContainer

    property QtObject blendedColors: AdaptedMaterialScheme {
        color: root.artDominantColor
    }

    // ── Semantic Material 3 Expressive Tokens with Guaranteed Contrast ────
    // Surface
    readonly property color cardBgColor: {
        if (root.useDynamicColors) {
            return root.mediaHovered
                ? ColorUtils.mix(blendedColors.colLayer1, blendedColors.colOnLayer1, 0.92)
                : blendedColors.colLayer1;
        }
        return root.mediaHovered ? Appearance.colors.colLayer1Hover : Appearance.colors.colLayer1Base;
    }

    // Typography
    readonly property color cardTextColor: root.useDynamicColors
        ? blendedColors.colOnLayer0
        : Appearance.colors.colOnSurface

    readonly property color cardSubtextColor: root.useDynamicColors
        ? blendedColors.colSubtext
        : Appearance.colors.colOnSurfaceVariant

    // Play / Pause Button
    readonly property color btnPlayBg: root.isPlaying
        ? (root.useDynamicColors ? blendedColors.colPrimary : Appearance.colors.colPrimary)
        : (root.useDynamicColors ? blendedColors.colPrimaryContainer : Appearance.colors.colPrimaryContainer)

    readonly property color btnPlayBgHover: root.isPlaying
        ? (root.useDynamicColors ? blendedColors.colPrimaryHover : Appearance.colors.colPrimaryHover)
        : (root.useDynamicColors ? blendedColors.colPrimaryContainerHover : Appearance.colors.colPrimaryContainerHover)

    readonly property color btnPlayIconColor: root.isPlaying
        ? (root.useDynamicColors ? blendedColors.colOnPrimary : Appearance.colors.colOnPrimary)
        : (root.useDynamicColors ? blendedColors.colOnPrimaryContainer : Appearance.colors.colOnPrimaryContainer)

    // Next Button
    readonly property color btnNextBg: root.useDynamicColors
        ? blendedColors.colSecondaryContainer
        : Appearance.colors.colSecondaryContainer

    readonly property color btnNextBgHover: root.useDynamicColors
        ? blendedColors.colSecondaryContainerHover
        : Appearance.colors.colSecondaryContainerHover

    readonly property color btnNextIconColor: root.useDynamicColors
        ? blendedColors.colOnSecondaryContainer
        : Appearance.colors.colOnSecondaryContainer

    // ── Track Text Transition Animation ────────────────────────────────────
    Connections {
        target: root.currentPlayer
        function onTrackTitleChanged() {
            root.displayTitle = root.title;
            root.displayArtist = root.artist;
        }
        function onTrackArtistChanged() {
            root.displayArtist = root.artist;
        }
    }

    onTitleChanged: {
        if (root.displayTitle === "") {
            root.displayTitle = root.title;
            root.displayArtist = root.artist;
        } else if (songSwitchAnimation) {
            songSwitchAnimation.stop();
            songSwitchAnimation.start();
        }
    }

    onIdentityChanged: {
        if (root.displayTitle !== "" && songSwitchAnimation) {
            songSwitchAnimation.stop();
            songSwitchAnimation.start();
        }
    }

    SequentialAnimation {
        id: songSwitchAnimation
        ParallelAnimation {
            NumberAnimation {
                target: root
                property: "titleOpacity"
                to: 0.0
                duration: Appearance.reducedMotion ? 0 : 120
                easing.type: Easing.OutQuad
            }
            NumberAnimation {
                target: root
                property: "titleYOffset"
                to: -8
                duration: Appearance.reducedMotion ? 0 : 120
                easing.type: Easing.OutQuad
            }
        }
        PropertyAction { target: root; property: "displayTitle"; value: root.title }
        PropertyAction { target: root; property: "displayArtist"; value: root.artist }
        PropertyAction { target: root; property: "titleYOffset"; value: 8 }
        ParallelAnimation {
            NumberAnimation {
                target: root
                property: "titleOpacity"
                to: 1.0
                duration: Appearance.reducedMotion ? 0 : 180
                easing.type: Easing.OutCubic
            }
            NumberAnimation {
                target: root
                property: "titleYOffset"
                to: 0.0
                duration: Appearance.reducedMotion ? 0 : 180
                easing.type: Easing.OutCubic
            }
        }
    }

    // ── Android-Style Album Art Crossfade & Scale Morph Transition ──────────
    property bool imgAActive: true
    property real artScaleA: 1.0
    property real artScaleB: 1.0
    property string activeArtSource: ""

    onEffectiveArtSourceChanged: {
        root.applyArtTransition(root.effectiveArtSource);
    }

    function applyArtTransition(newSrc) {
        if (!root._initialized) {
            artImgA.source = newSrc;
            artImgA.opacity = newSrc !== "" ? 1.0 : 0.0;
            artImgB.source = "";
            artImgB.opacity = 0.0;
            root.imgAActive = true;
            root.artScaleA = 1.0;
            root.artScaleB = 1.0;
            root.activeArtSource = newSrc;
            return;
        }

        const currentActive = root.imgAActive ? artImgA : artImgB;
        const currentIncoming = root.imgAActive ? artImgB : artImgA;

        if (newSrc === root.activeArtSource && currentActive.source !== "") {
            return;
        }

        // Silent upgrade when curl finishes downloading cached local file for the same track
        if (root.artDownloaded && newSrc === ("file://" + root.artFilePath) && currentActive.source === root.artUrl) {
            currentActive.source = newSrc;
            root.activeArtSource = newSrc;
            return;
        }

        root.activeArtSource = newSrc;

        if (newSrc === "") {
            artFadeOutAnim.target = currentActive;
            artFadeOutAnim.restart();
            return;
        }

        if (currentActive.source === "" || currentActive.opacity <= 0.01) {
            currentActive.source = newSrc;
            currentActive.opacity = 1.0;
            if (root.imgAActive) root.artScaleA = 1.0;
            else root.artScaleB = 1.0;
            return;
        }

        currentIncoming.source = newSrc;
        currentIncoming.opacity = 0.0;
        if (root.imgAActive) root.artScaleB = 0.88;
        else root.artScaleA = 0.88;

        if (currentIncoming.status === Image.Ready) {
            root.startCrossfade();
        } else {
            artPreloadTimer.restart();
        }
    }

    function startCrossfade() {
        artPreloadTimer.stop();
        if (artCrossfadeAnim.running) artCrossfadeAnim.stop();

        const active = root.imgAActive ? artImgA : artImgB;
        const incoming = root.imgAActive ? artImgB : artImgA;

        artIncomingFade.target = incoming;
        artIncomingScale.target = root;
        artIncomingScale.property = root.imgAActive ? "artScaleB" : "artScaleA";

        artOutgoingFade.target = active;
        artOutgoingScale.target = root;
        artOutgoingScale.property = root.imgAActive ? "artScaleA" : "artScaleB";

        artCrossfadeAnim.restart();
    }

    Timer {
        id: artPreloadTimer
        interval: 180
        repeat: false
        onTriggered: root.startCrossfade()
    }

    ParallelAnimation {
        id: artCrossfadeAnim
        onFinished: {
            const oldActive = root.imgAActive ? artImgA : artImgB;
            oldActive.source = "";
            oldActive.opacity = 0;
            root.imgAActive = !root.imgAActive;
        }

        NumberAnimation {
            id: artIncomingFade
            property: "opacity"
            from: 0.0
            to: 1.0
            duration: Appearance.reducedMotion ? 0 : 280
            easing.type: Easing.OutCubic
        }
        NumberAnimation {
            id: artIncomingScale
            from: 0.88
            to: 1.0
            duration: Appearance.reducedMotion ? 0 : 280
            easing.type: Easing.OutCubic
        }
        NumberAnimation {
            id: artOutgoingFade
            property: "opacity"
            from: 1.0
            to: 0.0
            duration: Appearance.reducedMotion ? 0 : 220
            easing.type: Easing.OutQuad
        }
        NumberAnimation {
            id: artOutgoingScale
            from: 1.0
            to: 0.90
            duration: Appearance.reducedMotion ? 0 : 220
            easing.type: Easing.OutQuad
        }
    }

    SequentialAnimation {
        id: artFadeOutAnim
        property Item target
        NumberAnimation {
            target: artFadeOutAnim.target
            property: "opacity"
            to: 0.0
            duration: Appearance.reducedMotion ? 0 : 200
            easing.type: Easing.OutQuad
        }
        ScriptAction {
            script: {
                if (artFadeOutAnim.target) artFadeOutAnim.target.source = "";
            }
        }
    }

    Component.onCompleted: {
        root.displayTitle = root.title;
        root.displayArtist = root.artist;
        root.checkAndDownloadArt();
        root._initialized = true;
        if (root.effectiveArtSource !== "") {
            root.applyArtTransition(root.effectiveArtSource);
        }
    }

    // ── Content Structure ──────────────────────────────────────────────────
    Item {
        id: contentRoot
        anchors.fill: parent
        anchors.leftMargin: root.dotMargin
        anchors.rightMargin: root.dotMargin
        anchors.topMargin: root.dotMarginV
        anchors.bottomMargin: root.dotMarginV

        // Clean Material 3 Expressive Background Surface (No album art background!)
        Rectangle {
            id: cardBg
            anchors.fill: parent
            radius: root.widgetRadius
            color: root.cardBgColor

            Behavior on color {
                enabled: !Appearance.reducedMotion
                animation: Appearance.animation.elementMoveFast.colorAnimation.createObject(cardBg)
            }

            // ── Drag & Reorder Overlay (behind interactive buttons) ─────────
            MouseArea {
                id: dragOverlay
                anchors.fill: parent
                z: 0
                acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton | Qt.BackButton | Qt.ForwardButton
                preventStealing: true
                cursorShape: Qt.PointingHandCursor
                hoverEnabled: true
                property real pressCoord: 0
                property bool dragActive: false

                onPressed: (event) => {
                    if (event.button === Qt.LeftButton) {
                        pressCoord = root.isVertical ? event.y : event.x;
                    }
                }
                onPositionChanged: (event) => {
                    if (!pressed || !(event.buttons & Qt.LeftButton)) return;
                    var cur = root.isVertical ? event.y : event.x;
                    var dist = Math.abs(cur - pressCoord);
                    if (!dragActive && dist > 5 && root.delegateIndex >= 0) {
                        dragActive = true;
                        if (root.dockContent) root.dockContent.startItemDrag(root.delegateIndex, dragOverlay, event.x, event.y);
                    }
                    if (dragActive) {
                        if (root.dockContent) root.dockContent.moveItemDrag(dragOverlay, event.x, event.y);
                    }
                }
                onReleased: (event) => {
                    if (dragActive) {
                        dragActive = false;
                        if (root.dockContent) root.dockContent.endItemDrag();
                        return;
                    }
                    if (event.button === Qt.LeftButton) {
                        root.toggleMediaControls();
                    } else if (event.button === Qt.MiddleButton) {
                        Ryoku.Media.toggle();
                    } else if (event.button === Qt.RightButton || event.button === Qt.ForwardButton) {
                        if (root.currentPlayer?.canGoNext)
                            root.currentPlayer.next();
                    } else if (event.button === Qt.BackButton) {
                        if (root.currentPlayer?.canGoPrevious)
                            root.currentPlayer.previous();
                    }
                }
                onCanceled: {
                    if (dragActive) {
                        dragActive = false;
                        if (root.dockContent) root.dockContent.cancelDrag();
                    }
                }
            }

            // ── Horizontal Layout (3 slots: Artwork squircle, Track info, Transport buttons) ──
            Item {
                id: horizontalView
                anchors.fill: parent
                visible: !root.isVertical

                RowLayout {
                    anchors.fill: parent
                    anchors.leftMargin: 8
                    anchors.rightMargin: 8
                    anchors.topMargin: 4
                    anchors.bottomMargin: 4
                    spacing: 8

                    // 1. Album Art with Android Transition & Squircle Shape
                    Rectangle {
                        id: albumArtWrapper
                        Layout.alignment: Qt.AlignVCenter
                        Layout.preferredWidth: Math.max(28, Math.min(38, cardBg.height - 10))
                        Layout.preferredHeight: Layout.preferredWidth
                        radius: Math.min(Layout.preferredWidth * 0.28, Appearance.rounding.small)
                        color: root.btnNextBg

                        scale: albumArtMouseArea.pressed ? 0.94 : (albumArtMouseArea.containsMouse ? 1.04 : (root.isPlaying ? 1.0 : 0.96))
                        Behavior on scale {
                            enabled: !Appearance.reducedMotion
                            NumberAnimation {
                                duration: Appearance.animation.elementMoveFast.duration
                                easing.type: Easing.OutCubic
                            }
                        }

                        // Antialiased clipping mask
                        Rectangle {
                            id: artMask
                            anchors.fill: parent
                            radius: albumArtWrapper.radius
                            visible: false
                        }

                        layer.enabled: true
                        layer.effect: OpacityMask {
                            maskSource: artMask
                        }

                        // Fallback placeholder icon
                        MaterialSymbol {
                            renderType: Text.CurveRendering
                            anchors.centerIn: parent
                            text: "music_note"
                            iconSize: Math.round(albumArtWrapper.Layout.preferredWidth * 0.52)
                            fill: 1
                            color: root.btnNextIconColor
                            visible: (!artImgA.visible || artImgA.opacity < 0.05) && (!artImgB.visible || artImgB.opacity < 0.05)
                        }

                        // Dual-image crossfade & scale morph
                        StyledImage {
                            id: artImgA
                            anchors.fill: parent
                            fillMode: Image.PreserveAspectCrop
                            cache: true
                            antialiasing: true
                            scale: root.artScaleA
                            visible: opacity > 0.001
                            onStatusChanged: {
                                if (!root.imgAActive && status === Image.Ready && artPreloadTimer.running) {
                                    root.startCrossfade();
                                }
                            }
                        }

                        StyledImage {
                            id: artImgB
                            anchors.fill: parent
                            fillMode: Image.PreserveAspectCrop
                            cache: true
                            antialiasing: true
                            scale: root.artScaleB
                            opacity: 0.0
                            visible: opacity > 0.001
                            onStatusChanged: {
                                if (root.imgAActive && status === Image.Ready && artPreloadTimer.running) {
                                    root.startCrossfade();
                                }
                            }
                        }

                        MouseArea {
                            id: albumArtMouseArea
                            anchors.fill: parent
                            enabled: root.currentPlayer?.canTogglePlaying ?? false
                            hoverEnabled: true
                            cursorShape: enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
                            preventStealing: true
                            onClicked: Ryoku.Media.toggle()
                        }
                    }

                    // 2. Track Title & Artist Info (Flexibly shrinks and elides as Next button expands)
                    ColumnLayout {
                        Layout.fillWidth: true
                        Layout.alignment: Qt.AlignVCenter
                        spacing: 1
                        clip: true

                        StyledText {
                            Layout.fillWidth: true
                            font.pixelSize: Math.max(11, Math.min(13, Math.round(cardBg.height * 0.28)))
                            font.weight: Font.Bold
                            font.styleName: "Rounded"
                            color: root.cardTextColor
                            text: root.displayTitle
                            maximumLineCount: 1
                            elide: Text.ElideRight
                            opacity: root.titleOpacity
                            transform: Translate { y: root.titleYOffset }
                            verticalAlignment: Text.AlignVCenter
                        }

                        StyledText {
                            Layout.fillWidth: true
                            font.pixelSize: Math.max(9, Math.min(11, Math.round(cardBg.height * 0.23)))
                            font.weight: Font.Medium
                            color: root.cardSubtextColor
                            text: root.displayArtist
                            maximumLineCount: 1
                            elide: Text.ElideRight
                            opacity: root.titleOpacity
                            transform: Translate { y: root.titleYOffset }
                            verticalAlignment: Text.AlignVCenter
                        }
                    }

                    // 3. Transport Buttons (Play/Pause permanent, Next reveals on hover)
                    RowLayout {
                        Layout.alignment: Qt.AlignVCenter
                        spacing: Math.round(5 * Math.min(1.0, nextBtn.Layout.preferredWidth / Math.max(1, nextBtn.targetWidth)))
                        z: 5

                        // Play/Pause Button with M3 Shape Morphing
                        Rectangle {
                            id: playBtn
                            implicitWidth: Math.max(30, Math.min(34, cardBg.height - 10))
                            implicitHeight: implicitWidth
                            Layout.alignment: Qt.AlignVCenter

                            // Material 3 Shape as State: Circle when paused ↔ Squircle when playing
                            property real buttonRadius: root.isPlaying ? Appearance.rounding.small : (height / 2)
                            radius: buttonRadius
                            Behavior on buttonRadius {
                                enabled: !Appearance.reducedMotion
                                NumberAnimation {
                                    duration: Appearance.animation.elementMoveFast.duration
                                    easing.type: Easing.OutQuint
                                }
                            }

                            color: playBtnMouseArea.containsMouse ? root.btnPlayBgHover : root.btnPlayBg
                            Behavior on color {
                                enabled: !Appearance.reducedMotion
                                animation: Appearance.animation.elementMoveFast.colorAnimation.createObject(playBtn)
                            }

                            scale: playBtnMouseArea.pressed ? 0.92 : (playBtnMouseArea.containsMouse ? 1.06 : 1.0)
                            opacity: root.currentPlayer?.canTogglePlaying ? 1.0 : 0.55
                            Behavior on scale {
                                enabled: !Appearance.reducedMotion
                                NumberAnimation { duration: 120; easing.type: Easing.OutQuad }
                            }

                            // Interactive Ripple Flash
                            Rectangle {
                                id: playRipple
                                anchors.centerIn: parent
                                width: playBtn.width
                                height: playBtn.height
                                radius: playBtn.radius
                                color: root.btnPlayIconColor
                                opacity: 0
                                visible: opacity > 0.001
                            }
                            SequentialAnimation {
                                id: playRippleAnim
                                NumberAnimation { target: playRipple; property: "opacity"; from: 0.25; to: 0.0; duration: Appearance.reducedMotion ? 0 : 250; easing.type: Easing.OutQuad }
                            }

                            // Icon micro-bounce on state toggle
                            property real iconBounceScale: 1.0
                            Connections {
                                target: root
                                function onIsPlayingChanged() {
                                    if (!Appearance.reducedMotion) {
                                        playBounceAnim.restart();
                                    }
                                }
                            }
                            SequentialAnimation {
                                id: playBounceAnim
                                NumberAnimation { target: playBtn; property: "iconBounceScale"; to: 0.74; duration: 80; easing.type: Easing.OutQuad }
                                NumberAnimation { target: playBtn; property: "iconBounceScale"; to: 1.0; duration: 180; easing.type: Easing.OutBack }
                            }

                            MaterialSymbol {
                                renderType: Text.CurveRendering
                                anchors.centerIn: parent
                                text: root.isPlaying ? "pause" : "play_arrow"
                                iconSize: Math.round(playBtn.height * 0.58)
                                fill: 1
                                color: root.btnPlayIconColor
                                scale: playBtn.iconBounceScale
                            }

                            MouseArea {
                                id: playBtnMouseArea
                                anchors.fill: parent
                                enabled: root.currentPlayer?.canTogglePlaying ?? false
                                hoverEnabled: true
                                cursorShape: enabled ? Qt.PointingHandCursor : Qt.ArrowCursor
                                preventStealing: true

                                onClicked: {
                                    playRippleAnim.restart();
                                    Ryoku.Media.toggle();
                                }
                            }
                        }

                        // Next Button (Reveals on Hover, slides in and pushes title/artist)
                        Rectangle {
                            id: nextBtn
                            readonly property real targetWidth: Math.max(26, Math.min(30, cardBg.height - 14))
                            implicitHeight: targetWidth
                            Layout.alignment: Qt.AlignVCenter
                            Layout.preferredWidth: root.mediaHovered && root.currentPlayer?.canGoNext ? targetWidth : 0
                            radius: targetWidth / 2
                            clip: true
                            opacity: root.mediaHovered ? 1.0 : 0.0
                            visible: Layout.preferredWidth > 0.5 || opacity > 0.01

                            Behavior on Layout.preferredWidth {
                                enabled: !Appearance.reducedMotion
                                NumberAnimation {
                                    duration: Appearance.animation.elementMoveFast.duration
                                    easing.type: Easing.OutCubic
                                }
                            }

                            Behavior on opacity {
                                enabled: !Appearance.reducedMotion
                                NumberAnimation {
                                    duration: Appearance.animation.elementMoveFast.duration
                                    easing.type: Easing.OutCubic
                                }
                            }

                            color: nextBtnMouseArea.containsMouse ? root.btnNextBgHover : root.btnNextBg
                            Behavior on color {
                                enabled: !Appearance.reducedMotion
                                animation: Appearance.animation.elementMoveFast.colorAnimation.createObject(nextBtn)
                            }

                            scale: nextBtnMouseArea.pressed ? 0.90 : (nextBtnMouseArea.containsMouse ? 1.08 : 1.0)
                            Behavior on scale {
                                enabled: !Appearance.reducedMotion
                                NumberAnimation { duration: 120; easing.type: Easing.OutQuad }
                            }

                            // Interactive Ripple Flash
                            Rectangle {
                                id: nextRipple
                                anchors.centerIn: parent
                                width: nextBtn.targetWidth
                                height: nextBtn.targetWidth
                                radius: width / 2
                                color: root.btnNextIconColor
                                opacity: 0
                                visible: opacity > 0.001
                            }
                            SequentialAnimation {
                                id: nextRippleAnim
                                NumberAnimation { target: nextRipple; property: "opacity"; from: 0.25; to: 0.0; duration: Appearance.reducedMotion ? 0 : 250; easing.type: Easing.OutQuad }
                            }

                            property real iconTranslateX: 0.0
                            SequentialAnimation {
                                id: nextNudgeAnim
                                NumberAnimation { target: nextBtn; property: "iconTranslateX"; to: 3.0; duration: 70; easing.type: Easing.OutQuad }
                                NumberAnimation { target: nextBtn; property: "iconTranslateX"; to: 0.0; duration: 150; easing.type: Easing.OutBack }
                            }

                            MaterialSymbol {
                                renderType: Text.CurveRendering
                                anchors.centerIn: parent
                                transform: Translate {
                                    x: nextBtn.iconTranslateX + (1.0 - nextBtn.opacity) * 8
                                }
                                text: "skip_next"
                                iconSize: Math.round(nextBtn.targetWidth * 0.60)
                                fill: 1
                                color: root.btnNextIconColor
                                opacity: nextBtn.opacity
                            }

                            MouseArea {
                                id: nextBtnMouseArea
                                anchors.fill: parent
                                enabled: nextBtn.Layout.preferredWidth > 10
                                hoverEnabled: true
                                cursorShape: Qt.PointingHandCursor
                                preventStealing: true

                                onClicked: {
                                    nextRippleAnim.restart();
                                    if (!Appearance.reducedMotion) {
                                        nextNudgeAnim.restart();
                                    }
                                    if (root.currentPlayer?.canGoNext)
                                        root.currentPlayer.next();
                                }
                            }
                        }
                    }
                }
            }

            // ── Vertical Compact Layout ────────────────────────────────────
            Item {
                id: verticalView
                anchors.fill: parent
                visible: root.isVertical

                Rectangle {
                    id: vertArtWrapper
                    anchors.fill: parent
                    anchors.margins: 3
                    radius: Math.min(width * 0.28, Appearance.rounding.small)
                    color: root.btnNextBg

                    Rectangle {
                        id: vertArtMask
                        anchors.fill: parent
                        radius: vertArtWrapper.radius
                        visible: false
                    }

                    layer.enabled: true
                    layer.effect: OpacityMask {
                        maskSource: vertArtMask
                    }

                    MaterialSymbol {
                        renderType: Text.CurveRendering
                        anchors.centerIn: parent
                        text: "music_note"
                        iconSize: Math.round(parent.width * 0.55)
                        fill: 1
                        color: root.btnNextIconColor
                        visible: !vertArtImg.visible || vertArtImg.opacity < 0.05
                    }

                    StyledImage {
                        id: vertArtImg
                        anchors.fill: parent
                        fillMode: Image.PreserveAspectCrop
                        cache: true
                        antialiasing: true
                        source: root.effectiveArtSource
                        visible: status === Image.Ready && source !== ""
                    }
                }

                // Morphing Play/Pause Badge Overlay in Vertical Mode
                Rectangle {
                    anchors.centerIn: parent
                    width: parent.width * 0.60
                    height: width
                    radius: root.isPlaying ? Appearance.rounding.small : (height / 2)
                    color: ColorUtils.transparentize(root.btnPlayBg, root.mediaHovered ? 0.05 : 0.25)
                    scale: root.mediaHovered ? 1.08 : 0.95

                    Behavior on radius {
                        enabled: !Appearance.reducedMotion
                        NumberAnimation { duration: 250; easing.type: Easing.OutQuint }
                    }
                    Behavior on scale {
                        enabled: !Appearance.reducedMotion
                        NumberAnimation { duration: 150; easing.type: Easing.OutQuad }
                    }
                    Behavior on color {
                        enabled: !Appearance.reducedMotion
                        animation: Appearance.animation.elementMoveFast.colorAnimation.createObject(this)
                    }

                    MaterialSymbol {
                        renderType: Text.CurveRendering
                        anchors.centerIn: parent
                        text: root.isPlaying ? "pause" : "play_arrow"
                        iconSize: Math.round(parent.height * 0.60)
                        fill: 1
                        color: root.btnPlayIconColor
                    }
                }
            }
        }
    }

    // ── Tooltip ────────────────────────────────────────────────────────────
    DockTooltip {
        id: mediaTooltip
        parentItem: root
        text: root.hasTrack ? (root.displayTitle + " · " + root.displayArtist) : Translation.tr("No media")
        showTooltip: root.mediaHovered && !GlobalStates.mediaControlsOpen
        tooltipOffset: -root.dotMargin * 0.5
    }
}
