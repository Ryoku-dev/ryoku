import QtQuick
import Lithic
import Ryoku.Ui.Singletons
import "keymap.js" as Keymap

// a long list of layouts or styles to choose one from, with a search field
Dialog {
    id: chooser

    // [{ code, name }]
    property var options: []
    property string current: ""
    property var chooseFn: null
    property string query: ""
    readonly property var shown: Keymap.filter(chooser.options, chooser.query)

    // focus goes back to opener when the chooser closes
    function openWith(title, options, current, choose, opener) {
        chooser.title = title;
        chooser.options = options;
        chooser.current = current;
        chooser.chooseFn = choose;
        chooser.opener = opener || null;
        chooser.query = "";
        search.text = "";
        chooser.open();
    }

    function choose(index) {
        var e = chooser.shown[index];
        if (!e)
            return;
        var fn = chooser.chooseFn;
        chooser.close();
        if (fn)
            fn(e.code);
    }

    function selectCurrent() {
        var at = chooser.shown.findIndex(e => e.code === chooser.current);
        list.select(Math.max(0, at));
        list.listView.positionViewAtIndex(Math.max(0, at), ListView.Center);
    }

    dialogWidth: 520
    actions: [{ text: I18n.tr("Cancel"), kind: "quiet" }]
    onOpened: {
        Qt.callLater(chooser.selectCurrent);
        search.forceActiveFocus();
    }
    onShownChanged: list.select(0)

    Column {
        width: parent.width
        spacing: LithicTokens.space.s3

        Search {
            id: search
            width: parent.width
            placeholderText: I18n.tr("Search by name or code")
            accessibleName: I18n.tr("Search %1").arg(chooser.title)
            // also on a clear (Escape, the clear button), which changes the text without an edit
            onTextChanged: chooser.query = search.text
            onAccepted: chooser.choose(list.currentIndex)
            Keys.onDownPressed: list.select(Math.min(chooser.shown.length - 1, list.currentIndex + 1))
            Keys.onUpPressed: list.select(Math.max(0, list.currentIndex - 1))
        }

        RowList {
            id: list
            width: parent.width
            height: Math.min(360, Math.max(LithicTokens.size.factsRowHeight, chooser.shown.length * (LithicTokens.size.factsRowHeight + LithicTokens.size.rowListGap)))
            model: chooser.shown
            Accessible.name: chooser.title
            Keys.onReturnPressed: chooser.choose(list.currentIndex)
            Keys.onEnterPressed: chooser.choose(list.currentIndex)
            delegate: Item {
                id: entry
                required property var modelData
                required property int index
                width: ListView.view.width - LithicTokens.size.scrollThumb - LithicTokens.space.s1
                height: LithicTokens.size.factsRowHeight
                Accessible.role: Accessible.ListItem
                Accessible.name: entry.modelData.name

                Label {
                    anchors.left: parent.left
                    anchors.leftMargin: LithicTokens.space.s3
                    anchors.right: code.left
                    anchors.rightMargin: LithicTokens.space.s3
                    anchors.verticalCenter: parent.verticalCenter
                    text: entry.modelData.name
                    typeRole: "row"
                    ink: entry.ListView.isCurrentItem ? Lithic.onSel : Lithic.fg
                }
                Label {
                    id: code
                    anchors.right: mark.left
                    anchors.rightMargin: LithicTokens.space.s2
                    anchors.verticalCenter: parent.verticalCenter
                    text: entry.modelData.code
                    typeRole: "monoSm"
                    ink: entry.ListView.isCurrentItem ? Lithic.onSel2 : Lithic.fg3
                }
                Glyph {
                    id: mark
                    anchors.right: parent.right
                    anchors.rightMargin: LithicTokens.space.s3
                    anchors.verticalCenter: parent.verticalCenter
                    width: entry.modelData.code === chooser.current ? implicitWidth : 0
                    visible: entry.modelData.code === chooser.current
                    name: "check"
                    ink: entry.ListView.isCurrentItem ? Lithic.onSel : Lithic.fg2
                }
                MouseArea {
                    anchors.fill: parent
                    cursorShape: Qt.PointingHandCursor
                    onClicked: chooser.choose(entry.index)
                }
            }
        }

        Label {
            visible: chooser.shown.length === 0
            width: parent.width
            text: I18n.tr("Nothing matches %1").arg(chooser.query)
            typeRole: "detail"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }
}
