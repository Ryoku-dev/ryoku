pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Widgets
import caelestia.utils

Item {
    id: root

    readonly property int status: loader.item?.status ?? Image.Null // qmllint disable missing-property
    readonly property real actualSize: Math.min(width, height)
    property real implicitSize
    property url source

    implicitWidth: implicitSize
    implicitHeight: implicitSize

    Loader {
        id: loader

        // The backing image requests its source when it completes, before anchors and
        // layout have resolved, so a size taken from laid-out geometry is still 0 there
        // and the icon provider answers with its 2px fallback. Wait for a real size.
        active: root.source.toString() !== "" && root.width > 0 && root.height > 0
        asynchronous: true
        anchors.fill: parent
        sourceComponent: root.source.toString().startsWith("image://icon/") ? iconImage : cachingImage
    }

    Component {
        id: cachingImage

        CachingImage {
            path: Paths.toLocalFile(root.source)
            fillMode: Image.PreserveAspectFit
        }
    }

    Component {
        id: iconImage

        IconImage {
            source: root.source
            asynchronous: true
        }
    }
}
