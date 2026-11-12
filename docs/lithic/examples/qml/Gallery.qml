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
    readonly property bool capturing: capturePath.length > 0
    readonly property int pageHeight: pageLoader.item
                                      ? Math.max(900, Math.ceil(pageLoader.item.implicitHeight))
                                      : 900

    width: 1440
    height: capturing ? pageHeight : 900
    visible: true
    color: Lithic.ground
    title: "Lithic QML gallery: " + pageName

    Flickable {
        id: scroller
        anchors.fill: parent
        contentWidth: width
        contentHeight: window.pageHeight
        boundsBehavior: Flickable.StopAtBounds
        interactive: !window.capturing

        Item {
            id: captureSurface
            width: scroller.width
            height: window.pageHeight

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
    }

    // The reader's look, as the other galleries take it from flags or the query
    function applyLook() {
        Lithic.contrast = Lithic.environment("LITHIC_CONTRAST") || "standard";
        Lithic.tint = Lithic.environment("LITHIC_TINT") || "off";
    }

    Connections {
        target: Lithic
        function onPaletteReadyChanged() {
            if (Lithic.paletteReady)
                window.applyLook();
        }
    }

    Component.onCompleted: if (Lithic.paletteReady) applyLook()

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
