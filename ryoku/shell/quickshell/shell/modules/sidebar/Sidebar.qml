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
    readonly property var clearances: SidebarState.railClearances(screen)
    readonly property real gap: SidebarState.edgeGap * s
    readonly property real screenWidth: screen && screen.width > 0 ? screen.width : 1280
    readonly property real screenHeight: screen && screen.height > 0 ? screen.height : 800
    readonly property real availableWidth: Math.max(1, screenWidth - clearances.left - clearances.right - gap * 2)
    readonly property real availableHeight: Math.max(1, screenHeight - clearances.top - clearances.bottom - gap * 2)
    readonly property string selectedDesign: Designs.sidebar
    readonly property string storeUrl: Designs.sidebarReady ? Designs.sidebarUrl : ""
    property string failedUrl: ""
    readonly property bool storeRuns: selectedDesign !== Designs.shipped
        && Designs.sidebarReady && storeUrl !== "" && storeUrl !== failedUrl
    readonly property var storeItem: storeLoader.status === Loader.Ready ? storeLoader.item : null
    readonly property string storePlacement: storeRuns && storeItem
        ? normalizedPlacement(storeItem.placement, "") : ""
    readonly property string placement: storePlacement !== ""
        ? storePlacement : normalizedPlacement(SidebarState.position, "top-left")
    readonly property bool sidePlacement: placement === "left" || placement === "right"
    readonly property real stockWidth: Math.min(availableWidth, 688 * s)
    readonly property real stockFallbackHeight: (Tokens.s4 * 2 + Tokens.ctlH + Tokens.s1
        + Tokens.s3 + Tokens.cellH * 4) * s
    readonly property real stockHeight: Math.min(availableHeight, shippedLoader.item
        ? shippedLoader.item.fittedHeight : stockFallbackHeight)
    readonly property real wantedWidth: storeRuns
        ? Math.min(availableWidth, usableSize(storeItem ? storeItem.implicitWidth : 0, stockWidth))
        : stockWidth
    readonly property real wantedHeight: storeRuns
        ? (sidePlacement ? availableHeight
            : Math.min(availableHeight, usableSize(storeItem ? storeItem.implicitHeight : 0, stockHeight)))
        : stockHeight
    readonly property real roomX: clearances.left + gap
    readonly property real roomY: clearances.top + gap
    readonly property real panelX: placement === "top-right" || placement === "right"
        ? roomX + availableWidth - wantedWidth
        : placement === "top-center" || placement === "center"
            ? roomX + (availableWidth - wantedWidth) / 2 : roomX
    readonly property real panelY: placement === "center"
        ? roomY + (availableHeight - wantedHeight) / 2 : roomY
    readonly property int panelTransformOrigin: placement === "top-right" ? Item.TopRight
        : placement === "top-center" ? Item.Top
        : placement === "left" ? Item.Left
        : placement === "right" ? Item.Right
        : placement === "center" ? Item.Center : Item.TopLeft
    readonly property real panelSlideX: placement === "left" ? -Tokens.s2 * s
        : placement === "right" ? Tokens.s2 * s : 0
    readonly property real panelSlideY: sidePlacement ? 0 : -Tokens.s2 * s
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

    Behavior on reveal {
        enabled: !Motion.reduce && !Tokens.reduceMotion
        NumberAnimation {
            duration: root.active ? SidebarState.enterDuration : SidebarState.exitDuration
            easing.type: Easing.Bezier
            easing.bezierCurve: root.active ? SidebarState.enterCurve : SidebarState.exitCurve
        }
    }

    // The dismiss surface never animates. Only the small corner buffer redraws.
    // Since it does not redraw, a mask edited after its first frame never reaches
    // the compositor, so its hole is the panel's whole column, which stays fixed
    // while an inline drawer or section grows the panel inside it.
    PanelWindow {
        id: dismiss
        screen: root.screen
        visible: root.active
        color: "transparent"
        exclusionMode: ExclusionMode.Ignore
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
                x: root.panelX
                y: root.roomY
                width: root.wantedWidth
                height: root.availableHeight
            }
            Region { intersection: Intersection.Subtract; width: dismiss.width; height: root.clearances.top }
            Region { intersection: Intersection.Subtract; y: dismiss.height - root.clearances.bottom; width: dismiss.width; height: root.clearances.bottom }
            Region { intersection: Intersection.Subtract; width: root.clearances.left; height: dismiss.height }
            Region { intersection: Intersection.Subtract; x: dismiss.width - root.clearances.right; width: root.clearances.right; height: dismiss.height }
        }
        MouseArea {
            anchors.fill: parent
            focus: root.active
            Keys.onEscapePressed: SidebarState.close(root.screen)
            acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
            onPressed: Qt.callLater(() => SidebarState.close(root.screen))
        }
    }

    PanelWindow {
        id: panel
        screen: root.screen
        visible: root.active || root.reveal > 0.001
        color: "transparent"
        exclusionMode: ExclusionMode.Ignore
        exclusiveZone: 0
        anchors { top: true; left: true }
        margins.top: root.roomY
        margins.left: root.panelX
        implicitWidth: root.wantedWidth
        implicitHeight: root.availableHeight
        WlrLayershell.layer: WlrLayer.Overlay
        WlrLayershell.namespace: "ryoku-sidebar-left"
        WlrLayershell.keyboardFocus: root.active ? WlrKeyboardFocus.Exclusive : WlrKeyboardFocus.None

        // The dismiss hole spans the whole column, so a press in its clear part
        // above or below the panel has to dismiss here instead.
        MouseArea {
            anchors.fill: parent
            acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
            onPressed: mouse => {
                if (mouse.y >= surface.y && mouse.y < surface.y + surface.height) {
                    mouse.accepted = false;
                    return;
                }
                Qt.callLater(() => SidebarState.close(root.screen));
            }
        }

        FocusScope {
            id: surface
            y: root.panelY - root.roomY
            width: parent.width
            height: root.wantedHeight
            opacity: root.reveal
            scale: 0.985 + root.reveal * 0.015
            transformOrigin: root.panelTransformOrigin
            transform: Translate {
                x: (1 - root.reveal) * root.panelSlideX
                y: (1 - root.reveal) * root.panelSlideY
            }
            focus: root.active
            Keys.onEscapePressed: SidebarState.close(root.screen)
            onFocusChanged: if (focus) Qt.callLater(() => surface.forceActiveFocus())

            Loader {
                id: shippedLoader
                anchors.fill: parent
                active: !root.storeRuns
                sourceComponent: Item {
                    readonly property real fittedHeight: chrome.fittedHeight

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
