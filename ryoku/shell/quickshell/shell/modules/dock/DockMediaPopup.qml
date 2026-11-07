pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import stage as Stage
import shell.services
import "../bar/popouts" as BarPopouts

Scope {
    id: root

    readonly property var request: Stage.GlobalStates.mediaControlsDockAnchor
    readonly property var targetScreens: {
        const name = String(root.request?.screen ?? "");
        const matches = Quickshell.screens.filter(screen => String(screen?.name ?? "") === name);
        return matches.length > 0 ? matches
            : (Quickshell.screens.length > 0 ? [Quickshell.screens[0]] : []);
    }

    Variants {
        model: Stage.GlobalStates.mediaControlsOpen ? root.targetScreens : []

        PanelWindow {
            id: popupWindow

            required property var modelData
            screen: modelData
            visible: true
            color: "transparent"
            exclusionMode: ExclusionMode.Ignore
            exclusiveZone: 0
            WlrLayershell.namespace: "quickshell:dock-media"
            WlrLayershell.layer: WlrLayer.Overlay
            WlrLayershell.keyboardFocus: WlrKeyboardFocus.Exclusive
            anchors {
                top: true
                bottom: true
                left: true
                right: true
            }

            FocusScope {
                anchors.fill: parent
                focus: true
                Keys.onEscapePressed: Stage.GlobalStates.mediaControlsOpen = false
                Keys.onSpacePressed: Media.toggle()

                MouseArea {
                    anchors.fill: parent
                    onClicked: Stage.GlobalStates.mediaControlsOpen = false
                }

                BarPopouts.MusicPopout {
                    id: card

                    readonly property real gap: 12
                    readonly property real outputX: popupWindow.screen?.x ?? 0
                    readonly property real outputY: popupWindow.screen?.y ?? 0
                    readonly property real anchorX: Number(root.request?.x ?? 0) - outputX
                    readonly property real anchorY: Number(root.request?.y ?? 0) - outputY
                    readonly property real anchorWidth: Number(root.request?.width ?? 0)
                    readonly property real anchorHeight: Number(root.request?.height ?? 0)
                    readonly property string anchorSide: String(root.request?.side ?? "bottom")
                    readonly property real desiredX: anchorSide === "left"
                        ? anchorX + anchorWidth + gap
                        : anchorSide === "right"
                            ? anchorX - width - gap
                            : anchorX + (anchorWidth - width) / 2
                    readonly property real desiredY: anchorSide === "top"
                        ? anchorY + anchorHeight + gap
                        : anchorSide === "bottom"
                            ? anchorY - height - gap
                            : anchorY + (anchorHeight - height) / 2

                    open: true
                    x: Math.max(gap, Math.min(popupWindow.width - width - gap, desiredX))
                    y: Math.max(gap, Math.min(popupWindow.height - height - gap, desiredY))
                }
            }
        }
    }
}
