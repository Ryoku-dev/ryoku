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
    // the reader wants the newest lines: kept until they scroll away, back when they return to the end
    property bool tail: true

    onActiveChanged: {
        if (drawer.active) {
            drawer.tail = true;
            Qt.callLater(flick.toEnd);
        }
    }
    onPathChanged: {
        drawer.tailText = "";
        drawer.tail = true;
    }

    FileView {
        id: logFile
        path: drawer.active ? drawer.path : ""
        watchChanges: true
        atomicWrites: false
        onFileChanged: logFile.reload()
        // a read that lands while the run truncates or rewrites the file keeps the last good text
        onLoaded: {
            var text = logFile.text();
            if (text === "" && drawer.following)
                return;
            drawer.tailText = text.split("\n").slice(-400).join("\n");
        }
        onLoadFailed: {
            if (!drawer.following)
                drawer.tailText = "";
        }
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

        property bool pinning: false
        // the flickable re-seats contentY as its size changes; only a move at a steady size is the reader's
        property string seenSize: ""
        function sizeSteady() {
            var now = contentHeight + ":" + height;
            var steady = now === flick.seenSize;
            flick.seenSize = now;
            return steady;
        }
        function atEnd() {
            return contentHeight <= height || contentY >= contentHeight - height - 2;
        }
        function toEnd() {
            flick.pinning = true;
            contentY = Math.max(0, contentHeight - height);
            flick.pinning = false;
        }
        onContentYChanged: {
            if (!flick.pinning && flick.sizeSteady())
                drawer.tail = flick.atEnd();
        }
        onContentHeightChanged: {
            flick.sizeSteady();
            if (drawer.tail)
                flick.toEnd();
        }
        onHeightChanged: {
            flick.sizeSteady();
            if (drawer.tail)
                flick.toEnd();
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
