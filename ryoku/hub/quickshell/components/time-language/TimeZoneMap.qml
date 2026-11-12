pragma ComponentBehavior: Bound

import QtQuick
import Quickshell.Io
import Lithic
import Ryoku.Ui.Singletons
import "../../world-land.js" as World
import "../../tzmap.js" as Tz

// choose a time zone on a world map or by name; emits chosen(zone) on Set
Dialog {
    id: root

    property string currentZone: ""
    property var zones: []
    property var selected: null
    property var hovered: null
    property string query: ""
    property string localTime: ""
    readonly property var results: Tz.search(root.zones, root.query).slice(0, 6)
    readonly property bool changed: root.selected !== null && root.selected.tz !== root.currentZone

    signal chosen(string zone)

    title: I18n.tr("Choose your time zone")
    dialogWidth: 960
    actions: [
        { text: I18n.tr("Cancel"), kind: "quiet" },
        { text: I18n.tr("Set time zone"), kind: "gold", enabled: root.changed }
    ]
    onActionTriggered: index => {
        if (index === 1 && root.changed)
            root.chosen(root.selected.tz);
    }

    function show() {
        root.query = "";
        search.text = "";
        if (root.zones.length === 0)
            zoneLoad.running = true;
        else
            root.selectByName(root.currentZone);
        root.open();
        // the sky is drawn for the moment the map opens; nothing repaints at rest
        sky.requestPaint();
        search.forceActiveFocus();
    }

    function selectByName(name) {
        for (var i = 0; i < root.zones.length; i++) {
            if (root.zones[i].tz === name) {
                root.pick(root.zones[i]);
                return;
            }
        }
        root.selected = null;
        root.localTime = "";
    }

    function pick(z) {
        if (!z)
            return;
        root.selected = z;
        clockProc.command = ["env", "TZ=" + z.tz, "date", TimeFormat.is24h ? "+%H:%M  %Z  UTC%z" : "+%-I:%M %p  %Z  UTC%z"];
        clockProc.running = true;
    }

    function pickAt(x, y) {
        if (root.zones.length === 0)
            return;
        root.pick(Tz.nearest(root.zones, Tz.latAt(y, map.height), Tz.lonAt(x, map.width)));
    }

    function zoneName(tz) {
        return String(tz || "").replace(/_/g, " ");
    }

    // a zone shared by countries may note another country's area (Asia/Tokyo: Eyre Bird Observatory)
    function noteOf(z, searching) {
        if (!z || !z.note)
            return "";
        if (String(z.cc).indexOf(",") < 0)
            return z.note;
        var q = root.query.trim().toLowerCase();
        return searching && q.length > 0 && z.note.toLowerCase().indexOf(q) >= 0 ? z.note : "";
    }

    Process {
        id: zoneLoad
        command: ["sh", "-c", "cat /usr/share/zoneinfo/zone1970.tab 2>/dev/null || cat /usr/share/zoneinfo/zone.tab 2>/dev/null"]
        stdout: StdioCollector {
            onStreamFinished: {
                root.zones = Tz.parseZoneTab(this.text);
                root.selectByName(root.currentZone);
            }
        }
    }

    Process {
        id: clockProc
        stdout: StdioCollector { onStreamFinished: root.localTime = this.text.trim() }
    }

    Column {
        width: parent.width
        spacing: LithicTokens.space.s3

        Label {
            width: parent.width
            text: I18n.tr("Click near a city on the map, or search for one. The clock changes when you press Set.")
            typeRole: "body"
            inkRole: "fg2"
            wrapMode: Text.Wrap
            elide: Text.ElideNone
        }

        Search {
            id: search
            width: Math.min(parent.width, LithicTokens.size.settingsControlMax)
            placeholderText: I18n.tr("Search a city or zone")
            accessibleName: I18n.tr("Search a city or zone")
            onTextChanged: root.query = text
            Keys.onDownPressed: if (resultMenu.visible) resultMenu.focusItem(0, true)
            onAccepted: {
                if (root.results.length > 0) {
                    root.pick(root.results[0]);
                    search.text = "";
                }
            }
        }

        Item {
            id: mapBox
            width: parent.width
            // the map keeps its 2:1 shape but never pushes the actions off a short window
            height: Math.max(160, Math.min(width / 2, (root.parent ? root.parent.height : 800) - 380))

            Item {
                id: map
                anchors.centerIn: parent
                width: Math.min(mapBox.width, mapBox.height * 2)
                height: width / 2
                clip: true

                Canvas {
                    id: land
                    anchors.fill: parent
                    readonly property color sea: Lithic.field
                    readonly property color ground: Lithic.alpha(Lithic.fg, 0.16)
                    readonly property color coast: Lithic.edge
                    readonly property color grid: Lithic.vein
                    onSeaChanged: requestPaint()
                    onGroundChanged: requestPaint()
                    onWidthChanged: requestPaint()
                    onHeightChanged: requestPaint()
                    onPaint: {
                        var ctx = getContext("2d");
                        var w = width, h = height;
                        ctx.reset();
                        if (w <= 0 || h <= 0)
                            return;
                        ctx.fillStyle = land.sea;
                        ctx.fillRect(0, 0, w, h);
                        ctx.strokeStyle = land.grid;
                        ctx.lineWidth = 1;
                        for (var lon = -150; lon < 180; lon += 30) {
                            ctx.beginPath(); ctx.moveTo(Tz.px(lon, w), 0); ctx.lineTo(Tz.px(lon, w), h); ctx.stroke();
                        }
                        for (var lat = -60; lat < 90; lat += 30) {
                            ctx.beginPath(); ctx.moveTo(0, Tz.py(lat, h)); ctx.lineTo(w, Tz.py(lat, h)); ctx.stroke();
                        }
                        ctx.fillStyle = land.ground;
                        ctx.strokeStyle = land.coast;
                        ctx.lineWidth = 0.6;
                        var rings = World.rings;
                        for (var i = 0; i < rings.length; i++) {
                            var r = rings[i];
                            if (r.length < 3)
                                continue;
                            ctx.beginPath();
                            for (var j = 0; j < r.length; j++) {
                                var X = Tz.px(r[j][0], w), Y = Tz.py(r[j][1], h);
                                if (j === 0) ctx.moveTo(X, Y); else ctx.lineTo(X, Y);
                            }
                            ctx.closePath();
                            ctx.fill();
                            ctx.stroke();
                        }
                    }
                }

                // night and the line between day and night, as they are when the map opens
                Canvas {
                    id: sky
                    anchors.fill: parent
                    readonly property color night: Lithic.alpha(Lithic.ground, 0.5)
                    readonly property color edge: Lithic.alpha(Lithic.gold, 0.6)
                    onNightChanged: requestPaint()
                    onWidthChanged: requestPaint()
                    onHeightChanged: requestPaint()
                    onPaint: {
                        var ctx = getContext("2d");
                        var w = width, h = height;
                        ctx.reset();
                        if (w <= 0 || h <= 0)
                            return;
                        var sub = Tz.subsolar(new Date());
                        var step = 3;
                        ctx.fillStyle = sky.night;
                        for (var x = 0; x < w; x += step) {
                            var lon = Tz.lonAt(x + step / 2, w);
                            var tb = Tz.terminatorLat(lon, sub);
                            if (tb === null) {
                                if (!Tz.isDay(0, lon, sub))
                                    ctx.fillRect(x, 0, step, h);
                            } else {
                                var yb = Tz.py(tb, h);
                                if (sub.lat > 0)
                                    ctx.fillRect(x, yb, step, h - yb);
                                else
                                    ctx.fillRect(x, 0, step, yb);
                            }
                        }
                        ctx.strokeStyle = sky.edge;
                        ctx.lineWidth = 1.4;
                        ctx.beginPath();
                        var started = false;
                        for (var x2 = 0; x2 <= w; x2 += step) {
                            var tb2 = Tz.terminatorLat(Tz.lonAt(x2, w), sub);
                            if (tb2 === null) {
                                started = false;
                                continue;
                            }
                            if (!started) {
                                ctx.moveTo(x2, Tz.py(tb2, h));
                                started = true;
                            } else {
                                ctx.lineTo(x2, Tz.py(tb2, h));
                            }
                        }
                        ctx.stroke();
                        ctx.fillStyle = sky.edge;
                        ctx.beginPath();
                        ctx.arc(Tz.px(sub.lon, w), Tz.py(sub.lat, h), 4, 0, 2 * Math.PI);
                        ctx.fill();
                    }
                }

                Repeater {
                    model: root.zones
                    delegate: Rectangle {
                        required property var modelData
                        readonly property bool current: modelData.tz === root.currentZone
                        width: current ? 7 : 4
                        height: width
                        radius: width / 2
                        x: Tz.px(modelData.lon, map.width) - width / 2
                        y: Tz.py(modelData.lat, map.height) - height / 2
                        color: current ? Lithic.gold : Lithic.alpha(Lithic.fg, 0.5)
                    }
                }

                Rectangle {
                    visible: root.hovered !== null && mapMouse.containsMouse
                    width: 16
                    height: 16
                    radius: 8
                    color: "transparent"
                    border.width: 1.5
                    border.color: Lithic.fg
                    x: root.hovered ? Tz.px(root.hovered.lon, map.width) - 8 : 0
                    y: root.hovered ? Tz.py(root.hovered.lat, map.height) - 8 : 0
                }

                // the chosen zone's pin
                Item {
                    visible: root.selected !== null
                    x: root.selected ? Tz.px(root.selected.lon, map.width) : 0
                    y: root.selected ? Tz.py(root.selected.lat, map.height) : 0
                    ChiselNumberBehavior on x { duration: LithicTokens.motion.select }
                    ChiselNumberBehavior on y { duration: LithicTokens.motion.select }
                    Rectangle {
                        width: 2
                        height: 14
                        x: -1
                        y: -14
                        color: Lithic.gold
                    }
                    CutRect {
                        width: 12
                        height: 12
                        x: -6
                        y: -24
                        cut: 3
                        fill: Lithic.gold
                    }
                }

                MouseArea {
                    id: mapMouse
                    anchors.fill: parent
                    hoverEnabled: true
                    cursorShape: Qt.CrossCursor
                    onPositionChanged: m => {
                        tip.mx = m.x;
                        tip.my = m.y;
                        if (root.zones.length > 0)
                            root.hovered = Tz.nearest(root.zones, Tz.latAt(m.y, map.height), Tz.lonAt(m.x, map.width));
                    }
                    onExited: root.hovered = null
                    onClicked: m => root.pickAt(m.x, m.y)
                }

                CutRect {
                    id: tip
                    property real mx: 0
                    property real my: 0
                    visible: root.hovered !== null && mapMouse.containsMouse
                    x: Math.max(2, Math.min(mx + 12, map.width - width - 2))
                    y: Math.max(2, my - height - 8)
                    width: tipText.implicitWidth + LithicTokens.space.s3 * 2
                    height: tipText.implicitHeight + LithicTokens.space.s1 * 2
                    cut: LithicTokens.cut.token
                    fill: Lithic.stone4
                    strokeColor: Lithic.vein
                    strokeWidth: LithicTokens.size.hairline
                    Label {
                        id: tipText
                        anchors.centerIn: parent
                        text: root.hovered ? Tz.pretty(root.hovered.tz) : ""
                        typeRole: "caption"
                    }
                }
            }

            // search results over the map's top edge
            Menu {
                id: resultMenu
                visible: root.query.length > 0 && root.results.length > 0
                x: 0
                y: 0
                z: 2
                menuWidth: Math.min(mapBox.width, LithicTokens.size.settingsControlMax)
                items: root.results.map(z => ({ label: root.zoneName(z.tz) + (root.noteOf(z, true) ? " · " + root.noteOf(z, true) : "") }))
                onTriggered: index => {
                    root.pick(root.results[index]);
                    search.text = "";
                    search.forceActiveFocus();
                }
                Keys.onEscapePressed: search.forceActiveFocus()
            }
        }

        Row {
            width: parent.width
            spacing: LithicTokens.space.s4

            Column {
                width: parent.width
                spacing: LithicTokens.size.settingsDetailGap
                Label {
                    width: parent.width
                    text: root.selected ? root.zoneName(root.selected.tz) : I18n.tr("No zone chosen yet")
                    typeRole: "row"
                }
                Label {
                    width: parent.width
                    visible: text.length > 0
                    text: [root.localTime, root.noteOf(root.selected, false)].filter(s => s.length > 0).join("  ·  ")
                    typeRole: "detail"
                    numeric: true
                }
            }
        }
    }
}
