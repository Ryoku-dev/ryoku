pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Templates as T
import Lithic
import Ryoku.Ui.Singletons

Item {
    id: root
    required property var panel
    property real s: 1
    property string source: ""
    property string sourceOverride: ""
    property string target: ""
    readonly property bool popupOpen: languages.visible
    property bool choosingSource: false
    property string query: ""
    readonly property var choices: {
        const value = query.trim().toLowerCase();
        const automatic = choosingSource ? I18n.tr("Detect automatically") : I18n.tr("Your language");
        const selected = choosingSource ? sourceOverride : target;
        const rows = [{ label: automatic, code: "", glyph: selected === "" ? "check" : "" }];
        for (var i = 0; i < I18n.langs.length; ++i) {
            const language = I18n.langs[i];
            const name = String(language.name);
            const nativeName = String(language.native || "");
            const code = String(language.code);
            if (value && name.toLowerCase().indexOf(value) < 0 && nativeName.toLowerCase().indexOf(value) < 0 && code.toLowerCase().indexOf(value) < 0)
                continue;
            const label = I18n.tr(name);
            rows.push({ label: label, shortcut: nativeName && nativeName !== label ? nativeName : "", code: code, glyph: selected === code ? "check" : "" });
        }
        return rows;
    }
    readonly property string sourceName: source ? I18n.tr(panel.languageName(source)) : I18n.tr("Detect language")
    readonly property string targetName: target ? I18n.tr(panel.languageName(target)) : I18n.tr("Your language")
    signal sourceRequested(string source)
    signal targetRequested(string target)
    implicitHeight: 32 * s
    function close() { languages.close(); }
    function open(sourceMenu) {
        if (languages.visible && choosingSource === sourceMenu) { languages.close(); return; }
        languages.close();
        choosingSource = sourceMenu;
        query = "";
        panel.preparePopup("language");
        languages.open();
    }
    Row {
        width: parent.width
        spacing: 8 * root.s
        SightActionButton {
            id: sourceChip
            s: root.s; text: root.sourceName
            width: Math.max(1, (root.width - arrow.width - 16 * root.s) / 2)
            contentItem: Item {
                Label { anchors.left: parent.left; anchors.right: sourceChevron.left; anchors.rightMargin: 6 * root.s; anchors.verticalCenter: parent.verticalCenter; text: sourceChip.text; typeRole: "label"; inkRole: "fg2"; elide: Text.ElideRight }
                Glyph { id: sourceChevron; anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter; name: "expand_more"; size: 17 * root.s; ink: Lithic.fg2 }
            }
            onClicked: root.open(true)
        }
        Glyph { id: arrow; anchors.verticalCenter: parent.verticalCenter; name: "arrow_forward"; size: 17 * root.s; ink: Lithic.fg3 }
        SightActionButton {
            id: targetChip
            s: root.s; text: root.targetName
            width: Math.max(1, (root.width - arrow.width - 16 * root.s) / 2)
            contentItem: Item {
                Label { anchors.left: parent.left; anchors.right: targetChevron.left; anchors.rightMargin: 6 * root.s; anchors.verticalCenter: parent.verticalCenter; text: targetChip.text; typeRole: "label"; inkRole: "fg2"; elide: Text.ElideRight }
                Glyph { id: targetChevron; anchors.right: parent.right; anchors.verticalCenter: parent.verticalCenter; name: "expand_more"; size: 17 * root.s; ink: Lithic.fg2 }
            }
            onClicked: root.open(false)
        }
    }
    Popover {
        id: languages
        parent: root
        anchorItem: root.choosingSource ? sourceChip : targetChip
        opener: root.choosingSource ? sourceChip : targetChip
        menuFocus: false
        closePolicy: T.Popup.CloseOnEscape | T.Popup.CloseOnPressOutsideParent
        width: Math.min(320 * root.s, Math.max(1, root.panel.usable.width - 16 * root.s))
        height: Math.min(300 * root.s, root.panel.usable.height * 0.6)
        function calculatePlacement() { return root.panel.popupPlacement(anchorItem, width, height, root); }
        onOpened: { placeAtAnchor(); Qt.callLater(function() { if (contents.item) contents.item.focusSearch(); }); }
        background: CutRect { cut: 8; fill: Lithic.stone4; strokeColor: Lithic.vein; strokeWidth: 1; transform: Translate { y: languages.slideOffset } }
        contentItem: Loader {
            id: contents
            transform: Translate { y: languages.slideOffset }
            active: languages.visible
            sourceComponent: Component {
                Column {
                    width: languages.width
                    readonly property real lithicScale: root.s
                    spacing: 6 * root.s
                    function focusSearch() { search.forceActiveFocus(); }
                    TextField {
                        id: search
                        width: parent.width; height: 44 * root.s
                        font.pixelSize: 14 * root.s
                        placeholderText: I18n.tr("Find a language")
                        text: root.query
                        onTextEdited: root.query = text
                    }
                    Flickable {
                        width: parent.width; height: Math.max(1, languages.height - 50 * root.s)
                        contentWidth: width; contentHeight: menu.implicitHeight
                        clip: true; boundsBehavior: Flickable.StopAtBounds
                        Menu {
                            id: menu
                            width: parent.width
                            items: root.choices
                            onTriggered: index => {
                                const code = String(root.choices[index].code);
                                if (root.choosingSource) root.sourceRequested(code);
                                else root.targetRequested(code);
                                languages.close();
                            }
                        }
                    }
                }
            }
        }
    }
}
