import QtQuick
import QtQuick.Window
import Lithic

Window {
    id: window

    readonly property string requestedPage: Lithic.environment("LITHIC_PAGE").toLowerCase()
    readonly property string pageName: ["foundation", "controls", "lists", "overlays"].indexOf(requestedPage) >= 0
                                       ? requestedPage
                                       : "foundation"
    readonly property string pageFile: pageName.charAt(0).toUpperCase() + pageName.slice(1)
    readonly property string capturePath: Lithic.environment("LITHIC_CAPTURE_PATH")
    readonly property int pageHeight: pageLoader.item
                                      ? Math.max(900, Math.ceil(pageLoader.item.implicitHeight))
                                      : 900

    width: 1440
    height: pageHeight
    visible: true
    color: Lithic.ground
    title: "Lithic gallery"

    Item {
        id: captureSurface
        anchors.fill: parent

        Rectangle {
            anchors.fill: parent
            color: Lithic.ground
        }

        Loader {
            id: pageLoader
            anchors.fill: parent
            asynchronous: false
            source: "pages/" + window.pageFile + ".qml"
            onStatusChanged: {
                if (status !== Loader.Error)
                    return;
                console.error("Lithic gallery page " + window.pageName + " failed to load");
                Qt.exit(2);
            }
        }
    }

    Timer {
        interval: 120
        repeat: false
        running: window.capturePath.length > 0
                 && Lithic.paletteReady
                 && pageLoader.status === Loader.Ready
        onTriggered: captureSurface.grabToImage(function(result) {
            if (!result.saveToFile(window.capturePath)) {
                Qt.exit(1);
                return;
            }
            Qt.quit();
        }, Qt.size(window.width, window.height))
    }
}
