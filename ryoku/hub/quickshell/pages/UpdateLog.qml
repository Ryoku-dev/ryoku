import QtQuick
import QtQuick.Controls
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons

// The update's full raw log, the Hub's `ryoku update -v`: package transactions,
// builds, and doctor output that the curated run view leaves out.
// It reads the file only while open, and follows the tail as the run writes.
Rectangle {
    id: drawer

    required property string path

    radius: Tokens.radius
    color: Tokens.paperLift
    border.width: Tokens.border
    border.color: Tokens.lineSoft
    clip: true

    property string tailText: ""

    FileView {
        id: logFile
        path: drawer.visible ? drawer.path : ""
        watchChanges: true
        atomicWrites: false
        onFileChanged: logFile.reload()
        onLoaded: {
            // follow the tail unless the reader scrolled up to read
            const follow = flick.atEnd;
            const lines = logFile.text().split("\n");
            drawer.tailText = lines.slice(-400).join("\n");
            if (follow)
                Qt.callLater(flick.toEnd);
        }
    }

    // the log grows by appends the watcher may not report on every filesystem
    Timer {
        interval: 1000
        running: drawer.visible
        repeat: true
        onTriggered: logFile.reload()
    }

    Flickable {
        id: flick
        anchors.fill: parent
        anchors.margins: Tokens.s3
        contentWidth: width
        contentHeight: body.implicitHeight
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        readonly property bool atEnd: contentHeight <= height || contentY >= contentHeight - height - 24
        function toEnd() { contentY = Math.max(0, contentHeight - height); }

        Text {
            id: body
            width: flick.width - Tokens.s3
            text: drawer.tailText !== "" ? drawer.tailText : I18n.tr("Nothing in the log yet.")
            color: Tokens.inkDim; font.family: Tokens.mono; font.pixelSize: Tokens.fMicro
            lineHeight: 1.25
            wrapMode: Text.WrapAnywhere
            textFormat: Text.PlainText
        }
    }
}
