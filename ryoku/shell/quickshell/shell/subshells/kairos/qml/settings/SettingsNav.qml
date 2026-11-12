pragma ComponentBehavior: Bound

import QtQuick
import Ryoku.Ui.Singletons
import "../kit"

// the list side of the list-detail layout: search on top, then the categories, or the matches while searching
Item {
    id: nav

    // [{ id, group, title, icon, hue, summary }]
    required property var categories
    // [{ page, key, title, words }]: every row on every page
    required property var entries
    property string route: ""
    property string routeKey: ""
    property alias query: search.text
    property alias searchField: search.field
    signal picked(string page, string key)
    signal searchLeft()

    readonly property var words: nav.query.toLowerCase().split(/\s+/).filter(function (w) { return w.length > 0; })
    readonly property var results: {
        if (nav.words.length === 0)
            return [];
        var out = [];
        for (var i = 0; i < nav.entries.length; i++) {
            var e = nav.entries[i];
            var cat = nav.category(e.page);
            var hay = (e.title + " " + cat.title + " " + e.words).toLowerCase();
            if (nav.words.every(function (w) { return hay.indexOf(w) >= 0; }))
                out.push({ page: e.page, key: e.key, title: e.title, section: cat.title, icon: cat.icon, hue: cat.hue });
        }
        return out;
    }

    function category(id) {
        for (var i = 0; i < nav.categories.length; i++)
            if (nav.categories[i].id === id)
                return nav.categories[i];
        return nav.categories[0];
    }
    function position(list, i) {
        if (list.length === 1)
            return "only";
        return i === 0 ? "first" : i === list.length - 1 ? "last" : "middle";
    }
    function group(n) {
        return nav.categories.filter(function (c) { return c.group === n; });
    }

    Label {
        id: heading
        anchors { top: parent.top; left: parent.left; right: parent.right; topMargin: 20; leftMargin: 12 }
        role: "headline"
        text: I18n.tr("Kairos settings")
    }

    SearchBar {
        id: search
        anchors { top: heading.bottom; left: parent.left; right: parent.right; topMargin: 14 }
        onEscaped: nav.searchLeft()
        onTextChanged: flick.contentY = 0
    }

    Flickable {
        id: flick
        anchors { top: search.bottom; bottom: parent.bottom; left: parent.left; right: parent.right; topMargin: 16 }
        contentWidth: width
        contentHeight: list.height + 16
        boundsBehavior: Flickable.StopAtBounds
        clip: true

        Column {
            id: list
            width: flick.width

            Column {
                width: parent.width
                visible: nav.words.length === 0
                spacing: 12

                Repeater {
                    model: [0, 1, 2]
                    delegate: Column {
                        id: groupColumn
                        required property int modelData
                        readonly property var members: nav.group(groupColumn.modelData)
                        width: parent.width
                        spacing: Md.seam

                        Repeater {
                            model: groupColumn.members
                            delegate: CategoryRow {
                                required property var modelData
                                required property int index
                                width: groupColumn.width
                                position: nav.position(groupColumn.members, index)
                                icon: modelData.icon
                                hue: modelData.hue
                                headline: modelData.title
                                supporting: modelData.summary
                                selected: nav.route === modelData.id
                                onClicked: nav.picked(modelData.id, "")
                            }
                        }
                    }
                }
            }

            Column {
                width: parent.width
                visible: nav.words.length > 0
                spacing: Md.seam

                Repeater {
                    model: nav.results
                    delegate: CategoryRow {
                        required property var modelData
                        required property int index
                        width: parent.width
                        position: nav.position(nav.results, index)
                        icon: modelData.icon
                        hue: modelData.hue
                        headline: modelData.title
                        supporting: modelData.section
                        selected: nav.route === modelData.page && nav.routeKey === modelData.key
                        onClicked: nav.picked(modelData.page, modelData.key)
                    }
                }

                EmptyState {
                    visible: nav.results.length === 0
                    icon: "search_off"
                    title: I18n.tr("No settings match")
                    body: I18n.tr("Try another word, like size, clock or weather")
                }
            }
        }
    }
}
