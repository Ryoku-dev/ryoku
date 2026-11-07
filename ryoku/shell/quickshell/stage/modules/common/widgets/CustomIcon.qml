import QtQuick
import Quickshell
import Quickshell.Widgets
import Qt5Compat.GraphicalEffects

Item {
    id: root
    
    property bool colorize: false
    property color color
    property string source: ""
    property string iconFolder: Qt.resolvedUrl("../../../assets/icons")
    width: 30
    height: 30
    
    IconImage {
        id: iconImage
        anchors.fill: parent
        source: {
            const value = String(root.source ?? "");
            if (value.length === 0)
                return "";
            if (value.startsWith("/") || value.includes("://") || value.startsWith("qrc:"))
                return value;
            const filename = value.includes(".") ? value : value + ".svg";
            return root.iconFolder + "/" + filename;
        }
        implicitSize: root.height
    }

    Loader {
        active: root.colorize
        anchors.fill: iconImage
        sourceComponent: ColorOverlay {
            source: iconImage
            color: root.color
        }
    }
}
