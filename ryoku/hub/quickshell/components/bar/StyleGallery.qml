import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// the bar style gallery: a click selects a tile, its plaque or Enter acts on it
Column {
    id: root

    required property var styles
    property string selectedId: ""
    // until the user picks a tile, the selection follows the style in use
    property bool picked: false
    readonly property var tiles: root.styles.tiles
    readonly property var selectedCard: root.styles.card(root.selectedId)

    signal act(string styleId, string action)
    // once per page open: where the tile in use sits, so the page can bring it into view
    signal revealActive(real top, real bottom)
    property bool revealed: false

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
        if (!root.picked && root.indexOf(root.styles.active) >= 0)
            root.selectedId = root.styles.active;
        else if (root.indexOf(root.selectedId) < 0)
            root.selectedId = root.tiles[0].id;
        gallery.currentIndex = root.indexOf(root.selectedId);
        if (!root.revealed && root.selectedId === root.styles.active && root.styles.wearable(root.styles.card(root.selectedId))) {
            root.revealed = true;
            var row = Math.floor(gallery.currentIndex / gallery.columns);
            var top = gallery.y + row * (gallery.tileHeight + gallery.gap);
            Qt.callLater(root.revealActive, top, top + gallery.tileHeight);
        }
    }
    readonly property string activeId: root.styles.active
    onActiveIdChanged: root.syncSelection()
    onTilesChanged: {
        root.refreshing = true;
        Qt.callLater(root.syncSelection);
    }
    Component.onCompleted: root.syncSelection()

    function describe(c) {
        if (!c)
            return "";
        var parts = [];
        if (c.summary)
            parts.push(c.summary);
        if (c.category === "subshells")
            parts.push(I18n.tr("A complete desktop: its own bar, dock and panels"));
        var blocked = root.styles.blockedReason(c);
        if (c.unavailable && c.unavailableReason)
            parts.push(I18n.tr(c.unavailableReason));
        else if (c.downloadPaused && c.downloadPauseReason)
            parts.push(I18n.tr(c.downloadPauseReason));
        else if (blocked !== "")
            parts.push(blocked);
        if (root.styles.updatable(c))
            parts.push(I18n.tr("Version %1 is installed and %2 is ready").arg(c.installedVersion).arg(c.version));
        else if (c.installed && c.source === "shipped")
            parts.push(I18n.tr("Comes with Ryoku"));
        else if (c.installed && c.installedVersion)
            parts.push(I18n.tr("Version %1 from RyoStore").arg(c.installedVersion));
        else if (c.version)
            parts.push(I18n.tr("Version %1 in RyoStore").arg(c.version));
        return c.name + ": " + parts.filter(p => p.length > 0).map(p => p.replace(/[.\s]+$/, "")).join(". ") + ".";
    }

    DesignGallery {
        id: gallery
        width: parent.width
        accessibleName: I18n.tr("Bar styles")
        model: root.tiles
        onActivated: index => {
            var tile = root.tiles[index];
            if (tile && tile.action !== "")
                root.act(tile.id, tile.action);
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
            onActionTriggered: root.act(modelData.id, modelData.action)
        }
    }

    Label {
        width: parent.width
        visible: text.length > 0
        text: root.describe(root.selectedCard)
        typeRole: "body"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    Label {
        width: parent.width
        text: I18n.tr("Light, Standard and Heavy compare how much memory and processor time a style needs while it runs. Heavy styles cost more battery.")
        typeRole: "detail"
        inkRole: "fg3"
        wrapMode: Text.Wrap
        elide: Text.ElideNone
    }

    Row {
        visible: !!root.styles.failed
        width: parent.width
        spacing: LithicTokens.space.s3

        Label {
            id: failure
            anchors.verticalCenter: parent.verticalCenter
            width: Math.min(implicitWidth, parent.width - retry.width - LithicTokens.space.s3)
            text: {
                var f = root.styles.failed;
                if (!f)
                    return "";
                var c = root.styles.card(f.id);
                var name = c ? c.name : f.id;
                var lead = f.verb === "update" ? I18n.tr("Could not update %1").arg(name) : I18n.tr("Could not install %1").arg(name);
                var why = f.message ? f.message.replace(/[.\s]+$/, "") : "";
                return lead + (why ? ": " + why : "") + ". " + I18n.tr("Check the connection, then try again.");
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
            onClicked: root.styles.install(root.styles.failed.id, root.styles.failed.verb)
        }
    }
}
