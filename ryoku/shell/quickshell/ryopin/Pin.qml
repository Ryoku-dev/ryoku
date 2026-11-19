import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui.Singletons
import Lithic
import "Singletons"
import "lib/pinlayout.js" as PinLayout

FocusScope {
    id: pin
    required property var entry
    required property var output
    required property var controller
    property bool shown: true
    property bool tracing: entry.tracing || false
    property bool blinked: entry.blinked || false
    property real alpha: entry.alpha === undefined ? 1 : entry.alpha
    property int frameW: 48
    property int frameH: 48
    property bool seeded: false
    property bool notified: false
    readonly property bool manipulating: gesture.pressed || exporting
    property bool exporting: false
    readonly property int screenW: output ? output.width : 0
    readonly property int screenH: output ? output.height : 0
    readonly property bool measured: shot.sourceSize.width > 0 && shot.sourceSize.height > 0
    property string readout: ""
    signal closed(string path)
    signal painted()
    signal activated()

    width: frameW
    height: frameH
    visible: shown
    onMeasuredChanged: if (measured && !seeded) seedPosition()
    Component.onCompleted: if (measured && !seeded) seedPosition()
    onXChanged: remember()
    onYChanged: remember()
    onFrameWChanged: remember()
    onFrameHChanged: remember()
    onTracingChanged: {
        entry.tracing = tracing;
        if (tracing) controller.tracingPath = entry.path;
        else blinked = false;
    }
    onBlinkedChanged: entry.blinked = blinked
    onAlphaChanged: entry.alpha = alpha
    onScreenWChanged: if (seeded) reclamp()
    onScreenHChanged: if (seeded) reclamp()

    property Region inputRegion: Region {
        x: pin.x
        y: pin.y
        width: pin.shown && !pin.tracing ? pin.frameW : 0
        height: pin.shown && !pin.tracing ? pin.frameH : 0
        Region {
            x: pin.x + tools.x; y: pin.y + tools.y
            width: pin.shown && !pin.tracing ? tools.width : 0
            height: pin.shown && !pin.tracing ? tools.height : 0
        }
        Region {
            x: pin.x + traceHandle.x; y: pin.y + traceHandle.y
            width: pin.shown && pin.tracing ? traceHandle.width : 0
            height: pin.shown && pin.tracing ? traceHandle.height : 0
        }
    }

    function remember() {
        if (seeded) entry.geometry = { x: x, y: y, w: frameW, h: frameH, output: output.name };
    }
    function seedPosition() {
        var geometry = entry.geometry;
        var size = PinLayout.fitSize(shot.sourceSize.width, shot.sourceSize.height, screenW, screenH);
        if (geometry && geometry.output === output.name) {
            frameW = geometry.w;
            frameH = geometry.h;
            x = geometry.x;
            y = geometry.y;
        } else {
            frameW = size.w;
            frameH = size.h;
            var position = PinLayout.initialPosition(entry.placement, entry.index, size, output);
            x = position.x;
            y = position.y;
        }
        seeded = true;
        remember();
    }
    function reclamp() {
        var position = PinLayout.clamp({x: x, y: y}, {w: frameW, h: frameH}, screenW, screenH);
        x = position.x;
        y = position.y;
    }
    function zoom(factor) {
        var geometry = PinLayout.zoom({x: x, y: y, w: frameW, h: frameH}, factor, screenW, screenH);
        frameW = geometry.w;
        frameH = geometry.h;
        x = geometry.x;
        y = geometry.y;
        showReadout(Math.round(frameW / Math.max(1, shot.sourceSize.width) * 100));
    }
    function setAlpha(value) {
        alpha = Math.max(0.05, Math.min(1, value));
        showReadout(Math.round(alpha * 100));
    }
    function showReadout(percent) {
        readout = I18n.tr("%1%").arg(percent);
        readoutTimer.restart();
    }
    function toggleTracing() { tracing = !tracing; activated(); }
    Connections {
        target: pin.controller
        function onBlinkRequested(path) { if (pin.tracing && path === pin.entry.path) pin.toggleBlink(); }
        function onExitTracingRequested(path) { if (pin.tracing && path === pin.entry.path) pin.toggleTracing(); }
    }
    function toggleBlink() { blinked = !blinked; }
    function editShot() {
        Spawn.run(["env", "RYOSHOT_OPEN=" + entry.path, "ryoku-cmd-ryoshot"]);
        closed(entry.path);
    }
    function copyShot() { activated(); Bus.run("clip-copy image/png " + entry.path); }
    function copyLink() { activated(); linkFile.setText(String(entry.path)); }
    function exportImage() {
        if (exporting) return;
        activated();
        exporting = true;
        try {
            var uri = "file://" + entry.path.split("/").map(encodeURIComponent).join("/");
            dragProxy.Drag.mimeData = { "text/uri-list": uri + "\r\n", "image/png": imageData.data() };
            dragProxy.Drag.imageSource = uri;
            dragProxy.Drag.active = true;
            dragProxy.Drag.startDrag(Qt.CopyAction);
        } finally {
            dragProxy.Drag.active = false;
            dragProxy.Drag.mimeData = {};
            dragProxy.Drag.imageSource = "";
            exporting = false;
        }
    }
    Keys.onPressed: event => {
        if (tracing || event.isAutoRepeat) return;
        if (event.key >= Qt.Key_0 && event.key <= Qt.Key_9)
            setAlpha(event.key === Qt.Key_0 ? 1 : (event.key - Qt.Key_0) / 10);
        else if (event.key === Qt.Key_Plus || event.key === Qt.Key_Equal) zoom(1.25);
        else if (event.key === Qt.Key_Minus) zoom(0.75);
        else if (event.key === Qt.Key_T) toggleTracing();
        else if (event.key === Qt.Key_B) toggleBlink();
        else if (event.key === Qt.Key_H) controller.hidden = true;
        else if (event.key === Qt.Key_F) controller.focusOne(entry.path);
        else if (event.key === Qt.Key_Escape || event.key === Qt.Key_Delete) closed(entry.path);
        else return;
        event.accepted = true;
    }

    FileView {
        id: imageData
        path: pin.entry.path
        preload: false
        blockLoading: true
    }
    FileView {
        id: linkFile
        path: (Quickshell.env("XDG_RUNTIME_DIR") || "/tmp") + "/ryoku/pins/.path.txt"
        preload: false
        printErrors: false
        onSaved: Bus.run("clip-copy text/plain " + linkFile.path)
    }
    Item {
        id: dragProxy
        visible: false
        Drag.dragType: Drag.None
        Drag.supportedActions: Qt.CopyAction
        Drag.proposedAction: Qt.CopyAction
        Drag.imageSourceSize: Qt.size(96, 96)
    }
    CutRect {
        id: frame
        anchors.fill: parent
        design: Tokens.design
        cut: Tokens.radius
        fill: Theme.panelSolid
        strokeColor: Theme.hair
        strokeWidth: 1
        opacity: pin.blinked ? 0 : pin.alpha
        Behavior on opacity { NumberAnimation { duration: Theme.snap } }
        Image {
            id: shot
            anchors.fill: parent
            anchors.margins: 1
            source: "file://" + pin.entry.path
            fillMode: Image.PreserveAspectFit
            smooth: true
            mipmap: true
            onStatusChanged: if (status === Image.Error) console.error("ryopin: cannot load", pin.entry.path)
        }
    }
    MouseArea {
        id: gesture
        anchors.fill: parent
        enabled: !pin.tracing
        acceptedButtons: Qt.LeftButton | Qt.RightButton | Qt.MiddleButton
        preventStealing: true
        cursorShape: pressed ? Qt.ClosedHandCursor : Qt.OpenHandCursor
        property point startPoint
        property real startX: 0
        property real startY: 0
        property real startAlpha: 1
        property int startWidth: 0
        property int startHeight: 0
        property bool opacityGesture: false
        property bool nativeDrag: false
        property bool dragHandled: false
        onPressed: event => {
            pin.activated();
            if (event.button === Qt.MiddleButton) { pin.closed(pin.entry.path); return; }
            startPoint = mapToItem(null, event.x, event.y);
            startX = pin.x; startY = pin.y;
            startWidth = pin.frameW; startHeight = pin.frameH;
            startAlpha = pin.alpha;
            opacityGesture = event.button === Qt.RightButton && event.x >= pin.width / 2;
            nativeDrag = (event.modifiers & Qt.AltModifier) !== 0;
            dragHandled = false;
        }
        onPositionChanged: event => {
            if (!pressed || dragHandled) return;
            var point = mapToItem(null, event.x, event.y);
            var dx = point.x - startPoint.x;
            var dy = point.y - startPoint.y;
            if (nativeDrag) {
                if (Math.abs(dx) + Math.abs(dy) >= 8) { dragHandled = true; pin.exportImage(); }
            } else if (pressedButtons & Qt.RightButton) {
                if (opacityGesture) pin.setAlpha(startAlpha + (dx - dy) / 250);
                else {
                    var factor = Math.max(0.1, 1 + (dx - dy) / 250);
                    var geometry = PinLayout.zoom({x: startX, y: startY, w: startWidth, h: startHeight}, factor, pin.screenW, pin.screenH);
                    pin.frameW = geometry.w; pin.frameH = geometry.h;
                    pin.x = geometry.x; pin.y = geometry.y;
                    pin.showReadout(Math.round(pin.frameW / Math.max(1, shot.sourceSize.width) * 100));
                }
            } else {
                pin.x = startX + dx; pin.y = startY + dy;
                pin.reclamp();
            }
        }
        onWheel: event => {
            pin.activated();
            var delta = event.angleDelta.y || event.pixelDelta.y;
            if (!delta) return;
            if (event.modifiers & Qt.ControlModifier) pin.setAlpha(pin.alpha + (delta > 0 ? 0.05 : -0.05));
            else pin.zoom(delta > 0 ? 1.1 : 0.9);
            event.accepted = true;
        }
    }
    CutRect {
        id: tools
        x: Math.max(-pin.x, Math.min(0, pin.screenW - pin.x - width))
        y: Math.max(-pin.y, Math.min(pin.frameH + 8, pin.screenH - pin.y - height))
        width: buttons.width + 8
        height: buttons.height + 8
        visible: !pin.tracing
        design: Tokens.design
        cut: Tokens.radius
        fill: Theme.panelSolid
        strokeWidth: 1
        strokeColor: Theme.hair
        Grid {
            id: buttons
            anchors.centerIn: parent
            columns: Math.max(1, Math.min(8, Math.floor((pin.screenW - 16) / 36)))
            spacing: 4
            PinButton { iconName: "open_with"; label: I18n.tr("Drag image to another app"); onDragRequested: pin.exportImage() }
            PinButton { iconName: "edit"; label: I18n.tr("Edit"); onClicked: pin.editShot() }
            PinButton { iconName: "content_copy"; label: I18n.tr("Copy image"); onClicked: pin.copyShot() }
            PinButton { iconName: "link"; label: I18n.tr("Copy path"); onClicked: pin.copyLink() }
            PinButton { iconName: "layers"; label: I18n.tr("Trace (T)"); onClicked: pin.toggleTracing() }
            PinButton { iconName: "visibility_off"; label: I18n.tr("Hide all (H)"); onClicked: pin.controller.hidden = true }
            PinButton { iconName: "center_focus_strong"; label: I18n.tr("Focus this pin (F)"); selected: pin.controller.focusedPin === pin.entry.path; onClicked: { pin.activated(); pin.controller.focusOne(pin.entry.path); } }
            PinButton { iconName: "close"; label: I18n.tr("Close"); onClicked: pin.closed(pin.entry.path) }
        }
    }
    CutRect {
        id: traceHandle
        x: Math.max(-pin.x, Math.min(0, pin.screenW - pin.x - width))
        y: Math.max(-pin.y, Math.min(pin.frameH + 4, pin.screenH - pin.y - height))
        width: traceButtons.width + 8
        height: 40
        visible: pin.tracing
        design: Tokens.design
        cut: Tokens.radius
        fill: Theme.panelSolid
        strokeColor: Theme.hair
        strokeWidth: 1
        Row {
            id: traceButtons
            anchors.centerIn: parent
            spacing: 4
            PinButton { iconName: pin.blinked ? "visibility" : "visibility_off"; label: I18n.tr("Blink pin (B)"); onClicked: pin.toggleBlink() }
            PinButton { iconName: "undo"; label: I18n.tr("Exit tracing"); onClicked: pin.toggleTracing() }
        }
    }
    CutRect {
        anchors.horizontalCenter: parent.horizontalCenter
        y: 8
        width: readoutText.implicitWidth + 16
        height: 28
        visible: opacity > 0
        opacity: readoutTimer.running ? 1 : 0
        design: Tokens.design
        cut: Tokens.radius
        fill: Theme.panelSolid
        strokeColor: Theme.hair
        strokeWidth: 1
        Behavior on opacity { NumberAnimation { duration: Theme.snap } }
        Text { id: readoutText; anchors.centerIn: parent; text: pin.readout; color: Theme.ink; font.family: Theme.mono; font.pixelSize: 11 }
    }
    Timer { id: readoutTimer; interval: 900 }
    FrameAnimation {
        running: pin.visible && pin.measured && !pin.notified
        onTriggered: { pin.notified = true; pin.painted(); }
    }
}
