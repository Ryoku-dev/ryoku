pragma ComponentBehavior: Bound

import QtQuick
import Quickshell
import Quickshell.Io
import Ryoku.Ui
import Ryoku.Ui.Singletons

// The compositor picker on the Global page: every window manager Ryoku can run,
// the active one marked, available alternatives offered as switches, and
// unavailable rows explaining why. Driven entirely by `ryoku-hub wm list`, so
// it never names a compositor: rows, packages and config dirs arrive as data.
Column {
    id: ctl

    property var providers: []
    // chose(target, current): a non-active provider was picked. current is the
    // active row we would be leaving, so the sheet can offer keep-or-remove.
    signal chose(var target, var current)

    function reload() { listProc.running = false; listProc.running = true; }
    function currentRow() {
        for (var i = 0; i < ctl.providers.length; i++)
            if (ctl.providers[i].active)
                return ctl.providers[i];
        return null;
    }
    function cap(name) { return name.length ? name.charAt(0).toUpperCase() + name.slice(1) : name; }
    function hasAvailableAlternative() {
        for (var i = 0; i < ctl.providers.length; i++)
            if (!ctl.providers[i].active && ctl.providers[i].available === true)
                return true;
        return false;
    }

    spacing: 0
    Component.onCompleted: ctl.reload()

    Process {
        id: listProc
        command: ["ryoku-hub", "wm", "list"]
        stdout: StdioCollector {
            onStreamFinished: {
                try { ctl.providers = JSON.parse(this.text); }
                catch (e) { ctl.providers = []; }
            }
        }
    }

    Section {
        id: sect
        width: ctl.width
        title: I18n.tr("COMPOSITOR")

        Column {
            width: sect.width
            spacing: Tokens.s2

            Repeater {
                model: ctl.providers
                Rectangle {
                    id: row
                    required property var modelData
                    readonly property bool isActive: row.modelData.active === true
                    readonly property bool isAvailable: row.modelData.available === true
                    readonly property bool isSelectable: !row.isActive && row.isAvailable
                    width: parent.width
                    height: 62
                    radius: Tokens.radius
                    opacity: row.isActive || row.isAvailable ? 1 : 0.62
                    color: row.isActive ? Tokens.tint5
                        : (row.isSelectable && rowTap.pressed ? Tokens.tint16
                        : (row.isSelectable && rowHov.hovered ? Tokens.tint10 : "transparent"))
                    border.width: Tokens.border
                    border.color: (row.isActive || (row.isSelectable && rowHov.hovered)) ? Tokens.lineStrong : Tokens.line
                    Behavior on color { ColorAnimation { duration: Tokens.snap } }

                    Column {
                        anchors { left: parent.left; leftMargin: Tokens.s4; right: mark.left; rightMargin: Tokens.s3; verticalCenter: parent.verticalCenter }
                        spacing: 3
                        Text {
                            text: ctl.cap(row.modelData.name)
                            color: row.isAvailable || row.isActive ? Tokens.ink : Tokens.inkMuted
                            font.family: Tokens.display
                            font.pixelSize: Tokens.fRow
                            font.weight: Font.Medium
                        }
                        Text {
                            width: parent.width
                            elide: Text.ElideRight
                            text: row.isActive ? I18n.tr("RUNNING NOW")
                                : (!row.isAvailable
                                ? (row.modelData.reason
                                    ? I18n.tr("Not available on this system: %1").arg(row.modelData.reason)
                                    : I18n.tr("Not available on this system."))
                                : (row.modelData.installed ? I18n.tr("INSTALLED")
                                : (row.modelData.deployed ? I18n.tr("READY") : I18n.tr("NOT INSTALLED"))))
                            color: Tokens.inkMuted
                            font.family: Tokens.mono
                            font.pixelSize: Tokens.fTiny
                            font.letterSpacing: 0.6
                        }
                    }

                    // The active provider wears a bone chip; available
                    // alternatives wear a switch cue and take the tap.
                    Item {
                        id: mark
                        anchors { right: parent.right; rightMargin: Tokens.s4; verticalCenter: parent.verticalCenter }
                        width: row.isActive ? chip.width : (row.isSelectable ? hint.width : 0)
                        height: 20
                        Rectangle {
                            id: chip
                            visible: row.isActive
                            width: curLabel.width + Tokens.s3
                            height: 20
                            radius: Tokens.radius
                            color: Tokens.bone
                            Text {
                                id: curLabel
                                anchors.centerIn: parent
                                text: I18n.tr("CURRENT")
                                color: Tokens.inkOnBone
                                font.family: Tokens.ui
                                font.pixelSize: Tokens.fTiny
                                font.weight: Font.Medium
                                font.letterSpacing: Tokens.trackLabel
                            }
                        }
                        Text {
                            id: hint
                            visible: row.isSelectable
                            anchors.verticalCenter: parent.verticalCenter
                            anchors.right: parent.right
                            text: I18n.tr("REVIEW SWITCH  \u2192")
                            color: rowHov.hovered ? Tokens.ink : Tokens.inkFaint
                            font.family: Tokens.ui
                            font.pixelSize: Tokens.fMicro
                            font.weight: Font.Medium
                            font.letterSpacing: Tokens.trackLabel
                        }
                    }

                    HoverHandler { id: rowHov; enabled: row.isSelectable; cursorShape: Qt.PointingHandCursor }
                    TapHandler {
                        id: rowTap
                        enabled: row.isSelectable
                        onTapped: {
                            if (row.isSelectable)
                                ctl.chose(row.modelData, ctl.currentRow());
                        }
                    }
                }
            }

            Text {
                width: parent.width
                wrapMode: Text.WordWrap
                text: ctl.hasAvailableAlternative()
                    ? I18n.tr("A switch takes effect at your next login, installing the chosen compositor first if it is not already on the machine. Every setting lives in one store, so a switch never loses your preferences.")
                    : I18n.tr("This system runs one desktop. Other desktops are listed here when they become available.")
                color: Tokens.inkFaint
                font.family: Tokens.ui
                font.pixelSize: Tokens.fSmall
                lineHeight: 1.3
            }
        }
    }
}
