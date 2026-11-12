import QtQuick
import QtQuick.Effects
import Quickshell.Widgets
import shell.services as Ryoku
import Ryoku.Ui.Singletons
import "../kit"

FocusScope {
    id: menu

    property var entry: null
    property int current: 0
    property real maxHeight: 10000
    signal picked(var action)
    signal dismissed()

    readonly property var rows: {
        var list = [{ label: I18n.tr("Open"), glyph: "open_in_new", action: null }];
        var actions = menu.entry && menu.entry.actions ? menu.entry.actions : [];
        for (var i = 0; i < actions.length; i++)
            if (actions[i] && actions[i].name)
                list.push({ label: actions[i].name, image: actions[i].icon || "", action: actions[i] });
        return list;
    }

    readonly property int rowH: 44
    // computed, not read off the Column, which lays out after the caller places the menu
    readonly property real fullHeight: menu.rows.length * (menu.rowH + Md.seam) - Md.seam + 16

    implicitWidth: 236
    implicitHeight: Math.min(menu.fullHeight, menu.maxHeight)
    onEntryChanged: flick.contentY = 0

    onCurrentChanged: {
        const top = menu.current * (menu.rowH + Md.seam);
        if (top < flick.contentY)
            flick.contentY = top;
        else if (top + menu.rowH > flick.contentY + flick.height)
            flick.contentY = top + menu.rowH - flick.height;
    }

    function step(d) {
        menu.current = (menu.current + d + menu.rows.length) % menu.rows.length;
    }

    Keys.onPressed: function (event) {
        if (event.key === Qt.Key_Down || event.key === Qt.Key_Tab) {
            menu.step(1);
        } else if (event.key === Qt.Key_Up || event.key === Qt.Key_Backtab) {
            menu.step(-1);
        } else if (event.key === Qt.Key_Return || event.key === Qt.Key_Enter || event.key === Qt.Key_Space) {
            menu.picked(menu.rows[menu.current].action);
        } else if (event.key === Qt.Key_Escape || event.key === Qt.Key_Menu) {
            menu.dismissed();
        } else {
            return;
        }
        event.accepted = true;
    }

    RectangularShadow {
        anchors.fill: parent
        radius: Md.rLg
        blur: 12
        offset: Qt.vector2d(0, 3)
        color: Qt.rgba(0, 0, 0, 0.45)
    }

    Rectangle {
        anchors.fill: parent
        radius: Md.rLg
        color: Md.containerHigh
    }

    Flickable {
        id: flick
        anchors { fill: parent; margins: 8 }
        clip: true
        contentHeight: column.height
        boundsBehavior: Flickable.StopAtBounds

        Column {
            id: column
            width: flick.width
            spacing: Md.seam

            Repeater {
                model: menu.rows

                delegate: Item {
                    id: row
                    required property var modelData
                    required property int index
                    readonly property bool on: row.index === menu.current

                    width: column.width
                    height: menu.rowH

                    Rectangle {
                        anchors.fill: parent
                        radius: Md.rMd
                        color: row.on ? Md.containerHighest : Md.alpha(Md.containerHighest, 0)
                    }

                    Icon {
                        visible: !row.modelData.image
                        anchors { left: parent.left; leftMargin: 12; verticalCenter: parent.verticalCenter }
                        name: row.modelData.glyph || "bolt"
                        size: 20
                        color: Md.inkMuted
                    }
                    IconImage {
                        visible: !!row.modelData.image
                        anchors { left: parent.left; leftMargin: 12; verticalCenter: parent.verticalCenter }
                        implicitSize: 20
                        source: row.modelData.image ? Ryoku.Icons.path(row.modelData.image, true) : ""
                    }

                    Label {
                        anchors { left: parent.left; leftMargin: 44; right: parent.right; rightMargin: 12; verticalCenter: parent.verticalCenter }
                        role: "body"
                        text: row.modelData.label
                    }

                    StateLayer {
                        radius: Md.rMd
                        ring: false
                        activeFocusOnTab: false
                        onEntered: menu.current = row.index
                        onActivated: menu.picked(row.modelData.action)
                        onSecondary: menu.picked(row.modelData.action)
                    }
                }
            }
        }
    }
}
