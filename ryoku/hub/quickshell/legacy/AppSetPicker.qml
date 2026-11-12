pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import Quickshell
import Ryoku.Ui
import Ryoku.Ui.Singletons

// Chooses a set of installed applications by id: the ones the launcher hides
// (invert: a switch reads "shown"), or the ones it pins (switch reads "pinned").
// Every entry the launcher would normally list is shown; an entry already in
// the set stays listed even if its package marks it NoDisplay, so it can be
// taken out again. Emits the whole new list of ids; the caller keeps it in its draft.
// A full-bleed overlay: its own scrim dismisses, the card swallows clicks.
Item {
    id: root

    property string title: I18n.tr("Hidden apps")
    property string countText: I18n.tr("%1 hidden")
    property bool invert: true
    property var hidden: []
    signal changed(var ids)
    signal dismissed()

    property string query: ""
    onVisibleChanged: {
        if (visible) {
            root.query = "";
            search.clear();
            search.grabFocus();
        }
    }

    readonly property var hiddenSet: {
        var s = {};
        var h = root.hidden || [];
        for (var i = 0; i < h.length; i++)
            s[h[i]] = true;
        return s;
    }

    readonly property var catalog: {
        var out = [];
        var src = (typeof DesktopEntries !== "undefined" && DesktopEntries.applications)
            ? DesktopEntries.applications.values : [];
        for (var i = 0; i < src.length; i++) {
            var e = src[i];
            if (!e || !e.id)
                continue;
            if (e.noDisplay && !root.hiddenSet[e.id])
                continue;
            out.push({ id: e.id, name: e.name || e.id, icon: e.icon || "" });
        }
        out.sort(function (a, b) { return a.name.toLowerCase().localeCompare(b.name.toLowerCase()); });
        return out;
    }
    readonly property var filtered: {
        var q = root.query.toLowerCase();
        if (!q.length)
            return root.catalog;
        return root.catalog.filter(function (e) {
            return (e.name + " " + e.id).toLowerCase().indexOf(q) >= 0;
        });
    }

    function setHidden(appId, hide) {
        var next = [];
        var h = root.hidden || [];
        for (var i = 0; i < h.length; i++)
            if (h[i] !== appId)
                next.push(h[i]);
        if (hide)
            next.push(appId);
        root.changed(next);
    }

    Rectangle {
        anchors.fill: parent
        color: Qt.rgba(0, 0, 0, 0.5)
        MouseArea { anchors.fill: parent; onClicked: root.dismissed() }
    }

    Rectangle {
        anchors.centerIn: parent
        width: Math.min(parent.width * 0.7, 520)
        height: Math.min(parent.height * 0.86, 620)
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
        Text {
            id: count
            anchors { right: parent.right; rightMargin: Tokens.s4; verticalCenter: hdr.verticalCenter }
            text: root.countText.arg((root.hidden || []).length)
            color: Tokens.inkFaint
            font.family: Tokens.mono
            font.pixelSize: Tokens.fSmall
        }

        Field {
            id: search
            anchors {
                left: parent.left; right: parent.right; top: hdr.bottom
                leftMargin: Tokens.s4; rightMargin: Tokens.s4; topMargin: Tokens.s3
            }
            toolbar: true
            placeholder: I18n.tr("Search apps…")
            clearOnEscape: false
            onEdited: (value) => root.query = value
            onEscaped: root.dismissed()
        }

        ListView {
            id: list
            anchors {
                left: parent.left; right: parent.right; top: search.bottom; bottom: parent.bottom
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
                readonly property bool isHidden: !!root.hiddenSet[appRow.modelData.id]
                width: list.width
                height: 42
                radius: Tokens.radius
                color: rowHover.hovered ? Tokens.paperLift : "transparent"

                Image {
                    id: ico
                    anchors { left: parent.left; leftMargin: 10; verticalCenter: parent.verticalCenter }
                    width: 22; height: 22
                    sourceSize: Qt.size(44, 44)
                    asynchronous: true
                    opacity: (root.invert && appRow.isHidden) ? 0.4 : 1
                    source: appRow.modelData.icon ? Quickshell.iconPath(appRow.modelData.icon, true) : ""
                }
                Column {
                    anchors {
                        left: ico.right; right: toggle.left; verticalCenter: parent.verticalCenter
                        leftMargin: 10; rightMargin: 10
                    }
                    spacing: 1
                    Text {
                        width: parent.width
                        text: appRow.modelData.name
                        elide: Text.ElideRight
                        color: (root.invert && appRow.isHidden) ? Tokens.inkFaint : Tokens.ink
                        font.family: Tokens.ui
                        font.pixelSize: 13
                    }
                    Text {
                        width: parent.width
                        text: appRow.modelData.id
                        elide: Text.ElideRight
                        color: Tokens.inkFaint
                        font.family: Tokens.mono
                        font.pixelSize: 10
                    }
                }
                // invert: on = shown, off = hidden; otherwise on = in the set
                Sw {
                    id: toggle
                    anchors { right: parent.right; rightMargin: 10; verticalCenter: parent.verticalCenter }
                    on: root.invert ? !appRow.isHidden : appRow.isHidden
                    onToggled: (v) => root.setHidden(appRow.modelData.id, root.invert ? !v : v)
                }
                HoverHandler { id: rowHover }
            }
        }
    }
}
