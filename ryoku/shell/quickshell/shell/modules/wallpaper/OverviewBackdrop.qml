pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Wayland
import Ryoku.Ui.Singletons
import "Singletons"

// An extra Background-layer surface that draws the current wallpaper for niri's
// overview backdrop and the gaps it exposes during workspace switches.
//
// It sits below the desktop's opaque wallpaper (modules/desktop, the
// ryoku-widgets Bottom-layer surface), so normal desktop use reads as before.
//
// The window stays mapped because niri needs it before either animation starts.
// The wallpaper decode lives while the overview is open or for 20 seconds after
// a workspace switch. That decode was effectively the surface's whole ~45 MB
// cost at 2560x1600. After an idle spell, the first switch shows the plain
// backdrop colour while the image loads for following switches. Nothing maps on
// a compositor with no overview backdrop.
Item {
    id: root

    required property var screen
    // The active compositor can host a surface inside its overview backdrop.
    property bool available: false
    // The compositor's native overview is open, so the backdrop is on screen.
    property bool overviewOpen: false
    // The live wallpaper source the desktop paints (file url + revision), reused
    // so a wallpaper change carries straight into the backdrop with no second
    // pipeline to keep in sync.
    property string wallpaperUrl: ""

    readonly property string screenWorkspaceId: {
        const screenName = root.screen ? root.screen.name : "";
        const output = screenName.length > 0 ? Wm.outputByName(screenName) : null;
        const outputWorkspace = output ? String(output.activeWorkspace ?? "") : "";
        if (outputWorkspace.length > 0)
            return outputWorkspace;
        const focused = Wm.focusedWorkspace;
        return focused ? String(focused.id ?? focused.name ?? "") : "";
    }

    readonly property bool shown: root.available && OverviewBackdropConfig.enabled
    readonly property bool active: root.shown && root.source.length > 0
    readonly property bool imageWanted: root.active
        && (root.overviewOpen || imageHold.running)
    readonly property bool blurWanted: root.active && root.overviewOpen
        && OverviewBackdropConfig.blurEnabled
    property bool blurHeld: false

    onOverviewOpenChanged: {
        if (!root.overviewOpen && root.active)
            imageHold.restart();
    }

    onScreenWorkspaceIdChanged: {
        if (root.screenWorkspaceId.length > 0)
            imageHold.restart();
    }

    onBlurWantedChanged: {
        if (root.blurWanted) {
            blurRelease.stop();
            root.blurHeld = true;
            captureTimer.restart();
        } else {
            blurRelease.restart();
        }
    }

    Component.onCompleted: {
        root.blurHeld = root.blurWanted;
        if (root.blurHeld)
            captureTimer.restart();
    }

    Timer {
        id: imageHold
        interval: 20000
    }

    Timer {
        id: blurRelease
        interval: Motion.reduce ? 0 : Motion.wallpaperFade
        onTriggered: {
            if (!root.blurWanted)
                root.blurHeld = false;
        }
    }

    Timer {
        id: captureTimer
        interval: 0
        onTriggered: {
            if (root.blurHeld && img.status === Image.Ready)
                blurSource.scheduleUpdate();
        }
    }

    // Follow the live wallpaper, or paint a per-card backdrop image the user
    // pinned when they turned following off and set one.
    readonly property string source: {
        const custom = OverviewBackdropConfig.customPath;
        if (!OverviewBackdropConfig.followWallpaper && custom.length > 0)
            return custom.indexOf("file://") === 0 ? custom : "file://" + custom;
        return root.wallpaperUrl;
    }

    PanelWindow {
        id: win
        // niri also exposes this backdrop between workspaces and needs it
        // mapped before the overview opening animation begins.
        visible: root.active
        screen: root.screen
        color: "transparent"
        exclusionMode: ExclusionMode.Ignore
        WlrLayershell.layer: WlrLayer.Background
        WlrLayershell.namespace: "ryoku-overview-backdrop"
        WlrLayershell.keyboardFocus: WlrKeyboardFocus.None

        anchors {
            top: true
            left: true
            right: true
            bottom: true
        }

        Item {
            anchors.fill: parent
            clip: true
            opacity: img.status === Image.Ready ? 1 : 0

            Behavior on opacity {
                enabled: !Motion.reduce
                NumberAnimation {
                    duration: Motion.wallpaperFade
                    easing.type: Easing.OutCubic
                }
            }

            // The sharp wallpaper is the workspace-switch backdrop. It is the
            // only file decode; the overview blur samples it into a reduced
            // transient texture rather than decoding the file again.
            Image {
                id: img
                anchors.fill: parent
                anchors.margins: -64
                source: root.imageWanted ? root.source : ""
                fillMode: Image.PreserveAspectCrop
                cache: false
                asynchronous: true
                smooth: true
                sourceSize.width: Math.max(1, Math.ceil(win.width
                    * (win.screen ? win.screen.devicePixelRatio : 1)))
                sourceSize.height: Math.max(1, Math.ceil(win.height
                    * (win.screen ? win.screen.devicePixelRatio : 1)))
                onStatusChanged: {
                    if (status === Image.Ready && root.blurHeld)
                        captureTimer.restart();
                }
            }

            ShaderEffectSource {
                id: blurSource
                anchors.fill: img
                sourceItem: root.blurHeld ? img : null
                textureSize: Qt.size(
                    Math.max(1, Math.ceil(img.width
                        * (win.screen ? win.screen.devicePixelRatio : 1) / 4)),
                    Math.max(1, Math.ceil(img.height
                        * (win.screen ? win.screen.devicePixelRatio : 1) / 4)))
                live: false
                recursive: false
                hideSource: false
                smooth: true
                visible: false
            }

            MultiEffect {
                id: blurFx
                anchors.fill: img
                source: blurSource
                blurEnabled: root.blurHeld
                blur: root.blurWanted
                    ? Math.min(1, OverviewBackdropConfig.blur / 100) : 0
                // The source texture is quarter-linear; 16 samples enlarge to
                // the previous 64 px visible radius.
                blurMax: 16
                visible: img.status === Image.Ready && root.blurHeld

                Behavior on blur {
                    enabled: !Motion.reduce
                    NumberAnimation {
                        duration: Motion.wallpaperFade
                        easing.type: Easing.OutCubic
                    }
                }
            }

            Rectangle {
                anchors.fill: parent
                color: "black"
                opacity: OverviewBackdropConfig.dim / 100
                visible: opacity > 0
            }
        }
    }
}
