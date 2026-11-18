pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons

Item {
    id: root
    required property var panel
    property var anchorItem: null
    property real s: 1
    readonly property bool popupOpen: popup.visible
    readonly property var entries: {
        const image = panel.card.kind !== "text";
        const available = !!panel.session.sessionId && !panel.session.busy;
        const rows = [
            { key: "keep", label: I18n.tr("Keep on screen"), glyph: "picture_in_picture" },
            { key: "edit", label: I18n.tr("Open in editor"), glyph: "edit", disabled: !image },
            { key: "image", label: I18n.tr("Copy image"), glyph: "image", disabled: !image },
            { divider: true },
            { key: "continue", label: I18n.tr("Continue in Rashin"), glyph: "open_in_new", disabled: !available },
            { key: "handoff", label: I18n.tr("Hand off to an agent"), glyph: "arrow_forward", disabled: !available }
        ];
        if (panel.mode === "text") {
            rows.push({ divider: true });
            rows.push({ key: "breaks", label: I18n.tr("Keep line breaks"), checked: panel.keepBreaks });
            rows.push({ key: "read", label: I18n.tr("Read with Rashin"), glyph: "text_fields", disabled: panel.session.busy });
        }
        if (panel.catalog.feature("quiet") || panel.catalog.feature("save")) {
            rows.push({ divider: true });
            if (panel.catalog.feature("save"))
                rows.push({ key: "save", label: I18n.tr("Save to Rashin"), glyph: "bookmark_add", disabled: !String(panel.resultText || "").trim().length || panel.busy });
            if (panel.catalog.feature("quiet"))
                rows.push({ key: "quiet", label: I18n.tr("Not now in %1").arg(String(panel.card.app || I18n.tr("this app"))), glyph: "notifications_paused" });
        }
        return rows;
    }
    function toggle() { if (popup.visible) popup.close(); else popup.open(); }
    function close() { popup.close(); }
    function fire(index) {
        const key = entries[index].key;
        popup.close();
        if (key === "keep") panel.keep();
        else if (key === "edit") panel.openEditor();
        else if (key === "image") panel.copyImage();
        else if (key === "continue") panel.session.continueInRashin();
        else if (key === "handoff") panel.session.handoff();
        else if (key === "breaks") panel.keepBreaks = !panel.keepBreaks;
        else if (key === "read") panel.readWithRashin();
        else if (key === "quiet") panel.notNow();
        else if (key === "save") {
            const action = panel.catalog.action(panel.action);
            panel.save(panel.resultText, action ? action.label : I18n.tr("Theia"));
        }
    }
    Popover {
        id: popup
        parent: root.anchorItem ? root.anchorItem.parent : root.parent
        anchorItem: root.anchorItem; opener: root.anchorItem
        closePolicy: T.Popup.CloseOnEscape | T.Popup.CloseOnPressOutsideParent
        implicitWidth: Math.min(290 * root.s, Math.max(1, root.panel.usable.width
                                  - LithicTokens.size.popoverAnchorGap * 2 * root.s))
        implicitHeight: menu.implicitHeight
        function calculatePlacement() {
            return root.panel.popupPlacement(root.anchorItem, implicitWidth, implicitHeight, parent);
        }
        Menu {
            id: menu
            readonly property real lithicScale: root.s
            menuWidth: popup.implicitWidth
            items: root.entries
            onTriggered: index => root.fire(index)
        }
        onOpened: Qt.callLater(function() { menu.focusItem(-1, false); })
    }
}
