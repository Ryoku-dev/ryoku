import QtQuick
import shell.services
import "../stage/Singletons" as StageCfg
import stage as Stage
import "../bar/barstyles/python/dock" as PythonDock
import inir.modules.iris.dock as ShimaDock
Item {
    id: root

    required property var screen
    property bool surfaceVisible: true
    readonly property string design: Dock.design
    // A dock installed from RyoStore runs the Dock.qml of its own view. A
    // design the Store has not installed draws the Ryoku dock, and so does one
    // whose view failed to load, until an update names another view.
    readonly property string storeUrl: Dock.storeDesign ? DockProducts.urlFor(root.design) : ""
    property string failedUrl: ""
    readonly property bool storeRuns: root.storeUrl !== "" && root.storeUrl !== root.failedUrl
    readonly property var item: designLoader.item?.surface ?? null
    // The Stage Editor frames this monitor: every design stays mounted,
    // revealed and drawn inside the editor's card, and publishes the edge it
    // occupies for layouts that keep clear of it (desktop icons). The dock
    // catalogue edits the live surface, so hiding it while that page is open
    // edits an invisible thing.
    readonly property bool stageEditing: StageCfg.StageSession.onMonitor(
        root.screen ? root.screen.name : "")
        || (Stage.GlobalStates.editModeMonitor === (root.screen?.name ?? "")
            && Stage.GlobalStates.editProgress > 0)

    width: 0
    height: 0

    Loader {
        id: designLoader
        // A saved store design waits for the Store's index rather than drawing
        // the Ryoku dock first and swapping a moment later.
        active: root.surfaceVisible && Dock.cfg("enabled", false) && root.design !== "none" && root.design !== "ii"
            && (!Dock.storeDesign || DockProducts.ready)
        asynchronous: true
        sourceComponent: root.storeRuns ? storeDesign
            : root.design === "python" ? pythonDesign
            : root.design === "shima" ? shimaDesign : ryokuDesign
    }

    Component {
        id: storeDesign
        Loader {
            id: storeLoader
            readonly property string url: root.storeUrl
            asynchronous: true
            onUrlChanged: storeLoader.load()
            Component.onCompleted: storeLoader.load()
            function load() {
                storeLoader.setSource(storeLoader.url, {
                    screen: root.screen,
                    surfaceVisible: root.surfaceVisible,
                    stageEditing: root.stageEditing
                });
            }
            onStatusChanged: {
                if (storeLoader.status !== Loader.Error)
                    return;
                console.warn("dock: the store dock " + root.design + " did not load; drawing the Ryoku dock");
                root.failedUrl = storeLoader.url;
            }
            Binding {
                when: storeLoader.item !== null
                target: storeLoader.item
                property: "surfaceVisible"
                value: root.surfaceVisible
            }
            Binding {
                when: storeLoader.item !== null
                target: storeLoader.item
                property: "stageEditing"
                value: root.stageEditing
            }
        }
    }

    Component {
        id: ryokuDesign
        Item {
            property alias surface: dockSurface
            width: 0
            height: 0
            DockSurface {
                id: dockSurface
                screen: root.screen
                stageEditing: root.stageEditing
                visible: root.surfaceVisible && Dock.cfg("enabled", false)
            }
        }
    }

    Component {
        id: pythonDesign
        Item {
            width: 0
            height: 0
            PythonDock.Dock {
                targetScreen: root.screen
                surfaceVisible: root.surfaceVisible
                stageEditing: root.stageEditing
            }
        }
    }

    Component {
        id: shimaDesign
        Item {
            width: 0
            height: 0
            ShimaDock.IrisDockSurface {
                screen: root.screen
                surfaceVisible: root.surfaceVisible
                stageEditing: root.stageEditing
            }
        }
    }
}
