import QtQuick
import Lithic
import Ryoku.Ui.Singletons

// a searchable list of choices; Enter or a click takes one
Dialog {
    id: root

    // [{ value, label }]
    property var options: []
    property bool fontPreview: false
    property string query: ""
    property var chosen
    // why the choices could not be read, in plain words; it offers Retry
    property string failure: ""

    signal picked(var value)
    signal retry()

    readonly property var shown: {
        var q = root.query.trim().toLowerCase();
        if (q === "")
            return root.options;
        return root.options.filter(o => o.label.toLowerCase().indexOf(q) >= 0 || String(o.value).toLowerCase().indexOf(q) >= 0);
    }

    function indexOfChosen() {
        for (var i = 0; i < root.shown.length; i++)
            if (root.shown[i].value === root.chosen)
                return i;
        return root.shown.length > 0 ? 0 : -1;
    }

    function take() {
        var i = list.currentIndex;
        if (i >= 0 && i < root.shown.length)
            root.picked(root.shown[i].value);
    }

    actions: root.failure !== ""
             ? [{ text: I18n.tr("Cancel"), kind: "quiet" }, { text: I18n.tr("Retry"), kind: "quiet", glyph: "refresh", closeOnTrigger: false }]
             : [{ text: I18n.tr("Cancel"), kind: "quiet" }]
    onActionTriggered: index => {
        if (index === 1 && root.failure !== "")
            root.retry();
    }
    onAboutToShow: {
        search.text = "";
        list.currentIndex = root.indexOfChosen();
    }
    onOpened: {
        list.listView.positionViewAtIndex(Math.max(0, list.currentIndex), ListView.Center);
        search.forceActiveFocus();
    }
    onShownChanged: list.currentIndex = root.indexOfChosen()

    Column {
        width: parent.width
        spacing: LithicTokens.space.s3

        Search {
            id: search
            width: parent.width
            placeholderText: I18n.tr("Search")
            accessibleName: I18n.tr("Search %1").arg(root.title)
            // the clear plaque and Escape empty it without an edit, so follow every change
            onTextChanged: root.query = text
            Keys.onUpPressed: list.select(Math.max(0, list.currentIndex - 1))
            Keys.onDownPressed: list.select(Math.min(root.shown.length - 1, list.currentIndex + 1))
            Keys.onReturnPressed: root.take()
            Keys.onEnterPressed: root.take()
        }

        RowList {
            id: list
            width: parent.width
            height: Math.min(LithicTokens.size.listMedium, Math.max(LithicTokens.size.menuItem, root.shown.length * (LithicTokens.size.menuItem + LithicTokens.size.rowListGap)))
            visible: root.shown.length > 0
            model: root.shown
            Keys.onReturnPressed: root.take()
            Keys.onEnterPressed: root.take()
            delegate: Item {
                id: choice
                required property var modelData
                required property int index
                width: ListView.view.width
                height: LithicTokens.size.menuItem

                Label {
                    anchors.left: parent.left
                    anchors.leftMargin: LithicTokens.space.s3
                    anchors.right: check.left
                    anchors.rightMargin: LithicTokens.space.s2
                    anchors.verticalCenter: parent.verticalCenter
                    text: choice.modelData.label
                    typeRole: "body"
                    ink: choice.ListView.isCurrentItem ? Lithic.onSel : Lithic.fg
                    font.family: root.fontPreview ? choice.modelData.value : Lithic.type.body.family
                }
                Glyph {
                    id: check
                    anchors.right: parent.right
                    anchors.rightMargin: LithicTokens.space.s3
                    anchors.verticalCenter: parent.verticalCenter
                    visible: choice.modelData.value === root.chosen
                    name: "check"
                    size: LithicTokens.size.pickerArrow
                    ink: choice.ListView.isCurrentItem ? Lithic.onSel : Lithic.fg2
                }
                TapHandler {
                    onTapped: {
                        list.currentIndex = choice.index;
                        root.take();
                    }
                }
                HoverHandler {
                    cursorShape: Qt.PointingHandCursor
                }
            }
        }

        Label {
            visible: root.shown.length === 0 && root.query !== ""
            width: parent.width
            text: I18n.tr("Nothing matches “%1”").arg(root.query)
            typeRole: "detail"
            inkRole: "fg3"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }

        Label {
            visible: root.failure !== ""
            width: parent.width
            text: root.failure
            typeRole: "detail"
            ink: Lithic.jasper
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }
    }
}
