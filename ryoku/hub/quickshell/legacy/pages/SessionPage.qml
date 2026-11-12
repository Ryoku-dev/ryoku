pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons
import "../Singletons"

// Session (DESIGN.md section 11, ADVANCED). The two thin login-time lists folded
// into one page: what the session runs at login (desktop.autostart, layered over
// Ryoku's own autostart) and the variables it exports (desktop.env, over the base
// session). Each "key" is an array of entries, so each cluster is a bespoke list
// editor -- a SettingCard holding a column of Field rows and its add/clear footer
// -- not a settings sheet. The shell owns the rail, the side panel and the
// Save/Revert/Reset action bar; edits land only via Save (provider config regen
// plus reload), never live, and a session reads both only at login, so a change
// takes effect at the next login. Every value is a Token.
Item {
    id: pg

    property var hub

    // gated so an empty state does not flash before `desktop get` returns.
    readonly property bool ready: pg.hub ? pg.hub.wmLoaded === true : false
    readonly property bool autostartSupported: Settings.modelsKey("desktop.autostart")
    readonly property bool environmentSupported: Settings.modelsKey("desktop.env")

    // live arrays from the draft: autostart is [{command}], env is [{key,value}].
    readonly property var cmdRows: pg.autostartSupported && pg.hub ? (pg.hub.hyprVal("desktop.autostart") || []) : []
    readonly property var envRows: pg.environmentSupported && pg.hub ? (pg.hub.hyprVal("desktop.env") || []) : []

    // hyprEdit swaps the whole array, so a Repeater rebinds and rebuilds the
    // delegate owning a focused field. rows therefore commit on editing-finished
    // only, and every helper hands hyprEdit a fresh slice rather than mutating the
    // live list. A TapHandler button never steals that focus, so a +/-/clear tap
    // used to act on the stale draft and eat the typed command (#294): the field
    // still holds the edit when the tap fires. Focus is exclusive, so the page
    // tracks the one live edit and every helper folds it into its slice first.
    property var liveEdit: null
    function foldLive(a, list) {
        var e = pg.liveEdit;
        if (!e || e.list !== list || e.i < 0 || e.i >= a.length)
            return;
        var row = a[e.i];
        if ("command" in e && row.command !== e.command) {
            row = Object.assign({}, row);
            row.command = e.command;
        }
        if ("key" in e && row.key !== e.key) {
            row = Object.assign({}, row);
            row.key = e.key;
        }
        if ("value" in e && row.value !== e.value) {
            row = Object.assign({}, row);
            row.value = e.value;
        }
        a[e.i] = row;
    }
    function patchCmd(i, val) {
        if (!pg.hub)
            return;
        var a = (pg.hub.hyprVal("desktop.autostart") || []).slice();
        a[i] = Object.assign({}, a[i]);
        a[i].command = val;
        if (pg.liveEdit && pg.liveEdit.list === "cmd" && pg.liveEdit.i === i)
            pg.liveEdit = null;
        pg.hub.hyprEdit("desktop.autostart", a);
    }
    function removeCmd(i) {
        if (!pg.hub)
            return;
        var a = (pg.hub.hyprVal("desktop.autostart") || []).slice();
        pg.foldLive(a, "cmd");
        a.splice(i, 1);
        pg.liveEdit = null;
        pg.hub.hyprEdit("desktop.autostart", a);
    }
    function addCmd() {
        if (!pg.hub)
            return;
        var a = (pg.hub.hyprVal("desktop.autostart") || []).slice();
        pg.foldLive(a, "cmd");
        a.push({ "command": "" });
        pg.liveEdit = null;
        pg.hub.hyprEdit("desktop.autostart", a);
    }
    function clearCmd() {
        if (pg.hub) {
            pg.liveEdit = null;
            pg.hub.hyprEdit("desktop.autostart", []);
        }
    }

    function patchEnv(i, key, val) {
        if (!pg.hub)
            return;
        var a = (pg.hub.hyprVal("desktop.env") || []).slice();
        a[i] = Object.assign({}, a[i]);
        a[i][key] = val;
        if (pg.liveEdit && pg.liveEdit.list === "env" && pg.liveEdit.i === i)
            pg.liveEdit = null;
        pg.hub.hyprEdit("desktop.env", a);
    }
    function removeEnv(i) {
        if (!pg.hub)
            return;
        var a = (pg.hub.hyprVal("desktop.env") || []).slice();
        pg.foldLive(a, "env");
        a.splice(i, 1);
        pg.liveEdit = null;
        pg.hub.hyprEdit("desktop.env", a);
    }
    function addEnv() {
        if (!pg.hub)
            return;
        var a = (pg.hub.hyprVal("desktop.env") || []).slice();
        pg.foldLive(a, "env");
        a.push({ "key": "", "value": "" });
        pg.liveEdit = null;
        pg.hub.hyprEdit("desktop.env", a);
    }
    function clearEnv() {
        if (pg.hub) {
            pg.liveEdit = null;
            pg.hub.hyprEdit("desktop.env", []);
        }
    }

    // ── a cluster footer: entry count on the left, Clear all + add on the right,
    // over a hairline. Shared by both lists so they read identically. ──
    component ClusterFoot: Item {
        id: foot
        property int entries: 0
        property bool topRule: true
        signal requestAdd()
        signal requestClear()

        anchors.left: parent.left
        anchors.right: parent.right
        height: Tokens.rowH

        Rectangle {
            visible: foot.topRule
            anchors { left: parent.left; right: parent.right; top: parent.top }
            anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4
            height: 1; color: Tokens.lineSoft
        }
        Text {
            anchors.left: parent.left; anchors.leftMargin: Tokens.s4
            anchors.verticalCenter: parent.verticalCenter
            // an entry count is file-truth chrome, so mono (DESIGN.md section 2).
            text: foot.entries === 1 ? I18n.tr("%1 ENTRY").arg(foot.entries) : I18n.tr("%1 ENTRIES").arg(foot.entries)
            color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: Tokens.fTiny
        }
        Row {
            anchors.right: parent.right; anchors.rightMargin: Tokens.s4
            anchors.verticalCenter: parent.verticalCenter
            spacing: Tokens.s3
            Btn {
                anchors.verticalCenter: parent.verticalCenter
                text: I18n.tr("CLEAR ALL")
                armed: foot.entries > 0
                stealFocus: true
                onAct: foot.requestClear()
            }
            IconBtn {
                anchors.verticalCenter: parent.verticalCenter
                glyph: "+"
                stealFocus: true
                onAct: foot.requestAdd()
            }
        }
    }

    // ── head: eyebrow, Fraunces title, blurb. Spans the body width and starts at
    // its left inset, so the title aligns with the first card column. ──
    Column {
        id: head
        anchors { left: parent.left; right: parent.right; top: parent.top }
        // the register row sits off the title: a rule over a 32px
        // title needs more than the gap between two lines of body text
        spacing: Tokens.s3

        Row {
            // the register row holds a fixed box, so the rule and the seal keep
            // their distance from the title on every page
            height: Tokens.s5
            spacing: Tokens.s2
            Rectangle {
                width: 16; height: 1; color: Tokens.ink
                anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: UiPreferences.brandMark("力"); color: Tokens.ink; font.family: Tokens.jp
                font.pixelSize: 11; anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: I18n.tr("SYSTEM"); color: Tokens.inkMuted; font.family: Tokens.ui
                font.pixelSize: 9; font.weight: Font.Medium; font.letterSpacing: Tokens.trackMark
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        Text {
            text: I18n.tr("Session"); color: Tokens.ink
            font.family: Tokens.display; font.pixelSize: Tokens.fTitle
        }
        Text {
            // a blurb is prose, so it keeps a reading cap while the body fills.
            width: Math.min(parent.width, 720)
            text: I18n.tr("What runs at login, and the variables your session exports.")
            color: Tokens.inkMuted; font.family: Tokens.ui
            font.pixelSize: Tokens.fBody; wrapMode: Text.WordWrap
        }
    }

    // ── the two clusters, laid side by side to fill the window; each fills a
    // column of the balanced grid, stacking only when the window is narrow. ──
    Flickable {
        id: flick
        anchors {
            left: parent.left; right: parent.right
            top: head.bottom; bottom: parent.bottom
            topMargin: Tokens.s5
        }
        contentWidth: width
        contentHeight: Math.max(col.height, height)
        clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
        WheelScroll { }

        CardColumns {
            id: col
            width: flick.width - Tokens.s3
            spacing: Tokens.s5
            // centre the two clusters in the body when the window is taller
            fillTo: flick.height
            // two clusters: two columns fill the width, no empty third.
            maxColumns: 2

            // ── AT LOGIN: the autostart commands ──
            SettingCard {
                id: loginCard
                width: col.colWidth
                title: I18n.tr("AT LOGIN")
                summary: pg.autostartSupported
                    ? (pg.cmdRows.length === 1 ? I18n.tr("%1 ENTRY").arg(pg.cmdRows.length) : I18n.tr("%1 ENTRIES").arg(pg.cmdRows.length))
                    : I18n.tr("NOT AVAILABLE")
                opacity: pg.autostartSupported ? 1 : 0.62

                Repeater {
                    model: pg.cmdRows

                    delegate: Item {
                        id: cmdRow
                        required property int index
                        required property var modelData

                        anchors.left: parent.left
                        anchors.right: parent.right
                        height: Tokens.rowH

                        // no top rule on the first row: the card header's own rule
                        // already parts it from the title.
                        Rectangle {
                            visible: cmdRow.index > 0
                            anchors { left: parent.left; right: parent.right; top: parent.top }
                            anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4
                            height: 1; color: Tokens.lineSoft
                        }
                        // a command is a config literal run by the shell, so mono.
                        Field {
                            id: cmdField
                            anchors.left: parent.left; anchors.leftMargin: Tokens.s4
                            anchors.right: cmdRemove.left; anchors.rightMargin: Tokens.s2
                            anchors.verticalCenter: parent.verticalCenter
                            tabular: true
                            placeholder: I18n.tr("command to run (e.g. nm-applet)")
                            text: cmdRow.modelData.command
                            onEdited: (v) => {
                                pg.liveEdit = { list: "cmd", i: cmdRow.index, command: v };
                            }
                            onCommitted: (v) => {
                                if (v !== cmdRow.modelData.command)
                                    pg.patchCmd(cmdRow.index, v);
                            }
                        }
                        IconBtn {
                            id: cmdRemove
                            anchors.right: parent.right; anchors.rightMargin: Tokens.s4
                            anchors.verticalCenter: parent.verticalCenter
                            // a paired minus, not a trash icon: remove is not danger
                            // and there is no red on the sheet to carry one.
                            glyph: "\u2212"
                            onAct: pg.removeCmd(cmdRow.index)
                        }
                    }
                }

                // empty state, gated on load so it does not flash before data lands.
                Item {
                    anchors.left: parent.left; anchors.right: parent.right
                    visible: pg.ready && pg.autostartSupported && pg.cmdRows.length === 0
                    height: visible ? loginEmpty.height + Tokens.s5 * 2 : 0
                    Empty {
                        id: loginEmpty
                        anchors.centerIn: parent
                        caption: I18n.tr("No autostart commands yet. Add one to get started.")
                    }
                }
                Item {
                    anchors.left: parent.left; anchors.right: parent.right
                    visible: pg.ready && !pg.autostartSupported
                    height: visible ? loginUnavailable.height + Tokens.s5 * 2 : 0
                    Empty {
                        id: loginUnavailable
                        anchors.centerIn: parent
                        caption: I18n.tr("The current session provider cannot add login commands.")
                    }
                }

                ClusterFoot {
                    visible: pg.autostartSupported
                    entries: pg.cmdRows.length
                    topRule: pg.cmdRows.length > 0 || (pg.ready && pg.cmdRows.length === 0)
                    onRequestAdd: pg.addCmd()
                    onRequestClear: pg.clearCmd()
                }
            }

            // ── ENVIRONMENT: the session variables ──
            SettingCard {
                id: envCard
                width: col.colWidth
                title: I18n.tr("ENVIRONMENT")
                summary: pg.environmentSupported
                    ? (pg.envRows.length === 1 ? I18n.tr("%1 ENTRY").arg(pg.envRows.length) : I18n.tr("%1 ENTRIES").arg(pg.envRows.length))
                    : I18n.tr("NOT AVAILABLE")
                opacity: pg.environmentSupported ? 1 : 0.62

                Repeater {
                    model: pg.envRows

                    delegate: Item {
                        id: envRow
                        required property int index
                        required property var modelData

                        readonly property real gap: Tokens.s2
                        readonly property real fieldsW: width - Tokens.s4 * 2 - envRemove.width - gap * 2
                        readonly property real keyW: Math.round(fieldsW * 0.42)

                        anchors.left: parent.left
                        anchors.right: parent.right
                        height: Tokens.rowH

                        Rectangle {
                            visible: envRow.index > 0
                            anchors { left: parent.left; right: parent.right; top: parent.top }
                            anchors.leftMargin: Tokens.s4; anchors.rightMargin: Tokens.s4
                            height: 1; color: Tokens.lineSoft
                        }
                        // key: a config variable name, so mono (file-truth boundary).
                        Field {
                            id: keyField
                            anchors.left: parent.left; anchors.leftMargin: Tokens.s4
                            anchors.verticalCenter: parent.verticalCenter
                            width: envRow.keyW
                            tabular: true
                            placeholder: I18n.tr("NAME (e.g. MOZ_ENABLE_WAYLAND)")
                            text: envRow.modelData.key
                            onEdited: (v) => {
                                pg.liveEdit = { list: "env", i: envRow.index, key: v };
                            }
                            onCommitted: (v) => {
                                if (v !== envRow.modelData.key)
                                    pg.patchEnv(envRow.index, "key", v);
                            }
                        }
                        Field {
                            id: valField
                            anchors.left: keyField.right; anchors.leftMargin: envRow.gap
                            anchors.right: envRemove.left; anchors.rightMargin: envRow.gap
                            anchors.verticalCenter: parent.verticalCenter
                            tabular: true
                            placeholder: I18n.tr("value (e.g. 1)")
                            text: envRow.modelData.value
                            onCommitted: (v) => {
                                if (v !== envRow.modelData.value)
                                    pg.patchEnv(envRow.index, "value", v);
                            }
                        }
                        IconBtn {
                            id: envRemove
                            anchors.right: parent.right; anchors.rightMargin: Tokens.s4
                            anchors.verticalCenter: parent.verticalCenter
                            glyph: "\u2212"
                            onAct: pg.removeEnv(envRow.index)
                        }
                    }
                }

                Item {
                    anchors.left: parent.left; anchors.right: parent.right
                    visible: pg.ready && pg.environmentSupported && pg.envRows.length === 0
                    height: visible ? envEmpty.height + Tokens.s5 * 2 : 0
                    Empty {
                        id: envEmpty
                        anchors.centerIn: parent
                        caption: I18n.tr("No custom variables yet. Add one to get started.")
                    }
                }
                Item {
                    anchors.left: parent.left; anchors.right: parent.right
                    visible: pg.ready && !pg.environmentSupported
                    height: visible ? envUnavailable.height + Tokens.s5 * 2 : 0
                    Empty {
                        id: envUnavailable
                        anchors.centerIn: parent
                        caption: I18n.tr("The current session provider cannot set environment variables.")
                    }
                }

                ClusterFoot {
                    visible: pg.environmentSupported
                    entries: pg.envRows.length
                    topRule: pg.envRows.length > 0 || (pg.ready && pg.envRows.length === 0)
                    onRequestAdd: pg.addEnv()
                    onRequestClear: pg.clearEnv()
                }
            }
        }
    }
}
