pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import stage
import stage.modules.ii.editMode
import "Singletons" as StageCfg

// The Stage Editor, mounted over Ryoku's live desktop (docs/stage.md).
//
// The chrome (the shrinking toolbar, the catalogue drawer, the per-widget menu)
// is the reference's Edit Mode, ported verbatim into the `stage` module; it
// frames one screen at a time and builds its surfaces only while the mode is on,
// so an idle desktop pays nothing. This host is the seam: Ryoku's Edit widgets
// session (StageSession) and the ported mode drive each other, so the toolbar
// frames the desktop Ryoku is already editing (the same widgets.json store, the
// same WidgetSlot drag/resize) rather than a second one.
Scope {
    id: root

    // The ported chrome, once for the whole shell (it is per-screen inside).
    EditModeChrome {
    }

    // Ryoku's Edit widgets session opens the ported mode on its monitor, with
    // the catalogue it asked for already showing; leaving the session closes
    // the mode, and asking for another catalogue while open switches to it.
    Connections {
        target: StageCfg.StageSession
        function onModeChanged() {
            if (StageCfg.StageSession.widgets) {
                GlobalStates.openEditMode(StageCfg.StageSession.monitor);
                root.showSection();
            } else {
                GlobalStates.editMode = false;
            }
        }
        function onSectionChanged() {
            if (StageCfg.StageSession.widgets)
                root.showSection();
        }
    }
    // A section may name a page inside it ("wallpaper/wallpapers"), so a
    // hand-off can land on the wallpaper grid rather than its root.
    function showSection() {
        const parts = StageCfg.StageSession.section.split("/");
        GlobalStates.editDrawerSection = parts[0];
        GlobalStates.editDrawerPage = parts.slice(1).join("/");
        GlobalStates.editDrawerOpen = true;
    }

    // The chrome's Done (and Escape to the top of its ladder) leaves the mode;
    // mirror that back into Ryoku's session so the desktop drops its frames.
    Connections {
        target: GlobalStates
        function onEditModeChanged() {
            if (!GlobalStates.editMode && GlobalStates._editReopenMonitor === ""
                    && StageCfg.StageSession.widgets)
                StageCfg.StageSession.leave();
        }
        function onEditModeMonitorChanged() {
            if (StageCfg.StageSession.widgets
                    && GlobalStates.editModeMonitor !== ""
                    && StageCfg.StageSession.monitor !== GlobalStates.editModeMonitor) {
                StageCfg.StageSession.selection = [];
                StageCfg.StageSession.inspecting = "";
                StageCfg.StageSession.monitor = GlobalStates.editModeMonitor;
            }
        }
    }
}
