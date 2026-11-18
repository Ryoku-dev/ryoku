import QtQuick
import Quickshell
import Quickshell.Wayland
import Ryoku.Ui.Singletons
import Lithic
import "Singletons"

PanelWindow {
    id: canvas
    required property var controller
    property var entries: []
    property var inputRegions: []
    property var activePin: null
    property bool dragging: false
    signal painted()

    anchors { top: true; bottom: true; left: true; right: true }
    color: "transparent"
    exclusionMode: ExclusionMode.Ignore
    WlrLayershell.namespace: "ryopin"
    WlrLayershell.layer: WlrLayer.Overlay
    WlrLayershell.keyboardFocus: controller.hidden || !activePin || activePin.tracing
        ? WlrKeyboardFocus.None : WlrKeyboardFocus.OnDemand
    mask: Region {
        width: canvas.dragging ? canvas.width : 0
        height: canvas.dragging ? canvas.height : 0
        regions: canvas.inputRegions
    }
    function refreshRegions() {
        var regions = [restoreRegion];
        var dragging = false;
        for (var i = 0; i < pins.count; ++i) {
            var item = pins.itemAt(i);
            if (!item) continue;
            regions.push(item.inputRegion);
            dragging = dragging || item.manipulating;
        }
        inputRegions = regions;
        canvas.dragging = dragging;
    }
    function activate(item) {
        activePin = item;
        if (!item.tracing) item.forceActiveFocus();
    }

    Repeater {
        id: pins
        model: canvas.entries
        Pin {
            id: pinDelegate
            required property var modelData
            entry: modelData
            output: canvas.screen
            controller: canvas.controller
            shown: !canvas.controller.hidden && (!canvas.controller.focusedPin || canvas.controller.focusedPin === entry.path)
            onActivated: canvas.activate(pinDelegate)
            onManipulatingChanged: canvas.refreshRegions()
            onPainted: canvas.painted()
            onClosed: path => canvas.controller.removePin(path)
        }
        onItemAdded: (index, item) => {
            canvas.refreshRegions();
            if (!canvas.activePin) canvas.activate(item);
        }
        onItemRemoved: (index, item) => {
            if (canvas.activePin === item) canvas.activePin = null;
            canvas.refreshRegions();
        }
    }
    Region {
        id: restoreRegion
        x: restoreChip.x
        y: restoreChip.y
        width: restoreChip.visible ? restoreChip.width : 0
        height: restoreChip.visible ? restoreChip.height : 0
    }
    CutRect {
        id: restoreChip
        x: 16
        y: 16
        width: restoreRow.width + 16
        height: 32
        visible: canvas.controller.hidden || canvas.controller.focusedPin !== ""
        cut: Tokens.radius
        design: Tokens.design
        fill: Theme.panelSolid
        strokeColor: Theme.hair
        strokeWidth: 1
        Row {
            id: restoreRow
            anchors.centerIn: parent
            spacing: 8
            Glyph { name: "visibility"; size: 16; ink: Theme.ink; anchors.verticalCenter: parent.verticalCenter }
            Text { text: I18n.tr("Restore pins"); color: Theme.ink; font.family: Theme.ui; font.pixelSize: 11; anchors.verticalCenter: parent.verticalCenter }
        }
        MouseArea { anchors.fill: parent; cursorShape: Qt.PointingHandCursor; onClicked: canvas.controller.restore() }
    }
}
