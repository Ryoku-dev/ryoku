pragma ComponentBehavior: Bound

import QtQuick
import QtQuick.Controls
import Ryoku.Ui
import Ryoku.Ui.Singletons
import ".."

Item {
    id: root

    property var kit: null
    property string query: ""
    property bool current: false
    readonly property int matches: root.query.length === 0 ? root.total : root.countMatches()
    readonly property var rail: root.makeRail()
    property string railCurrent: ""
    signal requestClose()

    property var categories: []
    property bool loaded: false
    readonly property int total: root.countBinds(root.categories)
    readonly property bool searching: root.query.length > 0

    function activate() {
        root.forceActiveFocus();
    }

    function handleEscape() {
        return false;
    }

    function usableCount(binds) {
        var count = 0;
        for (var i = 0; i < binds.length; i++) {
            if (!binds[i].unhonored)
                count++;
        }
        return count;
    }

    function countBinds(cats) {
        var count = 0;
        for (var i = 0; i < cats.length; i++)
            count += root.usableCount(cats[i].binds || []);
        return count;
    }

    function matchBind(bind, text) {
        if (bind.desc && bind.desc.toLowerCase().indexOf(text) >= 0)
            return true;
        if (bind.hint && bind.hint.toLowerCase().indexOf(text) >= 0)
            return true;
        if (bind.taps > 0 && I18n.tr("Tap %1 times").arg(bind.taps).toLowerCase().indexOf(text) >= 0)
            return true;
        if (bind.whileHighlighting && I18n.tr("Hold while highlighting").toLowerCase().indexOf(text) >= 0)
            return true;
        var keys = bind.keys ? bind.keys.join(" ").toLowerCase() : "";
        return keys.indexOf(text) >= 0;
    }

    function categoryMatches(category, text) {
        var binds = category.binds || [];
        if (!text || category.name.toLowerCase().indexOf(text) >= 0)
            return binds.length;
        var count = 0;
        for (var i = 0; i < binds.length; i++) {
            if (root.matchBind(binds[i], text))
                count++;
        }
        return count;
    }

    function countMatches() {
        var count = 0;
        for (var i = 0; i < root.categories.length; i++)
            count += root.categoryMatches(root.categories[i], root.query);
        return count;
    }

    function railId(index) {
        return "category-" + index;
    }

    function makeRail() {
        var result = [];
        for (var i = 0; i < root.categories.length; i++) {
            var category = root.categories[i];
            result.push({
                id: root.railId(i),
                label: category.name,
                count: root.searching
                    ? root.categoryMatches(category, root.query)
                    : root.usableCount(category.binds || [])
            });
        }
        return result;
    }

    function selectedIndex() {
        var match = /^category-(\d+)$/.exec(root.railCurrent);
        if (!match)
            return 0;
        return Math.max(0, Math.min(Number(match[1]), root.categories.length - 1));
    }

    readonly property var filtered: {
        var result = [];
        for (var i = 0; i < root.categories.length; i++) {
            var category = root.categories[i];
            var binds = category.binds || [];
            if (!root.query || category.name.toLowerCase().indexOf(root.query) >= 0) {
                result.push(category);
                continue;
            }
            var keep = [];
            for (var j = 0; j < binds.length; j++) {
                if (root.matchBind(binds[j], root.query))
                    keep.push(binds[j]);
            }
            if (keep.length > 0)
                result.push({ name: category.name, binds: keep });
        }
        return result;
    }

    readonly property var shownCategories: {
        if (root.searching)
            return root.filtered;
        if (root.categories.length === 0)
            return [];
        return [root.categories[root.selectedIndex()]];
    }

    function ensureRail() {
        if (root.categories.length === 0) {
            root.railCurrent = "";
            return;
        }
        var index = root.selectedIndex();
        var expected = root.railId(index);
        if (root.railCurrent !== expected)
            root.railCurrent = expected;
    }

    onCategoriesChanged: root.ensureRail()
    onRailCurrentChanged: body.contentY = 0
    onShownCategoriesChanged: body.contentY = 0

    Flickable {
        id: body
        anchors.fill: parent
        clip: true
        contentHeight: pane.implicitHeight
        boundsBehavior: Flickable.StopAtBounds
        interactive: contentHeight > height
        ScrollBar.vertical: ScrollRail {
            policy: body.contentHeight > body.height
                ? ScrollBar.AsNeeded : ScrollBar.AlwaysOff
        }
        WheelScroll { }

        Column {
            id: pane
            width: body.width - Tokens.s3
            spacing: Tokens.s6

            Repeater {
                model: root.shownCategories
                delegate: CategoryBlock {
                    required property var modelData
                    width: pane.width
                    name: modelData.name
                    binds: modelData.binds
                    searching: root.searching
                }
            }
        }
    }

    Text {
        anchors.centerIn: parent
        visible: root.loaded && root.shownCategories.length === 0
        text: I18n.tr("No shortcuts match “%1”").arg(root.query)
        color: Tokens.inkFaint
        font.family: Tokens.ui
        font.pixelSize: Tokens.fBody
    }
}
