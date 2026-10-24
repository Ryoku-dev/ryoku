import QtQuick
import QtQuick.Layouts
import Quickshell
import Ryoku.Ui.Singletons
import "../../../reusables"
import "../../../"
import "../../"

Item {
    id: root

    property var module: null
    property var widget: root

    property string orientation: module ? module.orientation : "horizontal"
    readonly property bool isVertical: orientation === "vertical"

    readonly property bool isCompact: module ? module.isCompact : false
    readonly property var barWindow: module ? module.barWindow : null
    readonly property bool moduleActive: module ? module.moduleActive : true
    // The pills, numbers and pacman faces tint their slots with widget.barOpacity, and this face is their widget.
    readonly property real barOpacity: module && module.barOpacity !== undefined ? module.barOpacity : ThemeBackend.elementOpacity

    readonly property var wsList: Wm.workspaces

    property int configRevision: 0

    Connections {
        target: (typeof Config !== "undefined") ? Config : null
        function onSettingsLoaded() { root.configRevision++; }
        function onRawSettingsChanged() { root.configRevision++; }
    }

    property string workspacesStyle: {
        let dummy = configRevision;
        if (module && module.variant) return module.variant;
        let bs = (typeof Config !== "undefined") ? Config.getSetting("bar", {}) : {};
        if (bs && bs.workspacesStyle) return bs.workspacesStyle;
        if (bs && bs.workspaces && bs.workspaces.style) return bs.workspaces.style;
        return "pills";
    }

    property int baseWorkspaceCount: {
        let dummy = configRevision;
        if (typeof Config !== "undefined" && Config.rawSettings) {
            if (Config.rawSettings.bar && Config.rawSettings.bar.workspaceCount !== undefined) {
                return Math.max(2, Math.min(10, Config.rawSettings.bar.workspaceCount));
            }
            if (Config.rawSettings.general && Config.rawSettings.general.workspaceCount !== undefined) {
                return Math.max(2, Math.min(10, Config.rawSettings.general.workspaceCount));
            }
            if (Config.rawSettings.workspaceCount !== undefined) {
                return Math.max(2, Math.min(10, Config.rawSettings.workspaceCount));
            }
        }
        return 8;
    }

    property int workspaceCount: Math.max(2, (activeIndex >= baseWorkspaceCount) ? (activeIndex + 1) : baseWorkspaceCount)

    property bool hideEmptyWorkspaces: {
        let dummy = configRevision;
        if (typeof Config !== "undefined" && Config.rawSettings) {
            let bar = Config.rawSettings.bar || {};
            let side = Config.rawSettings.sideBar || {};
            if (isVertical && side.hideEmptyWorkspaces !== undefined)
                return Boolean(side.hideEmptyWorkspaces);
            if (isVertical && bar.sideHideEmptyWorkspaces !== undefined)
                return Boolean(bar.sideHideEmptyWorkspaces);
            if (bar.hideEmptyWorkspaces !== undefined)
                return Boolean(bar.hideEmptyWorkspaces);
        }
        return false;
    }

    ListModel {
        id: workspaceListModel
    }

    function syncModel() {
        let target = workspaceCount;
        while (workspaceListModel.count < target) {
            workspaceListModel.append({ "modelData": workspaceListModel.count });
        }
        while (workspaceListModel.count > target) {
            workspaceListModel.remove(workspaceListModel.count - 1);
        }
    }

    onWorkspaceCountChanged: syncModel()

    function findRepeater(obj) {
        if (!obj) return null;
        if (obj.model !== undefined && obj.count !== undefined && typeof obj.itemAt === "function") {
            return obj;
        }
        if (obj.children) {
            for (let i = 0; i < obj.children.length; i++) {
                let res = findRepeater(obj.children[i]);
                if (res) return res;
            }
        }
        if (obj.data) {
            for (let j = 0; j < obj.data.length; j++) {
                let res = findRepeater(obj.data[j]);
                if (res) return res;
            }
        }
        return null;
    }

    function attachModel() {
        if (faceLoader.item) {
            faceLoader.item.widget = root;
            let rep = findRepeater(faceLoader.item);
            if (rep && rep.model !== workspaceListModel) {
                rep.model = workspaceListModel;
            }
        }
    }

    function s(val) {
        if (barWindow && typeof barWindow.cs === "function") return barWindow.cs(val);
        if (barWindow && typeof barWindow.s === "function") return barWindow.s(val);
        if (typeof Scaler !== "undefined" && typeof Scaler.s === "function") return Math.round(Scaler.s(val));
        return val;
    }

    function numericSlot(workspace) {
        let n = parseInt(workspace.name !== undefined && workspace.name !== "" ? workspace.name : workspace.id);
        return (!isNaN(n) && n > 0) ? n : -1;
    }

    function wsForSlot(index) {
        let list = wsList || [];
        for (let i = 0; i < list.length; i++) {
            if (!list[i].special && numericSlot(list[i]) === index + 1) return list[i];
        }
        return null;
    }

    function isOccupied(index) {
        let workspace = wsForSlot(index);
        return workspace !== null && workspace.occupied === true;
    }

    function isShown(index) {
        if (!hideEmptyWorkspaces) return true;
        return index <= activeIndex || isOccupied(index);
    }

    function focusWorkspace(index) {
        let workspace = wsForSlot(index);
        Wm.focusWorkspace(workspace ? String(workspace.id) : String(index + 1));
    }

    property int activeIndex: {
        let focused = Wm.focusedWorkspace;
        if (!focused) return -1;
        let slot = numericSlot(focused);
        return slot > 0 ? slot - 1 : -1;
    }
    Component.onCompleted: syncModel()



    property real targetLength: (moduleActive && workspaceCount > 0 && faceLoader.item) ? (isVertical ? faceLoader.item.implicitHeight : faceLoader.item.implicitWidth) + s(isCompact ? 18 : 22) : 0
    implicitWidth: isVertical ? (parent ? parent.width : 0) : targetLength
    implicitHeight: isVertical ? targetLength : (parent ? parent.height : 0)

    property real wheelAccumulator: 0
    Timer {
        id: wsWheelTimer
        interval: 200
        onTriggered: root.wheelAccumulator = 0
    }

    MouseArea {
        id: wsScrollArea
        anchors.fill: parent
        z: 10
        acceptedButtons: Qt.NoButton
        cursorShape: Qt.PointingHandCursor
        onWheel: wheel => {
            wsWheelTimer.restart();
            root.wheelAccumulator += wheel.angleDelta.y;
            const threshold = 120;
            if (Math.abs(root.wheelAccumulator) >= threshold) {
                let steps = Math.trunc(root.wheelAccumulator / threshold);
                root.wheelAccumulator = root.wheelAccumulator % threshold;

                if (root.workspaceCount > 1) {
                    let cur = root.activeIndex;
                    let nextIndex = 0;
                    if (cur < 0) {
                        nextIndex = steps > 0 ? (root.workspaceCount - 1) : 0;
                    } else if (steps > 0) {
                        nextIndex = (cur - 1 + root.workspaceCount) % root.workspaceCount;
                    } else if (steps < 0) {
                        nextIndex = (cur + 1) % root.workspaceCount;
                    }
                    if (nextIndex !== root.activeIndex) root.focusWorkspace(nextIndex);
                }
            }
        }
    }

    Loader {
        id: faceLoader
        z: 2
        anchors.left: root.isVertical ? undefined : parent.left
        anchors.leftMargin: root.isVertical ? 0 : s(isCompact ? 18 : 22) / 2
        anchors.verticalCenter: root.isVertical ? undefined : parent.verticalCenter
        anchors.top: root.isVertical ? parent.top : undefined
        anchors.topMargin: root.isVertical ? s(isCompact ? 18 : 22) / 2 : 0
        anchors.horizontalCenter: root.isVertical ? parent.horizontalCenter : undefined
        source: BarModuleRegistry.variantFaceFile("workspaces", root.workspacesStyle)
        onLoaded: {
            attachModel();
            attachTimer.restart();
        }
    }

    Timer {
        id: attachTimer
        interval: 0
        onTriggered: root.attachModel()
    }

    Binding {
        target: faceLoader.item
        property: "widget"
        value: root
    }
}
