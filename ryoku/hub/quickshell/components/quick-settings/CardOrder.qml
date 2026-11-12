import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "../../Singletons"
import "../../components"
import "../desktop-dock"

// the cards add-ons bring to the panel, in the order the panel shows them;
// which add-ons show a card is chosen on Add-ons
Slab {
    id: root

    required property Item host
    required property var hub
    property string pageId: "quick-settings"

    readonly property var store: Changes.stores["quicksettings"] || null
    readonly property var cardsBind: ({ store: "quicksettings", key: "cards" })
    readonly property var cards: root.store ? root.store.cards : []

    function move(index, by) {
        var ids = root.cards.map(c => c.id);
        var to = index + by;
        if (index < 0 || to < 0 || to >= ids.length)
            return;
        var id = ids.splice(index, 1)[0];
        ids.splice(to, 0, id);
        var name = root.cards[index].label;
        Changes.set(root.cardsBind, ids.join(","), I18n.tr("Extension card order"), root.pageId,
                    by < 0 ? I18n.tr("Moved %1 up").arg(name) : I18n.tr("Moved %1 down").arg(name));
        Qt.callLater(root.refocus, id);
    }

    function refocus(id) {
        var at = root.cards.findIndex(c => c.id === id);
        var item = at >= 0 ? cardRepeater.itemAt(at) : null;
        if (item)
            item.focusRow();
    }

    // the card that last held the keyboard here: a reorder or its undo rereads the add-ons and
    // rebuilds every card, which drops focus to the window, so the card gets it back after
    property string keptCard: ""
    function ownerCard(item) {
        for (var it = item; it && it !== root; it = it.parent)
            if (it.cardId !== undefined)
                return it.cardId;
        return "";
    }
    function giveBack() {
        var f = root.Window.activeFocusItem;
        if (root.keptCard !== "" && !(f && f.activeFocusOnTab))
            root.refocus(root.keptCard);
    }
    onCardsChanged: Qt.callLater(root.giveBack)
    Connections {
        target: root.Window
        function onActiveFocusItemChanged() {
            var f = root.Window.activeFocusItem;
            // focus passing through the window mid-rebuild keeps the card
            if (f && f.activeFocusOnTab)
                root.keptCard = root.ownerCard(f);
        }
    }

    Column {
        width: parent.width

        OwnRow {
            host: root.host
            rowId: root.cards.length === 0 ? "extension-cards" : ""
            first: true
            visible: root.cards.length === 0
            title: I18n.tr("Order of extension cards")
            detail: !root.store || !root.store.cardsLoaded ? I18n.tr("Reading your add-ons")
                : root.store.cardsError !== "" ? ""
                : I18n.tr("No add-on brings a card yet. When one does, it shows here.")
        }

        Repeater {
            id: cardRepeater
            model: root.cards

            delegate: Column {
                id: entry
                required property var modelData
                required property int index
                readonly property string cardId: entry.modelData.id
                function focusRow() { cardRow.forceActiveFocus(Qt.TabFocusReason); }
                width: parent.width

                OwnRow {
                    id: cardRow
                    host: root.host
                    rowId: entry.index === 0 ? "extension-cards" : ""
                    first: entry.index === 0
                    title: entry.modelData.label
                    detail: entry.modelData.summary
                    focusPolicy: Qt.StrongFocus
                    Keys.onPressed: event => {
                        if (!(event.modifiers & Qt.AltModifier))
                            return;
                        if (event.key === Qt.Key_Up) {
                            root.move(entry.index, -1);
                            event.accepted = true;
                        } else if (event.key === Qt.Key_Down) {
                            root.move(entry.index, 1);
                            event.accepted = true;
                        }
                    }

                    IconPlaque {
                        anchors.verticalCenter: parent.verticalCenter
                        kind: "quiet"
                        small: true
                        glyph: "arrow_upward"
                        enabled: entry.index > 0 && !root.store.placing
                        accessibleName: I18n.tr("Move %1 up").arg(entry.modelData.label)
                        onClicked: root.move(entry.index, -1)
                    }
                    IconPlaque {
                        anchors.verticalCenter: parent.verticalCenter
                        kind: "quiet"
                        small: true
                        glyph: "arrow_downward"
                        enabled: entry.index < root.cards.length - 1 && !root.store.placing
                        accessibleName: I18n.tr("Move %1 down").arg(entry.modelData.label)
                        onClicked: root.move(entry.index, 1)
                    }
                }
            }
        }

        RowNote {
            width: parent.width
            text: root.store && root.store.cardsError !== "" ? root.store.cardsError : ""
            onRetry: root.store.refreshCards()
        }

        RowNote {
            width: parent.width
            text: Changes.error(root.cardsBind)
            onRetry: Changes.retry(root.cardsBind)
        }

        HandoffRow {
            id: chooseRow
            readonly property Item settingRow: chooseRow
            width: parent.width
            first: false
            glyph: "arrow_forward"
            text: I18n.tr("Choose which add-ons show a card")
            detail: I18n.tr("Turn add-ons on or off and choose where each one shows")
            onTriggered: root.hub.navigate("add-ons", "")
            Component.onCompleted: root.host.rowItems["choose-add-ons"] = chooseRow
        }
    }
}
