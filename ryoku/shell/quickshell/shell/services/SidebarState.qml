pragma Singleton
pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Ryoku.Ui.Singletons
import "lib/screens.js" as Screens

Singleton {
    id: root

    property int intentRevision: 0

    readonly property real motionMultiplier: Config.sidebars.motion === "quick" ? 0.6
        : Config.sidebars.motion === "calm" ? 1.5 : 1.0
    readonly property int enterDuration: (Motion.reduce || Tokens.reduceMotion)
        ? 0 : Math.round(Motion.dur(420) * motionMultiplier)
    readonly property int exitDuration: (Motion.reduce || Tokens.reduceMotion)
        ? 0 : Math.round(Motion.dur(260) * motionMultiplier)
    readonly property var enterCurve: [0.16, 1, 0.3, 1, 1, 1]
    readonly property var exitCurve: [0, 0, 0.58, 1, 1, 1]

    readonly property bool anyOpen: {
        var revision = root.intentRevision;
        var list = states.instances;
        for (var i = 0; i < list.length; ++i) {
            if (list[i].leftOpen || list[i].rightOpen
                    || list[i].leftProgress > 0.001 || list[i].rightProgress > 0.001)
                return true;
        }
        return false;
    }

    function screenName(screen) {
        if (typeof screen === "string")
            return screen;
        return screen && screen.name ? screen.name : "";
    }

    function sliceFor(screen) {
        var name = root.screenName(screen);
        if (name !== "")
            return Screens.sliceForName(states.instances, name);
        var focused = Screens.sliceForName(states.instances, Wm.focusedOutput);
        if (focused)
            return focused;
        return states.instances.length > 0 ? states.instances[0] : null;
    }

    function validSide(side) {
        return side === "left" || side === "right";
    }


    function sideEnabled(side) {
        return root.validSide(side) && Config.sidebars[side].enabled;
    }
    function isOpen(screen, side) {
        var slice = root.sliceFor(screen);
        if (!slice || !root.validSide(side))
            return false;
        return side === "left" ? slice.leftOpen : slice.rightOpen;
    }

    function progress(screen, side) {
        var slice = root.sliceFor(screen);
        if (!slice || !root.validSide(side))
            return 0;
        return side === "left" ? slice.leftProgress : slice.rightProgress;
    }

    function activeTab(screen, side) {
        var slice = root.sliceFor(screen);
        if (!slice || !root.validSide(side))
            return "";
        return side === "left" ? slice.leftTab : slice.rightTab;
    }

    function selectTab(side, screen, tab) {
        var slice = root.sliceFor(screen);
        if (!slice || !root.validSide(side) || !tab)
            return;
        if (side === "left")
            slice.leftTab = tab;
        else
            slice.rightTab = tab;
    }

    function setProgress(screen, side, value) {
        var slice = root.sliceFor(screen);
        if (!slice || !root.validSide(side))
            return;
        var next = Math.max(0, Math.min(1, Number(value) || 0));
        if (side === "left")
            slice.leftProgress = next;
        else
            slice.rightProgress = next;
    }

    function open(side, screen, tab) {
        var slice = root.sliceFor(screen);
        if (!slice || !root.sideEnabled(side))
            return;
        if (side === "left") {
            slice.rightOpen = false;
            if (tab)
                slice.leftTab = tab;
            slice.leftOpen = true;
        } else {
            slice.leftOpen = false;
            if (tab)
                slice.rightTab = tab;
            slice.rightOpen = true;
        }
        root.intentRevision++;
    }

    function toggle(side, screen, tab) {
        var slice = root.sliceFor(screen);
        if (!slice || !root.sideEnabled(side))
            return;
        var opened = side === "left" ? slice.leftOpen : slice.rightOpen;
        var currentTab = side === "left" ? slice.leftTab : slice.rightTab;
        if (opened && tab && tab !== currentTab) {
            root.selectTab(side, screen, tab);
            return;
        }
        if (opened) {
            if (side === "left")
                slice.leftOpen = false;
            else
                slice.rightOpen = false;
            root.intentRevision++;
            return;
        }
        root.open(side, screen, tab);
    }

    function closeAll(screen) {
        var slice = root.sliceFor(screen);
        if (!slice)
            return;
        if (!slice.leftOpen && !slice.rightOpen)
            return;
        slice.leftOpen = false;
        slice.rightOpen = false;
        root.intentRevision++;
    }

    function slideOffset(screen) {
        var slice = root.sliceFor(screen);
        if (!slice)
            return 0;
        var scale = Tokens.uiScaleFor(root.screenName(screen));
        var distance = Config.sidebars.width * scale * Config.sidebars.wallpaperSlide;
        return distance * (slice.leftProgress - slice.rightProgress);
    }

    function consumeRequest(requestedId, monitor, context) {
        var value = requestedId || "";
        var split = value.indexOf("#");
        var id = split >= 0 ? value.substring(0, split) : value;
        var tab = split >= 0 ? value.substring(split + 1) : "";
        if (!tab && typeof context === "string")
            tab = context.charAt(0) === "#" ? context.substring(1) : context;
        else if (!tab && context && typeof context === "object")
            tab = context.tab || context.page || "";
        if (id !== "sidebar-left" && id !== "sidebar-right")
            return;
        root.toggle(id === "sidebar-left" ? "left" : "right", monitor, tab);
    }

    Connections {
        target: ShellState
        function onSurfaceRequested(id, mon, context) {
            root.consumeRequest(id, mon, context);
        }
        function onSurfaceClosed(id, mon) {
            var base = (id || "").split("#")[0];
            if (base === "sidebar-left" || base === "sidebar-right")
                root.closeAll(mon);
        }
    }

    Connections {
        target: Config
        function onSidebarsChanged() {
            var changed = false;
            var list = states.instances;
            for (var i = 0; i < list.length; ++i) {
                if (!Config.sidebars.left.enabled && list[i].leftOpen) {
                    list[i].leftOpen = false;
                    changed = true;
                }
                if (!Config.sidebars.right.enabled && list[i].rightOpen) {
                    list[i].rightOpen = false;
                    changed = true;
                }
            }
            if (changed)
                root.intentRevision++;
        }
    }

    Variants {
        id: states
        model: ShellState.screens

        PersistentProperties {
            required property var modelData
            property bool leftOpen: false
            property bool rightOpen: false
            property real leftProgress: 0
            property real rightProgress: 0
            property string leftTab: "controls"
            property string rightTab: "overview"
        }
    }
}
