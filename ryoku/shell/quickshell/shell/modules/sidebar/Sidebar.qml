pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import Ryoku.Ui.Singletons
import shell.services

Scope {
    id: root

    required property var screen
    property bool active: false
    readonly property real s: Tokens.uiScaleFor(screen ? screen.name : "")
    readonly property real gap: SidebarState.edgeGap * s
    readonly property real halo: Math.ceil((QsLook.shadowBlur + QsLook.shadowY) * s)
    readonly property real workWidth: dismiss.width > 0 ? dismiss.width
        : screen && screen.width > 0 ? screen.width : 1280
    readonly property real workHeight: dismiss.height > 0 ? dismiss.height
        : screen && screen.height > 0 ? screen.height : 800
    readonly property real availableWidth: Math.max(1, workWidth - gap * 2)
    readonly property real availableHeight: Math.max(1, workHeight - gap * 2)
    readonly property string selectedDesign: Designs.sidebar
    readonly property string storeUrl: Designs.sidebarReady ? Designs.sidebarUrl : ""
    property string failedUrl: ""
    readonly property bool storeRuns: selectedDesign !== Designs.shipped
        && Designs.sidebarReady && storeUrl !== "" && storeUrl !== failedUrl
    readonly property var storeItem: storeLoader.status === Loader.Ready ? storeLoader.item : null
    readonly property string storePlacement: storeRuns && storeItem
        ? normalizedPlacement(storeItem.placement, "") : ""
    readonly property string placement: storePlacement !== ""
        ? storePlacement : normalizedPlacement(SidebarState.position, "top-right")
    readonly property bool sidePlacement: placement === "left" || placement === "right"
    readonly property bool leftEdge: placement === "top-left" || placement === "left"
    readonly property bool rightEdge: placement === "top-right" || placement === "right"
    readonly property real stockWidth: Math.min(availableWidth, Math.round(QsLook.width * s))
    readonly property real stockHeight: Math.min(availableHeight, shippedLoader.item
        ? shippedLoader.item.fittedHeight : Math.round(320 * s))
    readonly property real wantedWidth: storeRuns
        ? Math.min(availableWidth, usableSize(storeItem ? storeItem.implicitWidth : 0, stockWidth))
        : stockWidth
    readonly property real wantedHeight: storeRuns
        ? (sidePlacement ? availableHeight
            : Math.min(availableHeight, usableSize(storeItem ? storeItem.implicitHeight : 0, stockHeight)))
        : stockHeight
    // niri clamps negative layer margins so the surface stays flush and the card moves inside it
    readonly property bool anchoredSide: leftEdge || rightEdge
    readonly property real columnWidth: wantedWidth + (anchoredSide ? gap + halo : halo * 2)
    readonly property real columnX: leftEdge ? 0 : rightEdge ? workWidth - columnWidth
        : (workWidth - columnWidth) / 2
    readonly property real cardX: leftEdge ? gap : halo
    readonly property real cardY: placement === "center"
        ? Math.max(gap, Math.round((workHeight - wantedHeight) / 2)) : gap
    readonly property int origin: placement === "center" ? Item.Center
        : rightEdge ? Item.TopRight : leftEdge ? Item.TopLeft : Item.Top
    property real reveal: active ? 1 : 0

    function normalizedPlacement(value, fallback) {
        const candidate = String(value ?? "");
        return candidate === "top-left" || candidate === "top-center"
            || candidate === "top-right" || candidate === "left"
            || candidate === "right" || candidate === "center"
            ? candidate : fallback;
    }

    function usableSize(value, fallback) {
        const candidate = Number(value);
        return Number.isFinite(candidate) && candidate > 0 ? candidate : fallback;
    }

    function stepBack(): void {
        const shipped = shippedLoader.item;
        if (!root.storeRuns && shipped && shipped.back())
            return;
        SidebarState.close(root.screen);
    }

    Behavior on reveal {
        NumberAnimation {
            duration: QsLook.panelMs(root.active ? QsLook.openMs : QsLook.closeMs)
            easing.type: Easing.BezierSpline
            easing.bezierCurve: root.active ? QsLook.arrive : QsLook.leave
        }
    }

    // a surface that never redraws keeps its first input mask so a moved hole needs one frame
    property bool holeNudge: false
    onColumnXChanged: root.holeNudge = !root.holeNudge
    onColumnWidthChanged: root.holeNudge = !root.holeNudge
    onAvailableHeightChanged: root.holeNudge = !root.holeNudge

    PanelWindow {
        id: dismiss
        screen: root.screen
        visible: root.active
        color: "transparent"
        exclusionMode: ExclusionMode.Normal
        exclusiveZone: 0
        anchors { top: true; bottom: true; left: true; right: true }
        WlrLayershell.layer: WlrLayer.Overlay
        WlrLayershell.namespace: "ryoku-corner-dismiss"
        // Both windows must be modal for the click-away surface to receive input.
        WlrLayershell.keyboardFocus: root.active ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None
        mask: Region {
            width: dismiss.width
            height: dismiss.height
            Region {
                intersection: Intersection.Subtract
                x: root.columnX
                width: root.columnWidth
                height: dismiss.height
            }
        }

        Rectangle {
            width: 1
            height: 1
            color: root.holeNudge ? Qt.rgba(0, 0, 0, 0.004) : "transparent"
        }
        MouseArea {
            anchors.fill: parent
            focus: root.active
            Keys.onEscapePressed: root.stepBack()
            acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
            onPressed: Qt.callLater(() => SidebarState.close(root.screen))
        }
    }

    PanelWindow {
        id: panel
        screen: root.screen
        visible: root.active || root.reveal > 0.001
        color: "transparent"
        exclusionMode: ExclusionMode.Normal
        exclusiveZone: 0
        anchors {
            top: true
            bottom: true
            left: root.leftEdge
            right: root.rightEdge
        }
        implicitWidth: root.columnWidth
        WlrLayershell.layer: WlrLayer.Overlay
        WlrLayershell.namespace: "ryoku-sidebar-left"
        WlrLayershell.keyboardFocus: root.active ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None

        MouseArea {
            anchors.fill: parent
            acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
            onPressed: mouse => {
                if (mouse.x >= root.cardX && mouse.x < root.cardX + root.wantedWidth
                        && mouse.y >= root.cardY && mouse.y < root.cardY + root.wantedHeight) {
                    mouse.accepted = false;
                    return;
                }
                Qt.callLater(() => SidebarState.close(root.screen));
            }
        }

        Item {
            id: slip
            x: 0
            y: root.cardY
            width: panel.width
            height: QsLook.lithic && !root.storeRuns
                ? Math.ceil((root.wantedHeight + root.halo) * Math.min(1, root.reveal))
                : panel.height - root.cardY
            clip: QsLook.lithic && !root.storeRuns && root.reveal < 0.999

            FocusScope {
                id: card
                x: root.cardX
                y: 0
                width: root.wantedWidth
                height: root.wantedHeight
                opacity: Math.min(1, root.reveal * (QsLook.lithic ? 3 : QsLook.adwaita ? 1 : 1.6))
                scale: QsLook.material ? 0.92 + 0.08 * root.reveal
                    : QsLook.adwaita ? 0.97 + 0.03 * Math.min(1, root.reveal) : 1
                transformOrigin: root.origin
                transform: Translate {
                    y: (1 - Math.min(1, root.reveal)) * (QsLook.fluent ? -12 : QsLook.adwaita ? -6 : 0) * root.s
                }
                focus: root.active
                Keys.onEscapePressed: root.stepBack()
                onFocusChanged: if (focus) Qt.callLater(() => card.forceActiveFocus())

                Loader {
                    id: shippedLoader
                    anchors.fill: parent
                    active: !root.storeRuns
                    sourceComponent: Item {
                        readonly property real fittedHeight: chrome.fittedHeight

                        function back(): bool {
                            return chrome.back();
                        }

                        SidebarFrame {
                            anchors.fill: parent
                            s: root.s
                        }
                        SidebarChrome {
                            id: chrome
                            anchors.fill: parent
                            screen: root.screen
                            s: root.s
                            active: root.active
                            onCloseRequested: SidebarState.close(root.screen)
                        }
                    }
                }

                Loader {
                    id: storeLoader
                    anchors.fill: parent
                    active: root.storeRuns
                    asynchronous: true
                    visible: root.storeRuns
                    readonly property string url: root.storeUrl

                    onUrlChanged: storeLoader.load()
                    Component.onCompleted: storeLoader.load()

                    function load() {
                        if (storeLoader.url === "") {
                            storeLoader.source = "";
                            return;
                        }
                        storeLoader.setSource(storeLoader.url, {
                            screen: root.screen,
                            us: root.s,
                            active: root.active,
                            availableWidth: root.availableWidth,
                            availableHeight: root.availableHeight
                        });
                    }

                    onStatusChanged: {
                        if (storeLoader.status !== Loader.Error || storeLoader.url === ""
                                || root.failedUrl === storeLoader.url)
                            return;
                        console.warn("sidebar: the store sidebar " + root.selectedDesign
                            + " did not load; drawing the Ryoku sidebar");
                        root.failedUrl = storeLoader.url;
                    }
                }

                Binding {
                    target: storeLoader.item
                    property: "screen"
                    value: root.screen
                    when: storeLoader.status === Loader.Ready && storeLoader.item !== null
                    restoreMode: Binding.RestoreNone
                }
                Binding {
                    target: storeLoader.item
                    property: "us"
                    value: root.s
                    when: storeLoader.status === Loader.Ready && storeLoader.item !== null
                    restoreMode: Binding.RestoreNone
                }
                Binding {
                    target: storeLoader.item
                    property: "active"
                    value: root.active
                    when: storeLoader.status === Loader.Ready && storeLoader.item !== null
                    restoreMode: Binding.RestoreNone
                }
                Binding {
                    target: storeLoader.item
                    property: "availableWidth"
                    value: root.availableWidth
                    when: storeLoader.status === Loader.Ready && storeLoader.item !== null
                    restoreMode: Binding.RestoreNone
                }
                Binding {
                    target: storeLoader.item
                    property: "availableHeight"
                    value: root.availableHeight
                    when: storeLoader.status === Loader.Ready && storeLoader.item !== null
                    restoreMode: Binding.RestoreNone
                }
                Connections {
                    target: storeLoader.status === Loader.Ready ? storeLoader.item : null
                    ignoreUnknownSignals: true
                    function onCloseRequested() {
                        SidebarState.close(root.screen);
                    }
                }
            }
        }
    }
}
