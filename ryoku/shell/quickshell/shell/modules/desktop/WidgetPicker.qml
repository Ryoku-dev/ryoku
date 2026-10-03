pragma ComponentBehavior: Bound
import QtQuick
import QtQuick.Controls
import "Singletons"
import "../../components"
import Ryoku.Ui
import Ryoku.Ui.Singletons

// The Widgets picker: an attached panel that grows out of the Edit widgets bar.
// The whole roster is too long for one honest list (it was: a single strip of
// forty-plus rows you scrubbed), so the panel reads like a settings page: a
// category rail down the left (Ryoku, Shima, Python, each installed plugin set,
// with a live count) and a two-column grid of widget cards for the chosen
// category. A card carries the widget's glyph, name, one-line hint, an on/off
// dot, and (once on) a tune affordance that opens that widget's editor; the
// whole card toggles. Typing in the search field drops into a flat result grid
// across every category, each card wearing its category as an eyebrow, so a
// "Clock" hit always says which Clock it is. Same paper-and-ink surface as the
// free work area above the bar (maxWidth / maxPanelHeight) and the grid
// scrolls when a category overflows, so a name or hint never truncates.
//
// Keyboard: the host grants focus by taking it on click
// (WlrKeyboardFocus.OnDemand). Down from the search enters the grid; arrows
// move the highlight (the grid is two columns, so Up/Down step a row), Space
// or Enter toggles the highlighted card, Esc closes -- search first, then the
// panel, matching the session's escape ladder.
Rectangle {
    id: panel

    // The flat roster from Desktop.addItems: [{ id, label, icon, enabled, group }].
    property var items: []
    // Geometry the host allows: the free work area above the bar.
    property real maxWidth: 620
    property real maxPanelHeight: 600

    signal toggle(string id)
    signal requestClose()
    // Open the widget inspector (Customize sheet) for an enabled row.
    signal customize(string id)

    // Short descriptions under each name. The labels and icons live in the roster;
    // these one-line hints are presentation copy, so they live with the surface
    // that shows them, keyed by widget id (a built-in id or a roster prefix). A
    // plugin carries none, so its card is glyph + name + dot, nothing missing.
    readonly property var hints: ({
        "clock": "Time and date on the wallpaper",
        "calendar": "A month at a glance",
        "music": "Now playing, with controls",
        "aio": "Time, weather and media together",
        "stats": "Live CPU, memory and network",
        "weather": "Local conditions and forecast",
        "notes": "A quick scratch pad",
        "dayprogress": "How much of the day is left",
        "shape": "A plain decorative accent",
        "visualizer": "Audio spectrum on the desktop",
        "irisClock": "A Shima clock face",
        "irisWeather": "A Shima weather face",
        "irisMedia": "A Shima now-playing face",
        "irisControls": "Quick toggles as a face",
        "irisMonth": "A full month calendar",
        "irisAgenda": "Your next few events",
        "irisTodo": "Tasks and checklists",
        "irisNotes": "Shima sticky notes",
        "irisTimers": "Countdowns and a stopwatch",
        "irisScreen": "Time spent on screen today",
        "irisVitals": "CPU, memory and temperatures",
        "irisBattery": "Battery levels at a glance",
        "irisWorld": "Clocks around the world",
        "irisDate": "Just today's date",
        "irisProfile": "Your profile card",
        "irisUptime": "Time since last boot",
        "irisNews": "Scrolling headlines",
        "irisCustomImage": "A picture of your own",
        "irisEditorial": "A magazine-style headline",
        "irisConverter": "Convert image formats",
        "irisJp": "Vertical Japanese type",
        "irisVisualizer": "A Shima audio spectrum",
        "pythonVisualizer": "A Python audio spectrum",
        "pythonTime": "A Python clock face",
        "pythonMusic": "A Python now-playing face",
        "pythonWeather": "A Python weather face",
        "pythonImage": "A picture of your own",
        "pythonUser": "Your profile card",
        "pythonCpu": "Live CPU usage",
        "pythonRam": "Live memory usage",
        "pythonTemp": "Sensor temperatures",
        "pythonDisk": "Disk usage",
        "pythonBattery": "Battery at a glance",
        "pythonGithub": "Your contribution grid"
    })

    // The live on/off for an id, read from `items` rather than a card's snapshot
    // so a toggle re-tints its card (and flips its switch) without rebuilding.
    function isOn(id) {
        const it = panel.items || [];
        for (var i = 0; i < it.length; i++)
            if (it[i].id === id)
                return it[i].enabled === true;
        return false;
    }
    function hintFor(id) { return panel.hints[id] || ""; }

    // ── categories ──────────────────────────────────────────────────────────
    // The rail's entries, in roster order: the built-ins as "Ryoku widgets",
    // the vendored faces under their own captions, each plugin set as its own.
    // The list is derived from `items`, so a new set appears with no change
    // here and an empty set never shows.
    function computeCats() {
        const out = [];
        const src = panel.items || [];
        for (var i = 0; i < src.length; i++) {
            const g = src[i].group || "";
            var found = -1;
            for (var c = 0; c < out.length; c++)
                if (out[c].group === g) { found = c; break; }
            if (found < 0) {
                out.push({ group: g, caption: panel.captionFor(g), gloss: panel.glossFor(g),
                    eyebrow: panel.eyebrowFor(g), on: src[i].enabled === true ? 1 : 0,
                    total: 1 });
            } else {
                out[found].total += 1;
                if (src[i].enabled === true)
                    out[found].on += 1;
            }
        }
        return out;
    }
    function captionFor(g) { return g === "" ? "Ryoku widgets" : g; }
    // The kanji seal each caption wears (docs/ui-ux.md masthead idiom). The
    // built-ins keep 部品 (parts), the two vendored suites take their own.
    function glossFor(g) {
        if (g === "") return "部品";
        if (g === "Shima widgets") return "島";
        if (g === "Python widgets") return "蛇";
        return "";
    }
    // The short Latin eyebrow a search-result card wears so a hit says which
    // family it belongs to.
    function eyebrowFor(g) {
        if (g === "") return "RYOKU";
        if (g === "Shima widgets") return "SHIMA";
        if (g === "Python widgets") return "PYTHON";
        return g.toUpperCase();
    }

    readonly property var cats: panel.computeCats()
    onCatsChanged: {
        if (panel.curCat >= panel.cats.length)
            panel.curCat = 0;
        panel.recompute();
    }
    property int curCat: 0
    onCurCatChanged: {
        panel.curNav = -1;
        grid.contentY = 0;
        panel.recompute();
    }

    // ── the visible set: one category, or every match while searching ──────
    // The display set is recomputed on any roster, query or category change,
    // but the array reference only swaps when its *shape* changes (a widget
    // added/removed, the query narrowing the set, a new category). A plain
    // on/off toggle leaves the shape untouched, so the Repeater keeps its
    // cards, the scroll position and the highlight; each card reads its live
    // state through isOn(), which follows `items` reactively.
    property string query: ""
    onQueryChanged: {
        panel.curNav = -1;
        grid.contentY = 0;
        panel.recompute();
    }

    property var _cards: []
    property string _sig: ""
    function recompute() {
        const src = panel.items || [];
        const ql = (panel.query || "").trim().toLowerCase();
        const out = [];
        for (var i = 0; i < src.length; i++) {
            const e = src[i];
            const name = I18n.tr(e.label || e.id);
            if (ql.length > 0) {
                const hay = (name + " " + (e.id || "") + " " + panel.hintFor(e.id)).toLowerCase();
                if (hay.indexOf(ql) < 0)
                    continue;
            } else {
                const cur = panel.cats[panel.curCat];
                if (!cur || (e.group || "") !== cur.group)
                    continue;
            }
            out.push(e);
        }
        var sig = "";
        for (var j = 0; j < out.length; j++)
            sig += out[j].id + "|";
        if (sig !== panel._sig) {
            panel._sig = sig;
            panel._cards = out;
        }
    }
    readonly property var filtered: panel._cards
    onItemsChanged: panel.recompute()
    Component.onCompleted: panel.recompute()
    readonly property bool searching: (panel.query || "").trim().length > 0

    // The keyboard highlight, an index into `filtered`.
    property int curNav: -1
    function toggleCurrent() {
        const e = panel.filtered[panel.curNav];
        if (e)
            panel.toggle(e.id);
    }
    function moveNav(d) {
        const n = panel.filtered.length;
        if (n === 0)
            return;
        panel.curNav = panel.curNav < 0 ? (d > 0 ? 0 : n - 1)
            : Math.max(0, Math.min(n - 1, panel.curNav + d));
        panel.keepVisible();
    }
    // Keep the highlighted card inside the viewport when the keyboard moves it.
    function keepVisible() {
        const it = rep.itemAt(panel.curNav);
        if (!it)
            return;
        if (it.y < grid.contentY)
            grid.contentY = it.y;
        else if (it.y + it.height > grid.contentY + grid.height)
            grid.contentY = it.y + it.height - grid.height;
    }
    // Focus the search field for a fresh session whenever the panel appears.
    onVisibleChanged: {
        if (panel.visible) {
            search.text = "";
            panel.curNav = -1;
            grid.contentY = 0;
            search.forceActiveFocus();
        }
    }

    readonly property int pad: Theme.s3
    readonly property int searchH: 34
    readonly property int railW: 132
    readonly property int cardH: 104
    readonly property int cardGap: Theme.s2

    width: Math.min(620, panel.maxWidth)
    // The panel is tall enough for the grid it holds, capped by the work area;
    // an empty result still floors at one row so the "no match" line has room.
    readonly property real gridRows: Math.ceil(panel.filtered.length / 2)
    readonly property real gridDesired: panel.filtered.length === 0
        ? Theme.s7
        : panel.gridRows * panel.cardH + Math.max(0, panel.gridRows - 1) * panel.cardGap
    height: Math.min(panel.maxPanelHeight,
        Math.max(360, panel.pad + panel.searchH + Theme.s3 + panel.gridDesired + panel.pad * 2))
    radius: Theme.menuRadius
    color: Theme.surface
    border.width: 1
    border.color: Theme.line

    // ── search: filters as you type; Down drops into the grid, Esc closes ──
    Rectangle {
        id: searchBox
        anchors { top: parent.top; left: parent.left; right: parent.right; margins: panel.pad }
        height: panel.searchH
        radius: Theme.menuTileRadius
        color: "transparent"
        border.width: search.activeFocus ? 2 : 1
        border.color: search.activeFocus ? Theme.ink : (searchHover.hovered ? Theme.lineStrong : Theme.line)
        Behavior on border.color { ColorAnimation { duration: Theme.quick } }

        HoverHandler { id: searchHover; cursorShape: Qt.IBeamCursor }

        MaterialIcon {
            id: sIcon
            anchors { left: parent.left; leftMargin: Theme.s3; verticalCenter: parent.verticalCenter }
            text: "search"
            font.pixelSize: 18
            color: search.activeFocus ? Theme.ink : Theme.inkDim
        }
        TextInput {
            id: search
            anchors {
                left: sIcon.right; leftMargin: Theme.s2
                right: clearBtn.visible ? clearBtn.left : parent.right; rightMargin: Theme.s2
                verticalCenter: parent.verticalCenter
            }
            verticalAlignment: Text.AlignVCenter
            color: Theme.ink
            font.family: Theme.font
            font.pixelSize: Theme.fSmall
            selectByMouse: true
            clip: true
            onTextChanged: panel.query = text
            Keys.onDownPressed: e => {
                if (panel.filtered.length > 0) {
                    panel.curNav = 0;
                    grid.forceActiveFocus();
                    panel.keepVisible();
                }
                e.accepted = true;
            }
            Keys.onEscapePressed: e => { panel.requestClose(); e.accepted = true; }

            Text {
                anchors.fill: parent
                visible: search.text === ""
                text: I18n.tr("Search widgets")
                color: Theme.inkDim
                font: search.font
                verticalAlignment: Text.AlignVCenter
                elide: Text.ElideRight
            }
        }
        // A quiet clear affordance so the mouse-only path can reset the filter.
        MaterialIcon {
            id: clearBtn
            visible: search.text.length > 0
            anchors { right: parent.right; rightMargin: Theme.s3; verticalCenter: parent.verticalCenter }
            text: "close"
            font.pixelSize: 16
            color: clearMa.containsMouse ? Theme.ink : Theme.inkDim
            MouseArea {
                id: clearMa
                anchors.fill: parent
                anchors.margins: -Theme.s1
                hoverEnabled: true
                cursorShape: Qt.PointingHandCursor
                onClicked: { search.text = ""; search.forceActiveFocus(); }
            }
        }
    }

    // ── the category rail: one chip per family, count on the right ─────────
    // Hidden while searching: the result grid spans every family, so a rail
    // that highlights none would only waste width.
    Column {
        id: rail
        visible: !panel.searching && panel.cats.length > 1
        anchors {
            top: searchBox.bottom; topMargin: Theme.s3
            left: parent.left; leftMargin: panel.pad
            bottom: parent.bottom; bottomMargin: panel.pad
        }
        width: panel.railW
        spacing: Theme.s1

        Repeater {
            model: panel.cats
            delegate: Item {
                id: cat
                required property var modelData
                required property int index
                readonly property bool selected: index === panel.curCat
                width: rail.width
                height: Theme.s7
                scale: catMa.pressed ? 0.97 : 1
                Behavior on scale { NumberAnimation { duration: Theme.quick; easing.type: Theme.ease } }
                Rectangle {
                    anchors.fill: parent
                    radius: Theme.menuTileRadius
                    color: cat.selected ? Theme.bone
                        : catMa.pressed ? Theme.tilePress
                        : catMa.containsMouse ? Theme.tileHover : "transparent"
                    border.width: 1
                    border.color: cat.selected ? Theme.bone : Theme.line
                    Behavior on color { ColorAnimation { duration: Theme.quick } }
                }
                Row {
                    anchors {
                        left: parent.left; leftMargin: Theme.s3
                        right: parent.right; rightMargin: Theme.s3
                        verticalCenter: parent.verticalCenter
                    }
                    spacing: Theme.s1
                    Text {
                        anchors.verticalCenter: parent.verticalCenter
                        width: parent.width - glossText.implicitWidth - countText.implicitWidth - 2 * parent.spacing
                        text: I18n.tr(cat.modelData.caption)
                        color: cat.selected ? Theme.inkOnBone : Theme.inkSoft
                        font.family: Theme.font
                        font.pixelSize: Theme.fSmall
                        font.weight: cat.selected ? Font.DemiBold : Font.Medium
                        elide: Text.ElideRight
                        Behavior on color { ColorAnimation { duration: Theme.quick } }
                    }
                    Text {
                        id: glossText
                        anchors.verticalCenter: parent.verticalCenter
                        visible: text.length > 0
                        text: cat.modelData.gloss
                        color: cat.selected ? Theme.inkOnBone : Theme.faint
                        font.family: Theme.fontJp
                        font.pixelSize: Theme.fMicro
                    }
                    Text {
                        id: countText
                        anchors.verticalCenter: parent.verticalCenter
                        text: cat.modelData.on + "/" + cat.modelData.total
                        color: cat.selected ? Theme.inkOnBone : Theme.inkDim
                        font.family: Theme.mono
                        font.pixelSize: Theme.fMicro
                        font.weight: Font.DemiBold
                        Behavior on color { ColorAnimation { duration: Theme.quick } }
                    }
                }
                MouseArea {
                    id: catMa
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.PointingHandCursor
                    onClicked: {
                        panel.curCat = cat.index;
                        panel.curNav = -1;
                        grid.contentY = 0;
                    }
                }
            }
        }
    }

    // ── the grid: two columns of cards, natural vertical scroll ────────────
    Flickable {
        id: grid
        anchors {
            top: searchBox.bottom; topMargin: Theme.s3
            left: rail.visible ? rail.right : parent.left
            right: parent.right; bottom: parent.bottom
            leftMargin: rail.visible ? Theme.s3 : panel.pad
            rightMargin: panel.pad; bottomMargin: panel.pad
        }
        clip: true
        contentWidth: width
        contentHeight: cards.height
        boundsBehavior: Flickable.StopAtBounds

        WheelScroll {}
        ScrollBar.vertical: ScrollRail {}

        // The grid holds keyboard nav; the search field hands focus down with
        // Down. Two columns, so Up/Down step a row and Left/Right a card.
        Keys.onUpPressed: e => {
            if (panel.curNav <= 0) {
                panel.curNav = -1;
                search.forceActiveFocus();
            } else {
                panel.moveNav(-2);
            }
            e.accepted = true;
        }
        Keys.onDownPressed: e => { panel.moveNav(2); e.accepted = true; }
        Keys.onLeftPressed: e => { panel.moveNav(-1); e.accepted = true; }
        Keys.onRightPressed: e => { panel.moveNav(1); e.accepted = true; }
        Keys.onSpacePressed: e => { panel.toggleCurrent(); e.accepted = true; }
        Keys.onReturnPressed: e => { panel.toggleCurrent(); e.accepted = true; }
        Keys.onEnterPressed: e => { panel.toggleCurrent(); e.accepted = true; }
        Keys.onEscapePressed: e => { panel.requestClose(); e.accepted = true; }

        Grid {
            id: cards
            width: grid.width
            columns: 2
            columnSpacing: panel.cardGap
            rowSpacing: panel.cardGap

            Repeater {
                id: rep
                model: panel.filtered

                delegate: Rectangle {
                    id: card
                    required property var modelData
                    required property int index
                    readonly property var entry: card.modelData
                    readonly property bool on: panel.isOn(card.entry.id)
                    readonly property bool active: card.index === panel.curNav

                    width: (cards.width - panel.cardGap) / 2
                    height: panel.cardH
                    radius: Theme.menuTileRadius
                    // An on card sits on its tile; an off card is quiet paper
                    // with a hairline, so the lit ones read as the desktop's
                    // current cast at a glance.
                    color: card.active ? Theme.tilePress
                        : cardMa.containsMouse ? Theme.tileHover
                        : card.on ? Theme.tile : "transparent"
                    border.width: card.active ? 2 : 1
                    border.color: card.active ? Theme.ink
                        : card.on ? Theme.lineStrong : Theme.line
                    Behavior on color { ColorAnimation { duration: Theme.quick } }
                    Behavior on border.color { ColorAnimation { duration: Theme.quick } }

                    // category eyebrow, only where the family is not obvious:
                    // search results span every category.
                    Text {
                        id: eyebrow
                        anchors { top: parent.top; topMargin: Theme.s2; left: parent.left; leftMargin: Theme.s3 }
                        visible: panel.searching
                        text: panel.eyebrowFor(card.entry.group || "")
                        color: Theme.faint
                        font.family: Theme.font
                        font.pixelSize: Theme.fMicro
                        font.weight: Font.DemiBold
                        font.letterSpacing: Theme.trackMark
                    }

                    MaterialIcon {
                        id: cardGlyph
                        anchors { left: parent.left; leftMargin: Theme.s3; top: eyebrow.visible ? eyebrow.bottom : parent.top; topMargin: eyebrow.visible ? Theme.s1 : Theme.s3 }
                        text: card.entry.icon || "widgets"
                        font.pixelSize: 22
                        fill: card.on ? 1 : 0
                        color: card.on ? Theme.ink : Theme.inkDim
                        Behavior on color { ColorAnimation { duration: Theme.quick } }
                    }

                    Text {
                        id: cardName
                        anchors {
                            left: parent.left; leftMargin: Theme.s3
                            right: parent.right; rightMargin: Theme.s3
                            top: cardGlyph.bottom; topMargin: Theme.s1
                        }
                        width: parent.width
                        text: I18n.tr(card.entry.label || card.entry.id)
                        color: card.on ? Theme.ink : Theme.inkSoft
                        font.family: Theme.font
                        font.pixelSize: Theme.fBody
                        font.weight: card.on ? Font.DemiBold : Font.Medium
                        elide: Text.ElideRight
                        Behavior on color { ColorAnimation { duration: Theme.quick } }
                    }
                    Text {
                        anchors {
                            left: parent.left; leftMargin: Theme.s3
                            right: parent.right; rightMargin: Theme.s3
                            top: cardName.bottom; topMargin: 1
                            bottom: parent.bottom; bottomMargin: Theme.s2
                        }
                        visible: panel.hintFor(card.entry.id).length > 0
                        text: I18n.tr(panel.hintFor(card.entry.id))
                        color: Theme.inkDim
                        font.family: Theme.font
                        font.pixelSize: Theme.fSmall
                        wrapMode: Text.WordWrap
                        maximumLineCount: 2
                        elide: Text.ElideRight
                    }

                    // Click anywhere on the card toggles it and moves the
                    // highlight there. Declared before the tune affordance so
                    // the tune button sits on top and keeps its own clicks.
                    MouseArea {
                        id: cardMa
                        anchors.fill: parent
                        hoverEnabled: true
                        cursorShape: Qt.PointingHandCursor
                        onClicked: {
                            panel.curNav = card.index;
                            grid.forceActiveFocus();
                            panel.toggle(card.entry.id);
                        }
                    }

                    // The on/off mark: a filled dot, top-right, so the lit
                    // cards read as the desktop's current cast at a glance.
                    Rectangle {
                        id: cardDot
                        anchors { top: parent.top; topMargin: Theme.s3; right: parent.right; rightMargin: Theme.s3 }
                        width: 10; height: 10
                        radius: 5
                        color: card.on ? Theme.ink : "transparent"
                        border.width: 1
                        border.color: card.on ? Theme.ink : Theme.lineStrong
                        Behavior on color { ColorAnimation { duration: Theme.quick } }
                        Behavior on border.color { ColorAnimation { duration: Theme.quick } }
                    }

                    // A quiet Customize affordance: open the widget's editor for
                    // an on card, so a face can be tuned from the picker too.
                    MaterialIcon {
                        id: tuneBtn
                        visible: card.on
                        anchors { right: parent.right; rightMargin: Theme.s3; bottom: parent.bottom; bottomMargin: Theme.s2 }
                        text: "tune"
                        font.pixelSize: 18
                        color: tuneMa.containsMouse ? Theme.ink : Theme.inkDim
                        Behavior on color { ColorAnimation { duration: Theme.quick } }
                        MouseArea {
                            id: tuneMa
                            anchors.fill: parent
                            anchors.margins: -Theme.s1
                            hoverEnabled: true
                            cursorShape: Qt.PointingHandCursor
                            onClicked: panel.customize(card.entry.id)
                        }
                    }
                }
            }
        }
    }

    // Nothing matched the search: honest empty paper, not a blank panel.
    Text {
        anchors.centerIn: grid
        visible: panel.filtered.length === 0
        text: I18n.tr("No widgets match")
        color: Theme.inkDim
        font.family: Theme.font
        font.pixelSize: Theme.fSmall
    }
}
