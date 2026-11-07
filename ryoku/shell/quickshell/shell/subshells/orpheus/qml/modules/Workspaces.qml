pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons
import "../kit" as K
import "../settings" as S
import "workspaces" as W

Item {
    id: root

    required property var scene
    required property string lane
    property bool present: true

    property var rows: []
    property string appliedSignature: ""
    readonly property string rowSignature: workspaceSignature()

    function field(value) {
        const text = String(value);
        return text.length + ":" + text;
    }

    function workspaceSignature() {
        const live = Wm.workspaces || [];
        const iconStyle = S.Prefs.workspaceStyle === "icons";
        let outputsReported = false;
        for (let index = 0; index < live.length; ++index) {
            if (String(live[index].output || "").length > 0) {
                outputsReported = true;
                break;
            }
        }

        const parts = [
            field(S.Prefs.workspaceStyle),
            S.Prefs.workspaceShowEmpty ? "1" : "0",
            S.Prefs.workspaceSpare ? "1" : "0",
            field(Wm.workspaceModel),
            field(scene.screenName),
            outputsReported ? "1" : "0"
        ];
        for (let index = 0; index < live.length; ++index) {
            const workspace = live[index];
            if (workspace.special === true)
                continue;
            if (outputsReported && scene.screenName.length > 0
                    && String(workspace.output || "") !== scene.screenName)
                continue;

            const key = workspace.id !== undefined ? workspace.id : workspace.name;
            let count = Math.max(0, Number(workspace.windows) || 0);
            const apps = [];
            if (iconStyle) {
                const windows = Wm.windowsOnWorkspace(key);
                count = windows.length;
                const seen = ({});
                for (let windowIndex = 0; windowIndex < windows.length; ++windowIndex) {
                    const appId = String(windows[windowIndex].appId || "").trim();
                    if (appId.length > 0 && seen[appId] !== true) {
                        seen[appId] = true;
                        apps.push(appId);
                    }
                }
            }
            const rank = workspace.index !== undefined ? workspace.index
                : workspace.idx !== undefined ? workspace.idx : index + 1;
            parts.push(field(key), field(workspace.name || key), field(rank),
                workspace.active === true ? "1" : "0",
                workspace.occupied === true ? "1" : "0",
                workspace.urgent === true ? "1" : "0",
                String(count), field(apps.join("\u001f")));
        }
        return parts.join("\u001e");
    }

    function rebuildRows() {
        const signature = root.rowSignature;
        if (signature === root.appliedSignature)
            return;
        root.rows = root.buildRows();
        root.appliedSignature = signature;
    }

    function buildRows() {
        const live = Wm.workspaces || [];
        let outputsReported = false;
        for (let index = 0; index < live.length; ++index) {
            if (String(live[index].output || "").length > 0) {
                outputsReported = true;
                break;
            }
        }

        const shown = [];
        for (let index = 0; index < live.length; ++index) {
            const workspace = live[index];
            if (workspace.special === true)
                continue;
            if (outputsReported && scene.screenName.length > 0
                    && String(workspace.output || "") !== scene.screenName)
                continue;

            const key = workspace.id !== undefined ? workspace.id : workspace.name;
            let count = Math.max(0, Number(workspace.windows) || 0);
            const apps = [];
            if (S.Prefs.workspaceStyle === "icons") {
                const windows = Wm.windowsOnWorkspace(key);
                count = windows.length;
                const seen = ({});
                for (let windowIndex = 0; windowIndex < windows.length; ++windowIndex) {
                    const appId = String(windows[windowIndex].appId || "").trim();
                    if (appId.length > 0 && seen[appId] !== true) {
                        seen[appId] = true;
                        apps.push(appId);
                    }
                }
            }

            const occupied = workspace.occupied === true || count > 0;
            if (!S.Prefs.workspaceShowEmpty && !occupied && workspace.active !== true)
                continue;

            const rank = workspace.index !== undefined ? Number(workspace.index)
                : workspace.idx !== undefined ? Number(workspace.idx) : index + 1;
            shown.push({
                key: key,
                label: String(workspace.name || key),
                index: Number.isFinite(rank) && rank > 0 ? rank : index + 1,
                active: workspace.active === true,
                occupied: occupied,
                urgent: workspace.urgent === true,
                apps: apps,
                windowCount: count
            });
        }
        shown.sort((left, right) => {
            const leftIndex = Number(left.index);
            const rightIndex = Number(right.index);
            if (!Number.isFinite(leftIndex) || !Number.isFinite(rightIndex))
                return 0;
            return leftIndex - rightIndex;
        });
        return S.Prefs.workspaceShowEmpty && Wm.workspaceModel === "fixed"
            ? root.padded(shown) : shown;
    }


    // A fixed workspace model numbers its slots, yet only the ones in use are
    // reported. Fill the holes, plus (unless workspaceSpare is off) one fresh slot
    // after the last occupied one, so empty workspaces show the way the setting
    // promises.
    function padded(shown) {
        const byNumber = ({});
        let last = 0;
        for (let index = 0; index < shown.length; ++index) {
            const number = Number(shown[index].label);
            if (!Number.isInteger(number) || number < 1 || byNumber[number] !== undefined)
                return shown;
            byNumber[number] = shown[index];
            last = Math.max(last, number);
        }
        if (last === 0)
            return shown;
        const end = S.Prefs.workspaceSpare && byNumber[last].occupied ? last + 1 : last;
        const filled = [];
        for (let number = 1; number <= end; ++number) {
            const row = byNumber[number];
            filled.push(row ? Object.assign({}, row, { index: number }) : {
                key: String(number),
                label: String(number),
                index: number,
                active: false,
                occupied: false,
                urgent: false,
                apps: [],
                windowCount: 0
            });
        }
        return filled;
    }

    onRowSignatureChanged: rebuildRows()
    Component.onCompleted: rebuildRows()

    implicitWidth: strip.implicitWidth + 2 * K.Style.hitPad
    implicitHeight: K.Style.barH
    height: K.Style.barH

    W.Strip {
        id: strip
        anchors.centerIn: parent
        style: S.Prefs.workspaceStyle
        square: S.Prefs.form === "square"
        model: root.rows
        onActivated: key => Wm.focusWorkspace(key)
        onOverviewRequested: Wm.toggleOverview()
        onScrolled: steps => Wm.cycleWorkspace(steps > 0 ? -1 : 1)
    }
}
