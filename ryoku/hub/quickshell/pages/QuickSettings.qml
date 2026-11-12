import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../Singletons"
import "../components"
import "../components/quick-settings"
import "../schema/quick-settings.js" as Schema

// the Super+Escape panel: its blocks and their order, its design, where it
// opens, and the cards add-ons bring to it
Item {
    id: root

    required property var hub
    property string pendingRow: ""
    property Item blockList: null

    readonly property var schema: ({
        title: Schema.title,
        lead: Schema.lead,
        sections: Schema.sections,
        related: Schema.related,
        rows: Schema.rows.filter(r => r.template !== true)
    })

    readonly property string stateLine: {
        var where = { "top-left": I18n.tr("the top left"), "top-center": I18n.tr("the top centre"), "top-right": I18n.tr("the top right"), "center": I18n.tr("the centre") };
        var pos = Changes.value(Schema.row("position").bind);
        var gap = Changes.value(Schema.row("edge-gap").bind);
        var shown = 0;
        for (var i = 0; i < Schema.blocks.length; i++)
            if (Changes.value({ store: "quicksettings", key: "block:" + Schema.blocks[i].id }) === true)
                shown++;
        var line = I18n.tr("Opens at %1, %2 px from the edges.").arg(where[pos] || where["top-left"]).arg(gap);
        return line + " " + I18n.tr("Shows %1 of %2 blocks.").arg(shown).arg(Schema.blocks.length);
    }

    function own(id) {
        var r = Schema.row(id);
        return !!r && r.template === true;
    }

    function focusRow(rowId) {
        if (!rowId)
            return;
        if (!root.own(rowId)) {
            page.focusRow(rowId);
            return;
        }
        root.pendingRow = rowId;
        landRetry.tries = 0;
        landRetry.restart();
    }

    // a bespoke row exists once its section is built; a part waits for its block to open
    Timer {
        id: landRetry
        property int tries: 0
        interval: 60
        repeat: true
        onTriggered: {
            var id = root.pendingRow;
            var row = Schema.row(id);
            if (row && row.part && root.blockList && root.blockList.reveal(row.block)) {
                interval = LithicTokens.motion.unfold + 40;
                return;
            }
            interval = 60;
            if (page.rowItems[id]) {
                stop();
                root.pendingRow = "";
                page.land(id);
            } else if (++tries > 30) {
                stop();
            }
        }
    }

    Component.onCompleted: {
        Changes.ensureStore("quicksettings", Qt.resolvedUrl("../components/quick-settings/QuickSettingsStore.qml"));
        var store = Changes.stores["quicksettings"];
        if (store)
            store.refreshCards();
    }

    SettingsPage {
        id: page
        anchors.fill: parent
        hub: root.hub
        pageId: "quick-settings"
        schema: root.schema
        stateLine: root.stateLine
        bodies: ({ blocks: blocksBody, design: designBody })
        footers: ({ cards: cardsBody })
        onLayoutChanged: {
            if (root.pendingRow !== "" && !landRetry.running) {
                var id = root.pendingRow;
                root.pendingRow = "";
                Qt.callLater(root.focusRow, id);
            }
        }
    }

    Component {
        id: blocksBody
        Column {
            spacing: LithicTokens.space.s2
            Label {
                width: parent.width
                text: I18n.tr("Hidden blocks are not built at all, so they cost nothing while the panel is open. Alt with the arrow keys moves the block that has focus.")
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            BlockList {
                id: blocks
                width: parent.width
                host: page
                Component.onCompleted: root.blockList = blocks
                Component.onDestruction: if (root.blockList === blocks) root.blockList = null
            }
        }
    }

    Component {
        id: designBody
        Column {
            id: designs
            // search lands on the gallery
            readonly property Item settingRow: chooser
            spacing: LithicTokens.space.s3
            Component.onCompleted: page.rowItems["panel-design"] = designs
            Component.onDestruction: if (page.rowItems["panel-design"] === designs) delete page.rowItems["panel-design"]

            DesignShelf {
                id: shelf
                category: "sidebars"
                settingKey: "designs.sidebar"
                changeLabel: "Quick settings design"
                pageId: "quick-settings"
                useNotice: I18n.tr("Quick settings now uses %1")
            }
            DesignChooser {
                id: chooser
                width: parent.width
                shelf: shelf
                accessibleName: I18n.tr("Quick settings designs")
            }
        }
    }

    Component {
        id: cardsBody
        Column {
            spacing: LithicTokens.space.s2
            Label {
                width: parent.width
                text: I18n.tr("The cards show on the Extensions page in this order. Alt with the arrow keys moves the card that has focus.")
                typeRole: "detail"
                inkRole: "fg2"
                wrapMode: Text.Wrap
                elide: Text.ElideNone
            }
            CardOrder {
                width: parent.width
                host: page
                hub: root.hub
            }
        }
    }
}
