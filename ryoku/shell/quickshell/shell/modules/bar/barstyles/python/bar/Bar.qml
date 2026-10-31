import QtQuick
import QtQuick.Layouts
import QtQuick.Window
import QtQuick.Controls
import Quickshell
import Quickshell.Io
import Quickshell.Wayland
import Ryoku.Ui.Singletons
import "../reusables"
import "../"
import "../../../../../services/lib/screens.js" as Screens
import shell.modules.stage.frame

Variants {
    // One band per screen that still shows a bar (Ryoku Settings > Displays).
    model: Screens.uniqueByName(Quickshell.screens).filter(screen => Tokens.barEnabledFor(screen.name))
    delegate: Component {
        PanelWindow {
            id: barWindow
            visible: barConfigReady

            property bool pendingReload: false
            property bool startupFilesReady: false

            property bool isFullscreenActive: Wm.outputHasFullscreen(
                barWindow.screen ? barWindow.screen.name : "")

            onIsFullscreenActiveChanged: {
                if (barWindowRoot.isFullscreenActive) {
                    OsdController.isFullscreen = true;
                } else if (!OsdController.screen || OsdController.screen === barWindowRoot.screen) {
                    OsdController.isFullscreen = false;
                }
            }

            WlrLayershell.keyboardFocus: (barWindowRoot.isNotifOpen || barWindowRoot.isSysOpen)
                ? WlrKeyboardFocus.OnDemand
                : WlrKeyboardFocus.None

            property int configRevision: 0

            Connections {
                target: (typeof Config !== "undefined") ? Config : null
                function onSettingsLoaded() {
                    barWindowRoot.configRevision++;
                }
                function onDataReadyChanged() {
                    barWindowRoot.configRevision++;
                }
                function onRawSettingsChanged() {
                    barWindowRoot.configRevision++;
                }
            }

            Component.onCompleted: {
                if (typeof Config !== "undefined" && Config.dataReady) {
                    barWindowRoot.configRevision++;
                }
            }

            property string barStyle: {
                let dummy = configRevision;
                if (typeof Config === "undefined" || !Config.rawSettings || !Config.rawSettings.bar) return "modular";
                let s = Config.rawSettings.bar.style;
                if (typeof s === "string") return s;
                if (s && typeof s === "object") {
                    if (s.fill || s.mode === "fill") return "fill";
                    if (s.solid || s.mode === "solid") return "solid";
                }
                return "modular";
            }
            property bool isFill: barStyle === "fill"
            property bool isSolid: barStyle === "solid" || barStyle === "fill"
            property bool distinctPills: {
                let dummy = configRevision;
                return (typeof Config !== "undefined" && Config.rawSettings && Config.rawSettings.bar && Config.rawSettings.bar.distinctPills !== undefined) ? Config.rawSettings.bar.distinctPills : false;
            }
            property bool centerByEdges: {
                let dummy = configRevision;
                return (typeof Config !== "undefined" && Config.rawSettings && Config.rawSettings.bar && Config.rawSettings.bar.centerByEdges !== undefined) ? Config.rawSettings.bar.centerByEdges : true;
            }

            property bool barConfigReady: {
                let dummy = configRevision;
                if (typeof Config === "undefined") return true;
                if (Config.dataReady !== undefined) return Config.dataReady;
                return true;
            }

            property bool autohide: {
                let dummy = configRevision;
                return (typeof Config !== "undefined" && Config.rawSettings && Config.rawSettings.bar && Config.rawSettings.bar.autohide !== undefined) ? Config.rawSettings.bar.autohide : false;
            }
            property int autohideTimeout: {
                let dummy = configRevision;
                return (typeof Config !== "undefined" && Config.rawSettings && Config.rawSettings.bar && Config.rawSettings.bar.autohideTimeout !== undefined) ? Config.rawSettings.bar.autohideTimeout : 1000;
            }
            property real barOpacity: {
                let dummy = configRevision;
                if (typeof Config !== "undefined" && Config.rawSettings && Config.rawSettings.bar
                        && Config.rawSettings.bar.opacity !== undefined) {
                    return Math.max(0, Math.min(1, Number(Config.rawSettings.bar.opacity) / 100.0));
                }
                return ThemeBackend.elementOpacity;
            }

            HoverHandler {
                id: barHover
                acceptedDevices: PointerDevice.Mouse | PointerDevice.TouchPad
            }

            Timer {
                id: hideTimer
                interval: barWindowRoot.autohideTimeout
            }

            function checkHideTimer() {
                if (!barHover.hovered && barWindowRoot.autohide) {
                    hideTimer.restart();
                } else {
                    hideTimer.stop();
                }
            }

            Connections {
                target: barHover
                function onHoveredChanged() { barWindowRoot.checkHideTimer(); }
            }

            property bool isRevealed: {
                if (frameLayer.framed) return true;
                if (!autohide) return true;
                if (barHover.hovered) return true;
                if (hideTimer.running) return true;
                return false;
            }

            Timer {
                interval: 50
                running: true
                onTriggered: barWindowRoot.startupFilesReady = true
            }

            IpcHandler {
                target: "topbar"
                // Ryoku seam: no whole-shell reload from inside the resident
                // Ryoku instance; the style's settings apply live through the
                // daemon frames. The verbs stay so the ported scripts land.
                function forceReload() {
                    console.info("python: settings apply live; shell reload skipped")
                }
                function queueReload() {
                    barWindow.pendingReload = false
                }
                function toggleUpdate() {
                    let target = null;
                    let layout = barWindowRoot.isVertical ? verticalLoader.item : horizontalLoader.item;
                    if (layout) target = layout.getWidget("left");
                    if (target && typeof target.toggleUpdate === "function") {
                        target.toggleUpdate();
                    }
                }
            }

            required property var modelData
            screen: modelData

            property string barPosition: {
                let dummy = configRevision;
                return (typeof Config !== "undefined" && Config.rawSettings && Config.rawSettings.bar && Config.rawSettings.bar.position !== undefined) ? Config.rawSettings.bar.position : "top";
            }
            property bool positionChanging: false

            property real barWidthPercent: {
                let dummy = configRevision;
                return (typeof Config !== "undefined" && Config.rawSettings && Config.rawSettings.bar && Config.rawSettings.bar.width !== undefined) ? Config.rawSettings.bar.width : 100;
            }
            property real edgePadding: (autohide && !isFill) ? s(4) : 0
            readonly property real barContentWidth: frameLayer.rest.width
            readonly property real barContentHeight: frameLayer.rest.height
            property real effectiveBarWidth: Math.round(isVertical ? barContentWidth : ((barContentWidth - (autohide ? edgePadding * 2 : 0)) * (barWidthPercent / 100.0)))
            property real horizontalOffset: Math.round(isVertical ? 0 : ((barContentWidth - effectiveBarWidth) / 2))

            property real effectiveBarHeight: Math.round(!isVertical ? barContentHeight : ((barContentHeight - (autohide ? edgePadding * 2 : 0)) * (barWidthPercent / 100.0)))
            property real verticalOffset: Math.round(!isVertical ? 0 : ((barContentHeight - effectiveBarHeight) / 2))

            readonly property var horizontalLayout: horizontalLoader.item
            readonly property var verticalLayout: verticalLoader.item

            property real currentBarMinX: (horizontalLayout && horizontalLayout.dynamicMaxX > horizontalLayout.dynamicMinX) ? horizontalLayout.dynamicMinX : horizontalOffset
            property real currentBarMaxX: (horizontalLayout && horizontalLayout.dynamicMaxX > horizontalLayout.dynamicMinX) ? horizontalLayout.dynamicMaxX : (horizontalOffset + effectiveBarWidth)

            property real currentBarMinY: (verticalLayout && verticalLayout.dynamicMaxY > verticalLayout.dynamicMinY) ? verticalLayout.dynamicMinY : verticalOffset
            property real currentBarMaxY: (verticalLayout && verticalLayout.dynamicMaxY > verticalLayout.dynamicMinY) ? verticalLayout.dynamicMaxY : (verticalOffset + effectiveBarHeight)

            property bool fillAttached: isFill && (isVertical
                ? (currentBarMinY > 0.5 || currentBarMaxY < barContentHeight - 0.5)
                : (currentBarMinX > 0.5 || currentBarMaxX < barContentWidth - 0.5))

            Timer {
                id: positionChangeTimer
                interval: 200
                onTriggered: barWindowRoot.positionChanging = false
            }

            onBarPositionChanged: {
                barWindowRoot.positionChanging = true;
                positionChangeTimer.restart();
            }

            property bool isVertical: barPosition === "left" || barPosition === "right"

            property real baseScale: Scaler.baseScale

            function s(val) {
                return Math.round(Scaler.s(val));
            }

            readonly property real baseThickness: 40
            readonly property real minThickness: 28
            readonly property real maxThickness: 48
            property real thicknessSetting: {
                let dummy = configRevision;
                let v = (typeof Config !== "undefined" && Config.rawSettings && Config.rawSettings.bar && Config.rawSettings.bar.height !== undefined) ? Number(Config.rawSettings.bar.height) : baseThickness;
                if (isNaN(v)) v = baseThickness;
                return Math.max(minThickness, Math.min(maxThickness, v));
            }
            property real contentRatio: thicknessSetting / baseThickness

            function cs(val) {
                return Math.round(Scaler.s(val) * contentRatio);
            }

            property int barHeight: s(thicknessSetting)
            property real cornerRadius: s(12)
            readonly property real surfaceThickness: barHeight + (isFill ? cornerRadius : edgePadding)

            property real baseOffsetY: {
                if (barPosition === "bottom") {
                    return isFill ? (barContentHeight - barHeight) : (barContentHeight - barHeight - edgePadding);
                } else {
                    return isFill ? 0 : edgePadding;
                }
            }

            anchors {
                top: barPosition === "top" || barWindowRoot.isVertical
                bottom: barPosition === "bottom" || barWindowRoot.isVertical
                left: barPosition === "left" || !barWindowRoot.isVertical
                right: barPosition === "right" || !barWindowRoot.isVertical
            }

            implicitHeight: barWindow.isVertical ? 0 : frameLayer.extent
            implicitWidth: barWindow.isVertical ? frameLayer.extent : 0

            margins {
                top: isFill ? 0 : (barPosition === "bottom" ? 0 : (autohide ? 0 : s(4)))
                bottom: isFill ? 0 : (barPosition === "top" ? 0 : (autohide ? 0 : s(4)))
                left: isFill ? 0 : (barPosition === "right" ? 0 : (autohide ? 0 : s(4)))
                right: isFill ? 0 : (barPosition === "left" ? 0 : (autohide ? 0 : s(4)))
            }

            exclusiveZone: (!barConfigReady || autohide) ? 0 : barHeight
            color: "transparent"

            property real activeMaskHeight: (autohide && !isRevealed) ? s(4) : (isVertical ? (isFill ? barContentHeight : (effectiveBarHeight + edgePadding * 2)) : (isFill ? (barHeight + cornerRadius) : (barHeight + edgePadding)))
            property real activeMaskWidth: (autohide && !isRevealed) ? s(4) : (isVertical ? (isFill ? (barHeight + cornerRadius) : (barHeight + edgePadding)) : (isFill ? barContentWidth : (effectiveBarWidth + edgePadding * 2)))

            readonly property rect mainInputRect: frameLayer.maskRect(
                isVertical ? (barPosition === "right" ? (barContentWidth - activeMaskWidth) : 0) : currentBarMinX,
                isVertical ? currentBarMinY : (barPosition === "bottom" ? baseOffsetY : 0),
                isVertical ? activeMaskWidth : (currentBarMaxX - currentBarMinX),
                isVertical ? (currentBarMaxY - currentBarMinY) : (isFill ? barHeight : activeMaskHeight))
            readonly property rect startCornerInputRect: frameLayer.maskRect(
                isVertical ? (barPosition === "right" ? (barContentWidth - barHeight - cornerRadius) : barHeight) : 0,
                isVertical ? 0 : (barPosition === "bottom" ? (baseOffsetY - cornerRadius) : (baseOffsetY + barHeight)),
                (isFill && !fillAttached) ? cornerRadius : 0,
                (isFill && !fillAttached) ? cornerRadius : 0)
            readonly property rect endCornerInputRect: frameLayer.maskRect(
                isVertical ? (barPosition === "right" ? (barContentWidth - barHeight - cornerRadius) : barHeight) : (barContentWidth - cornerRadius),
                isVertical ? (barContentHeight - cornerRadius) : (barPosition === "bottom" ? (baseOffsetY - cornerRadius) : (baseOffsetY + barHeight)),
                (isFill && !fillAttached) ? cornerRadius : 0,
                (isFill && !fillAttached) ? cornerRadius : 0)

            mask: Region {
                Region {
                    x: Math.round(barWindow.mainInputRect.x)
                    y: Math.round(barWindow.mainInputRect.y)
                    width: Math.round(barWindow.mainInputRect.width)
                    height: Math.round(barWindow.mainInputRect.height)
                }
                Region {
                    x: Math.round(barWindow.startCornerInputRect.x)
                    y: Math.round(barWindow.startCornerInputRect.y)
                    width: Math.round(barWindow.startCornerInputRect.width)
                    height: Math.round(barWindow.startCornerInputRect.height)
                }
                Region {
                    x: Math.round(barWindow.endCornerInputRect.x)
                    y: Math.round(barWindow.endCornerInputRect.y)
                    width: Math.round(barWindow.endCornerInputRect.width)
                    height: Math.round(barWindow.endCornerInputRect.height)
                }
            }

            property string activeWidget: ""
            property bool isNotifOpen: activeWidget === "notifications"
            property bool isSysOpen: activeWidget === "system"

            onActiveWidgetChanged: {
                if (!barWindow.isNotifOpen && !barWindow.isSysOpen && barWindow.pendingReload) {
                    barWindow.pendingReload = false;
                }
            }

            FileView {
                id: widgetWatcher
                path: (barWindowRoot.startupFilesReady && Caching.runDir) ? (Caching.runDir + "/current_widget") : ""
                watchChanges: true
                onFileChanged: reload()
                onLoaded: {
                    let txt = text().trim();
                    let widget = "";
                    let targetScreen = "";

                    try {
                        let parsed = JSON.parse(txt);
                        if (parsed && typeof parsed === "object") {
                            widget = parsed.widget || "";
                            targetScreen = parsed.screen || "";
                        } else if (typeof parsed === "string") {
                            widget = parsed;
                        }
                    } catch (e) {
                        widget = txt;
                    }

                    let myScreenName = (barWindowRoot.screen && barWindowRoot.screen.name) ? barWindowRoot.screen.name : "";
                    let effectiveWidget = "";
                    if (widget === "notifications" || widget === "system") {
                        if (!targetScreen || targetScreen === myScreenName) {
                            effectiveWidget = widget;
                        }
                    }

                    if (barWindowRoot.activeWidget !== effectiveWidget) {
                        barWindowRoot.activeWidget = effectiveWidget;
                    }
                }
            }

            property bool isStartupReady: false
            Timer { interval: 10; running: true; onTriggered: barWindowRoot.isStartupReady = true }

            property bool startupCascadeFinished: false
            Timer { interval: 1050; running: true; onTriggered: barWindowRoot.startupCascadeFinished = true }

            property bool fastPollerLoaded: false
            property bool isDataReady: fastPollerLoaded
            Timer { interval: 400; running: true; onTriggered: barWindow.isDataReady = true }

            // The bar is part of the desktop the Stage Editor shrinks into its card.
            StageFramedLayer {
                id: frameLayer
                interactive: false
                screenName: barWindow.screen?.name ?? ""
                edge: barWindow.barPosition
                restThickness: barWindow.surfaceThickness
                screenWidth: barWindow.screen?.width ?? 1920
                screenHeight: barWindow.screen?.height ?? 1080
                originX: barWindow.anchors.left ? barWindow.margins.left : screenWidth - barWindow.width - barWindow.margins.right
                originY: barWindow.anchors.top ? barWindow.margins.top : screenHeight - barWindow.height - barWindow.margins.bottom

                SideBar {
                    id: verticalWrapper
                    barWindow: barWindow
                    property real hideOffsetX: {
                        if (!barWindow || barWindow.isRevealed) return 0;
                        let offset = barWindow.barHeight + barWindow.edgePadding + barWindow.s(10);
                        return barWindow.barPosition === "right" ? offset : -offset;
                    }
                    transform: Translate {
                        x: verticalWrapper.hideOffsetX
                        Behavior on x { NumberAnimation { duration: 300; easing.type: Easing.OutQuint } }
                    }
                }

                TopBar {
                    id: contentWrapper
                    barWindow: barWindow
                }
            }
        }
    }
}
