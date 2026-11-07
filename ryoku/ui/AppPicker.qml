import QtQuick
import QtQuick.Controls
import Ryoku.Ui.Singletons

// Modal command picker: a filterable list of installed applications (each app's
// name plus the command it launches -- an executable cheatsheet) and a free-text
// field for any command. Data-driven: the caller supplies `apps` as [{name,cmd}],
// so this carries no platform dependency and is reused for app-role launchers and
// custom Run-command binds. Emits picked(cmd) on a list tap, a custom command, or
// Enter; dismissed() on Escape.
Rectangle {
    id: pick

    property string title: I18n.tr("App")
    property var apps: []          // [{ name, cmd }] installed apps, caller-supplied
    property string current: ""    // current command, highlighted in the list
    signal picked(string cmd)
    signal dismissed()

    width: 380
    height: 460
    radius: Tokens.radius
    color: Tokens.paperLift
    border.width: Tokens.border
    border.color: Tokens.lineStrong

    function open() { q.text = ""; q.forceActiveFocus(); }

    readonly property var shown: {
        var f = ("" + q.text).toLowerCase().trim();
        if (f === "")
            return pick.apps || [];
        return (pick.apps || []).filter(function (a) {
            return ("" + a.name).toLowerCase().indexOf(f) >= 0
                || ("" + a.cmd).toLowerCase().indexOf(f) >= 0;
        });
    }

    // header: title + match count.
    Row {
        id: hdr
        anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s3 }
        Text {
            text: pick.title.toUpperCase(); color: Tokens.ink
            font.family: Tokens.ui; font.pixelSize: 10; font.weight: Font.Medium
            font.letterSpacing: Tokens.trackLabel
        }
        Item { width: parent.width - 240; height: 1 }
        Text {
            text: pick.shown.length + " / " + (pick.apps ? pick.apps.length : 0)
            color: Tokens.inkFaint; font.family: Tokens.mono; font.pixelSize: 9
        }
    }

    // filter: the interface for a long app list; Enter takes the top match.
    Rectangle {
        id: filt
        anchors { left: parent.left; right: parent.right; top: hdr.bottom }
        anchors.leftMargin: Tokens.s3; anchors.rightMargin: Tokens.s3; anchors.topMargin: Tokens.s2
        height: 30; color: "transparent"; radius: Tokens.radius
        border.width: q.activeFocus ? 2 : Tokens.border
        border.color: q.activeFocus ? Tokens.ink : Tokens.line
        TextInput {
            id: q
            anchors.fill: parent; anchors.leftMargin: 8; anchors.rightMargin: 8
            verticalAlignment: Text.AlignVCenter
            color: Tokens.ink; font.family: Tokens.ui; font.pixelSize: 12
            selectByMouse: true
            Keys.onReturnPressed: if (pick.shown.length) pick.picked(pick.shown[0].cmd)
            Keys.onEscapePressed: pick.dismissed()
            Text {
                anchors.fill: parent; visible: q.text === ""
                text: I18n.tr("Filter apps\u2026"); color: Tokens.inkMuted; font: q.font
                verticalAlignment: Text.AlignVCenter
            }
        }
    }

    // custom command: pinned to the bottom for anything the list does not offer.
    Rectangle {
        id: custom
        anchors { left: parent.left; right: parent.right; bottom: parent.bottom }
        anchors.leftMargin: Tokens.s3; anchors.rightMargin: Tokens.s3; anchors.bottomMargin: Tokens.s3
        height: 30; color: "transparent"; radius: Tokens.radius
        border.width: cq.activeFocus ? 2 : Tokens.border
        border.color: cq.activeFocus ? Tokens.ink : Tokens.line
        TextInput {
            id: cq
            anchors.fill: parent; anchors.leftMargin: 8; anchors.rightMargin: 8
            verticalAlignment: Text.AlignVCenter
            color: Tokens.ink; font.family: Tokens.mono; font.pixelSize: 12
            selectByMouse: true
            Keys.onReturnPressed: if (("" + cq.text).trim()) pick.picked(("" + cq.text).trim())
            Keys.onEscapePressed: pick.dismissed()
            Text {
                anchors.fill: parent; visible: cq.text === ""
                text: I18n.tr("or type any command\u2026"); color: Tokens.inkMuted; font: cq.font
                verticalAlignment: Text.AlignVCenter
            }
        }
    }

    // the app list: name over the command it inserts.
    Flickable {
        id: listF
        anchors { left: parent.left; right: parent.right; top: filt.bottom; bottom: custom.top }
        anchors.leftMargin: Tokens.s3; anchors.rightMargin: Tokens.s3
        anchors.topMargin: Tokens.s2; anchors.bottomMargin: Tokens.s2
        contentWidth: width; contentHeight: lc.height; clip: true
        boundsBehavior: Flickable.StopAtBounds
        ScrollBar.vertical: ScrollBar { contentItem: Rectangle { implicitWidth: 3; color: Tokens.line } }
        Column {
            id: lc
            width: listF.width
            Repeater {
                model: pick.shown
                delegate: Rectangle {
                    id: appRow
                    required property var modelData
                    readonly property bool sel: pick.current === appRow.modelData.cmd
                    width: lc.width; height: 34
                    color: arh.hovered ? Tokens.bone : "transparent"
                    Behavior on color { ColorAnimation { duration: 70 } }
                    Column {
                        anchors { left: parent.left; leftMargin: 8; right: selMark.left; rightMargin: 6; verticalCenter: parent.verticalCenter }
                        spacing: 1
                        Text {
                            width: parent.width
                            text: appRow.modelData.name
                            color: arh.hovered ? Tokens.inkOnBone : (appRow.sel ? Tokens.ink : Tokens.inkDim)
                            font.family: Tokens.ui; font.pixelSize: 12; elide: Text.ElideRight
                        }
                        Text {
                            width: parent.width
                            text: appRow.modelData.cmd
                            color: arh.hovered ? Tokens.inkOnBone : Tokens.inkFaint
                            font.family: Tokens.mono; font.pixelSize: 9; elide: Text.ElideRight
                        }
                    }
                    Text {
                        id: selMark
                        anchors { right: parent.right; rightMargin: 8; verticalCenter: parent.verticalCenter }
                        visible: appRow.sel
                        text: "\u25CF"; color: arh.hovered ? Tokens.inkOnBone : Tokens.ink; font.pixelSize: 7
                    }
                    HoverHandler { id: arh; cursorShape: Qt.PointingHandCursor }
                    TapHandler { onTapped: pick.picked(appRow.modelData.cmd) }
                }
            }
        }
    }
}
