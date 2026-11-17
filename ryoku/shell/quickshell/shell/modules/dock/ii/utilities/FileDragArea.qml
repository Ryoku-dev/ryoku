import shell.services as ShellIcons
import QtQuick
import Quickshell
import stage.modules.common.functions
import ".."

/**
 * Press and travel: a native drag carrying `paths` as a text/uri-list, the
 * way the dock exports a pinned file (DockContent.moveItemDrag) — the QDrag
 * runs to the drop synchronously, so the area is back to rest when it returns.
 * A press that stays is a click.
 */
MouseArea {
    id: area

    property var paths: []
    property string dragIcon: "text-x-generic"
    signal dragFinished(int action)

    property point pressPoint: Qt.point(0, 0)
    property bool dragging: false
    property bool dragHandled: false

    acceptedButtons: Qt.LeftButton
    preventStealing: true
    cursorShape: Qt.PointingHandCursor
    hoverEnabled: true

    onPressed: event => {
        area.pressPoint = Qt.point(event.x, event.y);
        area.dragging = false;
        area.dragHandled = false;
    }
    onPositionChanged: event => {
        if (!area.pressed || area.dragging || area.dragHandled || area.paths.length === 0)
            return;
        if (Math.abs(event.x - area.pressPoint.x) + Math.abs(event.y - area.pressPoint.y) < 8)
            return;
        area.dragging = true;
        area.dragHandled = true;
        let dropAction = Qt.IgnoreAction;
        GlobalStates.fileDragActive = true;
        try {
            dragProxy.Drag.mimeData = { "text/uri-list": UtilityFiles.uriList(area.paths) };
            dragProxy.Drag.imageSource = ShellIcons.Icons.path(area.dragIcon, "text-x-generic");
            dragProxy.Drag.active = true;
            const result = dragProxy.Drag.startDrag(Qt.CopyAction);
            dropAction = typeof result === "number" ? result : Qt.IgnoreAction;
        } finally {
            dragProxy.Drag.active = false;
            dragProxy.Drag.mimeData = {};
            dragProxy.Drag.imageSource = "";
            GlobalStates.fileDragActive = false;
            area.dragging = false;
        }
        Qt.callLater(function() {
            area.dragFinished(dropAction);
        });
    }
    onReleased: {
        area.dragging = false;
        area.dragHandled = false;
        GlobalStates.fileDragActive = false;
    }
    onCanceled: {
        area.dragging = false;
        GlobalStates.fileDragActive = false;
        area.dragHandled = false;
    }
    Component.onDestruction: {
        if (area.dragging)
            GlobalStates.fileDragActive = false;
    }

    Item {
        id: dragProxy
        visible: false
        Drag.dragType: Drag.None
        Drag.supportedActions: Qt.CopyAction
        Drag.proposedAction: Qt.CopyAction
        Drag.imageSourceSize: Qt.size(48, 48)
    }
}
