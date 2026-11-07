pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Wayland
import Ryoku.Ui.Singletons
import stage.services as StageServices
import stage as Stage
import shell.services
import "ii" as Ii
import "ii/utilities/DockUtilityCatalog.js" as DockUtilityCatalog

/**
 * The ii dock design's global host.
 *
 * The port's Dock.qml is a Scope: it repeats itself across the screens it is
 * told to cover, so this host mounts it once for the whole session, gated on
 * the shared store selecting the "ii" design. The same gate owns the rest of
 * the design's bundle: the live-draw overlay per screen, the alarm ringing
 * popup and the clock app window. Its settings are the Stage Editor's dock
 * pages, which reach the design's stateful services through `actions`.
 */
Scope {
    id: root

    readonly property bool active: Dock.design === "ii" && Dock.cfg("enabled", false)

    // What the Stage Editor's dock pages cannot do through the shared store:
    // the saved layouts, the folders and the utility tiles, which the
    // design's own services and catalogue keep in step with its order. Built
    // only while the design is up, so another design never instantiates this
    // one's services.
    LazyLoader {
        id: actions
        active: root.active

        QtObject {
            // The receiver is session-owned, not tile-owned: incoming shares
            // must be accepted even when the Send utility is not on the dock.
            readonly property var localSendService: StageServices.LocalSend
            readonly property var presets: Ii.DockPresets.presetsList ?? []
            readonly property int activePreset: {
                Ii.Config.options.dock.pinnedApps;
                Ii.Config.options.dock.order;
                Ii.Config.options.dock.enableMediaWidget;
                Ii.Config.options.dock.enableWeatherWidget;
                Ii.Config.options.dock.enableSportsWidget;
                Ii.Config.options.dock.enableLivePreviewWidget;
                Ii.DockPresets.presetsList;
                return Ii.DockPresets.findMatchingPresetIndex();
            }
            function applyPreset(index: int): void {
                const preset = Ii.DockPresets.presetsList[index];
                if (preset)
                    Ii.DockPresets.applyPreset(preset);
            }
            function savePreset(name: string): void {
                Ii.DockPresets.saveCurrentAsPreset(name);
            }
            function deletePreset(index: int): void {
                Ii.DockPresets.deletePreset(index);
            }
            function addFolder(path: string): void {
                Ii.TaskbarApps.addPinnedFile(path);
            }
            function removeFolder(path: string): void {
                Ii.TaskbarApps.removePinnedFile(path);
            }
            function moveFolder(from: int, to: int): void {
                Ii.TaskbarApps.reorderPinnedFileByIndex(from, to);
            }
            readonly property var utilityGroups: DockUtilityCatalog.groups
            readonly property var utilityKinds: DockUtilityCatalog.kinds
            readonly property var utilities: DockUtilityCatalog.normalize(Ii.Config.options.dock.utilityWidgets ?? [])
            function setUtility(kind: string, on: bool): void {
                const current = Ii.Config.options.dock.utilityWidgets ?? [];
                const key = DockUtilityCatalog.orderKey(kind);
                const order = Array.from(Ii.Config.options.dock.order ?? []);
                Dock.setDesignCfg("ii", "utilityWidgets", on
                    ? DockUtilityCatalog.withKind(current, kind, false)
                    : DockUtilityCatalog.withoutKind(current, kind));
                const at = order.indexOf(key);
                if (on && at < 0) {
                    let index = order.length;
                    while (index > 0 && ["trash", "overview", "pin"].indexOf(order[index - 1]) >= 0)
                        index--;
                    order.splice(index, 0, key);
                    Dock.setDesignCfg("ii", "order", order);
                } else if (!on && at >= 0) {
                    order.splice(at, 1);
                    Dock.setDesignCfg("ii", "order", order);
                }
            }
        }
    }

    Binding {
        target: Dock
        property: "designActions"
        value: actions.item
    }
    QtObject {
        id: mediaPopupOwner
        readonly property bool isClosing: false
        property Item anchorItem: null
        function close(): void {
            Ii.GlobalStates.mediaControlsOpen = false;
        }
        function closeImmediately(): void {
            Ii.GlobalStates.mediaControlsOpen = false;
        }
    }

    Connections {
        target: Ii.GlobalStates
        function onMediaControlsOpenChanged() {
            if (Ii.GlobalStates.mediaControlsOpen)
                Stage.GlobalStates.claimIiDockPopup(mediaPopupOwner);
            else
                Stage.GlobalStates.releaseIiDockPopup(mediaPopupOwner);
        }
    }

    Component.onCompleted: {
        if (Ii.GlobalStates.mediaControlsOpen)
            Stage.GlobalStates.claimIiDockPopup(mediaPopupOwner);
    }
    Component.onDestruction: Stage.GlobalStates.releaseIiDockPopup(mediaPopupOwner)


    Loader {
        active: root.active
        asynchronous: true
        sourceComponent: Component {
            Ii.Dock {}
        }
    }

    // A compositor without the focus-grab protocol still needs true modal
    // dismissal. This catcher sits below the dock's overlay layer, so its
    // popup remains interactive while presses on apps or the desktop close it.
    Variants {
        model: root.active && Stage.GlobalStates.iiDockPopup !== null
            && Wm.caps.focusGrab !== true ? Quickshell.screens : []

        PanelWindow {
            required property var modelData
            screen: modelData
            visible: true
            color: "transparent"
            exclusionMode: ExclusionMode.Ignore
            WlrLayershell.namespace: "quickshell:dock-dismiss"
            WlrLayershell.layer: WlrLayer.Top
            WlrLayershell.keyboardFocus: WlrKeyboardFocus.None
            anchors { top: true; bottom: true; left: true; right: true }

            MouseArea {
                anchors.fill: parent
                onPressed: Qt.callLater(() => Stage.GlobalStates.dismissIiDockPopup())
            }
        }
    }

    // Live draw: one window per screen, mounted while the feature is on.
    Variants {
        model: root.active ? Quickshell.screens : []

        Loader {
            required property var modelData
            active: Ii.LiveDraw.enabled
            sourceComponent: Component {
                Ii.LiveDrawWindow {
                }
            }
            onLoaded: if (item)
                item.screen = modelData
        }
    }

    Loader {
        active: root.active
        sourceComponent: Component {
            Ii.AlarmRingingPopup {}
        }
    }


    Loader {
        active: root.active
        asynchronous: true
        sourceComponent: Component {
            DockMediaPopup {}
        }
    }
}
