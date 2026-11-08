pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Effects
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../desktop/Singletons" as WidgetStore
import "Singletons"

// The widget depth-pick overlay: an edit mode, not a look. While
// Session.depthPick names a desktop widget, that desktop draws the wallpaper's
// depth map tinted over everything; one left-click writes the sampled value
// into the widget's depth, right-click cancels.
Item {
    id: pick

    anchors.fill: parent
    property string wallPath: ""
    property string monitor: ""
    property string placeTool: ""
    property bool cover: true

    readonly property bool on: StageSession.depthPick !== ""
        && StageSession.onMonitor(pick.monitor)
        && StageBackend.spatialFor(pick.wallPath)
    property point cursor: Qt.point(-1, -1)
    property real sample: -1

    opacity: pick.on ? 1 : 0
    visible: opacity > 0
    enabled: pick.on
    Behavior on opacity { NumberAnimation { duration: 160; easing.type: Easing.OutCubic } }

    function receiveSample(text, commitSample) {
        const value = parseFloat(("" + text).trim().replace(/^ok\s+/, ""));
        if (!isFinite(value) || value < 0 || value > 1)
            return;
        pick.sample = value;
        if (commitSample)
            pick.commit(StageSession.depthPick, value);
    }

    // a newer preview drops an older one still in flight, same for commits
    property int sampleSeq: 0
    property int commitSeq: 0
    function request(mx, my, commitSample) {
        if (pick.wallPath === "" || pick.width < 2 || pick.height < 2)
            return;
        pick.cursor = Qt.point(Math.max(0, Math.min(pick.width - 1, mx)),
            Math.max(0, Math.min(pick.height - 1, my)));
        const seq = commitSample ? ++pick.commitSeq : ++pick.sampleSeq;
        Bus.run(["stage", "depth-sample", pick.wallPath,
            "" + (pick.cursor.x / pick.width), "" + (pick.cursor.y / pick.height),
            "" + Math.round(pick.width), "" + Math.round(pick.height),
            pick.cover ? "Cover" : "Fill"].join(" "), function (ok, output) {
            if (ok && seq === (commitSample ? pick.commitSeq : pick.sampleSeq))
                pick.receiveSample(output, commitSample);
        });
    }

    function commit(dest, value) {
        StageSession.depthPick = "";
        if (dest === "")
            return;
        if (dest.indexOf("plugin:") === 0) {
            writer.command = [pick.placeTool, dest.slice(7), "depth", "" + value];
            writer.running = true;
            return;
        }
        WidgetStore.Config.setWidgetDepth(pick.monitor, dest, value);
    }

    Process {
        id: writer
    }

    Image {
        id: map
        anchors.fill: parent
        source: pick.on ? StageBackend.depthUrlFor(pick.wallPath) : ""
        fillMode: Image.PreserveAspectCrop
        asynchronous: true
        cache: false
        visible: false
    }
    MultiEffect {
        anchors.fill: map
        source: map
        visible: pick.opacity > 0
        opacity: 0.62
        saturation: 0
        colorization: 0.9
        colorizationColor: Qt.rgba(1.0, 0.62, 0.2, 1.0)
    }
    Rectangle {
        anchors.fill: parent
        color: Qt.rgba(0, 0, 0, 0.45)
    }

    MouseArea {
        anchors.fill: parent
        acceptedButtons: Qt.LeftButton | Qt.RightButton
        hoverEnabled: true
        cursorShape: Qt.CrossCursor

        onClicked: mouse => {
            if (mouse.button === Qt.RightButton) {
                StageSession.depthPick = "";
                return;
            }
            pick.request(mouse.x, mouse.y, true);
        }
        onPositionChanged: mouse => {
            pick.cursor = Qt.point(mouse.x, mouse.y);
            pick.sample = -1;
            preview.restart();
        }
        onCanceled: preview.stop()
    }
    Timer {
        id: preview
        interval: 130
        onTriggered: {
            if (pick.cursor.x >= 0)
                pick.request(pick.cursor.x, pick.cursor.y, false);
        }
    }

    Rectangle {
        visible: pick.cursor.x >= 0
        x: pick.cursor.x - width / 2
        y: pick.cursor.y - height / 2
        width: 26
        height: 26
        radius: 13
        color: "transparent"
        border.color: Qt.rgba(1, 1, 1, 0.7)
        border.width: 1.5
    }
    Rectangle {
        x: Math.min(pick.width - width - 8, pick.cursor.x + 18)
        y: Math.min(pick.height - height - 8, pick.cursor.y + 18)
        visible: pick.cursor.x >= 0 && pick.sample >= 0
        width: pill.implicitWidth + 14
        height: 22
        radius: Tokens.radius
        color: Qt.rgba(0, 0, 0, 0.72)
        Text {
            id: pill
            anchors.centerIn: parent
            text: pick.sample.toFixed(2)
            color: Tokens.ink
            font.family: Tokens.mono
            font.pixelSize: Tokens.fSmall
            font.weight: Font.DemiBold
        }
    }
}
