import QtQuick
import shell.services
import "../stage/Singletons" as StageCfg
import "../bar/barstyles/python/dock" as PythonDock
import inir.modules.iris.dock as ShimaDock
Item {
    id: root

    required property var screen
    property bool surfaceVisible: true
    readonly property string design: Dock.design
    readonly property var item: root.design === "ryoku" ? designLoader.item?.surface : null
    // The Stage Editor frames this monitor: every design stays mounted,
    // revealed and raised, and publishes the edge it occupies so the mode's
    // viewport shrinks around it. The dock catalogue edits the live surface,
    // so hiding it while that page is open edits an invisible thing.
    readonly property bool stageEditing: StageCfg.StageSession.onMonitor(
        root.screen ? root.screen.name : "")

    width: 0
    height: 0

    Loader {
        id: designLoader
        active: root.surfaceVisible && Dock.cfg("enabled", false) && root.design !== "none"
        asynchronous: true
        sourceComponent: root.design === "python" ? pythonDesign
            : root.design === "shima" ? shimaDesign : ryokuDesign
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
