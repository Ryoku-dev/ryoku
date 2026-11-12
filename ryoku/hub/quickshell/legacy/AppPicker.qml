pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons

// A filterable list of installed applications -- the same app-picker idea the
// Keybinds page uses to bind an app to a role. Type to filter, click to choose;
// emits the app's launch command. Used by the Desktop Widgets music-app setting.
// A full-bleed overlay: its own scrim dismisses, the card swallows clicks.
Item {
    id: root

    property string title: I18n.tr("Music app")
    signal chosen(string cmd)
    signal chosenApp(string appId, string appName)
    signal dismissed()

    readonly property var catalog: {
        var out = [];
        var src = (typeof DesktopEntries !== "undefined" && DesktopEntries.applications)
            ? DesktopEntries.applications.values : [];
        for (var i = 0; i < src.length; i++) {
            var e = src[i];
            if (!e || e.noDisplay)
                continue;
            var cmd = ("" + ((e.command || []).join(" "))).trim();
            if (!cmd)
                continue;
            out.push({ name: e.name || cmd, cmd: cmd, id: (e.id || "") });
        }
        out.sort(function (a, b) { return a.name.toLowerCase().localeCompare(b.name.toLowerCase()); });
        return out;
    }
    property string query: ""
    readonly property var filtered: {
        var q = root.query.toLowerCase();
        if (!q.length)
            return root.catalog;
        return root.catalog.filter(function (e) {
            return (e.name + " " + e.cmd).toLowerCase().indexOf(q) >= 0;
        });
    }

    Rectangle {
        anchors.fill: parent
        color: Qt.rgba(0, 0, 0, 0.5)
        MouseArea { anchors.fill: parent; onClicked: root.dismissed() }
    }

    Rectangle {
        anchors.centerIn: parent
        width: Math.min(parent.width * 0.7, 460)
        height: Math.min(parent.height * 0.82, 540)
        radius: Tokens.radius * 2
        color: Tokens.paper
        border.width: Tokens.border
        border.color: Tokens.lineStrong

        MouseArea { anchors.fill: parent }  // swallow clicks so the scrim doesn't dismiss

        Text {
            id: hdr
            anchors { left: parent.left; right: parent.right; top: parent.top; margins: Tokens.s4 }
            text: root.title
            color: Tokens.ink
            font.family: Tokens.display
            font.pixelSize: Tokens.fValue
        }

        Rectangle {
            id: searchBox
            anchors {
                left: parent.left; right: parent.right; top: hdr.bottom
                leftMargin: Tokens.s4; rightMargin: Tokens.s4; topMargin: Tokens.s3
            }
            height: 34
            radius: Tokens.radius
            color: "transparent"
            border.width: search.activeFocus ? 2 : Tokens.border
            border.color: search.activeFocus ? Tokens.ink : Tokens.line

            TextInput {
                id: search
                anchors.fill: parent
                anchors.leftMargin: 10
                anchors.rightMargin: 10
                verticalAlignment: Text.AlignVCenter
                color: Tokens.ink
                font.family: Tokens.ui
                font.pixelSize: 13
                focus: true
                clip: true
                selectByMouse: true
                onTextChanged: root.query = text
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                anchors.left: parent.left
                anchors.leftMargin: 10
                visible: search.text.length === 0
                text: I18n.tr("Search apps…")
                color: Tokens.inkFaint
                font: search.font
            }
        }

        ListView {
            id: list
            anchors {
                left: parent.left; right: parent.right; top: searchBox.bottom; bottom: parent.bottom
                leftMargin: Tokens.s4; rightMargin: Tokens.s4; topMargin: Tokens.s3; bottomMargin: Tokens.s4
            }
            clip: true
            model: root.filtered
            spacing: 1
            ScrollBar.vertical: ScrollRail { policy: ScrollBar.AsNeeded }
            WheelScroll { }

            delegate: Rectangle {
                id: appRow
                required property var modelData
                width: list.width
                height: 42
                radius: Tokens.radius
                color: rowHover.hovered ? Tokens.paperLift : "transparent"

                Column {
                    anchors {
                        left: parent.left; right: parent.right; verticalCenter: parent.verticalCenter
                        leftMargin: 10; rightMargin: 10
                    }
                    spacing: 1
                    Text {
                        width: parent.width
                        text: appRow.modelData.name
                        elide: Text.ElideRight
                        color: Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: 13
                    }
                    Text {
                        width: parent.width
                        text: appRow.modelData.cmd
                        elide: Text.ElideRight
                        color: Tokens.inkFaint
                        font.family: Tokens.mono
                        font.pixelSize: 10
                    }
                }
                HoverHandler { id: rowHover }
                TapHandler { onTapped: { root.chosen(appRow.modelData.cmd); root.chosenApp(appRow.modelData.id, appRow.modelData.name); } }
            }
        }
    }
}
