import QtQuick
import Quickshell.Io
import Lithic
import Ryoku.Ui.Singletons

// the update's whole raw log: package installs, builds and the health check
// the run view leaves out; read only while open, following the tail during a run
Item {
    id: drawer

    property string path: ""
    property bool active: false
    property bool following: false
    property string tailText: ""

    FileView {
        id: logFile
        path: drawer.active ? drawer.path : ""
        watchChanges: true
        atomicWrites: false
        onFileChanged: logFile.reload()
        onLoaded: {
            // follow the tail unless the reader scrolled up to read
            var follow = flick.atEnd;
            drawer.tailText = logFile.text().split("\n").slice(-400).join("\n");
            if (follow)
                Qt.callLater(flick.toEnd);
        }
        onLoadFailed: drawer.tailText = ""
    }

    // appends are not reported on every filesystem
    Timer {
        interval: 1000
        running: drawer.active && drawer.following
        repeat: true
        onTriggered: logFile.reload()
    }

    CutRect {
        anchors.fill: parent
        cut: LithicTokens.cut.small
        fill: Lithic.field
    }

    Flickable {
        id: flick
        anchors.fill: parent
        anchors.margins: LithicTokens.space.s3
        contentWidth: width
        contentHeight: body.implicitHeight
        clip: true
        boundsBehavior: Flickable.StopAtBounds

        readonly property bool atEnd: contentHeight <= height || contentY >= contentHeight - height - 24
        function toEnd() {
            contentY = Math.max(0, contentHeight - height);
        }

        Label {
            id: body
            width: flick.width - LithicTokens.space.s3
            text: drawer.tailText !== "" ? drawer.tailText : I18n.tr("Nothing in the log yet.")
            typeRole: "monoSm"
            inkRole: drawer.tailText !== "" ? "fg2" : "fg3"
            wrapMode: Text.WrapAnywhere
            elide: Text.ElideNone
            textFormat: Text.PlainText
            lineHeightMode: Text.ProportionalHeight
            lineHeight: 1.2
        }
    }

    ScrollRail {
        flickable: flick
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.right: parent.right
    }
}
