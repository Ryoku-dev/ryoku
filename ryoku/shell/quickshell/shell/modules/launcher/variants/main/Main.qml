import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import Ryoku.Ui.Singletons
import "../../shared/Singletons"
import "../../shared/providers" as SharedProviders
import "." as MainVariant
import "../../../../services/lib/screens.js" as Screens

// Main's compact command palette: a separate search row over an image-backed
// dashboard, loaded behind the stable launcher selector.
Scope {
    id: root

    property string openMon: ""
    property var activeSurface: null
    property var activeLauncher: null
    property var activeBt: null
    readonly property bool open: openMon !== ""
    readonly property bool shown: open
        || (activeSurface !== null && activeSurface.visible)

    // launcher weather units from the config: "auto" follows the locale.
    Binding {
        target: Weather
        property: "unitOverride"
        value: LauncherConfig.weatherUnit === "auto" ? "" : LauncherConfig.weatherUnit
    }

    function focusedMonitor() {
        var name = Wm.focusedOutput;
        return name ? name : (Quickshell.screens.length > 0 ? Quickshell.screens[0].name : "");
    }

    function show(mon) {
        root.openMon = (mon && mon.length) ? mon : root.focusedMonitor();
    }
    function hide() {
        root.openMon = "";
    }
    function toggle(mon) {
        if (root.open)
            root.hide();
        else
            root.show(mon);
    }

    function stateDump() {
        var dump = root.activeLauncher ? root.activeLauncher.stateDump()
            : { query: "", resultCount: 0, selectedIndex: -1 };
        dump.open = root.shown;
        dump.monitor = root.openMon;
        dump.btConnected = root.activeBt ? root.activeBt.connected.length : 0;
        return dump;
    }
    SharedProviders.Providers {
        id: sharedProviders
    }


    Variants {
        model: Screens.uniqueByName(Quickshell.screens)

        PanelWindow {
            id: win
            required property var modelData
            // cap the monitor-derived scale so a tall display doesn't balloon the
            // palette; 1.0 at 1080p PHYSICAL, at most 1.2 on bigger screens,
            // times fontScale. Physical: Quickshell's height is logical, and a
            // logical-only term shrinks the palette as the display scale rises.
            readonly property real s: Screens.monitorScale(modelData, Wm.outputScale(modelData ? modelData.name : ""), 1.2) * Math.max(0.8, Math.min(1.4, Config.fontScale)) * Tokens.uiScaleFor(modelData ? modelData.name : "")
            readonly property bool shown: root.openMon === modelData.name

            screen: modelData
            visible: shown || closing.running
            color: "transparent"
            exclusionMode: ExclusionMode.Ignore
            WlrLayershell.namespace: "launcher"
            WlrLayershell.layer: WlrLayer.Overlay
            // hold the grab until the window unmaps (end of the close morph).
            // dropping it while still mapped strands the keyboard on the dead
            // layer: the app looks focused but can't type until a real focus
            // change. unmapping with the grab held hands the keyboard back.
            WlrLayershell.keyboardFocus: WlrKeyboardFocus.Exclusive

            anchors { top: true; bottom: true; left: true; right: true }

            // a brief grace so the close morph can play before the window drops.
            Timer { id: closing; interval: Motion.open; repeat: false }
            onShownChanged: {
                if (shown) {
                    closing.stop();
                    root.activeSurface = win;
                    root.activeLauncher = launcher;
                    root.activeBt = btBubbles;
                } else {
                    closing.restart();
                }
            }
            onVisibleChanged: {
                if (!visible && root.activeSurface === win)
                    root.activeSurface = null;
            }

            // blurred backdrop: a still snapshot of the desktop presented through
            // a Qt blur so the palette floats over frost without driving the
            // compositor. captured once per open (live: false) so the full-screen
            // window never samples itself. skipped on weak GPUs, at zero blur,
            // and where the compositor cannot host a long-lived capturing
            // surface: this view lives as long as the per-screen window, not as
            // long as one open.
            Item {
                id: frost
                anchors.fill: parent
                readonly property int radius: LauncherConfig.bgBlur | 0
                readonly property bool wanted: !Performance.blurDisabled
                    && radius > 0
                    && Wm.caps.persistentScreenCapture === true
                visible: opacity > 0.001 && frostCapture.hasContent
                opacity: win.shown ? 1 : 0
                Behavior on opacity { NumberAnimation { duration: Motion.open; easing.type: Easing.OutCubic } }

                ScreencopyView {
                    id: frostCapture
                    anchors.fill: parent
                    visible: false
                    live: false
                    paintCursor: false
                    captureSource: (frost.wanted && win.visible) ? win.modelData : null
                }

                ShaderEffectSource {
                    id: frostSource
                    anchors.fill: parent
                    sourceItem: frostCapture
                    live: frost.visible
                    hideSource: false
                    recursive: false
                    smooth: true
                    visible: false
                }

                MultiEffect {
                    anchors.fill: parent
                    source: frostSource
                    autoPaddingEnabled: false
                    blurEnabled: true
                    blur: 1
                    blurMax: Math.max(1, Math.min(64, frost.radius))
                }
            }

            // dim + click-out scrim.
            Rectangle {
                anchors.fill: parent
                color: Qt.rgba(0, 0, 0, 0.35)
                opacity: win.shown ? 1 : 0
                visible: opacity > 0.001
                Behavior on opacity { NumberAnimation { duration: Motion.open; easing.type: Easing.OutCubic } }
                MouseArea { anchors.fill: parent; onClicked: root.hide() }
            }

            MainVariant.Launcher {
                id: launcher
                providerSet: sharedProviders
                anchors.horizontalCenter: parent.horizontalCenter
                anchors.top: parent.top
                // off resting height, not the live one, so growing results push the
                // body down while the search row holds its position.
                anchors.topMargin: Math.round((parent.height - launcher.restingHeight) * 0.32)
                s: win.s
                shown: win.shown
                onRequestClose: root.hide()
            }

            // Detached Bluetooth bubbles under the palette: one square card
            // per connected device (BtConnections renders nothing otherwise),
            // riding the same open/close morph as the card above.
            MainVariant.BtConnections {
                id: btBubbles
                anchors.horizontalCenter: parent.horizontalCenter
                anchors.top: launcher.bottom
                anchors.topMargin: 10 * win.s
                width: launcher.width
                s: win.s
                transformOrigin: Item.Top
                opacity: win.shown ? 1 : 0
                scale: win.shown ? 1 : 0.92
                Behavior on opacity { NumberAnimation { duration: Motion.open; easing.type: Easing.OutCubic } }
                Behavior on scale {
                    NumberAnimation { duration: Motion.open; easing.type: Motion.easeMorph; easing.bezierCurve: Motion.morphCurve }
                }
            }
        }
    }
}
