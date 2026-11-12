import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// a design gallery for one overlay: a click selects a tile, its plaque or Enter acts on it
Column {
    id: root

    required property var shelf
    property string accessibleName: ""
    // why the design in use does not show right now, when the bar style draws its own
    property string ownerNote: ""
    // an optional quiet action under the gallery, such as showing the design in use
    property string tryText: ""
    signal tryRequested()
    property string selectedId: ""
    property bool picked: false
    readonly property var tiles: root.shelf.tiles

    spacing: LithicTokens.space.s3

    function indexOf(id) {
        for (var i = 0; i < root.tiles.length; i++)
            if (root.tiles[i].id === id)
                return i;
        return -1;
    }

    // a new model resets the grid's index, so the selection settles after it
    property bool refreshing: false
    function syncSelection() {
        root.refreshing = false;
        if (root.tiles.length === 0)
            return;
        if (!root.picked && root.indexOf(root.shelf.active) >= 0)
            root.selectedId = root.shelf.active;
        else if (root.indexOf(root.selectedId) < 0)
            root.selectedId = root.tiles[0].id;
        gallery.currentIndex = root.indexOf(root.selectedId);
    }
    readonly property string activeId: root.shelf.active
    onActiveIdChanged: root.syncSelection()
    onTilesChanged: {
        root.refreshing = true;
        Qt.callLater(root.syncSelection);
    }
    Component.onCompleted: root.syncSelection()

    // a slow install, a failure or a stop that lands out of view scrolls in, in the page that holds the gallery
    function reveal(item) {
        for (var p = root.parent; p; p = p.parent) {
            if (p.flickable !== undefined && typeof p.reveal === "function") {
                var y = item.mapToItem(p.flickable, 0, 0).y;
                if (y < 0 || y + item.height > p.flickable.height)
                    p.reveal(item);
                return;
            }
        }
    }
    // scrolls the gallery to the selected design's tile
    function showSelected() {
        var tile = gallery.gridView.itemAtIndex(root.indexOf(root.selectedId));
        root.reveal(tile || gallery);
    }
    property Item revealing: null
    Timer {
        id: revealTimer
        interval: 50
        onTriggered: if (root.revealing && root.revealing.visible) root.reveal(root.revealing)
    }
    function revealSoon(item) {
        root.revealing = item;
        revealTimer.restart();
    }
    Connections {
        target: root.shelf
        function onSlowChanged() { if (root.shelf.slow) root.revealSoon(progress); }
        function onFailedChanged() { if (root.shelf.failed) root.revealSoon(failure); }
        function onStoppedChanged() { if (root.shelf.stopped) root.revealSoon(stoppedNote); }
    }

    DesignGallery {
        id: gallery
        width: parent.width
        accessibleName: root.accessibleName
        model: root.tiles
        onActivated: index => {
            var tile = root.tiles[index];
            if (tile && tile.action !== "")
                root.shelf.act(tile.id, tile.action);
        }
        onCurrentIndexChanged: {
            if (root.refreshing || currentIndex < 0 || currentIndex >= root.tiles.length)
                return;
            if (gallery.activeFocus)
                root.picked = true;
            root.selectedId = root.tiles[currentIndex].id;
        }

        delegate: DesignTile {
            required property var modelData
            required property int index

            width: gallery.tileWidth
            height: gallery.tileHeight
            focusPolicy: Qt.NoFocus
            title: modelData.title
            status: modelData.status
            resource: modelData.resource
            source: modelData.source
            current: modelData.current
            unavailable: modelData.unavailable
            busy: modelData.busy
            actionText: modelData.actionText
            actionKind: modelData.actionKind
            selected: index === gallery.currentIndex
            focusVisible: selected && gallery.visualFocus
            onClicked: {
                gallery.forceActiveFocus(Qt.MouseFocusReason);
                gallery.select(index);
            }
            onActionTriggered: {
                root.picked = true;
                gallery.select(index);
                root.shelf.act(modelData.id, modelData.action);
            }
        }
    }

    Label {
        width: parent.width
        visible: text.length > 0
        text: root.shelf.describe(root.shelf.card(root.selectedId))
        typeRole: "body"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    Plaque {
        visible: root.tryText.length > 0
        kind: "quiet"
        small: true
        glyph: "visibility"
        text: root.tryText
        onClicked: root.tryRequested()
    }

    Label {
        width: parent.width
        visible: root.ownerNote.length > 0
        text: root.ownerNote
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    Column {
        id: progress
        visible: root.shelf.slow
        width: parent.width
        spacing: LithicTokens.space.s2
        // Cancel hides once the install ends; the keys go back to the designs, not into nothing
        onVisibleChanged: {
            if (!progress.visible && stop.activeFocus) {
                gallery.forceActiveFocus(stop.visualFocus ? Qt.TabFocusReason : Qt.MouseFocusReason);
                Qt.callLater(root.showSelected);
            }
        }

        BladeProgress {
            width: parent.width
            active: root.shelf.slow
            indeterminate: true
        }
        Row {
            width: parent.width
            spacing: LithicTokens.space.s3

            Label {
                anchors.verticalCenter: parent.verticalCenter
                width: Math.min(implicitWidth, parent.width - stop.width - LithicTokens.space.s3)
                text: {
                    var c = root.shelf.card(root.shelf.busyId);
                    var name = c ? c.name : root.shelf.busyId;
                    return root.shelf.busyVerb === "update" ? I18n.tr("Still updating %1").arg(name) : I18n.tr("Still installing %1").arg(name);
                }
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            Plaque {
                id: stop
                kind: "quiet"
                small: true
                text: I18n.tr("Cancel")
                onClicked: root.shelf.cancel()
            }
        }
    }

    Label {
        id: stoppedNote
        width: parent.width
        visible: !!root.shelf.stopped && !root.shelf.slow && !root.shelf.failed
        text: {
            var s = root.shelf.stopped;
            if (!s)
                return "";
            var c = root.shelf.card(s.id);
            var name = c ? c.name : s.id;
            return (s.verb === "update" ? I18n.tr("The update of %1 stopped before it finished.") : I18n.tr("The install of %1 stopped before it finished.")).arg(name)
                + " " + I18n.tr("The designs above show what is on this computer now.");
        }
        typeRole: "detail"
        inkRole: "fg2"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    Row {
        id: failure
        visible: !!root.shelf.failed
        width: parent.width
        spacing: LithicTokens.space.s3
        onVisibleChanged: {
            if (!failure.visible && retry.activeFocus) {
                gallery.forceActiveFocus(retry.visualFocus ? Qt.TabFocusReason : Qt.MouseFocusReason);
                Qt.callLater(root.showSelected);
            }
        }

        Label {
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, parent.width - retry.width - LithicTokens.space.s3)
            text: {
                var f = root.shelf.failed;
                if (!f)
                    return "";
                var c = root.shelf.card(f.id);
                var name = c ? c.name : f.id;
                // a missing tool says what to do; the lead names what failed
                if (f.missing)
                    return (f.verb === "update" ? I18n.tr("Could not update %1.") : I18n.tr("Could not install %1.")).arg(name) + " " + f.message;
                var why = f.message ? f.message.replace(/[.\s]+$/, "") : "";
                if (f.verb === "update")
                    return why ? I18n.tr("Could not update %1: %2. Check the connection, then try again.").arg(name).arg(why)
                               : I18n.tr("Could not update %1. Check the connection, then try again.").arg(name);
                return why ? I18n.tr("Could not install %1: %2. Check the connection, then try again.").arg(name).arg(why)
                           : I18n.tr("Could not install %1. Check the connection, then try again.").arg(name);
            }
            typeRole: "detail"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
        Plaque {
            id: retry
            kind: "quiet"
            small: true
            glyph: "refresh"
            text: I18n.tr("Retry")
            onClicked: root.shelf.install(root.shelf.failed.id, root.shelf.failed.verb)
        }
    }
}
