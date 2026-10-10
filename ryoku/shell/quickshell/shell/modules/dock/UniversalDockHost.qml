import QtQuick
import shell.services
import "../bar/barstyles/python/dock" as PythonDock
import inir.modules.iris.dock as ShimaDock

Item {
    id: root

    required property var screen
    property bool surfaceVisible: true
    readonly property string design: Dock.design
    readonly property var item: root.design === "ryoku" ? designLoader.item?.surface : null

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
            }
        }
    }
}
