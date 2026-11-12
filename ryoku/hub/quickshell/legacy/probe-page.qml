import Quickshell
import QtQuick
import "pages"
ShellRoot {
    FloatingWindow {
        BarStudioPage { anchors.fill: parent; hub: QtObject {} }
    }
}
